package testsim_test

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/testsim"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
)

// Integration tests: the full proxy (server, sessions, cache, /api/v1,
// health, metrics, transport) against the B650 sim, using the generic
// Redfish driver. ASRock driver tests plug in the same way by returning the
// asrock driver from ProxyOptions.Drivers (see doc.go).

func redfishDrivers(auth rf.AuthMode) func(tm *transport.Manager) []bmc.Driver {
	return func(tm *transport.Manager) []bmc.Driver {
		d := genericredfish.New(tm)
		d.Auth = auth
		return []bmc.Driver{d}
	}
}

func setup(t *testing.T, auth rf.AuthMode, popt func(*testsim.ProxyOptions), smod ...func(*testsim.Options)) (*testsim.Sim, *testsim.Proxy) {
	t.Helper()
	sim := testsim.StartB650(t, smod...)
	o := testsim.ProxyOptions{Drivers: redfishDrivers(auth),
		V1: v1.Config{PowerPollInterval: 5 * time.Millisecond, PowerPollTimeout: time.Second}}
	if popt != nil {
		popt(&o)
	}
	return sim, testsim.StartProxy(t, o)
}

func wantStatus(t *testing.T, r *testsim.Resp, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status %d want %d: %s", r.Status, want, r.Body)
	}
}

func TestB650SensorsPowerEvents(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, nil)

	var caps v1.CapabilitiesResponse
	r := p.Get(sim, "/api/v1/capabilities")
	wantStatus(t, r, 200)
	r.JSON(t, &caps)
	if caps.Driver != "redfish" || caps.Board != "B650D4U" || !strings.HasPrefix(caps.Firmware, "4.10") {
		t.Fatalf("capabilities: %+v", caps.CapabilitiesReport)
	}

	var sens v1.SensorsResponse
	r = p.Get(sim, "/api/v1/sensors")
	wantStatus(t, r, 200)
	r.JSON(t, &sens)
	if len(sens.Sensors) <= 50 {
		t.Fatalf("%d sensors: Redfish nextLink paging (50/page) not followed", len(sens.Sensors))
	}

	var ps v1.PowerStateResponse
	p.Get(sim, "/api/v1/power").JSON(t, &ps)
	if ps.State != bmc.PowerOn {
		t.Fatalf("power %v", ps.State)
	}
	c := sim.Creds()
	post := func(body string) *testsim.Resp {
		return p.Do(testsim.Req{Method: "POST", Path: "/api/v1/power", Host: sim.Host(), User: c.User, Pass: c.Pass, Body: body})
	}
	var res v1.PowerActionResult
	r = post(`{"action":"off"}`)
	wantStatus(t, r, 200)
	r.JSON(t, &res)
	if res.State != bmc.PowerOff || sim.PowerOn() {
		t.Fatalf("off: %+v", res)
	}
	wantStatus(t, post(`{"action":"on"}`), 200)

	// A graceful shutdown the host ignores: the proxy polls and reports
	// 202 + warning instead of assuming success.
	sim.SetPowerIgnoresSoftOff(true)
	r = post(`{"action":"shutdown","timeoutSeconds":0.2}`)
	wantStatus(t, r, 202)
	res = v1.PowerActionResult{}
	r.JSON(t, &res)
	if res.Warning == "" || res.State != bmc.PowerOn || sim.Stats().IgnoredSoftOffs != 1 {
		t.Fatalf("ignored soft-off: %+v", res)
	}

	// Events, then incremental since=lastId.
	var ev v1.EventsResponse
	r = p.Get(sim, "/api/v1/events")
	wantStatus(t, r, 200)
	r.JSON(t, &ev)
	if len(ev.Entries) == 0 || ev.LastID == "" {
		t.Fatalf("events: %+v", ev)
	}
	id := sim.Redfish().AddSELEntry("CPU Temp upper critical", "Critical")
	var inc v1.EventsResponse
	p.Get(sim, "/api/v1/events?since="+ev.LastID).JSON(t, &inc)
	if len(inc.Entries) != 1 || inc.Entries[0].ID != id {
		t.Fatalf("incremental: %+v (want only %s)", inc.Entries, id)
	}
}

func TestSessionReuseOneLoginForNCalls(t *testing.T) {
	sim, p := setup(t, rf.AuthSession, nil)
	paths := []string{"/api/v1/power", "/api/v1/sensors", "/api/v1/system", "/api/v1/events", "/api/v1/fru"}
	for i := 0; i < 3; i++ {
		for _, path := range paths {
			wantStatus(t, p.Get(sim, path), 200)
		}
	}
	var wg sync.WaitGroup
	c := sim.Creds()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := p.Try(testsim.Req{Path: paths[i%len(paths)], Host: sim.Host(), User: c.User, Pass: c.Pass})
			if err != nil || r.Status != 200 {
				t.Errorf("concurrent %d: %v %v", i, err, r)
			}
		}(i)
	}
	wg.Wait()
	if sim.Logins() != 1 || p.Sessions.Counters().Opens != 1 || sim.LiveSessions() != 1 {
		t.Fatalf("logins=%d opens=%d live=%d: want one session for %d calls",
			sim.Logins(), p.Sessions.Counters().Opens, sim.LiveSessions(), 3*len(paths)+10)
	}
}

func TestSingleflightCoalescing(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, nil)
	wantStatus(t, p.Get(sim, "/api/v1/sensors"), 200) // open the session
	before := sim.Stats().Requests
	wantStatus(t, p.Get(sim, "/api/v1/sensors"), 200)
	perCall := sim.Stats().Requests - before
	if perCall == 0 {
		t.Fatal("cache TTL must be 0 for this test")
	}

	sim.SetLatency(150 * time.Millisecond)
	before = sim.Stats().Requests
	var wg sync.WaitGroup
	c := sim.Creds()
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := p.Try(testsim.Req{Path: "/api/v1/sensors", Host: sim.Host(), User: c.User, Pass: c.Pass})
			if err != nil || r.Status != 200 {
				t.Errorf("coalesced call: %v %v", err, r)
			}
		}()
	}
	wg.Wait()
	if got := sim.Stats().Requests - before; got != perCall {
		t.Fatalf("10 concurrent /sensors cost %d southbound requests, one call costs %d", got, perCall)
	}
}

func TestEvasive403BackoffThen503(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, nil)
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200)

	// Two caps: retried with backoff, the third attempt succeeds.
	sim.InjectEvasive403(2)
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200)
	if sim.Stats().Evasive403s != 2 {
		t.Fatalf("evasive403s %d", sim.Stats().Evasive403s)
	}
	hs, _ := p.Transport.HostStats(sim.Host())
	if hs.Retries < 2 {
		t.Fatalf("retries %d", hs.Retries)
	}

	// Attempts exhausted: 503 busy, never 401/403.
	sim.InjectEvasive403(3)
	r := p.Get(sim, "/api/v1/power")
	wantStatus(t, r, 503)
	if e := r.Envelope(t); e.Error.Class != bmc.Class(bmc.ErrBusy) {
		t.Fatalf("envelope %+v", e)
	}
	if p.Sessions.Counters().Relogins != 0 {
		t.Fatal("an evasive 403 must not be treated as an auth failure")
	}
}

func TestRedfishLaneStaysUnderEvasiveCap(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, nil)
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200)
	sim.SetLatency(30 * time.Millisecond)
	sim.ResetStats()
	var wg sync.WaitGroup
	c := sim.Creds()
	paths := []string{"/api/v1/power", "/api/v1/sensors", "/api/v1/system", "/api/v1/events", "/api/v1/fru",
		"/api/v1/inventory/cpus", "/api/v1/inventory/memory", "/api/v1/network", "/api/v1/users", "/api/v1/firmware"}
	for _, path := range paths {
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = p.Try(testsim.Req{Path: path, Host: sim.Host(), User: c.User, Pass: c.Pass})
			}()
		}
	}
	wg.Wait()
	st := sim.Stats()
	if st.MaxInFlightRF > transport.LaneRedfish.Concurrency || st.Evasive403s != 0 {
		t.Fatalf("max in-flight %d (lane %d), evasive %d", st.MaxInFlightRF, transport.LaneRedfish.Concurrency, st.Evasive403s)
	}
}

func TestCircuitBreakerTripAndHalfOpen(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, func(o *testsim.ProxyOptions) {
		o.Transport = transport.Config{
			AttemptTimeout: 150 * time.Millisecond,
			Retry:          transport.RetryPolicy{MaxAttempts: 1},
			Breaker:        transport.BreakerConfig{FailureThreshold: 3, OpenTimeout: 300 * time.Millisecond},
		}
	})
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200)

	sim.Wedge()
	for i := 0; i < 3; i++ {
		wantStatus(t, p.Get(sim, "/api/v1/power"), 504)
	}
	if st := p.Transport.BreakerState(sim.Host()); st != transport.BreakerOpen {
		t.Fatalf("breaker %v after 3 timeouts", st)
	}
	seen := sim.Stats().Requests
	start := time.Now()
	r := p.Get(sim, "/api/v1/power")
	wantStatus(t, r, 503)
	if time.Since(start) > 100*time.Millisecond || sim.Stats().Requests != seen {
		t.Fatalf("open breaker must fail fast without touching the BMC (%v, %d new requests)", time.Since(start), sim.Stats().Requests-seen)
	}
	var hs struct {
		Circuit string `json:"circuit"`
	}
	p.Do(testsim.Req{Path: "/api/v1/bmcs/" + sim.Host() + "/status", NoAuth: true}).JSON(t, &hs)
	if hs.Circuit != "open" {
		t.Fatalf("status circuit %q", hs.Circuit)
	}

	sim.Unwedge()
	time.Sleep(350 * time.Millisecond)
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200) // half-open probe succeeds
	if st := p.Transport.BreakerState(sim.Host()); st != transport.BreakerClosed {
		t.Fatalf("breaker %v after successful probe", st)
	}
}

func TestBackendRestartRelogin(t *testing.T) {
	sim, p := setup(t, rf.AuthSession, nil)
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200)
	sim.Restart() // drops every session on the BMC
	wantStatus(t, p.Get(sim, "/api/v1/system"), 200)
	if sim.Logins() != 2 {
		t.Fatalf("logins %d: want exactly one re-login after the restart", sim.Logins())
	}
}

func TestGracefulShutdownLogsOutSessions(t *testing.T) {
	sim, p := setup(t, rf.AuthSession, nil)
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200)
	if sim.LiveSessions() != 1 {
		t.Fatalf("live %d", sim.LiveSessions())
	}
	// An in-flight request is drained before the logout.
	sim.SetLatency(150 * time.Millisecond)
	done := make(chan *testsim.Resp, 1)
	c := sim.Creds()
	go func() {
		r, _ := p.Try(testsim.Req{Path: "/api/v1/sensors", Host: sim.Host(), User: c.User, Pass: c.Pass})
		done <- r
	}()
	time.Sleep(40 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if r := <-done; r == nil || r.Status != 200 {
		t.Fatalf("in-flight request not drained: %+v", r)
	}
	if sim.LiveSessions() != 0 || sim.Stats().Logouts != 1 {
		t.Fatalf("after shutdown: live=%d logouts=%d", sim.LiveSessions(), sim.Stats().Logouts)
	}
}

func TestClockSkewNormalization(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, nil, func(o *testsim.Options) { o.ClockOffset = -time.Hour })
	var ev v1.EventsResponse
	r := p.Get(sim, "/api/v1/events")
	wantStatus(t, r, 200)
	r.JSON(t, &ev)
	if ev.ClockOffsetSeconds == nil || *ev.ClockOffsetSeconds > -3590 || *ev.ClockOffsetSeconds < -3610 {
		t.Fatalf("clock offset %v, want about -3600", ev.ClockOffsetSeconds)
	}
	for _, e := range ev.Entries {
		d := e.Time.Corrected.Sub(e.Time.Raw)
		if d < 3590*time.Second || d > 3610*time.Second {
			t.Fatalf("entry %s: corrected-raw = %v, want +1h", e.ID, d)
		}
	}
	var caps v1.CapabilitiesResponse
	p.Get(sim, "/api/v1/capabilities").JSON(t, &caps)
	if len(caps.Warnings) == 0 || !strings.Contains(caps.Warnings[0], "behind") {
		t.Fatalf("no skew warning: %+v", caps.Warnings)
	}
}

func TestX470RedfishCredentialsRejected(t *testing.T) {
	sim := testsim.StartX470(t)
	p := testsim.StartProxy(t, testsim.ProxyOptions{Drivers: redfishDrivers(rf.AuthBasic)})
	r := p.Get(sim, "/api/v1/power")
	wantStatus(t, r, 401)
	if sim.Stats().Logins != 0 {
		t.Fatal("X470 Redfish must never accept the admin account")
	}
	// The generic driver still detected it as a Redfish service (fallback),
	// over the legacy TLS profile.
	hs, _ := p.Transport.HostStats(sim.Host())
	if hs.TLS == nil || hs.TLS.Profile != transport.ProfileLegacy {
		t.Fatalf("tls %+v", hs.TLS)
	}
}

func TestMethodOverrideNeverReachesBMC(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, nil)
	r := p.Do(testsim.Req{Path: "/api/v1/power", Host: sim.Host(), User: sim.Creds().User, Pass: sim.Creds().Pass,
		Header: map[string]string{"X-HTTP-Method-Override": "DELETE"}})
	wantStatus(t, r, 200)
	// Direct transport use with the header set: stripped before the wire.
	ep := p.Transport.Endpoint(sim.Host(), transport.LaneDefault)
	req, _ := ep.NewRequest(context.Background(), "GET", "/redfish/v1/", nil)
	req.Header.Set("X-HTTP-Method-Override", "DELETE")
	resp, err := ep.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if n := sim.Stats().MethodOverrides; n != 0 {
		t.Fatalf("%d requests reached the BMC with X-HTTP-Method-Override", n)
	}
}

func TestTraceLogsRedactCredentials(t *testing.T) {
	for _, secrets := range []bool{false, true} {
		sim, p := setup(t, rf.AuthSession, func(o *testsim.ProxyOptions) { o.Debug = "all=trace"; o.LogSecrets = secrets })
		wantStatus(t, p.Get(sim, "/api/v1/power"), 200)
		logs := p.Logs.String()
		basic := base64.StdEncoding.EncodeToString([]byte(sim.Creds().User + ":" + sim.Creds().Pass))
		leaked := strings.Contains(logs, sim.Creds().Pass) || strings.Contains(logs, basic)
		if leaked != secrets {
			t.Fatalf("logSecrets=%v but leaked=%v", secrets, leaked)
		}
	}
}

func TestMetricsAndHealth(t *testing.T) {
	sim, p := setup(t, rf.AuthBasic, nil)
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200)
	resp, err := http.Get(p.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(buf), strconv.Quote(sim.Host())) {
		t.Fatalf("metrics %d missing host series", resp.StatusCode)
	}
	wantStatus(t, p.Do(testsim.Req{Path: "/api/v1/health", NoAuth: true}), 200)
}
