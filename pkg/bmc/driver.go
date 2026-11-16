// Package bmc is the vendor-neutral "plug" contract of bmcproxy.
//
// A vendor driver implements Driver and registers itself from an init
// function (database/sql style):
//
//	func init() { bmc.Register(&Driver{}) }
//
// and cmd/bmcproxy blank-imports the driver package. The core resolves a
// Driver for a Target with Resolve, opens a Session with the caller's
// pass-through Creds, and type-asserts the Session against the small
// capability interfaces in capabilities.go. A capability the Session does not
// implement (or for which it returns ErrUnsupported) becomes HTTP 501.
//
// This package has no dependencies on the rest of bmcproxy.
package bmc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
)

// DefaultHTTPSPort and DefaultIPMIPort are used when Target ports are zero.
const (
	DefaultHTTPSPort = 443
	DefaultIPMIPort  = 623
)

// Target identifies one BMC. It is non-secret and safe to log and cache.
type Target struct {
	// Host is the IP address or DNS name, without port.
	Host string
	// Port is the HTTPS port; 0 means 443.
	Port int
	// IPMIPort is the RMCP+ port; 0 means 623.
	IPMIPort int
	// VendorHint is the optional X-BMC-Vendor override (a Driver Name). When
	// set, Resolve uses that driver without auto-detection.
	VendorHint string
}

// ParseTarget parses an X-BMC-Host value: "host", "host:port", "[v6]" or
// "[v6]:port".
func ParseTarget(hostport string) (Target, error) {
	s := strings.TrimSpace(hostport)
	if s == "" {
		return Target{}, fmt.Errorf("%w: empty BMC host", ErrInvalidArgument)
	}
	if strings.Contains(s, "/") || strings.Contains(s, "@") {
		return Target{}, fmt.Errorf("%w: BMC host %q must be host[:port]", ErrInvalidArgument, s)
	}
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		// no port
		host = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
		if strings.ContainsAny(host, "[]") || (strings.Count(host, ":") == 1) {
			return Target{}, fmt.Errorf("%w: BMC host %q: %v", ErrInvalidArgument, s, err)
		}
		return Target{Host: host}, nil
	}
	p, err := strconv.Atoi(portStr)
	if err != nil || p <= 0 || p > 65535 {
		return Target{}, fmt.Errorf("%w: BMC port %q", ErrInvalidArgument, portStr)
	}
	if host == "" {
		return Target{}, fmt.Errorf("%w: empty BMC host", ErrInvalidArgument)
	}
	return Target{Host: host, Port: p}, nil
}

// HTTPSPort returns Port or 443.
func (t Target) HTTPSPort() int {
	if t.Port == 0 {
		return DefaultHTTPSPort
	}
	return t.Port
}

// Addr returns "host:port" for HTTPS (IPv6 bracketed). It is also the key used
// by pkg/transport for per-host limiting and circuit breaking.
func (t Target) Addr() string { return net.JoinHostPort(t.Host, strconv.Itoa(t.HTTPSPort())) }

// IPMIAddr returns "host:ipmiport".
func (t Target) IPMIAddr() string {
	p := t.IPMIPort
	if p == 0 {
		p = DefaultIPMIPort
	}
	return net.JoinHostPort(t.Host, strconv.Itoa(p))
}

// BaseURL returns "https://host[:port]" with no trailing slash (port omitted
// when 443).
func (t Target) BaseURL() string {
	if t.HTTPSPort() == DefaultHTTPSPort {
		h := t.Host
		if strings.Contains(h, ":") {
			h = "[" + h + "]"
		}
		return "https://" + h
	}
	return "https://" + t.Addr()
}

// Key is a stable cache key for the target (Addr; VendorHint excluded).
func (t Target) Key() string { return t.Addr() }

func (t Target) String() string { return t.Addr() }

// Creds are the BMC's own credentials, passed through per request. They must
// never be logged or persisted. LogValue and String redact the password.
type Creds struct {
	User string
	Pass string
}

// LogValue implements slog.LogValuer: only the user name is logged.
func (c Creds) LogValue() slog.Value {
	return slog.GroupValue(slog.String("user", c.User), slog.String("pass", "[REDACTED]"))
}

func (c Creds) String() string { return c.User + ":[REDACTED]" }

// GoString redacts in %#v too.
func (c Creds) GoString() string {
	return fmt.Sprintf("bmc.Creds{User:%q, Pass:\"[REDACTED]\"}", c.User)
}

// Score values for Match.Score. Detection compares scores across drivers.
const (
	ScoreNone     = 0   // not this driver's BMC
	ScoreFallback = 10  // generic: e.g. any Redfish service
	ScoreLikely   = 50  // vendor matched, board unknown
	ScoreCertain  = 100 // vendor and board identified; Resolve stops here
)

// Match is a driver's answer to Detect.
type Match struct {
	Score        int    // see Score* constants; 0 = no match
	Vendor       string // driver/vendor name, e.g. "asrock"
	Manufacturer string // as reported, e.g. "ASRockRack"
	Board        string // e.g. "X470D4U", "B650D4U"
	Firmware     string // live-reported BMC firmware version, e.g. "2.32.00"
	// Backends lists protocols found reachable during detection
	// ("redfish", "ami", "ipmi"); authentication is established at Open.
	Backends []string
	// Details carries free-form non-secret detection facts for /debug/targets.
	Details map[string]string
}

// Driver is the plug interface every vendor implements.
type Driver interface {
	// Name is the stable lower-case vendor name used for registration,
	// X-BMC-Vendor and the /api/v1/vendor/{name}/ prefix.
	Name() string
	// Detect probes the target without credentials (e.g. Redfish ServiceRoot)
	// and reports how well this driver fits. It must be cheap, must not
	// mutate the BMC, and must return a zero Match (not an error) when the
	// BMC is clearly someone else's. Drivers that need credentials to detect
	// also implement CredDetector.
	Detect(ctx context.Context, t Target) (Match, error)
	// Open authenticates with c and returns a Session. Open may reuse
	// knowledge from a previous Detect but must work without it (target
	// cache miss). Errors should wrap ErrAuth / ErrTimeout / ErrUpstream.
	Open(ctx context.Context, t Target, c Creds) (Session, error)
}

// CredDetector is optionally implemented by drivers whose detection needs
// credentials (e.g. AMI /api/asrr/model-name). Resolve calls DetectWithCreds
// instead of Detect when creds are available.
type CredDetector interface {
	DetectWithCreds(ctx context.Context, t Target, c Creds) (Match, error)
}

// Prioritized is optionally implemented to order detection. Higher runs
// first; the default is 0. Generic fallbacks (generic Redfish) should return a
// negative priority.
type Prioritized interface {
	Priority() int
}
