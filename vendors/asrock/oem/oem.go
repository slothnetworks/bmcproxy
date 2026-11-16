// Package oem implements the ASRock Rack-only AMI /api operations behind
// the /api/v1/vendor/asrock/* routes and the AMI-served generic
// capabilities that need board dialect handling: NIC bonding, NC-SI, keep
// NIC link up, fan control (including the X470 two-step table read under
// an exclusive per-host lane), power source, POST code, MCA log, PROCHOT,
// web timeout, component flash status, power restore policy and BMC cold
// reset.
//
// Every operation picks its endpoint from the board profile
// (boards.Profile). Dialects marked "probe" are resolved on first use: the
// newer endpoint is tried and, if the BMC answers 404 code 1010 / 405
// (bmc.ErrUnsupported), the older one; the answer is remembered for the
// Client's lifetime.
package oem

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
)

// Pkg is the OpError.Pkg value for this package.
const Pkg = "driver.asrock.oem"

// API is the subset of the AMI /api client the operations need. The ASRock
// session implements it on top of *amimegarac.Client (adding session-loss
// detection and per-endpoint timeouts).
type API interface {
	// DoJSON sends body as JSON (nil = none), checks the status and decodes
	// the response into dst (nil = discard). See amimegarac.Client.DoJSON.
	DoJSON(ctx context.Context, method, path string, body, dst any) error
	// Exclusive logs in if needed, then runs fn while holding the host's
	// AMI lane so a multi-request sequence cannot interleave with other
	// requests. Calls made with the ctx passed to fn reuse the held slot.
	Exclusive(ctx context.Context, fn func(ctx context.Context) error) error
}

// Client runs OEM operations against one BMC.
type Client struct {
	API API
	P   boards.Profile

	mu     sync.Mutex
	fan    boards.FanDialect
	linkUp boards.LinkUpDialect
}

// New returns a Client for api with board profile p.
func New(api API, p boards.Profile) *Client {
	return &Client{API: api, P: p, fan: p.Fan, linkUp: p.LinkUp}
}

func unsupported(op, why string) error {
	return &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, Detail: why, Err: bmc.ErrUnsupported}
}

func invalid(op, format string, args ...any) error {
	return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, format, args...)
}

// ---- generic map helpers ----

func get(ctx context.Context, api API, path string) (map[string]any, error) {
	var m map[string]any
	if err := api.DoJSON(ctx, http.MethodGet, path, nil, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

// first returns the first object of an array response, or the object
// itself.
func getFirst(ctx context.Context, api API, path string) (map[string]any, any, error) {
	var raw any
	if err := api.DoJSON(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return nil, nil, err
	}
	switch v := raw.(type) {
	case map[string]any:
		return v, raw, nil
	case []any:
		if len(v) > 0 {
			if m, ok := v[0].(map[string]any); ok {
				return m, raw, nil
			}
		}
	}
	return map[string]any{}, raw, nil
}

// Int reads a loosely typed integer field.
func Int(m map[string]any, k string) (int, bool) {
	v, ok := m[k]
	if !ok {
		return 0, false
	}
	f, ok := amimegarac.ParseFloat(v)
	return int(f), ok
}

// Float reads a loosely typed float field (nil for "N/A").
func Float(m map[string]any, k string) *float64 {
	if f, ok := amimegarac.ParseFloat(m[k]); ok {
		return &f
	}
	return nil
}

// Str reads a string field (CleanString applied).
func Str(m map[string]any, k string) string {
	switch v := m[k].(type) {
	case string:
		return amimegarac.CleanString(v)
	case nil:
		return ""
	default:
		return amimegarac.CleanString(fmt.Sprint(v))
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- dialect probing ----

func (c *Client) fanDialect(ctx context.Context) (boards.FanDialect, error) {
	c.mu.Lock()
	d := c.fan
	c.mu.Unlock()
	if d != boards.FanProbe {
		return d, nil
	}
	if _, err := get(ctx, c.API, "settings/fan/mode"); err == nil {
		d = boards.FanSettings
	} else if errors.Is(err, bmc.ErrUnsupported) || errors.Is(err, bmc.ErrNotFound) {
		d = boards.FanASRR
	} else {
		return d, err
	}
	c.mu.Lock()
	c.fan = d
	c.mu.Unlock()
	return d, nil
}

func (c *Client) linkUpDialect(ctx context.Context) (boards.LinkUpDialect, error) {
	c.mu.Lock()
	d := c.linkUp
	c.mu.Unlock()
	if d != boards.LinkUpProbe {
		return d, nil
	}
	if _, err := get(ctx, c.API, "settings/ncsilinkup"); err == nil {
		d = boards.LinkUpNCSI
	} else if errors.Is(err, bmc.ErrUnsupported) || errors.Is(err, bmc.ErrNotFound) {
		d = boards.LinkUpVetobit
	} else {
		return d, err
	}
	c.mu.Lock()
	c.linkUp = d
	c.mu.Unlock()
	return d, nil
}

// ---- bonding ----

func bondFrom(m map[string]any) BondingStatus {
	id, _ := Int(m, "id")
	en, _ := Int(m, "bond_enable")
	ac, _ := Int(m, "auto_configuration_enable")
	ifc := Str(m, "bond_ifc")
	if ifc == "" {
		ifc = Str(m, "interface_name")
	}
	return BondingStatus{ID: id, Enabled: en != 0, Mode: Str(m, "bond_mode"), AutoConfig: ac != 0, Interface: ifc, Raw: m}
}

// Bonding reads settings/network-bond.
func (c *Client) Bonding(ctx context.Context) (BondingStatus, error) {
	m, err := get(ctx, c.API, "settings/network-bond")
	if err != nil {
		return BondingStatus{}, err
	}
	return bondFrom(m), nil
}

// SetBonding writes settings/network-bond with the full object
// {id, bond_enable, bond_mode, auto_configuration_enable[, bond_ifc]}. The
// BMC restarts its network: the caller must treat this as disruptive.
func (c *Client) SetBonding(ctx context.Context, enable bool, mode, ifc string, autoConfig *bool) (BondingStatus, error) {
	const op = "SetBonding"
	cur, err := c.Bonding(ctx)
	if err != nil {
		return BondingStatus{}, err
	}
	if mode == "" {
		mode = cur.Mode
	}
	if mode == "" {
		mode = "active-backup"
	}
	id := cur.ID
	if id == 0 {
		id = 1
	}
	ac := cur.AutoConfig
	if autoConfig != nil {
		ac = *autoConfig
	}
	body := map[string]any{"id": id, "bond_enable": b2i(enable), "bond_mode": mode, "auto_configuration_enable": b2i(ac)}
	if c.P.BondIfc {
		if ifc == "" {
			ifc = cur.Interface
		}
		if ifc == "" {
			return BondingStatus{}, invalid(op, "interface is required on %s (bond_ifc)", c.P.Key)
		}
		body["bond_ifc"] = ifc
	}
	var echo map[string]any
	if err := c.API.DoJSON(ctx, http.MethodPut, "settings/network-bond", body, &echo); err != nil {
		return BondingStatus{}, err
	}
	if len(echo) == 0 {
		echo = body
	}
	return bondFrom(echo), nil
}

// ---- NC-SI ----

func ncsiFrom(m map[string]any) NCSIMode {
	r := NCSIMode{InterfaceName: Str(m, "interface_name"), Raw: m}
	r.ChannelNumber, _ = Int(m, "channel_number")
	r.PackageID, _ = Int(m, "package_id")
	if v, ok := Int(m, "mode"); ok {
		r.Mode = &v
	}
	if v, ok := Int(m, "active"); ok {
		r.Active = &v
	}
	return r
}

// NCSIMode reads settings/ncsi/mode.
func (c *Client) NCSIMode(ctx context.Context) (NCSIMode, error) {
	m, _, err := getFirst(ctx, c.API, "settings/ncsi/mode")
	if err != nil {
		return NCSIMode{}, err
	}
	return ncsiFrom(m), nil
}

// SetNCSIMode merges req into the current settings/ncsi/mode object and PUTs
// it back (disruptive: the BMC may move its management link).
func (c *Client) SetNCSIMode(ctx context.Context, req SetNCSIModeRequest) (NCSIMode, error) {
	cur, _, err := getFirst(ctx, c.API, "settings/ncsi/mode")
	if err != nil {
		return NCSIMode{}, err
	}
	body := map[string]any{}
	for k, v := range cur {
		body[k] = v
	}
	if req.InterfaceName != nil {
		body["interface_name"] = *req.InterfaceName
	}
	if req.Mode != nil {
		body["mode"] = *req.Mode
	}
	if req.ChannelNumber != nil {
		body["channel_number"] = *req.ChannelNumber
	}
	if req.PackageID != nil {
		body["package_id"] = *req.PackageID
	}
	delete(body, "active") // read-only (B650)
	if err := c.API.DoJSON(ctx, http.MethodPut, "settings/ncsi/mode", body, nil); err != nil {
		return NCSIMode{}, err
	}
	return ncsiFrom(body), nil
}

// ---- keep NIC link up ----

// KeepNicLinkUp reads asrr/getvetobit (X470) or settings/ncsilinkup (B650).
func (c *Client) KeepNicLinkUp(ctx context.Context) (KeepNicLinkUp, error) {
	d, err := c.linkUpDialect(ctx)
	if err != nil {
		return KeepNicLinkUp{}, err
	}
	if d == boards.LinkUpVetobit {
		m, err := get(ctx, c.API, "asrr/getvetobit")
		if err != nil {
			return KeepNicLinkUp{}, err
		}
		v, _ := Int(m, "vetobit")
		return KeepNicLinkUp{Enabled: v != 0, Dialect: d.String(), Raw: m}, nil
	}
	m, err := get(ctx, c.API, "settings/ncsilinkup")
	if err != nil {
		return KeepNicLinkUp{}, err
	}
	a, _ := Int(m, "sharednic")
	b, _ := Int(m, "sharednic2")
	sa, sb := a != 0, b != 0
	return KeepNicLinkUp{Enabled: sa || sb, Dialect: d.String(), SharedNIC: &sa, SharedNIC2: &sb, Raw: m}, nil
}

// SetKeepNicLinkUp writes asrr/setvetobit (PUT {vetobit}) or
// settings/ncsilinkup (PUT {sharednic, sharednic2}).
func (c *Client) SetKeepNicLinkUp(ctx context.Context, req SetKeepNicLinkUpRequest) (KeepNicLinkUp, error) {
	const op = "SetKeepNicLinkUp"
	d, err := c.linkUpDialect(ctx)
	if err != nil {
		return KeepNicLinkUp{}, err
	}
	if d == boards.LinkUpVetobit {
		if req.Enabled == nil {
			return KeepNicLinkUp{}, invalid(op, "enabled is required (vetobit dialect)")
		}
		if err := c.API.DoJSON(ctx, http.MethodPut, "asrr/setvetobit", map[string]any{"vetobit": b2i(*req.Enabled)}, nil); err != nil {
			return KeepNicLinkUp{}, err
		}
		return c.KeepNicLinkUp(ctx)
	}
	cur, err := get(ctx, c.API, "settings/ncsilinkup")
	if err != nil {
		return KeepNicLinkUp{}, err
	}
	body := map[string]any{}
	for k, v := range cur {
		body[k] = v
	}
	if req.Enabled != nil {
		body["sharednic"], body["sharednic2"] = b2i(*req.Enabled), b2i(*req.Enabled)
	}
	if req.SharedNIC != nil {
		body["sharednic"] = b2i(*req.SharedNIC)
	}
	if req.SharedNIC2 != nil {
		body["sharednic2"] = b2i(*req.SharedNIC2)
	}
	if req.Enabled == nil && req.SharedNIC == nil && req.SharedNIC2 == nil {
		return KeepNicLinkUp{}, invalid(op, "nothing to change")
	}
	if err := c.API.DoJSON(ctx, http.MethodPut, "settings/ncsilinkup", body, nil); err != nil {
		return KeepNicLinkUp{}, err
	}
	return c.KeepNicLinkUp(ctx)
}

// ---- fan ----

func fanNumbered(m map[string]any, prefix, suffix string) map[string]int {
	out := map[string]int{}
	for k := range m {
		if !strings.HasPrefix(k, prefix) || !strings.HasSuffix(k, suffix) {
			continue
		}
		n := strings.TrimSuffix(strings.TrimPrefix(k, prefix), suffix)
		if _, err := strconv.Atoi(n); err != nil {
			continue
		}
		if v, ok := Int(m, k); ok {
			out[n] = v
		}
	}
	return out
}

// FanMode reads the fan control mode.
func (c *Client) FanMode(ctx context.Context) (FanModeResponse, error) {
	d, err := c.fanDialect(ctx)
	if err != nil {
		return FanModeResponse{}, err
	}
	if d == boards.FanASRR {
		m, _, err := getFirst(ctx, c.API, "asrr/settings/getfancontrolmode")
		if err != nil {
			return FanModeResponse{}, err
		}
		return FanModeResponse{Dialect: d.String(), Modes: fanNumbered(m, "mode", ""), Raw: m}, nil
	}
	var raw any
	if err := c.API.DoJSON(ctx, http.MethodGet, "settings/fan/mode", nil, &raw); err != nil {
		return FanModeResponse{}, err
	}
	return FanModeResponse{Dialect: d.String(), Raw: raw}, nil
}

// SetFanMode writes the fan control mode (read-merge-write of the whole
// object).
func (c *Client) SetFanMode(ctx context.Context, req SetFanModeRequest) (FanModeResponse, error) {
	const op = "SetFanMode"
	d, err := c.fanDialect(ctx)
	if err != nil {
		return FanModeResponse{}, err
	}
	if d == boards.FanASRR {
		if len(req.Modes) == 0 && len(req.Raw) == 0 {
			return FanModeResponse{}, invalid(op, "modes is required")
		}
		cur, _, err := getFirst(ctx, c.API, "asrr/settings/getfancontrolmode")
		if err != nil {
			return FanModeResponse{}, err
		}
		body := map[string]any{}
		for k, v := range cur {
			body[k] = v
		}
		for k, v := range req.Modes {
			n := strings.TrimPrefix(k, "mode")
			if i, err := strconv.Atoi(n); err != nil || i < 1 || i > 16 {
				return FanModeResponse{}, invalid(op, "bad fan number %q (want 1..16)", k)
			}
			body["mode"+n] = v
		}
		for k, v := range req.Raw {
			body[k] = v
		}
		if err := c.API.DoJSON(ctx, http.MethodPost, "asrr/settings/setfancontrolmode", body, nil); err != nil {
			return FanModeResponse{}, err
		}
		return c.FanMode(ctx)
	}
	if len(req.Raw) == 0 {
		return FanModeResponse{}, invalid(op, "raw is required on the settings/fan dialect")
	}
	cur, _, err := getFirst(ctx, c.API, "settings/fan/mode")
	if err != nil {
		return FanModeResponse{}, err
	}
	for k, v := range req.Raw {
		cur[k] = v
	}
	if err := c.API.DoJSON(ctx, http.MethodPut, "settings/fan/mode", cur, nil); err != nil {
		return FanModeResponse{}, err
	}
	return c.FanMode(ctx)
}

func openTable(table int, m map[string]any, raw any) FanTable {
	ft := FanTable{Type: "open", Table: table, Raw: raw}
	temps, duties := fanNumbered(m, "Get_Temp", ""), fanNumbered(m, "Get_Duty", "")
	ft.Temps, ft.Duties = ordered(temps), ordered(duties)
	return ft
}

func ordered(m map[string]int) []int {
	if len(m) == 0 {
		return nil
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		i, _ := strconv.Atoi(k)
		keys = append(keys, i)
	}
	sort.Ints(keys)
	out := make([]int, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[strconv.Itoa(k)])
	}
	return out
}

// TableRef names one fan table to read.
type TableRef struct {
	Type  string // "open" | "closed"
	Table int
}

// FanTables reads fan control tables. On the asrr dialect each table is a
// two-step sequence (POST set{open,close}looptablenumber {table_numble},
// then GET getfan{open,close}loopcontroltable) that selects state on the
// BMC, so the whole read runs under the exclusive AMI lane: no other
// request to the host can slip between the select and the read.
func (c *Client) FanTables(ctx context.Context, refs []TableRef) (FanTablesResponse, error) {
	const op = "FanTables"
	for i := range refs {
		refs[i].Type = strings.ToLower(strings.TrimSpace(refs[i].Type))
		if refs[i].Type != "open" && refs[i].Type != "closed" {
			return FanTablesResponse{}, invalid(op, "bad table type %q (want open|closed)", refs[i].Type)
		}
		if refs[i].Table <= 0 {
			refs[i].Table = 1
		}
	}
	d, err := c.fanDialect(ctx)
	if err != nil {
		return FanTablesResponse{}, err
	}
	if d == boards.FanASRR {
		for _, r := range refs {
			if err := checkASRRTable(op, r.Type, r.Table); err != nil {
				return FanTablesResponse{}, err
			}
		}
	}
	out := FanTablesResponse{Dialect: d.String(), Tables: []FanTable{}}
	if d == boards.FanSettings {
		for _, r := range refs {
			p := "settings/fan/openloop"
			if r.Type == "closed" {
				p = "settings/fan/closedloop"
			}
			var raw any
			if err := c.API.DoJSON(ctx, http.MethodGet, p, nil, &raw); err != nil {
				return FanTablesResponse{}, err
			}
			out.Tables = append(out.Tables, FanTable{Type: r.Type, Table: r.Table, Raw: raw})
		}
		return out, nil
	}
	err = c.API.Exclusive(ctx, func(ctx context.Context) error {
		for _, r := range refs {
			sel, read := "asrr/settings/setopenlooptablenumber", "asrr/settings/getfanopenloopcontroltable"
			if r.Type == "closed" {
				sel, read = "asrr/settings/setcloselooptablenumber", "asrr/settings/getfancloseloopcontroltable"
			}
			// "table_numble" (sic) is the firmware's field name.
			if err := c.API.DoJSON(ctx, http.MethodPost, sel, map[string]any{"table_numble": r.Table}, nil); err != nil {
				return err
			}
			var raw any
			if err := c.API.DoJSON(ctx, http.MethodGet, read, nil, &raw); err != nil {
				return err
			}
			m := map[string]any{}
			switch v := raw.(type) {
			case map[string]any:
				m = v
			case []any:
				if len(v) > 0 {
					if mm, ok := v[0].(map[string]any); ok {
						m = mm
					}
				}
			}
			if r.Type == "open" {
				out.Tables = append(out.Tables, openTable(r.Table, m, raw))
			} else {
				out.Tables = append(out.Tables, FanTable{Type: "closed", Table: r.Table, Raw: raw})
			}
		}
		return nil
	})
	if err != nil {
		return FanTablesResponse{}, err
	}
	return out, nil
}

// asrr table_numble ranges offered by the X470 3.02 UI
// (docs/api/legacy-api-endpoints.json allowed_values): open loop 1..8,
// closed loop 1..10.
const (
	asrrMaxOpenTable   = 8
	asrrMaxClosedTable = 10
)

func checkASRRTable(op, typ string, n int) error {
	limit := asrrMaxOpenTable
	if typ == "closed" {
		limit = asrrMaxClosedTable
	}
	if n < 1 || n > limit {
		return invalid(op, "%s-loop table %d out of range (1..%d on the asrr dialect)", typ, n, limit)
	}
	return nil
}

// SetFanTable writes one fan control table.
//
// asrr (X470): POST asrr/settings/setfanopenloopcontroltable {table_numble,
// temp1..24, duty1..24} or setfancloseloopcontroltable {table_numble,
// temp_of_/duty_cycle_percent_of_/time_of_{speedup,slowdown}_fan_duty_cycle}
// (closed-loop fields go in Raw). Endpoint names and body keys come from
// the X470 3.02 UI JS (docs/api/legacy-api-endpoints.json).
// UNVERIFIED: never written on hardware (the live probe only round-tripped
// the selectors and the reads, docs/x470/scripts/rt.py).
//
// settings/fan (B650 7.02/9.05, live 4.10): the collections are GET-only;
// a table is written with PUT settings/fan/{openloop,closedloop}/{id}
// (docs/api/legacy-api-endpoints.json item_path). UNVERIFIED: that {id} is
// the table number as the proxy passes it (Table, default 1).
func (c *Client) SetFanTable(ctx context.Context, req SetFanTableRequest) error {
	const op = "SetFanTable"
	typ := strings.ToLower(strings.TrimSpace(req.Type))
	if typ != "open" && typ != "closed" {
		return invalid(op, "type must be open or closed")
	}
	if req.Table <= 0 {
		req.Table = 1
	}
	d, err := c.fanDialect(ctx)
	if err != nil {
		return err
	}
	if d == boards.FanSettings {
		if len(req.Raw) == 0 {
			return invalid(op, "raw is required on the settings/fan dialect")
		}
		p := "settings/fan/openloop/"
		if typ == "closed" {
			p = "settings/fan/closedloop/"
		}
		return c.API.DoJSON(ctx, http.MethodPut, p+strconv.Itoa(req.Table), req.Raw, nil)
	}
	if err := checkASRRTable(op, typ, req.Table); err != nil {
		return err
	}
	body := map[string]any{"table_numble": req.Table}
	if typ == "open" {
		if len(req.Temps) != len(req.Duties) {
			return invalid(op, "temps and duties must have the same length")
		}
		if len(req.Temps) > 24 {
			return invalid(op, "at most 24 points")
		}
		for i := range req.Temps {
			body["temp"+strconv.Itoa(i+1)] = req.Temps[i]
			body["duty"+strconv.Itoa(i+1)] = req.Duties[i]
		}
	}
	for k, v := range req.Raw {
		body[k] = v
	}
	if len(body) == 1 {
		return invalid(op, "no table data (temps/duties or raw)")
	}
	p := "asrr/settings/setfanopenloopcontroltable"
	if typ == "closed" {
		p = "asrr/settings/setfancloseloopcontroltable"
	}
	return c.API.DoJSON(ctx, http.MethodPost, p, body, nil)
}

// FanDuty reads fan duties: X470 asrr/GetFanDuty (current duty); X570
// asrr/settings/getfandutymanualmode (configured manual duty: GetFanDuty is
// absent, 404 on live 1.20); B650 settings/fan/manual.
func (c *Client) FanDuty(ctx context.Context) (FanDutyResponse, error) {
	d, err := c.fanDialect(ctx)
	if err != nil {
		return FanDutyResponse{}, err
	}
	if d == boards.FanASRR && c.P.FanDutyManual {
		m, _, err := getFirst(ctx, c.API, "asrr/settings/getfandutymanualmode")
		if err != nil {
			return FanDutyResponse{}, err
		}
		return FanDutyResponse{Dialect: d.String(), Source: "manual", Duties: fanNumbered(m, "duty", ""), Raw: m}, nil
	}
	if d == boards.FanASRR {
		m, _, err := getFirst(ctx, c.API, "asrr/GetFanDuty")
		if err != nil {
			return FanDutyResponse{}, err
		}
		return FanDutyResponse{Dialect: d.String(), Source: "current", Duties: fanNumbered(m, "FAN", "_Duty"), Raw: m}, nil
	}
	var raw any
	if err := c.API.DoJSON(ctx, http.MethodGet, "settings/fan/manual", nil, &raw); err != nil {
		return FanDutyResponse{}, err
	}
	return FanDutyResponse{Dialect: d.String(), Source: "manual", Raw: raw}, nil
}

// SetManualFanDuty writes manual fan duties (read-merge-write).
func (c *Client) SetManualFanDuty(ctx context.Context, req SetManualFanDutyRequest) error {
	const op = "SetManualFanDuty"
	d, err := c.fanDialect(ctx)
	if err != nil {
		return err
	}
	for k, v := range req.Duties {
		if v < 0 || v > 100 {
			return invalid(op, "duty for fan %s out of range (0..100)", k)
		}
	}
	if d == boards.FanASRR {
		if len(req.Duties) == 0 {
			return invalid(op, "duties is required")
		}
		cur, _, err := getFirst(ctx, c.API, "asrr/settings/getfandutymanualmode")
		if err != nil {
			return err
		}
		body := map[string]any{}
		for k, v := range cur {
			body[k] = v
		}
		for k, v := range req.Duties {
			n := strings.TrimPrefix(k, "duty")
			if i, err := strconv.Atoi(n); err != nil || i < 1 || i > 16 {
				return invalid(op, "bad fan number %q (want 1..16)", k)
			}
			body["duty"+n] = v
		}
		return c.API.DoJSON(ctx, http.MethodPost, "asrr/settings/setfandutymanualmode", body, nil)
	}
	if len(req.Raw) == 0 {
		return invalid(op, "raw is required on the settings/fan dialect")
	}
	cur, _, err := getFirst(ctx, c.API, "settings/fan/manual")
	if err != nil {
		return err
	}
	for k, v := range req.Raw {
		cur[k] = v
	}
	return c.API.DoJSON(ctx, http.MethodPut, "settings/fan/manual", cur, nil)
}

// ResetFanSettings restores the default fan configuration.
func (c *Client) ResetFanSettings(ctx context.Context) error {
	d, err := c.fanDialect(ctx)
	if err != nil {
		return err
	}
	if d == boards.FanASRR {
		return c.API.DoJSON(ctx, http.MethodPost, "asrr/settings/setclearallfansetting", map[string]any{}, nil)
	}
	return c.API.DoJSON(ctx, http.MethodPost, "settings/fan/load_default", map[string]any{}, nil)
}

// ---- power source / POST code / MCA / prochot / web timeout ----

// PowerSource reads asrr/PowerSource.
func (c *Client) PowerSource(ctx context.Context) (PowerSourceResponse, error) {
	var raw []map[string]any
	if err := c.API.DoJSON(ctx, http.MethodGet, "asrr/PowerSource", nil, &raw); err != nil {
		return PowerSourceResponse{}, err
	}
	out := PowerSourceResponse{PowerSupplies: []PowerSupply{}}
	for i, m := range raw {
		st, _ := Int(m, "power_supply_status")
		out.PowerSupplies = append(out.PowerSupplies, PowerSupply{
			Index: i, Status: st,
			ACInputVoltage: Float(m, "ac_input_voltage"), ACInputCurrent: Float(m, "ac_input_current"),
			ACInputPower:       Float(m, "ac_input_power"),
			DC12VOutputVoltage: Float(m, "dc_12v_output_voltage"), DC12VOutputCurrent: Float(m, "dc_12v_output_current"),
			DC12VOutputPower: Float(m, "dc_12v_output_power"),
			Serial:           Str(m, "pws_serial_number"), Raw: m,
		})
	}
	return out, nil
}

// PostCode reads the last BIOS POST code.
func (c *Client) PostCode(ctx context.Context) (PostCodeResponse, error) {
	if c.P.PostCode == boards.PostCodeLogs {
		var raw any
		err := c.API.DoJSON(ctx, http.MethodGet, "logs/postcode", nil, &raw)
		if err == nil {
			return PostCodeResponse{Source: "logs/postcode", Raw: raw}, nil
		}
		if !errors.Is(err, bmc.ErrUnsupported) {
			return PostCodeResponse{}, err
		}
	}
	m, err := get(ctx, c.API, "asrr/getbioscode")
	if err != nil {
		return PostCodeResponse{}, err
	}
	r := PostCodeResponse{Source: "asrr/getbioscode", Raw: m}
	if v, ok := Int(m, "poststatus"); ok {
		r.Status = &v
	}
	if v, ok := Int(m, "postdata"); ok {
		r.Code = fmt.Sprintf("0x%04x", v)
	}
	return r, nil
}

// MCALog reads asrr/getmcalog.
func (c *Client) MCALog(ctx context.Context) (MCALogResponse, error) {
	m, err := get(ctx, c.API, "asrr/getmcalog")
	if err != nil {
		return MCALogResponse{}, err
	}
	v, _ := Int(m, "MCADump")
	return MCALogResponse{DumpPresent: v != 0, MCADump: v}, nil
}

// ClearMCALog sends DELETE asrr/clearmcalog (X470 only).
func (c *Client) ClearMCALog(ctx context.Context) error {
	if !c.P.ClearMCA {
		return unsupported("ClearMCALog", "asrr/clearmcalog has no handler on "+c.P.Key)
	}
	return c.API.DoJSON(ctx, http.MethodDelete, "asrr/clearmcalog", nil, nil)
}

// Prochot reads asrr/getprochot (X470 only).
func (c *Client) Prochot(ctx context.Context) (ProchotResponse, error) {
	if !c.P.Prochot {
		return ProchotResponse{}, unsupported("Prochot", "asrr/getprochot has no handler on "+c.P.Key)
	}
	m, err := get(ctx, c.API, "asrr/getprochot")
	if err != nil {
		return ProchotResponse{}, err
	}
	v, _ := Int(m, "prochot")
	return ProchotResponse{Enabled: v != 0, Value: v}, nil
}

// SetProchot sends PUT asrr/setprochot {prochot} (X470 only).
func (c *Client) SetProchot(ctx context.Context, enabled bool) (ProchotResponse, error) {
	if !c.P.Prochot {
		return ProchotResponse{}, unsupported("SetProchot", "asrr/setprochot has no handler on "+c.P.Key)
	}
	if err := c.API.DoJSON(ctx, http.MethodPut, "asrr/setprochot", map[string]any{"prochot": b2i(enabled)}, nil); err != nil {
		return ProchotResponse{}, err
	}
	return c.Prochot(ctx)
}

// WebEnableTimeout reads asrr/getWebEnableTimeout.
func (c *Client) WebEnableTimeout(ctx context.Context) (WebEnableTimeout, error) {
	m, err := get(ctx, c.API, "asrr/getWebEnableTimeout")
	if err != nil {
		return WebEnableTimeout{}, err
	}
	v, _ := Int(m, "WebEnableTimeout")
	return WebEnableTimeout{Enabled: v != 0, Value: v}, nil
}

// SetWebEnableTimeout sends POST asrr/setWebEnableTimeout {enable_timeout}.
func (c *Client) SetWebEnableTimeout(ctx context.Context, enabled bool) (WebEnableTimeout, error) {
	if err := c.API.DoJSON(ctx, http.MethodPost, "asrr/setWebEnableTimeout", map[string]any{"enable_timeout": b2i(enabled)}, nil); err != nil {
		return WebEnableTimeout{}, err
	}
	return c.WebEnableTimeout(ctx)
}

// BIOSUpdateOption reads asrr/maintenance/BIOS/UpdateOption.
func (c *Client) BIOSUpdateOption(ctx context.Context) (BIOSUpdateOption, error) {
	m, err := get(ctx, c.API, "asrr/maintenance/BIOS/UpdateOption")
	if err != nil {
		return BIOSUpdateOption{}, err
	}
	v, _ := Int(m, "action")
	return BIOSUpdateOption{Action: v}, nil
}

// NormalizeComponent validates a component name and returns its canonical
// upper-case form.
func NormalizeComponent(c string) (string, error) {
	u := strings.ToUpper(strings.TrimSpace(c))
	for _, x := range boards.AllComponents {
		if u == x {
			return u, nil
		}
	}
	return "", invalid("Component", "unknown component %q (want %s)", c, strings.Join(boards.AllComponents, ","))
}

// ComponentStatus reads asrr/maintenance/{C}/status and, when present,
// flash-progress.
func (c *Client) ComponentStatus(ctx context.Context, component string) (ComponentStatus, error) {
	comp, err := NormalizeComponent(component)
	if err != nil {
		return ComponentStatus{}, err
	}
	if !c.P.HasComponent(comp) {
		return ComponentStatus{}, unsupported("ComponentStatus", fmt.Sprintf("no %s flash handler on %s", comp, c.P.Key))
	}
	out := ComponentStatus{Component: comp}
	st, raw, err := getFirst(ctx, c.API, "asrr/maintenance/"+comp+"/status")
	if err != nil {
		return ComponentStatus{}, err
	}
	out.Status = raw
	if v, ok := Int(st, "response"); ok {
		out.Response = &v
	}
	out.Progress, out.Action = Str(st, "progress"), Str(st, "action")
	fp, fraw, err := getFirst(ctx, c.API, "asrr/maintenance/"+comp+"/flash-progress")
	switch {
	case err == nil:
		out.FlashProgress = fraw
		if p := Str(fp, "progress"); p != "" {
			out.Progress = p
		}
		if a := Str(fp, "action"); a != "" {
			out.Action = a
		}
	case errors.Is(err, bmc.ErrUnsupported), errors.Is(err, bmc.ErrNotFound):
		// no flash-progress handler for this component on this board
	default:
		return ComponentStatus{}, err
	}
	return out, nil
}

// BMCColdReset sends PUT asrr/bmccoldreset {} (every session is lost).
func (c *Client) BMCColdReset(ctx context.Context) error {
	return c.API.DoJSON(ctx, http.MethodPut, "asrr/bmccoldreset", map[string]any{}, nil)
}

// ---- power restore policy (B650 asrr) ----

// Restore codes of asrr/{Get,Set}PowerRestore. The field name powerRestore
// is confirmed by the B650 7.02/9.05 UI JS (docs/api/legacy-api-catalog.md)
// and by a live read on the X570D4U 1.20 ({"powerRestore": 1},
// docs/x570/live-probe.md). UNVERIFIED: the values are assumed to follow the
// IPMI chassis encoding (0 always off, 1 last state, 2 always on); the X570
// Redfish PowerRestorePolicy is null, so nothing cross-checks the 1.
const (
	restoreAlwaysOff = 0
	restoreLastState = 1
	restoreAlwaysOn  = 2
)

// GetPowerRestore reads asrr/GetPowerRestore {powerRestore}.
func (c *Client) GetPowerRestore(ctx context.Context) (bmc.RestorePolicy, error) {
	m, err := get(ctx, c.API, "asrr/GetPowerRestore")
	if err != nil {
		return bmc.RestoreUnknown, err
	}
	v, ok := Int(m, "powerRestore")
	if !ok {
		return bmc.RestoreUnknown, bmc.Errorf(bmc.ErrUpstream, Pkg, "GetPowerRestore", "no powerRestore field")
	}
	switch v {
	case restoreAlwaysOff:
		return bmc.RestoreAlwaysOff, nil
	case restoreLastState:
		return bmc.RestoreLastState, nil
	case restoreAlwaysOn:
		return bmc.RestoreAlwaysOn, nil
	}
	return bmc.RestoreUnknown, nil
}

// SetPowerRestore sends POST asrr/SetPowerRestore {powerRestore}.
func (c *Client) SetPowerRestore(ctx context.Context, p bmc.RestorePolicy) error {
	var v int
	switch p {
	case bmc.RestoreAlwaysOff:
		v = restoreAlwaysOff
	case bmc.RestoreLastState:
		v = restoreLastState
	case bmc.RestoreAlwaysOn:
		v = restoreAlwaysOn
	default:
		return invalid("SetPowerRestore", "unknown policy %q", p)
	}
	return c.API.DoJSON(ctx, http.MethodPost, "asrr/SetPowerRestore", map[string]any{"powerRestore": v}, nil)
}
