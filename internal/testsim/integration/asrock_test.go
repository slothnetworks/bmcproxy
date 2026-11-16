// Package integration_test runs the full bmcproxy (server, sessions,
// cache, /api/v1, vendor routes, health, metrics, transport) with the
// production driver set (vendors/asrock + vendors/generic/redfish) against
// the HAR-replay simulators of both boards (internal/testsim), covering the
// PLAN "Verification" list. No hardware is contacted.
package integration_test

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/testsim"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/ipmi"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

var bothBoards = []string{"x470", "b650"}

func wantStatus(t *testing.T, r *testsim.Resp, want int) {
	t.Helper()
	if r.Status != want {
		t.Fatalf("status %d want %d: %s", r.Status, want, r.Body)
	}
}

// req builds a request against sim with its credentials.
func req(sim *testsim.Sim, method, path string, body any, hdr map[string]string) testsim.Req {
	c := sim.Creds()
	return testsim.Req{Method: method, Path: path, Host: sim.Host(), User: c.User, Pass: c.Pass, Body: body, Header: hdr}
}

func capMap(t *testing.T, r *testsim.Resp) (v1.CapabilitiesResponse, map[bmc.Capability]bmc.CapabilityReport) {
	t.Helper()
	var caps v1.CapabilitiesResponse
	r.JSON(t, &caps)
	m := map[bmc.Capability]bmc.CapabilityReport{}
	for _, c := range caps.Capabilities {
		m[c.Capability] = c
	}
	return caps, m
}

// TestReadsBothBoards: capabilities, system, power, sensors, events
// (incremental), audit, fru, inventory, network, users, services, time and
// firmware on both boards, all through the asrock driver.
func TestReadsBothBoards(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)

			r := p.Get(sim, "/api/v1/capabilities")
			wantStatus(t, r, 200)
			caps, cm := capMap(t, r)
			wantBoard := map[string]string{"x470": "X470D4U", "b650": "B650D4U"}[board]
			if caps.Driver != "asrock" || caps.Board != wantBoard {
				t.Fatalf("capabilities: driver %q board %q", caps.Driver, caps.Board)
			}
			for _, c := range []bmc.Capability{bmc.CapIdentity, bmc.CapPower, bmc.CapSensors, bmc.CapEventLog, bmc.CapFRU,
				bmc.CapInventory, bmc.CapNetwork, bmc.CapUsers, bmc.CapServices, bmc.CapTime, bmc.CapFirmwareInventory,
				bmc.CapIndicatorLED, bmc.CapBond, bmc.CapBootOverride, bmc.CapPowerRestorePolicy} {
				if !cm[c].Supported {
					t.Errorf("%s: %s not supported", board, c)
				}
			}

			var sys bmc.SystemInfo
			r = p.Get(sim, "/api/v1/system")
			wantStatus(t, r, 200)
			r.JSON(t, &sys)
			if sys.Board != wantBoard || sys.BMCFirmware == "" || sys.BIOSVersion == "" || sys.PowerState != bmc.PowerOn {
				t.Fatalf("system: %+v", sys)
			}

			var ps v1.PowerStateResponse
			p.Get(sim, "/api/v1/power").JSON(t, &ps)
			if ps.State != bmc.PowerOn {
				t.Fatalf("power %v", ps.State)
			}

			var sens v1.SensorsResponse
			r = p.Get(sim, "/api/v1/sensors")
			wantStatus(t, r, 200)
			r.JSON(t, &sens)
			if len(sens.Sensors) == 0 {
				t.Fatal("no sensors")
			}
			if board == "b650" && len(sens.Sensors) != 60 {
				t.Fatalf("B650 sensors = %d, want all 60 (two Redfish pages)", len(sens.Sensors))
			}

			// Events, then incremental since=lastId.
			var ev v1.EventsResponse
			r = p.Get(sim, "/api/v1/events")
			wantStatus(t, r, 200)
			r.JSON(t, &ev)
			if len(ev.Entries) == 0 || ev.LastID == "" {
				t.Fatalf("events: %s", r.Body)
			}
			var newID string
			if board == "x470" {
				newID = fmt.Sprint(sim.AddEvent("CPU Temp", "Upper Critical going high"))
			} else {
				newID = sim.Redfish().AddSELEntry("CPU Temp upper critical", "Critical")
			}
			var inc v1.EventsResponse
			r = p.Get(sim, "/api/v1/events?since="+ev.LastID)
			wantStatus(t, r, 200)
			r.JSON(t, &inc)
			if len(inc.Entries) != 1 || inc.Entries[0].ID != newID {
				t.Fatalf("incremental: %s (want only %s)", r.Body, newID)
			}
			if board == "x470" && sim.Count("GET", "/api/logs/event") == 0 {
				t.Fatal("X470 incremental read did not use logs/event?LASTEVENTID")
			}

			var audit v1.AuditResponse
			r = p.Get(sim, "/api/v1/logs/audit")
			wantStatus(t, r, 200)
			r.JSON(t, &audit)
			if len(audit.Entries) == 0 {
				t.Fatal("audit log empty")
			}

			var fru v1.FRUResponse
			r = p.Get(sim, "/api/v1/fru")
			wantStatus(t, r, 200)
			r.JSON(t, &fru)
			if len(fru.FRU) == 0 || fru.FRU[0].Board.Product != wantBoard {
				t.Fatalf("fru: %s", r.Body)
			}

			for _, kind := range []string{"cpus", "memory", "storage", "baseboard"} {
				wantStatus(t, p.Get(sim, "/api/v1/inventory/"+kind), 200)
			}
			if board == "x470" {
				wantStatus(t, p.Get(sim, "/api/v1/inventory/pcie"), 200)
			}

			var nw v1.NetworkResponse
			r = p.Get(sim, "/api/v1/network")
			wantStatus(t, r, 200)
			r.JSON(t, &nw)
			if len(nw.Interfaces) == 0 {
				t.Fatal("no interfaces")
			}
			var users v1.UsersResponse
			r = p.Get(sim, "/api/v1/users")
			wantStatus(t, r, 200)
			r.JSON(t, &users)
			if len(users.Users) == 0 || strings.Contains(string(r.Body), "assword\":\"") {
				t.Fatalf("users: %s", r.Body)
			}
			var svc v1.ServicesResponse
			r = p.Get(sim, "/api/v1/services")
			wantStatus(t, r, 200)
			r.JSON(t, &svc)
			if len(svc.Services) == 0 {
				t.Fatal("no services")
			}
			var tc bmc.TimeConfig
			r = p.Get(sim, "/api/v1/time")
			wantStatus(t, r, 200)
			r.JSON(t, &tc)
			if tc.BMCTime.IsZero() || len(tc.NTPServers) == 0 {
				t.Fatalf("time: %s", r.Body)
			}
			var fw v1.FirmwareResponse
			r = p.Get(sim, "/api/v1/firmware")
			wantStatus(t, r, 200)
			r.JSON(t, &fw)
			found := false
			for _, c := range fw.Components {
				if strings.EqualFold(c.ID, "bmc") && c.Version == sim.Profile().Firmware {
					found = true
				}
			}
			if !found {
				t.Fatalf("firmware lacks BMC %s: %s", sim.Profile().Firmware, r.Body)
			}

			// Everything above rode one /api login.
			if sim.Logins() != 1 {
				t.Fatalf("%d logins for the whole read sweep, want 1", sim.Logins())
			}
		})
	}
}

// TestCapabilityBackends: the B650 serves its Redfish-backed capabilities
// from Redfish and ASRock OEM gaps from /api; the X470 is /api + IPMI.
func TestCapabilityBackends(t *testing.T) {
	sim, p, _ := setupBoard(t, "b650", nil)
	_, cm := capMap(t, p.Get(sim, "/api/v1/capabilities"))
	for c, want := range map[bmc.Capability]string{
		bmc.CapIdentity: "redfish", bmc.CapPower: "redfish", bmc.CapSensors: "redfish", bmc.CapEventLog: "redfish",
		bmc.CapIndicatorLED: "redfish", bmc.CapBootOverride: "redfish", bmc.CapNetwork: "redfish", bmc.CapUsers: "redfish",
		bmc.CapTime: "redfish", bmc.CapFirmwareInventory: "redfish",
		bmc.CapFRU: "ami", bmc.CapBond: "ami", bmc.CapPowerRestorePolicy: "ami",
	} {
		if cm[c].Backend != want {
			t.Errorf("B650 %s backend %q want %q", c, cm[c].Backend, want)
		}
	}

	xsim, xp, _ := setupBoard(t, "x470", nil)
	_, xm := capMap(t, xp.Get(xsim, "/api/v1/capabilities"))
	for c, want := range map[bmc.Capability]string{
		bmc.CapPower: "ami", bmc.CapSensors: "ami", bmc.CapBootOverride: "ipmi", bmc.CapPowerRestorePolicy: "ipmi",
	} {
		if xm[c].Backend != want {
			t.Errorf("X470 %s backend %q want %q", c, xm[c].Backend, want)
		}
	}
	// The X470 Redfish (admin rejected) is never touched by the driver.
	if n := xsim.Count("", "/redfish/v1/Systems"); n != 0 {
		t.Fatalf("X470 Redfish below the root was queried %d times", n)
	}
}

// TestX470IPMICapabilities: boot override and restore policy go through
// the injected IPMI dialer.
func TestX470IPMICapabilities(t *testing.T) {
	sim, p, ip := setupBoard(t, "x470", nil)
	wantStatus(t, p.Do(req(sim, "PUT", "/api/v1/boot", map[string]any{"target": "pxe", "mode": "once", "bootMode": "uefi"}, nil)), 204)
	var boot bmc.BootOverrideConfig
	r := p.Get(sim, "/api/v1/boot")
	wantStatus(t, r, 200)
	r.JSON(t, &boot)
	if boot.Target != bmc.BootPXE || boot.Mode != bmc.BootOnce {
		t.Fatalf("boot: %s", r.Body)
	}
	wantStatus(t, p.Do(req(sim, "PUT", "/api/v1/power/restore-policy", map[string]any{"policy": "alwaysOn"}, nil)), 204)
	var rp v1.RestorePolicyBody
	p.Get(sim, "/api/v1/power/restore-policy").JSON(t, &rp)
	ip.mu.Lock()
	dials, restore, dev := ip.dials, ip.restore, ip.boot.Device
	ip.mu.Unlock()
	if rp.Policy != bmc.RestoreAlwaysOn || restore != ipmi.RestoreAlwaysOn || dev != ipmi.BootPXE {
		t.Fatalf("ipmi state: policy %v restore %v boot %v", rp.Policy, restore, dev)
	}
	if dials != 1 {
		t.Fatalf("IPMI dialed %d times, want 1 (lazy, reused)", dials)
	}
}

// TestB650RestorePolicyASRR: B650 restore policy over asrr/{Get,Set}PowerRestore.
func TestB650RestorePolicyASRR(t *testing.T) {
	sim, p, ip := setupBoard(t, "b650", nil)
	wantStatus(t, p.Do(req(sim, "PUT", "/api/v1/power/restore-policy", map[string]any{"policy": "alwaysOff"}, nil)), 204)
	if v := sim.OEMObject("/api/asrr/GetPowerRestore")["powerRestore"]; fmt.Sprint(v) != "0" {
		t.Fatalf("powerRestore %v want 0", v)
	}
	var rp v1.RestorePolicyBody
	p.Get(sim, "/api/v1/power/restore-policy").JSON(t, &rp)
	if rp.Policy != bmc.RestoreAlwaysOff || ip.dials != 0 {
		t.Fatalf("policy %v, ipmi dials %d", rp.Policy, ip.dials)
	}
}

func TestLEDRoundTrip(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			for _, st := range []bmc.LEDState{bmc.LEDOn, bmc.LEDOff} {
				wantStatus(t, p.Do(req(sim, "POST", "/api/v1/led", map[string]any{"state": st}, nil)), 204)
				var led v1.LEDBody
				r := p.Get(sim, "/api/v1/led")
				wantStatus(t, r, 200)
				r.JSON(t, &led)
				if led.State != st {
					t.Fatalf("led %v want %v", led.State, st)
				}
			}
			if board == "x470" {
				// uid_command 2 (on) then 0 (off), as confirmed live.
				var cmds []string
				for _, rq := range sim.Requests() {
					if rq.Path == "/api/actions/chassis-led" {
						cmds = append(cmds, strings.ReplaceAll(rq.Body, " ", ""))
					}
				}
				if strings.Join(cmds, ",") != `{"uid_command":2},{"uid_command":0}` {
					t.Fatalf("chassis-led bodies %v", cmds)
				}
			}
		})
	}
}

// TestPowerActionsPolling: on/off converge after polling; a soft-off the
// host ignores (mid-POST) yields 202 + warning, not a false success.
func TestPowerActionsPolling(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil, func(o *testsim.Options) { o.PowerDelay = 30 * time.Millisecond })
			post := func(body string) *testsim.Resp { return p.Do(req(sim, "POST", "/api/v1/power", body, nil)) }
			var res v1.PowerActionResult
			r := post(`{"action":"off"}`)
			wantStatus(t, r, 200)
			r.JSON(t, &res)
			if res.State != bmc.PowerOff || !res.Converged || !res.Polled || res.Polls < 2 || sim.PowerOn() {
				t.Fatalf("off: %s", r.Body)
			}
			r = post(`{"action":"on"}`)
			wantStatus(t, r, 200)
			sim.SetPowerIgnoresSoftOff(true)
			r = post(`{"action":"shutdown","timeoutSeconds":0.2}`)
			wantStatus(t, r, 202)
			res = v1.PowerActionResult{}
			r.JSON(t, &res)
			if res.Warning == "" || res.State != bmc.PowerOn || res.Converged || sim.Stats().IgnoredSoftOffs != 1 {
				t.Fatalf("ignored soft-off: %s", r.Body)
			}
			if board == "x470" {
				var codes []string
				for _, rq := range sim.Requests() {
					if rq.Path == "/api/actions/power" {
						codes = append(codes, strings.ReplaceAll(rq.Body, " ", ""))
					}
				}
				if strings.Join(codes, ",") != `{"power_command":0},{"power_command":1},{"power_command":5}` {
					t.Fatalf("power_command sequence %v", codes)
				}
			}
		})
	}
}

// TestTimeSyncFullObject: X470 PUT settings/date-time carries the full
// writable object (never a partial body: DateTime500s == 0) and fixes skew.
func TestTimeSyncFullObject(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil, func(o *testsim.Options) { o.ClockOffset = -2 * time.Hour })
			var tc bmc.TimeConfig
			r := p.Get(sim, "/api/v1/time")
			wantStatus(t, r, 200)
			r.JSON(t, &tc)
			if math.Abs(tc.OffsetSeconds+7200) > 5 {
				t.Fatalf("offset %v want ~-7200: %s", tc.OffsetSeconds, r.Body)
			}
			r = p.Do(req(sim, "POST", "/api/v1/time/sync", nil, nil))
			wantStatus(t, r, 200)
			if off := sim.ClockOffset(); off < -5*time.Second || off > 5*time.Second {
				t.Fatalf("BMC clock offset after sync: %v", off)
			}
			if board == "x470" {
				wantStatus(t, p.Do(req(sim, "PUT", "/api/v1/time", map[string]any{"ntpServers": []string{"ntp1.example", "ntp2.example"}}, nil)), 200)
				puts := 0
				for _, rq := range sim.Requests() {
					if rq.Method == "PUT" && rq.Path == "/api/settings/date-time" {
						puts++
						for _, k := range []string{`"timezone"`, `"utc_minutes"`, `"auto_date"`, `"daylight"`, `"timestamp"`, `"primary_ntp"`} {
							if !strings.Contains(rq.Body, k) {
								t.Fatalf("date-time PUT lacks %s: %s", k, rq.Body)
							}
						}
						if strings.Contains(rq.Body, `"id"`) || strings.Contains(rq.Body, "localized_timestamp") {
							t.Fatalf("date-time PUT carries response-only fields: %s", rq.Body)
						}
					}
				}
				if puts != 2 {
					t.Fatalf("%d date-time PUTs, want 2", puts)
				}
			}
			if n := sim.Stats().DateTime500s; n != 0 {
				t.Fatalf("%d partial date-time PUTs (500)", n)
			}
		})
	}
}

// TestClockSkewNormalization: SEL timestamps from a BMC whose clock is 2 h
// behind are corrected to real UTC, both raw and corrected exposed; the
// skew surfaces as a capabilities warning.
func TestClockSkewNormalization(t *testing.T) {
	sim, p, _ := setupBoard(t, "x470", nil, func(o *testsim.Options) { o.ClockOffset = -2 * time.Hour })
	var ev v1.EventsResponse
	p.Get(sim, "/api/v1/events").JSON(t, &ev)
	id := fmt.Sprint(sim.AddEvent("CPU Temp", "Upper Critical going high"))
	var inc v1.EventsResponse
	r := p.Get(sim, "/api/v1/events?since="+ev.LastID)
	wantStatus(t, r, 200)
	r.JSON(t, &inc)
	if len(inc.Entries) != 1 || inc.Entries[0].ID != id {
		t.Fatalf("events: %s", r.Body)
	}
	e := inc.Entries[0]
	now := time.Now()
	if d := now.Sub(e.Time.Corrected); d < -time.Minute || d > time.Minute {
		t.Fatalf("corrected %v not near now %v", e.Time.Corrected, now)
	}
	if d := e.Time.Corrected.Sub(e.Time.Raw); d < 2*time.Hour-time.Minute || d > 2*time.Hour+time.Minute {
		t.Fatalf("raw %v corrected %v: want ~2h apart", e.Time.Raw, e.Time.Corrected)
	}
	if inc.ClockOffsetSeconds == nil || math.Abs(*inc.ClockOffsetSeconds+7200) > 5 {
		t.Fatalf("clockOffsetSeconds %v", inc.ClockOffsetSeconds)
	}
	caps, _ := capMap(t, p.Get(sim, "/api/v1/capabilities"))
	if len(caps.Warnings) == 0 || !strings.Contains(strings.Join(caps.Warnings, " "), "behind") {
		t.Fatalf("no skew warning: %v", caps.Warnings)
	}
}

// TestVendorRoutes: bonding, fan mode, keep-NIC-link-up on both boards.
func TestVendorRoutes(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			var bond oem.BondingStatus
			r := p.Get(sim, "/api/v1/vendor/asrock/bonding")
			wantStatus(t, r, 200)
			r.JSON(t, &bond)
			if bond.Mode != "active-backup" || bond.ID != 1 {
				t.Fatalf("bonding: %s", r.Body)
			}

			var fm oem.FanModeResponse
			if board == "x470" {
				r = p.Do(req(sim, "POST", "/api/v1/vendor/asrock/setFanMode", map[string]any{"modes": map[string]int{"3": 1}}, nil))
				wantStatus(t, r, 200)
				r.JSON(t, &fm)
				if fm.Dialect != "asrr" || fm.Modes["3"] != 1 || fm.Modes["1"] != 0 || len(fm.Modes) != 16 {
					t.Fatalf("setFanMode: %s", r.Body)
				}
				// The whole mode1..16 object was POSTed (read-merge-write).
				body := lastBody(sim, "POST", "/api/asrr/settings/setfancontrolmode")
				if !strings.Contains(body, `"mode16":0`) || !strings.Contains(body, `"mode3":1`) {
					t.Fatalf("setfancontrolmode body %s", body)
				}
			} else {
				r = p.Do(req(sim, "POST", "/api/v1/vendor/asrock/setFanMode", map[string]any{"raw": map[string]any{"min_duty": 30}}, nil))
				wantStatus(t, r, 200)
				r.JSON(t, &fm)
				if fm.Dialect != "settings/fan" || fmt.Sprint(sim.OEMObject("/api/settings/fan/mode")["min_duty"]) != "30" {
					t.Fatalf("setFanMode: %s", r.Body)
				}
				if body := lastBody(sim, "PUT", "/api/settings/fan/mode"); !strings.Contains(body, `"platform":"AMD_platform"`) {
					t.Fatalf("settings/fan/mode PUT is not the full object: %s", body)
				}
			}

			var ku oem.KeepNicLinkUp
			r = p.Do(req(sim, "POST", "/api/v1/vendor/asrock/setKeepNicLinkUp", map[string]any{"enabled": true}, nil))
			wantStatus(t, r, 200)
			r.JSON(t, &ku)
			if !ku.Enabled {
				t.Fatalf("setKeepNicLinkUp: %s", r.Body)
			}
			if board == "x470" {
				if ku.Dialect != "vetobit" || fmt.Sprint(sim.OEMObject("/api/asrr/getvetobit")["vetobit"]) != "1" {
					t.Fatalf("vetobit: %s", r.Body)
				}
			} else {
				o := sim.OEMObject("/api/settings/ncsilinkup")
				if ku.Dialect != "ncsilinkup" || fmt.Sprint(o["sharednic"]) != "1" || fmt.Sprint(o["sharednic2"]) != "1" {
					t.Fatalf("ncsilinkup: %s %v", r.Body, o)
				}
			}
			r = p.Get(sim, "/api/v1/vendor/asrock/keepNicLinkUp")
			wantStatus(t, r, 200)
		})
	}
}

func lastBody(sim *testsim.Sim, method, path string) string {
	b := ""
	for _, rq := range sim.Requests() {
		if rq.Method == method && rq.Path == path {
			b = strings.ReplaceAll(rq.Body, " ", "")
		}
	}
	return b
}

// TestX470FanTablesTwoStepUnderLoad: concurrent fanTables requests (plus
// unrelated reads) never interleave the X470 select+read pair, and each
// caller gets the table it asked for.
func TestX470FanTablesTwoStepUnderLoad(t *testing.T) {
	sim, p, _ := setupBoard(t, "x470", nil, func(o *testsim.Options) { o.Latency = time.Millisecond })
	wantStatus(t, p.Get(sim, "/api/v1/power"), 200) // open the session
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 3; i++ {
		for n := 2; n <= 8; n++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				r, err := p.Try(req(sim, "GET", fmt.Sprintf("/api/v1/vendor/asrock/fanTables?type=open&table=%d", n), nil, nil))
				if err != nil || r.Status != 200 {
					errs <- fmt.Errorf("table %d: %v %v", n, err, r)
					return
				}
				var ft oem.FanTablesResponse
				if err := jsonUnmarshal(r.Body, &ft); err != nil || len(ft.Tables) != 1 || len(ft.Tables[0].Duties) != 24 {
					errs <- fmt.Errorf("table %d: %v %s", n, err, r.Body)
					return
				}
				if got := ft.Tables[0].Duties[0]; got != n*10+1 {
					errs <- fmt.Errorf("asked for table %d, got table %d's data (interleaved)", n, (got-1)/10)
				}
			}(n)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r, err := p.Try(req(sim, "GET", "/api/v1/sensors", nil, nil)); err != nil || r.Status != 200 {
				errs <- fmt.Errorf("sensors: %v %v", err, r)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	st := sim.Stats()
	if st.FanInterleaves != 0 || st.MaxInFlightAPI > 1 {
		t.Fatalf("fan interleaves %d, max in-flight /api %d", st.FanInterleaves, st.MaxInFlightAPI)
	}
	// Out-of-range tables are refused before touching the BMC (UI: 1..8).
	before := sim.Count("POST", "/api/asrr/settings/setopenlooptablenumber")
	wantStatus(t, p.Do(req(sim, "GET", "/api/v1/vendor/asrock/fanTables?type=open&table=9", nil, nil)), 400)
	if sim.Count("POST", "/api/asrr/settings/setopenlooptablenumber") != before {
		t.Fatal("out-of-range table reached the BMC")
	}
}

// TestRawPassthroughAndDenylist: raw/api works with the proxy's session;
// crash-prone and leaky paths are 403 and never reach the BMC.
func TestRawPassthroughAndDenylist(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			r := p.Do(req(sim, "GET", "/api/v1/vendor/asrock/raw/api/asrr/fw-info", nil, nil))
			wantStatus(t, r, 200)
			if !strings.Contains(string(r.Body), sim.Profile().Firmware) {
				t.Fatalf("raw fw-info: %s", r.Body)
			}
			for _, path := range []string{
				"/api/v1/vendor/asrock/raw/api/logs/help-data",
				"/api/v1/vendor/asrock/raw/api/logs/HELP-DATA",
				"/api/v1/vendor/asrock/raw/api/logs/./help-data",
				"/api/v1/vendor/asrock/raw/api/asrr/mca-file",
				"/api/v1/vendor/asrock/raw/api/GlobalLuaTables",
				"/api/v1/vendor/asrock/raw/redfish/GlobalLuaTables",
			} {
				r := p.Do(req(sim, "GET", path, nil, nil))
				if r.Status != http.StatusForbidden && !(board == "x470" && strings.Contains(path, "raw/redfish") && r.Status == 501) {
					t.Errorf("%s: %d %s", path, r.Status, r.Body)
				}
			}
			for _, pfx := range []string{"/api/logs/help-data", "/api/asrr/mca-file", "/redfish/v1/GlobalLuaTables", "/api/GlobalLuaTables"} {
				if n := sim.Count("", pfx); n != 0 {
					t.Errorf("%s reached the sim %d times", pfx, n)
				}
			}
			if st := sim.Stats(); st.Restarts != 0 {
				t.Fatalf("backend restarted %d times", st.Restarts)
			}
		})
	}
}

// TestDisruptiveGating: disableBonding needs --allow-disruptive AND
// X-BMC-Confirm: true; with both it reaches the sim (only the sim).
func TestDisruptiveGating(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			// Without --allow-disruptive: 403 even with the confirm header.
			sim, p, _ := setupBoard(t, board, nil)
			confirm := map[string]string{"X-BMC-Confirm": "true"}
			for _, h := range []map[string]string{nil, confirm} {
				r := p.Do(req(sim, "POST", "/api/v1/vendor/asrock/disableBonding", map[string]any{"interface": "eth0"}, h))
				wantStatus(t, r, 403)
				if e := r.Envelope(t); e.Error.Class != "disruptive_denied" {
					t.Fatalf("class %q", e.Error.Class)
				}
			}
			if n := sim.Count("PUT", "/api/settings/network-bond"); n != 0 {
				t.Fatalf("denied request reached the BMC %d times", n)
			}

			// With --allow-disruptive: still 403 without the header.
			sim2, p2, _ := setupBoard(t, board, func(o *testsim.ProxyOptions) { o.AllowDisruptive = true })
			wantStatus(t, p2.Do(req(sim2, "POST", "/api/v1/vendor/asrock/disableBonding", map[string]any{"interface": "eth0"}, nil)), 403)
			wantStatus(t, p2.Do(req(sim2, "DELETE", "/api/v1/events", nil, nil)), 403)
			r := p2.Do(req(sim2, "POST", "/api/v1/vendor/asrock/disableBonding", map[string]any{"interface": "eth0"}, confirm))
			wantStatus(t, r, 200)
			var bond oem.BondingStatus
			r.JSON(t, &bond)
			if bond.Enabled {
				t.Fatalf("still bonded: %s", r.Body)
			}
			body := lastBody(sim2, "PUT", "/api/settings/network-bond")
			for _, k := range []string{`"id":1`, `"bond_enable":0`, `"bond_mode":"active-backup"`, `"auto_configuration_enable":1`} {
				if !strings.Contains(body, k) {
					t.Fatalf("network-bond PUT lacks %s: %s", k, body)
				}
			}
			if hasIfc := strings.Contains(body, `"bond_ifc":"eth0"`); hasIfc != (board == "b650") {
				t.Fatalf("bond_ifc on %s: %s", board, body)
			}
			if fmt.Sprint(sim2.OEMObject("/api/settings/network-bond")["bond_enable"]) != "0" {
				t.Fatal("sim bond not disabled")
			}
		})
	}
}

// TestConcurrencyOneLoginSerialisedAPI: N concurrent mixed calls → one
// login, never more than one in-flight /api request per host.
func TestConcurrencyOneLoginSerialisedAPI(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil, func(o *testsim.Options) { o.Latency = 2 * time.Millisecond })
			paths := []string{"/api/v1/power", "/api/v1/sensors", "/api/v1/fru", "/api/v1/vendor/asrock/bonding",
				"/api/v1/events", "/api/v1/vendor/asrock/keepNicLinkUp", "/api/v1/system", "/api/v1/vendor/asrock/fanMode"}
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
			if st.MaxInFlightAPI > 1 {
				t.Fatalf("max in-flight /api = %d, want <= 1", st.MaxInFlightAPI)
			}
			if board == "b650" && st.Evasive403s != 0 {
				t.Fatalf("tripped mod_evasive %d times", st.Evasive403s)
			}
		})
	}
}

// TestBackendRestartSessionLost: a backend restart (Restart + 502) makes
// the driver return ErrSessionLost; the session manager invalidates the
// host, logs in once more and the request succeeds.
func TestBackendRestartSessionLost(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			wantStatus(t, p.Get(sim, "/api/v1/fru"), 200) // FRU is /api on both boards
			logins := sim.Logins()
			var once sync.Once
			sim.Handle("GET /api/fru", func(w http.ResponseWriter, r *http.Request) {
				hit := false
				once.Do(func() { hit = true })
				if !hit {
					sim.Handle("GET /api/fru", nil)
					return
				}
				sim.Restart()
				w.Header().Set("Content-Type", "text/html")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte("<html><body><h1>502 - Bad Gateway</h1></body></html>"))
			})
			r := p.Get(sim, "/api/v1/fru")
			sim.Handle("GET /api/fru", nil)
			// The retry may have hit the handler's pass-through branch
			// (empty 200) or the fixture: either way the proxy must not
			// surface the restart.
			if r.Status != 200 {
				t.Fatalf("after restart: %d %s", r.Status, r.Body)
			}
			if got := sim.Logins() - logins; got != 1 {
				t.Fatalf("%d relogins after restart, want exactly 1", got)
			}
			if c := p.Sessions.Counters(); c.Relogins != 1 {
				t.Fatalf("session manager relogins %d", c.Relogins)
			}
			wantStatus(t, p.Get(sim, "/api/v1/fru"), 200)
			if sim.Logins()-logins != 1 {
				t.Fatal("extra login after recovery")
			}
		})
	}
}

// TestGracefulShutdownLogsOut: shutdown logs out every cached session on
// every BMC.
func TestGracefulShutdownLogsOut(t *testing.T) {
	x := startSim(t, "x470")
	b := startSim(t, "b650")
	p := testsim.StartProxy(t, testsim.ProxyOptions{Drivers: drivers(&fakeIPMI{})})
	for _, sim := range []*testsim.Sim{x, b} {
		wantStatus(t, p.Get(sim, "/api/v1/fru"), 200)
		if sim.APISessions() != 1 {
			t.Fatalf("%v: %d /api sessions", sim, sim.APISessions())
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	for _, sim := range []*testsim.Sim{x, b} {
		if sim.LiveSessions() != 0 || sim.Stats().Logouts != 1 {
			t.Fatalf("%v: live %d logouts %d after shutdown", sim, sim.LiveSessions(), sim.Stats().Logouts)
		}
	}
}

// TestMethodOverrideNeverReachesBMC: method-override headers on northbound
// requests (generic and raw passthrough) are stripped.
func TestMethodOverrideNeverReachesBMC(t *testing.T) {
	for _, board := range bothBoards {
		t.Run(board, func(t *testing.T) {
			sim, p, _ := setupBoard(t, board, nil)
			h := map[string]string{"X-HTTP-Method-Override": "DELETE", "X-HTTP-Method": "DELETE", "X-Method-Override": "DELETE"}
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/power", nil, h)), 200)
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/vendor/asrock/raw/api/chassis-status", nil, h)), 200)
			wantStatus(t, p.Do(req(sim, "GET", "/api/v1/vendor/asrock/bonding", nil, h)), 200)
			if n := sim.Stats().MethodOverrides; n != 0 {
				t.Fatalf("%d requests reached the BMC with a method override", n)
			}
			if sim.APISessions() != 1 {
				t.Fatal("session was logged out by an override")
			}
		})
	}
}
