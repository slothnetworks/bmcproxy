package boards

import (
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// X570 firmware versions seen live (1.20.00, docs/x570/live-probe.md). The
// dumped image is 2.11.00 (docs/firmware/x570d4u.md).
var x570Verified = []string{"1.20"}

// X570D4U is the X570D4U / X570D4U-2L2T (AM4, AST2500, SPX 13 / 5.X
// codebase) profile. It mixes the two older generations: the B650 stack
// (Redfish 1.8.0 that accepts the synced /api admin, modern TLS, no
// certlogin, settable_flag sensors, ntp_auto_date, asrr/GetPowerRestore)
// with the X470 /api dialect (asrr/settings/* fans, asrr/getvetobit).
//
// Evidence (live 1.20.00 unless noted):
//   - Redfish: yes for control and identity (admin Basic auth and sessions
//     work, PAMEnabled false; the /api user is synced into the Redfish store
//     by ipmiredfishusersync like the B650). But its data is poor, so these
//     capabilities prefer /api (AMIFirst):
//   - Sensors: Chassis/Self/Sensors has 3 members with null readings,
//     Thermal/Power 35; /api/sensors has 48 with readings.
//   - Event log: Systems/Self/LogServices has only BIOS, the Manager SEL
//     2 entries; /api has 3639 records. Single EventLog entries stalled
//     Redfish for ~3 minutes (60 s timeouts).
//   - Firmware inventory: only BMC has a Version; asrr/fw-info has
//     BMC/BIOS/ME/microcode.
//   - TLS: modern (TLS1.2 ECDHE/DHE AEAD + TLS1.3), self-signed AMI cert.
//   - Login: no certlogin. CSRF is enforced on GETs too (the client always
//     sends it). Login takes ~2.9 s.
//   - Fan: asrr/settings/* (settings/fan/mode 404). asrr/GetFanDuty is
//     absent (404): FanDuty reads asrr/settings/getfandutymanualmode.
//   - Keep-NIC-link-up: asrr/getvetobit (settings/ncsilinkup 404).
//   - date-time: ntp_auto_date. Sensors: settable_flag, "NA" thresholds.
//   - logs/eventlog: no event_log_count trailer on 1.20 (the string is in
//     the 2.11 libasrr; the driver skips a trailer either way). A full
//     logs/event?LASTEVENTID=0 read is ~4 MB / 34 s for 3639 records.
//   - Inventory: host_inventory (system_inventory/* 500 code 11272 with the
//     host off, and the 2.11 UI uses host_inventory).
//   - Restore policy: asrr/GetPowerRestore {powerRestore} (Redfish
//     PowerRestorePolicy is null).
//   - POST code: logs/postcode. No bond_ifc, no prochot, no clearmcalog.
//   - asrr/selsummary hangs 30 s then 500 code 1007: short timeout.
//
// The opt-in x470_pam build tag is NOT extended to this board: Redfish
// already accepts the /api admin (UNIFIED_USER_ACCOUNT sync), so there is
// nothing to enable (see x470_pam_autoenable.go, which checks Key
// "x470d4u").
//
// Other versions: 2.x+ (dumped 2.11.00 has the same handler set plus
// getfan100druingbmcupdate) keep this profile, unverified. An unparseable
// version keeps the transport choices but probes the /api dialects and
// Redfish.
func X570D4U(fw string) Profile {
	p := Profile{
		Key:             "x570d4u",
		Firmware:        fw,
		Verified:        verified(fw, x570Verified),
		TLS:             transport.ProfileModern,
		Redfish:         RedfishYes,
		IPMI:            true,
		Fan:             FanASRR,
		FanDutyManual:   true,
		LinkUp:          LinkUpVetobit,
		Inventory:       InventoryHost,
		Restore:         RestoreASRR,
		PostCode:        PostCodeLogs,
		AutoDateField:   "ntp_auto_date",
		SensorMaskField: "settable_flag",
		ZeroThresholds:  false,
		EventTrailer:    false,
		BondIfc:         false,
		Prochot:         false,
		ClearMCA:        false,
		Components:      []string{"BIOS", "BP"},
		AMIFirst:        []bmc.Capability{bmc.CapSensors, bmc.CapEventLog, bmc.CapEventLogClear, bmc.CapFirmwareInventory},
		Timeouts: map[string]time.Duration{
			"/api/asrr/selsummary": 8 * time.Second,
			// The first incremental read (LASTEVENTID=0) returns the whole
			// SEL: 34 s live for 3639 records. Allow more than the 60 s
			// default attempt timeout for a fuller SEL, and never retry it.
			"/api/logs/event": 90 * time.Second,
		},
	}
	if _, ok := MajorVersion(fw); !ok {
		p.Fan = FanProbe
		p.FanDutyManual = false
		p.LinkUp = LinkUpProbe
		p.Redfish = RedfishProbe
	}
	return p
}
