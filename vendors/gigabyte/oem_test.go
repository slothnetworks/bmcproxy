package gigabyte

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

func TestFanProfiles(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	fp, err := s.FanProfiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Source != bmc.BackendRedfish || fp.Mode == "" || len(fp.Profiles) < 2 {
		t.Fatalf("profiles: %+v", fp)
	}
	names := map[string]FanProfile{}
	for _, p := range fp.Profiles {
		names[p.Name] = p
	}
	def, ok := names["default"]
	if !ok || len(def.Policies) != 4 || len(def.Policies[0].Duties) == 0 {
		t.Fatalf("default profile: %+v", def)
	}
	if np, ok := names["NEW_PROFILE"]; !ok || np.Active != (fp.Mode == "NEW_PROFILE") {
		t.Fatalf("NEW_PROFILE: %+v (mode %s)", np, fp.Mode)
	}
	if f.ami.logins != 0 {
		t.Fatal("fan profiles should be Redfish-only")
	}
	dev, err := s.FanProfileDevices(ctx)
	if err != nil || len(dev.Devices) == 0 || dev.Devices[0].VendorID == "" {
		t.Fatalf("devices: %+v %v", dev, err)
	}
}

func TestFanModeViaAMIWithCSRF(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	m, err := s.FanMode(ctx)
	if err != nil || m.Mode != "NEW_PROFILE" || m.Source != bmc.BackendAMI {
		t.Fatalf("fan mode: %+v %v", m, err)
	}
	// Every AMI request carried the CSRF token (the fake kills the session
	// otherwise, which would force a second login).
	m, err = s.FanMode(ctx)
	if err != nil || f.ami.logins != 1 {
		t.Fatalf("second read: %+v %v logins=%d", m, err, f.ami.logins)
	}
	for _, r := range f.srv.Reqs() {
		if strings.HasPrefix(r.Path, "/api/settings") && r.Header.Get("X-CSRFTOKEN") == "" {
			t.Fatalf("%s %s without X-CSRFTOKEN", r.Method, r.Path)
		}
	}
}

func TestSetFanMode(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	m, err := s.SetFanMode(ctx, "default")
	if err != nil || m.Mode != "default" {
		t.Fatalf("set default: %+v %v", m, err)
	}
	if len(f.ami.posts) != 1 || f.ami.posts[0] != `{"strMode":"default"}` {
		t.Fatalf("POST bodies: %q", f.ami.posts)
	}
	if m, err := s.SetFanMode(ctx, "NEW_PROFILE"); err != nil || m.Mode != "NEW_PROFILE" {
		t.Fatalf("set NEW_PROFILE: %+v %v", m, err)
	}
	for _, bad := range []string{"nope", "", strings.Repeat("x", 65)} {
		if _, err := s.SetFanMode(ctx, bad); !errors.Is(err, bmc.ErrInvalidArgument) {
			t.Fatalf("SetFanMode(%q): %v", bad, err)
		}
	}
	if len(f.ami.posts) != 2 {
		t.Fatalf("invalid modes reached the BMC: %q", f.ami.posts)
	}
}

func TestSetFanModeRoute(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	rs := routeMap(t)
	r := httptest.NewRequest("POST", "/x", strings.NewReader(`{"mode":"default"}`))
	out, err := rs["POST setFanMode"].Handler(s, r)
	if err != nil || out.(FanMode).Mode != "default" {
		t.Fatalf("route: %+v %v", out, err)
	}
	for _, body := range []string{``, `{"mode":"default","x":1}`, `nope`} {
		r := httptest.NewRequest("POST", "/x", strings.NewReader(body))
		if _, err := rs["POST setFanMode"].Handler(s, r); !errors.Is(err, bmc.ErrInvalidArgument) {
			t.Fatalf("body %q: %v", body, err)
		}
	}
	out, err = rs["GET fanProfiles"].Handler(s, httptest.NewRequest("GET", "/x", nil))
	if err != nil || len(out.(FanProfiles).Profiles) == 0 {
		t.Fatalf("fanProfiles route: %v", err)
	}
}

func TestFanProfileUnsupportedWithoutGbtOem(t *testing.T) {
	f := newFake(t)
	f.srv.Lock()
	delete(f.srv.Resources["/redfish/v1/Chassis/Self/Thermal"], "Oem")
	f.srv.Unlock()
	s := f.open(t)
	if _, err := s.FanProfiles(ctx); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("want ErrUnsupported, got %v", err)
	}
}
