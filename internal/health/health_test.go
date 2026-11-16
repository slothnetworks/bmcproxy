package health_test

import (
	"testing"

	"github.com/slothnetworks/bmcproxy/internal/health"
	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

func TestHealth(t *testing.T) {
	d := &servertest.Driver{NameV: "acme", Prio: 7, Routes: servertest.SampleRoutes()}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	var hb health.Health
	r := h.Do(servertest.Req{Path: "/api/v1/health", NoAuth: true})
	r.JSON(t, &hb)
	if r.Status != 200 || hb.Status != "ok" || hb.Version != "test" || len(hb.Drivers) != 1 ||
		hb.Drivers[0].Name != "acme" || hb.Drivers[0].Priority != 7 || hb.Drivers[0].VendorRoutes != len(servertest.SampleRoutes()) {
		t.Fatalf("health %d %s", r.Status, r.Body)
	}
}

func TestStatus(t *testing.T) {
	d := &servertest.Driver{NameV: "acme", Configure: func(s *servertest.Session) { s.Clock = -3 * 3600e9 }}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	var st health.Status
	h.Do(servertest.Req{Path: "/api/v1/bmcs/10.5.5.5/status", NoAuth: true}).JSON(t, &st)
	if st.Known || st.Reachable != nil || st.Health != bmc.HealthUnknown || st.Circuit != "closed" {
		t.Fatalf("unknown host status %+v", st)
	}
	h.Do(servertest.Req{Path: "/api/v1/power", Host: "10.5.5.5"})
	r := h.Do(servertest.Req{Path: "/api/v1/bmcs/10.5.5.5/status", NoAuth: true})
	r.JSON(t, &st)
	if r.Status != 200 || !st.Known || st.Reachable == nil || !*st.Reachable || st.Vendor != "acme" || st.Board != "FAKE1" ||
		st.Firmware != "1.0" || st.Sessions != 1 || len(st.Backends) != 1 || st.ClockOffsetSeconds == nil ||
		*st.ClockOffsetSeconds != -10800 || len(st.Warnings) == 0 || st.Health != bmc.HealthWarning {
		t.Fatalf("status %s", r.Body)
	}
	// probe=true runs unauthenticated detection.
	h.Do(servertest.Req{Path: "/api/v1/bmcs/10.6.6.6:8443/status?probe=true", NoAuth: true}).JSON(t, &st)
	if st.Host != "10.6.6.6:8443" || st.Vendor != "acme" || st.LastProbe.IsZero() {
		t.Fatalf("probe status %+v", st)
	}
	if r := h.Do(servertest.Req{Path: "/api/v1/bmcs/host:notaport/status", NoAuth: true}); r.Status != 400 {
		t.Fatalf("bad host: %d", r.Status)
	}
}
