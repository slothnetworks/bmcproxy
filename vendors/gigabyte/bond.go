// NIC bonding over the AMI /api settings/network-bond handler.

package gigabyte

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// Evidence for the settings/network-bond object on Gigabyte MegaRAC SP-X:
//
//   - the handler is compiled into spx_restservice on both boards
//     (firmware-dumped/MC13-LE0-000, firmware-dumped/MZ32-AR0-00;
//     docs/gigabyte/mz32-ar0-frontend.md §3 lists /api/settings/network-bond);
//   - the MZ32-AR0 12.61.39 web UI (www_plain/source.min.js, models//bond
//     and views/settings/network/bond) is a Backbone model at
//     /api/settings/network-bond: it fetches the object, binds bond_enable,
//     bond_ifc and auto_configuration_enable, shows bond_mode read-only
//     ("This field is not configurable") and saves with
//     model.save({bond_enable, auto_configuration_enable, bond_ifc}), i.e.
//     the whole fetched object with those three fields replaced. bond_ifc
//     is one of eth0 ("Shared"), eth1 ("Dedicated") or both ("Failover");
//   - the live probes saw bond0 carrying the BMC address on both boards
//     (settings/network interface_name "bond0");
//   - live read-only GET /api/settings/network-bond on 2026-11-16 (MC13-LE0
//     13.04.15 and MZ32-AR0 12.61.39, identical): {"id":1, "bond_enable":1,
//     "bond_mode":"active-backup", "bond_ifc":"both",
//     "auto_configuration_enable":1} (testdata/network-bond-*.json,
//     docs/gigabyte/*-live-probe.md). The spx_restservice handler strings
//     of both builds list these field names (MC13-LE0 also the bond_ifc
//     values eth0/eth1/both next to them);
//   - the ASRock driver (vendors/asrock/oem SetBonding) drives the same AMI
//     handler with a full-object PUT {id, bond_enable, bond_mode,
//     auto_configuration_enable[, bond_ifc]}.
//
// The GET shape is confirmed on hardware. Because the object carries an
// id, the UI's Backbone save is a PUT of the whole fetched object.
//
// UNVERIFIED: no Gigabyte network-bond PUT has been sent to hardware (it
// restarts BMC networking). The write method and full-object body come
// from the UI code and the ASRock handler; the PUT response body (echo or
// not) and whether the firmware accepts bond_enable=0 with bond_ifc "both"
// (refused here, see SetBonding) are unknown.

const apiNetworkBond = "settings/network-bond"

// gigabyteBondIfc are the bond_ifc values of the 12.61.39 UI.
var gigabyteBondIfc = map[string]bool{"eth0": true, "eth1": true, "both": true}

// BondingStatus is GET /api/v1/vendor/gigabyte/bonding (AMI settings/network-bond).
type BondingStatus struct {
	// Bond id on the BMC (1 when the BMC does not report one).
	ID int `json:"id" example:"1"`
	// Whether NIC bonding is enabled.
	Enabled bool `json:"enabled" example:"true"`
	// Bonding mode in effect (read-only on Gigabyte).
	Mode string `json:"mode,omitempty" example:"active-backup"`
	// Interface selection bond_ifc: eth0 (shared), eth1 (dedicated) or both (failover).
	Interface string `json:"interface,omitempty" example:"both"`
	// Whether the bond configures itself automatically.
	AutoConfig bool `json:"autoConfig" example:"true"`
	// Raw is the BMC object as returned.
	Raw map[string]any `json:"raw,omitempty"`
} //@name GigabyteBondingStatus

// EnableBondingRequest is POST /api/v1/vendor/gigabyte/enableBonding.
type EnableBondingRequest struct {
	// Bonding mode. Gigabyte does not let it be changed: it must be empty or the current mode.
	Mode string `json:"mode,omitempty" example:"active-backup"`
	// bond_ifc: eth0 (shared), eth1 (dedicated) or both (failover); default the current one.
	Interface string `json:"interface,omitempty" example:"both"`
} //@name GigabyteEnableBondingRequest

// DisableBondingRequest is POST /api/v1/vendor/gigabyte/disableBonding.
type DisableBondingRequest struct {
	// bond_ifc to keep after un-bonding (eth0 or eth1); default the current one.
	Interface string `json:"interface,omitempty" example:"eth1"`
} //@name GigabyteDisableBondingRequest

func intField(m map[string]any, k string) (int, bool) {
	switch v := m[k].(type) {
	case float64:
		return int(v), true
	case json.Number:
		n, err := v.Int64()
		return int(n), err == nil
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case string:
		var n int
		_, err := fmt.Sscan(strings.TrimSpace(v), &n)
		return n, err == nil
	}
	return 0, false
}

func strField(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return strings.TrimSpace(s)
}

func bondStatus(m map[string]any) BondingStatus {
	id, ok := intField(m, "id")
	if !ok || id == 0 {
		id = 1
	}
	en, _ := intField(m, "bond_enable")
	ac, _ := intField(m, "auto_configuration_enable")
	return BondingStatus{ID: id, Enabled: en != 0, Mode: strField(m, "bond_mode"), Interface: strField(m, "bond_ifc"),
		AutoConfig: ac != 0, Raw: m}
}

func (s *Session) bondObject(ctx context.Context, op string) (map[string]any, error) {
	if s.ami == nil {
		return nil, bmc.Unsupported(Pkg, op+" (no AMI /api on "+s.prof.Key+")")
	}
	var m map[string]any
	if err := s.ami.Get(ctx, apiNetworkBond, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, Err: bmc.ErrUpstream, Detail: "empty settings/network-bond object"}
	}
	return m, nil
}

// Bonding reads AMI settings/network-bond.
func (s *Session) Bonding(ctx context.Context) (BondingStatus, error) {
	m, err := s.bondObject(ctx, "Bonding")
	if err != nil {
		return BondingStatus{}, err
	}
	return bondStatus(m), nil
}

// SetBonding enables or disables bonding: it GETs the full
// settings/network-bond object, replaces bond_enable (and bond_ifc /
// auto_configuration_enable when given) and PUTs the whole object back
// (AMI handlers answer 500 to partial bodies). The BMC restarts its
// networking: callers must gate this as disruptive. mode must be empty or
// the current bond_mode (the Gigabyte UI shows it as not configurable).
//
// Un-bonding with bond_ifc "both" is refused: the BMC address has to stay
// on one port. UNVERIFIED: the UI would send it unchanged and the firmware
// behaviour is unknown; the write method and body come from the MZ32-AR0
// 12.61.39 UI and the ASRock handler; never exercised on Gigabyte hardware.
func (s *Session) SetBonding(ctx context.Context, enable bool, mode, ifc string, autoConfig *bool) (BondingStatus, error) {
	const op = "SetBonding"
	ifc = strings.ToLower(strings.TrimSpace(ifc))
	if ifc != "" && !gigabyteBondIfc[ifc] {
		return BondingStatus{}, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "interface must be eth0 (shared), eth1 (dedicated) or both (failover), got %q", ifc)
	}
	if !enable && ifc == "both" {
		return BondingStatus{}, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "un-bonding needs one interface (eth0 or eth1) to keep the address")
	}
	cur, err := s.bondObject(ctx, op)
	if err != nil {
		return BondingStatus{}, err
	}
	if m := strings.TrimSpace(mode); m != "" && !strings.EqualFold(m, strField(cur, "bond_mode")) {
		return BondingStatus{}, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "bond_mode is not configurable on Gigabyte (current %q)", strField(cur, "bond_mode"))
	}
	body := make(map[string]any, len(cur)+3)
	for k, v := range cur {
		body[k] = v
	}
	if _, ok := body["id"]; !ok {
		body["id"] = 1
	}
	body["bond_enable"] = b2i(enable)
	if ifc != "" {
		body["bond_ifc"] = ifc
	}
	if autoConfig != nil {
		body["auto_configuration_enable"] = b2i(*autoConfig)
	}
	var echo map[string]any
	if err := s.ami.DoJSON(ctx, http.MethodPut, apiNetworkBond, body, &echo); err != nil {
		return BondingStatus{}, err
	}
	if len(echo) == 0 || echo["bond_enable"] == nil {
		echo = body
	}
	return bondStatus(echo), nil
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Bond implements bmc.BondManager (AMI settings/network-bond).
func (s *Session) Bond(ctx context.Context) (bmc.BondConfig, error) {
	b, err := s.Bonding(ctx)
	if err != nil {
		return bmc.BondConfig{}, err
	}
	bc := bmc.BondConfig{ID: b.ID, Enabled: b.Enabled, Mode: b.Mode, AutoConfig: b.AutoConfig, Oem: b.Raw}
	if b.Interface != "" {
		bc.Interfaces = []string{b.Interface}
	}
	return bc, nil
}

// SetBond implements bmc.BondManager (disruptive: the BMC restarts its
// networking). The first interface is bond_ifc.
func (s *Session) SetBond(ctx context.Context, b bmc.BondConfig) error {
	ifc := ""
	if len(b.Interfaces) > 0 {
		ifc = b.Interfaces[0]
	}
	ac := b.AutoConfig
	_, err := s.SetBonding(ctx, b.Enabled, b.Mode, ifc, &ac)
	return err
}

// decodeOptional decodes an optional JSON body (empty = zero value).
func decodeOptional(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, r.URL.Path, "read body: %v", err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: decode request body: %v", bmc.ErrInvalidArgument, err)
	}
	return nil
}
