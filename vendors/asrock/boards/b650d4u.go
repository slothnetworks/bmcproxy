package boards

import (
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// B650 firmware versions seen live (4.10.00) or in the HAR (1.10.00).
var b650Verified = []string{"4.10", "1.10"}

// B650D4U is the B650D4U (AM5) profile.
//
// Firmware 2.x and later (live 4.10, dumped 7.02/9.05):
//   - Redfish: yes (admin Basic auth works; Redfish-first, /api for OEM
//     features and gaps: restore policy, FRU, bonding, fan, NC-SI).
//   - TLS: modern (TLS1.2/1.3, ECDHE/AEAD).
//   - Fan: settings/fan/* (live 4.10 answers settings/fan/mode).
//   - Keep-NIC-link-up: settings/ncsilinkup {sharednic, sharednic2}.
//   - date-time: ntp_auto_date (+ ptp_*). Sensors: settable_flag, unset
//     thresholds "NA", unit "°C" (HAR 1.10: "deg_c").
//   - logs/eventlog ends with a {"event_log_count":N} trailer.
//   - Restore policy: asrr/GetPowerRestore / SetPowerRestore.
//   - asrr/selsummary hangs ~30 s then 500 code 50012: short timeout.
//   - No prochot, no clearmcalog.
//
// Firmware 1.x (HAR 1.10) still uses the X470-style asrr/settings fan
// endpoints and asrr/getvetobit; Redfish is probed.
func B650D4U(fw string) Profile {
	p := Profile{
		Key:             "b650d4u",
		Firmware:        fw,
		Verified:        verified(fw, b650Verified),
		TLS:             transport.ProfileModern,
		Redfish:         RedfishYes,
		IPMI:            true,
		Fan:             FanSettings,
		LinkUp:          LinkUpNCSI,
		Inventory:       InventoryHost,
		Restore:         RestoreASRR,
		PostCode:        PostCodeLogs,
		AutoDateField:   "ntp_auto_date",
		SensorMaskField: "settable_flag",
		ZeroThresholds:  false,
		EventTrailer:    true,
		BondIfc:         true,
		Prochot:         false,
		ClearMCA:        false,
		Components:      []string{"BIOS", "CPLD", "FPGA", "RETIMER"},
		Timeouts: map[string]time.Duration{
			"/api/asrr/selsummary": 8 * time.Second,
		},
	}
	if major, ok := MajorVersion(fw); ok && major < 2 {
		p.Key = "b650d4u-1.x"
		p.Fan = FanASRR
		p.LinkUp = LinkUpVetobit
		p.Redfish = RedfishProbe
		p.PostCode = PostCodeBIOSCode
		p.Components = []string{"BIOS"}
	} else if !ok {
		// Unparseable version: keep the modern defaults but probe dialects.
		p.Fan = FanProbe
		p.LinkUp = LinkUpProbe
		p.Redfish = RedfishProbe
	}
	return p
}
