package gigabyte

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// MZ32-AR0 (AMI SP-X 12.61.39) fixture tests. The Redfish bodies are the
// research agent's sanitized crawl; the /api bodies below are synthesized
// from the field lists and values documented in
// docs/gigabyte/mz32-ar0-frontend.md §4 (no raw /api capture is available
// to this driver's tests).

const mz32Fixture = "testdata/mz32ar0-12.61.39.json"

// mz32Events is a synthetic /api/logs/event body in the documented shape.
var mz32Events = []map[string]any{
	{"id": 1, "record_type": "system event record", "timestamp": 1619237902, "sensor_type": "Event Logging Disabled", "sensor_name": "SEL", "sensor_number": 225,
		"generator_type": "BIOS", "event_direction": "Asserted", "event_reading_class": "sensor-specific", "event_description": "Log Area Reset/Cleared", "event_assertSeverity": "OK"},
	{"id": 2, "record_type": "system event record", "timestamp": 1619237950, "sensor_type": "Temperature", "sensor_name": "CPU0_TEMP", "sensor_number": 1,
		"generator_type": "BMC", "event_direction": "Asserted", "event_reading_class": "threshold", "event_description": "Upper Critical - going high", "event_assertSeverity": "Critical"},
	{"id": 3, "record_type": "system event record", "timestamp": 1619238000, "sensor_type": "Temperature", "sensor_name": "CPU0_TEMP", "sensor_number": 1,
		"generator_type": "BMC", "event_direction": "Deasserted", "event_reading_class": "threshold", "event_description": "Upper Critical - going high", "event_assertSeverity": "OK"},
}

func newMZ32(t *testing.T) *fake {
	t.Helper()
	f := newFakeFrom(t, mz32Fixture, false)
	f.srv.Lock()
	f.srv.Handlers["/redfish/v1/$metadata"] = func(w http.ResponseWriter, r *http.Request) {
		// static/gbt_oem_metadata of the 12.61.39 image
		_, _ = w.Write([]byte(`<edmx:Edmx Version="4.0"><edmx:Reference Uri="/redfish/v1/schemas/GBTThermal.v1_0_0.xml"><edmx:Include Namespace="GBTThermal.v1_0_0"/></edmx:Reference></edmx:Edmx>`))
	}
	f.ami.api["/api/firmware-info"] = map[string]any{"device_id": 32, "revision": 129, "fw_rev_1": 12, "fw_rev_2": 97, "ipmi_version": 2,
		"mfg_id_0": 10, "mfg_id_1": 60, "mfg_id_2": 0, "prod_id": 4104, "aux": 39, "fw_ver": "12.61.39", "bios_ver": "",
		"date": "Jul 2 2025", "time": "11:14:05 CST", "active_image": 1, "sku_ver": "1.0"}
	f.srv.Handlers["/api/firmware-info"] = f.ami.static
	f.srv.Handlers["/api/logs/event"] = func(w http.ResponseWriter, r *http.Request) {
		if !f.ami.check(w, r) {
			return
		}
		since, _ := strconv.Atoi(r.URL.Query().Get("LASTEVENTID"))
		out := []map[string]any{}
		for _, e := range mz32Events {
			if e["id"].(int) > since {
				out = append(out, e)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
	f.srv.Unlock()
	return f
}

func noBadQueries(t *testing.T, f *fake) {
	t.Helper()
	for _, r := range f.srv.Reqs() {
		if strings.Contains(strings.ToLower(r.RawQuery), "%24") {
			t.Fatalf("percent-encoded $ sent (ignored by SP-X 12.x): %s?%s", r.Path, r.RawQuery)
		}
		if strings.Contains(r.RawQuery, "$filter") {
			t.Fatalf("$filter sent (page-local on SP-X 12.x): %s?%s", r.Path, r.RawQuery)
		}
		if strings.Contains(r.Path, "SubProcessors") {
			t.Fatalf("walked the SubProcessors crawl trap: %s", r.Path)
		}
	}
}

func TestMZ32Detect(t *testing.T) {
	f := newMZ32(t)
	m, err := f.driver().Detect(ctx, f.tg)
	if err != nil || m.Score != scoreUnauth || m.Details["redfishVersion"] != "1.8.0" || m.Details["stack"] != "AMI MegaRAC" {
		t.Fatalf("unauth: %+v %v", m, err)
	}
	noAuthSent(t, f.srv)
	m, err = f.driver().DetectWithCreds(ctx, f.tg, bmc.Creds{User: testUser, Pass: testPass})
	if err != nil || m.Score != bmc.ScoreCertain || m.Manufacturer != "GIGABYTE" || m.Board != "MZ32-AR0-00" ||
		m.Firmware != "12.61.39" || m.Details["profile"] != "mz32ar0" || m.Details["verified"] != "true" {
		t.Fatalf("creds: %+v %v", m, err)
	}
}

func TestMZ32ProfileAndCapabilities(t *testing.T) {
	f := newMZ32(t)
	s := f.open(t)
	p := s.Profile()
	if p.Key != "mz32ar0" || !p.Verified || p.Redfish.SkipPageSize != 0 || p.Redfish.QueryCaps == nil || p.Redfish.QueryCaps.Filter {
		t.Fatalf("profile: %+v", p)
	}
	if s.Info().Degraded != "" {
		t.Fatalf("degraded: %q", s.Info().Degraded)
	}

	ss, err := s.Sensors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	by := map[bmc.SensorType]int{}
	for _, x := range ss {
		by[x.Type]++
	}
	if by[bmc.SensorType("temperature")] < 12 || by[bmc.SensorType("fan")] < 6 || by[bmc.SensorType("voltage")] < 12 {
		t.Fatalf("sensors not merged from Thermal/Power: %v", by)
	}

	if cpus, err := s.CPUs(ctx); err != nil || len(cpus) != 1 {
		t.Fatalf("cpus: %d %v", len(cpus), err)
	}
	frus, err := s.FRU(ctx)
	if err != nil || len(frus) == 0 || frus[0].Board.Serial != fakeSerial {
		t.Fatalf("FRU: %+v %v", frus, err)
	}
	fw, err := s.Firmware(ctx)
	ok := false
	for _, c := range fw {
		ok = ok || c.Version == "12.61.39"
	}
	if err != nil || !ok {
		t.Fatalf("firmware: %+v %v", fw, err)
	}
	if led, err := s.GetIndicatorLED(ctx); err != nil || led == "" {
		t.Fatalf("led: %v %v", led, err)
	}
	if fp, err := s.FanProfiles(ctx); err != nil || len(fp.Profiles) == 0 {
		t.Fatalf("fan profiles: %+v %v", fp, err)
	}
	noBadQueries(t, f)
}

// Power cycle and NMI have no ResetType on 12.61.39: 501 without any POST.
func TestMZ32PowerCycleUnsupported(t *testing.T) {
	f := newMZ32(t)
	s := f.open(t)
	for _, a := range []bmc.PowerAction{bmc.ActionCycle, bmc.ActionNMI} {
		if err := s.Power(ctx, a); !errors.Is(err, bmc.ErrUnsupported) {
			t.Fatalf("%s: want ErrUnsupported, got %v", a, err)
		}
	}
	if n := f.srv.Count(http.MethodPost, "/redfish/v1/Systems/Self/Actions"); n != 0 {
		t.Fatalf("%d reset POSTs sent", n)
	}
}

func TestMZ32EventsFromAMI(t *testing.T) {
	f := newMZ32(t)
	s := f.open(t)
	page, err := s.Events(ctx, bmc.EventQuery{})
	if err != nil || len(page.Entries) != 3 || page.LastID != "3" {
		t.Fatalf("events: %+v %v", page, err)
	}
	e := page.Entries[1]
	if e.Backend != bmc.BackendAMI || e.SensorName != "CPU0_TEMP" || e.Severity != bmc.HealthCritical ||
		e.Direction != "assertion" || e.Time.Raw.Unix() != 1619237950 {
		t.Fatalf("entry: %+v", e)
	}
	page, err = s.Events(ctx, bmc.EventQuery{SinceID: "2"})
	if err != nil || len(page.Entries) != 1 || page.Entries[0].ID != "3" {
		t.Fatalf("since 2: %+v %v", page, err)
	}
	if _, err := s.Events(ctx, bmc.EventQuery{SinceID: "x"}); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Fatalf("bad since: %v", err)
	}
	if f.ami.logins != 1 {
		t.Fatalf("logins %d", f.ami.logins)
	}
}

// Without /api/logs/event the Redfish SEL (57 entries, nextLink paging) is
// used.
func TestMZ32EventsFallbackToRedfish(t *testing.T) {
	f := newMZ32(t)
	f.srv.Lock()
	delete(f.srv.Handlers, "/api/logs/event")
	f.srv.Handlers["/api/logs/event"] = f.ami.static // 404 {"code":1010}
	f.srv.Unlock()
	s := f.open(t)
	page, err := s.Events(ctx, bmc.EventQuery{})
	if err != nil || len(page.Entries) != 57 || page.Entries[0].Backend != bmc.BackendRedfish {
		n := len(page.Entries)
		t.Fatalf("fallback: %d entries %v", n, err)
	}
	noBadQueries(t, f)
}

func TestMZ32FirmwareInfo(t *testing.T) {
	f := newMZ32(t)
	s := f.open(t)
	fi, err := s.FirmwareInfo(ctx)
	if err != nil || fi.Version != "12.61.39" || fi.ActiveImage != 1 || fi.ManufacturerID != 15370 || fi.ProductID != 4104 {
		t.Fatalf("firmware info: %+v %v", fi, err)
	}
	// MC13-LE0 live capture shape too.
	mc := newFake(t).open(t)
	fi, err = mc.FirmwareInfo(ctx)
	if err != nil || fi.Version != "13.04.15" || fi.ManufacturerID != 15370 || fi.ActiveImage != 1 {
		t.Fatalf("mc13 firmware info: %+v %v", fi, err)
	}
}

func TestMZ32RawDenylist(t *testing.T) {
	rs := routeMap(t)
	api := rs[" raw/api/{path...}"].TierFor
	for _, p := range []string{"ae/run", "ae_test", "ipmi_response", "sample/get", "kvm/token", "remote_control/get/kvm/launch",
		"GBT/OEM/CMC/Chassis/Power/x", "system_inventory_gbt/smbios-file", "system_inventory_gbt/bios-setup-file",
		"maintenance/backup_config", "maintenance/dual_image_config", "settings/dns/restart", "actions/x", "logs/help-data"} {
		if _, err := api(rawReq("GET", p)); !errors.Is(err, bmc.ErrForbidden) {
			t.Errorf("raw/api GET %s: %v, want forbidden", p, err)
		}
	}
	for _, p := range []string{"system_inventory_gbt/cpu_info", "logs/event", "firmware-info", "GSM"} {
		if tier, err := api(rawReq("GET", p)); err != nil || tier != bmc.TierRead {
			t.Errorf("raw/api GET %s: %v %v, want read", p, tier, err)
		}
	}
	if _, err := rs[" raw/redfish/{path...}"].TierFor(rawReq("GET", "EventService/SSE")); !errors.Is(err, bmc.ErrForbidden) {
		t.Errorf("SSE not denied: %v", err)
	}
	// The session's AMI client enforces the profile deny list too.
	f := newMZ32(t)
	s := f.open(t)
	if err := s.AMI().Get(ctx, "ipmi_response", nil); !errors.Is(err, bmc.ErrForbidden) {
		t.Fatalf("AMI client: %v", err)
	}
	for _, r := range f.srv.Reqs() {
		if strings.Contains(r.Path, "ipmi_response") {
			t.Fatal("denied path reached the BMC")
		}
	}
}
