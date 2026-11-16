package sessions_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

type clock struct{ ns atomic.Int64 }

func (c *clock) Now() time.Time      { return time.Unix(0, c.ns.Load()) }
func (c *clock) Add(d time.Duration) { c.ns.Add(int64(d)) }
func newClock() *clock {
	c := &clock{}
	c.ns.Store(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano())
	return c
}
func target(t *testing.T, h string) bmc.Target {
	tg, err := bmc.ParseTarget(h)
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

func newMgr(t *testing.T, d *servertest.Driver, clk *clock) *sessions.Manager {
	reg := bmc.NewRegistry()
	reg.Register(d)
	cfg := sessions.Config{Registry: reg, IdleTTL: time.Minute, SweepInterval: -1, Obs: obs.Discard(), SkewSync: true}
	if clk != nil {
		cfg.Now = clk.Now
	}
	m := sessions.New(cfg)
	t.Cleanup(func() { _ = m.Shutdown(context.Background()) })
	return m
}

var creds = bmc.Creds{User: "admin", Pass: "hunter2-secret"}

func TestReuseAndSingleflightOpen(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", OpenDelay: 30 * time.Millisecond}
	m := newMgr(t, d, nil)
	tg := target(t, "10.0.0.1")
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, err := m.Acquire(context.Background(), tg, creds)
			if err != nil {
				t.Error(err)
				return
			}
			h.Release()
		}()
	}
	wg.Wait()
	for range 5 {
		h, err := m.Acquire(context.Background(), tg, creds)
		if err != nil {
			t.Fatal(err)
		}
		if !h.Reused {
			t.Error("expected reuse")
		}
		h.Release()
	}
	if n := d.Opens.Load(); n != 1 {
		t.Fatalf("Open called %d times, want 1", n)
	}
	// A different password is a different key.
	h, err := m.Acquire(context.Background(), tg, bmc.Creds{User: "admin", Pass: "other"})
	if err != nil {
		t.Fatal(err)
	}
	h.Release()
	if n := d.Opens.Load(); n != 2 {
		t.Fatalf("Open called %d times, want 2", n)
	}
}

func TestIdleEvictionLogsOut(t *testing.T) {
	clk := newClock()
	d := &servertest.Driver{NameV: "fake"}
	m := newMgr(t, d, clk)
	tg := target(t, "10.0.0.1")
	h, err := m.Acquire(context.Background(), tg, creds)
	if err != nil {
		t.Fatal(err)
	}
	clk.Add(2 * time.Minute)
	if n := m.Sweep(); n != 0 {
		t.Fatalf("evicted in-use session (%d)", n)
	}
	h.Release()
	clk.Add(30 * time.Second)
	if n := m.Sweep(); n != 0 {
		t.Fatalf("evicted before TTL (%d)", n)
	}
	clk.Add(31 * time.Second)
	if n := m.Sweep(); n != 1 {
		t.Fatalf("Sweep evicted %d, want 1", n)
	}
	if c := d.Sessions()[0].Closed.Load(); c != 1 {
		t.Fatalf("Close called %d times, want 1", c)
	}
	if m.Len() != 0 {
		t.Fatal("session still cached")
	}
	if m.Counters().Evictions != 1 {
		t.Fatal("eviction not counted")
	}
}

func TestShutdownLogsOutAll(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	m := newMgr(t, d, nil)
	for _, host := range []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"} {
		h, err := m.Acquire(context.Background(), target(t, host), creds)
		if err != nil {
			t.Fatal(err)
		}
		h.Release()
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, s := range d.Sessions() {
		if s.Closed.Load() != 1 {
			t.Fatalf("session %s not logged out", s.Target)
		}
	}
	if _, err := m.Acquire(context.Background(), target(t, "10.0.0.1"), creds); !errors.Is(err, bmc.ErrBusy) {
		t.Fatalf("Acquire after shutdown: %v", err)
	}
}

func TestInvalidateHostAndKey(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	m := newMgr(t, d, nil)
	a, b := target(t, "10.0.0.1"), target(t, "10.0.0.2")
	for _, tg := range []bmc.Target{a, b} {
		for _, c := range []bmc.Creds{creds, {User: "op", Pass: "x"}} {
			h, err := m.Acquire(context.Background(), tg, c)
			if err != nil {
				t.Fatal(err)
			}
			h.Release()
		}
	}
	if n := m.InvalidateHost("10.0.0.1"); n != 2 {
		t.Fatalf("InvalidateHost dropped %d, want 2", n)
	}
	if m.Len() != 2 || m.HostSessions("10.0.0.2") != 2 {
		t.Fatal("other host affected")
	}
	closed := 0
	for _, s := range d.Sessions() {
		closed += int(s.Closed.Load())
	}
	if closed != 2 {
		t.Fatalf("closed %d, want 2", closed)
	}
	if !m.Invalidate(sessions.KeyFor(b, creds)) || m.Len() != 1 {
		t.Fatal("Invalidate(key) failed")
	}
	// In-use sessions are closed only on release.
	h, _ := m.Acquire(context.Background(), b, bmc.Creds{User: "op", Pass: "x"})
	m.InvalidateHost(b.Key())
	s := d.Sessions()[3]
	if s.Closed.Load() != 0 {
		t.Fatal("closed while in use")
	}
	h.Release()
	servertest.Eventually(t, time.Second, func() bool { return s.Closed.Load() == 1 }, "deferred close")
}

func TestDoReloginOnce(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	m := newMgr(t, d, nil)
	tg := target(t, "10.0.0.1")
	h, _ := m.Acquire(context.Background(), tg, creds)
	h.Release()
	d.Sessions()[0].FailAuth.Store(1)
	calls := 0
	err := m.Do(context.Background(), tg, creds, func(ctx context.Context, h *sessions.Handle) error {
		calls++
		_, err := h.Session.(*servertest.Session).SystemInfo(ctx)
		return err
	})
	if err != nil || calls != 2 || d.Opens.Load() != 2 {
		t.Fatalf("err=%v calls=%d opens=%d", err, calls, d.Opens.Load())
	}
	ti, _ := m.Targets().Get(tg.Key())
	if ti.Degraded == "" || ti.Relogins != 1 {
		t.Fatalf("target not marked degraded: %+v", ti)
	}
	// A fresh session's auth failure is surfaced without retry.
	d2 := &servertest.Driver{NameV: "fake", Creds: map[string]string{"admin": "right"}}
	m2 := newMgr(t, d2, nil)
	err = m2.Do(context.Background(), tg, creds, func(context.Context, *sessions.Handle) error { return nil })
	if !errors.Is(err, bmc.ErrAuth) || d2.Opens.Load() != 1 {
		t.Fatalf("err=%v opens=%d", err, d2.Opens.Load())
	}
}

func TestSnapshotsNeverExposeSecrets(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Configure: func(s *servertest.Session) { s.Clock = 3 * time.Hour }}
	m := newMgr(t, d, nil)
	tg := target(t, "10.0.0.1")
	h, err := m.Acquire(context.Background(), tg, creds)
	if err != nil {
		t.Fatal(err)
	}
	h.Release()
	b, _ := json.Marshal(map[string]any{"s": m.Snapshot(), "t": m.Targets().Snapshot(), "k": sessions.KeyFor(tg, creds).String()})
	if strings.Contains(string(b), creds.Pass) {
		t.Fatalf("password leaked: %s", b)
	}
	snap := m.Snapshot()
	if len(snap) != 1 || snap[0].User != "admin" || snap[0].Driver != "fake" {
		t.Fatalf("snapshot %+v", snap)
	}
	// Clock skew measured via TimeManager at open.
	off, ok := m.Targets().ClockOffset(tg.Key())
	if !ok || off != 3*time.Hour {
		t.Fatalf("clock offset %v %v", off, ok)
	}
	ti, _ := m.Targets().Get(tg.Key())
	if ti.Vendor != "fake" || ti.Board != "FAKE1" || len(ti.Backends) != 1 {
		t.Fatalf("target %+v", ti)
	}
}

func TestTargetCacheSkipsDetect(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	m := newMgr(t, d, nil)
	tg := target(t, "10.0.0.1")
	for _, c := range []bmc.Creds{creds, {User: "u2", Pass: "p"}, {User: "u3", Pass: "p"}} {
		h, err := m.Acquire(context.Background(), tg, c)
		if err != nil {
			t.Fatal(err)
		}
		h.Release()
	}
	if n := d.Detects.Load(); n != 1 {
		t.Fatalf("Detect called %d times, want 1", n)
	}
}

func TestDoSessionLostInvalidatesHost(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	m := newMgr(t, d, nil)
	tg := target(t, "10.0.0.2")
	other := bmc.Creds{User: "other", Pass: "pw"}
	for _, c := range []bmc.Creds{creds, other} {
		h, err := m.Acquire(context.Background(), tg, c)
		if err != nil {
			t.Fatal(err)
		}
		h.Release()
	}
	calls := 0
	err := m.Do(context.Background(), tg, creds, func(ctx context.Context, h *sessions.Handle) error {
		calls++
		if calls == 1 {
			return bmc.Wrap("test", "op", bmc.ErrSessionLost)
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	// Both cached sessions (both users) were dropped; only the retry's session remains.
	servertest.Eventually(t, time.Second, func() bool {
		closed := 0
		for _, s := range d.Sessions() {
			closed += int(s.Closed.Load())
		}
		return closed == 2
	}, "all host sessions closed")
	if got := bmc.HTTPStatus(bmc.ErrSessionLost); got != 502 {
		t.Fatalf("ErrSessionLost status=%d want 502", got)
	}
}
