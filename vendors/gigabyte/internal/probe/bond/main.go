//go:build gigabytelive

// Command bond is a READ-ONLY live check of the AMI settings/network-bond
// object on a Gigabyte MegaRAC SP-X BMC. It is excluded from default builds
// (tag gigabytelive):
//
//	set -a; . ./mc13-le0-000.env; set +a
//	go run -tags gigabytelive ./vendors/gigabyte/internal/probe/bond
//
// It makes exactly three requests, one at a time: POST /api/session
// (login), GET /api/settings/network-bond and DELETE /api/session (logout,
// retried once on 5xx by the client). Credentials come only from the
// environment (BMC_HOST, BMC_USER, BMC_PASS) and are never printed. The
// printed object is sanitized: strings that look like MAC or IP addresses
// are replaced.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte"
)

var (
	macRe = regexp.MustCompile(`(?i)\b[0-9a-f]{2}([:-][0-9a-f]{2}){5}\b`)
	ipRe  = regexp.MustCompile(`\b\d{1,3}(\.\d{1,3}){3}\b`)
	ip6Re = regexp.MustCompile(`(?i)\b[0-9a-f]{0,4}(:[0-9a-f]{0,4}){2,7}\b`)
)

func sanitize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = sanitize(e)
		}
	case []any:
		for i, e := range x {
			x[i] = sanitize(e)
		}
	case string:
		x = macRe.ReplaceAllString(x, "00:00:5e:00:53:00")
		x = ipRe.ReplaceAllString(x, "192.0.2.1")
		return ip6Re.ReplaceAllString(x, "2001:db8::1")
	}
	return v
}

func main() {
	host, user, pass := os.Getenv("BMC_HOST"), os.Getenv("BMC_USER"), os.Getenv("BMC_PASS")
	if host == "" || user == "" || pass == "" {
		fmt.Fprintln(os.Stderr, "BMC_HOST, BMC_USER and BMC_PASS must be set")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	tg, err := bmc.ParseTarget(host)
	if err != nil {
		fmt.Fprintln(os.Stderr, "target:", err)
		os.Exit(2)
	}
	cl, err := amimegarac.New(amimegarac.Config{
		Endpoint: gigabyte.Default.Transport().Endpoint(tg.Addr(), transport.LaneAMI),
		Target:   tg,
		Creds:    bmc.Creds{User: user, Pass: pass},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "client:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := cl.Login(ctx); err != nil {
		fmt.Printf("login %dms ERR %v\n", time.Since(t0).Milliseconds(), err)
		os.Exit(1)
	}
	fmt.Printf("login %dms ok\n", time.Since(t0).Milliseconds())
	defer func() {
		t0 := time.Now()
		err := cl.Logout(context.Background())
		fmt.Printf("logout %dms err=%v\n", time.Since(t0).Milliseconds(), err)
	}()

	t0 = time.Now()
	var raw []byte
	err = cl.Get(ctx, "settings/network-bond", &raw)
	fmt.Printf("GET settings/network-bond %dms err=%v bytes=%d\n", time.Since(t0).Milliseconds(), err, len(raw))
	if err != nil {
		return
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		fmt.Printf("not JSON: %v\n", err)
		return
	}
	v = sanitize(v)
	if m, ok := v.(map[string]any); ok {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %s: %T\n", k, m[k])
		}
	}
	out, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("%s\n", out)
}
