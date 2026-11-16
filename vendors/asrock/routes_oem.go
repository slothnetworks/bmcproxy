// Typed vendor routes (/api/v1/vendor/asrock/*) and their swag annotations.

package asrock

import (
	"net/http"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
)

// oemRoutes lists the typed vendor routes (raw passthroughs are in raw.go).
func oemRoutes() []bmc.Route {
	return []bmc.Route{
		{Method: http.MethodGet, Path: "bonding", Tier: bmc.TierRead, Handler: withSession(handleBonding), Summary: "BMC LAN bonding status"},
		{Method: http.MethodPost, Path: "enableBonding", Tier: bmc.TierDisruptive, Handler: withSession(handleEnableBonding), Summary: "Enable BMC LAN bonding"},
		{Method: http.MethodPost, Path: "disableBonding", Tier: bmc.TierDisruptive, Handler: withSession(handleDisableBonding), Summary: "Disable BMC LAN bonding"},
		{Method: http.MethodGet, Path: "ncsiMode", Tier: bmc.TierRead, Handler: withSession(handleNCSIMode), Summary: "NC-SI mode"},
		{Method: http.MethodPost, Path: "setNcsiMode", Tier: bmc.TierDisruptive, Handler: withSession(handleSetNCSIMode), Summary: "Set NC-SI mode"},
		{Method: http.MethodGet, Path: "keepNicLinkUp", Tier: bmc.TierRead, Handler: withSession(handleKeepNicLinkUp), Summary: "Keep NIC link up while host is off"},
		{Method: http.MethodPost, Path: "setKeepNicLinkUp", Tier: bmc.TierWrite, Handler: withSession(handleSetKeepNicLinkUp), Summary: "Set keep-NIC-link-up"},
		{Method: http.MethodGet, Path: "fanMode", Tier: bmc.TierRead, Handler: withSession(handleFanMode), Summary: "Fan control mode"},
		{Method: http.MethodPost, Path: "setFanMode", Tier: bmc.TierWrite, Handler: withSession(handleSetFanMode), Summary: "Set fan control mode"},
		{Method: http.MethodGet, Path: "fanTables", Tier: bmc.TierRead, Handler: withSession(handleFanTables), Summary: "Fan curve tables"},
		{Method: http.MethodPost, Path: "setFanTable", Tier: bmc.TierWrite, Handler: withSession(handleSetFanTable), Summary: "Write a fan curve table"},
		{Method: http.MethodGet, Path: "fanDuty", Tier: bmc.TierRead, Handler: withSession(handleFanDuty), Summary: "Current fan duty"},
		{Method: http.MethodPost, Path: "setManualFanDuty", Tier: bmc.TierWrite, Handler: withSession(handleSetManualFanDuty), Summary: "Set manual fan duty"},
		{Method: http.MethodPost, Path: "resetFanSettings", Tier: bmc.TierWrite, Handler: withSession(handleResetFanSettings), Summary: "Reset fan settings to defaults"},
		{Method: http.MethodGet, Path: "powerSource", Tier: bmc.TierRead, Handler: withSession(handlePowerSource), Summary: "PSU / power source readings"},
		{Method: http.MethodGet, Path: "postCode", Tier: bmc.TierRead, Handler: withSession(handlePostCode), Summary: "Last BIOS POST code"},
		{Method: http.MethodGet, Path: "mcaLog", Tier: bmc.TierRead, Handler: withSession(handleMCALog), Summary: "CPU machine-check (MCA) dump status"},
		{Method: http.MethodPost, Path: "clearMcaLog", Tier: bmc.TierDisruptive, Handler: withSession(handleClearMCALog), Summary: "Clear the MCA dump"},
		{Method: http.MethodGet, Path: "prochot", Tier: bmc.TierRead, Handler: withSession(handleProchot), Summary: "CPU PROCHOT assertion setting"},
		{Method: http.MethodPost, Path: "setProchot", Tier: bmc.TierWrite, Handler: withSession(handleSetProchot), Summary: "Set CPU PROCHOT assertion"},
		{Method: http.MethodGet, Path: "webEnableTimeout", Tier: bmc.TierRead, Handler: withSession(handleWebEnableTimeout), Summary: "Web session idle-timeout switch"},
		{Method: http.MethodPost, Path: "setWebEnableTimeout", Tier: bmc.TierWrite, Handler: withSession(handleSetWebEnableTimeout), Summary: "Set web session idle-timeout switch"},
		{Method: http.MethodGet, Path: "biosUpdateOption", Tier: bmc.TierRead, Handler: withSession(handleBIOSUpdateOption), Summary: "BIOS update option"},
		{Method: http.MethodGet, Path: "componentStatus/{component}", Tier: bmc.TierRead, Handler: withSession(handleComponentStatus), Summary: "Firmware component flash status"},
		{Method: http.MethodPost, Path: "updateComponent/{component}", Tier: bmc.TierDisruptive, Handler: withSession(handleUpdateComponent), Summary: "Flash a firmware component (not implemented)"},
		{Method: http.MethodPost, Path: "bmcColdReset", Tier: bmc.TierDisruptive, Handler: withSession(handleBMCColdReset), Summary: "BMC cold reset"},
	}
}

// handleBonding serves GET /api/v1/vendor/asrock/bonding.
//
//	@Summary		BMC LAN bonding status
//	@Description	Tier: **read**. All boards: AMI /api `GET settings/network-bond` ({id,bond_enable,bond_mode,auto_configuration_enable}; B650 also bond_ifc, X470/X570 never).
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.BondingStatus
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/bonding [get]
func handleBonding(s *Session, r *http.Request) (any, error) {
	return s.oem.Bonding(r.Context())
}

// handleEnableBonding serves POST /api/v1/vendor/asrock/enableBonding.
//
//	@Summary		Enable BMC LAN bonding
//	@Description	Tier: **disruptive** (the BMC restarts its network; the session may drop). Both boards: read `settings/network-bond`, then `PUT settings/network-bond` with the full object, bond_enable=1, the requested bond_mode and (B650/B550) bond_ifc. Mode defaults to the current one; interface is ignored on X470.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	true	"Must be true; the proxy must also run with --allow-disruptive"
//	@Param			body			body		oem.EnableBondingRequest	false	"bond mode and (B650) interface"
//	@Success		200				{object}	oem.BondingStatus
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/enableBonding [post]
func handleEnableBonding(s *Session, r *http.Request) (any, error) {
	var req oem.EnableBondingRequest
	if err := decodeOptional(r, &req); err != nil {
		return nil, err
	}
	return s.oem.SetBonding(r.Context(), true, req.Mode, req.Interface, nil)
}

// handleDisableBonding serves POST /api/v1/vendor/asrock/disableBonding.
//
//	@Summary		Disable BMC LAN bonding
//	@Description	Tier: **disruptive** (the BMC restarts its network). Both boards: `PUT settings/network-bond` with bond_enable=0 (full object; B650 bond_ifc = the interface to keep, default the current one). Body optional.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	true	"Must be true; the proxy must also run with --allow-disruptive"
//	@Param			body			body		oem.DisableBondingRequest	false	"interface to keep (B650)"
//	@Success		200				{object}	oem.BondingStatus
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/disableBonding [post]
func handleDisableBonding(s *Session, r *http.Request) (any, error) {
	var req oem.DisableBondingRequest
	if err := decodeOptional(r, &req); err != nil {
		return nil, err
	}
	return s.oem.SetBonding(r.Context(), false, "", req.Interface, nil)
}

// handleNCSIMode serves GET /api/v1/vendor/asrock/ncsiMode.
//
//	@Summary		NC-SI mode
//	@Description	Tier: **read**. Both boards: `GET settings/ncsi/mode` ({channel_number, interface_name, mode, package_id}; B650 adds active).
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.NCSIMode
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/ncsiMode [get]
func handleNCSIMode(s *Session, r *http.Request) (any, error) {
	return s.oem.NCSIMode(r.Context())
}

// handleSetNCSIMode serves POST /api/v1/vendor/asrock/setNcsiMode.
//
//	@Summary		Set NC-SI mode
//	@Description	Tier: **disruptive** (the shared-NIC link can drop). Both boards: read `settings/ncsi/mode`, merge the given fields, `PUT settings/ncsi/mode` with the full object.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	true	"Must be true; the proxy must also run with --allow-disruptive"
//	@Param			body			body		oem.SetNCSIModeRequest	true	"fields to change"
//	@Success		200				{object}	oem.NCSIMode
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/setNcsiMode [post]
func handleSetNCSIMode(s *Session, r *http.Request) (any, error) {
	var req oem.SetNCSIModeRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	return s.oem.SetNCSIMode(r.Context(), req)
}

// handleKeepNicLinkUp serves GET /api/v1/vendor/asrock/keepNicLinkUp.
//
//	@Summary		Keep NIC link up while host is off
//	@Description	Tier: **read**. X470, X570 (live 1.20: {"vetobit":0}; settings/ncsilinkup 404) and B650 1.x: `GET asrr/getvetobit` {vetobit}. B650 4.x/B550: `GET settings/ncsilinkup` {sharednic, sharednic2}. The dialect is chosen by board profile, probed when unknown.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.KeepNicLinkUp
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/keepNicLinkUp [get]
func handleKeepNicLinkUp(s *Session, r *http.Request) (any, error) {
	return s.oem.KeepNicLinkUp(r.Context())
}

// handleSetKeepNicLinkUp serves POST /api/v1/vendor/asrock/setKeepNicLinkUp.
//
//	@Summary		Set keep-NIC-link-up
//	@Description	Tier: **write**. X470/X570: `PUT asrr/setvetobit` {vetobit}. B650 4.x: `PUT settings/ncsilinkup` {sharednic, sharednic2} (enabled sets both unless given individually).
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			body			body		oem.SetKeepNicLinkUpRequest	true	"desired state"
//	@Success		200				{object}	oem.KeepNicLinkUp
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/setKeepNicLinkUp [post]
func handleSetKeepNicLinkUp(s *Session, r *http.Request) (any, error) {
	var req oem.SetKeepNicLinkUpRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	return s.oem.SetKeepNicLinkUp(r.Context(), req)
}

// handleFanMode serves GET /api/v1/vendor/asrock/fanMode.
//
//	@Summary		Fan control mode
//	@Description	Tier: **read**. X470/X570: `GET asrr/settings/getfancontrolmode` (an array with one {mode1..mode16} object, live-confirmed on X470 2.32 and X570 1.20 with every fan at 0 = the default; the meaning of the other codes is unverified). B650 4.x: `GET settings/fan/mode`. Raw vendor object included.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.FanModeResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/fanMode [get]
func handleFanMode(s *Session, r *http.Request) (any, error) {
	return s.oem.FanMode(r.Context())
}

// handleSetFanMode serves POST /api/v1/vendor/asrock/setFanMode.
//
//	@Summary		Set fan control mode
//	@Description	Tier: **write**. X470/X570: `POST asrr/settings/setfancontrolmode` (full mode1..16 object; unspecified fans keep their mode). B650 4.x: `PUT settings/fan/mode` (raw object merged over the current one).
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			body			body		oem.SetFanModeRequest	true	"modes keyed by fan number 1..16 (X470), or raw vendor fields (B650)"
//	@Success		200				{object}	oem.FanModeResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/setFanMode [post]
func handleSetFanMode(s *Session, r *http.Request) (any, error) {
	var req oem.SetFanModeRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	return s.oem.SetFanMode(r.Context(), req)
}

// handleFanTables serves GET /api/v1/vendor/asrock/fanTables.
//
//	@Summary		Fan curve tables
//	@Description	Tier: **read**. X470/X570: two-step per table, held under the per-host exclusive AMI lane so no other request interleaves: `POST asrr/settings/setopenlooptablenumber {table_numble}` then `GET asrr/settings/getfanopenloopcontroltable` (Get_Temp1..24/Get_Duty1..24); closed loop uses setcloselooptablenumber + getfancloseloopcontroltable. B650 4.x: `GET settings/fan/openloop` / `closedloop` (one call, no selection step).
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			type			query		string	false	"comma list of open,closed"	default(open,closed)
//	@Param			table			query		string	false	"comma list of table numbers (X470)"	default(1)
//	@Success		200				{object}	oem.FanTablesResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/fanTables [get]
func handleFanTables(s *Session, r *http.Request) (any, error) {
	refs, err := fanRefs(r)
	if err != nil {
		return nil, err
	}
	return s.oem.FanTables(r.Context(), refs)
}

// handleSetFanTable serves POST /api/v1/vendor/asrock/setFanTable.
//
//	@Summary		Write a fan curve table
//	@Description	Tier: **write**. X470/X570: one `POST asrr/settings/setfanopenloopcontroltable` {table_numble 1..8, temp1..24, duty1..24} or `setfancloseloopcontroltable` {table_numble 1..10 plus the closed-loop fields in raw} (names and keys from the X470 UI JS; never written on hardware). B650 4.x: `PUT settings/fan/openloop/{table}` / `closedloop/{table}` (raw object; the collections themselves are GET-only). Returns 204.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			body			body		oem.SetFanTableRequest	true	"table"
//	@Success		204				"no content"
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/setFanTable [post]
func handleSetFanTable(s *Session, r *http.Request) (any, error) {
	var req oem.SetFanTableRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	return nil, s.oem.SetFanTable(r.Context(), req)
}

// handleFanDuty serves GET /api/v1/vendor/asrock/fanDuty.
//
//	@Summary		Current fan duty
//	@Description	Tier: **read**. X470: `GET asrr/GetFanDuty` (current FANn_Duty %, source `current`). X570: GetFanDuty is absent (404 on 1.20), so `GET asrr/settings/getfandutymanualmode` (configured manual duty1..16, source `manual`). B650 4.x: `GET settings/fan/manual` (source `manual`).
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.FanDutyResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/fanDuty [get]
func handleFanDuty(s *Session, r *http.Request) (any, error) {
	return s.oem.FanDuty(r.Context())
}

// handleSetManualFanDuty serves POST /api/v1/vendor/asrock/setManualFanDuty.
//
//	@Summary		Set manual fan duty
//	@Description	Tier: **write**. X470/X570: `POST asrr/settings/setfandutymanualmode` {duty1..16} (fans must be in manual mode). B650 4.x: `PUT settings/fan/manual` (raw object merged over the current one). Duty is 0..100 percent. Returns 204.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			body			body		oem.SetManualFanDutyRequest	true	"duties in percent keyed by fan number 1..16"
//	@Success		204				"no content"
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/setManualFanDuty [post]
func handleSetManualFanDuty(s *Session, r *http.Request) (any, error) {
	var req oem.SetManualFanDutyRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	return nil, s.oem.SetManualFanDuty(r.Context(), req)
}

// handleResetFanSettings serves POST /api/v1/vendor/asrock/resetFanSettings.
//
//	@Summary		Reset fan settings to defaults
//	@Description	Tier: **write**. X470/X570: `POST asrr/settings/setclearallfansetting`. B650 4.x: `POST settings/fan/load_default`. Returns 204.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		204				"no content"
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/resetFanSettings [post]
func handleResetFanSettings(s *Session, r *http.Request) (any, error) {
	return nil, s.oem.ResetFanSettings(r.Context())
}

// handlePowerSource serves GET /api/v1/vendor/asrock/powerSource.
//
//	@Summary		PSU / power source readings
//	@Description	Tier: **read**. Both boards: `GET asrr/PowerSource` (per-PSU AC input / 12 V output). Boards without PMBus PSUs return zero rows or 501.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.PowerSourceResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/powerSource [get]
func handlePowerSource(s *Session, r *http.Request) (any, error) {
	return s.oem.PowerSource(r.Context())
}

// handlePostCode serves GET /api/v1/vendor/asrock/postCode.
//
//	@Summary		Last BIOS POST code
//	@Description	Tier: **read**. X470 (and B650 1.x): `GET asrr/getbioscode` {poststatus, postdata}. B650 4.x and X570: `GET logs/postcode`.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.PostCodeResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/postCode [get]
func handlePostCode(s *Session, r *http.Request) (any, error) {
	return s.oem.PostCode(r.Context())
}

// handleMCALog serves GET /api/v1/vendor/asrock/mcaLog.
//
//	@Summary		CPU machine-check (MCA) dump status
//	@Description	Tier: **read**. Both boards: `GET asrr/getmcalog` {MCADump}. The dump file download (`asrr/mca-file`) is denylisted (it crashes the web backend).
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.MCALogResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/mcaLog [get]
func handleMCALog(s *Session, r *http.Request) (any, error) {
	return s.oem.MCALog(r.Context())
}

// handleClearMCALog serves POST /api/v1/vendor/asrock/clearMcaLog.
//
//	@Summary		Clear the MCA dump
//	@Description	Tier: **disruptive** (diagnostic data is lost). X470: `DELETE asrr/clearmcalog`. B650/B550/X570: 501 (no handler). Returns 204.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	true	"Must be true; the proxy must also run with --allow-disruptive"
//	@Success		204				"no content"
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/clearMcaLog [post]
func handleClearMCALog(s *Session, r *http.Request) (any, error) {
	return nil, s.oem.ClearMCALog(r.Context())
}

// handleProchot serves GET /api/v1/vendor/asrock/prochot.
//
//	@Summary		CPU PROCHOT assertion setting
//	@Description	Tier: **read**. X470 only: `GET asrr/getprochot` {prochot}. B650/B550/X570: 501.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.ProchotResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/prochot [get]
func handleProchot(s *Session, r *http.Request) (any, error) {
	return s.oem.Prochot(r.Context())
}

// handleSetProchot serves POST /api/v1/vendor/asrock/setProchot.
//
//	@Summary		Set CPU PROCHOT assertion
//	@Description	Tier: **write**. X470 only: `PUT asrr/setprochot` {prochot}. B650/B550/X570: 501.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			body			body		oem.SetProchotRequest	true	"desired state"
//	@Success		200				{object}	oem.ProchotResponse
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/setProchot [post]
func handleSetProchot(s *Session, r *http.Request) (any, error) {
	var req oem.SetProchotRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	return s.oem.SetProchot(r.Context(), req.Enabled)
}

// handleWebEnableTimeout serves GET /api/v1/vendor/asrock/webEnableTimeout.
//
//	@Summary		Web session idle-timeout switch
//	@Description	Tier: **read**. Both boards: `GET asrr/getWebEnableTimeout` {WebEnableTimeout}.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.WebEnableTimeout
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/webEnableTimeout [get]
func handleWebEnableTimeout(s *Session, r *http.Request) (any, error) {
	return s.oem.WebEnableTimeout(r.Context())
}

// handleSetWebEnableTimeout serves POST /api/v1/vendor/asrock/setWebEnableTimeout.
//
//	@Summary		Set web session idle-timeout switch
//	@Description	Tier: **write**. Both boards: `POST asrr/setWebEnableTimeout` {enable_timeout}.
//	@Tags			asrock
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			body			body		oem.SetWebEnableTimeoutRequest	true	"desired state"
//	@Success		200				{object}	oem.WebEnableTimeout
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/setWebEnableTimeout [post]
func handleSetWebEnableTimeout(s *Session, r *http.Request) (any, error) {
	var req oem.SetWebEnableTimeoutRequest
	if err := bmc.DecodeJSON(r, &req); err != nil {
		return nil, err
	}
	return s.oem.SetWebEnableTimeout(r.Context(), req.Enabled)
}

// handleBIOSUpdateOption serves GET /api/v1/vendor/asrock/biosUpdateOption.
//
//	@Summary		BIOS update option
//	@Description	Tier: **read**. Both boards: `GET asrr/maintenance/BIOS/UpdateOption` {action} (preserve/clear NVRAM on flash; value encoding unverified).
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Success		200				{object}	oem.BIOSUpdateOption
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/biosUpdateOption [get]
func handleBIOSUpdateOption(s *Session, r *http.Request) (any, error) {
	return s.oem.BIOSUpdateOption(r.Context())
}

// handleComponentStatus serves GET /api/v1/vendor/asrock/componentStatus/{component}.
//
//	@Summary		Firmware component flash status
//	@Description	Tier: **read**. Both boards: `GET asrr/maintenance/{C}/status` and `.../flash-progress`. Components per board: X470 BIOS, CPLD; B650 4.x BIOS, CPLD, FPGA, RETIMER; X570 BIOS, BP; others 501. Accepted names: BIOS, CPLD, FPGA, RETIMER, PSU, BP.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			component		path		string	true	"component"	Enums(BIOS,CPLD,FPGA,RETIMER,PSU,BP)
//	@Success		200				{object}	oem.ComponentStatus
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/componentStatus/{component} [get]
func handleComponentStatus(s *Session, r *http.Request) (any, error) {
	return s.oem.ComponentStatus(r.Context(), r.PathValue("component"))
}

// handleUpdateComponent serves POST /api/v1/vendor/asrock/updateComponent/{component}.
//
//	@Summary		Flash a firmware component (not implemented)
//	@Description	Tier: **disruptive**. Planned: `asrr/maintenance/{C}/firmware` upload, `upgrade`, then poll `flash-progress`. Not implemented in v1: always 501 after validating the component name.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	true	"Must be true; the proxy must also run with --allow-disruptive"
//	@Param			component		path		string	true	"component"	Enums(BIOS,CPLD,FPGA,RETIMER,PSU,BP)
//	@Success		204				"no content"
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/updateComponent/{component} [post]
func handleUpdateComponent(s *Session, r *http.Request) (any, error) {
	if _, err := oem.NormalizeComponent(r.PathValue("component")); err != nil {
		return nil, err
	}
	return nil, bmc.Unsupported(Pkg, "updateComponent (firmware flashing is not implemented in v1)")
}

// handleBMCColdReset serves POST /api/v1/vendor/asrock/bmcColdReset.
//
//	@Summary		BMC cold reset
//	@Description	Tier: **disruptive** (the BMC reboots; unreachable for ~1-3 minutes, every session on the host is lost). Both boards: `PUT asrr/bmccoldreset {}`. The proxy marks the session lost so the next call reopens.
//	@Tags			asrock
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (asrock)"
//	@Param			X-BMC-Confirm	header		string	true	"Must be true; the proxy must also run with --allow-disruptive"
//	@Success		200				{object}	oem.ColdResetResult
//	@Failure		400				{object}	server.ErrorBody	"bad request / X-BMC-Host"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		403				{object}	server.ErrorBody	"tier gate (disruptive) or forbidden path"
//	@Failure		404				{object}	server.ErrorBody	"target handled by another driver"
//	@Failure		501				{object}	server.ErrorBody	"not supported on this board/firmware"
//	@Failure		502				{object}	server.ErrorBody	"BMC error (vendor detail included)"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/vendor/asrock/bmcColdReset [post]
func handleBMCColdReset(s *Session, r *http.Request) (any, error) {
	if err := s.oem.BMCColdReset(r.Context()); err != nil {
		return nil, err
	}
	s.ami.MarkHostLost("bmcColdReset requested")
	return oem.ColdResetResult{Accepted: true, Message: "BMC cold reset requested; the BMC is unreachable for ~1-3 minutes"}, nil
}
