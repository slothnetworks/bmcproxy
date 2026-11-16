package transport

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// BreakerState is a circuit breaker state.
type BreakerState int

const (
	// BreakerClosed: requests flow normally.
	BreakerClosed BreakerState = iota
	// BreakerOpen: requests fail fast (ErrBusy / 503) until the cool-down
	// elapses.
	BreakerOpen
	// BreakerHalfOpen: a limited number of probe requests are let through;
	// success closes the breaker, failure re-opens it.
	BreakerHalfOpen
)

func (s BreakerState) String() string {
	switch s {
	case BreakerClosed:
		return "closed"
	case BreakerOpen:
		return "open"
	case BreakerHalfOpen:
		return "half-open"
	}
	return fmt.Sprintf("state(%d)", int(s))
}

// MarshalText renders the state name in JSON.
func (s BreakerState) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// Gauge returns 0 closed, 1 half-open, 2 open (for a metrics gauge).
func (s BreakerState) Gauge() float64 {
	switch s {
	case BreakerOpen:
		return 2
	case BreakerHalfOpen:
		return 1
	}
	return 0
}

// BreakerConfig configures a per-host breaker.
type BreakerConfig struct {
	// FailureThreshold consecutive failures trip the breaker. Default 5.
	FailureThreshold int
	// OpenTimeout is the cool-down before half-opening. Default 30s.
	OpenTimeout time.Duration
	// HalfOpenProbes is how many concurrent probes half-open allows.
	// Default 1.
	HalfOpenProbes int
	// Disabled turns the breaker into a no-op (always closed).
	Disabled bool
}

func (c BreakerConfig) withDefaults() BreakerConfig {
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 5
	}
	if c.OpenTimeout <= 0 {
		c.OpenTimeout = 30 * time.Second
	}
	if c.HalfOpenProbes <= 0 {
		c.HalfOpenProbes = 1
	}
	return c
}

// Outcome is reported back to the breaker after a guarded call.
type Outcome int

const (
	// OutcomeSuccess: the BMC answered (any status not counted as failure).
	OutcomeSuccess Outcome = iota
	// OutcomeFailure: connection error, timeout, 502/503/504, evasive 403.
	OutcomeFailure
	// OutcomeIgnore: the call says nothing about BMC health (caller
	// cancelled); releases a half-open probe slot without a transition.
	OutcomeIgnore
)

// ErrBreakerOpen is returned by Breaker.Allow while open (wrapped into
// bmc.ErrBusy by Endpoint).
var ErrBreakerOpen = errors.New("circuit breaker open")

// Breaker is a consecutive-failure circuit breaker. Safe for concurrent use.
type Breaker struct {
	cfg      BreakerConfig
	now      func() time.Time
	onChange func(from, to BreakerState)

	mu       sync.Mutex
	state    BreakerState
	failures int
	openedAt time.Time
	probes   int
	trips    uint64
}

// NewBreaker builds a breaker. now may be nil (time.Now); onChange may be nil
// and is called without the lock held.
func NewBreaker(cfg BreakerConfig, now func() time.Time, onChange func(from, to BreakerState)) *Breaker {
	if now == nil {
		now = time.Now
	}
	return &Breaker{cfg: cfg.withDefaults(), now: now, onChange: onChange}
}

// Allow asks permission for one call. On success it returns a done func that
// must be called exactly once with the outcome. While open it returns
// ErrBreakerOpen.
func (b *Breaker) Allow() (done func(Outcome), err error) {
	if b.cfg.Disabled {
		return func(Outcome) {}, nil
	}
	b.mu.Lock()
	var change [2]BreakerState
	changed := false
	switch b.state {
	case BreakerOpen:
		if b.now().Sub(b.openedAt) < b.cfg.OpenTimeout {
			b.mu.Unlock()
			return nil, ErrBreakerOpen
		}
		change, changed = [2]BreakerState{BreakerOpen, BreakerHalfOpen}, true
		b.state = BreakerHalfOpen
		b.probes = 0
		fallthrough
	case BreakerHalfOpen:
		if b.probes >= b.cfg.HalfOpenProbes {
			b.mu.Unlock()
			b.notify(changed, change)
			return nil, ErrBreakerOpen
		}
		b.probes++
		b.mu.Unlock()
		b.notify(changed, change)
		return b.doneFunc(true), nil
	}
	b.mu.Unlock()
	return b.doneFunc(false), nil
}

func (b *Breaker) doneFunc(probe bool) func(Outcome) {
	var once sync.Once
	return func(o Outcome) { once.Do(func() { b.record(o, probe) }) }
}

func (b *Breaker) record(o Outcome, probe bool) {
	b.mu.Lock()
	from := b.state
	if probe && b.probes > 0 {
		b.probes--
	}
	switch o {
	case OutcomeSuccess:
		b.failures = 0
		if b.state == BreakerHalfOpen {
			b.state = BreakerClosed
		}
	case OutcomeFailure:
		b.failures++
		if b.state == BreakerHalfOpen || (b.state == BreakerClosed && b.failures >= b.cfg.FailureThreshold) {
			b.state = BreakerOpen
			b.openedAt = b.now()
			b.trips++
		}
	}
	to := b.state
	b.mu.Unlock()
	b.notify(from != to, [2]BreakerState{from, to})
}

func (b *Breaker) notify(changed bool, c [2]BreakerState) {
	if changed && b.onChange != nil {
		b.onChange(c[0], c[1])
	}
}

// State returns the current state. An open breaker whose cool-down has
// elapsed is still reported open until the next Allow half-opens it.
func (b *Breaker) State() BreakerState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// BreakerSnapshot is a point-in-time view of a breaker.
type BreakerSnapshot struct {
	State               BreakerState `json:"state"`
	ConsecutiveFailures int          `json:"consecutiveFailures"`
	OpenedAt            time.Time    `json:"openedAt,omitzero"`
	Trips               uint64       `json:"trips"`
}

// Snapshot returns the breaker's state and counters.
func (b *Breaker) Snapshot() BreakerSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := BreakerSnapshot{State: b.state, ConsecutiveFailures: b.failures, Trips: b.trips}
	if b.state != BreakerClosed {
		s.OpenedAt = b.openedAt
	}
	return s
}

// Reset forces the breaker closed (e.g. operator action, or after a known
// BMC reboot completed).
func (b *Breaker) Reset() {
	b.mu.Lock()
	from := b.state
	b.state, b.failures, b.probes = BreakerClosed, 0, 0
	b.mu.Unlock()
	b.notify(from != BreakerClosed, [2]BreakerState{from, BreakerClosed})
}
