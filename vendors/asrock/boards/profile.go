// Package boards holds the ASRock Rack board+firmware profiles: which
// southbound protocol to prefer, the AMI /api dialect (endpoint and field
// names), the TLS generation, extra deny rules and per-endpoint timeouts.
//
// Profiles are keyed off the LIVE-reported BMC firmware (asrr/fw-info
// BMC_fw_version), never the dumped image version: every board in the fleet
// runs a different version than its image (X470 live 2.32 vs image 3.02,
// B650 live 4.10 vs images 7.02/9.05 vs HAR 1.10). An unknown board or
// version gets a profile whose dialect fields say "probe", and the driver
// falls back to capability probing (try the newer endpoint, fall back to the
// older one on 404/405, remember the answer per session).
package boards

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// RedfishMode says whether the driver should try Redfish for this profile.
type RedfishMode int

const (
	// RedfishNo: do not use Redfish (X470 2.32: admin gets 401 AccessDenied;
	// /api + IPMI instead).
	RedfishNo RedfishMode = iota
	// RedfishYes: Redfish-first; /api for OEM features and gaps (B650 4.10).
	RedfishYes
	// RedfishProbe: try Redfish Basic auth at Open; use it if it works,
	// otherwise /api (unknown boards/versions).
	RedfishProbe
)

func (m RedfishMode) String() string {
	switch m {
	case RedfishNo:
		return "no"
	case RedfishYes:
		return "yes"
	case RedfishProbe:
		return "probe"
	}
	return "RedfishMode(" + strconv.Itoa(int(m)) + ")"
}

// FanDialect selects the fan-control endpoints.
type FanDialect int

const (
	// FanProbe: try settings/fan/* first, fall back to asrr/settings/*.
	FanProbe FanDialect = iota
	// FanASRR: asrr/settings/{get,set}fancontrolmode, the two-step
	// set{open,close}looptablenumber + getfan{open,close}loopcontroltable,
	// setfandutymanualmode, GetFanDuty, setclearallfansetting (X470, B650 1.x).
	FanASRR
	// FanSettings: settings/fan/{mode,openloop,closedloop,manual,load_default}
	// (B650 7.02/9.05 and live 4.10).
	FanSettings
)

func (d FanDialect) String() string {
	switch d {
	case FanASRR:
		return "asrr"
	case FanSettings:
		return "settings/fan"
	}
	return "probe"
}

// LinkUpDialect selects the "keep NIC link up" endpoints.
type LinkUpDialect int

const (
	// LinkUpProbe: try settings/ncsilinkup, fall back to asrr/getvetobit.
	LinkUpProbe LinkUpDialect = iota
	// LinkUpVetobit: asrr/getvetobit + PUT asrr/setvetobit {vetobit}
	// (X470 UI JS; also seen on B650 1.10). UNVERIFIED: the PUT body
	// {vetobit} comes from the UI JS and was never written on hardware.
	LinkUpVetobit
	// LinkUpNCSI: settings/ncsilinkup GET/PUT {sharednic, sharednic2}
	// (B650 7.02/9.05 binaries and UI JS). UNVERIFIED: not probed on the
	// live B650 4.10; assumed from the 7.02/9.05 builds.
	LinkUpNCSI
)

func (d LinkUpDialect) String() string {
	switch d {
	case LinkUpVetobit:
		return "vetobit"
	case LinkUpNCSI:
		return "ncsilinkup"
	}
	return "probe"
}

// InventoryDialect selects the host inventory endpoints.
type InventoryDialect int

const (
	// InventoryHost: host_inventory/host_interface_*_info (all boards).
	InventoryHost InventoryDialect = iota
	// InventorySystem: system_inventory/{cpu_info,dimm_info,pci_info} first
	// (X470), falling back to host_inventory when it answers 500 code 11272
	// (host off / nothing pushed by the BIOS yet).
	InventorySystem
)

func (d InventoryDialect) String() string {
	if d == InventorySystem {
		return "system_inventory"
	}
	return "host_inventory"
}

// RestoreBackend selects how the AC power restore policy is read/written.
type RestoreBackend int

const (
	// RestoreRedfish: ComputerSystem.PowerRestorePolicy.
	RestoreRedfish RestoreBackend = iota
	// RestoreASRR: asrr/GetPowerRestore + asrr/SetPowerRestore (B650).
	RestoreASRR
	// RestoreIPMI: IPMI chassis status / set power restore policy (X470,
	// whose /api has no restore-policy handler).
	RestoreIPMI
)

// PostCodeDialect selects the POST code endpoint.
type PostCodeDialect int

const (
	// PostCodeBIOSCode: asrr/getbioscode {poststatus, postdata}.
	PostCodeBIOSCode PostCodeDialect = iota
	// PostCodeLogs: logs/postcode, falling back to asrr/getbioscode.
	PostCodeLogs
)

// Profile describes one board+firmware generation.
type Profile struct {
	// Key is a stable profile id: "x470d4u", "b650d4u", "b650d4u-1.x",
	// "b550d4u", "unknown".
	Key string `json:"key"`
	// Board is the live board name (asrr/model-name mb_name).
	Board string `json:"board"`
	// Firmware is the live BMC firmware version (asrr/fw-info).
	Firmware string `json:"firmware"`
	// Verified reports that this exact board+firmware was seen live or in a
	// HAR capture; false means the profile was inferred.
	Verified bool `json:"verified"`

	// TLS is the transport TLS profile (X470 legacy RSA-kx, B650 modern).
	TLS transport.TLSProfile `json:"-"`
	// LoginExtra holds extra /api login form fields (X470 certlogin=0).
	LoginExtra url.Values `json:"-"`
	// Redfish says whether to try Redfish.
	Redfish RedfishMode `json:"-"`
	// IPMI allows the lazy IPMI backend (boot override / restore policy).
	IPMI bool `json:"ipmi"`

	Fan       FanDialect       `json:"-"`
	LinkUp    LinkUpDialect    `json:"-"`
	Inventory InventoryDialect `json:"-"`
	Restore   RestoreBackend   `json:"-"`
	PostCode  PostCodeDialect  `json:"-"`
	// AutoDateField is the NTP enable field of settings/date-time:
	// "auto_date" (X470) or "ntp_auto_date" (B650/B550). The driver also
	// adapts to whichever key the BMC actually returns.
	AutoDateField string `json:"autoDateField"`
	// SensorMaskField is the threshold readable/settable mask field of
	// /api/sensors: "settable_readable_threshMask" (X470) or
	// "settable_flag" (B650/B550).
	SensorMaskField string `json:"sensorMaskField"`
	// ZeroThresholds: unset thresholds are reported as 0.0 (X470) rather
	// than "NA"; the readable mask decides which are real.
	ZeroThresholds bool `json:"zeroThresholds"`
	// EventTrailer: logs/eventlog (and possibly logs/event) ends with a
	// {"event_log_count":N} element that is not an event (B650/B550).
	EventTrailer bool `json:"eventTrailer"`
	// BondIfc: PUT settings/network-bond takes bond_ifc (B650/B550).
	BondIfc bool `json:"bondIfc"`
	// Prochot: asrr/getprochot + setprochot exist (X470 only).
	Prochot bool `json:"prochot"`
	// ClearMCA: DELETE asrr/clearmcalog exists (X470 only).
	ClearMCA bool `json:"clearMca"`
	// Components lists the asrr/maintenance/{C} components with a status
	// handler on this generation.
	Components []string `json:"components"`

	// ExtraDeny adds board-specific deny rules on top of
	// amimegarac.DefaultDenyRules.
	ExtraDeny []amimegarac.DenyRule `json:"-"`
	// Timeouts are per-endpoint attempt timeouts keyed by the canonical
	// /api path (e.g. "/api/asrr/selsummary"). Matching requests are also
	// never retried.
	Timeouts map[string]time.Duration `json:"-"`
}

// AllComponents lists every asrr/maintenance component name the vendor
// routes accept.
var AllComponents = []string{"BIOS", "CPLD", "FPGA", "RETIMER", "PSU", "BP"}

// TimeoutFor returns the per-endpoint timeout for a canonical /api path.
func (p Profile) TimeoutFor(canonicalPath string) (time.Duration, bool) {
	if len(p.Timeouts) == 0 {
		return 0, false
	}
	d, ok := p.Timeouts[strings.ToLower(canonicalPath)]
	return d, ok
}

// HasComponent reports whether c (case-insensitive) has a status handler.
func (p Profile) HasComponent(c string) bool {
	for _, x := range p.Components {
		if strings.EqualFold(x, c) {
			return true
		}
	}
	return false
}

// Generation families used before the board is known (login + TLS).
const (
	// FamilyX470 is the SPX 2.x/3.x generation (ServiceRoot v1_3_x, no
	// Vendor): legacy TLS, certlogin=0.
	FamilyX470 = "x470"
	// FamilyModern is the B650/B550 generation.
	FamilyModern = "modern"
)

// Lookup returns the profile for a live board name and firmware version.
// It never fails: unknown boards get the probing profile.
func Lookup(board, firmware string) Profile {
	b := strings.ToUpper(strings.TrimSpace(board))
	var p Profile
	switch {
	case strings.HasPrefix(b, "X470D4U"):
		p = X470D4U(firmware)
	case strings.HasPrefix(b, "B650D4U"):
		p = B650D4U(firmware)
	case strings.HasPrefix(b, "B550D4U"):
		p = B550D4U(firmware)
	default:
		p = Unknown(firmware)
	}
	p.Board = strings.TrimSpace(board)
	p.Firmware = strings.TrimSpace(firmware)
	return p
}

// ForFamily returns a provisional profile for a generation family, used to
// log in before the board is known.
func ForFamily(family string) Profile {
	if family == FamilyX470 {
		return X470D4U("")
	}
	return Unknown("")
}

// Unknown is the capability-probing profile for boards or versions we have
// never seen: Redfish is probed, every dialect choice is probed lazily, TLS
// auto-negotiates (modern, falling back to legacy), IPMI is allowed as a last
// resort.
func Unknown(fw string) Profile {
	return Profile{
		Key:             "unknown",
		Firmware:        fw,
		TLS:             transport.ProfileAuto,
		Redfish:         RedfishProbe,
		IPMI:            true,
		Fan:             FanProbe,
		LinkUp:          LinkUpProbe,
		Inventory:       InventoryHost,
		Restore:         RestoreRedfish,
		PostCode:        PostCodeLogs,
		AutoDateField:   "ntp_auto_date",
		SensorMaskField: "settable_flag",
		EventTrailer:    true,
		Components:      AllComponents,
		Timeouts:        defaultSlowEndpoints(),
	}
}

// defaultSlowEndpoints are endpoints known to hang on some firmware.
func defaultSlowEndpoints() map[string]time.Duration {
	return map[string]time.Duration{
		// B650 4.10: 500 code 50012 after a ~30 s hang.
		"/api/asrr/selsummary": 8 * time.Second,
	}
}

// MajorVersion parses the leading integer of a firmware version
// ("4.10.00" -> 4). ok is false when it does not parse.
func MajorVersion(fw string) (int, bool) {
	fw = strings.TrimSpace(fw)
	if i := strings.IndexByte(fw, '.'); i >= 0 {
		fw = fw[:i]
	}
	n, err := strconv.Atoi(fw)
	return n, err == nil
}
