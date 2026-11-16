package boards

import (
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// X470 firmware versions seen live or in captures (live 2.32.00; the dumped
// image and the endpoint catalog are 3.02.00).
var x470Verified = []string{"2.32"}

// X470D4U is the X470D4U (AM4, SPX 2.x/3.x) profile.
//
//   - Redfish: no. Live 2.32 rejects the admin account (401 AccessDenied,
//     Basic and session; separate Redfish account store, PAM gated off). The
//     opt-in x470_pam build tag can flip this (see x470_pam_autoenable.go).
//   - TLS: legacy (SSLv3-TLS1.2, RSA key exchange only, expired cert).
//   - Login: certlogin=0 like the web UI.
//   - Fan: asrr/settings/* with the two-step table select+read.
//   - Keep-NIC-link-up: asrr/getvetobit / setvetobit.
//   - date-time: auto_date. Sensors: settable_readable_threshMask, unset
//     thresholds reported as 0.0.
//   - Inventory: system_inventory/* (500 code 11272 while the host is off),
//     host_inventory fallback.
//   - Restore policy and boot override: IPMI (no /api handler).
//   - prochot and clearmcalog exist.
func X470D4U(fw string) Profile {
	return Profile{
		Key:             "x470d4u",
		Firmware:        fw,
		Verified:        verified(fw, x470Verified),
		TLS:             transport.ProfileLegacy,
		LoginExtra:      amimegarac.X470LoginExtra(),
		Redfish:         RedfishNo,
		IPMI:            true,
		Fan:             FanASRR,
		LinkUp:          LinkUpVetobit,
		Inventory:       InventorySystem,
		Restore:         RestoreIPMI,
		PostCode:        PostCodeBIOSCode,
		AutoDateField:   "auto_date",
		SensorMaskField: "settable_readable_threshMask",
		ZeroThresholds:  true,
		EventTrailer:    false,
		BondIfc:         false,
		Prochot:         true,
		ClearMCA:        true,
		Components:      []string{"BIOS", "CPLD"},
		ExtraDeny: []amimegarac.DenyRule{
			// Java KVM launcher download; skipped as unsafe by the live
			// probe (docs/x470/live-probe.md) and never needed by the proxy.
			{Pattern: "asrr/java-console", Reason: "Java console JNLP download (X470; skipped as unsafe in the live probe)"},
		},
		Timeouts: defaultSlowEndpoints(),
	}
}

func verified(fw string, list []string) bool {
	for _, v := range list {
		if strings.HasPrefix(strings.TrimSpace(fw), v) {
			return true
		}
	}
	return false
}
