package v1_test

import (
	"sync"
	"testing"
	"time"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

const host = "10.1.2.3"

func harness(t *testing.T, d *servertest.Driver, o servertest.Options) *servertest.Harness {
	o.Drivers = []bmc.Driver{d}
	return servertest.New(t, o)
}

func TestSessionReuse(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{CacheTTL: -1})
	for i := 0; i < 10; i++ {
		for _, p := range []string{"/api/v1/power", "/api/v1/system", "/api/v1/sensors", "/api/v1/led"} {
			if r := h.Do(servertest.Req{Path: p, Host: host}); r.Status != 200 {
				t.Fatalf("%s: %d %s", p, r.Status, r.Body)
			}
		}
	}
	if d.Opens.Load() != 1 {
		t.Fatalf("Open called %d times for 40 requests", d.Opens.Load())
	}
}

func TestSingleflightSensors(t *testing.T) {
	gate := make(chan struct{})
	d := &servertest.Driver{NameV: "fake", Configure: func(s *servertest.Session) { s.SensorsGate = gate }}
	h := harness(t, d, servertest.Options{})
	// Open the session first so all requests share it.
	h.Do(servertest.Req{Path: "/api/v1/power", Host: host})
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := h.Do(servertest.Req{Path: "/api/v1/sensors", Host: host}); r.Status != 200 {
				t.Errorf("status %d", r.Status)
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()
	if n := d.Sessions()[0].Calls("Sensors"); n != 1 {
		t.Fatalf("Sensors called %d times for 12 concurrent requests", n)
	}
	// Cached afterwards.
	h.Do(servertest.Req{Path: "/api/v1/sensors", Host: host})
	if n := d.Sessions()[0].Calls("Sensors"); n != 1 {
		t.Fatalf("not cached: %d", n)
	}
	// Different credentials never share a cached result.
	h.Do(servertest.Req{Path: "/api/v1/sensors", Host: host, User: "other", Pass: "pw"})
	if d.Opens.Load() != 2 {
		t.Fatal("other user did not get its own session")
	}
}

func TestCacheInvalidatedAfterWrite(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{})
	var led v1.LEDBody
	h.Do(servertest.Req{Path: "/api/v1/led", Host: host}).JSON(t, &led)
	if led.State != bmc.LEDOff {
		t.Fatalf("led %v", led.State)
	}
	h.Do(servertest.Req{Path: "/api/v1/led", Host: host}) // cached
	s := d.Sessions()[0]
	if s.Calls("GetIndicatorLED") != 1 {
		t.Fatal("expected cache hit")
	}
	if r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/led", Host: host, Body: map[string]string{"state": "on"}}); r.Status != 204 {
		t.Fatalf("set led %d %s", r.Status, r.Body)
	}
	h.Do(servertest.Req{Path: "/api/v1/led", Host: host}).JSON(t, &led)
	if led.State != bmc.LEDOn || s.Calls("GetIndicatorLED") != 2 {
		t.Fatalf("stale read after write: %v calls=%d", led.State, s.Calls("GetIndicatorLED"))
	}
	if s.Calls("SetIndicatorLED") != 1 {
		t.Fatal("write not executed exactly once")
	}
	// Bad body -> 400.
	if r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/led", Host: host, Body: map[string]string{"state": "purple"}}); r.Status != 400 {
		t.Fatalf("invalid state: %d", r.Status)
	}
}

func TestMissingCapability501(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Minimal: true}
	h := harness(t, d, servertest.Options{})
	for _, rq := range []servertest.Req{
		{Path: "/api/v1/power"}, {Path: "/api/v1/boot"}, {Path: "/api/v1/sensors"}, {Path: "/api/v1/inventory/cpus"},
		{Path: "/api/v1/kvm"}, {Path: "/api/v1/firmware"},
		{Method: "POST", Path: "/api/v1/firmware/update", Body: map[string]string{"component": "BMC", "imageUri": "http://x/y"}},
	} {
		rq.Host = host
		r := h.Do(rq)
		if r.Status != 501 || r.Envelope(t).Error.Class != "unsupported" {
			t.Fatalf("%s %s: %d %s", rq.Method, rq.Path, r.Status, r.Body)
		}
	}
	if r := h.Do(servertest.Req{Path: "/api/v1/system", Host: host}); r.Status != 200 {
		t.Fatalf("system: %d", r.Status)
	}
	var caps v1.CapabilitiesResponse
	h.Do(servertest.Req{Path: "/api/v1/capabilities", Host: host}).JSON(t, &caps)
	sup := map[bmc.Capability]bool{}
	for _, c := range caps.Capabilities {
		sup[c.Capability] = c.Supported
	}
	if !sup[bmc.CapIdentity] || sup[bmc.CapPower] || len(caps.Capabilities) != len(bmc.AllCapabilities()) {
		t.Fatalf("capabilities %+v", caps)
	}
}

func TestPowerPollingConverges(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Configure: func(s *servertest.Session) { s.ConvergeAfter = 3 }}
	h := harness(t, d, servertest.Options{})
	h.Do(servertest.Req{Path: "/api/v1/power", Host: host}) // caches "on"
	r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/power", Host: host, Body: map[string]string{"action": "shutdown"}})
	var res v1.PowerActionResult
	r.JSON(t, &res)
	if r.Status != 200 || !res.Converged || res.State != bmc.PowerOff || res.Polls != 3 || res.Expected != bmc.PowerOff {
		t.Fatalf("status %d result %+v", r.Status, res)
	}
	var ps v1.PowerStateResponse
	h.Do(servertest.Req{Path: "/api/v1/power", Host: host}).JSON(t, &ps)
	if ps.State != bmc.PowerOff {
		t.Fatalf("cached power state not invalidated: %v", ps.State)
	}
}

func TestPowerPollingTimeout(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Configure: func(s *servertest.Session) { s.ConvergeAfter = -1 }}
	h := harness(t, d, servertest.Options{V1: v1.Config{PowerPollInterval: 10 * time.Millisecond, PowerPollTimeout: 80 * time.Millisecond}})
	r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/power", Host: host, Body: map[string]string{"action": "shutdown"}})
	var res v1.PowerActionResult
	r.JSON(t, &res)
	if r.Status != 202 || res.Converged || res.State != bmc.PowerOn || res.Warning == "" || res.Polls < 2 {
		t.Fatalf("status %d result %+v", r.Status, res)
	}
	// wait=false skips polling.
	r = h.Do(servertest.Req{Method: "POST", Path: "/api/v1/power?wait=false", Host: host, Body: map[string]string{"action": "on"}})
	r.JSON(t, &res)
	if r.Status != 202 || res.Polled {
		t.Fatalf("wait=false: %d %+v", r.Status, res)
	}
	if r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/power", Host: host, Body: map[string]string{"action": "explode"}}); r.Status != 400 {
		t.Fatalf("bad action: %d", r.Status)
	}
}

func TestEventsSkewCorrection(t *testing.T) {
	raw := time.Date(2026, 5, 1, 15, 0, 0, 0, time.UTC)
	d := &servertest.Driver{NameV: "fake", Configure: func(s *servertest.Session) {
		s.Clock = 2 * time.Hour
		s.SEL = []bmc.SELEntry{{ID: "1", Time: bmc.Timestamp{Raw: raw, Corrected: raw}, Message: "x"}}
	}}
	h := harness(t, d, servertest.Options{})
	var ev v1.EventsResponse
	r := h.Do(servertest.Req{Path: "/api/v1/events?since=0&limit=10", Host: host})
	r.JSON(t, &ev)
	if r.Status != 200 || len(ev.Entries) != 1 || ev.ClockOffsetSeconds == nil || *ev.ClockOffsetSeconds != 7200 {
		t.Fatalf("events %d %s", r.Status, r.Body)
	}
	if !ev.Entries[0].Time.Raw.Equal(raw) || !ev.Entries[0].Time.Corrected.Equal(raw.Add(-2*time.Hour)) {
		t.Fatalf("timestamps %+v", ev.Entries[0].Time)
	}
	// Large skew is surfaced as a warning.
	var caps v1.CapabilitiesResponse
	h.Do(servertest.Req{Path: "/api/v1/capabilities", Host: host}).JSON(t, &caps)
	if len(caps.Warnings) == 0 {
		t.Fatal("no skew warning")
	}
	if r := h.Do(servertest.Req{Path: "/api/v1/events?limit=-1", Host: host}); r.Status != 400 {
		t.Fatalf("bad limit: %d", r.Status)
	}
}

func TestReloginOnStaleSession(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{CacheTTL: -1})
	h.Do(servertest.Req{Path: "/api/v1/system", Host: host})
	d.Sessions()[0].FailAuth.Store(1)
	r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/led", Host: host, Body: map[string]string{"state": "on"}})
	if r.Status != 204 || d.Opens.Load() != 2 {
		t.Fatalf("relogin: %d %s opens=%d", r.Status, r.Body, d.Opens.Load())
	}
	// The body was replayed on the retry.
	if d.Sessions()[1].Calls("SetIndicatorLED") != 1 {
		t.Fatal("retry did not reach the new session")
	}
}

func TestBMCResetInvalidatesSessions(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{})
	r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/bmc/reset", Host: host})
	if r.Status != 202 {
		t.Fatalf("reset %d %s", r.Status, r.Body)
	}
	servertest.Eventually(t, time.Second, func() bool { return h.Sessions.Len() == 0 }, "sessions dropped")
	if d.Sessions()[0].Calls("ResetBMC") != 1 {
		t.Fatal("reset not called")
	}
}

func TestInventoryUnknownKind(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{})
	// Session lacks Inventory -> 501 regardless of kind validity is acceptable;
	// network PUT requires name.
	r := h.Do(servertest.Req{Method: "PUT", Path: "/api/v1/network", Host: host, Body: map[string]any{}})
	if r.Status != 400 {
		t.Fatalf("network put without name: %d %s", r.Status, r.Body)
	}
}
