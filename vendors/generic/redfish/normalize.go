package genericredfish

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
)

// Exported normalisers from Redfish resources to pkg/bmc types. Vendor
// drivers that read Redfish themselves can reuse them.

// PowerState maps a Redfish PowerState.
func PowerState(s string) bmc.PowerState {
	switch lower(s) {
	case "on":
		return bmc.PowerOn
	case "off":
		return bmc.PowerOff
	case "poweringon":
		return bmc.PowerPoweringOn
	case "poweringoff":
		return bmc.PowerPoweringOff
	}
	return bmc.PowerUnknown
}

// Health maps a Redfish Status to bmc.Health. Absent/disabled resources
// without a Health value are unknown.
func Health(st rf.Status) bmc.Health { return HealthString(st.Health) }

// HealthString maps a Redfish Health/Severity string.
func HealthString(h string) bmc.Health {
	switch lower(h) {
	case "ok":
		return bmc.HealthOK
	case "warning":
		return bmc.HealthWarning
	case "critical":
		return bmc.HealthCritical
	}
	return bmc.HealthUnknown
}

func present(st rf.Status) bool {
	s := lower(st.State)
	return s != "absent" && s != "disabled" && s != "unavailableoffline"
}

// ---- sensors ----

// SensorType maps Sensor.ReadingType.
func SensorType(readingType string) bmc.SensorType {
	switch lower(readingType) {
	case "temperature":
		return bmc.SensorTemperature
	case "voltage":
		return bmc.SensorVoltage
	case "rotational":
		return bmc.SensorFan
	case "current":
		return bmc.SensorCurrent
	case "power", "energyjoules", "energykwh", "energywh":
		return bmc.SensorPower
	case "":
		return bmc.SensorOther
	}
	return bmc.SensorOther
}

// Unit normalises a ReadingUnits value (UCUM), falling back to the unit
// implied by the sensor type when ReadingUnits is null (B650 sends null
// units even for voltages).
func Unit(units string, t bmc.SensorType, readingType string) string {
	switch strings.TrimSpace(units) {
	case "Cel", "C", "°C", "deg_c":
		return "C"
	case "V", "Volts":
		return "V"
	case "RPM", "{rev}/min", "1/min":
		return "RPM"
	case "A", "Amps":
		return "A"
	case "W", "Watts":
		return "W"
	case "%":
		return "%"
	case "":
	default:
		return units
	}
	switch t {
	case bmc.SensorTemperature:
		return "C"
	case bmc.SensorVoltage:
		return "V"
	case bmc.SensorFan:
		return "RPM"
	case bmc.SensorCurrent:
		return "A"
	case bmc.SensorPower:
		return "W"
	}
	if lower(readingType) == "percent" {
		return "%"
	}
	return ""
}

func thr(t *rf.Threshold) *float64 {
	if t == nil {
		return nil
	}
	return t.Reading
}

// SensorFromRedfish normalises a #Sensor resource.
func SensorFromRedfish(s rf.Sensor) bmc.Sensor {
	t := SensorType(s.ReadingType)
	out := bmc.Sensor{
		ID: s.ID, Name: s.Name, Type: t, Reading: s.Reading, Unit: Unit(s.ReadingUnits, t, s.ReadingType),
		Health: Health(s.Status), State: s.Status.State, Number: s.SensorNumber, Backend: bmc.BackendRedfish,
		Thresholds: bmc.Thresholds{
			LowerNonRecoverable: thr(s.Thresholds.LowerFatal),
			LowerCritical:       thr(s.Thresholds.LowerCritical),
			LowerNonCritical:    thr(s.Thresholds.LowerCaution),
			UpperNonCritical:    thr(s.Thresholds.UpperCaution),
			UpperCritical:       thr(s.Thresholds.UpperCritical),
			UpperNonRecoverable: thr(s.Thresholds.UpperFatal),
		},
	}
	if out.ID == "" {
		out.ID = s.Name
	}
	if !present(s.Status) {
		out.Reading = nil
	}
	if s.PhysicalContext != "" {
		out.Oem = map[string]any{"physicalContext": s.PhysicalContext}
	}
	return out
}

func legacySensor(id, name string, num *int, reading *float64, t bmc.SensorType, unit string, st rf.Status,
	lnr, lcr, lnc, unc, ucr, unr *float64) bmc.Sensor {
	s := bmc.Sensor{ID: id, Name: name, Number: num, Type: t, Reading: reading, Unit: unit, Health: Health(st), State: st.State,
		Backend: bmc.BackendRedfish, Thresholds: bmc.Thresholds{LowerNonRecoverable: lnr, LowerCritical: lcr, LowerNonCritical: lnc,
			UpperNonCritical: unc, UpperCritical: ucr, UpperNonRecoverable: unr}}
	if !present(st) {
		s.Reading = nil
	}
	return s
}

// SensorsFromThermal normalises the legacy Thermal resource.
func SensorsFromThermal(th *rf.Thermal) []bmc.Sensor {
	var out []bmc.Sensor
	if th == nil {
		return out
	}
	for _, t := range th.Temperatures {
		out = append(out, legacySensor("thermal.temp."+t.MemberID, t.Name, t.SensorNumber, t.ReadingCelsius, bmc.SensorTemperature, "C", t.Status,
			t.LowerThresholdFatal, t.LowerThresholdCritical, t.LowerThresholdNonCritical, t.UpperThresholdNonCritical, t.UpperThresholdCritical, t.UpperThresholdFatal))
	}
	for _, f := range th.Fans {
		unit := "RPM"
		if f.ReadingUnits == "Percent" {
			unit = "%"
		}
		out = append(out, legacySensor("thermal.fan."+f.MemberID, f.Name, f.SensorNumber, f.Reading, bmc.SensorFan, unit, f.Status,
			f.LowerThresholdFatal, f.LowerThresholdCritical, f.LowerThresholdNonCritical, f.UpperThresholdNonCritical, f.UpperThresholdCritical, f.UpperThresholdFatal))
	}
	return out
}

// SensorsFromPower normalises the legacy Power resource.
func SensorsFromPower(p *rf.Power) []bmc.Sensor {
	var out []bmc.Sensor
	if p == nil {
		return out
	}
	for _, v := range p.Voltages {
		out = append(out, legacySensor("power.volt."+v.MemberID, v.Name, v.SensorNumber, v.ReadingVolts, bmc.SensorVoltage, "V", v.Status,
			v.LowerThresholdFatal, v.LowerThresholdCritical, v.LowerThresholdNonCritical, v.UpperThresholdNonCritical, v.UpperThresholdCritical, v.UpperThresholdFatal))
	}
	for _, pc := range p.PowerControl {
		out = append(out, legacySensor("power.control."+pc.MemberID, pc.Name, nil, pc.PowerConsumedWatts, bmc.SensorPower, "W", pc.Status,
			nil, nil, nil, nil, nil, nil))
	}
	return out
}

// ---- logs ----

// LogTime parses Created (else EventTimestamp).
func LogTime(e rf.LogEntry) time.Time {
	for _, s := range []string{e.Created, e.EventTimestamp} {
		if t, err := rf.ParseTime(s); err == nil && !t.IsZero() {
			return t
		}
	}
	return time.Time{}
}

// SELFromLogEntry normalises a SEL LogEntry. offset is bmc - proxy.
func SELFromLogEntry(e rf.LogEntry, offset time.Duration) bmc.SELEntry {
	out := bmc.SELEntry{ID: e.ID, Time: bmc.NewTimestamp(LogTime(e), offset), Severity: HealthString(e.Severity),
		SensorType: e.SensorType, SensorNumber: e.SensorNumber, EventType: e.EventType, Message: e.Message,
		Generator: e.GeneratorID, Backend: bmc.BackendRedfish}
	switch lower(e.EntryCode) {
	case "assert":
		out.Direction = "assertion"
	case "deassert":
		out.Direction = "deassertion"
	}
	if out.EventType == "" {
		out.EventType = e.EntryCode
	}
	oem := map[string]any{}
	if e.MessageID != "" {
		oem["messageId"] = e.MessageID
	}
	if len(e.MessageArgs) > 0 {
		oem["messageArgs"] = e.MessageArgs
	}
	if e.OriginOfCondition.ID != "" {
		oem["originOfCondition"] = e.OriginOfCondition.ID
	}
	if e.EventTimestamp != "" && e.EventTimestamp != e.Created {
		oem["eventTimestamp"] = e.EventTimestamp
	}
	if len(oem) > 0 {
		out.Oem = oem
	}
	return out
}

// AuditFromLogEntry normalises an audit LogEntry.
func AuditFromLogEntry(e rf.LogEntry, offset time.Duration) bmc.AuditEntry {
	return bmc.AuditEntry{ID: e.ID, Time: bmc.NewTimestamp(LogTime(e), offset), Severity: HealthString(e.Severity),
		Message: e.Message, Facility: e.EntryType}
}

// idAfter reports whether id sorts after since (numeric when both are).
func idAfter(id, since string) bool {
	if since == "" {
		return true
	}
	a, errA := strconv.ParseInt(id, 10, 64)
	b, errB := strconv.ParseInt(since, 10, 64)
	if errA == nil && errB == nil {
		return a > b
	}
	return id > since
}

func idLess(a, b string) bool {
	x, errA := strconv.ParseInt(a, 10, 64)
	y, errB := strconv.ParseInt(b, 10, 64)
	if errA == nil && errB == nil {
		return x < y
	}
	return a < b
}

// ---- power / boot / LED ----

// resetPreferences lists ResetType candidates per action, best first.
var resetPreferences = map[bmc.PowerAction][]string{
	bmc.ActionOn:       {"On", "ForceOn", "PushPowerButton"},
	bmc.ActionOff:      {"ForceOff"},
	bmc.ActionShutdown: {"GracefulShutdown"},
	bmc.ActionCycle:    {"PowerCycle"},
	bmc.ActionReset:    {"ForceRestart", "GracefulRestart"},
	bmc.ActionNMI:      {"Nmi"},
}

// ResetTypeFor picks the ResetType for a power action given the service's
// allowable values (nil = unknown: the first preference is used). It
// returns ErrUnsupported when the service allows none of the candidates.
func ResetTypeFor(a bmc.PowerAction, allowable []string) (string, error) {
	prefs, ok := resetPreferences[a]
	if !ok {
		return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "Power", "unknown action %q", a)
	}
	if len(allowable) == 0 {
		return prefs[0], nil
	}
	for _, p := range prefs {
		for _, v := range allowable {
			if v == p {
				return p, nil
			}
		}
	}
	return "", bmc.Errorf(bmc.ErrUnsupported, Pkg, "Power", "action %q not allowed (ResetType allowable: %s)", a, strings.Join(allowable, ","))
}

// BootTargetFromRedfish maps BootSourceOverrideTarget.
func BootTargetFromRedfish(s string) bmc.BootTarget {
	switch s {
	case "Pxe":
		return bmc.BootPXE
	case "Hdd", "RemoteDrive":
		return bmc.BootDisk
	case "Cd":
		return bmc.BootCD
	case "Usb":
		return bmc.BootUSB
	case "BiosSetup":
		return bmc.BootBIOS
	case "Floppy":
		return bmc.BootFloppy
	}
	return bmc.BootNone
}

// BootTargetToRedfish maps a bmc.BootTarget.
func BootTargetToRedfish(t bmc.BootTarget) (string, error) {
	switch t {
	case bmc.BootNone, "":
		return "None", nil
	case bmc.BootPXE:
		return "Pxe", nil
	case bmc.BootDisk:
		return "Hdd", nil
	case bmc.BootCD:
		return "Cd", nil
	case bmc.BootUSB:
		return "Usb", nil
	case bmc.BootBIOS:
		return "BiosSetup", nil
	case bmc.BootFloppy:
		return "Floppy", nil
	}
	return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetBootOverride", "unknown boot target %q", t)
}

// BootModeFromRedfish maps BootSourceOverrideEnabled.
func BootModeFromRedfish(s string) bmc.BootOverrideMode {
	switch s {
	case "Once":
		return bmc.BootOnce
	case "Continuous":
		return bmc.BootContinuous
	}
	return bmc.BootDisabled
}

// LEDFromRedfish maps IndicatorLED / LocationIndicatorActive.
func LEDFromRedfish(led *string, active *bool) bmc.LEDState {
	if led != nil {
		switch lower(*led) {
		case "lit":
			return bmc.LEDOn
		case "blinking":
			return bmc.LEDBlinking
		case "off":
			return bmc.LEDOff
		}
	}
	if active != nil {
		if *active {
			return bmc.LEDOn
		}
		return bmc.LEDOff
	}
	return bmc.LEDUnknown
}

// RestorePolicyFromRedfish maps PowerRestorePolicy.
func RestorePolicyFromRedfish(s string) bmc.RestorePolicy {
	switch s {
	case "AlwaysOn":
		return bmc.RestoreAlwaysOn
	case "AlwaysOff":
		return bmc.RestoreAlwaysOff
	case "LastState":
		return bmc.RestoreLastState
	}
	return bmc.RestoreUnknown
}

// RestorePolicyToRedfish maps a bmc.RestorePolicy.
func RestorePolicyToRedfish(p bmc.RestorePolicy) (string, error) {
	switch p {
	case bmc.RestoreAlwaysOn:
		return "AlwaysOn", nil
	case bmc.RestoreAlwaysOff:
		return "AlwaysOff", nil
	case bmc.RestoreLastState:
		return "LastState", nil
	}
	return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetPowerRestorePolicy", "unknown policy %q", p)
}

// ---- inventory ----

func ival(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func fval(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// CPUFromProcessor normalises a Processor.
func CPUFromProcessor(p rf.Processor) bmc.CPU {
	return bmc.CPU{ID: p.ID, Socket: p.Socket, Manufacturer: p.Manufacturer, Model: p.Model, Cores: ival(p.TotalCores),
		Threads: ival(p.TotalThreads), MaxSpeedMHz: int(fval(p.MaxSpeedMHz)), Present: present(p.Status), Health: Health(p.Status)}
}

// MemoryFromRedfish normalises a Memory (DIMM) resource.
func MemoryFromRedfish(m rf.Memory) bmc.MemoryModule {
	slot := m.DeviceLocator
	if slot == "" {
		slot = m.Name
	}
	return bmc.MemoryModule{ID: m.ID, Slot: slot, Manufacturer: m.Manufacturer, PartNumber: strings.TrimSpace(m.PartNumber),
		Serial: m.SerialNumber, SizeMiB: int(fval(m.CapacityMiB)), SpeedMTs: int(fval(m.OperatingSpeedMhz)),
		Type: m.MemoryDeviceType, Present: present(m.Status) && fval(m.CapacityMiB) > 0, Health: Health(m.Status)}
}

// PCIeFromRedfish normalises a PCIeDevice (plus its first function, if any).
func PCIeFromRedfish(d rf.PCIeDevice, fn *rf.PCIeFunction) bmc.PCIeDevice {
	out := bmc.PCIeDevice{ID: d.ID, Name: d.Name, Manufacturer: d.Manufacturer, Slot: d.Slot.Location.PartLocation.ServiceLabel,
		LinkSpeed: d.PCIeInterface.PCIeType, Health: Health(d.Status)}
	if d.PCIeInterface.LanesInUse != nil {
		out.LinkWidth = fmt.Sprintf("x%d", *d.PCIeInterface.LanesInUse)
	}
	if out.Name == "" {
		out.Name = d.Model
	}
	if fn != nil {
		out.VendorID, out.DeviceID, out.SubsystemVendorID, out.SubsystemID, out.ClassCode =
			fn.VendorID, fn.DeviceID, fn.SubsystemVendorID, fn.SubsystemID, fn.ClassCode
	}
	return out
}

// StorageFromDrive normalises a Drive.
func StorageFromDrive(d rf.Drive) bmc.StorageDevice {
	return bmc.StorageDevice{ID: d.ID, Name: d.Name, Manufacturer: d.Manufacturer, Model: d.Model, Serial: d.SerialNumber,
		Firmware: d.Revision, CapacityBytes: int64(fval(d.CapacityBytes)), MediaType: lower(d.MediaType), Protocol: lower(d.Protocol),
		Health: Health(d.Status)}
}

// ---- network / users / services ----

// NetworkFromEthernet normalises an EthernetInterface.
func NetworkFromEthernet(e rf.EthernetInterface) bmc.NetworkInterface {
	out := bmc.NetworkInterface{ID: e.ID, Name: e.ID, MAC: strings.ToLower(e.MACAddress), Hostname: e.HostName,
		SpeedMbs: ival(e.SpeedMbps)}
	if e.Name != "" && out.Name == "" {
		out.Name = e.Name
	}
	out.Enabled = e.InterfaceEnabled == nil || *e.InterfaceEnabled
	switch e.LinkStatus {
	case "LinkUp":
		t := true
		out.LinkUp = &t
	case "LinkDown", "NoLink":
		f := false
		out.LinkUp = &f
	}
	if e.DHCPv4 != nil && e.DHCPv4.DHCPEnabled != nil {
		out.IPv4.DHCP = *e.DHCPv4.DHCPEnabled
	}
	for _, a := range e.IPv4Addresses {
		if a.Address != "" {
			out.IPv4.Address, out.IPv4.Netmask, out.IPv4.Gateway = a.Address, a.SubnetMask, a.Gateway
			if lower(a.AddressOrigin) == "dhcp" {
				out.IPv4.DHCP = true
			}
			break
		}
	}
	for _, a := range e.IPv6Addresses {
		if a.Address == "" {
			continue
		}
		out.IPv6.Enabled = true
		addr := a.Address
		if a.PrefixLength != nil {
			addr = fmt.Sprintf("%s/%d", a.Address, *a.PrefixLength)
		}
		out.IPv6.Addresses = append(out.IPv6.Addresses, addr)
		if lower(a.AddressOrigin) == "dhcpv6" {
			out.IPv6.DHCP = true
		}
	}
	out.IPv6.Gateway = e.IPv6DefaultGateway
	if e.VLAN != nil {
		out.VLAN.Enabled = e.VLAN.VLANEnable != nil && *e.VLAN.VLANEnable
		out.VLAN.ID = ival(e.VLAN.VLANID)
	}
	for _, ns := range e.StaticNameServers {
		if ns != nil && *ns != "" && *ns != "0.0.0.0" {
			out.DNS = append(out.DNS, *ns)
		}
	}
	if len(out.DNS) == 0 {
		for _, ns := range e.NameServers {
			if ns != "" && ns != "0.0.0.0" {
				out.DNS = append(out.DNS, ns)
			}
		}
	}
	return out
}

// PrivilegeFromRole maps a Redfish RoleId.
func PrivilegeFromRole(role string) bmc.Privilege {
	switch lower(role) {
	case "administrator":
		return bmc.PrivAdministrator
	case "operator":
		return bmc.PrivOperator
	case "readonly", "readonlyuser":
		return bmc.PrivUser
	case "noaccess", "":
		return bmc.PrivNoAccess
	}
	return bmc.PrivOEM
}

// UserFromAccount normalises a ManagerAccount (never includes a password).
func UserFromAccount(a rf.ManagerAccount) bmc.User {
	u := bmc.User{ID: a.ID, Name: a.UserName, Enabled: a.Enabled == nil || *a.Enabled, Privilege: PrivilegeFromRole(a.RoleID),
		Locked: a.Locked != nil && *a.Locked}
	if u.Privilege == bmc.PrivOEM {
		u.Oem = map[string]any{"roleId": a.RoleID}
	}
	return u
}

// ServicesFromNetworkProtocol normalises NetworkProtocol entries (sorted by
// name).
func ServicesFromNetworkProtocol(np *rf.NetworkProtocol) []bmc.Service {
	var out []bmc.Service
	if np == nil {
		return out
	}
	names := make([]string, 0, len(np.Services))
	for k := range np.Services {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		ps := np.Services[k]
		svc := bmc.Service{Name: lower(k), Enabled: ps.ProtocolEnabled != nil && *ps.ProtocolEnabled, Port: ival(ps.Port)}
		if k == "HTTPS" {
			svc.SecurePort = svc.Port
		}
		out = append(out, svc)
	}
	return out
}

// FirmwareFromSoftware normalises a SoftwareInventory member.
func FirmwareFromSoftware(s rf.SoftwareInventory) bmc.FirmwareComponent {
	name := s.Name
	if name == "" {
		name = s.ID
	}
	return bmc.FirmwareComponent{ID: s.ID, Name: name, Version: s.Version, Updateable: s.Updateable != nil && *s.Updateable,
		ReleaseDate: s.ReleaseDate, Health: Health(s.Status)}
}

// VirtualMediaFromRedfish normalises a VirtualMedia slot.
func VirtualMediaFromRedfish(v rf.VirtualMedia) bmc.VirtualMediaSlot {
	out := bmc.VirtualMediaSlot{ID: v.ID, Image: v.Image, Inserted: v.Inserted != nil && *v.Inserted,
		WriteProtected: v.WriteProtected != nil && *v.WriteProtected, ConnectedVia: v.ConnectedVia}
	seen := map[string]bool{}
	for _, mt := range v.MediaTypes {
		var n string
		switch mt {
		case "CD", "DVD":
			n = "cd"
		case "USBStick":
			n = "usb"
		case "Floppy":
			n = "floppy"
		default:
			n = lower(mt)
		}
		if !seen[n] {
			seen[n] = true
			out.MediaTypes = append(out.MediaTypes, n)
		}
	}
	return out
}
