package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoalesceAndTTL(t *testing.T) {
	now := time.Unix(1000, 0)
	var mu sync.Mutex
	c := New(Config{TTL: time.Second, Now: func() time.Time { mu.Lock(); defer mu.Unlock(); return now }})
	k := Key{Host: "h:443", User: "u", Op: "sensors"}
	var calls atomic.Int32
	gate := make(chan struct{})
	load := func(ctx context.Context) (any, error) {
		calls.Add(1)
		<-gate
		return 42, nil
	}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := c.Get(context.Background(), k, -1, load)
			if err != nil || v.(int) != 42 {
				t.Errorf("got %v %v", v, err)
			}
		}()
	}
	time.Sleep(20 * time.Millisecond)
	close(gate)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("loader ran %d times", calls.Load())
	}
	if _, err := c.Get(context.Background(), k, -1, load); err != nil || calls.Load() != 1 {
		t.Fatal("expected cache hit")
	}
	mu.Lock()
	now = now.Add(2 * time.Second)
	mu.Unlock()
	if _, err := c.Get(context.Background(), k, -1, load); err != nil || calls.Load() != 2 {
		t.Fatal("expected expiry")
	}
	st := c.Stats()
	if st.Misses != 2 || st.Hits != 1 || st.Coalesced != 9 {
		t.Fatalf("stats %+v", st)
	}
}

func TestErrorsNotCachedAndInvalidate(t *testing.T) {
	c := New(Config{TTL: time.Hour})
	k := Key{Host: "h:443", User: "u", Op: "x"}
	n := 0
	load := func(context.Context) (any, error) {
		n++
		if n == 1 {
			return nil, errors.New("boom")
		}
		return n, nil
	}
	if _, err := c.Get(context.Background(), k, -1, load); err == nil {
		t.Fatal("want error")
	}
	if v, _ := c.Get(context.Background(), k, -1, load); v.(int) != 2 {
		t.Fatal("error was cached")
	}
	c.InvalidateHost("other:443")
	if v, _ := c.Get(context.Background(), k, -1, load); v.(int) != 2 {
		t.Fatal("unrelated invalidation dropped entry")
	}
	c.InvalidateHost("h:443")
	if v, _ := c.Get(context.Background(), k, -1, load); v.(int) != 3 {
		t.Fatal("invalidation ignored")
	}
}

// A load that started before a write must not repopulate the cache after it.
func TestInvalidateDuringLoad(t *testing.T) {
	c := New(Config{TTL: time.Hour})
	k := Key{Host: "h:443", Op: "x"}
	started, release := make(chan struct{}), make(chan struct{})
	go func() {
		_, _ = c.Get(context.Background(), k, -1, func(context.Context) (any, error) {
			close(started)
			<-release
			return "stale", nil
		})
	}()
	<-started
	c.InvalidateHost("h:443")
	v, _ := c.Get(context.Background(), k, -1, func(context.Context) (any, error) { return "fresh", nil })
	close(release)
	if v != "fresh" {
		t.Fatalf("got %v, joined pre-write load", v)
	}
	time.Sleep(10 * time.Millisecond)
	v, _ = c.Get(context.Background(), k, -1, func(context.Context) (any, error) { return "fresh2", nil })
	if v == "stale" {
		t.Fatal("stale result stored after invalidation")
	}
}

func TestCallerCancelDoesNotCancelShared(t *testing.T) {
	c := New(Config{TTL: time.Hour})
	k := Key{Host: "h:443", Op: "x"}
	gate := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		_, err := c.Get(ctx, k, -1, func(lctx context.Context) (any, error) {
			<-gate
			return "v", lctx.Err()
		})
		done <- err
	}()
	time.Sleep(10 * time.Millisecond)
	res := make(chan any)
	go func() {
		v, _ := c.Get(context.Background(), k, -1, nil)
		res <- v
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("first caller: %v", err)
	}
	close(gate)
	if v := <-res; v != "v" {
		t.Fatalf("second caller got %v", v)
	}
}
