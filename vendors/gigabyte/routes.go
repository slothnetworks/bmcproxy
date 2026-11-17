// Typed vendor routes (/api/v1/vendor/gigabyte/*) and their swag annotations.

package gigabyte

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// VendorRoutes implements bmc.VendorRouter: everything under
// /api/v1/vendor/gigabyte/. Only genuinely Gigabyte-specific features are
// here; everything with a standard capability is served by the generic
// Redfish session.
func (d *Driver) VendorRoutes() []bmc.Route {
	return append(oemRoutes(), rawRoutes()...)
}

func oemRoutes() []bmc.Route {
	return []bmc.Route{
		{Method: http.MethodGet, Path: "fanProfiles", Tier: bmc.TierRead, Handler: withSession(handleFanProfiles), Summary: "Gigabyte fan profiles"},
		{Method: http.MethodGet, Path: "fanProfileDevices", Tier: bmc.TierRead, Handler: withSession(handleFanProfileDevices), Summary: "PCIe devices the fan profiles can key on"},
		{Method: http.MethodGet, Path: "fanMode", Tier: bmc.TierRead, Handler: withSession(handleFanMode), Summary: "Active fan profile"},
		{Method: http.MethodPost, Path: "setFanMode", Tier: bmc.TierWrite, Handler: withSession(handleSetFanMode), Summary: "Activate a fan profile"},
		{Method: http.MethodGet, Path: "firmwareInfo", Tier: bmc.TierRead, Handler: withSession(handleFirmwareInfo), Summary: "BMC firmware info incl. active image (dual image)"},
		{Method: http.MethodGet, Path: "bonding", Tier: bmc.TierRead, Handler: withSession(handleBonding), Summary: "NIC bonding (AMI settings/network-bond)"},
		{Method: http.MethodPost, Path: "enableBonding", Tier: bmc.TierDisruptive, Handler: withSession(handleEnableBonding), Summary: "Enable NIC bonding"},
		{Method: http.MethodPost, Path: "disableBonding", Tier: bmc.TierDisruptive, Handler: withSession(handleDisableBonding), Summary: "Disable NIC bonding"},
	}
}

// withSession adapts a *Session handler to bmc.HandlerFunc.
func withSession(fn func(*Session, *http.Request) (any, error)) bmc.HandlerFunc {
	return func(s bmc.Session, r *http.Request) (any, error) {
		gs, ok := s.(*Session)
		if !ok || gs == nil {
			return nil, bmc.Errorf(bmc.ErrNotFound, Pkg, r.URL.Path, "target is not served by the gigabyte driver (%T)", s)
		}
		return fn(gs, r)
	}
}

// decodeBody decodes a required JSON body (1 MiB cap, unknown fields refused).
func decodeBody(r *http.Request, v any) error {
	if r.Body == nil {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, r.URL.Path, "request body required")
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, r.URL.Path, "read body: %v", err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, r.URL.Path, "request body required")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: decode request body: %v", bmc.ErrInvalidArgument, err)
	}
	return nil
}

// handleFanProfiles serves GET /api/v1/vendor/gigabyte/fanProfiles.
//
//	@Summary		Gigabyte fan profiles
//	@Description	Tier: **read**. Redfish GBT OEM: `Chassis/{id}/Thermal` → `Oem.Gbt.FanprofileService` → `Fanprofile` ({strMode, strVersion, arrProfile[{strName, arrPolicy[]}]}), normalised. `active` marks the profile named by the current mode. MC13-LE0 13.04.15: verified live. 501 on boards whose Thermal has no `Oem.Gbt.FanprofileService`.
//	@Tags			gigabyte
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (gigabyte)"
//	@Success		200				{object}	FanProfiles
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/fanProfiles [get]
func handleFanProfiles(s *Session, r *http.Request) (any, error) {
	return s.FanProfiles(r.Context())
}

// handleFanProfileDevices serves GET /api/v1/vendor/gigabyte/fanProfileDevices.
//
//	@Summary		PCIe devices the fan profiles can key on
//	@Description	Tier: **read**. Redfish GBT OEM `FanprofileService/SupportPCIEDevice` ({arrPCIEDevices[{strName, hexVendorID, hexDeviceID, UNC, UC}]}): the PCIe devices (mostly GPUs) a fan policy can follow, with their upper non-critical / critical temperatures. MC13-LE0 13.04.15: verified live.
//	@Tags			gigabyte
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (gigabyte)"
//	@Success		200				{object}	FanProfileDevices
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/fanProfileDevices [get]
func handleFanProfileDevices(s *Session, r *http.Request) (any, error) {
	return s.FanProfileDevices(r.Context())
}

// handleFanMode serves GET /api/v1/vendor/gigabyte/fanMode.
//
//	@Summary		Active fan profile
//	@Description	Tier: **read**. AMI /api `GET settings/fanprofile/mode` ({"strMode"}) on boards with the MegaRAC /api (MC13-LE0; verified live), else the `strMode` of the Redfish Fanprofile. `default` is the factory curve.
//	@Tags			gigabyte
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (gigabyte)"
//	@Success		200				{object}	FanMode
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/fanMode [get]
func handleFanMode(s *Session, r *http.Request) (any, error) {
	return s.FanMode(r.Context())
}

// handleSetFanMode serves POST /api/v1/vendor/gigabyte/setFanMode.
//
//	@Summary		Activate a fan profile
//	@Description	Tier: **write**. Validates `mode` against the profile names from fanProfiles (plus `default`), then AMI /api `POST settings/fanprofile/mode` {"strMode": mode} and reads the mode back (502 if the BMC still reports another mode). `default` stops the custom profile. **UNVERIFIED**: request shape taken from the 13.06.30 web UI; never exercised on hardware. 501 on boards without the MegaRAC /api.
//	@Tags			gigabyte
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string				true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string				false	"Force a driver instead of auto-detection (gigabyte)"
//	@Param			body			body		SetFanModeRequest	true	"profile name or default"
//	@Success		200				{object}	FanMode
//	@Failure		400				{object}	server.ErrorBody	"bad request / unknown profile"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/setFanMode [post]
func handleSetFanMode(s *Session, r *http.Request) (any, error) {
	var req SetFanModeRequest
	if err := decodeBody(r, &req); err != nil {
		return nil, err
	}
	return s.SetFanMode(r.Context(), req.Mode)
}

// handleFirmwareInfo serves GET /api/v1/vendor/gigabyte/firmwareInfo.
//
//	@Summary		BMC firmware info incl. active image
//	@Description	Tier: **read**. AMI /api `GET firmware-info` (IPMI Get Device ID view): `fw_ver`, build date/time, IPMI IANA manufacturer id (15370 = Giga-Byte), product id and `active_image`, the running image of a dual-image BMC, which Redfish does not expose. MC13-LE0 13.04.15: verified live (`active_image 1`, `mfg_id_0..2`). MZ32-AR0 12.61.39: from the research probe (`active_image 1`, prod_id 4104). 501 on boards without the MegaRAC /api.
//	@Tags			gigabyte
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (gigabyte)"
//	@Success		200				{object}	BMCFirmwareInfo
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/firmwareInfo [get]
func handleFirmwareInfo(s *Session, r *http.Request) (any, error) {
	return s.FirmwareInfo(r.Context())
}

// handleBonding serves GET /api/v1/vendor/gigabyte/bonding.
//
//	@Summary		NIC bonding (AMI settings/network-bond)
//	@Description	Tier: **read**. AMI /api `GET settings/network-bond` on boards with the MegaRAC /api (MC13-LE0, MZ32-AR0): bond_enable, bond_mode (read-only), bond_ifc (eth0 shared / eth1 dedicated / both failover), auto_configuration_enable. Shape confirmed live on MC13-LE0 13.04.15 and MZ32-AR0 12.61.39 (`{id:1, bond_enable:1, bond_mode:"active-backup", bond_ifc:"both", auto_configuration_enable:1}`: bond0 in failover holds the BMC address). The generic equivalent is `GET /api/v1/network/bond`.
//	@Tags			gigabyte
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (gigabyte)"
//	@Success		200				{object}	BondingStatus
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/bonding [get]
func handleBonding(s *Session, r *http.Request) (any, error) {
	return s.Bonding(r.Context())
}

// handleEnableBonding serves POST /api/v1/vendor/gigabyte/enableBonding.
//
//	@Summary		Enable NIC bonding
//	@Description	Tier: **disruptive** (the BMC restarts its networking and may drop off the network). Reads `settings/network-bond`, sets bond_enable=1 (and bond_ifc when given) and PUTs the whole object back. `mode` must be empty or the current bond_mode (not configurable on Gigabyte). **UNVERIFIED**: write shape from the MZ32-AR0 12.61.39 web UI and the ASRock handler; never exercised on hardware.
//	@Tags			gigabyte
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (gigabyte)"
//	@Param			body			body		EnableBondingRequest	false	"interface (bond_ifc) and mode; both optional"
//	@Success		200				{object}	BondingStatus
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/enableBonding [post]
func handleEnableBonding(s *Session, r *http.Request) (any, error) {
	var req EnableBondingRequest
	if err := decodeOptional(r, &req); err != nil {
		return nil, err
	}
	return s.SetBonding(r.Context(), true, req.Mode, req.Interface, nil)
}

// handleDisableBonding serves POST /api/v1/vendor/gigabyte/disableBonding.
//
//	@Summary		Disable NIC bonding
//	@Description	Tier: **disruptive** (the BMC restarts its networking and may drop off the network). Reads `settings/network-bond`, sets bond_enable=0 (bond_ifc = the interface to keep when given: eth0 or eth1) and PUTs the whole object back. **UNVERIFIED** on hardware.
//	@Tags			gigabyte
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (gigabyte)"
//	@Param			body			body		DisableBondingRequest	false	"interface to keep (optional)"
//	@Success		200				{object}	BondingStatus
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/gigabyte/disableBonding [post]
func handleDisableBonding(s *Session, r *http.Request) (any, error) {
	var req DisableBondingRequest
	if err := decodeOptional(r, &req); err != nil {
		return nil, err
	}
	return s.SetBonding(r.Context(), false, "", req.Interface, nil)
}
