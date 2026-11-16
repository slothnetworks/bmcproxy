package v1

import (
	"encoding/json"

	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// PowerStateResponse is the body of GET /api/v1/power.
type PowerStateResponse struct {
	// Host power state.
	State bmc.PowerState `json:"state" example:"on" enums:"on,off,poweringOn,poweringOff,unknown"`
}

// PowerRequest is the body of POST /api/v1/power.
type PowerRequest struct {
	// Power action. "shutdown" is a graceful ACPI request the host may ignore.
	Action string `json:"action" example:"shutdown" enums:"on,off,shutdown,cycle,reset,nmi"`
	// Wait for the power state to converge (default true). Also ?wait=false.
	Wait *bool `json:"wait,omitempty" example:"true"`
	// Seconds to wait for convergence (default: server --power-poll-timeout, max 600).
	TimeoutSeconds *float64 `json:"timeoutSeconds,omitempty" example:"60"`
}

// RestorePolicyBody is GET/PUT /api/v1/power/restore-policy.
type RestorePolicyBody struct {
	// AC power-loss restore policy.
	Policy bmc.RestorePolicy `json:"policy" example:"lastState" enums:"alwaysOn,alwaysOff,lastState,unknown"`
}

// LEDBody is GET/POST /api/v1/led.
type LEDBody struct {
	// Chassis identify LED state.
	State bmc.LEDState `json:"state" example:"on" enums:"on,off,blinking,unknown"`
}

// SensorsResponse is GET /api/v1/sensors.
type SensorsResponse struct {
	// Normalized sensor readings; null readings/thresholds mean "not available".
	Sensors []bmc.Sensor `json:"sensors"`
}

// EventsResponse is GET /api/v1/events.
type EventsResponse struct {
	// SEL entries; each time carries the raw BMC time and the skew-corrected UTC time.
	Entries []bmc.SELEntry `json:"entries"`
	// Pass as ?since= to fetch only newer entries.
	LastID string `json:"lastId,omitempty" example:"42"`
	// More entries exist beyond ?limit=.
	More bool `json:"more,omitempty"`
	// Measured BMC clock offset (bmc - proxy) used for correction, if known.
	ClockOffsetSeconds *float64 `json:"clockOffsetSeconds,omitempty" example:"-3600"`
}

// AuditResponse is GET /api/v1/logs/audit.
type AuditResponse struct {
	// Audit entries with raw and skew-corrected times.
	Entries []bmc.AuditEntry `json:"entries"`
	// Measured BMC clock offset (bmc - proxy) used for correction, if known.
	ClockOffsetSeconds *float64 `json:"clockOffsetSeconds,omitempty" example:"-3600"`
}

// FRUResponse is GET /api/v1/fru.
type FRUResponse struct {
	// FRU devices.
	FRU []bmc.FRU `json:"fru"`
}

// InventoryResponse is GET /api/v1/inventory/{kind}. Exactly one of the
// collection fields is set, matching kind.
type InventoryResponse struct {
	// The requested inventory kind.
	Kind      bmc.InventoryKind   `json:"kind" example:"cpus" enums:"cpus,memory,pcie,storage,baseboard"`
	CPUs      []bmc.CPU           `json:"cpus,omitempty"`
	Memory    []bmc.MemoryModule  `json:"memory,omitempty"`
	PCIe      []bmc.PCIeDevice    `json:"pcie,omitempty"`
	Storage   []bmc.StorageDevice `json:"storage,omitempty"`
	Baseboard *bmc.Baseboard      `json:"baseboard,omitempty"`
}

// MarshalJSON always emits the collection named by Kind, as [] when it is
// empty (e.g. X470 with the host off), so clients never see {"kind":"cpus"}
// with the key missing. The other collections stay omitted.
func (r InventoryResponse) MarshalJSON() ([]byte, error) {
	type plain InventoryResponse // drops this method, avoiding recursion
	p := plain(r)
	switch r.Kind {
	case bmc.InventoryCPUs:
		if p.CPUs == nil {
			p.CPUs = []bmc.CPU{}
		}
		return json.Marshal(struct {
			plain
			CPUs []bmc.CPU `json:"cpus"`
		}{p, p.CPUs})
	case bmc.InventoryMemory:
		if p.Memory == nil {
			p.Memory = []bmc.MemoryModule{}
		}
		return json.Marshal(struct {
			plain
			Memory []bmc.MemoryModule `json:"memory"`
		}{p, p.Memory})
	case bmc.InventoryPCIe:
		if p.PCIe == nil {
			p.PCIe = []bmc.PCIeDevice{}
		}
		return json.Marshal(struct {
			plain
			PCIe []bmc.PCIeDevice `json:"pcie"`
		}{p, p.PCIe})
	case bmc.InventoryStorage:
		if p.Storage == nil {
			p.Storage = []bmc.StorageDevice{}
		}
		return json.Marshal(struct {
			plain
			Storage []bmc.StorageDevice `json:"storage"`
		}{p, p.Storage})
	}
	return json.Marshal(p)
}

// NetworkResponse is GET /api/v1/network.
type NetworkResponse struct {
	// BMC network interfaces.
	Interfaces []bmc.NetworkInterface `json:"interfaces"`
}

// UsersResponse is GET /api/v1/users.
type UsersResponse struct {
	// BMC accounts (never passwords).
	Users []bmc.User `json:"users"`
}

// ServicesResponse is GET /api/v1/services.
type ServicesResponse struct {
	// BMC network services.
	Services []bmc.Service `json:"services"`
}

// FirmwareResponse is GET /api/v1/firmware.
type FirmwareResponse struct {
	// Firmware components (BMC, BIOS, CPLD, ...).
	Components []bmc.FirmwareComponent `json:"components"`
}

// VirtualMediaResponse is GET /api/v1/virtual-media.
type VirtualMediaResponse struct {
	// Virtual media slots.
	Slots []bmc.VirtualMediaSlot `json:"slots"`
}

// BMCResetRequest is the (optional) body of POST /api/v1/bmc/reset.
type BMCResetRequest struct {
	// Reset kind (default cold).
	Kind bmc.BMCResetKind `json:"kind,omitempty" example:"cold" enums:"cold,warm"`
}

// BMCResetResponse acknowledges a BMC reset.
type BMCResetResponse struct {
	// The reset was accepted by the BMC; it is now rebooting.
	Accepted bool `json:"accepted" example:"true"`
	// Reset kind performed.
	Kind bmc.BMCResetKind `json:"kind" example:"cold"`
}

// CapabilitiesResponse is GET /api/v1/capabilities.
type CapabilitiesResponse struct {
	server.CapabilitiesReport
	// Measured BMC clock offset (bmc - proxy), if known.
	ClockOffsetSeconds *float64 `json:"clockOffsetSeconds,omitempty" example:"12.5"`
	// Warnings such as large clock skew.
	Warnings []string `json:"warnings,omitempty"`
}
