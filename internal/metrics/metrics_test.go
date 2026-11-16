package metrics_test

import (
	"strings"
	"testing"

	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

func TestMetricsExposition(t *testing.T) {
	d := &servertest.Driver{NameV: "acme"}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	h.Do(servertest.Req{Path: "/api/v1/sensors", Host: "10.7.7.7"})
	h.Do(servertest.Req{Path: "/api/v1/sensors", Host: "10.7.7.7"})
	h.Do(servertest.Req{Path: "/api/v1/boot", Host: "10.7.7.7"})
	// Touch the transport so per-host southbound series exist (no network: breaker state only).
	h.Transport.Endpoint("10.7.7.7:443", transport.LaneAMI)
	r := h.Do(servertest.Req{Path: "/metrics", NoAuth: true})
	if r.Status != 200 {
		t.Fatalf("status %d", r.Status)
	}
	body := string(r.Body)
	for _, want := range []string{
		`bmcproxy_http_requests_total{class="ok",code="200",host="10.7.7.7:443",route="GET /api/v1/sensors"} 2`,
		`bmcproxy_http_requests_total{class="unsupported",code="501",host="10.7.7.7:443",route="GET /api/v1/boot"} 1`,
		`bmcproxy_http_request_duration_seconds_bucket{route="GET /api/v1/sensors"`,
		`bmcproxy_sessions_live 1`,
		`bmcproxy_session_opens_total 1`,
		`bmcproxy_cache_hits_total 1`,
		`bmcproxy_cache_misses_total`,
		`bmcproxy_cache_hit_ratio`,
		`bmcproxy_sessions_live_by_host{host="10.7.7.7:443"} 1`,
		`go_goroutines`,
		`bmcproxy_breaker_state{host="10.7.7.7:443"} 0`,
		`bmcproxy_lane_in_flight{host="10.7.7.7:443",lane="ami"} 0`,
		`bmcproxy_southbound_attempts_total{host="10.7.7.7:443"} 0`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	if t.Failed() {
		t.Log(body)
	}
}
