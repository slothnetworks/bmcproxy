package asrock

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

var creds = bmc.Creds{User: testUser, Pass: testPass}

func TestDetectX470(t *testing.T) {
	f := newX470(t)
	d := newTestDriver(t, nil)
	ctx := context.Background()
	m, err := d.Detect(ctx, f.target(t))
	if err != nil {
		t.Fatal(err)
	}
	if m.Score <= 10 || m.Vendor != Name {
		t.Fatalf("unauth detect: %+v (must beat generic redfish ScoreFallback)", m)
	}
	m, err = d.DetectWithCreds(ctx, f.target(t), creds)
	if err != nil {
		t.Fatal(err)
	}
	if m.Score != bmc.ScoreCertain || m.Board != "X470D4U" || m.Firmware != "2.32.00" {
		t.Fatalf("cred detect: %+v", m)
	}
	// X470 login form carries certlogin=0 (the fake rejects it otherwise).
	logins := f.logins.Load()
	s := openSession(t, d, f)
	if f.logins.Load() != logins {
		t.Fatalf("Open after DetectWithCreds should reuse the handed-off login (logins %d -> %d)", logins, f.logins.Load())
	}
	if s.Profile().Key != "x470d4u" || !s.Profile().Verified {
		t.Fatalf("profile %+v", s.Profile())
	}
}

func TestDetectB650(t *testing.T) {
	f := newB650(t, "4.10.00")
	d := newTestDriver(t, nil)
	ctx := context.Background()
	m, err := d.Detect(ctx, f.target(t))
	if err != nil {
		t.Fatal(err)
	}
	if m.Score <= 10 {
		t.Fatalf("unauth detect: %+v", m)
	}
	m, err = d.DetectWithCreds(ctx, f.target(t), creds)
	if err != nil {
		t.Fatal(err)
	}
	if m.Score != bmc.ScoreCertain || m.Board != "B650D4U" || m.Firmware != "4.10.00" {
		t.Fatalf("cred detect: %+v", m)
	}
}

func TestDetectNotAMI(t *testing.T) {
	srv := newFake(t, "x470d4u-2.32.json")
	srv.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	d := newTestDriver(t, nil)
	m, _ := d.Detect(context.Background(), srv.target(t))
	if m.Score != 0 {
		t.Fatalf("plain web server matched: %+v", m)
	}
}

func TestCapabilitiesX470(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	caps := s.Info().Capabilities
	want := map[bmc.Capability]string{
		bmc.CapPower: bmc.BackendAMI, bmc.CapSensors: bmc.BackendAMI, bmc.CapEventLog: bmc.BackendAMI,
		bmc.CapBootOverride: bmc.BackendIPMI, bmc.CapPowerRestorePolicy: bmc.BackendIPMI,
		bmc.CapTime: bmc.BackendAMI, bmc.CapFRU: bmc.BackendAMI, bmc.CapBond: bmc.BackendAMI,
	}
	for c, b := range want {
		if caps[c] != b {
			t.Errorf("X470 %s served by %q, want %q (all: %v)", c, caps[c], b, caps)
		}
	}
	if _, ok := caps[bmc.CapVirtualMedia]; ok {
		t.Errorf("X470 has no Redfish, virtualMedia must be unsupported: %v", caps)
	}
	if s.Session != nil {
		t.Fatal("X470 must not open Redfish (separate account store)")
	}
	// Promoted generic-redfish methods must not nil-deref.
	if _, err := s.VirtualMedia(context.Background()); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("VirtualMedia: %v", err)
	}
}

func TestCapabilitiesB650(t *testing.T) {
	f := newB650(t, "4.10.00")
	s := openSession(t, newTestDriver(t, nil), f)
	caps := s.Info().Capabilities
	want := map[bmc.Capability]string{
		bmc.CapPower: bmc.BackendRedfish, bmc.CapSensors: bmc.BackendRedfish,
		bmc.CapFRU: bmc.BackendAMI, bmc.CapBond: bmc.BackendAMI, bmc.CapPowerRestorePolicy: bmc.BackendAMI,
		bmc.CapBootOverride: bmc.BackendRedfish,
	}
	for c, b := range want {
		if caps[c] != b {
			t.Errorf("B650 %s served by %q, want %q (all: %v)", c, caps[c], b, caps)
		}
	}
	var sawRF, sawAMI bool
	for _, b := range s.Info().Backends {
		sawRF = sawRF || b.Name == bmc.BackendRedfish && b.Authenticated
		sawAMI = sawAMI || b.Name == bmc.BackendAMI && b.Authenticated
	}
	if !sawRF || !sawAMI {
		t.Fatalf("backends: %+v", s.Info().Backends)
	}
}

func TestSensorsX470ZeroThresholds(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	ss, err := s.Sensors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var v3 *bmc.Sensor
	for i := range ss {
		if ss[i].Name == "3VSB" {
			v3 = &ss[i]
		}
	}
	if v3 == nil {
		t.Fatalf("no 3VSB in %d sensors", len(ss))
	}
	th := v3.Thresholds
	if th.LowerNonCritical != nil || th.UpperNonCritical != nil {
		t.Fatalf("X470 0.0 placeholders must be unset: %+v", th)
	}
	if th.LowerCritical == nil || *th.LowerCritical != 3.04 || th.UpperCritical == nil || *th.UpperCritical != 3.7 {
		t.Fatalf("thresholds: %+v", th)
	}
	if v3.Reading == nil || *v3.Reading != 3.4 || v3.Unit != "V" || v3.Health != bmc.HealthOK {
		t.Fatalf("sensor: %+v", v3)
	}
}

func TestSensorsB650NA(t *testing.T) {
	// The B650 fixture via the AMI path ("NA" strings), independent of
	// Redfish: decode directly with the profile quirk off.
	raw := loadFixture(t, "b650d4u-1.10.json")["GET /api/sensors"].Body
	var as []amiSensor
	if err := json.Unmarshal(raw, &as); err != nil {
		t.Fatal(err)
	}
	s := sensorFromAMI(as[0], false)
	if s.Thresholds.LowerNonCritical != nil || s.Thresholds.UpperNonCritical != nil {
		t.Fatalf("NA thresholds must be nil: %+v", s.Thresholds)
	}
	if s.Thresholds.LowerNonRecoverable == nil || *s.Thresholds.LowerNonRecoverable != 2.86 {
		t.Fatalf("LNR: %+v", s.Thresholds)
	}
	// Unavailable reading (accessible 213).
	as[0].Accessible.V, as[0].Accessible.Valid = 213, true
	if s := sensorFromAMI(as[0], false); s.Reading != nil || s.Health != bmc.HealthUnknown {
		t.Fatalf("accessible=213 must hide reading: %+v", s)
	}
}

func TestEventsIncrementalX470(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	ctx := context.Background()
	p, err := s.Events(ctx, bmc.EventQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Entries) == 0 || p.LastID == "" {
		t.Fatalf("page: %+v", p)
	}
	for i := 1; i < len(p.Entries); i++ {
		if p.Entries[i-1].ID >= p.Entries[i].ID && len(p.Entries[i-1].ID) == len(p.Entries[i].ID) {
			t.Fatalf("not ascending: %s then %s", p.Entries[i-1].ID, p.Entries[i].ID)
		}
	}
	p2, err := s.Events(ctx, bmc.EventQuery{SinceID: p.LastID})
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Entries) != 0 || p2.LastID != p.LastID {
		t.Fatalf("incremental page: %+v", p2)
	}
	reqs := f.seenPath("GET", "/api/logs/event")
	if len(reqs) != 2 || reqs[1].Query != "LASTEVENTID="+p.LastID {
		t.Fatalf("requests: %+v", reqs)
	}
	if n := len(f.seenPath("GET", "/api/logs/eventlog")); n != 0 {
		t.Fatalf("the full eventlog dump must not be used (%d calls)", n)
	}
	if _, err := s.Events(ctx, bmc.EventQuery{SinceID: "abc"}); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Fatalf("bad since: %v", err)
	}
}

func TestEventsB650TrailerSkipped(t *testing.T) {
	f := newB650(t, "1.10.00")
	// The B650 capture has no event log; reuse the sibling B550D4U-2T one.
	for k, v := range loadFixture(t, "b550d4u-2t-1.10.json") {
		if strings.HasPrefix(k, "GET /api/logs/eventlog") {
			f.responses[k] = v
		}
	}
	d := newTestDriver(t, nil)
	s := openSession(t, d, f)
	// Force the AMI path for the event log.
	s.mu.Lock()
	s.caps[bmc.CapEventLog] = bmc.BackendAMI
	s.mu.Unlock()
	p, err := s.Events(context.Background(), bmc.EventQuery{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range p.Entries {
		if e.ID == "0" || e.ID == "" {
			t.Fatalf("trailer leaked as an entry: %+v", e)
		}
	}
	if len(p.Entries) == 0 {
		t.Fatal("no entries")
	}
}

func TestCC7ReloginThroughDriver(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	before := f.logins.Load()
	f.killAll() // e.g. another client logged the session out
	if _, err := s.PowerState(context.Background()); err != nil {
		t.Fatalf("PowerState after session kill: %v", err)
	}
	if got := f.logins.Load(); got != before+1 {
		t.Fatalf("logins %d -> %d, want one re-login", before, got)
	}
}

func TestBackendRestartIsSessionLost(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	f.handle("GET /api/sensors", func(w http.ResponseWriter, r *http.Request) {
		f.killAll()
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("<html>502 Bad Gateway</html>"))
	})
	_, err := s.Sensors(context.Background())
	if !errors.Is(err, bmc.ErrSessionLost) {
		t.Fatalf("502: %v, want ErrSessionLost", err)
	}
	// Every later op on this session fails fast the same way.
	if _, err := s.PowerState(context.Background()); !errors.Is(err, bmc.ErrSessionLost) {
		t.Fatalf("next op: %v", err)
	}
	if !strings.Contains(s.Info().Degraded, "restart") && s.Info().Degraded == "" {
		t.Fatalf("degraded: %q", s.Info().Degraded)
	}
}

func TestPowerCodesAMI(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	var mu sync.Mutex
	var codes []int
	f.handle("POST /api/actions/power", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			C int `json:"power_command"`
		}
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		codes = append(codes, b.C)
		mu.Unlock()
		writeJSON(w, 200, `{"power_command":0}`)
	})
	for _, a := range []bmc.PowerAction{bmc.ActionOn, bmc.ActionOff, bmc.ActionShutdown, bmc.ActionCycle, bmc.ActionReset} {
		if err := s.Power(context.Background(), a); err != nil {
			t.Fatalf("%s: %v", a, err)
		}
	}
	if got := codes; len(got) != 5 || got[0] != 1 || got[1] != 0 || got[2] != 5 || got[3] != 2 || got[4] != 3 {
		t.Fatalf("power codes %v, want [1 0 5 2 3]", got)
	}
	st, err := s.PowerState(context.Background())
	if err != nil || st == bmc.PowerUnknown {
		t.Fatalf("state %v %v", st, err)
	}
}

func TestLEDAndRestoreAndBootX470(t *testing.T) {
	f := newX470(t)
	ip := &fakeIPMI{powerOn: true}
	s := openSession(t, newTestDriver(t, ip), f)
	ctx := context.Background()
	var led []string
	f.handle("POST /api/actions/chassis-led", func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(r.Body)
		_ = b
		var m map[string]int
		_ = json.NewDecoder(r.Body).Decode(&m)
		led = append(led, strings.Repeat("x", m["uid_command"]))
		writeJSON(w, 200, `{}`)
	})
	if err := s.SetIndicatorLED(ctx, bmc.LEDOn); err != nil {
		t.Fatal(err)
	}
	if len(led) != 1 || led[0] != "xx" {
		t.Fatalf("uid_command: %v", led)
	}
	if st, err := s.GetIndicatorLED(ctx); err != nil || st != bmc.LEDOff {
		t.Fatalf("led %v %v", st, err)
	}
	// Restore policy via IPMI on X470.
	if err := s.SetPowerRestorePolicy(ctx, bmc.RestoreAlwaysOn); err != nil {
		t.Fatal(err)
	}
	if p, err := s.GetPowerRestorePolicy(ctx); err != nil || p != bmc.RestoreAlwaysOn {
		t.Fatalf("restore %v %v", p, err)
	}
	// Boot override via IPMI.
	if err := s.SetBootOverride(ctx, bmc.BootOverrideConfig{Target: bmc.BootPXE, Mode: bmc.BootOnce, BootMode: "uefi"}); err != nil {
		t.Fatal(err)
	}
	if b, err := s.GetBootOverride(ctx); err != nil || b.Target != bmc.BootPXE {
		t.Fatalf("boot %+v %v", b, err)
	}
	if ip.dials != 1 {
		t.Fatalf("IPMI should be dialled lazily once, got %d", ip.dials)
	}
}

func TestRestoreB650ASRR(t *testing.T) {
	f := newB650(t, "4.10.00")
	s := openSession(t, newTestDriver(t, nil), f)
	f.setJSON("GET /api/asrr/GetPowerRestore", 200, map[string]any{"powerRestore": 1})
	var got map[string]any
	f.handle("POST /api/asrr/SetPowerRestore", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		writeJSON(w, 200, `{}`)
	})
	ctx := context.Background()
	p, err := s.GetPowerRestorePolicy(ctx)
	if err != nil || p != bmc.RestoreLastState {
		t.Fatalf("restore %v %v", p, err)
	}
	if err := s.SetPowerRestorePolicy(ctx, bmc.RestoreAlwaysOff); err != nil {
		t.Fatal(err)
	}
	if v, _ := got["powerRestore"].(float64); v != 0 || got == nil {
		t.Fatalf("SetPowerRestore body %v", got)
	}
}

func TestTimePutFullObject(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	var puts []map[string]any
	f.handle("PUT /api/settings/date-time", func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		puts = append(puts, m)
		writeJSON(w, 200, `{}`)
	})
	ctx := context.Background()
	tc, err := s.Time(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !tc.NTPEnabled || tc.Timezone != "GMT+0" || len(tc.NTPServers) != 2 || tc.BMCTime.Unix() != 1794808815 {
		t.Fatalf("time: %+v", tc)
	}
	if err := s.SetTime(ctx, bmc.TimeUpdate{NTPServers: []string{"ntp1.example", "ntp2.example"}}); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 11, 16, 1, 2, 3, 0, time.UTC)
	if err := s.SetTime(ctx, bmc.TimeUpdate{Time: &when}); err != nil {
		t.Fatal(err)
	}
	if len(puts) != 2 {
		t.Fatalf("puts: %v", puts)
	}
	// Every writable field of the GET object, minus the response-only id
	// and localized_timestamp that the UI never sends (an echo including
	// them 500'd on the live X470: docs/x470/scripts/rt.py).
	wantKeys := "auto_date,daylight,primary_ntp,secondary_ntp,timestamp,timezone,utc_minutes"
	for i, p := range puts {
		if got := strings.Join(sortedKeys(p), ","); got != wantKeys {
			t.Fatalf("put %d keys %s, want the full writable object %s", i, got, wantKeys)
		}
		if p["timezone"] != "GMT+0" || p["utc_minutes"] != float64(0) {
			t.Fatalf("timezone/utc_minutes not preserved: %v", p)
		}
	}
	if puts[0]["primary_ntp"] != "ntp1.example" || puts[0]["secondary_ntp"] != "ntp2.example" || puts[0]["auto_date"] != float64(1) || puts[0]["timestamp"] != float64(1794808815) {
		t.Fatalf("ntp put: %v", puts[0])
	}
	if puts[1]["timestamp"] != float64(when.Unix()) || puts[1]["auto_date"] != float64(0) {
		t.Fatalf("manual put: %v", puts[1])
	}
	if err := s.SetTime(ctx, bmc.TimeUpdate{}); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Fatalf("empty update: %v", err)
	}
}

func TestTimeB650NTPField(t *testing.T) {
	f := newB650(t, "4.10.00")
	f.setJSON("GET /api/settings/date-time", 200, map[string]any{"id": 1, "primary_ntp": "pool.ntp.org", "secondary_ntp": "", "ntp_auto_date": 2,
		"localized_timestamp": 1794781758, "utc_minutes": 0, "timestamp": 1794781758, "timezone": "Etc/GMT"})
	var put map[string]any
	f.handle("PUT /api/settings/date-time", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&put)
		writeJSON(w, 200, `{}`)
	})
	s := openSession(t, newTestDriver(t, nil), f)
	s.mu.Lock()
	s.caps[bmc.CapTime] = bmc.BackendAMI
	s.mu.Unlock()
	tc, err := s.Time(context.Background())
	if err != nil || tc.NTPEnabled {
		t.Fatalf("ntp_auto_date 2 is not syncing: %+v %v", tc, err)
	}
	on := true
	if err := s.SetTime(context.Background(), bmc.TimeUpdate{NTPEnabled: &on}); err != nil {
		t.Fatal(err)
	}
	if put["ntp_auto_date"] != float64(1) || put["timezone"] != "Etc/GMT" {
		t.Fatalf("put %v", put)
	}
	if _, ok := put["auto_date"]; ok {
		t.Fatalf("X470 field leaked into B650 put: %v", put)
	}
}

func TestUnsupportedIs501(t *testing.T) {
	f := newB650(t, "4.10.00")
	s := openSession(t, newTestDriver(t, nil), f)
	_, err := s.OEM().Prochot(context.Background())
	if !errors.Is(err, bmc.ErrUnsupported) || bmc.HTTPStatus(err) != 501 {
		t.Fatalf("B650 prochot: %v (%d)", err, bmc.HTTPStatus(err))
	}
	if err := s.OEM().ClearMCALog(context.Background()); bmc.HTTPStatus(err) != 501 {
		t.Fatalf("B650 clearMcaLog: %v", err)
	}
	// Unknown /api endpoint -> 404 code 1010 -> 501.
	err = s.api.Get(context.Background(), "asrr/no-such-thing", nil)
	if bmc.HTTPStatus(err) != 501 {
		t.Fatalf("1010: %v", err)
	}
}

func TestFanTwoStepUnderLock(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	var mu sync.Mutex
	selected := 0
	f.handle("POST /api/asrr/settings/setopenlooptablenumber", func(w http.ResponseWriter, r *http.Request) {
		var b map[string]int
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		selected = b["table_numble"]
		mu.Unlock()
		time.Sleep(20 * time.Millisecond) // give interlopers a chance
		writeJSON(w, 200, `{}`)
	})
	f.handle("GET /api/asrr/settings/getfanopenloopcontroltable", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n := selected
		mu.Unlock()
		writeJSON(w, 200, `{"Get_Temp1": `+itoa(n*10)+`, "Get_Duty1": `+itoa(n)+`}`)
	})
	ctx := context.Background()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = s.PowerState(ctx)
			}
		}()
	}
	resp, err := s.OEM().FanTables(ctx, []oem.TableRef{{Type: "open", Table: 1}, {Type: "open", Table: 2}, {Type: "open", Table: 3}})
	close(stop)
	wg.Wait()
	if err != nil {
		t.Fatal(err)
	}
	for i, tb := range resp.Tables {
		if tb.Table != i+1 || len(tb.Duties) == 0 || tb.Duties[0] != i+1 {
			t.Fatalf("table %d read the wrong selection: %+v", i+1, tb)
		}
	}
	// No other request may sit between a select and its read.
	reqs := f.seen()
	for i, r := range reqs {
		if r.Path == "/api/asrr/settings/setopenlooptablenumber" {
			if i+1 >= len(reqs) || reqs[i+1].Path != "/api/asrr/settings/getfanopenloopcontroltable" {
				next := "<end>"
				if i+1 < len(reqs) {
					next = reqs[i+1].Method + " " + reqs[i+1].Path
				}
				t.Fatalf("interleaved request after select: %s", next)
			}
		}
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestNetworkUsersServicesX470(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	ctx := context.Background()
	nics, err := s.NetworkInterfaces(ctx)
	if err != nil || len(nics) != 2 || nics[0].Name != "eth0" || nics[0].IPv4.Address != "192.0.2.10" || nics[1].IPv4.Address != "" {
		t.Fatalf("nics %+v %v", nics, err)
	}
	us, err := s.Users(ctx)
	if err != nil || len(us) == 0 {
		t.Fatalf("users %+v %v", us, err)
	}
	var put map[string]any
	f.handle("PUT /api/settings/network/1", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&put)
		writeJSON(w, 200, `{}`)
	})
	if err := s.SetNetworkInterface(ctx, bmc.NetworkInterfaceUpdate{Name: "eth0", VLAN: &bmc.VLANConfig{Enabled: true, ID: 42}}); err != nil {
		t.Fatal(err)
	}
	if put["vlan_id"] != float64(42) || put["ipv4_address"] != "192.0.2.10" || put["mac_address"] == nil {
		t.Fatalf("network put must carry the full object: %v", put)
	}
}

func TestUsersB650Dedup(t *testing.T) {
	raw := loadFixture(t, "b550d4u-2t-1.10.json")["GET /api/settings/users"]
	f := newB650(t, "4.10.00")
	f.setJSON("GET /api/settings/users", 200, json.RawMessage(raw.Body))
	s := openSession(t, newTestDriver(t, nil), f)
	s.mu.Lock()
	s.caps[bmc.CapUsers] = bmc.BackendAMI
	s.mu.Unlock()
	us, err := s.Users(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, u := range us {
		if seen[u.ID] {
			t.Fatalf("duplicate user id %s: %+v", u.ID, us)
		}
		seen[u.ID] = true
	}
}

func TestResetBMCMarksLost(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	f.handle("PUT /api/asrr/bmccoldreset", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, `{}`) })
	if err := s.ResetBMC(context.Background(), bmc.BMCResetCold); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PowerState(context.Background()); !errors.Is(err, bmc.ErrSessionLost) {
		t.Fatalf("after reset: %v", err)
	}
}

func TestCloseLogsOut(t *testing.T) {
	f := newX470(t)
	d := newTestDriver(t, nil)
	s, err := d.Open(context.Background(), f.target(t), creds)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.logouts.Load() != 1 {
		t.Fatalf("logouts %d", f.logouts.Load())
	}
	_ = s.Close(context.Background()) // idempotent
	if f.logouts.Load() != 1 {
		t.Fatalf("double logout")
	}
}

func TestWrongPasswordIsAuth(t *testing.T) {
	f := newX470(t)
	d := newTestDriver(t, nil)
	_, err := d.Open(context.Background(), f.target(t), bmc.Creds{User: testUser, Pass: "nope"})
	if !errors.Is(err, bmc.ErrAuth) {
		t.Fatalf("Open with bad password: %v", err)
	}
}
