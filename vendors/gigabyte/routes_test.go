package gigabyte

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

func routeMap(t *testing.T) map[string]bmc.Route {
	t.Helper()
	m := map[string]bmc.Route{}
	for _, r := range Default.VendorRoutes() {
		m[r.Method+" "+r.Path] = r
	}
	return m
}

func TestVendorRouteTiers(t *testing.T) {
	rs := routeMap(t)
	for k, want := range map[string]bmc.Tier{
		"GET fanProfiles":        bmc.TierRead,
		"GET fanProfileDevices":  bmc.TierRead,
		"GET fanMode":            bmc.TierRead,
		"POST setFanMode":        bmc.TierWrite,
		"GET firmwareInfo":       bmc.TierRead,
		"GET bonding":            bmc.TierRead,
		"POST enableBonding":     bmc.TierDisruptive,
		"POST disableBonding":    bmc.TierDisruptive,
		" raw/api/{path...}":     bmc.TierDisruptive,
		" raw/redfish/{path...}": bmc.TierDisruptive,
	} {
		r, ok := rs[k]
		if !ok {
			t.Errorf("route %q missing", k)
			continue
		}
		if r.Tier != want {
			t.Errorf("%q tier %v, want %v", k, r.Tier, want)
		}
		if r.Handler == nil || r.Summary == "" {
			t.Errorf("%q incomplete", k)
		}
	}
	if len(rs) != 10 {
		t.Errorf("%d routes, want 10", len(rs))
	}
}

func rawReq(method, p string) *http.Request {
	r := httptest.NewRequest(method, "/x", nil)
	r.SetPathValue("path", p)
	return r
}

func TestRawTiersAndDenylist(t *testing.T) {
	rs := routeMap(t)
	api, red := rs[" raw/api/{path...}"].TierFor, rs[" raw/redfish/{path...}"].TierFor
	type tc struct {
		method, path string
		tier         bmc.Tier
		err          error
	}
	for _, c := range []tc{
		{"GET", "chassis-status", bmc.TierRead, nil},
		{"GET", "settings/fanprofile/mode", bmc.TierRead, nil},
		{"POST", "settings/fanprofile/mode", bmc.TierWrite, nil},
		{"PUT", "settings/fanprofile", bmc.TierWrite, nil},
		{"DELETE", "settings/fanprofile/collection", bmc.TierDisruptive, nil},
		{"POST", "maintenance/restore_defaults", bmc.TierDisruptive, nil},
		{"PUT", "settings/network/1", bmc.TierDisruptive, nil},
		{"GET", "settings/network-bond", bmc.TierRead, nil},
		{"PUT", "settings/network-bond", bmc.TierDisruptive, nil},
		{"POST", "settings/network-bond", bmc.TierDisruptive, nil},
		{"PUT", "settings/one-touch-settings", bmc.TierDisruptive, nil},
		{"POST", "GBT/GpuManager/NVAPI/x", bmc.TierDisruptive, nil},
		{"GET", "logs/help-data", 0, bmc.ErrForbidden},
		{"GET", "hostinterface/diag/run", 0, bmc.ErrForbidden},
		{"GET", "logs/audit-file", 0, bmc.ErrForbidden},
		{"GET", "maintenance/firmware/dump", 0, bmc.ErrForbidden},
		{"POST", "save/bmcbrokenimage", 0, bmc.ErrForbidden},
		{"GET", "settings/download_image", 0, bmc.ErrForbidden},
		{"GET", "HOSTINTERFACE/Diag", 0, bmc.ErrForbidden},
		{"DELETE", "session", 0, bmc.ErrForbidden},
		{"POST", "/api/session", 0, bmc.ErrForbidden},
		{"GET", "settings/../session", 0, bmc.ErrForbidden},
	} {
		tier, err := api(rawReq(c.method, c.path))
		if c.err != nil {
			if !errors.Is(err, c.err) {
				t.Errorf("raw/api %s %s: err %v, want %v", c.method, c.path, err, c.err)
			}
			continue
		}
		if err != nil || tier != c.tier {
			t.Errorf("raw/api %s %s: %v %v, want %v", c.method, c.path, tier, err, c.tier)
		}
	}
	for _, c := range []tc{
		{"GET", "Systems/Self", bmc.TierRead, nil},
		{"GET", "redfish/v1/Chassis/Self/Thermal/FanprofileService/Fanprofile", bmc.TierRead, nil},
		{"PUT", "Chassis/Self/Thermal/FanprofileService/Fanprofile", bmc.TierWrite, nil},
		{"PATCH", "Systems/Self", bmc.TierWrite, nil},
		{"PATCH", "Managers/Self/NetworkProtocol", bmc.TierDisruptive, nil},
		{"POST", "Systems/Self/Actions/ComputerSystem.Reset", bmc.TierDisruptive, nil},
		{"POST", "Managers/Self/Actions/Oem/AMIManager.RedfishDBReset", bmc.TierDisruptive, nil},
		{"DELETE", "AccountService/Accounts/3", bmc.TierDisruptive, nil},
		{"GET", "SessionService/Sessions", bmc.TierRead, nil},
		{"POST", "SessionService/Sessions", 0, bmc.ErrForbidden},
		{"DELETE", "SessionService/Sessions/1", 0, bmc.ErrForbidden},
		{"GET", "GlobalLuaTables", 0, bmc.ErrForbidden},
		{"GET", "../../api/session", 0, bmc.ErrInvalidArgument},
		{"GET", "Systems/%5cx", 0, bmc.ErrInvalidArgument},
		{"GET", "Systems/%2e%2e/GlobalLuaTables", 0, bmc.ErrForbidden},
	} {
		tier, err := red(rawReq(c.method, c.path))
		if c.err != nil {
			if !errors.Is(err, c.err) {
				t.Errorf("raw/redfish %s %s: err %v, want %v", c.method, c.path, err, c.err)
			}
			continue
		}
		if err != nil || tier != c.tier {
			t.Errorf("raw/redfish %s %s: %v %v, want %v", c.method, c.path, tier, err, c.tier)
		}
	}
}

func call(t *testing.T, s *Session, route bmc.Route, r *http.Request) (any, error) {
	t.Helper()
	return route.Handler(s, r)
}

func TestRawPassthroughsEndToEnd(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	rs := routeMap(t)

	out, err := call(t, s, rs[" raw/redfish/{path...}"], rawReq("GET", "Systems/Self"))
	if err != nil {
		t.Fatal(err)
	}
	rr := out.(*bmc.RawResponse)
	if rr.Status != 200 || !strings.Contains(string(rr.Body), "MC13-LE0-000") {
		t.Fatalf("raw redfish: %d %.200s", rr.Status, rr.Body)
	}

	out, err = call(t, s, rs[" raw/api/{path...}"], rawReq("GET", "chassis-status"))
	if err != nil {
		t.Fatal(err)
	}
	rr = out.(*bmc.RawResponse)
	var cs map[string]any
	if rr.Status != 200 || json.Unmarshal(rr.Body, &cs) != nil || cs["power_status"] == nil {
		t.Fatalf("raw api: %d %s", rr.Status, rr.Body)
	}
	if f.ami.logins != 1 {
		t.Fatalf("AMI logins %d, want 1 (lazy)", f.ami.logins)
	}
	// Denylisted path refused by the handler too (not only TierFor).
	if _, err := call(t, s, rs[" raw/api/{path...}"], rawReq("GET", "logs/help-data")); !errors.Is(err, bmc.ErrForbidden) {
		t.Fatalf("handler denylist: %v", err)
	}
	// Close logs out of the AMI session.
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if f.ami.logouts != 1 || f.ami.liveSessions() != 0 {
		t.Fatalf("logouts=%d live=%d", f.ami.logouts, f.ami.liveSessions())
	}
}

func TestWrongSessionType(t *testing.T) {
	rs := routeMap(t)
	if _, err := rs["GET fanMode"].Handler(nil, httptest.NewRequest("GET", "/x", nil)); !errors.Is(err, bmc.ErrNotFound) {
		t.Fatalf("want ErrNotFound for a foreign session, got %v", err)
	}
}
