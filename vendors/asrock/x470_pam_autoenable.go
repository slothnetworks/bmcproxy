//go:build x470_pam

package asrock

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
)

// Opt-in X470D4U Redfish PAM auto-enable (build tag x470_pam).
//
// X470D4U firmware keeps a separate Redfish account store, so the /api user
// (e.g. admin) gets 401 from Redfish. The firmware has pam_ipmi wired into
// the Redfish PAM stack behind the OEM property PAMEnabled on
// /redfish/v1/AccountService/Configurations. With this tag the driver, on
// opening an X470D4U session, logs in to Redfish as a bootstrap admin
// (default "Administrator", password from the environment), reads
// PAMEnabled + ETag, PATCHes {"PAMEnabled":true} with If-Match and verifies;
// afterwards the caller's IPMI/web credentials work on Redfish too.
// This mirrors scripts/x470d4u-redfish-pam-auth.sh -a enable.
//
// The X570D4U is deliberately excluded (Key "x570d4u" never matches): its
// 1.20 Redfish already accepts the /api admin (UNIFIED_USER_ACCOUNT sync,
// PAMEnabled false), so there is nothing to enable and no reason to PATCH
// AccountService on it (docs/x570/live-probe.md).
//
// Environment:
//
//	BMCPROXY_X470_PAM_ADMIN_USER  bootstrap Redfish admin (default Administrator)
//	BMCPROXY_X470_PAM_ADMIN_PASS  its password (required; unset = feature off)
const (
	envPAMAdminUser = "BMCPROXY_X470_PAM_ADMIN_USER"
	envPAMAdminPass = "BMCPROXY_X470_PAM_ADMIN_PASS"
	pamConfigPath   = "/redfish/v1/AccountService/Configurations"
)

// pamDone remembers hosts already enabled (or found enabled) by this process.
var pamDone sync.Map

func x470PAMRedfish(ctx context.Context, d *Driver, t bmc.Target, c bmc.Creds, p boards.Profile) bool {
	if p.Key != "x470d4u" {
		return false
	}
	if _, ok := pamDone.Load(t.Addr()); ok {
		return true
	}
	pass := os.Getenv(envPAMAdminPass)
	if pass == "" {
		return false
	}
	user := os.Getenv(envPAMAdminUser)
	if user == "" {
		user = "Administrator"
	}
	log := d.log(ctx)
	cl, err := rf.New(d.Transport().Endpoint(t.Addr(), transport.LaneRedfish), rf.Options{
		Creds: bmc.Creds{User: user, Pass: pass}, Auth: rf.AuthBasic,
	})
	if err != nil {
		log.WarnContext(ctx, "x470 pam: redfish client", "err", err)
		return false
	}
	defer cl.Close(ctx)
	changed, err := enablePAM(ctx, cl)
	if err != nil {
		log.WarnContext(ctx, "x470 pam: enable failed; staying on /api + IPMI", "host", t.Addr(), "err", err)
		return false
	}
	pamDone.Store(t.Addr(), true)
	log.InfoContext(ctx, "x470 pam: Redfish PAM authentication enabled", "host", t.Addr(), "changed", changed,
		"note", "rotate the bootstrap Redfish admin password separately")
	return true
}

// enablePAM is the script's "enable" action against an authenticated client.
func enablePAM(ctx context.Context, cl *rf.Client) (changed bool, err error) {
	read := func() (bool, string, error) {
		var cfg struct {
			PAMEnabled any `json:"PAMEnabled"`
		}
		resp, err := cl.Get(ctx, pamConfigPath, &cfg, rf.WithOp("x470PAM.read"))
		if err != nil {
			if rf.IsStatus(err, http.StatusNotFound) {
				return false, "", fmt.Errorf("%s not found: not an AMI board with this OEM resource: %w", pamConfigPath, err)
			}
			return false, "", err
		}
		etag := ""
		if resp != nil {
			etag = resp.ETag
		}
		return strings.EqualFold(fmt.Sprint(cfg.PAMEnabled), "true"), etag, nil
	}
	on, etag, err := read()
	if err != nil {
		return false, err
	}
	if on {
		return false, nil
	}
	if etag == "" {
		return false, errors.New("no ETag on " + pamConfigPath + "; cannot do conditional PATCH")
	}
	_, err = cl.Patch(ctx, pamConfigPath, map[string]any{"PAMEnabled": true}, rf.WithIfMatch(etag), rf.WithOp("x470PAM.enable"))
	if err != nil {
		switch {
		case rf.IsStatus(err, http.StatusPreconditionRequired), rf.IsStatus(err, http.StatusPreconditionFailed):
			// ETag changed under us: one re-read and retry, like re-running the script.
			if _, etag, err = read(); err != nil {
				return false, err
			}
			if _, err = cl.Patch(ctx, pamConfigPath, map[string]any{"PAMEnabled": true}, rf.WithIfMatch(etag), rf.WithOp("x470PAM.enable")); err != nil {
				return false, fmt.Errorf("PATCH PAMEnabled (after ETag refresh): %w", err)
			}
		default:
			return false, fmt.Errorf("PATCH PAMEnabled: %w", err)
		}
	}
	if on, _, err = read(); err != nil {
		return true, err
	}
	if !on {
		return true, errors.New("verify failed: PAMEnabled still false after PATCH")
	}
	return true, nil
}
