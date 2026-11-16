package boards

import (
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
)

// MZ32AR0Verified lists the BMC firmware versions probed live on an
// MZ32-AR0 (docs/gigabyte/mz32-ar0-live-probe.md, research agent; the live
// unit runs the same 12.61.39 build as bmc-firmware/MZ32-AR0-00.bin).
var MZ32AR0Verified = []string{"12.61.39"}

func isMZ32AR0(board string) bool { return strings.HasPrefix(norm(board), "MZ32AR0") }

// MZ32AR0Deny is the MZ32-AR0 12.61.39 deny list (docs/firmware/mz32-ar0.md
// §2.1) on top of CommonDeny and amimegarac.DefaultDenyRules (which
// already cover logs/help-data, download_image, download_config, hpm,
// firmware/flash/restore and GlobalLuaTables). Rules are lower-case
// substrings of the canonical path and refuse every method.
var MZ32AR0Deny = append(append([]amimegarac.DenyRule(nil), CommonDeny...),
	amimegarac.DenyRule{Pattern: "maintenance/backup_config", Reason: "builds and downloads a configuration archive"},
	amimegarac.DenyRule{Pattern: "maintenance/firmware/", Reason: "firmware-update state machine (dump, dwldfwimg, verification, upgrade, flash-progress)"},
	amimegarac.DenyRule{Pattern: "maintenance/flash", Reason: "firmware flash / flashtrack"},
	amimegarac.DenyRule{Pattern: "maintenance/fwupdate_", Reason: "firmware-update state machine"},
	amimegarac.DenyRule{Pattern: "maintenance/nm_", Reason: "node-manager firmware maintenance"},
	amimegarac.DenyRule{Pattern: "maintenance/dual_image_config", Reason: "dual-image boot selection"},
	amimegarac.DenyRule{Pattern: "maintenance/bmc_recovery", Reason: "BMC recovery"},
	amimegarac.DenyRule{Pattern: "maintenance/upload_restore", Reason: "configuration restore upload"},
	amimegarac.DenyRule{Pattern: "maintenance/preserve", Reason: "preserve-configuration control for flashing"},
	amimegarac.DenyRule{Pattern: "maintenance/system_admin", Reason: "sysadmin shell account control"},
	amimegarac.DenyRule{Pattern: "maintenance/clear_model_patch", Reason: "clears the model patch"},
	amimegarac.DenyRule{Pattern: "maintenance/sku", Reason: "SKU programming"},
	amimegarac.DenyRule{Pattern: "maintenance/redfish", Reason: "Redfish service maintenance"},
	amimegarac.DenyRule{Pattern: "system_inventory_gbt/smbios-file", Reason: "SMBIOS blob file download"},
	amimegarac.DenyRule{Pattern: "system_inventory_gbt/bios-setup-file", Reason: "BIOS setup blob download (has an upload twin)"},
	amimegarac.DenyRule{Pattern: "logs/video-data", Reason: "video log file download"},
	amimegarac.DenyRule{Pattern: "logs/video-log/delete", Reason: "deletes recorded video"},
	amimegarac.DenyRule{Pattern: "logs/acd-file", Reason: "crash dump file download"},
	amimegarac.DenyRule{Pattern: "extlog/archive", Reason: "log archive download"},
	amimegarac.DenyRule{Pattern: "/api/ae/", Reason: "Automation Engine script execution"},
	amimegarac.DenyRule{Pattern: "/api/ae_test", Reason: "Automation Engine test execution"},
	amimegarac.DenyRule{Pattern: "ipmi_response", Reason: "raw IPMI command passthrough"},
	amimegarac.DenyRule{Pattern: "/api/sample/", Reason: "AMI sample handlers compiled into production: unknown side effects"},
	amimegarac.DenyRule{Pattern: "kvm/token", Reason: "mints KVM session tokens"},
	amimegarac.DenyRule{Pattern: "remote_control/get/kvm/launch", Reason: "mints a KVM JNLP launch"},
	amimegarac.DenyRule{Pattern: "settings/dns/restart", Reason: "restarts DNS / network services"},
	amimegarac.DenyRule{Pattern: "settings/pef/send_test_alert", Reason: "sends a test alert"},
	amimegarac.DenyRule{Pattern: "operations/ssl/certificate/generate", Reason: "regenerates the TLS certificate"},
	amimegarac.DenyRule{Pattern: "/api/actions/", Reason: "actions behind a URL"},
	amimegarac.DenyRule{Pattern: "powercontrol/setnodestatus", Reason: "Gigabyte node power control"},
	amimegarac.DenyRule{Pattern: "start-media", Reason: "starts a remote media redirection"},
	amimegarac.DenyRule{Pattern: "stop-media", Reason: "stops a remote media redirection"},
	amimegarac.DenyRule{Pattern: "gbt/oem/cmc/chassis/power", Reason: "Gigabyte CMC node power / power limit"},
	amimegarac.DenyRule{Pattern: "eventservice/sse", Reason: "long-lived Redfish SSE stream"},
	amimegarac.DenyRule{Pattern: "biosstaticfiles", Reason: "BIOS static file downloads"},
)

// sp12Quirks are the AMI MegaRAC SP-X 12.x (Redfish 1.8.0) query quirks seen
// on MZ32-AR0 12.61.39: every option except $filter works with a literal
// `$` (the client never percent-encodes it), but $filter is applied to the
// current page only and the nextLink drops it, so it is disabled and the
// proxy filters client-side. Paging (50, literal ?$skip=50 nextLink) is
// correct, so no SkipPageSize.
func sp12Quirks() rf.Quirks {
	return rf.Quirks{QueryCaps: &rf.QueryCaps{
		Select: true, Expand: true, ExpandAll: true, ExpandDot: true, ExpandLink: true, Levels: true, MaxLevels: 5,
		Filter: false, TopSkip: true, OnlyMember: true, Excerpt: false,
	}}
}

// MZ32AR0 is the Gigabyte MZ32-AR0 (single-socket EPYC SP3, AST2500, AMI
// MegaRAC SP-X 12.x "WolfPass" base) profile. Redfish 1.8.0 over Basic
// (SessionTimeout 30 s) is the primary southbound; the AMI /api is used for
// the decoded SEL, the fan-profile mode, firmware-info (active image) and
// raw/api.
//
// Quirks (docs/gigabyte/mz32-ar0-live-probe.md §5, §8):
//   - $filter page-local (disabled via QueryCaps); unknown $-params 501;
//   - Chassis/Self/Sensors only Energy/Power: sensors come from the legacy
//     Thermal/Power (generic merge);
//   - ResetType On/ForceOff/GracefulShutdown/ForceRestart only: power cycle
//     and NMI are 501 (generic ResetTypeFor);
//   - Redfish SEL split over three LogServices with hex Messages: events
//     come from /api/logs/event;
//   - Processors/CPU0/SubProcessors is a ~200-resource crawl trap (the
//     generic inventory never walks it);
//   - Bios @Redfish.Settings is a stub: no BIOS writes.
func MZ32AR0(board, firmware string) Profile {
	p := Profile{
		Key: "mz32ar0", Board: board, Firmware: firmware,
		Redfish:        sp12Quirks(),
		AMI:            true,
		AMICSRFOnGet:   true,
		ExtraDeny:      MZ32AR0Deny,
		SELTime:        SELTimeCreated,
		SELSource:      SELAMI,
		FanProfile:     true,
		BoardSerialOem: true,
		Notes:          "MZ32-AR0: Redfish-first (Basic); AMI /api for decoded SEL, fan-profile mode, firmware-info",
	}
	for _, v := range MZ32AR0Verified {
		if strings.TrimSpace(firmware) == v {
			p.Verified = true
		}
	}
	if !p.Verified {
		p.Notes += "; firmware " + firmware + " not probed live (12.61.39 quirks assumed)"
	}
	return p
}
