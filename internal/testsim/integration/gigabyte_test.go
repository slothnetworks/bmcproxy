package integration_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/testsim"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte/boards"
)

// Gigabyte rows of the integration matrix: the production driver set
// against the MC13-LE0 13.04.15 (live-verified) and MZ32-AR0 12.61.39
// (fixture-only) simulators. Per-board differences are data in
// gigabyteExpect; every test loops over gigabyteBoards.

type gbtExpect struct {
	// unsupportedPower are the power actions the board's ResetActionInfo
	// lacks (501 from the generic ResetTypeFor).
	unsupportedPower []string
	// selFromAMI: events come from /api/logs/event (MZ32-AR0).
	selFromAMI bool
	// selMessageTime: a SEL entry whose real time is the Message
	// "Timestamp :" field, not Created (MC13-LE0): id -> RFC 3339 time.
	selMessageTime map[string]string
}

var gigabyteExpect = map[string]gbtExpect{
	"mc13le0": {unsupportedPower: []string{"nmi"}, selMessageTime: map[string]string{"168": "2025-03-25T16:07:47Z"}},
	"mz32ar0": {unsupportedPower: []string{"cycle", "nmi"}, selFromAMI: true},
}

// gigabyteUnsupported are the capabilities neither Gigabyte board serves.
var gigabyteUnsupported = map[bmc.Capability]bool{bmc.CapFirmwareUpdate: true, bmc.CapKVM: true}

// fixtureSensorMins returns the per-type sensor counts the board's Redfish
// fixture allows (Thermal Temperatures/Fans, Power Voltages).
func fixtureSensorMins(t *testing.T, board string) map[bmc.SensorType]int {
	t.Helper()
	tree, err := testsim.LoadRedfishTree(board)
	if err != nil {
		t.Fatal(err)
	}
	n := func(path, key string) int { a, _ := tree[path][key].([]any); return len(a) }
	return map[bmc.SensorType]int{
		bmc.SensorTemperature: n("/redfish/v1/Chassis/Self/Thermal", "Temperatures"),
		bmc.SensorFan:         n("/redfish/v1/Chassis/Self/Thermal", "Fans"),
		bmc.SensorVoltage:     n("/redfish/v1/Chassis/Self/Power", "Voltages"),
	}
}

// TestGigabyteReads: capabilities, identity, sensors (Thermal/Power merged
// past the Energy/Power-only Sensors collection), SEL (paged past the
// MC13 50-member truncation, real event times; MZ32 from /api), FRU with
// the OEM board serial, inventory and firmware, all without tripping the
// simulators' query quirks.
func TestGigabyteReads(t *testing.T) {
	for _, board := range gigabyteBoards {
		t.Run(board, func(t *testing.T) {
			want := gigabyteExpect[board]
			sim, p, _ := setupBoard(t, board, nil)

			r := p.Get(sim, "/api/v1/capabilities")
			wantStatus(t, r, 200)
			caps, cm := capMap(t, r)
			if caps.Driver != "gigabyte" || caps.Board != boardName[board] || caps.Firmware != sim.Profile().Firmware {
				t.Fatalf("capabilities: driver %q board %q firmware %q", caps.Driver, caps.Board, caps.Firmware)
			}
			for _, c := range bmc.AllCapabilities() {
				if cm[c].Supported == gigabyteUnsupported[c] {
					t.Errorf("%s supported=%v", c, cm[c].Supported)
				}
			}
			for c, be := range map[bmc.Capability]string{bmc.CapSensors: "redfish", bmc.CapPower: "redfish"} {
				if cm[c].Backend != be {
					t.Errorf("%s backend %q want %q", c, cm[c].Backend, be)
				}
			}
			if want.selFromAMI != (cm[bmc.CapEventLog].Backend == "ami") {
				t.Errorf("eventLog backend %q", cm[bmc.CapEventLog].Backend)
			}

			var sys bmc.SystemInfo
			r = p.Get(sim, "/api/v1/system")
			wantStatus(t, r, 200)
			r.JSON(t, &sys)
			if sys.Board != boardName[board] || sys.BMCFirmware != sim.Profile().Firmware || sys.PowerState != bmc.PowerOn {
				t.Fatalf("system: %+v", sys)
			}

			var sens v1.SensorsResponse
			r = p.Get(sim, "/api/v1/sensors")
			wantStatus(t, r, 200)
			r.JSON(t, &sens)
			got := map[bmc.SensorType]int{}
			for _, s := range sens.Sensors {
				got[s.Type]++
			}
			for typ, min := range fixtureSensorMins(t, board) {
				if min == 0 || got[typ] < min {
					t.Errorf("%s sensors %d, fixture has %d", typ, got[typ], min)
				}
			}

			var ev v1.EventsResponse
			r = p.Get(sim, "/api/v1/events")
			wantStatus(t, r, 200)
			r.JSON(t, &ev)
			if len(ev.Entries) == 0 || ev.LastID == "" {
				t.Fatalf("events: %.300s", r.Body)
			}
			if board == "mc13le0" && len(ev.Entries) <= 50 {
				t.Fatalf("MC13 SEL %d entries: not paged past the 50-member truncation", len(ev.Entries))
			}
			for id, ts := range want.selMessageTime {
				found := false
				for _, e := range ev.Entries {
					if e.ID == id {
						found = true
						if got := e.Time.Raw.UTC().Format(time.RFC3339); got != ts {
							t.Errorf("SEL %s time %s, want the event time %s (not Created)", id, got, ts)
						}
					}
				}
				if !found {
					t.Errorf("SEL %s missing", id)
				}
			}
			var newID string
			if want.selFromAMI {
				newID = fmt.Sprint(sim.AddEvent("CPU0_TEMP", "Upper Critical - going high"))
			} else {
				newID = sim.Redfish().AddSELEntry("CPU0_TEMP upper critical", "Critical")
			}
			var inc v1.EventsResponse
			r = p.Get(sim, "/api/v1/events?since="+ev.LastID)
			wantStatus(t, r, 200)
			r.JSON(t, &inc)
			if len(inc.Entries) != 1 || inc.Entries[0].ID != newID {
				t.Fatalf("incremental: %s (want only %s)", r.Body, newID)
			}

			var fru v1.FRUResponse
			r = p.Get(sim, "/api/v1/fru")
			wantStatus(t, r, 200)
			r.JSON(t, &fru)
			if len(fru.FRU) == 0 || fru.FRU[0].Board.Serial != testsim.GigabyteBoardSerial || fru.FRU[0].Board.Product != boardName[board] {
				t.Fatalf("fru (board serial must come from the GBT OEM block): %s", r.Body)
			}
			r = p.Get(sim, "/api/v1/inventory/baseboard")
			wantStatus(t, r, 200)
			if !strings.Contains(string(r.Body), testsim.GigabyteBoardSerial) {
				t.Fatalf("baseboard: %s", r.Body)
			}
			for _, kind := range []string{"cpus", "memory", "storage", "pcie"} {
				wantStatus(t, p.Get(sim, "/api/v1/inventory/"+kind), 200)
			}
			for _, path := range []string{"/api/v1/network", "/api/v1/users", "/api/v1/services", "/api/v1/time", "/api/v1/logs/audit", "/api/v1/led", "/api/v1/boot"} {
				wantStatus(t, p.Get(sim, path), 200)
			}
			var fw v1.FirmwareResponse
			r = p.Get(sim, "/api/v1/firmware")
			wantStatus(t, r, 200)
			r.JSON(t, &fw)
			ok := false
			for _, c := range fw.Components {
				ok = ok || strings.EqualFold(c.ID, "bmc") && c.Version == sim.Profile().Firmware
			}
			if !ok {
				t.Fatalf("firmware: %s", r.Body)
			}

			// Redfish-first: the /api is logged in lazily (MZ32: for the
			// decoded SEL only), once, and never with a bad CSRF token.
			wantLogins := 0
			if want.selFromAMI {
				wantLogins = 1
			}
			st := sim.Stats()
			if st.Logins != wantLogins || st.CSRFRejects != 0 || st.AuthRejects != 0 {
				t.Fatalf("logins %d (want %d), CSRF rejects %d, auth rejects %d", st.Logins, wantLogins, st.CSRFRejects, st.AuthRejects)
			}
			for _, rq := range sim.Requests() {
				q := strings.ToLower(rq.RawQuery)
				switch {
				case board == "mc13le0" && strings.Contains(q, "$skip=0"):
					t.Fatalf("MC13: %s?%s ($skip=0 is a 400)", rq.Path, rq.RawQuery)
				case board == "mz32ar0" && (strings.Contains(q, "%24") || strings.Contains(q, "$filter")):
					t.Fatalf("MZ32: %s?%s (encoded $ ignored / $filter page-local)", rq.Path, rq.RawQuery)
				case strings.Contains(rq.Path, "SubProcessors"):
					t.Fatalf("walked the SubProcessors crawl trap: %s", rq.Path)
				}
			}
		})
	}
}

// TestGigabytePower: on/off converge; actions missing from the board's
// ResetActionInfo answer 501 without reaching the BMC.
func TestGigabytePower(t *testing.T) {
	for _, board := range gigabyteBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil, func(o *testsim.Options) { o.PowerDelay = 20 * time.Millisecond })
			for _, a := range []string{"off", "on"} {
				var res v1.PowerActionResult
				r := p.Do(req(sim, "POST", "/api/v1/power", map[string]any{"action": a}, nil))
				wantStatus(t, r, 200)
				r.JSON(t, &res)
				if !res.Converged || string(res.State) != a {
					t.Fatalf("%s: %s", a, r.Body)
				}
			}
			resets := sim.Count("POST", "/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset")
			for _, a := range gigabyteExpect[board].unsupportedPower {
				wantStatus(t, p.Do(req(sim, "POST", "/api/v1/power", map[string]any{"action": a}, nil)), 501)
			}
			if n := sim.Count("POST", "/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset"); n != resets {
				t.Fatalf("unsupported actions reached the BMC (%d resets)", n-resets)
			}
			if board == "mc13le0" {
				wantStatus(t, p.Do(req(sim, "POST", "/api/v1/power", map[string]any{"action": "cycle"}, nil)), 200)
			}
		})
	}
}

// TestGigabyteVendorRoutes: the read routes, the setFanMode write-tier
// route (no disruptive gate; unknown profiles refused before the BMC).
// Bonding routes are owned by a separate change and not exercised here.
func TestGigabyteVendorRoutes(t *testing.T) {
	tiers := map[string]bmc.Tier{}
	for _, rt := range gigabyte.Default.VendorRoutes() {
		tiers[rt.Method+" "+rt.Path] = rt.Tier
	}
	for k, want := range map[string]bmc.Tier{"GET fanProfiles": bmc.TierRead, "GET fanProfileDevices": bmc.TierRead, "GET fanMode": bmc.TierRead,
		"GET firmwareInfo": bmc.TierRead, "POST setFanMode": bmc.TierWrite} {
		if tiers[k] != want {
			t.Errorf("%s tier %v want %v", k, tiers[k], want)
		}
	}
	for _, board := range gigabyteBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			var fp gigabyte.FanProfiles
			r := p.Get(sim, "/api/v1/vendor/gigabyte/fanProfiles")
			wantStatus(t, r, 200)
			r.JSON(t, &fp)
			if len(fp.Profiles) < 2 || fp.Mode == "" {
				t.Fatalf("fanProfiles: %.300s", r.Body)
			}
			var dev gigabyte.FanProfileDevices
			r = p.Get(sim, "/api/v1/vendor/gigabyte/fanProfileDevices")
			wantStatus(t, r, 200)
			r.JSON(t, &dev)
			var fm gigabyte.FanMode
			r = p.Get(sim, "/api/v1/vendor/gigabyte/fanMode")
			wantStatus(t, r, 200)
			r.JSON(t, &fm)
			if fm.Mode != "NEW_PROFILE" || fm.Source != "ami" {
				t.Fatalf("fanMode: %s", r.Body)
			}
			var fi gigabyte.BMCFirmwareInfo
			r = p.Get(sim, "/api/v1/vendor/gigabyte/firmwareInfo")
			wantStatus(t, r, 200)
			r.JSON(t, &fi)
			if fi.Version != sim.Profile().Firmware || fi.ActiveImage != 1 || fi.ManufacturerID != 15370 {
				t.Fatalf("firmwareInfo: %s", r.Body)
			}

			// setFanMode: write tier.
			posts := sim.Count("POST", "/api/settings/fanprofile/mode")
			r = p.Do(req(sim, "POST", "/api/v1/vendor/gigabyte/setFanMode", map[string]any{"mode": "no-such-profile"}, nil))
			wantStatus(t, r, 400)
			if sim.Count("POST", "/api/settings/fanprofile/mode") != posts {
				t.Fatal("unknown profile reached the BMC")
			}
			r = p.Do(req(sim, "POST", "/api/v1/vendor/gigabyte/setFanMode", map[string]any{"mode": "default"}, nil))
			wantStatus(t, r, 200)
			fm = gigabyte.FanMode{}
			r.JSON(t, &fm)
			if fm.Mode != "default" || sim.OEMObject("/api/settings/fanprofile/mode")["strMode"] != "default" {
				t.Fatalf("setFanMode: %s", r.Body)
			}
			if st := sim.Stats(); st.Logins != 1 || st.CSRFRejects != 0 {
				t.Fatalf("logins %d CSRF rejects %d", st.Logins, st.CSRFRejects)
			}
		})
	}
}

// gigabyteDenyPaths turns every Gigabyte deny rule (plus the AMI
// defaults) into a raw path that the rule matches.
func gigabyteDenyPaths() []string {
	var out []string
	for _, r := range append(append([]amimegarac.DenyRule(nil), amimegarac.DefaultDenyRules...), boards.AllExtraDeny()...) {
		out = append(out, strings.Trim(r.Pattern, "/")+"/x")
	}
	return out
}

// TestGigabyteRawDenylist: the union of the Gigabyte deny lists is refused
// on raw/api and raw/redfish and never reaches the simulator; ordinary raw
// reads pass through on the proxy's sessions.
func TestGigabyteRawDenylist(t *testing.T) {
	for _, board := range gigabyteBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			r := p.Do(req(sim, "GET", "/api/v1/vendor/gigabyte/raw/api/firmware-info", nil, nil))
			wantStatus(t, r, 200)
			if !strings.Contains(string(r.Body), sim.Profile().Firmware) {
				t.Fatalf("raw/api firmware-info: %s", r.Body)
			}
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/vendor/gigabyte/raw/redfish/Systems/Self", nil, nil)), 200)
			paths := gigabyteDenyPaths()
			for _, dp := range paths {
				for _, pfx := range []string{"/api/v1/vendor/gigabyte/raw/api/", "/api/v1/vendor/gigabyte/raw/redfish/"} {
					if r := p.Do(req(sim, "GET", pfx+dp, nil, nil)); r.Status != http.StatusForbidden {
						t.Errorf("%s%s: %d %s", pfx, dp, r.Status, r.Body)
					}
				}
			}
			for _, rq := range sim.Requests() {
				lp := strings.ToLower(rq.Path)
				for _, dp := range paths {
					if strings.Contains(lp, strings.TrimSuffix(dp, "/x")) {
						t.Errorf("denylisted %s reached the sim", rq.Path)
					}
				}
			}
			if st := sim.Stats(); st.Restarts != 0 {
				t.Fatalf("backend restarted %d times", st.Restarts)
			}
		})
	}
}

// TestGigabyteConcurrencyOneLogin: N concurrent mixed calls (Redfish and
// /api-backed vendor routes) -> one session, one /api login, never more
// than one in-flight /api request.
func TestGigabyteConcurrencyOneLogin(t *testing.T) {
	for _, board := range gigabyteBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil, func(o *testsim.Options) { o.Latency = 2 * time.Millisecond })
			paths := []string{"/api/v1/power", "/api/v1/sensors", "/api/v1/vendor/gigabyte/fanMode", "/api/v1/vendor/gigabyte/firmwareInfo",
				"/api/v1/events", "/api/v1/system", "/api/v1/time"}
			var wg sync.WaitGroup
			const n = 40
			errs := make(chan error, n)
			for i := 0; i < n; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					r, err := p.Try(req(sim, "GET", paths[i%len(paths)], nil, nil))
					if err != nil || r.Status != 200 {
						errs <- fmt.Errorf("%s: %v %v", paths[i%len(paths)], err, r)
					}
				}(i)
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				t.Error(err)
			}
			st := sim.Stats()
			if st.Logins != 1 || p.Sessions.Counters().Opens != 1 {
				t.Fatalf("logins %d opens %d for %d calls, want 1", st.Logins, p.Sessions.Counters().Opens, n)
			}
			if st.MaxInFlightAPI > 1 || st.CSRFRejects != 0 {
				t.Fatalf("max in-flight /api %d, CSRF rejects %d", st.MaxInFlightAPI, st.CSRFRejects)
			}
		})
	}
}

// TestGigabyteGracefulShutdownLogsOut: shutdown logs out the lazily opened
// /api session on every Gigabyte BMC.
func TestGigabyteGracefulShutdownLogsOut(t *testing.T) {
	var sims []*testsim.Sim
	for _, board := range gigabyteBoards {
		sims = append(sims, startSim(t, board))
	}
	p := testsim.StartProxy(t, testsim.ProxyOptions{Drivers: drivers(&fakeIPMI{})})
	for _, sim := range sims {
		wantStatus(t, p.Get(sim, "/api/v1/vendor/gigabyte/fanMode"), 200)
		if sim.APISessions() != 1 {
			t.Fatalf("%v: %d /api sessions", sim, sim.APISessions())
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	for _, sim := range sims {
		if sim.LiveSessions() != 0 || sim.Stats().Logouts != 1 {
			t.Fatalf("%v: live %d logouts %d after shutdown", sim, sim.LiveSessions(), sim.Stats().Logouts)
		}
	}
}

// TestGigabyteMethodOverrideNeverReachesBMC: override headers are stripped
// on generic, vendor and raw routes (an honoured override would turn a GET
// into a DELETE of the /api session on these boards).
func TestGigabyteMethodOverrideNeverReachesBMC(t *testing.T) {
	for _, board := range gigabyteBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			h := map[string]string{"X-HTTP-Method-Override": "DELETE", "X-HTTP-Method": "DELETE", "X-Method-Override": "DELETE"}
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/power", nil, h)), 200)
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/vendor/gigabyte/fanMode", nil, h)), 200)
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/vendor/gigabyte/raw/api/chassis-status", nil, h)), 200)
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/vendor/gigabyte/raw/redfish/Systems/Self", nil, h)), 200)
			if n := sim.Stats().MethodOverrides; n != 0 {
				t.Fatalf("%d requests reached the BMC with a method override", n)
			}
			if sim.APISessions() != 1 {
				t.Fatal("session was logged out by an override")
			}
		})
	}
}
