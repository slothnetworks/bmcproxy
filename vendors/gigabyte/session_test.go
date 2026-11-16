package gigabyte

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

func TestOpenProfileAndInfo(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	if p := s.Profile(); p.Key != "mc13le0" || !p.Verified || p.Redfish.SkipPageSize != 50 {
		t.Fatalf("profile: %+v", p)
	}
	in := s.Info()
	if in.Driver != Name || in.Degraded != "" {
		t.Fatalf("info: %+v", in)
	}
	var ami *bmc.BackendStatus
	for i := range in.Backends {
		if in.Backends[i].Name == bmc.BackendAMI {
			ami = &in.Backends[i]
		}
	}
	if ami == nil || ami.Authenticated {
		t.Fatalf("AMI backend should be listed and lazy: %+v", in.Backends)
	}
	if f.ami.logins != 0 {
		t.Fatal("Open logged in to the AMI /api")
	}
	sys, err := s.SystemInfo(ctx)
	if err != nil || sys.Vendor != Name {
		t.Fatalf("system: %+v %v", sys, err)
	}
}

func TestOpenUnverifiedFirmware(t *testing.T) {
	f := newFake(t)
	f.srv.Lock()
	f.srv.Resources["/redfish/v1/Managers/Self"]["FirmwareVersion"] = "13.06.30"
	f.srv.Unlock()
	s := f.open(t)
	if s.Profile().Verified || !strings.Contains(s.Info().Degraded, "13.06.30 not verified") {
		t.Fatalf("want unverified: %+v / %q", s.Profile(), s.Info().Degraded)
	}
}

func TestSensorsMergeLegacyThermalPower(t *testing.T) {
	s := newFake(t).open(t)
	ss, err := s.Sensors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]bmc.Sensor{}
	for _, x := range ss {
		byName[x.Name] = x
	}
	if len(ss) < 20 {
		t.Fatalf("only %d sensors (Thermal/Power not merged?)", len(ss))
	}
	if x, ok := byName["CPU0_TEMP"]; !ok || x.Unit != "C" {
		t.Fatalf("CPU0_TEMP: %+v (ok=%v)", x, ok)
	}
	var energy, fan bool
	for _, x := range ss {
		if x.Unit == "J" {
			energy = true
		}
		if x.Type == "fan" {
			fan = true
		}
	}
	if !energy || !fan {
		t.Fatalf("energy(J)=%v fan=%v in %+v", energy, fan, ss)
	}
}

func TestEventsPagedPast50WithEventTimes(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	f.srv.ResetLog()
	page, err := s.Events(ctx, bmc.EventQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 150 {
		t.Fatalf("got %d SEL entries, want 150 (SkipPageSize quirk)", len(page.Entries))
	}
	var queries []string
	for _, r := range f.srv.Reqs() {
		if r.Path == "/redfish/v1/Managers/Self/LogServices/SEL/Entries" {
			queries = append(queries, r.RawQuery)
		}
		if strings.Contains(r.RawQuery, "$skip=0") {
			t.Fatalf("sent $skip=0 (400 on 13.04.15)")
		}
	}
	if len(queries) < 3 || queries[0] != "" || queries[1] != "$skip=50" {
		t.Fatalf("SEL queries: %q", queries)
	}
	var e *bmc.SELEntry
	for i := range page.Entries {
		if page.Entries[i].ID == "168" {
			e = &page.Entries[i]
		}
	}
	if e == nil {
		t.Fatal("entry 168 missing")
	}
	want := time.Date(2025, 3, 25, 16, 7, 47, 0, time.UTC)
	if !e.Time.Raw.Equal(want) || e.Oem["created"] != "2025-09-17T16:54:49Z" {
		t.Fatalf("entry 168 time %v created %v, want %v", e.Time.Raw, e.Oem["created"], want)
	}
	distinct := map[time.Time]bool{}
	for _, x := range page.Entries {
		distinct[x.Time.Raw] = true
	}
	if len(distinct) < 10 {
		t.Fatalf("only %d distinct event times: Created import time leaked through", len(distinct))
	}
}

func TestAuditLogPagedPast50(t *testing.T) {
	s := newFake(t).open(t)
	al, err := s.AuditLog(ctx, bmc.AuditQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(al) != 68 {
		t.Fatalf("got %d audit entries, want 68", len(al))
	}
}

func TestFRUAndBaseboardSerialFromOem(t *testing.T) {
	s := newFake(t).open(t)
	frus, err := s.FRU(ctx)
	if err != nil || len(frus) == 0 || frus[0].Board.Serial != fakeSerial {
		t.Fatalf("FRU: %+v %v", frus, err)
	}
	bb, err := s.Baseboard(ctx)
	if err != nil || bb.Serial != fakeSerial {
		t.Fatalf("baseboard: %+v %v", bb, err)
	}
	// The capture's placeholder serial is not used.
	if usable("0123456789") || usable("To be filled by O.E.M.") || !usable(fakeSerial) {
		t.Fatal("usable()")
	}
}

func TestFirmwareInventory(t *testing.T) {
	s := newFake(t).open(t)
	fw, err := s.Firmware(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range fw {
		got[c.Name] = c.Version
	}
	found := false
	for _, v := range got {
		if v == "13.04.15" {
			found = true
		}
	}
	if !found {
		t.Fatalf("BMC 13.04.15 not in firmware inventory: %v", got)
	}
}

func TestSELEventTime(t *testing.T) {
	for _, tc := range []struct {
		msg, ets string
		want     string
		ok       bool
	}{
		{"x, Timestamp : 2025-03-25T16:07:47+00:00, y", "2025-03-25T16:07:47+00:05", "2025-03-25T16:07:47Z", true},
		{"no stamp", "2025-03-25T16:07:47+00:05", "2025-03-25T16:07:47Z", true},
		{"Timestamp : 2025-03-25T18:07:47+02:00", "", "2025-03-25T16:07:47Z", true},
		{"", "", "", false},
	} {
		got, ok := SELEventTime(tc.msg, tc.ets)
		if ok != tc.ok || (ok && got.Format(time.RFC3339) != tc.want) {
			t.Errorf("SELEventTime(%q,%q) = %v %v, want %s %v", tc.msg, tc.ets, got, ok, tc.want, tc.ok)
		}
	}
}

// NetworkProtocol PATCH on AMI 13.x is an async task (202 + Location,
// task-service/lua_script/patch-networkprotcol.lua); SetTime must wait for
// the task and verify the read-back.
func TestSetTimeNTPAsync202(t *testing.T) {
	const npPath = "/redfish/v1/Managers/Self/NetworkProtocol"
	const taskPath = "/redfish/v1/TaskService/Tasks/1"
	f := newFake(t)
	var polls atomic.Int32
	var patched atomic.Value
	f.srv.Lock()
	f.srv.Handlers[npPath] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPatch {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			patched.Store(body)
			w.Header().Set("Location", taskPath)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"@odata.id":"` + taskPath + `","@odata.type":"#Task.v1_4_3.Task","TaskState":"New"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(f.srv.Get(npPath))
	}
	f.srv.Handlers[taskPath] = func(w http.ResponseWriter, r *http.Request) {
		state := "Running"
		if polls.Add(1) >= 3 {
			state = "Completed"
			np := f.srv.Get(npPath)
			ntp := np["NTP"].(map[string]any)
			ntp["ProtocolEnabled"] = true
			ntp["NTPServers"] = []any{"192.0.2.123", "time.nist.gov"}
			f.srv.Set(npPath, np)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"@odata.id": taskPath, "Id": "1", "TaskState": state, "TaskStatus": "OK"})
	}
	f.srv.Unlock()
	s := f.open(t)
	en := true
	if err := s.SetTime(ctx, bmc.TimeUpdate{NTPEnabled: &en, NTPServers: []string{"192.0.2.123"}}); err != nil {
		t.Fatalf("SetTime: %v", err)
	}
	if polls.Load() < 3 || patched.Load() == nil {
		t.Fatalf("task polled %d times; patch %v", polls.Load(), patched.Load())
	}
	tc, err := s.Time(ctx)
	if err != nil || !tc.NTPEnabled {
		t.Fatalf("time after set: %+v %v", tc, err)
	}
}
