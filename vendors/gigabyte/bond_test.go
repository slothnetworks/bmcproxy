package gigabyte

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// Live settings/network-bond captures (identical on both boards, 2026-11-16).
const (
	mc13BondFixture = "testdata/network-bond-mc13le0-13.04.15.json"
	mz32BondFixture = "testdata/network-bond-mz32ar0-12.61.39.json"
)

// fakeBond serves AMI settings/network-bond on the fake: GET returns the
// object, PUT must carry every key of it (AMI 500s on partial bodies).
type fakeBond struct {
	mu   sync.Mutex
	obj  map[string]any
	gets int
	puts []map[string]any
}

func (b *fakeBond) snapshot() (int, []map[string]any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.gets, append([]map[string]any(nil), b.puts...)
}

// addBond installs settings/network-bond on the fake, seeded from a live
// fixture file.
func addBond(t *testing.T, f *fake, file string) *fakeBond {
	t.Helper()
	fx := loadFixture(t, file)
	e, ok := fx.API["GET /api/settings/network-bond"]
	if !ok || e.Status != 200 {
		t.Fatalf("%s: no settings/network-bond capture", file)
	}
	// Round-trip so numbers are float64 as on the wire.
	raw, _ := json.Marshal(e.Body)
	b := &fakeBond{}
	if err := json.Unmarshal(raw, &b.obj); err != nil {
		t.Fatal(err)
	}
	f.srv.Lock()
	f.srv.Handlers["/api/settings/network-bond"] = func(w http.ResponseWriter, r *http.Request) {
		if !f.ami.check(w, r) {
			return
		}
		b.mu.Lock()
		defer b.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			b.gets++
			writeJSON(w, http.StatusOK, b.obj)
		case http.MethodPut:
			var m map[string]any
			if err := json.Unmarshal([]byte(f.ami.body(r)), &m); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": 1001})
				return
			}
			for k := range b.obj {
				if _, ok := m[k]; !ok {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Required Variable Not Found", "code": 1001})
					return
				}
			}
			b.puts = append(b.puts, m)
			b.obj = m
			writeJSON(w, http.StatusOK, m)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"cc": 1})
		}
	}
	f.srv.Unlock()
	return b
}

type bondBoard struct {
	name    string
	fixture string
	open    func(t *testing.T) *fake
}

var bondBoards = []bondBoard{
	{"MC13-LE0", mc13BondFixture, func(t *testing.T) *fake { return newFake(t) }},
	{"MZ32-AR0", mz32BondFixture, newMZ32},
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestBondReadLiveShape: the live object normalises to bmc.BondConfig and
// SessionInfo reports bond -> ami on both boards.
func TestBondReadLiveShape(t *testing.T) {
	for _, bb := range bondBoards {
		t.Run(bb.name, func(t *testing.T) {
			f := bb.open(t)
			addBond(t, f, bb.fixture)
			s := f.open(t)
			if got := s.Info().Capabilities[bmc.CapBond]; got != bmc.BackendAMI {
				t.Fatalf("bond backend %q, want ami", got)
			}
			if !bmc.Implements(s, bmc.CapBond) {
				t.Fatal("session does not implement BondManager")
			}
			bc, err := s.Bond(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if bc.ID != 1 || !bc.Enabled || bc.Mode != "active-backup" || len(bc.Interfaces) != 1 || bc.Interfaces[0] != "both" || !bc.AutoConfig {
				t.Fatalf("bond: %+v", bc)
			}
			if got := strings.Join(keys(bc.Oem), ","); got != "auto_configuration_enable,bond_enable,bond_ifc,bond_mode,id" {
				t.Fatalf("oem keys %s", got)
			}
			// Vendor route: same object, Gigabyte field names.
			out, err := routeMap(t)["GET bonding"].Handler(s, httptest.NewRequest("GET", "/x", nil))
			if err != nil {
				t.Fatal(err)
			}
			st := out.(BondingStatus)
			if st.ID != 1 || !st.Enabled || st.Mode != "active-backup" || st.Interface != "both" || !st.AutoConfig {
				t.Fatalf("bonding route: %+v", st)
			}
		})
	}
}

// TestSetBondFullObjectPut: SetBond GETs the object, mutates it and PUTs
// the whole object (same key set) back.
func TestSetBondFullObjectPut(t *testing.T) {
	for _, bb := range bondBoards {
		t.Run(bb.name, func(t *testing.T) {
			f := bb.open(t)
			b := addBond(t, f, bb.fixture)
			s := f.open(t)
			bc, err := s.Bond(ctx)
			if err != nil {
				t.Fatal(err)
			}
			// Un-bond, keeping eth1 (dedicated), auto configuration off.
			bc.Enabled, bc.Interfaces, bc.AutoConfig = false, []string{"eth1"}, false
			if err := s.SetBond(ctx, bc); err != nil {
				t.Fatal(err)
			}
			gets, puts := b.snapshot()
			if len(puts) != 1 || gets != 2 {
				t.Fatalf("%d GETs / %d PUTs, want 2 / 1", gets, len(puts))
			}
			p := puts[0]
			if got := strings.Join(keys(p), ","); got != "auto_configuration_enable,bond_enable,bond_ifc,bond_mode,id" {
				t.Fatalf("PUT keys %s (body %v)", got, p)
			}
			for k, want := range map[string]any{"id": float64(1), "bond_enable": float64(0), "bond_mode": "active-backup", "bond_ifc": "eth1", "auto_configuration_enable": float64(0)} {
				if p[k] != want {
					t.Errorf("PUT %s = %v, want %v (body %v)", k, p[k], want, p)
				}
			}
			if f.ami.logins != 1 {
				t.Fatalf("AMI logins %d, want 1 (lazy, reused)", f.ami.logins)
			}
		})
	}
}

// TestBondVendorRoutes: enableBonding / disableBonding through the route
// handlers (the server applies the disruptive tier; TestVendorRouteTiers
// pins it).
func TestBondVendorRoutes(t *testing.T) {
	f := newMZ32(t)
	b := addBond(t, f, mz32BondFixture)
	s := f.open(t)
	rs := routeMap(t)
	for _, k := range []string{"POST enableBonding", "POST disableBonding"} {
		if rs[k].Tier != bmc.TierDisruptive {
			t.Fatalf("%s tier %v", k, rs[k].Tier)
		}
	}
	req := func(body string) *http.Request {
		return httptest.NewRequest("POST", "/x", strings.NewReader(body))
	}
	out, err := rs["POST disableBonding"].Handler(s, req(`{"interface":"eth0"}`))
	if err != nil {
		t.Fatal(err)
	}
	if st := out.(BondingStatus); st.Enabled || st.Interface != "eth0" {
		t.Fatalf("disable: %+v", st)
	}
	// Empty body: enable with the current interface.
	out, err = rs["POST enableBonding"].Handler(s, req(``))
	if err != nil {
		t.Fatal(err)
	}
	if st := out.(BondingStatus); !st.Enabled || st.Interface != "eth0" || st.Mode != "active-backup" {
		t.Fatalf("enable: %+v", st)
	}
	out, err = rs["POST enableBonding"].Handler(s, req(`{"interface":"both","mode":"active-backup"}`))
	if err != nil || out.(BondingStatus).Interface != "both" {
		t.Fatalf("enable both: %+v %v", out, err)
	}
	for _, bad := range []struct{ route, body string }{
		{"POST enableBonding", `{"mode":"balance-rr"}`},
		{"POST enableBonding", `{"interface":"eth2"}`},
		{"POST enableBonding", `{"bond_ifc":"eth0"}`},
		{"POST disableBonding", `{"interface":"both"}`},
	} {
		if _, err := rs[bad.route].Handler(s, req(bad.body)); !errors.Is(err, bmc.ErrInvalidArgument) {
			t.Errorf("%s %s: %v, want invalid argument", bad.route, bad.body, err)
		}
	}
	if _, puts := b.snapshot(); len(puts) != 3 {
		t.Fatalf("%d PUTs, want 3", len(puts))
	}
}

func TestBondValidation(t *testing.T) {
	f := newFake(t)
	b := addBond(t, f, mc13BondFixture)
	s := f.open(t)
	for _, tc := range []struct {
		enable    bool
		mode, ifc string
	}{
		{true, "", "eth9"},           // unknown bond_ifc
		{false, "", "both"},          // un-bonding needs one interface
		{true, "balance-rr", "eth0"}, // bond_mode not configurable
	} {
		if _, err := s.SetBonding(ctx, tc.enable, tc.mode, tc.ifc, nil); !errors.Is(err, bmc.ErrInvalidArgument) {
			t.Errorf("%+v: %v, want invalid argument", tc, err)
		}
	}
	// Generic SetBond of the current (bonded, "both") object with only
	// Enabled flipped is refused too: un-bonding must name the survivor.
	bc, err := s.Bond(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bc.Enabled = false
	if err := s.SetBond(ctx, bc); !errors.Is(err, bmc.ErrInvalidArgument) {
		t.Fatalf("SetBond un-bond with both: %v", err)
	}
	if _, puts := b.snapshot(); len(puts) != 0 {
		t.Fatalf("%d invalid writes reached the BMC", len(puts))
	}
	st, err := s.SetBonding(ctx, true, "active-backup", "eth0", nil)
	if err != nil || !st.Enabled || st.Interface != "eth0" {
		t.Fatalf("enable: %+v %v", st, err)
	}
}

// TestBondUnsupportedWithoutAMI: no MegaRAC /api -> 501 from the generic
// methods and the vendor routes, and bond is not reported.
func TestBondUnsupportedWithoutAMI(t *testing.T) {
	f := newFake(t)
	s := f.open(t)
	s.ami = nil
	if _, err := s.Bond(ctx); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("Bond without /api: %v", err)
	}
	if err := s.SetBond(ctx, bmc.BondConfig{Enabled: true}); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("SetBond without /api: %v", err)
	}
	rs := routeMap(t)
	if _, err := rs["GET bonding"].Handler(s, httptest.NewRequest("GET", "/x", nil)); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("bonding route without /api: %v", err)
	}
	if _, err := rs["POST enableBonding"].Handler(s, httptest.NewRequest("POST", "/x", strings.NewReader(`{}`))); !errors.Is(err, bmc.ErrUnsupported) {
		t.Fatalf("enableBonding route without /api: %v", err)
	}
	if _, ok := s.Info().Capabilities[bmc.CapBond]; ok {
		t.Fatal("bond reported without an AMI /api")
	}
}
