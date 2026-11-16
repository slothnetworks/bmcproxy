package boards

import (
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// B550 firmware versions seen in the HAR (B550D4U-2T, 1.10.00).
var b550Verified = []string{"1.10"}

// B550D4U is the B550D4U / B550D4U-2T (AM4, newer SPX) profile. Only a HAR
// capture of 1.10.00 exists (captured as "B650D4U_2L2T" but model-name says
// B550D4U-2T), so most choices are probed:
//   - Redfish: probed (never tested).
//   - TLS: auto (modern, falling back to legacy).
//   - Fan / keep-NIC-link-up: probed (settings/fan/* then asrr/settings/*;
//     settings/ncsilinkup then asrr/getvetobit).
//   - date-time: ntp_auto_date. Sensors: settable_flag, "NA" thresholds.
//   - logs/eventlog trailer {"event_log_count":N}.
//   - POST code: asrr/getbioscode (HAR-confirmed); BIOS UpdateOption exists.
func B550D4U(fw string) Profile {
	return Profile{
		Key:             "b550d4u",
		Firmware:        fw,
		Verified:        verified(fw, b550Verified),
		TLS:             transport.ProfileAuto,
		Redfish:         RedfishProbe,
		IPMI:            true,
		Fan:             FanProbe,
		LinkUp:          LinkUpProbe,
		Inventory:       InventoryHost,
		Restore:         RestoreASRR,
		PostCode:        PostCodeBIOSCode,
		AutoDateField:   "ntp_auto_date",
		SensorMaskField: "settable_flag",
		EventTrailer:    true,
		BondIfc:         true,
		Components:      []string{"BIOS"},
		Timeouts: map[string]time.Duration{
			"/api/asrr/selsummary": 8 * time.Second,
		},
	}
}
