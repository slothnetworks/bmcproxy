package asrock

import (
	"context"
	"errors"
	"net/http"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
)

func errorsIs(err, target error) bool { return errors.Is(err, target) }

// chassisStatus is GET /api/chassis-status. X470: {power_status, led_status};
// B650 adds power_cycle_interval and power_on_after_flash.
type chassisStatus struct {
	PowerStatus amimegarac.NullableInt `json:"power_status"`
	LEDStatus   amimegarac.NullableInt `json:"led_status"`
}

func (c chassisStatus) power() bmc.PowerState {
	if !c.PowerStatus.Valid {
		return bmc.PowerUnknown
	}
	if c.PowerStatus.V != 0 {
		return bmc.PowerOn
	}
	return bmc.PowerOff
}

// led maps led_status, which uses the uid_command codes: 0 off and 2 on
// are live-confirmed on the X470 2.32 (docs/x470/live-probe.md §5.3/§12:
// uid_command 2 -> led_status 2, 0 -> 0). 1 = "Temporary On" (15 s blink)
// comes from the UI template. UNVERIFIED: 1 was never sent live, and the
// B650 encoding is assumed identical (same UI model).
func (c chassisStatus) led() bmc.LEDState {
	if !c.LEDStatus.Valid {
		return bmc.LEDUnknown
	}
	switch c.LEDStatus.V {
	case 0:
		return bmc.LEDOff
	case 1:
		return bmc.LEDBlinking
	case 2:
		return bmc.LEDOn
	}
	return bmc.LEDUnknown
}

func (s *Session) amiChassis(ctx context.Context) (chassisStatus, error) {
	var st chassisStatus
	err := s.api.Get(ctx, "chassis-status", &st)
	return st, err
}

// AMI power_command codes (POST /api/actions/power). 1, 5 and 0 were sent
// live on the X470 2.32 (docs/x470/live-probe.md §12: on, ignored soft-off,
// hard off); 2 and 3 come from the UI JS and the HAR analysis.
var amiPowerCodes = map[bmc.PowerAction]int{
	bmc.ActionOff:   0,
	bmc.ActionOn:    1,
	bmc.ActionCycle: 2,
	bmc.ActionReset: 3,
	// UNVERIFIED: 4 is IPMI chassis control "pulse diagnostic interrupt";
	// the UI never offers it and it was never sent live
	// (docs/x470/har-analysis.md §power, "whether 4 (NMI) is accepted").
	bmc.ActionNMI:      4,
	bmc.ActionShutdown: 5,
}

// PowerState implements bmc.PowerController.
func (s *Session) PowerState(ctx context.Context) (bmc.PowerState, error) {
	switch s.backend(bmc.CapPower) {
	case bmc.BackendRedfish:
		return s.Session.PowerState(ctx)
	case bmc.BackendAMI:
		st, err := s.amiChassis(ctx)
		if err != nil {
			return bmc.PowerUnknown, err
		}
		return st.power(), nil
	case bmc.BackendIPMI:
		ic, err := s.ipmiClient(ctx)
		if err != nil {
			return bmc.PowerUnknown, err
		}
		return ic.PowerState(ctx)
	}
	return bmc.PowerUnknown, s.unsupported("PowerState")
}

// Power implements bmc.PowerController. AMI: POST actions/power
// {"power_command": 1|0|5|2|3} for on|off|shutdown|cycle|reset. It does not
// wait: a graceful shutdown may be silently ignored mid-POST, so the API
// layer polls PowerState.
func (s *Session) Power(ctx context.Context, a bmc.PowerAction) error {
	if _, err := bmc.ParsePowerAction(string(a)); err != nil {
		return err
	}
	switch s.backend(bmc.CapPower) {
	case bmc.BackendRedfish:
		return s.Session.Power(ctx, a)
	case bmc.BackendAMI:
		code, ok := amiPowerCodes[a]
		if !ok {
			return bmc.Unsupported(Pkg, "Power "+string(a))
		}
		return s.api.DoJSON(ctx, http.MethodPost, "actions/power", map[string]any{"power_command": code}, nil)
	case bmc.BackendIPMI:
		ic, err := s.ipmiClient(ctx)
		if err != nil {
			return err
		}
		return ic.Power(ctx, a)
	}
	return s.unsupported("Power")
}

// GetPowerRestorePolicy implements bmc.PowerRestorePolicy. B650:
// asrr/GetPowerRestore; X470: IPMI chassis status (no /api handler).
func (s *Session) GetPowerRestorePolicy(ctx context.Context) (bmc.RestorePolicy, error) {
	switch s.backend(bmc.CapPowerRestorePolicy) {
	case bmc.BackendRedfish:
		return s.Session.GetPowerRestorePolicy(ctx)
	case bmc.BackendAMI:
		p, err := s.oem.GetPowerRestore(ctx)
		if errors.Is(err, bmc.ErrUnsupported) && s.prof.IPMI {
			return s.ipmiRestore(ctx)
		}
		return p, err
	case bmc.BackendIPMI:
		return s.ipmiRestore(ctx)
	}
	return bmc.RestoreUnknown, s.unsupported("GetPowerRestorePolicy")
}

func (s *Session) ipmiRestore(ctx context.Context) (bmc.RestorePolicy, error) {
	ic, err := s.ipmiClient(ctx)
	if err != nil {
		return bmc.RestoreUnknown, err
	}
	return ic.GetPowerRestorePolicy(ctx)
}

// SetPowerRestorePolicy implements bmc.PowerRestorePolicy. B650: POST
// asrr/SetPowerRestore {powerRestore}; X470: IPMI set power restore policy.
func (s *Session) SetPowerRestorePolicy(ctx context.Context, p bmc.RestorePolicy) error {
	if _, err := bmc.ParseRestorePolicy(string(p)); err != nil {
		return err
	}
	switch s.backend(bmc.CapPowerRestorePolicy) {
	case bmc.BackendRedfish:
		return s.Session.SetPowerRestorePolicy(ctx, p)
	case bmc.BackendAMI:
		err := s.oem.SetPowerRestore(ctx, p)
		if errors.Is(err, bmc.ErrUnsupported) && s.prof.IPMI {
			ic, ierr := s.ipmiClient(ctx)
			if ierr != nil {
				return ierr
			}
			return ic.SetPowerRestorePolicy(ctx, p)
		}
		return err
	case bmc.BackendIPMI:
		ic, err := s.ipmiClient(ctx)
		if err != nil {
			return err
		}
		return ic.SetPowerRestorePolicy(ctx, p)
	}
	return s.unsupported("SetPowerRestorePolicy")
}

// GetIndicatorLED implements bmc.IndicatorLED (AMI: chassis-status led_status).
func (s *Session) GetIndicatorLED(ctx context.Context) (bmc.LEDState, error) {
	switch s.backend(bmc.CapIndicatorLED) {
	case bmc.BackendRedfish:
		return s.Session.GetIndicatorLED(ctx)
	case bmc.BackendAMI:
		st, err := s.amiChassis(ctx)
		if err != nil {
			return bmc.LEDUnknown, err
		}
		return st.led(), nil
	}
	return bmc.LEDUnknown, s.unsupported("GetIndicatorLED")
}

// SetIndicatorLED implements bmc.IndicatorLED. AMI: POST actions/chassis-led
// {"uid_command": 0 off | 1 blink (timed, UNVERIFIED) | 2 on} (0/2
// live-confirmed on the X470, see chassisStatus.led).
func (s *Session) SetIndicatorLED(ctx context.Context, st bmc.LEDState) error {
	switch s.backend(bmc.CapIndicatorLED) {
	case bmc.BackendRedfish:
		return s.Session.SetIndicatorLED(ctx, st)
	case bmc.BackendAMI:
		var cmd int
		switch st {
		case bmc.LEDOff:
			cmd = 0
		case bmc.LEDBlinking:
			cmd = 1
		case bmc.LEDOn:
			cmd = 2
		default:
			return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetIndicatorLED", "unknown LED state %q (want on|off|blinking)", st)
		}
		return s.api.DoJSON(ctx, http.MethodPost, "actions/chassis-led", map[string]any{"uid_command": cmd}, nil)
	}
	return s.unsupported("SetIndicatorLED")
}

// GetBootOverride implements bmc.BootOverride: Redfish Boot.* (B650) or IPMI
// boot flags (X470; /api has no boot override).
func (s *Session) GetBootOverride(ctx context.Context) (bmc.BootOverrideConfig, error) {
	switch s.backend(bmc.CapBootOverride) {
	case bmc.BackendRedfish:
		return s.Session.GetBootOverride(ctx)
	case bmc.BackendIPMI:
		ic, err := s.ipmiClient(ctx)
		if err != nil {
			return bmc.BootOverrideConfig{}, err
		}
		return ic.GetBootOverride(ctx)
	}
	return bmc.BootOverrideConfig{}, s.unsupported("GetBootOverride")
}

// SetBootOverride implements bmc.BootOverride.
func (s *Session) SetBootOverride(ctx context.Context, b bmc.BootOverrideConfig) error {
	switch s.backend(bmc.CapBootOverride) {
	case bmc.BackendRedfish:
		return s.Session.SetBootOverride(ctx, b)
	case bmc.BackendIPMI:
		ic, err := s.ipmiClient(ctx)
		if err != nil {
			return err
		}
		return ic.SetBootOverride(ctx, b)
	}
	return s.unsupported("SetBootOverride")
}
