//go:build !x470_pam

package asrock

import (
	"context"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
)

// x470PAMRedfish is the default-build no-op: the driver never mutates BMC
// account configuration on its own. Build with -tags x470_pam to let the
// driver enable Redfish PAM authentication on X470D4U (see
// x470_pam_autoenable.go and scripts/x470d4u-redfish-pam-auth.sh).
func x470PAMRedfish(context.Context, *Driver, bmc.Target, bmc.Creds, boards.Profile) bool {
	return false
}
