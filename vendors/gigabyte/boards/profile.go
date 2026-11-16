// Package boards holds the Gigabyte (Giga Computing) board + firmware
// profiles: which southbound protocols a board speaks and the quirks of its
// live firmware. The driver selects a profile from the Redfish
// ComputerSystem Model and the Manager FirmwareVersion reported by the
// LIVE BMC (never the firmware image), and falls back to Unknown (capability
// probing) for boards or versions without evidence.
//
// Adding a board: write <board>.go with a constructor returning a Profile
// and add it to the known table below (see mc13le0.go). Keep quirks as data
// here; the driver code should not test board names.
package boards

import (
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
)

// SELTime selects where a SEL entry's event time comes from.
type SELTime int

const (
	// SELTimeCreated uses LogEntry.Created (standard; generic driver).
	SELTimeCreated SELTime = iota
	// SELTimeEvent uses LogEntry.EventTimestamp with its bogus UTC offset
	// replaced (the "Timestamp :" field of Message wins when present).
	// MC13-LE0 13.04.15: Created is the time the Redfish sync agent
	// imported the IPMI SEL (identical for every entry) and EventTimestamp
	// carries garbage offsets such as "+00:05" while the BMC runs in UTC.
	SELTimeEvent
)

// SELSource selects the backend of the event log.
type SELSource int

const (
	// SELRedfish reads Managers/{id}/LogServices/SEL over Redfish.
	SELRedfish SELSource = iota
	// SELAMI reads the decoded IPMI SEL from the AMI /api
	// (logs/event?LASTEVENTID=n) and falls back to Redfish when the /api
	// does not serve it. MZ32-AR0 12.61.39: Redfish holds only 57+86+4
	// entries split over three LogServices with hex-only Messages, while
	// /api/logs/event returns all 1,022 records decoded.
	SELAMI
)

// Profile describes one board + firmware.
type Profile struct {
	// Key is a stable identifier ("mc13le0", "unknown").
	Key string
	// Board and Firmware are the live values the profile was selected for.
	Board, Firmware string
	// Verified is true when this exact firmware version was probed live.
	Verified bool
	// Redfish quirks applied to every Redfish client of the session.
	Redfish rf.Quirks
	// AMI reports an AMI MegaRAC SP-X web stack with the legacy /api
	// (form login, QSESSIONID + X-CSRFTOKEN). The driver logs in lazily,
	// only for the vendor routes that need it (fan mode, raw/api).
	AMI bool
	// AMICSRFOnGet: the /api enforces the CSRF header on GETs as well and a
	// wrong token kills the session (MC13-LE0 13.04.15). amimegarac always
	// sends the token, so this is documentation for raw tooling.
	AMICSRFOnGet bool
	// ExtraDeny adds /api and Redfish deny rules on top of
	// amimegarac.DefaultDenyRules (raw passthrough and the /api client).
	ExtraDeny []amimegarac.DenyRule
	// SELTime selects the SEL timestamp source.
	SELTime SELTime
	// SELSource selects the event log backend.
	SELSource SELSource
	// FanProfile: the Redfish OEM GBTFanprofileService under
	// Chassis/{id}/Thermal (Oem.Gbt.FanprofileService) is present.
	FanProfile bool
	// BoardSerialOem: the real baseboard serial is in Chassis
	// Oem.GBTChassisOemProperty."Board Serial Number" (Chassis.SerialNumber
	// is the FRU chassis placeholder).
	BoardSerialOem bool
	// Notes is a human summary for /capabilities and logs.
	Notes string
}

// CommonDeny are the deny rules for every Gigabyte MegaRAC build (MC13-LE0
// firmware 13.06.30 handler inventory, docs/firmware/mc13-le0.md §5): file
// dumps and downloads, diagnostics that run on GET, crash/debug collectors
// and broken-image savers.
var CommonDeny = []amimegarac.DenyRule{
	{Pattern: "hostinterface/diag", Reason: "Gigabyte host diagnostics: start/abort/diagnose run tests on the host"},
	{Pattern: "logs/audit-file", Reason: "file download of the audit log (FastCGI file send)"},
	{Pattern: "logs/system-file", Reason: "file download of the system log (FastCGI file send)"},
	{Pattern: "logs/event-file", Reason: "file download of the SEL (FastCGI file send)"},
	{Pattern: "maintenance/firmware/dump", Reason: "firmware image dump"},
	{Pattern: "maintenance/bios_presv_update_log/dump", Reason: "BIOS update log dump"},
	{Pattern: "save/biosbrokenimage", Reason: "writes a broken-image capture to flash"},
	{Pattern: "save/bmcbrokenimage", Reason: "writes a broken-image capture to flash"},
	{Pattern: "acd/", Reason: "AMD crash dump collection/download"},
	{Pattern: "host/get_crash_dump", Reason: "crash dump collection"},
	{Pattern: "download_addc", Reason: "autonomous debug data collection download"},
	{Pattern: "settings/network/conftdownload", Reason: "network configuration file download"},
}

type known struct {
	match func(board string) bool
	build func(board, firmware string) Profile
}

// table lists the boards with evidence. Order matters (first match wins).
var table = []known{
	{match: isMC13LE0, build: MC13LE0},
	{match: isMZ32AR0, build: MZ32AR0},
}

// AllExtraDeny unions every profile's extra deny rules (the static
// denylist the raw passthrough checks before a session exists).
func AllExtraDeny() []amimegarac.DenyRule {
	var out []amimegarac.DenyRule
	for _, p := range []Profile{MC13LE0("", ""), MZ32AR0("", ""), Unknown("", "12.0", true), Unknown("", "13.0", true)} {
		out = append(out, p.ExtraDeny...)
	}
	return out
}

// fwMajor returns the major number of an AMI "12.61.39"-style version
// (-1 when unparsable). AMI MegaRAC SP-X 12.x (AST2500, Redfish 1.8) and
// 13.x (AST2600, Redfish 1.11+) need different quirks.
func fwMajor(fw string) int {
	fw = strings.TrimSpace(fw)
	n, i := 0, 0
	for ; i < len(fw) && fw[i] >= '0' && fw[i] <= '9'; i++ {
		n = n*10 + int(fw[i]-'0')
	}
	if i == 0 {
		return -1
	}
	return n
}

// Lookup returns the profile for a live board + firmware. Unknown boards
// get Unknown(board, firmware, true).
func Lookup(board, firmware string) Profile {
	for _, k := range table {
		if k.match(board) {
			return k.build(board, firmware)
		}
	}
	return Unknown(board, firmware, true)
}

// Unknown is the capability-probing fallback for a Gigabyte board without a
// profile. amiStack reports an AMI MegaRAC Redfish service (ServiceRoot
// Oem.Ami). The quirks follow the firmware generation: major 12 gets the
// MZ32-AR0 (SP-X 12.x) set, anything else the MC13-LE0 (SP-X 13.x) set,
// whose collection-truncation workaround is harmless on services that do
// send nextLinks.
func Unknown(board, firmware string, amiStack bool) Profile {
	p := Profile{Key: "unknown", Board: board, Firmware: firmware, BoardSerialOem: true,
		SELTime: SELTimeCreated, Notes: "unknown Gigabyte board/firmware: Redfish capability probing"}
	if amiStack {
		p.AMI = true
		p.FanProfile = true // probed: the routes 501 when the resource is absent
		if fwMajor(firmware) == 12 {
			p.Redfish = sp12Quirks()
			p.ExtraDeny = MZ32AR0Deny
			p.Notes += " (AMI SP-X 12.x quirks)"
		} else {
			p.Redfish = rf.Quirks{SkipPageSize: 50, NoExpand: true}
			p.ExtraDeny = CommonDeny
			p.Notes += " (AMI SP-X 13.x quirks)"
		}
	}
	return p
}

// norm upper-cases and strips spaces, dashes and underscores.
func norm(s string) string {
	return strings.NewReplacer(" ", "", "-", "", "_", "").Replace(strings.ToUpper(strings.TrimSpace(s)))
}
