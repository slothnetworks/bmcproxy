package asrock

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
)

// newX570 is an X570D4U 1.20.00 fake: the B650 fake (same AMI generation:
// no certlogin, settable_flag, Redfish accepting the /api admin) with the
// X570 identity and the X570 data-quality quirks from docs/x570/live-probe.md
// (asrr/fw-info and the Redfish placeholders below are the live values).
func newX570(t testing.TB) *fakeBMC {
	f := newB650(t, "")
	f.trailer = false // no event_log_count trailer on 1.20
	f.setJSON("GET /api/asrr/model-name", 200, map[string]any{"mb_name": "X570D4U", "sys_name": "N/A"})
	f.setJSON("GET /api/asrr/fw-info", 200, map[string]any{"BMC_fw_version": "1.20.00", "BIOS_fw_version": "L1.57",
		"ME_fw_version": "0.14.0.3E", "Micro_Code_version": "0a20120e", "CPLD_version": "N/A"})
	f.setJSON("GET /api/asrr/settings/getfandutymanualmode", 200, []any{map[string]any{"duty1": 35, "duty2": 40, "duty3": 0}})
	// Two SEL records in the live 1.20 shape (docs/x570/har-endpoints.json):
	// newest first, no trailer; early records carry negative timestamps.
	f.setJSON("GET /api/logs/eventlog", 200, []any{
		map[string]any{"id": 3639, "record_type": "system_event_record", "timestamp": 1721421175, "generator_type": "ipmb_slave_address",
			"sensor_type": "processor", "sensor_number": 145, "sensor_name": "CPU_PROCHOT", "event_direction": "deasserted",
			"event_reading_class": "discrete", "event_description": "state_asserted"},
		map[string]any{"id": 1, "record_type": "system_event_record", "timestamp": -17965, "generator_type": "ipmb_slave_address",
			"sensor_type": "power_supply", "sensor_number": 112, "sensor_name": "PSU1_Status", "event_direction": "asserted",
			"event_reading_class": "sensor_specific", "event_description": "presence_detected"},
	})
	f.setJSON("GET /api/asrr/GetFanDuty", 404, map[string]any{"error": "Invalid API Call", "code": 1010})
	f.rf.Lock()
	root := f.rf.Resources["/redfish/v1"]
	root["@odata.type"] = "#ServiceRoot.v1_5_2.ServiceRoot"
	root["RedfishVersion"] = "1.8.0"
	sys := f.rf.Resources["/redfish/v1/Systems/Self"]
	sys["Manufacturer"] = "To Be Filled By O.E.M."
	sys["Model"] = " "
	f.rf.Resources["/redfish/v1/UpdateService/FirmwareInventory/BIOS"]["Version"] = nil
	f.rf.Unlock()
	return f
}

func TestDetectX570(t *testing.T) {
	f := newX570(t)
	d := newTestDriver(t, nil)
	ctx := context.Background()
	m, err := d.Detect(ctx, f.target(t))
	if err != nil {
		t.Fatal(err)
	}
	if m.Score <= 10 || m.Vendor != Name {
		t.Fatalf("unauth detect: %+v", m)
	}
	if fam := d.family(ctx, f.target(t)); fam != boards.FamilyModern {
		t.Fatalf("ServiceRoot v1_5_2 / 1.8.0 family %q, want modern (no certlogin, modern TLS)", fam)
	}
	m, err = d.DetectWithCreds(ctx, f.target(t), creds)
	if err != nil {
		t.Fatal(err)
	}
	if m.Score != bmc.ScoreCertain || m.Board != "X570D4U" || m.Firmware != "1.20.00" {
		t.Fatalf("cred detect: %+v", m)
	}
	s := openSession(t, d, f)
	p := s.Profile()
	if p.Key != "x570d4u" || !p.Verified || p.Fan != boards.FanASRR || p.LinkUp != boards.LinkUpVetobit ||
		p.Restore != boards.RestoreASRR || p.EventTrailer || p.BondIfc || len(p.LoginExtra) != 0 {
		t.Fatalf("profile %+v", p)
	}
	// The x470_pam auto-enable is keyed to the X470 only.
	if familyForProfile(p) != boards.FamilyModern {
		t.Fatalf("X570 family %q", familyForProfile(p))
	}
}

func TestCapabilitiesX570(t *testing.T) {
	f := newX570(t)
	s := openSession(t, newTestDriver(t, nil), f)
	caps := s.Info().Capabilities
	want := map[bmc.Capability]string{
		bmc.CapIdentity: bmc.BackendRedfish, bmc.CapPower: bmc.BackendRedfish, bmc.CapBootOverride: bmc.BackendRedfish,
		bmc.CapSensors: bmc.BackendAMI, bmc.CapEventLog: bmc.BackendAMI, bmc.CapEventLogClear: bmc.BackendAMI,
		bmc.CapFirmwareInventory: bmc.BackendAMI, bmc.CapFRU: bmc.BackendAMI, bmc.CapPowerRestorePolicy: bmc.BackendAMI,
	}
	for c, b := range want {
		if caps[c] != b {
			t.Errorf("X570 %s served by %q, want %q (all: %v)", c, caps[c], b, caps)
		}
	}
}

// TestCapPrefsAMIFirstKeepsFallback: AMIFirst only reorders; if the /api
// login fails, the capabilities fall back to Redfish.
func TestCapPrefsAMIFirstKeepsFallback(t *testing.T) {
	prefs := capPrefs(boards.Lookup("X570D4U", "1.20.00"))
	if got := prefs[bmc.CapSensors]; len(got) != 2 || got[0] != bmc.BackendAMI || got[1] != bmc.BackendRedfish {
		t.Fatalf("sensors prefs %v", got)
	}
	if got := prefs[bmc.CapIdentity]; got[0] != bmc.BackendRedfish {
		t.Fatalf("identity prefs %v", got)
	}
	if got := capPrefs(boards.Lookup("B650D4U", "4.10.00"))[bmc.CapSensors]; got[0] != bmc.BackendRedfish {
		t.Fatalf("B650 sensors prefs %v", got)
	}
}

func TestX570IdentityAndFirmware(t *testing.T) {
	f := newX570(t)
	s := openSession(t, newTestDriver(t, nil), f)
	ctx := context.Background()
	si, err := s.SystemInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if si.Manufacturer != "ASRockRack" || si.Model != "" || si.Board != "X570D4U" || si.BMCFirmware != "1.20.00" {
		t.Fatalf("system info: %+v", si)
	}
	bb, err := s.Baseboard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bb.Manufacturer == "To Be Filled By O.E.M." {
		t.Fatalf("baseboard placeholder: %+v", bb)
	}
	fw, err := s.Firmware(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range fw {
		got[c.ID] = c.Version
	}
	if got["bmc"] != "1.20.00" || got["bios"] != "L1.57" || got["me"] != "0.14.0.3E" || got["cpld"] != "" {
		t.Fatalf("firmware %v", got)
	}
}

func TestX570FanDutyManual(t *testing.T) {
	f := newX570(t)
	s := openSession(t, newTestDriver(t, nil), f)
	fd, err := s.OEM().FanDuty(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fd.Source != "manual" || fd.Duties["1"] != 35 || fd.Duties["2"] != 40 {
		t.Fatalf("fan duty %+v", fd)
	}
	if n := len(f.seenPath(http.MethodGet, "/api/asrr/GetFanDuty")); n != 0 {
		t.Fatalf("absent asrr/GetFanDuty called %d times", n)
	}
}

func TestX570EventsNoTrailer(t *testing.T) {
	f := newX570(t)
	s := openSession(t, newTestDriver(t, nil), f)
	page, err := s.Events(context.Background(), bmc.EventQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 2 || page.LastID != "3639" || page.Entries[0].ID != "1" {
		b, _ := json.Marshal(page)
		t.Fatalf("events: %s", b)
	}
	// The negative (pre-RTC) timestamp is dropped, not reported as 1969.
	if !page.Entries[0].Time.Raw.IsZero() || page.Entries[1].Time.Raw.IsZero() {
		t.Fatalf("timestamps: %+v / %+v", page.Entries[0].Time, page.Entries[1].Time)
	}
	if n := len(f.seenPath(http.MethodGet, "/api/logs/event")); n != 1 {
		t.Fatalf("logs/event calls %d", n)
	}
	if n := f.rf.Count("GET", "/redfish/v1/Managers/Self/LogServices/SEL/Entries"); n != 0 {
		t.Fatalf("Redfish SEL read %d times", n)
	}
}

func TestIsOEMPlaceholder(t *testing.T) {
	for _, v := range []string{"", " ", "To Be Filled By O.E.M.", "to be filled by o.e.m.", "Default string", "N/A"} {
		if !isOEMPlaceholder(v) {
			t.Errorf("%q should be a placeholder", v)
		}
	}
	for _, v := range []string{"ASRockRack", "AsrockRack", "X570D4U"} {
		if isOEMPlaceholder(v) {
			t.Errorf("%q is real data", v)
		}
	}
}
