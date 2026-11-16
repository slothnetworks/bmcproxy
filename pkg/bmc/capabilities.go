package bmc

import (
	"context"
	"time"
)

// Session is an authenticated connection to one BMC, returned by
// Driver.Open. Sessions are cached and shared by concurrent northbound
// requests, so implementations must be safe for concurrent use.
//
// A Session additionally implements any subset of the capability interfaces
// below. The API layer type-asserts; a missing interface is HTTP 501. A
// Session may implement an interface but still return ErrUnsupported at
// runtime (e.g. the board profile lacks the endpoint) — that is also 501.
type Session interface {
	// Close logs out of every backend (AMI DELETE /api/session, Redfish
	// session DELETE, IPMI close) and releases resources. It is called on
	// cache eviction and shutdown and must be idempotent.
	Close(ctx context.Context) error
	// Info describes the session for /capabilities, /debug/* and /status.
	Info() SessionInfo
}

// Backend names used in SessionInfo and OpError.Backend.
const (
	BackendRedfish = "redfish"
	BackendAMI     = "ami" // AMI MegaRAC /api
	BackendIPMI    = "ipmi"
)

// BackendStatus reports one southbound protocol's state for a session.
type BackendStatus struct {
	Name          string `json:"name"`          // BackendRedfish, BackendAMI, BackendIPMI, ...
	Available     bool   `json:"available"`     // reachable
	Authenticated bool   `json:"authenticated"` // creds accepted
	Detail        string `json:"detail,omitempty"`
}

// SessionInfo describes an open session. It never contains secrets.
type SessionInfo struct {
	Driver   string    `json:"driver"`
	Target   string    `json:"target"` // Target.Addr()
	User     string    `json:"user,omitempty"`
	Board    string    `json:"board,omitempty"`
	Firmware string    `json:"firmware,omitempty"`
	OpenedAt time.Time `json:"openedAt"`
	// ClockOffset is bmcTime - proxyTime measured at open (0 if unknown).
	ClockOffset time.Duration   `json:"clockOffset"`
	Backends    []BackendStatus `json:"backends,omitempty"`
	// Capabilities maps each supported capability to the backend serving
	// it. A capability whose interface the session implements but which is
	// absent here (or maps to "") is reported as unsupported. If the map is
	// nil, implemented interfaces are assumed supported with an unknown
	// backend.
	Capabilities map[Capability]string `json:"capabilities,omitempty"`
	// Degraded is a non-empty reason when the session works in a reduced
	// mode (e.g. Redfish auth failed, fell back to /api).
	Degraded string `json:"degraded,omitempty"`
}

// Capability names a generic operation group (one per interface).
type Capability string

const (
	CapIdentity           Capability = "identity"
	CapPower              Capability = "power"
	CapPowerRestorePolicy Capability = "powerRestorePolicy"
	CapIndicatorLED       Capability = "indicatorLed"
	CapBootOverride       Capability = "bootOverride"
	CapSensors            Capability = "sensors"
	CapEventLog           Capability = "eventLog"
	CapEventLogClear      Capability = "eventLogClear"
	CapAuditLog           Capability = "auditLog"
	CapFRU                Capability = "fru"
	CapInventory          Capability = "inventory"
	CapNetwork            Capability = "network"
	CapBond               Capability = "bond"
	CapUsers              Capability = "users"
	CapServices           Capability = "services"
	CapTime               Capability = "time"
	CapFirmwareInventory  Capability = "firmwareInventory"
	CapFirmwareUpdate     Capability = "firmwareUpdate"
	CapVirtualMedia       Capability = "virtualMedia"
	CapKVM                Capability = "kvm"
	CapBMCReset           Capability = "bmcReset"
)

// ---- capability interfaces ----

// Identity: GET /system.
type Identity interface {
	SystemInfo(ctx context.Context) (SystemInfo, error)
}

// PowerController: GET/POST /power. Power must not assume a graceful
// shutdown took effect; callers poll PowerState.
type PowerController interface {
	PowerState(ctx context.Context) (PowerState, error)
	Power(ctx context.Context, a PowerAction) error
}

// PowerRestorePolicy: GET/PUT /power/restore-policy.
type PowerRestorePolicy interface {
	GetPowerRestorePolicy(ctx context.Context) (RestorePolicy, error)
	SetPowerRestorePolicy(ctx context.Context, p RestorePolicy) error
}

// IndicatorLED: POST /led.
type IndicatorLED interface {
	GetIndicatorLED(ctx context.Context) (LEDState, error)
	SetIndicatorLED(ctx context.Context, s LEDState) error
}

// BootOverride: GET/PUT /boot.
type BootOverride interface {
	GetBootOverride(ctx context.Context) (BootOverrideConfig, error)
	SetBootOverride(ctx context.Context, b BootOverrideConfig) error
}

// SensorReader: GET /sensors.
type SensorReader interface {
	Sensors(ctx context.Context) ([]Sensor, error)
}

// EventLog: GET /events?since= (incremental by record id).
type EventLog interface {
	Events(ctx context.Context, q EventQuery) (EventPage, error)
}

// EventLogClearer: DELETE /events (disruptive).
type EventLogClearer interface {
	ClearEvents(ctx context.Context) error
}

// AuditLog: GET /logs/audit.
type AuditLog interface {
	AuditLog(ctx context.Context, q AuditQuery) ([]AuditEntry, error)
}

// FRUReader: GET /fru.
type FRUReader interface {
	FRU(ctx context.Context) ([]FRU, error)
}

// Inventory: GET /inventory/{cpus,memory,pcie,storage,baseboard}. Methods a
// board cannot serve return ErrUnsupported.
type Inventory interface {
	CPUs(ctx context.Context) ([]CPU, error)
	Memory(ctx context.Context) ([]MemoryModule, error)
	PCIeDevices(ctx context.Context) ([]PCIeDevice, error)
	Storage(ctx context.Context) ([]StorageDevice, error)
	Baseboard(ctx context.Context) (Baseboard, error)
}

// NetworkManager: GET /network (read), PUT /network (disruptive).
type NetworkManager interface {
	NetworkInterfaces(ctx context.Context) ([]NetworkInterface, error)
	SetNetworkInterface(ctx context.Context, u NetworkInterfaceUpdate) error
}

// BondManager: BMC NIC bonding.
type BondManager interface {
	Bond(ctx context.Context) (BondConfig, error)
	SetBond(ctx context.Context, b BondConfig) error
}

// UserReader: GET /users.
type UserReader interface {
	Users(ctx context.Context) ([]User, error)
}

// ServiceReader: GET /services.
type ServiceReader interface {
	Services(ctx context.Context) ([]Service, error)
}

// TimeManager: GET /time, PUT /time, POST /time/sync. SyncTime sets the BMC
// clock to the proxy's current UTC (compensating for the measured offset)
// and returns the resulting config.
type TimeManager interface {
	Time(ctx context.Context) (TimeConfig, error)
	SetTime(ctx context.Context, u TimeUpdate) error
	SyncTime(ctx context.Context) (TimeConfig, error)
}

// FirmwareInventory: GET /firmware.
type FirmwareInventory interface {
	Firmware(ctx context.Context) ([]FirmwareComponent, error)
}

// FirmwareUpdater: POST /firmware/update (disruptive).
type FirmwareUpdater interface {
	UpdateFirmware(ctx context.Context, r FirmwareUpdateRequest) (Task, error)
	FirmwareTask(ctx context.Context, id string) (Task, error)
}

// VirtualMedia: GET /virtual-media, POST .../insert, .../eject.
type VirtualMedia interface {
	VirtualMedia(ctx context.Context) ([]VirtualMediaSlot, error)
	InsertMedia(ctx context.Context, r VirtualMediaInsert) error
	EjectMedia(ctx context.Context, slotID string) error
}

// KVMLauncher: GET /kvm.
type KVMLauncher interface {
	KVMLaunch(ctx context.Context) (KVMLaunchInfo, error)
}

// BMCReset: POST /bmc/reset (disruptive).
type BMCReset interface {
	ResetBMC(ctx context.Context, kind BMCResetKind) error
}

// ---- reporting ----

// AllCapabilities lists every capability in display order.
func AllCapabilities() []Capability {
	return []Capability{CapIdentity, CapPower, CapPowerRestorePolicy, CapIndicatorLED, CapBootOverride,
		CapSensors, CapEventLog, CapEventLogClear, CapAuditLog, CapFRU, CapInventory, CapNetwork, CapBond,
		CapUsers, CapServices, CapTime, CapFirmwareInventory, CapFirmwareUpdate, CapVirtualMedia, CapKVM, CapBMCReset}
}

// Implements reports whether s implements the interface for c.
func Implements(s Session, c Capability) bool {
	var ok bool
	switch c {
	case CapIdentity:
		_, ok = s.(Identity)
	case CapPower:
		_, ok = s.(PowerController)
	case CapPowerRestorePolicy:
		_, ok = s.(PowerRestorePolicy)
	case CapIndicatorLED:
		_, ok = s.(IndicatorLED)
	case CapBootOverride:
		_, ok = s.(BootOverride)
	case CapSensors:
		_, ok = s.(SensorReader)
	case CapEventLog:
		_, ok = s.(EventLog)
	case CapEventLogClear:
		_, ok = s.(EventLogClearer)
	case CapAuditLog:
		_, ok = s.(AuditLog)
	case CapFRU:
		_, ok = s.(FRUReader)
	case CapInventory:
		_, ok = s.(Inventory)
	case CapNetwork:
		_, ok = s.(NetworkManager)
	case CapBond:
		_, ok = s.(BondManager)
	case CapUsers:
		_, ok = s.(UserReader)
	case CapServices:
		_, ok = s.(ServiceReader)
	case CapTime:
		_, ok = s.(TimeManager)
	case CapFirmwareInventory:
		_, ok = s.(FirmwareInventory)
	case CapFirmwareUpdate:
		_, ok = s.(FirmwareUpdater)
	case CapVirtualMedia:
		_, ok = s.(VirtualMedia)
	case CapKVM:
		_, ok = s.(KVMLauncher)
	case CapBMCReset:
		_, ok = s.(BMCReset)
	}
	return ok
}

// CapabilityReport is one row of GET /capabilities.
type CapabilityReport struct {
	Capability Capability `json:"capability"`
	Supported  bool       `json:"supported"`
	Backend    string     `json:"backend,omitempty"`
}

// Describe merges interface implementation with SessionInfo.Capabilities
// into a full report over AllCapabilities.
func Describe(s Session) []CapabilityReport {
	info := s.Info()
	out := make([]CapabilityReport, 0, len(AllCapabilities()))
	for _, c := range AllCapabilities() {
		r := CapabilityReport{Capability: c}
		if Implements(s, c) {
			if info.Capabilities == nil {
				r.Supported = true
			} else if b, ok := info.Capabilities[c]; ok && b != "" {
				r.Supported, r.Backend = true, b
			}
		}
		out = append(out, r)
	}
	return out
}
