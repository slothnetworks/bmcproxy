package asrock

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

func routeByKey(t *testing.T, method, path string) bmc.Route {
	t.Helper()
	for _, r := range Default.VendorRoutes() {
		if r.Method == method && r.Path == path {
			return r
		}
	}
	t.Fatalf("no route %s %s", method, path)
	return bmc.Route{}
}

func TestVendorRouteTiers(t *testing.T) {
	want := map[string]bmc.Tier{
		"GET bonding": bmc.TierRead, "POST enableBonding": bmc.TierDisruptive, "POST disableBonding": bmc.TierDisruptive,
		"GET ncsiMode": bmc.TierRead, "POST setNcsiMode": bmc.TierDisruptive,
		"GET keepNicLinkUp": bmc.TierRead, "POST setKeepNicLinkUp": bmc.TierWrite,
		"GET fanMode": bmc.TierRead, "POST setFanMode": bmc.TierWrite,
		"GET fanTables": bmc.TierRead, "POST setFanTable": bmc.TierWrite,
		"POST setManualFanDuty": bmc.TierWrite, "GET fanDuty": bmc.TierRead, "POST resetFanSettings": bmc.TierWrite,
		"GET powerSource": bmc.TierRead, "GET postCode": bmc.TierRead,
		"GET mcaLog": bmc.TierRead, "POST clearMcaLog": bmc.TierDisruptive,
		"GET prochot": bmc.TierRead, "POST setProchot": bmc.TierWrite,
		"GET webEnableTimeout": bmc.TierRead, "POST setWebEnableTimeout": bmc.TierWrite,
		"GET biosUpdateOption": bmc.TierRead, "GET componentStatus/{component}": bmc.TierRead,
		"POST updateComponent/{component}": bmc.TierDisruptive, "POST bmcColdReset": bmc.TierDisruptive,
	}
	got := map[string]bmc.Tier{}
	for _, r := range Default.VendorRoutes() {
		if strings.HasPrefix(r.Path, "raw/") {
			if r.TierFor == nil || r.Method != "" {
				t.Fatalf("raw route %s must be ANY with TierFor", r.Path)
			}
			continue
		}
		got[r.Method+" "+r.Path] = r.Tier
	}
	for k, w := range want {
		if g, ok := got[k]; !ok || g != w {
			t.Errorf("%s: tier %v (present %v), want %v", k, g, ok, w)
		}
	}
	if len(got) != len(want) {
		t.Errorf("route count %d, want %d", len(got), len(want))
	}
}

func rawReq(method, prefix, p string) *http.Request {
	r := httptest.NewRequest(method, "/api/v1/vendor/asrock/"+prefix+"/"+p, nil)
	r.SetPathValue("path", p)
	return r
}

func TestRawTierAndDenylist(t *testing.T) {
	api := routeByKey(t, "", "raw/api/{path...}")
	rfr := routeByKey(t, "", "raw/redfish/{path...}")
	cases := []struct {
		rt     bmc.Route
		method string
		path   string
		tier   bmc.Tier
		err    error
	}{
		{api, "GET", "chassis-status", bmc.TierRead, nil},
		{api, "POST", "actions/chassis-led", bmc.TierWrite, nil},
		{api, "PUT", "settings/network-bond", bmc.TierDisruptive, nil},
		{api, "PUT", "maintenance/firmware", bmc.TierDisruptive, nil},
		{api, "POST", "maintenance/restore_defaults", bmc.TierDisruptive, nil},
		{api, "DELETE", "logs/event", bmc.TierDisruptive, nil},
		{api, "GET", "logs/help-data", 0, bmc.ErrForbidden},
		{api, "GET", "logs/../logs/help-data", 0, bmc.ErrForbidden},
		{api, "GET", "asrr/mca-file", 0, bmc.ErrForbidden},
		{api, "GET", "asrr/java-console", 0, bmc.ErrForbidden},
		{api, "DELETE", "session", 0, bmc.ErrForbidden},
		{api, "GET", "%2e%2e/%2e%2e/etc/passwd", 0, nil}, // canonicalised under /api; not an error by itself
		{rfr, "GET", "Systems/Self", bmc.TierRead, nil},
		{rfr, "PATCH", "Systems/Self", bmc.TierWrite, nil},
		{rfr, "POST", "Managers/Self/Actions/Manager.Reset", bmc.TierDisruptive, nil},
		{rfr, "PATCH", "AccountService/Accounts/2", bmc.TierDisruptive, nil},
		{rfr, "GET", "GlobalLuaTables", 0, bmc.ErrForbidden},
		{rfr, "DELETE", "Oem/Ami/GlobalLuaTables", 0, bmc.ErrForbidden},
		{rfr, "POST", "SessionService/Sessions", 0, bmc.ErrForbidden},
	}
	for _, c := range cases {
		tier, err := c.rt.EffectiveTier(rawReq(c.method, strings.TrimSuffix(c.rt.Path, "/{path...}"), c.path))
		if c.err != nil {
			if !errors.Is(err, c.err) {
				t.Errorf("%s %s %s: err %v, want %v", c.method, c.rt.Path, c.path, err, c.err)
			}
			continue
		}
		if c.path == "%2e%2e/%2e%2e/etc/passwd" {
			continue
		}
		if err != nil || tier != c.tier {
			t.Errorf("%s %s %s: tier %v err %v, want %v", c.method, c.rt.Path, c.path, tier, err, c.tier)
		}
	}
}

func TestRawAPIPassthrough(t *testing.T) {
	f := newX470(t)
	s := openSession(t, newTestDriver(t, nil), f)
	rt := routeByKey(t, "", "raw/api/{path...}")
	r := rawReq("GET", "raw/api", "asrr/fw-info")
	res, err := rt.Handler(s, r)
	if err != nil {
		t.Fatal(err)
	}
	rr, ok := res.(*bmc.RawResponse)
	if !ok || rr.Status != 200 || !strings.Contains(string(rr.Body), "2.32.00") {
		t.Fatalf("raw: %#v", res)
	}
	// Status passthrough (404 1010 is returned verbatim, not mapped).
	res, err = rt.Handler(s, rawReq("GET", "raw/api", "no/such"))
	if err != nil || res.(*bmc.RawResponse).Status != 404 {
		t.Fatalf("raw 404: %v %#v", err, res)
	}
	// Denylisted path never reaches the BMC even if TierFor were bypassed.
	before := len(f.seen())
	if _, err := rt.Handler(s, rawReq("GET", "raw/api", "logs/help-data")); !errors.Is(err, bmc.ErrForbidden) {
		t.Fatalf("help-data: %v", err)
	}
	if len(f.seen()) != before {
		t.Fatal("denylisted request reached the BMC")
	}
	// raw/redfish on X470 -> 501.
	rf := routeByKey(t, "", "raw/redfish/{path...}")
	if _, err := rf.Handler(s, rawReq("GET", "raw/redfish", "Systems/Self")); bmc.HTTPStatus(err) != 501 {
		t.Fatalf("X470 raw redfish: %v", err)
	}
}

func TestRawRedfishB650(t *testing.T) {
	f := newB650(t, "4.10.00")
	s := openSession(t, newTestDriver(t, nil), f)
	rf := routeByKey(t, "", "raw/redfish/{path...}")
	res, err := rf.Handler(s, rawReq("GET", "raw/redfish", "Systems/Self"))
	if err != nil {
		t.Fatal(err)
	}
	if rr := res.(*bmc.RawResponse); rr.Status != 200 || !strings.Contains(string(rr.Body), "ComputerSystem") {
		t.Fatalf("raw redfish: %d %s", rr.Status, rr.Body)
	}
}

func TestOEMDialects(t *testing.T) {
	ctx := context.Background()
	t.Run("x470 vetobit + asrr fan", func(t *testing.T) {
		f := newX470(t)
		f.setJSON("GET /api/asrr/getvetobit", 200, map[string]any{"vetobit": 1})
		f.setJSON("GET /api/asrr/settings/getfancontrolmode", 200, map[string]any{"mode1": 0, "mode2": 1})
		s := openSession(t, newTestDriver(t, nil), f)
		k, err := s.OEM().KeepNicLinkUp(ctx)
		if err != nil || !k.Enabled || k.Dialect != "vetobit" {
			t.Fatalf("vetobit %+v %v", k, err)
		}
		m, err := s.OEM().FanMode(ctx)
		if err != nil || m.Modes["2"] != 1 {
			t.Fatalf("fan mode %+v %v", m, err)
		}
		d, err := s.OEM().FanDuty(ctx)
		if err != nil || len(d.Duties) != 8 {
			t.Fatalf("fan duty %+v %v", d, err)
		}
		b, err := s.OEM().Bonding(ctx)
		if err != nil || b.Enabled || b.Mode != "active-backup" {
			t.Fatalf("bonding %+v %v", b, err)
		}
	})
	t.Run("b650 4.x ncsilinkup + settings fan", func(t *testing.T) {
		f := newB650(t, "4.10.00")
		f.setJSON("GET /api/settings/ncsilinkup", 200, map[string]any{"sharednic": 1, "sharednic2": 0})
		f.setJSON("GET /api/settings/fan/mode", 200, map[string]any{"mode": 0})
		var put map[string]any
		f.handle("PUT /api/settings/ncsilinkup", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&put)
			writeJSON(w, 200, `{}`)
		})
		s := openSession(t, newTestDriver(t, nil), f)
		k, err := s.OEM().KeepNicLinkUp(ctx)
		if err != nil || k.Dialect != "ncsilinkup" || !k.Enabled {
			t.Fatalf("ncsilinkup %+v %v", k, err)
		}
		on := true
		if _, err := s.OEM().SetKeepNicLinkUp(ctx, setKeep(&on)); err != nil {
			t.Fatal(err)
		}
		if put["sharednic"] != float64(1) || put["sharednic2"] != float64(1) {
			t.Fatalf("ncsilinkup put %v", put)
		}
		m, err := s.OEM().FanMode(ctx)
		if err != nil || m.Dialect != "settings/fan" {
			t.Fatalf("fan %+v %v", m, err)
		}
		if n := len(f.seenPath("GET", "/api/asrr/getvetobit")); n != 0 {
			t.Fatalf("4.x must not probe vetobit (%d)", n)
		}
	})
}

// TestEndToEndServer drives the driver through the real northbound server
// (sessions manager, tier gate, vendor route mounting).
func TestEndToEndServer(t *testing.T) {
	f := newX470(t)
	d := newTestDriver(t, nil)
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	req := func(method, path string, hdr map[string]string, body any) *servertest.Resp {
		return h.Do(servertest.Req{Method: method, Path: path, Host: f.host(), User: testUser, Pass: testPass, Header: hdr, Body: body})
	}
	r := req("GET", "/api/v1/capabilities", nil, nil)
	if r.Status != 200 || !strings.Contains(string(r.Body), `"asrock"`) {
		t.Fatalf("capabilities: %d %s", r.Status, r.Body)
	}
	r = req("GET", "/api/v1/vendor/asrock/bonding", nil, nil)
	if r.Status != 200 || !strings.Contains(string(r.Body), `"active-backup"`) {
		t.Fatalf("bonding: %d %s", r.Status, r.Body)
	}
	r = req("GET", "/api/v1/vendor/asrock/raw/api/logs/help-data", nil, nil)
	if r.Status != 403 || !strings.Contains(string(r.Body), "forbidden") {
		t.Fatalf("raw denylist: %d %s", r.Status, r.Body)
	}
	r = req("GET", "/api/v1/vendor/asrock/raw/api/chassis-status", nil, nil)
	if r.Status != 200 || !strings.Contains(string(r.Body), "power_status") {
		t.Fatalf("raw passthrough: %d %s", r.Status, r.Body)
	}
	r = req("GET", "/api/v1/vendor/asrock/componentStatus/NOPE", nil, nil)
	if r.Status != 400 {
		t.Fatalf("bad component: %d %s", r.Status, r.Body)
	}
	r = req("GET", "/api/v1/sensors", nil, nil)
	if r.Status != 200 || !strings.Contains(string(r.Body), "3VSB") {
		t.Fatalf("sensors: %d %.300s", r.Status, r.Body)
	}
}

func setKeep(on *bool) oem.SetKeepNicLinkUpRequest { return oem.SetKeepNicLinkUpRequest{Enabled: on} }
