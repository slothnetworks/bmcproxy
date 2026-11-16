package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

func fastRetry() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond}
}

func newMgr(t *testing.T, cfg Config) *Manager {
	t.Helper()
	if cfg.Retry == (RetryPolicy{}) {
		cfg.Retry = fastRetry()
	}
	if cfg.Obs == nil {
		cfg.Obs = obs.Discard()
	}
	m := NewManager(cfg)
	t.Cleanup(m.Close)
	return m
}

func hostOf(srv *httptest.Server) string { return srv.Listener.Addr().String() }

func get(t *testing.T, ep *Endpoint, url string) (*http.Response, error) {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	return ep.Do(req)
}

// ---- limiter ----

func TestLimiterNeverExceedsOneInFlight(t *testing.T) {
	var cur, peak, hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		hits.Add(1)
		time.Sleep(2 * time.Millisecond)
		cur.Add(-1)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	m := newMgr(t, Config{})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			method := "GET"
			if i%5 == 0 {
				method = "POST"
			}
			req, _ := http.NewRequest(method, srv.URL+"/api/x", strings.NewReader("{}"))
			resp, err := ep.Do(req)
			if err != nil {
				t.Error(err)
				return
			}
			resp.Body.Close()
		}(i)
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatalf("peak in-flight = %d, want 1", peak.Load())
	}
	if hits.Load() != 25 {
		t.Fatalf("hits %d", hits.Load())
	}
	st, _ := m.HostStats(hostOf(srv))
	if st.Requests != 25 || st.Classes[ClassOK] != 25 || len(st.Lanes) != 1 || st.Lanes[0].Limit != 1 || st.Latency.Count != 25 {
		t.Fatalf("stats %+v", st)
	}
}

func TestRedfishLaneAllowsFour(t *testing.T) {
	var cur, peak atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		cur.Add(-1)
	}))
	defer srv.Close()
	m := newMgr(t, Config{})
	ep := m.Endpoint(hostOf(srv), LaneRedfish)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if resp, err := get(t, ep, srv.URL+"/redfish/v1"); err == nil {
				resp.Body.Close()
			}
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for ep.Limiter().InFlight() < 4 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if q := ep.Limiter().Waiting(); q != 6 {
		t.Errorf("queued %d want 6", q)
	}
	close(release)
	wg.Wait()
	if peak.Load() != 4 {
		t.Fatalf("peak %d want 4", peak.Load())
	}
}

func TestLimiterWritePriorityFIFO(t *testing.T) {
	l := NewLimiter(1)
	ctx := context.Background()
	if err := l.Acquire(ctx, PriorityRead); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var order []string
	var wg sync.WaitGroup
	// enqueue deterministically: wait until the queue depth grows each time
	for i, x := range []struct {
		n string
		p Priority
	}{{"r1", PriorityRead}, {"r2", PriorityRead}, {"w1", PriorityWrite}, {"w2", PriorityWrite}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := l.Acquire(ctx, x.p); err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			order = append(order, x.n)
			mu.Unlock()
			l.Release()
		}()
		for l.Waiting() != i+1 {
			time.Sleep(time.Millisecond)
		}
	}
	l.Release()
	wg.Wait()
	if got := strings.Join(order, ","); got != "w1,w2,r1,r2" {
		t.Fatalf("order %s", got)
	}
}

func TestLimiterCancelRemovesWaiter(t *testing.T) {
	l := NewLimiter(1)
	_ = l.Acquire(context.Background(), PriorityRead)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := l.Acquire(ctx, PriorityRead); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v", err)
	}
	if l.Waiting() != 0 {
		t.Fatal("waiter not removed")
	}
	l.Release()
	if !l.TryAcquire() {
		t.Fatal("slot leaked")
	}
}

// ---- breaker ----

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestBreakerTripHalfOpen(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	var changes []string
	b := NewBreaker(BreakerConfig{FailureThreshold: 3, OpenTimeout: 10 * time.Second}, clk.Now, func(f, to BreakerState) {
		changes = append(changes, f.String()+">"+to.String())
	})
	for i := 0; i < 3; i++ {
		done, err := b.Allow()
		if err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		done(OutcomeFailure)
	}
	if b.State() != BreakerOpen {
		t.Fatalf("state %v", b.State())
	}
	if _, err := b.Allow(); !errors.Is(err, ErrBreakerOpen) {
		t.Fatal("should fail fast while open")
	}
	clk.Add(11 * time.Second)
	probe, err := b.Allow()
	if err != nil || b.State() != BreakerHalfOpen {
		t.Fatalf("half-open probe: %v %v", err, b.State())
	}
	if _, err := b.Allow(); err == nil {
		t.Fatal("only one probe allowed in half-open")
	}
	probe(OutcomeFailure)
	if b.State() != BreakerOpen {
		t.Fatal("failed probe should re-open")
	}
	clk.Add(11 * time.Second)
	probe, _ = b.Allow()
	probe(OutcomeSuccess)
	if b.State() != BreakerClosed {
		t.Fatal("successful probe should close")
	}
	want := "closed>open,open>half-open,half-open>open,open>half-open,half-open>closed"
	if strings.Join(changes, ",") != want {
		t.Fatalf("changes %v", changes)
	}
	if s := b.Snapshot(); s.Trips != 2 || s.ConsecutiveFailures != 0 {
		t.Fatalf("snapshot %+v", s)
	}
	// success resets the consecutive count
	for _, o := range []Outcome{OutcomeFailure, OutcomeFailure, OutcomeSuccess, OutcomeFailure, OutcomeFailure} {
		d, _ := b.Allow()
		d(o)
	}
	if b.State() != BreakerClosed {
		t.Fatal("non-consecutive failures should not trip")
	}
}

func TestEndpointBreakerFailsFastAndRecovers(t *testing.T) {
	var hits atomic.Int32
	var healthy atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if !healthy.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	clk := &fakeClock{t: time.Unix(0, 0)}
	var transitions atomic.Int32
	m := newMgr(t, Config{Now: clk.Now, Breaker: BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Minute},
		Hooks: Hooks{OnBreakerChange: func(string, BreakerState, BreakerState) { transitions.Add(1) }}})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	for i := 0; i < 2; i++ {
		resp, err := get(t, ep, srv.URL+"/api/x")
		if err != nil || resp.StatusCode != 502 {
			t.Fatalf("expected 502 response, got %v %v", resp, err)
		}
		resp.Body.Close()
	}
	if m.BreakerState(hostOf(srv)) != BreakerOpen {
		t.Fatal("breaker should be open")
	}
	_, err := get(t, ep, srv.URL+"/api/x")
	if !errors.Is(err, bmc.ErrBusy) || bmc.HTTPStatus(err) != 503 {
		t.Fatalf("expected fail-fast busy, got %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("server hit while open: %d", hits.Load())
	}
	// another lane on the same host shares the breaker
	if _, err := get(t, m.Endpoint(hostOf(srv), LaneRedfish), srv.URL+"/redfish/v1"); !errors.Is(err, bmc.ErrBusy) {
		t.Fatal("breaker must be per host")
	}
	healthy.Store(true)
	clk.Add(2 * time.Minute)
	resp, err := get(t, ep, srv.URL+"/api/x")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("half-open probe: %v %v", resp, err)
	}
	if m.BreakerState(hostOf(srv)) != BreakerClosed || transitions.Load() != 3 {
		t.Fatalf("state %v transitions %d", m.BreakerState(hostOf(srv)), transitions.Load())
	}
	st, _ := m.HostStats(hostOf(srv))
	if st.Classes[ClassBreakerOpen] != 2 || st.Breaker.Trips != 1 {
		t.Fatalf("stats %+v", st)
	}
}

// ---- retry ----

func flakyServer(t *testing.T, failures int, fail func(w http.ResponseWriter)) (*httptest.Server, *atomic.Int32) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		io.Copy(io.Discard, r.Body)
		if int(n) <= failures {
			fail(w)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func status503(w http.ResponseWriter) { w.WriteHeader(503) }
func evasive403(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(403)
	w.Write([]byte("<html><body><h1>403 Forbidden</h1></body></html>"))
}

func TestRetryOn503AndEvasive403(t *testing.T) {
	for name, fail := range map[string]func(http.ResponseWriter){"503": status503, "evasive403": evasive403} {
		t.Run(name, func(t *testing.T) {
			srv, hits := flakyServer(t, 2, fail)
			var attempts []AttemptInfo
			var mu sync.Mutex
			m := newMgr(t, Config{Hooks: Hooks{OnAttempt: func(a AttemptInfo) { mu.Lock(); attempts = append(attempts, a); mu.Unlock() }}})
			resp, err := get(t, m.Endpoint(hostOf(srv), LaneRedfish), srv.URL+"/redfish/v1/Systems")
			if err != nil || resp.StatusCode != 200 {
				t.Fatalf("%v %v", resp, err)
			}
			b, _ := io.ReadAll(resp.Body)
			if string(b) != `{"ok":true}` {
				t.Fatalf("body %q", b)
			}
			if hits.Load() != 3 || len(attempts) != 3 || !attempts[0].WillRetry || attempts[0].Class != ClassBusy || attempts[2].WillRetry {
				t.Fatalf("hits %d attempts %+v", hits.Load(), attempts)
			}
			st, _ := m.HostStats(hostOf(srv))
			if st.Retries != 2 || st.Attempts != 3 || st.Requests != 1 {
				t.Fatalf("stats %+v", st)
			}
		})
	}
}

func TestJSON403NotRetried(t *testing.T) {
	srv, hits := flakyServer(t, 5, func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(403)
		w.Write([]byte(`{"error":{"code":"Base.1.0.InsufficientPrivilege"}}`))
	})
	m := newMgr(t, Config{})
	resp, err := get(t, m.Endpoint(hostOf(srv), LaneRedfish), srv.URL+"/redfish/v1/x")
	if err != nil || resp.StatusCode != 403 || hits.Load() != 1 {
		t.Fatalf("resp %v err %v hits %d", resp, err, hits.Load())
	}
}

func TestExhausted503BecomesErrBusy(t *testing.T) {
	srv, hits := flakyServer(t, 100, status503)
	m := newMgr(t, Config{Breaker: BreakerConfig{Disabled: true}})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	_, err := get(t, ep, srv.URL+"/api/x")
	oe, ok := bmc.AsOpError(err)
	if !errors.Is(err, bmc.ErrBusy) || bmc.HTTPStatus(err) != 503 || !ok || oe.UpstreamStatus != 503 || hits.Load() != 3 {
		t.Fatalf("err %v hits %d", err, hits.Load())
	}
	// RawStatus returns the response instead
	req, _ := http.NewRequestWithContext(WithOptions(context.Background(), RequestOptions{RawStatus: true, NoRetry: true}), "GET", srv.URL+"/api/x", nil)
	resp, err := ep.Do(req)
	if err != nil || resp.StatusCode != 503 {
		t.Fatalf("raw: %v %v", resp, err)
	}
	// via http.Client the sentinel survives url.Error wrapping
	_, err = ep.HTTPClient().Get(srv.URL + "/api/x")
	if !errors.Is(err, bmc.ErrBusy) {
		t.Fatalf("client err %v", err)
	}
}

func TestWriteNotRetriedByDefault(t *testing.T) {
	srv, hits := flakyServer(t, 2, status503)
	m := newMgr(t, Config{})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	req, _ := http.NewRequest("POST", srv.URL+"/api/actions/power", strings.NewReader(`{"power_command":1}`))
	_, err := ep.Do(req)
	if !errors.Is(err, bmc.ErrBusy) || hits.Load() != 1 {
		t.Fatalf("write retried? err %v hits %d", err, hits.Load())
	}
	// PUT/DELETE also count as writes
	hits.Store(0)
	req, _ = http.NewRequest("DELETE", srv.URL+"/api/session", nil)
	_, _ = ep.Do(req)
	if hits.Load() != 1 {
		t.Fatalf("DELETE retried: %d", hits.Load())
	}
}

func TestIdempotentWriteRetried(t *testing.T) {
	srv, hits := flakyServer(t, 2, status503)
	m := newMgr(t, Config{})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	var bodies []string
	var mu sync.Mutex
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		if hits.Add(1) <= 2 {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte("ok"))
	})
	req, _ := http.NewRequestWithContext(WithIdempotent(context.Background()), "PUT", srv.URL+"/api/settings/date-time", strings.NewReader(`{"timestamp":1}`))
	resp, err := ep.Do(req)
	if err != nil || resp.StatusCode != 200 || hits.Load() != 3 {
		t.Fatalf("resp %v err %v hits %d", resp, err, hits.Load())
	}
	for _, b := range bodies {
		if b != `{"timestamp":1}` {
			t.Fatalf("body not replayed: %q", bodies)
		}
	}
}

func resetServer(t *testing.T, resets int) (*httptest.Server, *atomic.Int32) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		if int(hits.Add(1)) <= resets {
			c, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				if tc, ok := c.(interface{ SetLinger(int) error }); ok {
					tc.SetLinger(0) // RST
				}
				c.Close()
			}
			return
		}
		w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestRetryOnConnectionReset(t *testing.T) {
	srv, hits := resetServer(t, 1)
	m := newMgr(t, Config{})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	resp, err := get(t, ep, srv.URL+"/api/x")
	if err != nil || resp.StatusCode != 200 || hits.Load() != 2 {
		t.Fatalf("resp %v err %v hits %d", resp, err, hits.Load())
	}
	// a write whose request reached the server is not retried after a reset
	srv2, hits2 := resetServer(t, 1)
	ep2 := m.Endpoint(hostOf(srv2), LaneAMI)
	req, _ := http.NewRequest("POST", srv2.URL+"/api/x", strings.NewReader("{}"))
	_, err = ep2.Do(req)
	if !errors.Is(err, bmc.ErrUpstream) || bmc.HTTPStatus(err) != 502 || hits2.Load() != 1 {
		t.Fatalf("err %v hits %d", err, hits2.Load())
	}
}

func TestWriteRetriedWhenNeverSent(t *testing.T) {
	// nothing listening: dial fails before the request is written
	srv := httptest.NewServer(http.NotFoundHandler())
	url, host := srv.URL, hostOf(srv)
	srv.Close()
	var attempts atomic.Int32
	m := newMgr(t, Config{Breaker: BreakerConfig{Disabled: true}, Hooks: Hooks{OnAttempt: func(AttemptInfo) { attempts.Add(1) }}})
	req, _ := http.NewRequest("POST", url+"/api/x", strings.NewReader("{}"))
	_, err := m.Endpoint(host, LaneAMI).Do(req)
	if !errors.Is(err, bmc.ErrUpstream) || attempts.Load() != 3 {
		t.Fatalf("err %v attempts %d", err, attempts.Load())
	}
}

func TestAttemptTimeoutAndCallerDeadline(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		select {
		case <-time.After(500 * time.Millisecond):
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	m := newMgr(t, Config{AttemptTimeout: 20 * time.Millisecond, Retry: RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond}})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	_, err := get(t, ep, srv.URL+"/slow")
	if !errors.Is(err, bmc.ErrTimeout) || bmc.HTTPStatus(err) != 504 || hits.Load() != 2 {
		t.Fatalf("err %v hits %d", err, hits.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/slow", nil)
	_, err = ep.Do(req)
	if !errors.Is(err, context.Canceled) || bmc.HTTPStatus(err) != 499 {
		t.Fatalf("canceled: %v", err)
	}
}

// ---- exclusive / stream / headers ----

func TestExclusiveSequenceNotInterleaved(t *testing.T) {
	var mu sync.Mutex
	var seq []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, r.URL.Path)
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}))
	defer srv.Close()
	m := newMgr(t, Config{})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	started := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-started
		for i := 0; i < 3; i++ {
			if resp, err := get(t, ep, srv.URL+"/other"); err == nil {
				resp.Body.Close()
			}
		}
	}()
	err := ep.Exclusive(context.Background(), func(ctx context.Context) error {
		close(started)
		for _, p := range []string{"/select", "/read"} {
			time.Sleep(10 * time.Millisecond) // give the other goroutine a chance to interleave
			req, _ := http.NewRequestWithContext(ctx, "POST", srv.URL+p, nil)
			resp, err := ep.Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()
		}
		return nil
	})
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(seq[:2], ",") != "/select,/read" {
		t.Fatalf("interleaved: %v", seq)
	}
}

func TestStreamHoldsSlotUntilClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range []string{"X-HTTP-Method-Override", "X-HTTP-Method", "X-Method-Override"} {
			if r.Header.Get(h) != "" {
				w.WriteHeader(400)
				return
			}
		}
		w.Write(bytes.Repeat([]byte("x"), 1000))
	}))
	defer srv.Close()
	m := newMgr(t, Config{})
	ep := m.Endpoint(hostOf(srv), LaneAMI)
	req, _ := http.NewRequestWithContext(WithOptions(context.Background(), RequestOptions{Stream: true}), "GET", srv.URL+"/big", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	req.Header.Set("X-HTTP-Method", "DELETE")
	req.Header.Set("X-Method-Override", "DELETE")
	resp, err := ep.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("%v %v (override header must be stripped)", resp, err)
	}
	if ep.Limiter().InFlight() != 1 {
		t.Fatal("slot should be held while streaming")
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(b) != 1000 || ep.Limiter().InFlight() != 0 {
		t.Fatalf("len %d inflight %d", len(b), ep.Limiter().InFlight())
	}
}

// ---- TLS profiles ----

func legacyOnlyServer(t *testing.T) *httptest.Server {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("legacy-ok")) }))
	srv.TLS = &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS12,
		CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_128_CBC_SHA}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func TestTLSProfiles(t *testing.T) {
	srv := legacyOnlyServer(t)
	host := hostOf(srv)

	m := newMgr(t, Config{DefaultProfile: ProfileModern, Retry: RetryPolicy{MaxAttempts: 1}})
	if _, err := get(t, m.Endpoint(host, LaneAMI), srv.URL+"/"); err == nil {
		t.Fatal("modern profile should not negotiate RSA key exchange")
	}

	m = newMgr(t, Config{})
	m.SetProfile(host, ProfileLegacy)
	resp, err := get(t, m.Endpoint(host, LaneAMI), srv.URL+"/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("legacy: %v %v", resp, err)
	}
	st, _ := m.HostStats(host)
	if st.TLS == nil || st.TLS.CipherSuite != "TLS_RSA_WITH_AES_128_CBC_SHA" || st.TLS.Profile != ProfileLegacy {
		t.Fatalf("tls info %+v", st.TLS)
	}

	m = newMgr(t, Config{}) // ProfileAuto
	resp, err = get(t, m.Endpoint(host, LaneAMI), srv.URL+"/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("auto: %v %v", resp, err)
	}
	if lp, ok := m.dialer.learnedProfile(host); !ok || lp != ProfileLegacy {
		t.Fatalf("auto should learn legacy, got %v %v", lp, ok)
	}

	// modern server works with auto/modern
	msrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer msrv.Close()
	resp, err = get(t, m.Endpoint(hostOf(msrv), LaneRedfish), msrv.URL+"/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("modern: %v %v", resp, err)
	}
	st, _ = m.HostStats(hostOf(msrv))
	if st.TLS == nil || st.TLS.Version != "TLS 1.3" || st.TLS.Profile != ProfileModern {
		t.Fatalf("modern tls %+v", st.TLS)
	}
	cfg := TLSConfig(ProfileLegacy)
	if !cfg.InsecureSkipVerify || cfg.MinVersion != tls.VersionTLS10 || !TLSConfig(ProfileModern).InsecureSkipVerify {
		t.Fatal("tls config")
	}
}

// ---- instrumentation ----

func TestTraceAndHARRedaction(t *testing.T) {
	const pass = "Hunter2Password"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: "sessSECRETvalue", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":0,"CSRFToken":"csrfSECRETvalue"}`))
	}))
	defer srv.Close()
	for _, logSecrets := range []bool{false, true} {
		var buf bytes.Buffer
		dir := t.TempDir()
		o, err := obs.New(obs.Options{Writer: &buf, TraceHTTP: true, HAROut: dir, LogSecrets: logSecrets})
		if err != nil {
			t.Fatal(err)
		}
		m := newMgr(t, Config{Obs: o})
		ep := m.Endpoint(hostOf(srv), LaneAMI)
		ctx := obs.WithRequestID(context.Background(), "req-42")
		req, _ := http.NewRequestWithContext(WithOp(ctx, "amimegarac.Login"), "POST", srv.URL+"/api/session",
			strings.NewReader("username=admin&password="+pass))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("admin", pass)
		resp, err := ep.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		out := buf.String()
		secrets := []string{pass, "sessSECRETvalue", "csrfSECRETvalue", "YWRtaW46SHVudGVyMlBhc3N3b3Jk"}
		if !strings.Contains(out, "southbound dump") || !strings.Contains(out, "req_id=req-42") || !strings.Contains(out, "corr_id=") {
			t.Fatalf("missing trace/ids:\n%s", out)
		}
		raw, err := os.ReadFile(filepath.Join(dir, strings.ReplaceAll(hostOf(srv), ":", "_")+".har"))
		if err != nil {
			t.Fatal(err)
		}
		var h obs.HAR
		if err := json.Unmarshal(raw, &h); err != nil || h.Log.Version != "1.2" || len(h.Log.Entries) != 1 {
			t.Fatalf("har: %v %+v", err, h.Log)
		}
		e := h.Log.Entries[0]
		if e.Response.Status != 200 || e.Request.Method != "POST" || !strings.Contains(e.Comment, "op=amimegarac.Login") || e.Time <= 0 {
			t.Fatalf("entry %+v", e)
		}
		for _, s := range secrets {
			inLog, inHAR := strings.Contains(out, s), strings.Contains(string(raw), s)
			if !logSecrets && (inLog || inHAR) {
				t.Errorf("secret %q leaked (log=%v har=%v)", s, inLog, inHAR)
			}
			if logSecrets && (!inLog || !inHAR) {
				t.Errorf("secret %q missing with LogSecrets (log=%v har=%v)", s, inLog, inHAR)
			}
		}
	}
}

func TestDebugLogShowsRetries(t *testing.T) {
	srv, _ := flakyServer(t, 1, status503)
	var buf bytes.Buffer
	o := obs.NewForHandler(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: obs.LevelTrace}), slog.LevelInfo, false)
	o.Levels().Set(obs.SubTransport, slog.LevelDebug)
	m := newMgr(t, Config{Obs: o})
	resp, err := get(t, m.Endpoint(hostOf(srv), LaneAMI), srv.URL+"/api/x")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	out := buf.String()
	if !strings.Contains(out, "retrying southbound request") || !strings.Contains(out, "attempt=1/3") || strings.Contains(out, "southbound dump") {
		t.Fatalf("debug output:\n%s", out)
	}
}

func TestBackoffBounds(t *testing.T) {
	p := RetryPolicy{BaseDelay: 100 * time.Millisecond, MaxDelay: time.Second}
	for n, want := range map[int]time.Duration{1: 100 * time.Millisecond, 2: 200 * time.Millisecond, 3: 400 * time.Millisecond, 10: time.Second} {
		for i := 0; i < 50; i++ {
			d := p.Backoff(n)
			if d < want/2 || d > want {
				t.Fatalf("Backoff(%d)=%v outside [%v,%v]", n, d, want/2, want)
			}
		}
	}
}

func TestEvasiveClassifier(t *testing.T) {
	mk := func(ct string) *http.Response {
		return &http.Response{StatusCode: 403, Header: http.Header{"Content-Type": {ct}}}
	}
	if !DefaultIsEvasive403(mk("text/html"), []byte("<html>403</html>")) || !DefaultIsEvasive403(mk(""), nil) {
		t.Error("html/empty 403 should be evasive")
	}
	if DefaultIsEvasive403(mk("application/json"), []byte(`{"error":{}}`)) {
		t.Error("json 403 is a real privilege error")
	}
	if CanonicalHost("10.0.0.1") != "10.0.0.1:443" || CanonicalHost("h:8443") != "h:8443" || CanonicalHost("fe80::1") != "[fe80::1]:443" {
		t.Error("canonical host")
	}
}
