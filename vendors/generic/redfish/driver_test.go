package genericredfish_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
)

var ctx = context.Background()

func setup(t *testing.T) (*redfishtest.Server, *genericredfish.Driver, bmc.Target) {
	t.Helper()
	srv := redfishtest.New("admin", "secret")
	t.Cleanup(srv.Close)
	tm := transport.NewManager(transport.Config{Retry: transport.RetryPolicy{MaxAttempts: 1}, Breaker: transport.BreakerConfig{Disabled: true}})
	t.Cleanup(tm.Close)
	d := genericredfish.New(tm)
	d.Options.TaskPollInterval = time.Millisecond
	tg, err := bmc.ParseTarget(srv.Host())
	if err != nil {
		t.Fatal(err)
	}
	return srv, d, tg
}

func open(t *testing.T, d *genericredfish.Driver, tg bmc.Target) *genericredfish.Session {
	t.Helper()
	s, err := d.Open(ctx, tg, bmc.Creds{User: "admin", Pass: "secret"})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close(ctx) })
	return s.(*genericredfish.Session)
}

func TestRegisteredAndDetect(t *testing.T) {
	d, ok := bmc.Lookup("redfish")
	if !ok || d.(bmc.Prioritized).Priority() >= 0 {
		t.Fatalf("not registered with negative priority: %v", d)
	}
	srv, drv, tg := setup(t)
	m, err := drv.Detect(ctx, tg)
	if err != nil {
		t.Fatal(err)
	}
	if m.Score != bmc.ScoreFallback || m.Vendor != "redfish" || m.Manufacturer != "AMI" ||
		m.Details["redfishVersion"] != "1.15.1" || m.Details["product"] != "AMI Redfish Server" || m.Backends[0] != "redfish" {
		t.Fatalf("match: %+v", m)
	}
	for _, r := range srv.Reqs() {
		if r.Header.Get("Authorization") != "" {
			t.Fatal("Detect sent credentials")
		}
	}

	// Not a Redfish service -> zero match, no error.
	srv.Lock()
	srv.Handlers["/redfish/v1/"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>login</html>"))
	}
	srv.Unlock()
	_, drv2, _ := setup(t)
	m, err = drv2.Detect(ctx, tg)
	if err != nil || m.Score != 0 {
		t.Fatalf("non-redfish: %+v %v", m, err)
	}

	// Unreachable -> error.
	srv.Close()
	_, drv3, _ := setup(t)
	if _, err := drv3.Detect(ctx, tg); err == nil {
		t.Fatal("want error for closed host")
	}
}

func TestOpenAuthFailure(t *testing.T) {
	_, d, tg := setup(t)
	_, err := d.Open(ctx, tg, bmc.Creds{User: "admin", Pass: "bad"})
	if !errors.Is(err, bmc.ErrAuth) {
		t.Fatalf("want ErrAuth, got %v", err)
	}
}

func TestSessionInfoAndCapabilities(t *testing.T) {
	_, d, tg := setup(t)
	s := open(t, d, tg)
	info := s.Info()
	if info.Driver != "redfish" || info.Board != "B650D4U" || info.Firmware != "4.10.0" || info.User != "admin" {
		t.Fatalf("info: %+v", info)
	}
	for _, c := range bmc.AllCapabilities() {
		switch c {
		case bmc.CapBond, bmc.CapFirmwareUpdate, bmc.CapKVM:
			if info.Capabilities[c] != "" {
				t.Errorf("%s should not be reported", c)
			}
		default:
			if info.Capabilities[c] != "redfish" {
				t.Errorf("%s -> %q, want redfish", c, info.Capabilities[c])
			}
		}
	}
	rep := bmc.Describe(s)
	n := 0
	for _, r := range rep {
		if r.Supported {
			n++
		}
	}
	if n != len(bmc.AllCapabilities())-3 {
		t.Fatalf("described supported=%d", n)
	}
	if off := info.ClockOffset; off < -2*time.Second || off > 2*time.Second {
		t.Fatalf("clock offset %v", off)
	}
}

func TestIdentityPowerAndRestore(t *testing.T) {
	srv, d, tg := setup(t)
	s := open(t, d, tg)
	si, err := s.SystemInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if si.Manufacturer != "AsrockRack" || si.BIOSVersion != "10.15" || si.PowerState != bmc.PowerOff || si.BMCFirmware != "4.10.0" ||
		si.BMCMAC != "00:00:5e:00:53:01" || si.Serial != "" || si.Board != "B650D4U" {
		t.Fatalf("system info: %+v", si)
	}
	if err := s.Power(ctx, bmc.ActionOn); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.PowerState(ctx); st != bmc.PowerOn {
		t.Fatalf("state %v", st)
	}
	if err := s.Power(ctx, bmc.ActionShutdown); err != nil {
		t.Fatal(err)
	}
	if err := s.Power(ctx, bmc.ActionReset); err != nil {
		t.Fatal(err)
	}
	// B650 allows no Nmi.
	if err := s.Power(ctx, bmc.ActionNMI); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("nmi: %v", err)
	}
	var rts []string
	for _, a := range srv.Actions {
		rts = append(rts, a.Body)
	}
	got := strings.Join(rts, "|")
	if !strings.Contains(got, `"ResetType":"On"`) || !strings.Contains(got, `"ResetType":"GracefulShutdown"`) || !strings.Contains(got, `"ResetType":"ForceRestart"`) {
		t.Fatalf("reset bodies: %s", got)
	}
	p, err := s.GetPowerRestorePolicy(ctx)
	if err != nil || p != bmc.RestoreAlwaysOff {
		t.Fatalf("policy %v %v", p, err)
	}
	if err := s.SetPowerRestorePolicy(ctx, bmc.RestoreLastState); err != nil {
		t.Fatal(err)
	}
	if p, _ := s.GetPowerRestorePolicy(ctx); p != bmc.RestoreLastState {
		t.Fatalf("policy after set %v", p)
	}
}

func TestLEDAndBootOverride(t *testing.T) {
	srv, d, tg := setup(t)
	s := open(t, d, tg)
	if st, err := s.GetIndicatorLED(ctx); err != nil || st != bmc.LEDOff {
		t.Fatalf("led %v %v", st, err)
	}
	if err := s.SetIndicatorLED(ctx, bmc.LEDBlinking); err != nil {
		t.Fatal(err)
	}
	if srv.Get("/redfish/v1/Chassis/Self")["IndicatorLED"] != "Blinking" {
		t.Fatal("led not set")
	}
	var patch redfishtest.Request
	for _, r := range srv.Reqs() {
		if r.Method == "PATCH" {
			patch = r
		}
	}
	if patch.Path != "/redfish/v1/Chassis/Self" || patch.Header.Get("If-Match") != redfishtest.ETagService {
		t.Fatalf("LED patch without If-Match: %+v", patch)
	}

	cfg, err := s.GetBootOverride(ctx)
	if err != nil || cfg.Target != bmc.BootNone || cfg.Mode != bmc.BootDisabled || cfg.BootMode != "legacy" {
		t.Fatalf("boot %+v %v", cfg, err)
	}
	if err := s.SetBootOverride(ctx, bmc.BootOverrideConfig{Target: bmc.BootPXE, Mode: bmc.BootOnce, BootMode: "uefi"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ = s.GetBootOverride(ctx)
	if cfg.Target != bmc.BootPXE || cfg.Mode != bmc.BootOnce || cfg.BootMode != "uefi" {
		t.Fatalf("boot after set %+v", cfg)
	}
}

func TestSensorsNormalized(t *testing.T) {
	_, d, tg := setup(t)
	s := open(t, d, tg)
	sensors, err := s.Sensors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sensors) != 60 {
		t.Fatalf("sensors=%d", len(sensors))
	}
	var bat, cpu *bmc.Sensor
	for i := range sensors {
		switch {
		case sensors[i].Name == "VOLT_BAT":
			bat = &sensors[i]
		case strings.HasPrefix(sensors[i].Name, "TEMP_CPU") && cpu == nil:
			cpu = &sensors[i]
		}
	}
	if bat == nil || bat.Type != bmc.SensorVoltage || bat.Unit != "V" || *bat.Reading != 3.08 || bat.Health != bmc.HealthOK ||
		*bat.Thresholds.LowerCritical != 2.4 || *bat.Thresholds.LowerNonRecoverable != 2.22 || *bat.Thresholds.UpperCritical != 3.3 ||
		*bat.Thresholds.UpperNonRecoverable != 3.46 || bat.Thresholds.UpperNonCritical != nil || bat.Backend != "redfish" {
		t.Fatalf("VOLT_BAT: %+v", bat)
	}
	if cpu == nil || cpu.Reading != nil || cpu.Type != bmc.SensorTemperature || cpu.Unit != "C" || cpu.State != "Absent" {
		t.Fatalf("TEMP_CPU (host off): %+v", cpu)
	}
}

func TestSensorsLegacyFallback(t *testing.T) {
	srv, d, tg := setup(t)
	ch := srv.Get("/redfish/v1/Chassis/Self")
	delete(ch, "Sensors")
	srv.Set("/redfish/v1/Chassis/Self", ch)
	s := open(t, d, tg)
	sensors, err := s.Sensors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(sensors) != 5 {
		t.Fatalf("legacy sensors=%d: %+v", len(sensors), sensors)
	}
	m := map[string]bmc.Sensor{}
	for _, x := range sensors {
		m[x.Name] = x
	}
	if v := m["VOLT_3VSB"]; *v.Reading != 3.32 || *v.Thresholds.UpperCritical != 3.72 || *v.Number != 1 || v.Unit != "V" {
		t.Fatalf("VOLT_3VSB %+v", v)
	}
	if v := m["TEMP_CPU"]; v.Reading != nil || *v.Thresholds.UpperCritical != 95 {
		t.Fatalf("TEMP_CPU %+v", v)
	}
}

func TestEventsIncrementalAndClear(t *testing.T) {
	srv, d, tg := setup(t)
	s := open(t, d, tg)
	page, err := s.Events(ctx, bmc.EventQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 11 || page.LastID != "11" || page.More {
		t.Fatalf("all: %d last=%s", len(page.Entries), page.LastID)
	}
	e := page.Entries[3]
	if e.ID != "4" || e.Severity != bmc.HealthWarning || e.Direction != "assertion" || e.SensorType != "Temperature" ||
		*e.SensorNumber != 12 || e.Time.Raw.IsZero() || e.Backend != "redfish" || e.Oem["messageId"] != "IPMI.0.1.0.SensorEvent" {
		t.Fatalf("entry: %+v", e)
	}
	page, _ = s.Events(ctx, bmc.EventQuery{SinceID: "9"})
	if len(page.Entries) != 2 || page.Entries[0].ID != "10" || page.LastID != "11" {
		t.Fatalf("since 9: %+v", page)
	}
	page, _ = s.Events(ctx, bmc.EventQuery{SinceID: "2", Limit: 3})
	if len(page.Entries) != 3 || !page.More || page.LastID != "5" {
		t.Fatalf("limit: %+v", page)
	}
	id := srv.AddSELEntry("new event", "Critical")
	page, _ = s.Events(ctx, bmc.EventQuery{SinceID: "11"})
	if len(page.Entries) != 1 || page.Entries[0].ID != id || page.Entries[0].Severity != bmc.HealthCritical {
		t.Fatalf("new: %+v", page)
	}
	page, _ = s.Events(ctx, bmc.EventQuery{SinceID: id})
	if len(page.Entries) != 0 || page.LastID != id {
		t.Fatalf("none: %+v", page)
	}
	if err := s.ClearEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if page, _ = s.Events(ctx, bmc.EventQuery{}); len(page.Entries) != 0 {
		t.Fatalf("after clear: %d", len(page.Entries))
	}

	audit, err := s.AuditLog(ctx, bmc.AuditQuery{})
	if err != nil || len(audit) != 2 || audit[1].Message != "User admin logged in from 192.0.2.5" {
		t.Fatalf("audit %+v %v", audit, err)
	}
}

func TestInventoryFRUAndMore(t *testing.T) {
	_, d, tg := setup(t)
	s := open(t, d, tg)
	cpus, err := s.CPUs(ctx)
	if err != nil || len(cpus) != 1 || cpus[0].Cores != 16 || cpus[0].Threads != 32 || !cpus[0].Present {
		t.Fatalf("cpus %+v %v", cpus, err)
	}
	mem, err := s.Memory(ctx)
	if err != nil || len(mem) != 4 || !mem[0].Present || mem[0].SizeMiB != 32768 || mem[0].Type != "DDR5" || mem[1].Present {
		t.Fatalf("memory %+v %v", mem, err)
	}
	st, err := s.Storage(ctx)
	if err != nil || len(st) != 1 || st[0].Protocol != "nvme" || st[0].MediaType != "ssd" || st[0].CapacityBytes != 2000398934016 {
		t.Fatalf("storage %+v %v", st, err)
	}
	if _, err := s.PCIeDevices(ctx); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("pcie on fixture without PCIeDevices: %v", err)
	}
	bb, err := s.Baseboard(ctx)
	if err != nil || bb.Product != "B650D4U" || bb.BIOSVersion != "10.15" || bb.Serial != "" {
		t.Fatalf("baseboard %+v %v", bb, err)
	}
	frus, err := s.FRU(ctx)
	if err != nil || len(frus) != 1 || frus[0].Board.Product != "B650D4U" || frus[0].Product.Manufacturer != "AsrockRack" || frus[0].Chassis.Type != "Other" {
		t.Fatalf("fru %+v %v", frus, err)
	}

	nics, err := s.NetworkInterfaces(ctx)
	if err != nil || len(nics) != 1 {
		t.Fatalf("nics %v", err)
	}
	n := nics[0]
	if n.Name != "eth0" || n.IPv4.Address != "192.0.2.55" || n.IPv4.Gateway != "192.0.2.1" || n.IPv4.DHCP || !*n.LinkUp ||
		n.SpeedMbs != 1000 || len(n.DNS) != 1 || n.IPv6.Addresses[0] != "fe80::200:5eff:fe00:5301/64" {
		t.Fatalf("nic %+v", n)
	}
	users, err := s.Users(ctx)
	if err != nil || len(users) != 3 || users[2].Name != "admin" || users[2].Privilege != bmc.PrivAdministrator || users[1].Privilege != bmc.PrivOEM {
		t.Fatalf("users %+v %v", users, err)
	}
	svcs, err := s.Services(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sm := map[string]bmc.Service{}
	for _, x := range svcs {
		sm[x.Name] = x
	}
	if !sm["ipmi"].Enabled || sm["ipmi"].Port != 623 || sm["ntp"].Enabled || sm["https"].SecurePort != 443 {
		t.Fatalf("services %+v", svcs)
	}
	fw, err := s.Firmware(ctx)
	if err != nil || len(fw) != 2 || fw[1].Name != "BMC" || fw[1].Version != "4.10.00" || !fw[1].Updateable {
		t.Fatalf("firmware %+v %v", fw, err)
	}
}

func TestTimeVirtualMediaAndBMCReset(t *testing.T) {
	srv, d, tg := setup(t)
	srv.Lock()
	srv.ClockSkew = 3 * time.Hour
	srv.Unlock()
	s := open(t, d, tg)
	if off := s.Info().ClockOffset; off < 3*time.Hour-2*time.Second || off > 3*time.Hour+2*time.Second {
		t.Fatalf("clock offset %v", off)
	}
	tc, err := s.Time(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tc.UTCOffsetMinutes != -300 || tc.NTPEnabled || len(tc.NTPServers) != 2 || tc.OffsetSeconds < 3*3600-2 {
		t.Fatalf("time %+v", tc)
	}
	en := true
	if err := s.SetTime(ctx, bmc.TimeUpdate{NTPEnabled: &en, NTPServers: []string{"10.7.0.1", "10.7.0.2"}}); err != nil {
		t.Fatal(err)
	}
	ntp := srv.Get("/redfish/v1/Managers/Self/NetworkProtocol")["NTP"].(map[string]any)
	if ntp["ProtocolEnabled"] != true || ntp["NTPServers"].([]any)[1] != "10.7.0.2" {
		t.Fatalf("ntp %v", ntp)
	}
	srv.ResetLog()
	if _, err := s.SyncTime(ctx); err != nil {
		t.Fatal(err)
	}
	var dt string
	for _, r := range srv.Reqs() {
		if r.Method == "PATCH" && r.Path == "/redfish/v1/Managers/Self" {
			dt = r.Body
		}
	}
	if !strings.Contains(dt, `"DateTime":"`) || !strings.Contains(dt, `-05:00"`) {
		t.Fatalf("DateTime patch %q", dt)
	}

	slots, err := s.VirtualMedia(ctx)
	if err != nil || len(slots) != 1 || slots[0].ID != "CD1" || slots[0].Inserted || slots[0].MediaTypes[0] != "cd" {
		t.Fatalf("vm %+v %v", slots, err)
	}
	if err := s.InsertMedia(ctx, bmc.VirtualMediaInsert{SlotID: "CD1", Image: "http://10.7.0.1/boot.iso", WriteProtected: true}); err != nil {
		t.Fatal(err)
	}
	slots, _ = s.VirtualMedia(ctx)
	if !slots[0].Inserted || slots[0].Image != "http://10.7.0.1/boot.iso" {
		t.Fatalf("after insert %+v", slots[0])
	}
	if err := s.EjectMedia(ctx, "CD1"); err != nil {
		t.Fatal(err)
	}
	if err := s.EjectMedia(ctx, "nope"); !errors.Is(err, bmc.ErrNotFound) {
		t.Fatalf("missing slot: %v", err)
	}

	if err := s.ResetBMC(ctx, bmc.BMCResetCold); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetBMC(ctx, bmc.BMCResetWarm); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("warm reset on B650 should be unsupported: %v", err)
	}
}

func TestNewSessionReuseForVendorDriver(t *testing.T) {
	srv, d, tg := setup(t)
	// A vendor driver builds its own client (e.g. with board quirks) and
	// embeds the generic session, hiding capabilities it serves elsewhere.
	c, err := d.NewClient(tg, bmc.Creds{User: "admin", Pass: "secret"}, rf.AuthBasic)
	if err != nil {
		t.Fatal(err)
	}
	s, err := genericredfish.NewSession(ctx, c, genericredfish.SessionOptions{DriverName: "asrock", Target: tg, Board: "B650D4U",
		Firmware: "4.10.00", Exclude: []bmc.Capability{bmc.CapVirtualMedia}})
	if err != nil {
		t.Fatal(err)
	}
	info := s.Info()
	if info.Driver != "asrock" || info.Firmware != "4.10.00" || info.Capabilities[bmc.CapVirtualMedia] != "" || info.Capabilities[bmc.CapSensors] != "redfish" {
		t.Fatalf("info %+v", info)
	}
	if si, _ := s.SystemInfo(ctx); si.Vendor != "asrock" {
		t.Fatalf("vendor %q", si.Vendor)
	}
	_ = srv
	// Session-mode client works through the same session code.
	cs, _ := d.NewClient(tg, bmc.Creds{User: "admin", Pass: "secret"}, rf.AuthSession)
	ss, err := genericredfish.NewSession(ctx, cs, genericredfish.SessionOptions{Target: tg, CloseClient: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ss.Sensors(ctx); err != nil {
		t.Fatal(err)
	}
	if srv.LiveSessions() != 1 {
		t.Fatalf("live sessions %d", srv.LiveSessions())
	}
	ss.Close(ctx)
	ss.Close(ctx)
	if srv.LiveSessions() != 0 {
		t.Fatalf("session not logged out")
	}
}

// Gigabyte MC13-LE0 13.04.15 lists only power/energy monitors in Sensors;
// temperatures, fans and voltages live in Thermal/Power and are merged in.
func TestSensorsMergeLegacyWhenCollectionHasNoThermal(t *testing.T) {
	srv, d, tg := setup(t)
	srv.Lock()
	srv.Resources["/redfish/v1/Chassis/Self/Sensors"] = map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Sensors",
		"Members": []any{map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Sensors/Energy"},
			map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Sensors/Power"}}}
	srv.Resources["/redfish/v1/Chassis/Self/Sensors/Energy"] = map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Sensors/Energy",
		"Id": "Power Energy Monitor", "Name": "Power Energy Monitor", "Reading": 152043585, "ReadingType": "EnergyJoules"}
	srv.Resources["/redfish/v1/Chassis/Self/Sensors/Power"] = map[string]any{"@odata.id": "/redfish/v1/Chassis/Self/Sensors/Power",
		"Id": "Power Consumption Monitor", "Name": "Power Consumption Monitor", "Reading": 168, "ReadingType": "Power"}
	srv.Unlock()
	s := open(t, d, tg)
	sensors, err := s.Sensors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]bmc.Sensor{}
	for _, x := range sensors {
		m[x.Name] = x
	}
	if len(sensors) != 2+5 {
		t.Fatalf("sensors=%d (want 2 modern + 5 legacy): %+v", len(sensors), sensors)
	}
	if e := m["Power Energy Monitor"]; e.Unit != "J" || e.Type != bmc.SensorPower {
		t.Fatalf("energy sensor %+v", e)
	}
	if _, ok := m["TEMP_CPU"]; !ok {
		t.Fatalf("thermal sensors not merged: %+v", sensors)
	}

	// A collection that already has temperatures is used alone (B650).
	srv2, d2, tg2 := setup(t)
	_ = srv2
	s2 := open(t, d2, tg2)
	all, err := s2.Sensors(ctx)
	if err != nil || len(all) != 60 {
		t.Fatalf("B650 sensors=%d err=%v", len(all), err)
	}
}
