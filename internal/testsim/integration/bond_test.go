package integration_test

import (
	"fmt"
	"strings"
	"testing"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte"
)

// Generic NIC bonding (GET/PUT /api/v1/network/bond) on all five sims:
// GET works (backend ami in /capabilities); PUT (disruptive tier, not gated)
// writes the full settings/network-bond object to the sim.

func TestGenericBond(t *testing.T) {
	for _, board := range append(append([]string(nil), allBoards...), gigabyteBoards...) {
		t.Run(board, func(t *testing.T) {
			gbt := boardVendor[board] == "gigabyte"
			sim, p, _ := setupBoard(t, board, nil)

			var caps v1.CapabilitiesResponse
			r := p.Get(sim, "/api/v1/capabilities")
			wantStatus(t, r, 200)
			r.JSON(t, &caps)
			found := false
			for _, c := range caps.Capabilities {
				if c.Capability == bmc.CapBond {
					found = c.Supported && c.Backend == bmc.BackendAMI
				}
			}
			if !found {
				t.Fatalf("bond not reported as ami: %+v", caps.Capabilities)
			}

			var b v1.BondResponse
			r = p.Get(sim, "/api/v1/network/bond")
			wantStatus(t, r, 200)
			r.JSON(t, &b)
			if b.ID != 1 || b.Mode != "active-backup" || b.Oem == nil {
				t.Fatalf("bond: %s", r.Body)
			}
			if gbt && (!b.Enabled || len(b.Interfaces) != 1 || b.Interfaces[0] != "both" || !b.AutoConfig) {
				t.Fatalf("gigabyte bond (live shape): %s", r.Body)
			}

			body := map[string]any{"enabled": false, "interfaces": []string{"eth0"}}
			wantStatus(t, p.Do(req(sim, "PUT", "/api/v1/network/bond", map[string]any{"mode": "x"}, nil)), 400)
			if n := sim.Count("PUT", "/api/settings/network-bond"); n != 0 {
				t.Fatalf("invalid request reached the BMC %d times", n)
			}
			wantStatus(t, p.Do(req(sim, "PUT", "/api/v1/network/bond", body, nil)), 204)
			put := lastBody(sim, "PUT", "/api/settings/network-bond")
			for _, k := range []string{`"id":1`, `"bond_enable":0`, `"bond_mode":"active-backup"`, `"auto_configuration_enable":`} {
				if !strings.Contains(put, k) {
					t.Fatalf("network-bond PUT lacks %s: %s", k, put)
				}
			}
			if wantIfc := board == "b650" || gbt; strings.Contains(put, `"bond_ifc":"eth0"`) != wantIfc {
				t.Fatalf("bond_ifc on %s: %s", board, put)
			}
			if fmt.Sprint(sim.OEMObject("/api/settings/network-bond")["bond_enable"]) != "0" {
				t.Fatal("sim bond not disabled")
			}
			// The write dropped the cached read.
			b = v1.BondResponse{}
			r = p.Get(sim, "/api/v1/network/bond")
			wantStatus(t, r, 200)
			r.JSON(t, &b)
			if b.Enabled {
				t.Fatalf("stale bond after PUT: %s", r.Body)
			}
		})
	}
}

// TestGigabyteBondingVendorRoutes: GET bonding (read) and
// enable/disableBonding (disruptive) on both Gigabyte sims.
func TestGigabyteBondingVendorRoutes(t *testing.T) {
	for _, board := range gigabyteBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			var st gigabyte.BondingStatus
			r := p.Get(sim, "/api/v1/vendor/gigabyte/bonding")
			wantStatus(t, r, 200)
			r.JSON(t, &st)
			if st.ID != 1 || !st.Enabled || st.Mode != "active-backup" || st.Interface != "both" || !st.AutoConfig {
				t.Fatalf("bonding: %s", r.Body)
			}
			r = p.Do(req(sim, "POST", "/api/v1/vendor/gigabyte/disableBonding", map[string]any{"interface": "eth1"}, nil))
			wantStatus(t, r, 200)
			st = gigabyte.BondingStatus{}
			r.JSON(t, &st)
			if st.Enabled || st.Interface != "eth1" {
				t.Fatalf("disableBonding: %s", r.Body)
			}
			r = p.Do(req(sim, "POST", "/api/v1/vendor/gigabyte/enableBonding", map[string]any{"interface": "both"}, nil))
			wantStatus(t, r, 200)
			obj := sim.OEMObject("/api/settings/network-bond")
			if fmt.Sprint(obj["bond_enable"]) != "1" || obj["bond_ifc"] != "both" || obj["bond_mode"] != "active-backup" {
				t.Fatalf("sim object after enable: %v", obj)
			}
			if n := sim.Count("PUT", "/api/settings/network-bond"); n != 2 {
				t.Fatalf("%d PUTs, want 2", n)
			}
		})
	}
}
