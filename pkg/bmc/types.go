package bmc

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Normalized models returned by capability interfaces and JSON-encoded by
// the northbound API. JSON field names are lowerCamelCase. Optional numeric
// values are pointers (nil = unknown / not reported, e.g. an AMI "NA"
// threshold). Vendor-specific leftovers go in Oem maps.

// ---- power ----

// PowerState is the host (not BMC) power state.
type PowerState string

const (
	PowerOn          PowerState = "on"
	PowerOff         PowerState = "off"
	PowerPoweringOn  PowerState = "poweringOn"
	PowerPoweringOff PowerState = "poweringOff"
	PowerUnknown     PowerState = "unknown"
)

// PowerAction is a northbound power request
// (POST /power {"action": ...}). AMI codes: on=1 off=0 shutdown=5 cycle=2 reset=3.
type PowerAction string

const (
	ActionOn       PowerAction = "on"       // power on
	ActionOff      PowerAction = "off"      // hard (immediate) off
	ActionShutdown PowerAction = "shutdown" // graceful (ACPI) shutdown; may be ignored mid-POST — poll state
	ActionCycle    PowerAction = "cycle"    // off then on
	ActionReset    PowerAction = "reset"    // hard reset
	ActionNMI      PowerAction = "nmi"      // diagnostic interrupt (optional)
)

// ParsePowerAction validates s (case-insensitive).
func ParsePowerAction(s string) (PowerAction, error) {
	a := PowerAction(strings.ToLower(strings.TrimSpace(s)))
	switch a {
	case ActionOn, ActionOff, ActionShutdown, ActionCycle, ActionReset, ActionNMI:
		return a, nil
	}
	return "", fmt.Errorf("%w: unknown power action %q (want on|off|shutdown|cycle|reset|nmi)", ErrInvalidArgument, s)
}

// RestorePolicy is the AC power-loss restore policy.
type RestorePolicy string

const (
	RestoreAlwaysOn  RestorePolicy = "alwaysOn"
	RestoreAlwaysOff RestorePolicy = "alwaysOff"
	RestoreLastState RestorePolicy = "lastState"
	RestoreUnknown   RestorePolicy = "unknown"
)

// ParseRestorePolicy validates s.
func ParseRestorePolicy(s string) (RestorePolicy, error) {
	for _, p := range []RestorePolicy{RestoreAlwaysOn, RestoreAlwaysOff, RestoreLastState} {
		if strings.EqualFold(s, string(p)) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: unknown restore policy %q (want alwaysOn|alwaysOff|lastState)", ErrInvalidArgument, s)
}

// LEDState is the chassis identify LED state.
type LEDState string

const (
	LEDOn       LEDState = "on"
	LEDOff      LEDState = "off"
	LEDBlinking LEDState = "blinking"
	LEDUnknown  LEDState = "unknown"
)

// ---- boot override ----

// BootTarget is a one-shot / persistent boot device override.
type BootTarget string

const (
	BootNone   BootTarget = "none"
	BootPXE    BootTarget = "pxe"
	BootDisk   BootTarget = "disk"
	BootCD     BootTarget = "cd"
	BootUSB    BootTarget = "usb"
	BootBIOS   BootTarget = "bios" // enter firmware setup
	BootFloppy BootTarget = "floppy"
)

// BootOverrideMode is how long the override applies.
type BootOverrideMode string

const (
	BootOnce       BootOverrideMode = "once"
	BootContinuous BootOverrideMode = "continuous"
	BootDisabled   BootOverrideMode = "disabled"
)

// BootOverrideConfig is GET/PUT /boot.
type BootOverrideConfig struct {
	Target BootTarget       `json:"target"`
	Mode   BootOverrideMode `json:"mode"`
	// BootMode is "uefi", "legacy" or "" (unchanged / unknown).
	BootMode string `json:"bootMode,omitempty"`
}

// ---- identity ----

// SystemInfo is GET /system.
type SystemInfo struct {
	Vendor       string         `json:"vendor"` // driver name
	Manufacturer string         `json:"manufacturer,omitempty"`
	Board        string         `json:"board,omitempty"`
	Model        string         `json:"model,omitempty"`
	Serial       string         `json:"serial,omitempty"`
	UUID         string         `json:"uuid,omitempty"`
	BMCFirmware  string         `json:"bmcFirmware,omitempty"`
	BIOSVersion  string         `json:"biosVersion,omitempty"`
	BMCMAC       string         `json:"bmcMac,omitempty"`
	Hostname     string         `json:"hostname,omitempty"`
	PowerState   PowerState     `json:"powerState,omitempty"`
	Health       Health         `json:"health,omitempty"`
	Oem          map[string]any `json:"oem,omitempty"`
}

// Health is a normalized health/status value.
type Health string

const (
	HealthOK       Health = "ok"
	HealthWarning  Health = "warning"
	HealthCritical Health = "critical"
	HealthUnknown  Health = "unknown"
)

// ---- sensors ----

// SensorType is the normalized sensor class.
type SensorType string

const (
	SensorTemperature SensorType = "temperature"
	SensorVoltage     SensorType = "voltage"
	SensorFan         SensorType = "fan"
	SensorCurrent     SensorType = "current"
	SensorPower       SensorType = "power"
	SensorDiscrete    SensorType = "discrete"
	SensorOther       SensorType = "other"
)

// Thresholds are nullable: nil means not set / "NA".
type Thresholds struct {
	LowerNonRecoverable *float64 `json:"lowerNonRecoverable"`
	LowerCritical       *float64 `json:"lowerCritical"`
	LowerNonCritical    *float64 `json:"lowerNonCritical"`
	UpperNonCritical    *float64 `json:"upperNonCritical"`
	UpperCritical       *float64 `json:"upperCritical"`
	UpperNonRecoverable *float64 `json:"upperNonRecoverable"`
}

// Sensor is one entry of GET /sensors.
type Sensor struct {
	ID         string         `json:"id"`
	Number     *int           `json:"number,omitempty"` // IPMI sensor number
	Name       string         `json:"name"`
	Type       SensorType     `json:"type"`
	Reading    *float64       `json:"reading"`        // nil when unavailable / not present
	Unit       string         `json:"unit,omitempty"` // normalized: "C", "V", "RPM", "A", "W", "%"
	Health     Health         `json:"health"`
	State      string         `json:"state,omitempty"` // discrete/raw state text
	Thresholds Thresholds     `json:"thresholds"`
	Settable   bool           `json:"settable,omitempty"` // thresholds writable (AMI settable_flag)
	Backend    string         `json:"backend,omitempty"`
	Oem        map[string]any `json:"oem,omitempty"`
}

// F64 returns a pointer to v (helper for building optional readings).
func F64(v float64) *float64 { return &v }

// ---- event log / audit log ----

// Timestamp carries both the BMC-reported time and the clock-skew-corrected
// UTC time. Corrected = Raw - offset, where offset = bmcTime - proxyTime
// measured at session open.
type Timestamp struct {
	Raw       time.Time `json:"raw"`
	Corrected time.Time `json:"corrected"`
}

// NewTimestamp builds a Timestamp from a raw BMC time and the measured
// offset (bmc - proxy).
func NewTimestamp(raw time.Time, offset time.Duration) Timestamp {
	if raw.IsZero() {
		return Timestamp{}
	}
	return Timestamp{Raw: raw, Corrected: raw.Add(-offset).UTC()}
}

// SELEntry is one system event log record.
type SELEntry struct {
	ID           string         `json:"id"` // record id as string (AMI int, Redfish Id)
	Time         Timestamp      `json:"time"`
	Severity     Health         `json:"severity,omitempty"`
	SensorType   string         `json:"sensorType,omitempty"`
	SensorName   string         `json:"sensorName,omitempty"`
	SensorNumber *int           `json:"sensorNumber,omitempty"`
	EventType    string         `json:"eventType,omitempty"`
	Direction    string         `json:"direction,omitempty"` // "assertion" | "deassertion"
	Message      string         `json:"message"`
	EventData    string         `json:"eventData,omitempty"` // raw event data bytes (hex)
	Generator    string         `json:"generator,omitempty"`
	Backend      string         `json:"backend,omitempty"`
	Oem          map[string]any `json:"oem,omitempty"`
}

// EventQuery selects SEL entries incrementally.
type EventQuery struct {
	// SinceID returns only entries after this id ("" = from the start).
	// AMI maps it to LASTEVENTID=n.
	SinceID string `json:"sinceId,omitempty"`
	// Limit caps the number of entries (0 = driver default / all).
	Limit int `json:"limit,omitempty"`
}

// EventPage is the result of EventLog.Events.
type EventPage struct {
	Entries []SELEntry `json:"entries"`
	// LastID is the id to pass as SinceID next time ("" if no entries and
	// none known).
	LastID string `json:"lastId,omitempty"`
	// More reports that Limit truncated the result.
	More bool `json:"more,omitempty"`
}

// AuditQuery selects audit entries.
type AuditQuery struct {
	SinceID string `json:"sinceId,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// AuditEntry is one BMC audit/security log record.
type AuditEntry struct {
	ID       string         `json:"id"`
	Time     Timestamp      `json:"time"`
	User     string         `json:"user,omitempty"`
	Source   string         `json:"source,omitempty"` // client IP / interface
	Facility string         `json:"facility,omitempty"`
	Severity Health         `json:"severity,omitempty"`
	Message  string         `json:"message"`
	Oem      map[string]any `json:"oem,omitempty"`
}

// ---- FRU ----

// FRU is one FRU device.
type FRU struct {
	ID      int        `json:"id"`
	Name    string     `json:"name,omitempty"`
	Chassis FRUChassis `json:"chassis"`
	Board   FRUBoard   `json:"board"`
	Product FRUProduct `json:"product"`
}

type FRUChassis struct {
	Type       string   `json:"type,omitempty"`
	PartNumber string   `json:"partNumber,omitempty"`
	Serial     string   `json:"serial,omitempty"`
	Extra      []string `json:"extra,omitempty"`
}

type FRUBoard struct {
	MfgDate      *time.Time `json:"mfgDate,omitempty"`
	Manufacturer string     `json:"manufacturer,omitempty"`
	Product      string     `json:"product,omitempty"`
	Serial       string     `json:"serial,omitempty"`
	PartNumber   string     `json:"partNumber,omitempty"`
	FRUFileID    string     `json:"fruFileId,omitempty"`
	Extra        []string   `json:"extra,omitempty"`
}

type FRUProduct struct {
	Manufacturer string   `json:"manufacturer,omitempty"`
	Name         string   `json:"name,omitempty"`
	PartNumber   string   `json:"partNumber,omitempty"`
	Version      string   `json:"version,omitempty"`
	Serial       string   `json:"serial,omitempty"`
	AssetTag     string   `json:"assetTag,omitempty"`
	FRUFileID    string   `json:"fruFileId,omitempty"`
	Extra        []string `json:"extra,omitempty"`
}

// ---- inventory ----

// InventoryKind names the /inventory/{kind} collections.
type InventoryKind string

const (
	InventoryCPUs      InventoryKind = "cpus"
	InventoryMemory    InventoryKind = "memory"
	InventoryPCIe      InventoryKind = "pcie"
	InventoryStorage   InventoryKind = "storage"
	InventoryBaseboard InventoryKind = "baseboard"
)

type CPU struct {
	ID           string         `json:"id"`
	Socket       string         `json:"socket,omitempty"`
	Manufacturer string         `json:"manufacturer,omitempty"`
	Model        string         `json:"model,omitempty"`
	Cores        int            `json:"cores,omitempty"`
	Threads      int            `json:"threads,omitempty"`
	MaxSpeedMHz  int            `json:"maxSpeedMHz,omitempty"`
	Present      bool           `json:"present"`
	Health       Health         `json:"health,omitempty"`
	Oem          map[string]any `json:"oem,omitempty"`
}

type MemoryModule struct {
	ID           string         `json:"id"`
	Slot         string         `json:"slot,omitempty"`
	Manufacturer string         `json:"manufacturer,omitempty"`
	PartNumber   string         `json:"partNumber,omitempty"`
	Serial       string         `json:"serial,omitempty"`
	SizeMiB      int            `json:"sizeMiB,omitempty"`
	SpeedMTs     int            `json:"speedMTs,omitempty"`
	Type         string         `json:"type,omitempty"` // "DDR4", "DDR5"
	Present      bool           `json:"present"`
	Health       Health         `json:"health,omitempty"`
	Oem          map[string]any `json:"oem,omitempty"`
}

type PCIeDevice struct {
	ID                string         `json:"id"`
	Slot              string         `json:"slot,omitempty"`
	Name              string         `json:"name,omitempty"`
	Manufacturer      string         `json:"manufacturer,omitempty"`
	VendorID          string         `json:"vendorId,omitempty"` // hex "0x8086"
	DeviceID          string         `json:"deviceId,omitempty"`
	SubsystemVendorID string         `json:"subsystemVendorId,omitempty"`
	SubsystemID       string         `json:"subsystemId,omitempty"`
	ClassCode         string         `json:"classCode,omitempty"`
	LinkWidth         string         `json:"linkWidth,omitempty"`
	LinkSpeed         string         `json:"linkSpeed,omitempty"`
	Health            Health         `json:"health,omitempty"`
	Oem               map[string]any `json:"oem,omitempty"`
}

type StorageDevice struct {
	ID            string         `json:"id"`
	Name          string         `json:"name,omitempty"`
	Manufacturer  string         `json:"manufacturer,omitempty"`
	Model         string         `json:"model,omitempty"`
	Serial        string         `json:"serial,omitempty"`
	Firmware      string         `json:"firmware,omitempty"`
	CapacityBytes int64          `json:"capacityBytes,omitempty"`
	MediaType     string         `json:"mediaType,omitempty"` // "ssd", "hdd"
	Protocol      string         `json:"protocol,omitempty"`  // "nvme", "sata", "sas"
	Health        Health         `json:"health,omitempty"`
	Oem           map[string]any `json:"oem,omitempty"`
}

type Baseboard struct {
	Manufacturer string         `json:"manufacturer,omitempty"`
	Product      string         `json:"product,omitempty"`
	Serial       string         `json:"serial,omitempty"`
	PartNumber   string         `json:"partNumber,omitempty"`
	Version      string         `json:"version,omitempty"`
	BIOSVendor   string         `json:"biosVendor,omitempty"`
	BIOSVersion  string         `json:"biosVersion,omitempty"`
	BIOSDate     string         `json:"biosDate,omitempty"`
	Oem          map[string]any `json:"oem,omitempty"`
}

// ---- network ----

type IPv4Config struct {
	DHCP    bool   `json:"dhcp"`
	Address string `json:"address,omitempty"`
	Netmask string `json:"netmask,omitempty"`
	Gateway string `json:"gateway,omitempty"`
}

type IPv6Config struct {
	Enabled   bool     `json:"enabled"`
	DHCP      bool     `json:"dhcp"`
	Addresses []string `json:"addresses,omitempty"` // CIDR form
	Gateway   string   `json:"gateway,omitempty"`
}

type VLANConfig struct {
	Enabled bool `json:"enabled"`
	ID      int  `json:"id,omitempty"`
}

// NetworkInterface is one BMC network interface (GET /network).
type NetworkInterface struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"` // "eth0", "bond0"
	MAC      string         `json:"mac,omitempty"`
	Enabled  bool           `json:"enabled"`
	LinkUp   *bool          `json:"linkUp,omitempty"`
	SpeedMbs int            `json:"speedMbps,omitempty"`
	IPv4     IPv4Config     `json:"ipv4"`
	IPv6     IPv6Config     `json:"ipv6"`
	VLAN     VLANConfig     `json:"vlan"`
	DNS      []string       `json:"dns,omitempty"`
	Hostname string         `json:"hostname,omitempty"`
	Oem      map[string]any `json:"oem,omitempty"`
}

// NetworkInterfaceUpdate is PUT /network. Nil fields are left unchanged; the
// driver reads the full object, mutates, and writes it back.
type NetworkInterfaceUpdate struct {
	Name    string      `json:"name"` // required: which interface
	Enabled *bool       `json:"enabled,omitempty"`
	IPv4    *IPv4Config `json:"ipv4,omitempty"`
	IPv6    *IPv6Config `json:"ipv6,omitempty"`
	VLAN    *VLANConfig `json:"vlan,omitempty"`
	DNS     []string    `json:"dns,omitempty"`
}

// BondConfig is the BMC NIC bonding configuration.
type BondConfig struct {
	ID         int            `json:"id,omitempty"`
	Enabled    bool           `json:"enabled"`
	Mode       string         `json:"mode,omitempty"` // "active-backup", "balance-rr", ... or vendor number as string
	Interfaces []string       `json:"interfaces,omitempty"`
	AutoConfig bool           `json:"autoConfig"`
	Oem        map[string]any `json:"oem,omitempty"`
}

// ---- users / services ----

// Privilege is the normalized account role.
type Privilege string

const (
	PrivAdministrator Privilege = "administrator"
	PrivOperator      Privilege = "operator"
	PrivUser          Privilege = "user"
	PrivCallback      Privilege = "callback"
	PrivNoAccess      Privilege = "noAccess"
	PrivOEM           Privilege = "oem"
)

// User is a BMC account (never includes passwords).
type User struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Enabled   bool           `json:"enabled"`
	Privilege Privilege      `json:"privilege,omitempty"`
	Locked    bool           `json:"locked,omitempty"`
	Email     string         `json:"email,omitempty"`
	Oem       map[string]any `json:"oem,omitempty"`
}

// Service is a BMC network service (web, kvm, ssh, ...).
type Service struct {
	Name           string         `json:"name"`
	Enabled        bool           `json:"enabled"`
	Port           int            `json:"port,omitempty"`
	SecurePort     int            `json:"securePort,omitempty"`
	TimeoutSeconds int            `json:"timeoutSeconds,omitempty"`
	MaxSessions    int            `json:"maxSessions,omitempty"`
	ActiveSessions int            `json:"activeSessions,omitempty"`
	Interfaces     []string       `json:"interfaces,omitempty"`
	Oem            map[string]any `json:"oem,omitempty"`
}

// ---- time ----

// TimeConfig is GET /time.
type TimeConfig struct {
	BMCTime          time.Time `json:"bmcTime"`
	ProxyTime        time.Time `json:"proxyTime"`
	OffsetSeconds    float64   `json:"offsetSeconds"` // bmc - proxy
	Timezone         string    `json:"timezone,omitempty"`
	UTCOffsetMinutes int       `json:"utcOffsetMinutes"`
	NTPEnabled       bool      `json:"ntpEnabled"`
	NTPServers       []string  `json:"ntpServers,omitempty"`
}

// Offset returns OffsetSeconds as a Duration.
func (t TimeConfig) Offset() time.Duration {
	return time.Duration(t.OffsetSeconds * float64(time.Second))
}

// TimeUpdate is PUT /time. Nil/empty fields are unchanged. Drivers must
// read-modify-write the full vendor object (X470 500s on partial bodies) and
// preserve timezone/utc offset.
type TimeUpdate struct {
	Time       *time.Time `json:"time,omitempty"`       // manual clock set (UTC)
	NTPEnabled *bool      `json:"ntpEnabled,omitempty"` // enable/disable NTP
	NTPServers []string   `json:"ntpServers,omitempty"` // primary, secondary, ...
}

// ---- firmware ----

// FirmwareComponent is one entry of GET /firmware.
type FirmwareComponent struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"` // "BMC", "BIOS", "CPLD", ...
	Version     string         `json:"version"`
	Updateable  bool           `json:"updateable"`
	ReleaseDate string         `json:"releaseDate,omitempty"`
	Health      Health         `json:"health,omitempty"`
	Oem         map[string]any `json:"oem,omitempty"`
}

// FirmwareUpdateRequest starts an update.
type FirmwareUpdateRequest struct {
	Component string            `json:"component"`
	ImageURI  string            `json:"imageUri,omitempty"` // pull update (Redfish SimpleUpdate)
	Image     io.Reader         `json:"-"`                  // push update body
	ImageSize int64             `json:"imageSize,omitempty"`
	ImageName string            `json:"imageName,omitempty"`
	Options   map[string]string `json:"options,omitempty"` // e.g. preserve settings
}

// TaskState is a long-running operation state.
type TaskState string

const (
	TaskPending   TaskState = "pending"
	TaskRunning   TaskState = "running"
	TaskCompleted TaskState = "completed"
	TaskFailed    TaskState = "failed"
)

// Task tracks a long-running BMC operation.
type Task struct {
	ID              string    `json:"id"`
	State           TaskState `json:"state"`
	PercentComplete *int      `json:"percentComplete,omitempty"`
	Message         string    `json:"message,omitempty"`
	StartTime       time.Time `json:"startTime,omitzero"`
	EndTime         time.Time `json:"endTime,omitzero"`
}

// ---- virtual media / KVM / BMC reset ----

type VirtualMediaSlot struct {
	ID             string   `json:"id"`
	MediaTypes     []string `json:"mediaTypes,omitempty"` // "cd", "usb", "floppy"
	Inserted       bool     `json:"inserted"`
	Image          string   `json:"image,omitempty"`
	WriteProtected bool     `json:"writeProtected"`
	ConnectedVia   string   `json:"connectedVia,omitempty"`
}

// VirtualMediaInsert is POST /virtual-media/{id}/insert. Password is
// redacted by the logging layer (key "password").
type VirtualMediaInsert struct {
	SlotID           string `json:"slotId"`
	Image            string `json:"image"` // URL: nfs://, cifs://, http(s)://
	TransferProtocol string `json:"transferProtocol,omitempty"`
	UserName         string `json:"userName,omitempty"`
	Password         string `json:"password,omitempty"`
	WriteProtected   bool   `json:"writeProtected"`
}

// KVMLaunchInfo is GET /kvm.
type KVMLaunchInfo struct {
	Kind    string            `json:"kind"` // "html5", "java", "websocket"
	URL     string            `json:"url,omitempty"`
	Token   string            `json:"token,omitempty"`
	Expires time.Time         `json:"expires,omitzero"`
	Extra   map[string]string `json:"extra,omitempty"`
}

// BMCResetKind selects the BMC reset type.
type BMCResetKind string

const (
	BMCResetCold BMCResetKind = "cold"
	BMCResetWarm BMCResetKind = "warm"
)
