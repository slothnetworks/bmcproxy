package boards

import (
	"strings"

	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
)

// MC13LE0Verified lists the BMC firmware versions probed live on an
// MC13-LE0 (docs/gigabyte/mc13-le0-live-probe.md).
var MC13LE0Verified = []string{"13.04.15"}

func isMC13LE0(board string) bool { return strings.HasPrefix(norm(board), "MC13LE0") }

// MC13LE0 is the Gigabyte MC13-LE0 (AM5, AST2600, AMI MegaRAC SP-X 13.x)
// profile. Live 13.04.15 (Redfish 1.11.0, ServiceRoot v1_13_0) accepts the
// web admin over Redfish Basic and sessions, so Redfish is the primary
// southbound; the AMI /api is used only for the fan-profile mode.
//
// Quirks seen live on 13.04.15:
//   - collections stop at 50 members without Members@odata.nextLink
//     (SEL 150 entries, AuditLog 68) and $skip=0 is a 400;
//   - $expand is advertised but members come back as bare links;
//   - Chassis/Self/Sensors lists only the energy/power monitors
//     (temperatures, fans, voltages are in Thermal/Power);
//   - SEL Created is the import time; EventTimestamp has bogus offsets;
//   - $select on a collection drops every member.
func MC13LE0(board, firmware string) Profile {
	p := Profile{
		Key: "mc13le0", Board: board, Firmware: firmware,
		Redfish:        rf.Quirks{SkipPageSize: 50, NoExpand: true},
		AMI:            true,
		AMICSRFOnGet:   true,
		ExtraDeny:      CommonDeny,
		SELTime:        SELTimeEvent,
		FanProfile:     true,
		BoardSerialOem: true,
		Notes:          "MC13-LE0: Redfish-first (Basic); AMI /api for fan-profile mode",
	}
	for _, v := range MC13LE0Verified {
		if strings.TrimSpace(firmware) == v {
			p.Verified = true
		}
	}
	if !p.Verified {
		p.Notes += "; firmware " + firmware + " not probed live (13.04.15 quirks assumed)"
	}
	return p
}
