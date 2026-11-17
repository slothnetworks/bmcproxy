package v1

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// Annotation conventions: every BMC route takes X-BMC-Host (required) and
// X-BMC-Vendor (optional) headers plus Basic auth (the BMC's own
// credentials). cmd/openapigen rewrites these header
// parameters into shared components in openapi.json.

// getCapabilities reports what the target supports.
//
//	@Summary		Capabilities of the target BMC
//	@Description	Tier: **read**. Resolves the driver (auto-detected or `X-BMC-Vendor`), opens or reuses a session and reports, per generic capability, whether it is supported and which backend (redfish, ami, ipmi) serves it, plus the driver's vendor routes and their tiers. ASRock: detection via `asrr/model-name`/`asrr/fw-info` (AMI /api) or Redfish ServiceRoot; IPMI Get Device ID as fallback. Large clock skew is reported in `warnings`.
//	@Tags			capabilities
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	CapabilitiesResponse
//	@Failure		400				{object}	server.ErrorBody	"bad X-BMC-Host / unknown vendor"
//	@Failure		401				{object}	server.ErrorBody	"BMC rejected the credentials"
//	@Failure		501				{object}	server.ErrorBody	"no driver matches this BMC"
//	@Failure		502				{object}	server.ErrorBody	"BMC error"
//	@Failure		503				{object}	server.ErrorBody	"BMC busy / circuit open"
//	@Failure		504				{object}	server.ErrorBody	"BMC timeout"
//	@Security		BMCBasicAuth
//	@Router			/api/v1/capabilities [get]
func (a *api) getCapabilities(r *http.Request) (any, error) {
	return a.s.Call(r, func(ctx context.Context, h *sessions.Handle) (any, error) {
		body := CapabilitiesResponse{CapabilitiesReport: server.BuildCapabilities(h)}
		if off, ok := a.s.Sessions().Targets().ClockOffset(h.Key.Host); ok {
			secs := off.Seconds()
			body.ClockOffsetSeconds = &secs
			if w := SkewWarning(off, a.cfg.SkewWarn); w != "" {
				body.Warnings = append(body.Warnings, w)
			}
		}
		return body, nil
	})
}

// getSystem returns system identity.
//
//	@Summary		System identity
//	@Description	Tier: **read** (cached briefly, concurrent calls coalesced). Manufacturer, board, serial, BMC/BIOS versions. ASRock AMI /api: `asrr/fw-info`, `firmware-info`, `asrr/model-name`; Redfish: `Systems/Self`, `Managers/Self`; IPMI: mc info.
//	@Tags			system
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	bmc.SystemInfo
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody	"capability not supported"
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/system [get]
func (a *api) getSystem(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapIdentity, "SystemInfo", "", defTTL, func(ctx context.Context, c bmc.Identity) (any, error) {
		return c.SystemInfo(ctx)
	})
}

// getPower returns the host power state.
//
//	@Summary		Host power state
//	@Description	Tier: **read** (cached briefly). ASRock AMI /api: `chassis-status`; Redfish: `Systems/Self.PowerState`; IPMI: chassis status.
//	@Tags			power
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	PowerStateResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/power [get]
func (a *api) getPower(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapPower, "PowerState", "", defTTL, func(ctx context.Context, c bmc.PowerController) (any, error) {
		st, err := c.PowerState(ctx)
		return PowerStateResponse{State: st}, err
	})
}

// postPower performs a power action and polls for the result.
//
//	@Summary		Power action
//	@Description	Tier: **write**. Actions: on, off (hard), shutdown (graceful ACPI), cycle, reset, nmi. ASRock AMI /api: `actions/power` codes 1,0,5,2,3; Redfish: `ComputerSystem.Reset`; IPMI: chassis power.
//	@Description	The proxy never assumes the action took effect (a graceful shutdown can be silently ignored, e.g. mid-POST): it polls the power state until it reaches the expected state or `timeoutSeconds` passes. 200 = converged; 202 = accepted but not observed (see `warning`), or `wait=false`, or nmi.
//	@Tags			power
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string			true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string			false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			wait			query		bool			false	"Poll for convergence (default true)"
//	@Param			body			body		PowerRequest	true	"Power action"
//	@Success		200				{object}	PowerActionResult	"state converged"
//	@Success		202				{object}	PowerActionResult	"accepted; not (yet) converged"
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/power [post]
func (a *api) postPower(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapPower, "Power", func(ctx context.Context, c bmc.PowerController, h *sessions.Handle) (any, error) {
		return a.powerAction(ctx, c, h, r)
	})
}

// getRestorePolicy returns the AC power-loss restore policy.
//
//	@Summary		Power restore policy
//	@Description	Tier: **read**. ASRock: `asrr/GetPowerRestore` (B650) / IPMI chassis policy (X470); Redfish: `PowerRestorePolicy`.
//	@Tags			power
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	RestorePolicyBody
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/power/restore-policy [get]
func (a *api) getRestorePolicy(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapPowerRestorePolicy, "GetPowerRestorePolicy", "", defTTL, func(ctx context.Context, c bmc.PowerRestorePolicy) (any, error) {
		p, err := c.GetPowerRestorePolicy(ctx)
		return RestorePolicyBody{Policy: p}, err
	})
}

// putRestorePolicy sets the AC power-loss restore policy.
//
//	@Summary		Set power restore policy
//	@Description	Tier: **write**. ASRock: `asrr/SetPowerRestore` (B650) / IPMI chassis policy (X470); Redfish: `PowerRestorePolicy` PATCH.
//	@Tags			power
//	@Accept			json
//	@Param			X-BMC-Host		header	string				true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string				false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			body			body	RestorePolicyBody	true	"New policy"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/power/restore-policy [put]
func (a *api) putRestorePolicy(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapPowerRestorePolicy, "SetPowerRestorePolicy", func(ctx context.Context, c bmc.PowerRestorePolicy, _ *sessions.Handle) (any, error) {
		var body RestorePolicyBody
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		p, err := bmc.ParseRestorePolicy(string(body.Policy))
		if err != nil {
			return nil, err
		}
		return nil, c.SetPowerRestorePolicy(ctx, p)
	})
}

// getLED returns the identify LED state.
//
//	@Summary		Identify LED state
//	@Description	Tier: **read**. ASRock AMI /api: chassis identify status; Redfish: `IndicatorLED`; IPMI: chassis identify.
//	@Tags			led
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	LEDBody
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/led [get]
func (a *api) getLED(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapIndicatorLED, "GetIndicatorLED", "", defTTL, func(ctx context.Context, c bmc.IndicatorLED) (any, error) {
		st, err := c.GetIndicatorLED(ctx)
		return LEDBody{State: st}, err
	})
}

// postLED sets the identify LED.
//
//	@Summary		Set identify LED
//	@Description	Tier: **write**. ASRock AMI /api: `actions/chassis-led`; Redfish: `IndicatorLED` PATCH; IPMI: chassis identify.
//	@Tags			led
//	@Accept			json
//	@Param			X-BMC-Host		header	string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			body			body	LEDBody	true	"on, off or blinking"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/led [post]
func (a *api) postLED(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapIndicatorLED, "SetIndicatorLED", func(ctx context.Context, c bmc.IndicatorLED, _ *sessions.Handle) (any, error) {
		var body LEDBody
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		switch body.State {
		case bmc.LEDOn, bmc.LEDOff, bmc.LEDBlinking:
		default:
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "SetIndicatorLED", "state must be on, off or blinking")
		}
		return nil, c.SetIndicatorLED(ctx, body.State)
	})
}

// getBoot returns the boot override.
//
//	@Summary		Boot override
//	@Description	Tier: **read**. No AMI /api endpoint; Redfish: `Boot.BootSourceOverride*`; IPMI: chassis bootparam (X470 path).
//	@Tags			boot
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	bmc.BootOverrideConfig
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/boot [get]
func (a *api) getBoot(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapBootOverride, "GetBootOverride", "", defTTL, func(ctx context.Context, c bmc.BootOverride) (any, error) {
		return c.GetBootOverride(ctx)
	})
}

// putBoot sets the boot override.
//
//	@Summary		Set boot override
//	@Description	Tier: **write**. target: none, pxe, disk, cd, usb, bios, floppy; mode: once, continuous, disabled; bootMode: uefi, legacy or empty. Redfish: `Boot.BootSourceOverride*` PATCH; IPMI: chassis bootdev.
//	@Tags			boot
//	@Accept			json
//	@Param			X-BMC-Host		header	string					true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string					false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			body			body	bmc.BootOverrideConfig	true	"Boot override"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/boot [put]
func (a *api) putBoot(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapBootOverride, "SetBootOverride", func(ctx context.Context, c bmc.BootOverride, _ *sessions.Handle) (any, error) {
		var body bmc.BootOverrideConfig
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		if err := validBoot(body); err != nil {
			return nil, err
		}
		return nil, c.SetBootOverride(ctx, body)
	})
}

func validBoot(b bmc.BootOverrideConfig) error {
	switch b.Target {
	case bmc.BootNone, bmc.BootPXE, bmc.BootDisk, bmc.BootCD, bmc.BootUSB, bmc.BootBIOS, bmc.BootFloppy:
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, pkg, "SetBootOverride", "invalid target %q", b.Target)
	}
	switch b.Mode {
	case bmc.BootOnce, bmc.BootContinuous, bmc.BootDisabled:
	case "":
		return bmc.Errorf(bmc.ErrInvalidArgument, pkg, "SetBootOverride", "mode is required (once, continuous, disabled)")
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, pkg, "SetBootOverride", "invalid mode %q", b.Mode)
	}
	switch strings.ToLower(b.BootMode) {
	case "", "uefi", "legacy":
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, pkg, "SetBootOverride", "invalid bootMode %q", b.BootMode)
	}
	return nil
}

// getSensors returns sensor readings.
//
//	@Summary		Sensor readings
//	@Description	Tier: **read** (cached briefly; N concurrent identical requests cost one BMC call). Units normalized (C, V, RPM, A, W, %); unset thresholds ("NA") are null. ASRock AMI /api: `sensors` (+`sdr`); Redfish: `Chassis/Self/Sensors`, Thermal, Power; IPMI: sdr elist.
//	@Tags			sensors
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	SensorsResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/sensors [get]
func (a *api) getSensors(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapSensors, "Sensors", "", defTTL, func(ctx context.Context, c bmc.SensorReader) (any, error) {
		v, err := c.Sensors(ctx)
		return SensorsResponse{Sensors: nonNil(v)}, err
	})
}

// getEvents returns the system event log incrementally.
//
//	@Summary		System event log
//	@Description	Tier: **read**. Incremental by record id: pass the previous `lastId` as `since`. Each entry's `time` has `raw` (BMC clock) and `corrected` (raw minus the measured BMC clock offset, i.e. real UTC). ASRock AMI /api: `logs/event?LASTEVENTID=n`; Redfish: `LogServices/SEL/Entries`; IPMI: sel list.
//	@Tags			logs
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			since			query		string	false	"Return entries after this record id"
//	@Param			limit			query		int		false	"Maximum entries (0 = driver default)"
//	@Success		200				{object}	EventsResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/events [get]
func (a *api) getEvents(r *http.Request) (any, error) {
	limit, err := queryLimit(r)
	if err != nil {
		return nil, err
	}
	q := bmc.EventQuery{SinceID: r.URL.Query().Get("since"), Limit: limit}
	v, err := cachedRead(a, r, bmc.CapEventLog, "Events", server.CanonicalParams(r, "since", "limit"), defTTL, func(ctx context.Context, c bmc.EventLog) (any, error) {
		return c.Events(ctx, q)
	})
	if err != nil {
		return nil, err
	}
	page := v.(bmc.EventPage)
	// Copy: the cached page is shared between callers.
	body := EventsResponse{Entries: make([]bmc.SELEntry, len(page.Entries)), LastID: page.LastID, More: page.More}
	copy(body.Entries, page.Entries)
	if off, ok := a.hostOffset(r); ok {
		secs := off.Seconds()
		body.ClockOffsetSeconds = &secs
		for i := range body.Entries {
			body.Entries[i].Time = bmc.NewTimestamp(body.Entries[i].Time.Raw, off)
		}
	}
	return body, nil
}

// deleteEvents clears the system event log.
//
//	@Summary		Clear the system event log
//	@Description	Tier: **disruptive** (log data is lost). ASRock AMI /api: `DELETE logs/event`; Redfish: `LogService.ClearLog`; IPMI: sel clear.
//	@Tags			logs
//	@Param			X-BMC-Host		header	string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/events [delete]
func (a *api) deleteEvents(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapEventLogClear, "ClearEvents", func(ctx context.Context, c bmc.EventLogClearer, _ *sessions.Handle) (any, error) {
		return nil, c.ClearEvents(ctx)
	})
}

// getAudit returns the audit log.
//
//	@Summary		Audit log
//	@Description	Tier: **read**. Times carry raw and skew-corrected values. ASRock AMI /api: `logs/audit`; Redfish: Manager `LogServices`.
//	@Tags			logs
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			since			query		string	false	"Return entries after this id"
//	@Param			limit			query		int		false	"Maximum entries"
//	@Success		200				{object}	AuditResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/logs/audit [get]
func (a *api) getAudit(r *http.Request) (any, error) {
	limit, err := queryLimit(r)
	if err != nil {
		return nil, err
	}
	q := bmc.AuditQuery{SinceID: r.URL.Query().Get("since"), Limit: limit}
	v, err := cachedRead(a, r, bmc.CapAuditLog, "AuditLog", server.CanonicalParams(r, "since", "limit"), defTTL, func(ctx context.Context, c bmc.AuditLog) (any, error) {
		return c.AuditLog(ctx, q)
	})
	if err != nil {
		return nil, err
	}
	src := v.([]bmc.AuditEntry)
	body := AuditResponse{Entries: make([]bmc.AuditEntry, len(src))}
	copy(body.Entries, src)
	if off, ok := a.hostOffset(r); ok {
		secs := off.Seconds()
		body.ClockOffsetSeconds = &secs
		for i := range body.Entries {
			body.Entries[i].Time = bmc.NewTimestamp(body.Entries[i].Time.Raw, off)
		}
	}
	return body, nil
}

// getFRU returns FRU devices.
//
//	@Summary		FRU devices
//	@Description	Tier: **read**. ASRock AMI /api: `fru`; Redfish: Chassis/Assembly; IPMI: fru.
//	@Tags			inventory
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	FRUResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/fru [get]
func (a *api) getFRU(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapFRU, "FRU", "", defTTL, func(ctx context.Context, c bmc.FRUReader) (any, error) {
		v, err := c.FRU(ctx)
		return FRUResponse{FRU: nonNil(v)}, err
	})
}

// getInventory returns one inventory collection.
//
//	@Summary		Inventory
//	@Description	Tier: **read**. kind: cpus, memory, pcie, storage, baseboard; the matching field of the response is set. ASRock AMI /api: `host_inventory/*` (X470: `system_inventory/*`); Redfish: Processors, Memory, PCIeDevices, Storage. A kind the board cannot serve is 501.
//	@Tags			inventory
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			kind			path		string	true	"Inventory kind"	Enums(cpus, memory, pcie, storage, baseboard)
//	@Success		200				{object}	InventoryResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		404				{object}	server.ErrorBody	"unknown kind"
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/inventory/{kind} [get]
func (a *api) getInventory(r *http.Request) (any, error) {
	kind := bmc.InventoryKind(r.PathValue("kind"))
	return cachedRead(a, r, bmc.CapInventory, "Inventory", string(kind), defTTL, func(ctx context.Context, c bmc.Inventory) (any, error) {
		res := InventoryResponse{Kind: kind}
		var err error
		switch kind {
		case bmc.InventoryCPUs:
			var x []bmc.CPU
			x, err = c.CPUs(ctx)
			res.CPUs = nonNil(x)
		case bmc.InventoryMemory:
			var x []bmc.MemoryModule
			x, err = c.Memory(ctx)
			res.Memory = nonNil(x)
		case bmc.InventoryPCIe:
			var x []bmc.PCIeDevice
			x, err = c.PCIeDevices(ctx)
			res.PCIe = nonNil(x)
		case bmc.InventoryStorage:
			var x []bmc.StorageDevice
			x, err = c.Storage(ctx)
			res.Storage = nonNil(x)
		case bmc.InventoryBaseboard:
			var x bmc.Baseboard
			x, err = c.Baseboard(ctx)
			res.Baseboard = &x
		default:
			return nil, bmc.Errorf(bmc.ErrNotFound, pkg, "Inventory", "unknown inventory kind %q (cpus, memory, pcie, storage, baseboard)", kind)
		}
		if err != nil {
			return nil, err
		}
		return res, nil
	})
}

// getNetwork returns BMC network interfaces.
//
//	@Summary		BMC network interfaces
//	@Description	Tier: **read**. ASRock AMI /api: `settings/network`; Redfish: `EthernetInterfaces`; IPMI: lan print.
//	@Tags			network
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	NetworkResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/network [get]
func (a *api) getNetwork(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapNetwork, "NetworkInterfaces", "", defTTL, func(ctx context.Context, c bmc.NetworkManager) (any, error) {
		v, err := c.NetworkInterfaces(ctx)
		return NetworkResponse{Interfaces: nonNil(v)}, err
	})
}

// putNetwork updates one BMC network interface.
//
//	@Summary		Update a BMC network interface
//	@Description	Tier: **disruptive** (can cut BMC connectivity). Null fields are unchanged; the driver reads the full vendor object, mutates and writes it back. ASRock AMI /api: `PUT settings/network`; Redfish: `EthernetInterfaces` PATCH.
//	@Tags			network
//	@Accept			json
//	@Param			X-BMC-Host		header	string						true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string						false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			body			body	bmc.NetworkInterfaceUpdate	true	"Interface update (name required)"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/network [put]
func (a *api) putNetwork(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapNetwork, "SetNetworkInterface", func(ctx context.Context, c bmc.NetworkManager, _ *sessions.Handle) (any, error) {
		var body bmc.NetworkInterfaceUpdate
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		if body.Name == "" {
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "SetNetworkInterface", "name is required")
		}
		return nil, c.SetNetworkInterface(ctx, body)
	})
}

// getUsers returns BMC accounts.
//
//	@Summary		BMC accounts
//	@Description	Tier: **read**. Never includes passwords. ASRock AMI /api: `settings/users`; Redfish: `AccountService/Accounts`; IPMI: user list.
//	@Tags			accounts
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	UsersResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/users [get]
func (a *api) getUsers(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapUsers, "Users", "", defTTL, func(ctx context.Context, c bmc.UserReader) (any, error) {
		v, err := c.Users(ctx)
		return UsersResponse{Users: nonNil(v)}, err
	})
}

// getServices returns BMC network services.
//
//	@Summary		BMC network services
//	@Description	Tier: **read**. ASRock AMI /api: `settings/services`; Redfish: `NetworkProtocol`.
//	@Tags			accounts
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	ServicesResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/services [get]
func (a *api) getServices(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapServices, "Services", "", defTTL, func(ctx context.Context, c bmc.ServiceReader) (any, error) {
		v, err := c.Services(ctx)
		return ServicesResponse{Services: nonNil(v)}, err
	})
}

// getTime returns the BMC clock and NTP configuration.
//
//	@Summary		BMC clock and NTP
//	@Description	Tier: **read** (coalesced, not cached). Also refreshes the proxy's measured clock offset used to correct event timestamps. ASRock AMI /api: `settings/date-time`; Redfish: Manager `DateTime`, `DateTimeLocalOffset` + `NetworkProtocol.NTP`.
//	@Tags			time
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	bmc.TimeConfig
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/time [get]
func (a *api) getTime(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapTime, "Time", "", 0, func(ctx context.Context, c bmc.TimeManager) (any, error) {
		tc, err := c.Time(ctx)
		if err == nil {
			a.recordOffset(r, tc)
		}
		return tc, err
	})
}

// putTime sets the BMC clock and/or NTP configuration.
//
//	@Summary		Set clock and/or NTP
//	@Description	Tier: **write**. Set `time` for a manual clock set, or `ntpEnabled`/`ntpServers`. The driver reads the full vendor object, mutates and writes it back (the X470 500s on partial bodies) and preserves timezone. NTP changes may restart BMC networking. Returns the re-read config (verify after write), or 204 if the re-read failed.
//	@Tags			time
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string			true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string			false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			body			body		bmc.TimeUpdate	true	"Fields to change"
//	@Success		200				{object}	bmc.TimeConfig
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/time [put]
func (a *api) putTime(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapTime, "SetTime", func(ctx context.Context, c bmc.TimeManager, _ *sessions.Handle) (any, error) {
		var body bmc.TimeUpdate
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		if body.Time == nil && body.NTPEnabled == nil && body.NTPServers == nil {
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "SetTime", "nothing to change")
		}
		if err := c.SetTime(ctx, body); err != nil {
			return nil, err
		}
		tc, err := c.Time(ctx)
		if err != nil {
			return nil, nil
		}
		a.recordOffset(r, tc)
		return tc, nil
	})
}

// postTimeSync sets the BMC clock to the proxy's current UTC.
//
//	@Summary		Correct BMC clock skew
//	@Description	Tier: **write**. Sets the BMC clock to the proxy's current UTC (compensating for the measured offset) and returns the resulting config. ASRock AMI /api: `PUT settings/date-time`; Redfish: Manager `DateTime` PATCH.
//	@Tags			time
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	bmc.TimeConfig
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/time/sync [post]
func (a *api) postTimeSync(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapTime, "SyncTime", func(ctx context.Context, c bmc.TimeManager, _ *sessions.Handle) (any, error) {
		tc, err := c.SyncTime(ctx)
		if err == nil {
			a.recordOffset(r, tc)
		}
		return tc, err
	})
}

// getFirmware returns the firmware inventory.
//
//	@Summary		Firmware inventory
//	@Description	Tier: **read**. ASRock AMI /api: `asrr/fw-info`, `maintenance/hpm/componentversions`; Redfish: `FirmwareInventory`.
//	@Tags			firmware
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	FirmwareResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/firmware [get]
func (a *api) getFirmware(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapFirmwareInventory, "Firmware", "", defTTL, func(ctx context.Context, c bmc.FirmwareInventory) (any, error) {
		v, err := c.Firmware(ctx)
		return FirmwareResponse{Components: nonNil(v)}, err
	})
}

// postFirmwareUpdate starts a firmware update.
//
//	@Summary		Start a firmware update
//	@Description	Tier: **disruptive**. Pull: JSON `{component, imageUri}` (Redfish SimpleUpdate). Push: `application/octet-stream` body with `?component=&imageName=`. Returns 501 until the driver implements firmware update (PLAN milestone 5). Poll the returned task via `GET /api/v1/firmware/tasks/{id}`.
//	@Tags			firmware
//	@Accept			json
//	@Accept			octet-stream
//	@Produce		json
//	@Param			X-BMC-Host		header		string						true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string						false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			component		query		string						false	"Component (push mode)"
//	@Param			imageName		query		string						false	"Image file name (push mode)"
//	@Param			body			body		bmc.FirmwareUpdateRequest	false	"Pull request (JSON mode)"
//	@Success		202				{object}	bmc.Task
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/firmware/update [post]
func (a *api) postFirmwareUpdate(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapFirmwareUpdate, "UpdateFirmware", func(ctx context.Context, c bmc.FirmwareUpdater, _ *sessions.Handle) (any, error) {
		var req bmc.FirmwareUpdateRequest
		ct := strings.ToLower(r.Header.Get("Content-Type"))
		switch {
		case strings.HasPrefix(ct, "application/json"):
			if err := bmc.DecodeJSON(r, &req); err != nil {
				return nil, err
			}
			if req.ImageURI == "" {
				return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "UpdateFirmware", "imageUri is required for a JSON request (or send the image as application/octet-stream)")
			}
		case strings.HasPrefix(ct, "application/octet-stream"):
			q := r.URL.Query()
			req = bmc.FirmwareUpdateRequest{Component: q.Get("component"), ImageName: q.Get("imageName"), Image: r.Body, ImageSize: r.ContentLength}
		default:
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "UpdateFirmware", "Content-Type must be application/json or application/octet-stream")
		}
		if req.Component == "" {
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "UpdateFirmware", "component is required")
		}
		task, err := c.UpdateFirmware(ctx, req)
		if err != nil {
			return nil, err
		}
		return server.WithStatus(http.StatusAccepted, task), nil
	})
}

// getFirmwareTask returns a firmware update task.
//
//	@Summary		Firmware update task
//	@Description	Tier: **read** (never cached).
//	@Tags			firmware
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			id				path		string	true	"Task id"
//	@Success		200				{object}	bmc.Task
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		404				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/firmware/tasks/{id} [get]
func (a *api) getFirmwareTask(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapFirmwareUpdate, "FirmwareTask", func(ctx context.Context, c bmc.FirmwareUpdater, _ *sessions.Handle) (any, error) {
		return c.FirmwareTask(ctx, r.PathValue("id"))
	})
}

// getVirtualMedia returns virtual media slots.
//
//	@Summary		Virtual media slots
//	@Description	Tier: **read**. ASRock AMI /api: `settings/media/remote/*`; Redfish: `VirtualMedia`.
//	@Tags			virtual-media
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	VirtualMediaResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/virtual-media [get]
func (a *api) getVirtualMedia(r *http.Request) (any, error) {
	return cachedRead(a, r, bmc.CapVirtualMedia, "VirtualMedia", "", defTTL, func(ctx context.Context, c bmc.VirtualMedia) (any, error) {
		v, err := c.VirtualMedia(ctx)
		return VirtualMediaResponse{Slots: nonNil(v)}, err
	})
}

// postInsertMedia inserts virtual media into a slot.
//
//	@Summary		Insert virtual media
//	@Description	Tier: **write**. `image` is an nfs://, cifs:// or http(s):// URL; the share password is redacted from logs. ASRock AMI /api: `settings/media/remote/*`; Redfish: `VirtualMedia.InsertMedia`.
//	@Tags			virtual-media
//	@Accept			json
//	@Param			X-BMC-Host		header	string					true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string					false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			id				path	string					true	"Slot id"
//	@Param			body			body	bmc.VirtualMediaInsert	true	"Media to insert (slotId optional; must match path)"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		404	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/virtual-media/{id}/insert [post]
func (a *api) postInsertMedia(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapVirtualMedia, "InsertMedia", func(ctx context.Context, c bmc.VirtualMedia, _ *sessions.Handle) (any, error) {
		var body bmc.VirtualMediaInsert
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
		if body.SlotID != "" && body.SlotID != r.PathValue("id") {
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "InsertMedia", "slotId in body does not match path")
		}
		body.SlotID = r.PathValue("id")
		if body.Image == "" {
			return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "InsertMedia", "image is required")
		}
		return nil, c.InsertMedia(ctx, body)
	})
}

// postEjectMedia ejects virtual media from a slot.
//
//	@Summary		Eject virtual media
//	@Description	Tier: **write**. Redfish: `VirtualMedia.EjectMedia`; ASRock AMI /api: `settings/media/remote/*`.
//	@Tags			virtual-media
//	@Param			X-BMC-Host		header	string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header	string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			id				path	string	true	"Slot id"
//	@Success		204
//	@Failure		400	{object}	server.ErrorBody
//	@Failure		401	{object}	server.ErrorBody
//	@Failure		404	{object}	server.ErrorBody
//	@Failure		501	{object}	server.ErrorBody
//	@Failure		502	{object}	server.ErrorBody
//	@Failure		503	{object}	server.ErrorBody
//	@Failure		504	{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/virtual-media/{id}/eject [post]
func (a *api) postEjectMedia(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapVirtualMedia, "EjectMedia", func(ctx context.Context, c bmc.VirtualMedia, _ *sessions.Handle) (any, error) {
		return nil, c.EjectMedia(ctx, r.PathValue("id"))
	})
}

// getKVM returns KVM launch information.
//
//	@Summary		KVM launch info
//	@Description	Tier: **read**. Never cached or coalesced (the response carries a short-lived token). ASRock AMI /api: `kvm/token`, `remote_control/get/kvm/launch`. The KVM websocket proxy itself is out of v1 scope.
//	@Tags			kvm
//	@Produce		json
//	@Param			X-BMC-Host		header		string	true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	bmc.KVMLaunchInfo
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/kvm [get]
func (a *api) getKVM(r *http.Request) (any, error) {
	return direct(a, r, bmc.CapKVM, "KVMLaunch", func(ctx context.Context, c bmc.KVMLauncher, _ *sessions.Handle) (any, error) {
		return c.KVMLaunch(ctx)
	})
}

// postBMCReset resets the BMC.
//
//	@Summary		Reset the BMC
//	@Description	Tier: **disruptive**. Body optional (default cold). All cached sessions to the BMC are dropped and the target is marked degraded while it reboots. ASRock AMI /api: `asrr/bmccoldreset`; Redfish: `Manager.Reset`; IPMI: mc reset cold.
//	@Tags			bmc
//	@Accept			json
//	@Produce		json
//	@Param			X-BMC-Host		header		string			true	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string			false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Param			body			body		BMCResetRequest	false	"Reset kind"
//	@Success		202				{object}	BMCResetResponse
//	@Failure		400				{object}	server.ErrorBody
//	@Failure		401				{object}	server.ErrorBody
//	@Failure		501				{object}	server.ErrorBody
//	@Failure		502				{object}	server.ErrorBody
//	@Failure		503				{object}	server.ErrorBody
//	@Failure		504				{object}	server.ErrorBody
//	@Security		BMCBasicAuth
//	@Router			/api/v1/bmc/reset [post]
func (a *api) postBMCReset(r *http.Request) (any, error) {
	var body BMCResetRequest
	if r.ContentLength != 0 && r.Body != nil && r.Body != http.NoBody {
		if err := bmc.DecodeJSON(r, &body); err != nil {
			return nil, err
		}
	}
	switch body.Kind {
	case "":
		body.Kind = bmc.BMCResetCold
	case bmc.BMCResetCold, bmc.BMCResetWarm:
	default:
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, pkg, "ResetBMC", "kind must be cold or warm")
	}
	_, err := direct(a, r, bmc.CapBMCReset, "ResetBMC", func(ctx context.Context, c bmc.BMCReset, _ *sessions.Handle) (any, error) {
		return nil, c.ResetBMC(ctx, body.Kind)
	})
	if err != nil {
		return nil, err
	}
	// Every session to this BMC dies with the reset: drop them (logout is
	// best-effort) and re-detect next time.
	if t, ok := server.TargetFrom(r.Context()); ok {
		host := t.Key()
		a.s.Sessions().Targets().MarkDegraded(host, "BMC reset in progress", 5*time.Minute)
		go a.s.Sessions().InvalidateHost(host)
	}
	return server.WithStatus(http.StatusAccepted, BMCResetResponse{Accepted: true, Kind: body.Kind}), nil
}
