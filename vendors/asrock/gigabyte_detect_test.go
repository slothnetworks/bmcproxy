package asrock_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/testsim"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/asrock"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
)

// The ASRock driver must not claim another vendor's MegaRAC. Before the
// gigabyte driver existed it logged in to the MC13-LE0's /api and returned
// score 30; these tests replay the Gigabyte captures (internal/testsim
// StartMC13LE0 / StartMZ32AR0) against the ASRock driver alone.

func gigabyteSims(t *testing.T) map[string]*testsim.Sim {
	return map[string]*testsim.Sim{"mc13le0": testsim.StartMC13LE0(t), "mz32ar0": testsim.StartMZ32AR0(t)}
}

func testTransport(t *testing.T) *transport.Manager {
	tm := transport.NewManager(transport.Config{Retry: transport.RetryPolicy{MaxAttempts: 1}, Breaker: transport.BreakerConfig{Disabled: true}})
	t.Cleanup(tm.Close)
	return tm
}

func TestASRockDoesNotClaimGigabyte(t *testing.T) {
	ctx := context.Background()
	for name, sim := range gigabyteSims(t) {
		t.Run(name, func(t *testing.T) {
			d := asrock.New(asrock.Options{Transport: testTransport(t), HandoffTTL: -1})
			m, err := d.DetectWithCreds(ctx, sim.Target(), sim.Creds())
			if err != nil {
				t.Fatal(err)
			}
			if m.Score >= bmc.ScoreFallback || m.Score <= 0 || m.Board != "" || !strings.Contains(m.Details["nonASRock"], "$metadata") {
				t.Fatalf("DetectWithCreds: %+v", m)
			}
			// The unauthenticated evidence ($metadata GBT namespaces) was
			// enough: no /api login, no credentials sent anywhere.
			if n := sim.Count("POST", "/api/session"); n != 0 {
				t.Fatalf("%d /api logins", n)
			}
			for _, r := range sim.Requests() {
				if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Fatalf("credentials sent on %s %s", r.Method, r.Path)
				}
			}
		})
	}
}

// TestASRockNoASRRCapped: when the unauthenticated evidence is missing
// ($metadata unavailable), the /api login finds no asrr namespace and the
// match is capped below the generic Redfish fallback; the session is
// logged out again.
func TestASRockNoASRRCapped(t *testing.T) {
	ctx := context.Background()
	for name, sim := range gigabyteSims(t) {
		t.Run(name, func(t *testing.T) {
			sim.Handle("GET /redfish/v1/$metadata", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
			d := asrock.New(asrock.Options{Transport: testTransport(t), HandoffTTL: -1})
			m, err := d.DetectWithCreds(ctx, sim.Target(), sim.Creds())
			if err != nil {
				t.Fatal(err)
			}
			if m.Score >= bmc.ScoreFallback || m.Score <= 0 || m.Details["ami"] != "logged in; no asrr namespace" {
				t.Fatalf("DetectWithCreds: %+v", m)
			}
			if sim.Count("GET", "/api/asrr/model-name") == 0 {
				t.Fatal("asrr/model-name not probed")
			}
			testsim.Eventually(t, 2*time.Second, func() bool { return sim.APISessions() == 0 }, "/api session logged out")
		})
	}
}

// TestGenericBeatsASRockOnGigabyte: with only the asrock and generic
// Redfish drivers registered, the generic driver serves a Gigabyte board.
func TestGenericBeatsASRockOnGigabyte(t *testing.T) {
	ctx := context.Background()
	for name, sim := range gigabyteSims(t) {
		t.Run(name, func(t *testing.T) {
			tm := testTransport(t)
			reg := bmc.NewRegistry()
			reg.Register(asrock.New(asrock.Options{Transport: tm, HandoffTTL: -1}))
			reg.Register(genericredfish.New(tm))
			c := sim.Creds()
			for _, creds := range []*bmc.Creds{nil, &c} {
				d, m, err := reg.Resolve(ctx, sim.Target(), creds)
				if err != nil {
					t.Fatal(err)
				}
				// Unauthenticated, asrock's MegaRAC guess (35) still
				// outranks the fallback; with credentials it must not.
				if creds != nil && d.Name() != genericredfish.Name {
					t.Fatalf("resolved %s (%+v), want redfish", d.Name(), m)
				}
			}
			if n := sim.Count("POST", "/api/session"); n != 0 {
				t.Fatalf("%d /api logins during resolve", n)
			}
		})
	}
}
