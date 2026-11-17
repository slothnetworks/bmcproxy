// BMC NIC bonding: GET/PUT /api/v1/network/bond (bmc.BondManager).

package v1

import (
	"context"
	"net/http"
	"strings"

	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// BondResponse is GET /api/v1/network/bond.
type BondResponse struct {
	// Bond id on the BMC (1 on the known AMI boards).
	ID int `json:"id" example:"1"`
	// Whether the BMC NICs are bonded.
	Enabled bool `json:"enabled" example:"true"`
	// Bonding mode (Linux bonding mode name as the BMC reports it, e.g. active-backup).
	Mode string `json:"mode,omitempty" example:"active-backup"`
	// Interfaces in (or selected for) the bond, as the BMC names them. ASRock B650: bond_ifc (eth0/eth1); Gigabyte: bond_ifc (eth0 shared, eth1 dedicated, both failover). Empty when the BMC has no such field (ASRock X470).
	Interfaces []string `json:"interfaces,omitempty" example:"both"`
	// Whether the bond configures itself automatically (auto_configuration_enable).
	AutoConfig bool `json:"autoConfig" example:"true"`
	// The vendor object as returned by the BMC (AMI settings/network-bond).
	Oem map[string]any `json:"oem,omitempty"`
}

// BondRequest is PUT /api/v1/network/bond. Omitted fields keep their
// current value: the driver reads the full vendor object, mutates it and
// writes the whole object back.
type BondRequest struct {
	// Enable or disable bonding (required).
	Enabled *bool `json:"enabled" example:"true"`
	// Bonding mode; default the current one. Gigabyte: not configurable (must be empty or the current mode).
	Mode string `json:"mode,omitempty" example:"active-backup"`
	// Interface selection (bond_ifc; one entry); default the current one. ASRock B650: eth0/eth1 (required there to un-bond); Gigabyte: eth0, eth1 or both (both only while enabled).
	Interfaces []string `json:"interfaces,omitempty" example:"both"`
	// Automatic configuration; default the current setting.
	AutoConfig *bool `json:"autoConfig,omitempty" example:"true"`
}

// getBond returns the BMC NIC bonding configuration.
//
//	@Summary		BMC NIC bonding
//	@Description	Tier: **read** (cached briefly, concurrent calls coalesced). ASRock AMI /api: `settings/network-bond`; Gigabyte (MC13-LE0, MZ32-AR0) AMI /api: `settings/network-bond` (live: `{id, bond_enable, bond_mode, bond_ifc, auto_configuration_enable}`, bond0 holds the BMC address by default). 501 when the driver has no bonding support (Redfish-only and IPMI targets). `/api/v1/capabilities` reports the backend under `bond`.
//	@Tags			network
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	BondResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody	"capability not supported"
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/network/bond [get]
func (a *api) getBond(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapBond, "Bond", "", defTTL, func(ctx context.Context, c bmc.BondManager) (any, error) {
		b, err := c.Bond(ctx)
		return BondResponse{ID: b.ID, Enabled: b.Enabled, Mode: b.Mode, Interfaces: b.Interfaces, AutoConfig: b.AutoConfig, Oem: b.Oem}, err
	})
}

// putBond changes the BMC NIC bonding.
//
//	@Summary		Set BMC NIC bonding
//	@Description	Tier: **disruptive** (the BMC restarts its networking and can drop off the network). Omitted fields keep their current value: the driver reads the full `settings/network-bond` object, mutates it and PUTs the whole object back (AMI answers 500 to partial bodies). ASRock AMI /api: `PUT settings/network-bond`; Gigabyte AMI /api: `PUT settings/network-bond` (bond_mode is not configurable; write **UNVERIFIED** on hardware, shape from the MZ32-AR0 web UI). Cached reads for the host are dropped after the call.
//	@Tags			network
//	@Accept			json
//	@Param			X-BMC-Host		header	string		true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string		false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			body			body	BondRequest	true	"Bonding change (enabled required)"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody	"capability not supported"
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/network/bond [put]
func (a *api) putBond(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapBond, "SetBond", func(ctx context.Context, c bmc.BondManager, _ *sessions.Handle) (any, error) {
		var body BondRequest
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		if err := body.validate(); err != nil {
			return nil, err
		}
		cur, err := c.Bond(ctx)
		if err != nil {
			return nil, err
		}
		return nil, c.SetBond(ctx, body.apply(cur))
	})
}

// validate checks the request before anything reaches the BMC.
func (b *BondRequest) validate() error {
	const op = "SetBond"
	if b.Enabled == nil {
		return bmc.Errorf(bmc.ErrInvalidArgument, pkg, op, "enabled is required")
	}
	b.Mode = strings.TrimSpace(b.Mode)
	if len(b.Interfaces) > 1 {
		return bmc.Errorf(bmc.ErrInvalidArgument, pkg, op, "interfaces takes one entry (the BMC's bond_ifc), got %d", len(b.Interfaces))
	}
	for i, s := range b.Interfaces {
		s = strings.TrimSpace(s)
		if s == "" {
			return bmc.Errorf(bmc.ErrInvalidArgument, pkg, op, "interfaces[%d] is empty", i)
		}
		b.Interfaces[i] = s
	}
	return nil
}

// apply merges the request over the current configuration.
func (b *BondRequest) apply(cur bmc.BondConfig) bmc.BondConfig {
	cur.Enabled = *b.Enabled
	if b.Mode != "" {
		cur.Mode = b.Mode
	}
	if len(b.Interfaces) > 0 {
		cur.Interfaces = append([]string(nil), b.Interfaces...)
	}
	if b.AutoConfig != nil {
		cur.AutoConfig = *b.AutoConfig
	}
	return cur
}
