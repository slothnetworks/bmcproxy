package v1_test

import (
	"context"
	"testing"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

const bondPath = "/api/v1/network/bond"

var confirmHdr = map[string]string{"X-BMC-Confirm": "true"}

// TestBondRead: GET /api/v1/network/bond is a cached read of BondManager.
func TestBondRead(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Configure: func(s *servertest.Session) {
		s.BondCfg = bmc.BondConfig{ID: 1, Enabled: true, Mode: "active-backup", Interfaces: []string{"both"}, AutoConfig: true,
			Oem: map[string]any{"bond_ifc": "both"}}
	}}
	h := harness(t, d, servertest.Options{})
	var b v1.BondResponse
	r := h.Do(servertest.Req{Path: bondPath, Host: host})
	if r.Status != 200 {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
	r.JSON(t, &b)
	if b.ID != 1 || !b.Enabled || b.Mode != "active-backup" || !b.AutoConfig || len(b.Interfaces) != 1 || b.Interfaces[0] != "both" ||
		b.Oem["bond_ifc"] != "both" {
		t.Fatalf("bond: %+v", b)
	}
	h.Do(servertest.Req{Path: bondPath, Host: host})
	if n := d.Sessions()[0].Calls("Bond"); n != 1 {
		t.Fatalf("Bond called %d times, want 1 (cached)", n)
	}
}

// TestBondPutDisruptiveGating: PUT needs --allow-disruptive AND
// X-BMC-Confirm; denied requests never reach the driver.
func TestBondPutDisruptiveGating(t *testing.T) {
	body := map[string]any{"enabled": true, "interfaces": []string{"eth1"}}
	for _, tc := range []struct {
		name    string
		allow   bool
		hdr     map[string]string
		status  int
		class   string
		setBond int
	}{
		{"no flag, no confirm", false, nil, 403, "disruptive_denied", 0},
		{"no flag, confirm", false, confirmHdr, 403, "disruptive_denied", 0},
		{"flag, no confirm", true, nil, 403, "disruptive_denied", 0},
		{"flag, confirm=false", true, map[string]string{"X-BMC-Confirm": "false"}, 403, "disruptive_denied", 0},
		{"flag, confirm", true, confirmHdr, 204, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &servertest.Driver{NameV: "fake"}
			h := harness(t, d, servertest.Options{AllowDisruptive: tc.allow})
			r := h.Do(servertest.Req{Method: "PUT", Path: bondPath, Host: host, Body: body, Header: tc.hdr})
			if r.Status != tc.status {
				t.Fatalf("status %d %s", r.Status, r.Body)
			}
			if tc.class != "" && r.Envelope(t).Error.Class != tc.class {
				t.Fatalf("class %q", r.Envelope(t).Error.Class)
			}
			n := 0
			for _, s := range d.Sessions() {
				n += s.Calls("SetBond")
			}
			if n != tc.setBond {
				t.Fatalf("SetBond called %d times, want %d", n, tc.setBond)
			}
		})
	}
}

// TestBondPutMergesCurrent: omitted fields keep their current value.
func TestBondPutMergesCurrent(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{AllowDisruptive: true})
	r := h.Do(servertest.Req{Method: "PUT", Path: bondPath, Host: host, Body: map[string]any{"enabled": true, "interfaces": []string{" eth1 "}}, Header: confirmHdr})
	if r.Status != 204 {
		t.Fatalf("PUT: %d %s", r.Status, r.Body)
	}
	s := d.Sessions()[0]
	if s.Calls("SetBond") != 1 || !s.BondCfg.Enabled || s.BondCfg.Mode != "active-backup" || len(s.BondCfg.Interfaces) != 1 ||
		s.BondCfg.Interfaces[0] != "eth1" || !s.BondCfg.AutoConfig || s.BondCfg.ID != 1 {
		t.Fatalf("SetBond: calls %d cfg %+v", s.Calls("SetBond"), s.BondCfg)
	}
	r = h.Do(servertest.Req{Method: "PUT", Path: bondPath, Host: host, Body: map[string]any{"enabled": false, "autoConfig": false}, Header: confirmHdr})
	if r.Status != 204 {
		t.Fatalf("PUT 2: %d %s", r.Status, r.Body)
	}
	if s.BondCfg.Enabled || s.BondCfg.AutoConfig || s.BondCfg.Interfaces[0] != "eth1" || s.BondCfg.Mode != "active-backup" {
		t.Fatalf("SetBond 2: %+v", s.BondCfg)
	}
}

// TestBondPutValidation: malformed requests are 400 and never reach SetBond.
func TestBondPutValidation(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{AllowDisruptive: true})
	for name, body := range map[string]any{
		"missing enabled":   map[string]any{"mode": "active-backup"},
		"enabled not bool":  map[string]any{"enabled": "yes"},
		"unknown field":     map[string]any{"enabled": true, "bond_mode": 1},
		"two interfaces":    map[string]any{"enabled": true, "interfaces": []string{"eth0", "eth1"}},
		"empty interface":   map[string]any{"enabled": true, "interfaces": []string{" "}},
		"interfaces string": map[string]any{"enabled": true, "interfaces": "eth0"},
	} {
		r := h.Do(servertest.Req{Method: "PUT", Path: bondPath, Host: host, Body: body, Header: confirmHdr})
		if r.Status != 400 || r.Envelope(t).Error.Class != "invalid_argument" {
			t.Errorf("%s: %d %s", name, r.Status, r.Body)
		}
	}
	for _, s := range d.Sessions() {
		if n := s.Calls("SetBond"); n != 0 {
			t.Fatalf("invalid PUT reached the driver %d times", n)
		}
	}
}

// TestBondCacheInvalidatedAfterPut: the write drops the cached read.
func TestBondCacheInvalidatedAfterPut(t *testing.T) {
	d := &servertest.Driver{NameV: "fake"}
	h := harness(t, d, servertest.Options{AllowDisruptive: true})
	var b v1.BondResponse
	h.Do(servertest.Req{Path: bondPath, Host: host}).JSON(t, &b)
	if b.Enabled {
		t.Fatalf("initial bond: %+v", b)
	}
	if r := h.Do(servertest.Req{Method: "PUT", Path: bondPath, Host: host, Body: map[string]any{"enabled": true}, Header: confirmHdr}); r.Status != 204 {
		t.Fatalf("PUT: %d %s", r.Status, r.Body)
	}
	b = v1.BondResponse{}
	h.Do(servertest.Req{Path: bondPath, Host: host}).JSON(t, &b)
	if !b.Enabled {
		t.Fatalf("stale bond after write: %+v", b)
	}
	// One Bond for the first GET, one inside PUT (read-modify-write), one after.
	if n := d.Sessions()[0].Calls("Bond"); n != 3 {
		t.Fatalf("Bond called %d times, want 3", n)
	}
}

// TestBondUnsupported: a session without BondManager answers 501 for both
// methods (PUT only once the disruptive gate passes) and /capabilities
// reports bond unsupported.
func TestBondUnsupported(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Minimal: true}
	h := harness(t, d, servertest.Options{AllowDisruptive: true})
	if r := h.Do(servertest.Req{Path: bondPath, Host: host}); r.Status != 501 || r.Envelope(t).Error.Class != "unsupported" {
		t.Fatalf("GET: %d %s", r.Status, r.Body)
	}
	r := h.Do(servertest.Req{Method: "PUT", Path: bondPath, Host: host, Body: map[string]any{"enabled": true}, Header: confirmHdr})
	if r.Status != 501 || r.Envelope(t).Error.Class != "unsupported" {
		t.Fatalf("PUT: %d %s", r.Status, r.Body)
	}
	var caps v1.CapabilitiesResponse
	h.Do(servertest.Req{Path: "/api/v1/capabilities", Host: host}).JSON(t, &caps)
	for _, c := range caps.Capabilities {
		if c.Capability == bmc.CapBond && c.Supported {
			t.Fatalf("bond reported supported: %+v", c)
		}
	}
}

// amiBondSession reports the bond capability's backend like the ASRock and
// Gigabyte sessions do.
type amiBondSession struct{ *servertest.Session }

func (s amiBondSession) Info() bmc.SessionInfo {
	in := s.Session.Info()
	in.Capabilities = map[bmc.Capability]string{bmc.CapBond: bmc.BackendAMI}
	return in
}

type amiBondDriver struct{ *servertest.Driver }

func (d amiBondDriver) Open(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Session, error) {
	s, err := d.Driver.Open(ctx, t, c)
	if err != nil {
		return nil, err
	}
	return amiBondSession{s.(*servertest.Session)}, nil
}

// TestBondCapabilityReported: /capabilities lists bond with its backend.
func TestBondCapabilityReported(t *testing.T) {
	d := amiBondDriver{&servertest.Driver{NameV: "fake"}}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	var caps v1.CapabilitiesResponse
	if r := h.Do(servertest.Req{Path: "/api/v1/capabilities", Host: host}); r.Status != 200 {
		t.Fatalf("capabilities: %d %s", r.Status, r.Body)
	} else {
		r.JSON(t, &caps)
	}
	found := false
	for _, c := range caps.Capabilities {
		if c.Capability == bmc.CapBond {
			found = true
			if !c.Supported || c.Backend != bmc.BackendAMI {
				t.Fatalf("bond: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("bond missing from /capabilities")
	}
	if r := h.Do(servertest.Req{Path: bondPath, Host: host}); r.Status != 200 {
		t.Fatalf("GET through wrapped session: %d %s", r.Status, r.Body)
	}
}
