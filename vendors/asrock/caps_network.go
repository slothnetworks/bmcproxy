package asrock

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

// ---- network ----

func zeroAddr(s string) bool {
	switch strings.TrimSpace(s) {
	case "", "0.0.0.0", "::":
		return true
	}
	return false
}

func nicFromAMI(m map[string]any) bmc.NetworkInterface {
	id, _ := oem.Int(m, "id")
	lan, _ := oem.Int(m, "lan_enable")
	v4dhcp, _ := oem.Int(m, "ipv4_dhcp_enable")
	v6en, _ := oem.Int(m, "ipv6_enable")
	v6dhcp, _ := oem.Int(m, "ipv6_dhcp_enable")
	vlan, _ := oem.Int(m, "vlan_enable")
	vid, _ := oem.Int(m, "vlan_id")
	n := bmc.NetworkInterface{
		ID: strconv.Itoa(id), Name: oem.Str(m, "interface_name"), MAC: oem.Str(m, "mac_address"), Enabled: lan != 0,
		IPv4: bmc.IPv4Config{DHCP: v4dhcp != 0},
		IPv6: bmc.IPv6Config{Enabled: v6en != 0, DHCP: v6dhcp != 0},
		VLAN: bmc.VLANConfig{Enabled: vlan != 0, ID: vid},
		Oem:  map[string]any{},
	}
	if a := oem.Str(m, "ipv4_address"); !zeroAddr(a) {
		n.IPv4.Address = a
	}
	if a := oem.Str(m, "ipv4_subnet"); !zeroAddr(a) {
		n.IPv4.Netmask = a
	}
	if a := oem.Str(m, "ipv4_gateway"); !zeroAddr(a) {
		n.IPv4.Gateway = a
	}
	if a := oem.Str(m, "ipv6_address"); !zeroAddr(a) {
		p, _ := oem.Int(m, "ipv6_prefix")
		n.IPv6.Addresses = []string{a + "/" + strconv.Itoa(p)}
	}
	if a := oem.Str(m, "ipv6_gateway"); !zeroAddr(a) {
		n.IPv6.Gateway = a
	}
	if ch, ok := oem.Int(m, "channel_number"); ok {
		n.Oem["channelNumber"] = ch
	}
	if pr, ok := oem.Int(m, "vlan_priority"); ok {
		n.Oem["vlanPriority"] = pr
	}
	return n
}

func (s *Session) amiNetworkRaw(ctx context.Context) ([]map[string]any, error) {
	var raw []map[string]any
	if err := s.api.Get(ctx, "settings/network", &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *Session) amiNetwork(ctx context.Context) ([]bmc.NetworkInterface, error) {
	raw, err := s.amiNetworkRaw(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.NetworkInterface, 0, len(raw))
	for _, m := range raw {
		out = append(out, nicFromAMI(m))
	}
	return out, nil
}

// NetworkInterfaces implements bmc.NetworkManager. AMI: settings/network
// (X470 eth0+eth1, B650 bond0).
func (s *Session) NetworkInterfaces(ctx context.Context) ([]bmc.NetworkInterface, error) {
	switch s.backend(bmc.CapNetwork) {
	case bmc.BackendRedfish:
		return s.Session.NetworkInterfaces(ctx)
	case bmc.BackendAMI:
		return s.amiNetwork(ctx)
	}
	return nil, s.unsupported("NetworkInterfaces")
}

// SetNetworkInterface implements bmc.NetworkManager (disruptive: the BMC
// restarts networking). AMI: read settings/network, mutate the named
// interface's full object, PUT settings/network/{id}.
func (s *Session) SetNetworkInterface(ctx context.Context, u bmc.NetworkInterfaceUpdate) error {
	const op = "SetNetworkInterface"
	switch s.backend(bmc.CapNetwork) {
	case bmc.BackendRedfish:
		return s.Session.SetNetworkInterface(ctx, u)
	case bmc.BackendAMI:
	default:
		return s.unsupported(op)
	}
	if strings.TrimSpace(u.Name) == "" {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "name is required")
	}
	raw, err := s.amiNetworkRaw(ctx)
	if err != nil {
		return err
	}
	var cur map[string]any
	for _, m := range raw {
		if strings.EqualFold(oem.Str(m, "interface_name"), u.Name) {
			cur = m
			break
		}
	}
	if cur == nil {
		return bmc.Errorf(bmc.ErrNotFound, Pkg, op, "no interface %q", u.Name)
	}
	body := map[string]any{}
	for k, v := range cur {
		body[k] = v
	}
	if u.Enabled != nil {
		body["lan_enable"] = b2i(*u.Enabled)
	}
	if v := u.IPv4; v != nil {
		body["ipv4_dhcp_enable"] = b2i(v.DHCP)
		if !v.DHCP {
			if v.Address == "" || v.Netmask == "" {
				return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "static IPv4 needs address and netmask")
			}
			body["ipv4_address"], body["ipv4_subnet"] = v.Address, v.Netmask
			if v.Gateway != "" {
				body["ipv4_gateway"] = v.Gateway
			}
		}
	}
	if v := u.IPv6; v != nil {
		body["ipv6_enable"] = b2i(v.Enabled)
		body["ipv6_dhcp_enable"] = b2i(v.DHCP)
		if len(v.Addresses) > 0 {
			addr, prefix, _ := strings.Cut(v.Addresses[0], "/")
			body["ipv6_address"] = addr
			if p, err := strconv.Atoi(prefix); err == nil {
				body["ipv6_prefix"] = p
			}
		}
		if v.Gateway != "" {
			body["ipv6_gateway"] = v.Gateway
		}
	}
	if v := u.VLAN; v != nil {
		body["vlan_enable"] = b2i(v.Enabled)
		if v.Enabled {
			body["vlan_id"] = v.ID
		}
	}
	if len(u.DNS) > 0 {
		return bmc.Errorf(bmc.ErrUnsupported, Pkg, op, "DNS is configured via settings/dns on /api (not supported yet)")
	}
	id, _ := oem.Int(cur, "id")
	return s.api.DoJSON(ctx, http.MethodPut, "settings/network/"+strconv.Itoa(id), body, nil)
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- bonding (generic BondManager) ----

// Bond implements bmc.BondManager (AMI settings/network-bond).
func (s *Session) Bond(ctx context.Context) (bmc.BondConfig, error) {
	if s.backend(bmc.CapBond) != bmc.BackendAMI {
		return bmc.BondConfig{}, s.unsupported("Bond")
	}
	b, err := s.oem.Bonding(ctx)
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
// network). The first interface is used as B650 bond_ifc.
func (s *Session) SetBond(ctx context.Context, b bmc.BondConfig) error {
	if s.backend(bmc.CapBond) != bmc.BackendAMI {
		return s.unsupported("SetBond")
	}
	ifc := ""
	if len(b.Interfaces) > 0 {
		ifc = b.Interfaces[0]
	}
	ac := b.AutoConfig
	_, err := s.oem.SetBonding(ctx, b.Enabled, b.Mode, ifc, &ac)
	return err
}

// ---- users ----

func privFromAMI(p string) bmc.Privilege {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "administrator", "admin":
		return bmc.PrivAdministrator
	case "operator":
		return bmc.PrivOperator
	case "user":
		return bmc.PrivUser
	case "callback":
		return bmc.PrivCallback
	case "oem", "oem proprietary":
		return bmc.PrivOEM
	case "none", "no access", "noaccess", "":
		return bmc.PrivNoAccess
	}
	return bmc.Privilege(strings.ToLower(p))
}

// Users implements bmc.UserReader. AMI: settings/users. X470 has one row per
// user (network_privilege); B650 one row per user per channel (userid,
// privilege, accessByChannel), which is folded to one row per user id.
func (s *Session) Users(ctx context.Context) ([]bmc.User, error) {
	switch s.backend(bmc.CapUsers) {
	case bmc.BackendRedfish:
		return s.Session.Users(ctx)
	case bmc.BackendAMI:
	default:
		return nil, s.unsupported("Users")
	}
	var raw []map[string]any
	if err := s.api.Get(ctx, "settings/users", &raw); err != nil {
		return nil, err
	}
	out := []bmc.User{}
	seen := map[int]int{}
	for _, m := range raw {
		name := oem.Str(m, "name")
		if name == "" {
			continue
		}
		uid, ok := oem.Int(m, "userid")
		if !ok {
			uid, _ = oem.Int(m, "id")
		}
		access, _ := oem.Int(m, "access")
		priv := oem.Str(m, "network_privilege")
		if priv == "" {
			priv = oem.Str(m, "privilege")
		}
		u := bmc.User{ID: strconv.Itoa(uid), Name: name, Enabled: access != 0, Privilege: privFromAMI(priv),
			Email: oem.Str(m, "email_id"), Oem: map[string]any{}}
		if ch, ok := oem.Int(m, "channel"); ok {
			u.Oem["channel"] = ch
		}
		if v := oem.Str(m, "privilegeByChannel"); v != "" {
			u.Oem["privilegeByChannel"] = v
		}
		if v := oem.Str(m, "accessByChannel"); v != "" {
			u.Oem["accessByChannel"] = v
		}
		if len(u.Oem) == 0 {
			u.Oem = nil
		}
		if i, dup := seen[uid]; dup {
			// Another channel row for the same user: merge "enabled anywhere".
			out[i].Enabled = out[i].Enabled || u.Enabled
			if out[i].Privilege == bmc.PrivNoAccess && u.Privilege != bmc.PrivNoAccess {
				out[i].Privilege = u.Privilege
			}
			continue
		}
		seen[uid] = len(out)
		out = append(out, u)
	}
	return out, nil
}

// ---- services ----

// Services implements bmc.ServiceReader. AMI: settings/services. Disabled
// ports are negative (X470 bit 31 set, B650 -1); session counters carry a
// 0x80 flag bit (148 & 0x7F = 20 max, 129 & 0x7F = 1 active).
func (s *Session) Services(ctx context.Context) ([]bmc.Service, error) {
	switch s.backend(bmc.CapServices) {
	case bmc.BackendRedfish:
		return s.Session.Services(ctx)
	case bmc.BackendAMI:
	default:
		return nil, s.unsupported("Services")
	}
	var raw []map[string]any
	if err := s.api.Get(ctx, "settings/services", &raw); err != nil {
		return nil, err
	}
	out := make([]bmc.Service, 0, len(raw))
	for _, m := range raw {
		state, _ := oem.Int(m, "state")
		sv := bmc.Service{Name: oem.Str(m, "service_name"), Enabled: state != 0, Oem: map[string]any{}}
		if v, ok := portOf(m, "non_secure_port"); ok {
			sv.Port = v
		}
		if v, ok := portOf(m, "secure_port"); ok {
			sv.SecurePort = v
		}
		if t, ok := oem.Int(m, "time_out"); ok && t > 0 {
			sv.TimeoutSeconds = t
		}
		if v, ok := oem.Int(m, "maximum_sessions"); ok {
			sv.MaxSessions = v & 0x7f
		}
		if v, ok := oem.Int(m, "active_session"); ok {
			sv.ActiveSessions = v & 0x7f
		}
		if ifc := oem.Str(m, "interface_name"); ifc != "" && !amimegarac.IsNA(ifc) {
			sv.Interfaces = []string{ifc}
		}
		if v, ok := oem.Int(m, "singleport_status"); ok {
			sv.Oem["singlePort"] = v != 0
		}
		out = append(out, sv)
	}
	return out, nil
}

func portOf(m map[string]any, k string) (int, bool) {
	raw, ok := m[k]
	if !ok {
		return 0, false
	}
	var n json.Number
	switch v := raw.(type) {
	case float64:
		n = json.Number(strconv.FormatInt(int64(v), 10))
	case string:
		n = json.Number(v)
	default:
		return 0, false
	}
	i, err := n.Int64()
	if err != nil {
		return 0, false
	}
	p, disabled := amimegarac.DecodePort(i)
	if disabled {
		return 0, false
	}
	return p, true
}
