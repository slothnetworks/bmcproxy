//go:build live

// Package live_test runs the full proxy (production drivers: vendors/asrock
// + vendors/generic/redfish, real transport and IPMI) against ONE real BMC.
// It is read-only apart from an identify-LED round trip that restores the
// LED's original state.
//
// It is excluded from normal builds (build tag "live") and skips unless all
// of these are set in the environment:
//
//	BMCPROXY_LIVE_HOST  BMC address (ip, host or host:port)
//	BMCPROXY_LIVE_USER  BMC user
//	BMCPROXY_LIVE_PASS  BMC password
//
// Run deliberately, against a BMC you are allowed to touch:
//
//	go test -tags live -count=1 -v ./internal/testsim/live/
package live_test

import (
	"os"
	"strings"
	"testing"
	"time"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/testsim"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/asrock"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
)

type target struct{ host, user, pass string }

func liveTarget(t *testing.T) target {
	t.Helper()
	tg := target{os.Getenv("BMCPROXY_LIVE_HOST"), os.Getenv("BMCPROXY_LIVE_USER"), os.Getenv("BMCPROXY_LIVE_PASS")}
	if tg.host == "" || tg.user == "" || tg.pass == "" {
		t.Skip("BMCPROXY_LIVE_HOST, BMCPROXY_LIVE_USER and BMCPROXY_LIVE_PASS must all be set")
	}
	return tg
}

func startProxy(t *testing.T) *testsim.Proxy {
	t.Helper()
	return testsim.StartProxy(t, testsim.ProxyOptions{
		Drivers: func(tm *transport.Manager) []bmc.Driver {
			return []bmc.Driver{asrock.New(asrock.Options{Transport: tm}), genericredfish.New(tm)}
		},
		// Real BMCs are slow (the X470 /api takes seconds per call) and
		// must not be hammered: production-like timeouts, few retries.
		Transport: transport.Config{
			AttemptTimeout: 60 * time.Second,
			Retry:          transport.RetryPolicy{MaxAttempts: 2},
			Breaker:        transport.BreakerConfig{FailureThreshold: 5, OpenTimeout: 30 * time.Second},
		},
		V1: v1.Config{PowerPollInterval: 2 * time.Second, PowerPollTimeout: 60 * time.Second},
	})
}

func (tg target) req(method, path string, body any) testsim.Req {
	return testsim.Req{Method: method, Path: path, Host: tg.host, User: tg.user, Pass: tg.pass, Body: body}
}

// TestLiveReadOnly reads every generic read endpoint the driver reports as
// supported. Nothing is written.
func TestLiveReadOnly(t *testing.T) {
	tg := liveTarget(t)
	p := startProxy(t)

	r := p.Do(tg.req("GET", "/api/v1/capabilities", nil))
	if r.Status != 200 {
		t.Fatalf("capabilities: %d %s", r.Status, r.Body)
	}
	var caps v1.CapabilitiesResponse
	r.JSON(t, &caps)
	t.Logf("driver %s board %q firmware %q warnings %v", caps.Driver, caps.Board, caps.Firmware, caps.Warnings)
	supported := map[bmc.Capability]bool{}
	for _, c := range caps.Capabilities {
		supported[c.Capability] = c.Supported
		t.Logf("  %-24s supported=%v backend=%s", c.Capability, c.Supported, c.Backend)
	}

	reads := []struct {
		path string
		cap  bmc.Capability
	}{
		{"/api/v1/system", bmc.CapIdentity},
		{"/api/v1/power", bmc.CapPower},
		{"/api/v1/power/restore-policy", bmc.CapPowerRestorePolicy},
		{"/api/v1/led", bmc.CapIndicatorLED},
		{"/api/v1/boot", bmc.CapBootOverride},
		{"/api/v1/sensors", bmc.CapSensors},
		{"/api/v1/events", bmc.CapEventLog},
		{"/api/v1/fru", bmc.CapFRU},
		{"/api/v1/inventory/cpus", bmc.CapInventory},
		{"/api/v1/inventory/memory", bmc.CapInventory},
		{"/api/v1/network", bmc.CapNetwork},
		{"/api/v1/users", bmc.CapUsers},
		{"/api/v1/services", bmc.CapServices},
		{"/api/v1/time", bmc.CapTime},
		{"/api/v1/firmware", bmc.CapFirmwareInventory},
	}
	for _, rd := range reads {
		if !supported[rd.cap] {
			t.Logf("skip %s (%s unsupported)", rd.path, rd.cap)
			continue
		}
		r := p.Do(tg.req("GET", rd.path, nil))
		if r.Status != 200 {
			t.Errorf("%s: %d %s", rd.path, r.Status, r.Body)
			continue
		}
		if strings.Contains(strings.ToLower(string(r.Body)), `"password":"`) {
			t.Errorf("%s leaks a password field", rd.path)
		}
		t.Logf("%s: %d bytes", rd.path, len(r.Body))
	}
	if caps.Driver == "asrock" {
		for _, path := range []string{"/api/v1/vendor/asrock/bonding", "/api/v1/vendor/asrock/fanMode", "/api/v1/vendor/asrock/keepNicLinkUp"} {
			r := p.Do(tg.req("GET", path, nil))
			t.Logf("%s: %d %s", path, r.Status, r.Body)
		}
	}
}

// TestLiveLEDRoundTrip turns the identify LED on, reads it back and
// restores the original state.
func TestLiveLEDRoundTrip(t *testing.T) {
	tg := liveTarget(t)
	p := startProxy(t)
	var orig v1.LEDBody
	r := p.Do(tg.req("GET", "/api/v1/led", nil))
	if r.Status != 200 {
		t.Fatalf("led: %d %s", r.Status, r.Body)
	}
	r.JSON(t, &orig)
	t.Cleanup(func() {
		restore := orig.State
		if restore != bmc.LEDOn {
			restore = bmc.LEDOff
		}
		if r := p.Do(tg.req("POST", "/api/v1/led", map[string]any{"state": restore})); r.Status != 204 {
			t.Errorf("restoring LED to %v: %d %s", restore, r.Status, r.Body)
		}
	})
	for _, st := range []bmc.LEDState{bmc.LEDOn, bmc.LEDOff} {
		if r := p.Do(tg.req("POST", "/api/v1/led", map[string]any{"state": st})); r.Status != 204 {
			t.Fatalf("set LED %v: %d %s", st, r.Status, r.Body)
		}
		var got v1.LEDBody
		r := p.Do(tg.req("GET", "/api/v1/led", nil))
		r.JSON(t, &got)
		if got.State != st {
			t.Fatalf("LED reads %v after setting %v", got.State, st)
		}
	}
}
