package amimegarac

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

type chassisStatus struct {
	PowerStatus        NullableInt `json:"power_status"`
	LEDStatus          NullableInt `json:"led_status"`
	PowerCycleInterval NullableInt `json:"power_cycle_interval"`
}

func TestLoginCSRFAndSessionInfo(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	ctx := context.Background()

	var cs chassisStatus
	if err := c.Get(ctx, "/api/chassis-status", &cs); err != nil {
		t.Fatal(err)
	}
	if cs.PowerStatus.V != 1 || !cs.PowerCycleInterval.Valid || cs.PowerCycleInterval.V != 2000 {
		t.Fatalf("decoded %+v", cs)
	}
	reqs := f.seen()
	if len(reqs) != 2 {
		t.Fatalf("want login + GET, got %d requests", len(reqs))
	}
	login := reqs[0]
	if login.Method != "POST" || login.Path != "/api/session" {
		t.Fatalf("first request %s %s", login.Method, login.Path)
	}
	if !strings.Contains(login.Body, "username=admin") || !strings.Contains(login.Body, "password=s3cret") {
		t.Fatalf("login form %q", login.Body)
	}
	if login.Header.Get("X-Requested-With") != "XMLHttpRequest" {
		t.Error("login missing X-Requested-With")
	}
	get := reqs[1]
	if get.Header.Get("X-CSRFTOKEN") == "" || len(get.Header.Get("X-CSRFTOKEN")) != 8 {
		t.Errorf("GET missing CSRF header: %q", get.Header.Get("X-CSRFTOKEN"))
	}
	if !strings.HasPrefix(get.Header.Get("Cookie"), "QSESSIONID=") {
		t.Errorf("GET missing cookie: %q", get.Header.Get("Cookie"))
	}

	info, ok := c.Session()
	if !ok {
		t.Fatal("no session")
	}
	if info.Privilege != 4 || info.PrivilegeName != bmc.PrivAdministrator || info.ExtendedPriv != 259 ||
		info.UserID != 2 || info.Channel != 1 || !info.HTTPSEnabled || info.Generation != 1 {
		t.Errorf("session info %+v", info)
	}
	if _, leaked := info.Raw["CSRFToken"]; leaked {
		t.Error("Raw leaks CSRFToken")
	}
	if info.BMCDate.IsZero() || info.ClockOffset > -3*time.Second || info.ClockOffset < -8*time.Second {
		t.Errorf("clock offset from Date header: %v (date %v)", info.ClockOffset, info.BMCDate)
	}

	// Writes carry the token too, and the session is reused.
	var echo map[string]int
	if err := c.Post(ctx, "actions/chassis-led", map[string]int{"uid_command": 2}, &echo); err != nil {
		t.Fatal(err)
	}
	if echo["uid_command"] != 2 {
		t.Errorf("echo %v", echo)
	}
	if n := f.logins.Load(); n != 1 {
		t.Errorf("logins = %d, want 1", n)
	}
	last := f.seen()[2]
	if last.Header.Get("Content-Type") != "application/json" || last.Header.Get("X-CSRFTOKEN") == "" {
		t.Errorf("write headers %v", last.Header)
	}
}

func TestLoginExtraCertlogin(t *testing.T) {
	f := newFakeBMC(t)
	f.certlogin = true
	c := newClient(t, f, func(cfg *Config) { cfg.LoginExtra = X470LoginExtra() })
	if err := c.Login(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.seen()[0].Body, "certlogin=0") {
		t.Error("certlogin not sent")
	}
}

func TestLogout(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	ctx := context.Background()
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if f.logouts.Load() != 1 {
		t.Fatal("DELETE /api/session not received")
	}
	last := f.seen()[len(f.seen())-1]
	if last.Method != "DELETE" || last.Header.Get("X-CSRFTOKEN") == "" || last.Header.Get("Cookie") == "" {
		t.Errorf("logout request %+v", last)
	}
	if _, ok := c.Session(); ok {
		t.Error("session still present")
	}
	// Idempotent: a second logout does nothing.
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if f.logouts.Load() != 1 {
		t.Error("second logout hit the BMC")
	}
	if c.Stats().Logouts != 1 {
		t.Errorf("stats %+v", c.Stats())
	}
}

func TestCC7SingleReloginThenSuccess(t *testing.T) {
	f := newFakeBMC(t)
	var events []ReloginEvent
	var mu sync.Mutex
	c := newClient(t, f, func(cfg *Config) {
		cfg.Hooks.OnRelogin = func(e ReloginEvent) { mu.Lock(); events = append(events, e); mu.Unlock() }
	})
	ctx := context.Background()
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	f.killAll() // backend restart

	var sensors []map[string]any
	if err := c.Get(ctx, "sensors", &sensors); err != nil {
		t.Fatal(err)
	}
	if len(sensors) != 2 {
		t.Fatalf("sensors %d", len(sensors))
	}
	if n := f.logins.Load(); n != 2 {
		t.Errorf("logins = %d, want 2", n)
	}
	st := c.Stats()
	if st.Relogins != 1 || st.Logins != 2 || st.Generation != 2 || !st.Degraded || !c.Degraded() {
		t.Errorf("stats %+v", st)
	}
	if len(events) != 1 || events[0].Err != nil || !strings.Contains(events[0].Reason, "cc:7") {
		t.Errorf("relogin events %+v", events)
	}
}

func TestReloginFailsReturnsErrAuth(t *testing.T) {
	f := newFakeBMC(t)
	var hookErr error
	c := newClient(t, f, func(cfg *Config) {
		cfg.Hooks.OnRelogin = func(e ReloginEvent) { hookErr = e.Err }
	})
	ctx := context.Background()
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	f.killAll()
	f.setRejectLogin(true)

	err := c.Get(ctx, "/api/chassis-status", nil)
	if !errors.Is(err, bmc.ErrAuth) || bmc.HTTPStatus(err) != 401 {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	var oe *bmc.OpError
	if !errors.As(err, &oe) || oe.Pkg != Pkg || oe.Backend != bmc.BackendAMI {
		t.Errorf("not an amimegarac OpError: %#v", err)
	}
	if hookErr == nil {
		t.Error("OnRelogin did not report the failure")
	}
	if n := f.logins.Load(); n != 2 {
		t.Errorf("logins = %d, want 2 (one initial, one failed re-login)", n)
	}
	// Failure backoff: an immediate retry does not hit /api/session again.
	if err := c.Get(ctx, "/api/chassis-status", nil); !errors.Is(err, bmc.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	if n := f.logins.Load(); n != 2 {
		t.Errorf("login during backoff: logins = %d", n)
	}
}

func TestInitialLoginFailureIsErrAuth(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, func(cfg *Config) { cfg.Creds.Pass = "wrong" })
	err := c.Login(context.Background())
	if !errors.Is(err, bmc.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
	if strings.Contains(err.Error(), "wrong") {
		t.Error("error leaks the password")
	}
}

func TestConcurrentRequestsTriggerOneRelogin(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	ctx := context.Background()
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	f.killAll()
	f.loginDelay = 50 * time.Millisecond

	const n = 16
	var wg sync.WaitGroup
	var failures atomic.Int32
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Get(ctx, "/api/chassis-status", nil); err != nil {
				failures.Add(1)
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if failures.Load() != 0 {
		t.Fatalf("%d failures", failures.Load())
	}
	if got := f.logins.Load(); got != 2 {
		t.Errorf("logins = %d, want 2 (initial + exactly one re-login)", got)
	}
	if st := c.Stats(); st.Relogins != 1 {
		t.Errorf("relogins = %d", st.Relogins)
	}
}

func TestConcurrentInitialLoginSingleflight(t *testing.T) {
	f := newFakeBMC(t)
	f.loginDelay = 30 * time.Millisecond
	c := newClient(t, f, nil)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Get(context.Background(), "chassis-status", nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := f.logins.Load(); got != 1 {
		t.Errorf("logins = %d, want 1", got)
	}
	if c.Stats().Relogins != 0 {
		t.Error("initial login counted as a re-login")
	}
}

func TestWriteReplayRule(t *testing.T) {
	ctx := context.Background()

	t.Run("cc7 write is replayed", func(t *testing.T) {
		f := newFakeBMC(t)
		c := newClient(t, f, nil)
		if err := c.Login(ctx); err != nil {
			t.Fatal(err)
		}
		f.killAll()
		if err := c.Post(ctx, "/api/actions/chassis-led", map[string]int{"uid_command": 0}, nil); err != nil {
			t.Fatal(err)
		}
		if f.writes["/api/actions/chassis-led"] != 1 {
			t.Errorf("writes = %d", f.writes["/api/actions/chassis-led"])
		}
	})

	t.Run("non-cc7 401 write is not replayed", func(t *testing.T) {
		f := newFakeBMC(t)
		c := newClient(t, f, nil)
		if err := c.Login(ctx); err != nil {
			t.Fatal(err)
		}
		var calls atomic.Int32
		f.handle("POST /api/actions/power", func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(401)
				_, _ = w.Write([]byte("<html>401</html>"))
				return
			}
			writeJSON(w, 200, `{"power_command": 1}`)
		})
		err := c.Post(ctx, "/api/actions/power", map[string]int{"power_command": 1}, nil)
		if !errors.Is(err, bmc.ErrAuth) || !strings.Contains(err.Error(), "not replayed") {
			t.Fatalf("want ErrAuth not-replayed, got %v", err)
		}
		if calls.Load() != 1 {
			t.Errorf("write sent %d times", calls.Load())
		}
		if f.logins.Load() != 2 {
			t.Errorf("session not renewed: logins=%d", f.logins.Load())
		}
	})

	t.Run("idempotent write is replayed on any 401", func(t *testing.T) {
		f := newFakeBMC(t)
		c := newClient(t, f, nil)
		if err := c.Login(ctx); err != nil {
			t.Fatal(err)
		}
		var calls atomic.Int32
		f.handle("PUT /api/settings/network-bond", func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(401)
				return
			}
			writeJSON(w, 200, `{"id": 1, "bond_enable": 0, "bond_mode": "active-backup", "auto_configuration_enable": 1, "bond_ifc": "eth0"}`)
		})
		err := c.Put(transport.WithIdempotent(ctx), "settings/network-bond",
			map[string]any{"id": 1, "bond_enable": 0}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 2 {
			t.Errorf("calls = %d", calls.Load())
		}
	})

	t.Run("401 after relogin is ErrAuth, no loop", func(t *testing.T) {
		f := newFakeBMC(t)
		c := newClient(t, f, nil)
		f.handle("GET /api/settings/users", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 401, ccInvalidAuth)
		})
		err := c.Get(ctx, "settings/users", nil)
		if !errors.Is(err, bmc.ErrAuth) {
			t.Fatalf("got %v", err)
		}
		if f.logins.Load() != 2 {
			t.Errorf("logins = %d, want 2", f.logins.Load())
		}
	})
}

func TestDenylistBlocksWithoutContactingBMC(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, func(cfg *Config) {
		cfg.ExtraDeny = []DenyRule{{Pattern: "asrr/bmccoldreset", Reason: "board profile"}}
	})
	ctx := context.Background()
	paths := []struct{ method, path string }{
		{"GET", "/api/logs/help-data"},
		{"GET", "logs/help-data?file=x"},
		{"GET", "/api/../api/logs//help-data"},
		{"GET", "/api/%6cogs/help-data"},
		{"GET", "/API/LOGS/HELP-DATA"},
		{"GET", "/api/logs/help-data;foo"},
		{"GET", "/api/asrr/mca-file"},
		{"GET", "/api/settings/download_image"},
		{"GET", "/api/maintenance/hpm/upgradestatus"},
		{"GET", "/api/maintenance/hpm/verifyimagestatus"},
		{"DELETE", "/redfish/v1/GlobalLuaTables"},
		{"GET", "/api/x/globalluatables/y"},
		{"POST", "/api/asrr/bmccoldreset"},
	}
	for _, p := range paths {
		_, err := c.Do(ctx, &Request{Method: p.method, Path: p.path})
		if !errors.Is(err, bmc.ErrForbidden) || bmc.HTTPStatus(err) != 403 {
			t.Errorf("%s %s: want ErrForbidden, got %v", p.method, p.path, err)
		}
	}
	if err := c.Get(ctx, "logs/help-data", nil); !errors.Is(err, bmc.ErrForbidden) {
		t.Errorf("Get helper: %v", err)
	}
	if n := f.hits.Load(); n != 0 {
		t.Errorf("BMC was contacted %d times", n)
	}
	if got := c.Stats().DenylistBlocks; got != uint64(len(paths)+1) {
		t.Errorf("DenylistBlocks = %d", got)
	}
	// Board profiles can add rules at runtime too, but not remove them.
	c.Denylist().Add(DenyRule{Pattern: "chassis-status"})
	if err := c.Get(ctx, "chassis-status", nil); !errors.Is(err, bmc.ErrForbidden) {
		t.Errorf("runtime rule: %v", err)
	}
}

func TestInvalidPathsRejected(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	for _, p := range []string{"https://evil/api/x", "/api/a%2", "/api/a%252e", "/api/a\\b", "/api", "/api/x\r\nHost: y", ""} {
		if _, err := c.Do(context.Background(), &Request{Path: p}); !errors.Is(err, bmc.ErrInvalidArgument) {
			t.Errorf("%q: want ErrInvalidArgument, got %v", p, err)
		}
	}
	if _, err := c.Do(context.Background(), &Request{Method: "PROPFIND", Path: "x"}); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Errorf("custom method: %v", err)
	}
	if f.hits.Load() != 0 {
		t.Error("BMC contacted")
	}
}

func TestCanonicalPathStaysUnderAPI(t *testing.T) {
	cases := map[string]string{
		"chassis":                "/api/chassis",
		"/api/chassis":           "/api/chassis",
		"api/chassis":            "/api/chassis",
		"../../etc/passwd":       "/api/etc/passwd",
		"/api/../redfish/v1":     "/api/redfish/v1",
		"/api/settings//network": "/api/settings/network",
	}
	for in, want := range cases {
		got, _, err := CanonicalPath(in)
		if err != nil || got != want {
			t.Errorf("CanonicalPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	_, q, _ := CanonicalPath("/api/logs/event?LASTEVENTID=560")
	if q != "LASTEVENTID=560" {
		t.Errorf("query %q", q)
	}
}

func TestMethodOverrideNeverSent(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	h := http.Header{}
	h.Set("X-HTTP-Method-Override", "DELETE")
	h.Set("X-HTTP-Method", "DELETE")
	h.Set("X-Method-Override", "DELETE")
	h.Set("Cookie", "QSESSIONID=attacker")
	h.Set("X-CSRFTOKEN", "attacker")
	resp, err := c.Do(context.Background(), &Request{Method: "GET", Path: "chassis-status", Header: h})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 {
		t.Fatalf("status %d", resp.Status)
	}
	for _, r := range f.seen() {
		for _, k := range []string{"X-Http-Method-Override", "X-Http-Method", "X-Method-Override"} {
			if r.Header.Get(k) != "" {
				t.Errorf("%s %s carried %s", r.Method, r.Path, k)
			}
		}
		if strings.Contains(r.Header.Get("Cookie"), "attacker") || r.Header.Get("X-CSRFTOKEN") == "attacker" {
			t.Errorf("caller auth headers forwarded")
		}
	}
}

func TestBadGatewayInvalidatesHost(t *testing.T) {
	f := newFakeBMC(t)
	var mu sync.Mutex
	var hosts, reasons []string
	c := newClient(t, f, func(cfg *Config) {
		cfg.Hooks.OnHostInvalidated = func(h, r string) { mu.Lock(); hosts = append(hosts, h); reasons = append(reasons, r); mu.Unlock() }
	})
	f.handle("GET /api/asrr/fw-info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(502)
	})
	f.handle("GET /api/system_inventory/cpu_info", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 500, `{"error":"Error in Getting the Device Count Information","code":11272}`)
	})
	f.handle("GET /api/settings/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(500)
		_, _ = w.Write([]byte("<html><body>500 Internal Server Error</body></html>"))
	})
	ctx := context.Background()

	err := c.Get(ctx, "asrr/fw-info", nil)
	if !errors.Is(err, bmc.ErrUpstream) || bmc.HTTPStatus(err) != 502 {
		t.Fatalf("want ErrUpstream, got %v", err)
	}
	mu.Lock()
	if len(hosts) != 1 || hosts[0] != c.Host() || !strings.Contains(reasons[0], "502") {
		t.Errorf("hook calls %v %v", hosts, reasons)
	}
	mu.Unlock()
	if !c.Degraded() || c.Stats().HostInvalidations != 1 {
		t.Errorf("stats %+v", c.Stats())
	}

	// A JSON 500 is an ordinary upstream error: no host invalidation.
	err = c.Get(ctx, "system_inventory/cpu_info", nil)
	var ae *APIError
	if !errors.Is(err, bmc.ErrUpstream) || !errors.As(err, &ae) || ae.Code != 11272 {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(err.Error(), "11272") {
		t.Errorf("detail lacks code: %v", err)
	}
	if c.Stats().HostInvalidations != 1 {
		t.Error("JSON 500 invalidated the host")
	}
	// An HTML 500 is the crash page.
	_ = c.Get(ctx, "settings/users", nil)
	if c.Stats().HostInvalidations != 2 {
		t.Error("HTML 500 did not invalidate the host")
	}
}

func TestStatusMapping(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	f.handle("GET /api/settings/raid_management/x", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 405, `{"error":"Feature is not enabled","code":8500}`)
	})
	ctx := context.Background()
	if err := c.Get(ctx, "settings/fan/fan_mode", nil); !errors.Is(err, bmc.ErrUnsupported) {
		t.Errorf("404 1010: %v", err)
	}
	if err := c.Get(ctx, "settings/raid_management/x", nil); !errors.Is(err, bmc.ErrUnsupported) {
		t.Errorf("405 8500: %v", err)
	}
	// Raw Do returns non-2xx responses as-is (for passthrough).
	resp, err := c.Do(ctx, &Request{Path: "settings/fan/fan_mode"})
	if err != nil || resp.Status != 404 || !strings.Contains(string(resp.Body), "1010") {
		t.Errorf("raw: %v %+v", err, resp)
	}
}

func TestClock(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	s, err := c.Clock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.BMCTime.Unix() != 1794808815 || s.Config.Timezone != "GMT+0" || !s.Config.AutoDate.Valid {
		t.Errorf("sample %+v", s)
	}
	if s.Offset == 0 {
		t.Error("offset not computed")
	}
}

func TestQueryPassthrough(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil)
	var gotQuery string
	f.handle("GET /api/logs/event", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		writeJSON(w, 200, `[]`)
	})
	if err := c.Get(context.Background(), "logs/event?LASTEVENTID=560", nil); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "LASTEVENTID=560" {
		t.Errorf("query %q", gotQuery)
	}
}

func TestCanceledWaiterDoesNotCancelSharedLogin(t *testing.T) {
	f := newFakeBMC(t)
	f.loginDelay = 100 * time.Millisecond
	c := newClient(t, f, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := c.Login(ctx); err == nil {
		t.Fatal("expected cancellation")
	}
	// The detached login still finishes and is reused.
	time.Sleep(150 * time.Millisecond)
	if err := c.Get(context.Background(), "chassis-status", nil); err != nil {
		t.Fatal(err)
	}
	if n := f.logins.Load(); n != 1 {
		t.Errorf("logins = %d, want 1", n)
	}
}

func TestReportSessionLostIsSticky(t *testing.T) {
	f := newFakeBMC(t)
	var hooks int
	c := newClient(t, f, func(cfg *Config) {
		cfg.ReportSessionLost = true
		cfg.Hooks.OnHostInvalidated = func(string, string) { hooks++ }
	})
	ctx := context.Background()
	f.handle("GET /api/asrr/fw-info", func(w http.ResponseWriter, r *http.Request) {
		f.killAll()
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(502)
	})
	if err := c.Get(ctx, "sensors", nil); err != nil {
		t.Fatal(err)
	}
	// The request that saw the restart fails with ErrSessionLost (still a
	// 502 upstream error).
	err := c.Get(ctx, "asrr/fw-info", nil)
	if !errors.Is(err, bmc.ErrSessionLost) || !errors.Is(err, bmc.ErrUpstream) || bmc.HTTPStatus(err) != 502 {
		t.Fatalf("crash: %v", err)
	}
	if lost, why := c.HostLost(); !lost || !strings.Contains(why, "502") || hooks != 1 {
		t.Fatalf("HostLost=%v %q hooks=%d", lost, why, hooks)
	}
	// Every later call, Clock included, fails fast without reaching the BMC
	// or silently logging in again.
	hits, logins := f.hits.Load(), f.logins.Load()
	if err := c.Get(ctx, "chassis-status", nil); !errors.Is(err, bmc.ErrSessionLost) {
		t.Fatalf("after crash: %v", err)
	}
	if _, err := c.Clock(ctx); !errors.Is(err, bmc.ErrSessionLost) {
		t.Fatalf("Clock after crash: %v", err)
	}
	if f.hits.Load() != hits || f.logins.Load() != logins {
		t.Fatalf("session-lost calls reached the BMC (%d -> %d hits)", hits, f.hits.Load())
	}
	// Logout has nothing to log out (the session died with the backend).
	if err := c.Logout(ctx); err != nil || f.hits.Load() != hits {
		t.Fatalf("logout after loss: %v", err)
	}
	// An explicit Login starts afresh and clears the state.
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	if lost, _ := c.HostLost(); lost {
		t.Fatal("still lost after a fresh login")
	}
	if err := c.Get(ctx, "chassis-status", nil); err != nil {
		t.Fatalf("after relogin: %v", err)
	}
}

func TestMarkHostLost(t *testing.T) {
	f := newFakeBMC(t)
	c := newClient(t, f, nil) // MarkHostLost works without ReportSessionLost
	ctx := context.Background()
	if err := c.Get(ctx, "sensors", nil); err != nil {
		t.Fatal(err)
	}
	c.MarkHostLost("BMC reset requested")
	if err := c.SessionLostError("x"); !errors.Is(err, bmc.ErrSessionLost) {
		t.Fatalf("SessionLostError: %v", err)
	}
	if err := c.Get(ctx, "sensors", nil); !errors.Is(err, bmc.ErrSessionLost) || !strings.Contains(err.Error(), "reset") {
		t.Fatalf("after mark: %v", err)
	}
	if !c.Degraded() {
		t.Fatal("not degraded")
	}
	if err := c.Login(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, "sensors", nil); err != nil {
		t.Fatal(err)
	}
}
