package ipmi

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// NetFn/Cmd pairs used in logs and hooks (IPMI v2.0 Appendix G).
const (
	netfnChassis = 0x00
	netfnSensor  = 0x04
	netfnApp     = 0x06
	netfnStorage = 0x0A
	netfnTrans   = 0x0C
)

// Compile-time checks: the Client can back the IPMI-capable capabilities
// directly.
var (
	_ bmc.PowerController    = (*Client)(nil)
	_ bmc.PowerRestorePolicy = (*Client)(nil)
	_ bmc.IndicatorLED       = (*Client)(nil)
	_ bmc.BootOverride       = (*Client)(nil)
	_ bmc.SensorReader       = (*Client)(nil)
	_ bmc.EventLog           = (*Client)(nil)
	_ bmc.EventLogClearer    = (*Client)(nil)
	_ bmc.FRUReader          = (*Client)(nil)
	_ bmc.UserReader         = (*Client)(nil)
	_ bmc.BMCReset           = (*Client)(nil)
)

// ChassisStatus returns the raw chassis status (chassis status).
func (c *Client) ChassisStatus(ctx context.Context) (ChassisStatus, error) {
	var st ChassisStatus
	err := c.exec(ctx, "GetChassisStatus", netfnChassis, 0x01, func(ctx context.Context) error {
		var err error
		st, err = c.conn.ChassisStatus(ctx)
		return err
	})
	return st, err
}

// PowerState implements bmc.PowerController (power status).
func (c *Client) PowerState(ctx context.Context) (bmc.PowerState, error) {
	st, err := c.ChassisStatus(ctx)
	if err != nil {
		return bmc.PowerUnknown, err
	}
	if st.PowerOn {
		return bmc.PowerOn, nil
	}
	return bmc.PowerOff, nil
}

// ControlFor maps a power action to the Chassis Control value.
func ControlFor(a bmc.PowerAction) (ChassisControl, error) {
	switch a {
	case bmc.ActionOn:
		return ControlPowerUp, nil
	case bmc.ActionOff:
		return ControlPowerDown, nil
	case bmc.ActionShutdown:
		return ControlSoftOff, nil
	case bmc.ActionCycle:
		return ControlPowerCycle, nil
	case bmc.ActionReset:
		return ControlHardReset, nil
	case bmc.ActionNMI:
		return ControlNMI, nil
	}
	return 0, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "Power", "unknown action %q", a)
}

// Power implements bmc.PowerController (power on/off/soft/cycle/reset). A
// soft-off may be ignored by the host; callers poll PowerState.
func (c *Client) Power(ctx context.Context, a bmc.PowerAction) error {
	ctl, err := ControlFor(a)
	if err != nil {
		return err
	}
	return c.exec(ctx, "ChassisControl."+string(a), netfnChassis, 0x02, func(ctx context.Context) error {
		return c.conn.ChassisControl(ctx, ctl)
	})
}

// GetPowerRestorePolicy implements bmc.PowerRestorePolicy.
func (c *Client) GetPowerRestorePolicy(ctx context.Context) (bmc.RestorePolicy, error) {
	st, err := c.ChassisStatus(ctx)
	if err != nil {
		return bmc.RestoreUnknown, err
	}
	switch st.RestorePolicy {
	case RestoreAlwaysOff:
		return bmc.RestoreAlwaysOff, nil
	case RestorePrevious:
		return bmc.RestoreLastState, nil
	case RestoreAlwaysOn:
		return bmc.RestoreAlwaysOn, nil
	}
	return bmc.RestoreUnknown, nil
}

// SetPowerRestorePolicy implements bmc.PowerRestorePolicy (chassis policy).
func (c *Client) SetPowerRestorePolicy(ctx context.Context, p bmc.RestorePolicy) error {
	var code RestorePolicyCode
	switch p {
	case bmc.RestoreAlwaysOff:
		code = RestoreAlwaysOff
	case bmc.RestoreLastState:
		code = RestorePrevious
	case bmc.RestoreAlwaysOn:
		code = RestoreAlwaysOn
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetPowerRestorePolicy", "unknown policy %q", p)
	}
	return c.exec(ctx, "SetPowerRestorePolicy", netfnChassis, 0x06, func(ctx context.Context) error {
		return c.conn.SetPowerRestorePolicy(ctx, code)
	})
}

// GetIndicatorLED implements bmc.IndicatorLED from the chassis identify
// state (unknown when the BMC doesn't report it).
func (c *Client) GetIndicatorLED(ctx context.Context) (bmc.LEDState, error) {
	st, err := c.ChassisStatus(ctx)
	if err != nil {
		return bmc.LEDUnknown, err
	}
	if !st.IdentifySupported {
		return bmc.LEDUnknown, nil
	}
	switch st.IdentifyState {
	case 0:
		return bmc.LEDOff, nil
	case 1:
		return bmc.LEDBlinking, nil
	case 2:
		return bmc.LEDOn, nil
	}
	return bmc.LEDUnknown, nil
}

// SetIndicatorLED implements bmc.IndicatorLED (chassis identify): on ->
// force on indefinitely, blinking -> identify for 15 s, off -> interval 0.
func (c *Client) SetIndicatorLED(ctx context.Context, s bmc.LEDState) error {
	var interval uint8
	var force bool
	switch s {
	case bmc.LEDOn:
		force = true
	case bmc.LEDBlinking:
		interval = 15
	case bmc.LEDOff:
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetIndicatorLED", "unknown LED state %q", s)
	}
	return c.Identify(ctx, interval, force)
}

// Identify sends Chassis Identify (interval seconds, or force on).
func (c *Client) Identify(ctx context.Context, interval uint8, force bool) error {
	return c.exec(ctx, "ChassisIdentify", netfnChassis, 0x04, func(ctx context.Context) error {
		return c.conn.ChassisIdentify(ctx, interval, force)
	})
}

// BootTargetToDevice maps a bmc.BootTarget to the boot device selector.
func BootTargetToDevice(t bmc.BootTarget) (BootDevice, error) {
	switch t {
	case bmc.BootNone, "":
		return BootNoOverride, nil
	case bmc.BootPXE:
		return BootPXE, nil
	case bmc.BootDisk:
		return BootDisk, nil
	case bmc.BootCD:
		return BootCDROM, nil
	case bmc.BootBIOS:
		return BootBIOS, nil
	case bmc.BootFloppy:
		return BootFloppy, nil
	case bmc.BootUSB:
		// IPMI has no USB selector; remote media is the closest.
		return BootRemoteDisk, nil
	}
	return 0, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetBootOverride", "unknown boot target %q", t)
}

// BootDeviceToTarget maps a boot device selector to a bmc.BootTarget.
func BootDeviceToTarget(d BootDevice) bmc.BootTarget {
	switch d {
	case BootPXE:
		return bmc.BootPXE
	case BootDisk, BootSafeDisk, BootRemoteDisk:
		return bmc.BootDisk
	case BootCDROM, BootRemoteCD:
		return bmc.BootCD
	case BootBIOS:
		return bmc.BootBIOS
	case BootFloppy, BootRemoteFlop:
		return bmc.BootFloppy
	}
	return bmc.BootNone
}

// GetBootOverride implements bmc.BootOverride (bootdev get / boot flags).
func (c *Client) GetBootOverride(ctx context.Context) (bmc.BootOverrideConfig, error) {
	var f BootFlags
	err := c.exec(ctx, "GetBootFlags", netfnChassis, 0x09, func(ctx context.Context) error {
		var err error
		f, err = c.conn.BootFlags(ctx)
		return err
	})
	if err != nil {
		return bmc.BootOverrideConfig{}, err
	}
	cfg := bmc.BootOverrideConfig{Target: BootDeviceToTarget(f.Device), Mode: bmc.BootDisabled, BootMode: "legacy"}
	if f.EFI {
		cfg.BootMode = "uefi"
	}
	if f.Valid && f.Device != BootNoOverride {
		cfg.Mode = bmc.BootOnce
		if f.Persist {
			cfg.Mode = bmc.BootContinuous
		}
	}
	return cfg, nil
}

// SetBootOverride implements bmc.BootOverride (bootdev set). Mode
// disabled clears the override.
func (c *Client) SetBootOverride(ctx context.Context, b bmc.BootOverrideConfig) error {
	dev, err := BootTargetToDevice(b.Target)
	if err != nil {
		return err
	}
	persist := false
	switch b.Mode {
	case bmc.BootOnce, "":
	case bmc.BootContinuous:
		persist = true
	case bmc.BootDisabled:
		dev = BootNoOverride
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetBootOverride", "unknown mode %q", b.Mode)
	}
	efi := strings.EqualFold(b.BootMode, "uefi")
	return c.exec(ctx, "SetBootDevice", netfnChassis, 0x08, func(ctx context.Context) error {
		return c.conn.SetBootDevice(ctx, dev, efi, persist)
	})
}

// ---- sensors ----

// SensorUnit normalises an IPMI unit string.
func SensorUnit(u string) string {
	switch strings.ToLower(strings.TrimSpace(u)) {
	case "degrees c":
		return "C"
	case "volts":
		return "V"
	case "rpm":
		return "RPM"
	case "amps":
		return "A"
	case "watts":
		return "W"
	case "percent", "% unspecified":
		return "%"
	case "discrete", "unspecified", "":
		return ""
	}
	return u
}

// SensorFromRecord normalises an SDR record (sdr elist row).
func SensorFromRecord(r SensorRecord) bmc.Sensor {
	num := int(r.Number)
	unit := SensorUnit(r.Unit)
	s := bmc.Sensor{ID: strconv.Itoa(num), Number: &num, Name: r.Name, Unit: unit, Backend: bmc.BackendIPMI,
		Thresholds: bmc.Thresholds{LowerNonRecoverable: r.LNR, LowerCritical: r.LCR, LowerNonCritical: r.LNC,
			UpperNonCritical: r.UNC, UpperCritical: r.UCR, UpperNonRecoverable: r.UNR}}
	tn := strings.ToLower(r.TypeName)
	switch {
	case !r.Threshold:
		s.Type = bmc.SensorDiscrete
	case strings.Contains(tn, "temp") || unit == "C":
		s.Type = bmc.SensorTemperature
	case strings.Contains(tn, "voltage") || unit == "V":
		s.Type = bmc.SensorVoltage
	case strings.Contains(tn, "fan") || unit == "RPM":
		s.Type = bmc.SensorFan
	case strings.Contains(tn, "current") || unit == "A":
		s.Type = bmc.SensorCurrent
	case unit == "W":
		s.Type = bmc.SensorPower
	default:
		s.Type = bmc.SensorOther
	}
	if r.Present && r.Threshold {
		v := r.Value
		s.Reading = &v
	}
	s.State = r.Status
	if len(r.States) > 0 {
		s.State = strings.Join(r.States, ", ")
	}
	switch strings.ToLower(r.Status) {
	case "ok":
		s.Health = bmc.HealthOK
	case "lnc", "unc":
		s.Health = bmc.HealthWarning
	case "lcr", "ucr", "lnr", "unr":
		s.Health = bmc.HealthCritical
	default:
		s.Health = bmc.HealthUnknown
	}
	if !r.Present {
		s.Health, s.State = bmc.HealthUnknown, "unavailable"
	}
	if !r.Threshold {
		s.Oem = map[string]any{"sensorType": r.TypeName}
	}
	return s
}

// Sensors implements bmc.SensorReader (sdr elist).
func (c *Client) Sensors(ctx context.Context) ([]bmc.Sensor, error) {
	var recs []SensorRecord
	err := c.exec(ctx, "SDRList", netfnStorage, 0x23, func(ctx context.Context) error {
		var err error
		recs, err = c.conn.Sensors(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]bmc.Sensor, 0, len(recs))
	for _, r := range recs {
		out = append(out, SensorFromRecord(r))
	}
	return out, nil
}

// ---- SEL ----

// SELFromRecord normalises a SEL record.
func (c *Client) SELFromRecord(r SELRecord) bmc.SELEntry {
	c.mu.Lock()
	off := c.opts.ClockOffset
	c.mu.Unlock()
	num := int(r.SensorNumber)
	e := bmc.SELEntry{ID: strconv.Itoa(int(r.ID)), Time: bmc.NewTimestamp(r.Time, off), SensorType: r.SensorType,
		SensorNumber: &num, EventType: r.EventType, Message: r.Description, Backend: bmc.BackendIPMI,
		EventData: hex.EncodeToString(r.EventData[:]), Generator: fmt.Sprintf("0x%04x", r.GeneratorID)}
	if r.OEM {
		e.SensorNumber = nil
		e.Oem = map[string]any{"recordType": fmt.Sprintf("0x%02x", r.RecordType)}
	}
	if r.Deassertion {
		e.Direction = "deassertion"
	} else if !r.OEM {
		e.Direction = "assertion"
	}
	switch strings.ToLower(r.Severity) {
	case "ok", "info":
		e.Severity = bmc.HealthOK
	case "warning", "degraded", "non-fatal":
		e.Severity = bmc.HealthWarning
	case "critical":
		e.Severity = bmc.HealthCritical
	default:
		e.Severity = bmc.HealthUnknown
	}
	if e.Message == "" {
		e.Message = strings.TrimSpace(r.SensorType + " " + r.EventType)
	}
	return e
}

// Events implements bmc.EventLog (sel list) with record id > SinceID.
func (c *Client) Events(ctx context.Context, q bmc.EventQuery) (bmc.EventPage, error) {
	var since int64 = -1
	if q.SinceID != "" {
		n, err := strconv.ParseInt(q.SinceID, 0, 32)
		if err != nil {
			return bmc.EventPage{}, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "Events", "sinceId %q is not a SEL record id", q.SinceID)
		}
		since = n
	}
	var recs []SELRecord
	err := c.exec(ctx, "SELList", netfnStorage, 0x43, func(ctx context.Context) error {
		var err error
		recs, err = c.conn.SEL(ctx)
		return err
	})
	if err != nil {
		return bmc.EventPage{}, err
	}
	slices.SortStableFunc(recs, func(a, b SELRecord) int { return int(a.ID) - int(b.ID) })
	page := bmc.EventPage{Entries: []bmc.SELEntry{}, LastID: q.SinceID}
	for _, r := range recs {
		if int64(r.ID) <= since {
			continue
		}
		if q.Limit > 0 && len(page.Entries) >= q.Limit {
			page.More = true
			break
		}
		e := c.SELFromRecord(r)
		page.Entries = append(page.Entries, e)
		page.LastID = e.ID
	}
	return page, nil
}

// ClearEvents implements bmc.EventLogClearer (sel clear).
func (c *Client) ClearEvents(ctx context.Context) error {
	return c.exec(ctx, "ClearSEL", netfnStorage, 0x47, func(ctx context.Context) error { return c.conn.ClearSEL(ctx) })
}

// FRU implements bmc.FRUReader (fru print).
func (c *Client) FRU(ctx context.Context) ([]bmc.FRU, error) {
	var out []bmc.FRU
	err := c.exec(ctx, "FRURead", netfnStorage, 0x11, func(ctx context.Context) error {
		var err error
		out, err = c.conn.FRUs(ctx)
		return err
	})
	return out, err
}

// MCInfo returns Get Device ID (mc info).
func (c *Client) MCInfo(ctx context.Context) (DeviceInfo, error) {
	var d DeviceInfo
	err := c.exec(ctx, "GetDeviceID", netfnApp, 0x01, func(ctx context.Context) error {
		var err error
		d, err = c.conn.DeviceID(ctx)
		return err
	})
	return d, err
}

// ResetBMC implements bmc.BMCReset (mc reset cold|warm).
func (c *Client) ResetBMC(ctx context.Context, kind bmc.BMCResetKind) error {
	switch kind {
	case bmc.BMCResetCold, "":
		return c.exec(ctx, "ColdReset", netfnApp, 0x02, func(ctx context.Context) error { return c.conn.ColdReset(ctx) })
	case bmc.BMCResetWarm:
		return c.exec(ctx, "WarmReset", netfnApp, 0x03, func(ctx context.Context) error { return c.conn.WarmReset(ctx) })
	}
	return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "ResetBMC", "unknown reset kind %q", kind)
}

// LANConfig returns the LAN parameters of the configured channel (lan print).
func (c *Client) LANConfig(ctx context.Context) (LANConfig, error) {
	var lc LANConfig
	err := c.exec(ctx, "GetLanConfig", netfnTrans, 0x02, func(ctx context.Context) error {
		var err error
		lc, err = c.conn.LANConfig(ctx, c.opts.Channel)
		return err
	})
	return lc, err
}

// NetworkInterface normalises lan print into a bmc.NetworkInterface.
func (c *Client) NetworkInterface(ctx context.Context) (bmc.NetworkInterface, error) {
	lc, err := c.LANConfig(ctx)
	if err != nil {
		return bmc.NetworkInterface{}, err
	}
	return NetworkFromLAN(lc), nil
}

// NetworkFromLAN normalises a LANConfig.
func NetworkFromLAN(lc LANConfig) bmc.NetworkInterface {
	ch := strconv.Itoa(int(lc.Channel))
	n := bmc.NetworkInterface{ID: "lan" + ch, Name: "lan" + ch, Enabled: true,
		VLAN: bmc.VLANConfig{Enabled: lc.VLANOn, ID: int(lc.VLANID)}}
	if lc.MAC != nil {
		n.MAC = strings.ToLower(lc.MAC.String())
	}
	if lc.IP != nil {
		n.IPv4.Address = lc.IP.String()
	}
	if lc.Netmask != nil {
		n.IPv4.Netmask = lc.Netmask.String()
	}
	if lc.Gateway != nil && !lc.Gateway.IsUnspecified() {
		n.IPv4.Gateway = lc.Gateway.String()
	}
	n.IPv4.DHCP = lc.IPSource == "dhcp"
	n.Oem = map[string]any{"ipSource": lc.IPSource, "channel": int(lc.Channel)}
	return n
}

// Users implements bmc.UserReader (user list on the configured channel).
// Empty slots are skipped.
func (c *Client) Users(ctx context.Context) ([]bmc.User, error) {
	var recs []UserRecord
	err := c.exec(ctx, "GetUsers", netfnApp, 0x46, func(ctx context.Context) error {
		var err error
		recs, err = c.conn.Users(ctx, c.opts.Channel)
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]bmc.User, 0, len(recs))
	for _, r := range recs {
		if strings.TrimSpace(r.Name) == "" {
			continue
		}
		out = append(out, bmc.User{ID: strconv.Itoa(int(r.ID)), Name: r.Name, Enabled: r.Enabled, Privilege: PrivilegeFrom(r.Privilege),
			Oem: map[string]any{"callin": r.Callin, "linkAuth": r.LinkAuth, "ipmiMessaging": r.IPMIMsg}})
	}
	return out, nil
}

// PrivilegeFrom maps an IPMI privilege name.
func PrivilegeFrom(p string) bmc.Privilege {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "ADMINISTRATOR", "ADMIN":
		return bmc.PrivAdministrator
	case "OPERATOR":
		return bmc.PrivOperator
	case "USER":
		return bmc.PrivUser
	case "CALLBACK":
		return bmc.PrivCallback
	case "OEM", "OEM PROPRIETARY":
		return bmc.PrivOEM
	}
	return bmc.PrivNoAccess
}

// sensor reading command constant kept for hook consumers.
var _ = netfnSensor
