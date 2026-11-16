package transport

import (
	"container/list"
	"context"
	"sync"
)

// Priority orders waiters in a Limiter queue. Within a priority the queue is
// FIFO; all PriorityWrite waiters are served before any PriorityRead waiter.
type Priority int

const (
	// PriorityAuto derives the priority from the HTTP method: GET/HEAD/
	// OPTIONS are reads, everything else is a write.
	PriorityAuto Priority = iota
	// PriorityRead is the normal queue.
	PriorityRead
	// PriorityWrite jumps ahead of queued reads (a user-visible action
	// should not wait behind a sensor poll backlog).
	PriorityWrite
)

func (p Priority) String() string {
	switch p {
	case PriorityRead:
		return "read"
	case PriorityWrite:
		return "write"
	}
	return "auto"
}

// Limiter is a counting semaphore with a two-level priority FIFO queue and
// context-aware Acquire. It is the per-host (per lane) concurrency cap: 1 for
// AMI /api, up to 4 for Redfish.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	inUse  int
	high   list.List // *waiter
	normal list.List
}

type waiter struct {
	ch      chan struct{}
	granted bool
}

// NewLimiter returns a limiter admitting n concurrent holders (n < 1 is 1).
func NewLimiter(n int) *Limiter {
	if n < 1 {
		n = 1
	}
	return &Limiter{limit: n}
}

// Acquire blocks until a slot is free or ctx is done. PriorityAuto is
// treated as PriorityRead.
func (l *Limiter) Acquire(ctx context.Context, p Priority) error {
	l.mu.Lock()
	if l.inUse < l.limit && l.high.Len() == 0 && l.normal.Len() == 0 {
		l.inUse++
		l.mu.Unlock()
		return nil
	}
	if err := ctx.Err(); err != nil {
		l.mu.Unlock()
		return err
	}
	w := &waiter{ch: make(chan struct{})}
	q := &l.normal
	if p == PriorityWrite {
		q = &l.high
	}
	e := q.PushBack(w)
	l.mu.Unlock()

	select {
	case <-w.ch:
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		if w.granted {
			// granted concurrently with cancellation: give the slot back
			l.inUse--
			l.grantLocked()
		} else {
			q.Remove(e)
		}
		l.mu.Unlock()
		return ctx.Err()
	}
}

// TryAcquire takes a slot without waiting (and without jumping a queue).
func (l *Limiter) TryAcquire() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.inUse < l.limit && l.high.Len() == 0 && l.normal.Len() == 0 {
		l.inUse++
		return true
	}
	return false
}

// Release frees a slot and wakes the next waiter.
func (l *Limiter) Release() {
	l.mu.Lock()
	if l.inUse <= 0 {
		l.mu.Unlock()
		panic("transport: Limiter.Release without Acquire")
	}
	l.inUse--
	l.grantLocked()
	l.mu.Unlock()
}

func (l *Limiter) grantLocked() {
	for l.inUse < l.limit {
		var e *list.Element
		if e = l.high.Front(); e != nil {
			l.high.Remove(e)
		} else if e = l.normal.Front(); e != nil {
			l.normal.Remove(e)
		} else {
			return
		}
		w := e.Value.(*waiter)
		w.granted = true
		l.inUse++
		close(w.ch)
	}
}

// SetLimit changes the concurrency cap; raising it wakes waiters.
func (l *Limiter) SetLimit(n int) {
	if n < 1 {
		n = 1
	}
	l.mu.Lock()
	l.limit = n
	l.grantLocked()
	l.mu.Unlock()
}

// Limit returns the cap.
func (l *Limiter) Limit() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.limit
}

// InFlight returns the number of held slots.
func (l *Limiter) InFlight() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inUse
}

// Waiting returns the queue depth (both priorities).
func (l *Limiter) Waiting() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.high.Len() + l.normal.Len()
}
