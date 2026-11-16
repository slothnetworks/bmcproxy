package amimegarac

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"sync"
)

// DenyRule blocks every request whose canonical path (see CanonicalPath),
// lower-cased, contains Pattern (also lower-cased). Substring matching
// deliberately over-blocks: "logs/help-data" also blocks
// "logs/help-data;x" and "logs/help-data2".
type DenyRule struct {
	Pattern string
	Reason  string
}

// DefaultDenyRules lists the endpoints that every Client refuses. Each one
// was observed to crash or wedge the MegaRAC web backend, or to be unsafe to
// expose (docs/x470/live-probe.md §5.2/§10.2, docs/x470/endpoint-behaviour.json).
//
// Do not modify this slice at runtime; add board-specific rules with
// Config.ExtraDeny or (*Denylist).Add.
var DefaultDenyRules = []DenyRule{
	{"logs/help-data", "HTML 500 that crashes spx_restservice and invalidates all web sessions on the host"},
	{"asrr/mca-file", "502: the FastCGI backend goes away"},
	{"settings/download_image", "502: the FastCGI backend goes away (backend PID change observed)"},
	{"maintenance/download_config", "502: the FastCGI backend goes away"},
	{"maintenance/hpm/upgradestatus", "HTML 500 backend crasher"},
	{"hpm/verifyimagestatus", "HTML 500 backend crasher"},
	{"globalluatables", "Redfish GlobalLuaTables is unauthenticated and DELETE clears the Lua cache"},
}

// Denylist is a concurrency-safe, add-only set of DenyRules.
type Denylist struct {
	mu    sync.RWMutex
	rules []DenyRule
}

// NewDenylist returns a Denylist that holds DefaultDenyRules plus extra.
func NewDenylist(extra ...DenyRule) *Denylist {
	d := &Denylist{}
	d.Add(DefaultDenyRules...)
	d.Add(extra...)
	return d
}

// Add appends rules. Empty patterns are ignored. Rules cannot be removed.
func (d *Denylist) Add(rules ...DenyRule) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, r := range rules {
		p := strings.ToLower(strings.TrimSpace(r.Pattern))
		if p == "" {
			continue
		}
		d.rules = append(d.rules, DenyRule{Pattern: p, Reason: r.Reason})
	}
}

// Rules returns a copy of the rules.
func (d *Denylist) Rules() []DenyRule {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]DenyRule(nil), d.rules...)
}

// Match reports whether canonicalPath (from CanonicalPath) is denied. It
// lower-cases the path itself, so callers can pass any case.
func (d *Denylist) Match(canonicalPath string) (DenyRule, bool) {
	p := strings.ToLower(canonicalPath)
	d.mu.RLock()
	defer d.mu.RUnlock()
	for _, r := range d.rules {
		if strings.Contains(p, r.Pattern) {
			return r, true
		}
	}
	return DenyRule{}, false
}

// CanonicalPath normalises a caller-supplied /api path. It accepts
// "/api/x", "api/x" or "x", which all become "/api/x". The steps are:
//
//   - split off any "?query" (returned raw)
//   - percent-decode once; reject any '%' that is left, plus backslashes and
//     control characters
//   - clean "..", "." and "//" against a rooted path, so the result cannot
//     leave /api/
//   - prefix "/api" when the path is not already under it
//
// The returned path is decoded. Build the outgoing URL from it (see
// escapePath) so that the path that was matched is the path that is sent.
func CanonicalPath(p string) (canonical, rawQuery string, err error) {
	if strings.Contains(p, "://") {
		return "", "", fmt.Errorf("absolute URLs are not allowed: %q", p)
	}
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p, rawQuery = p[:i], p[i+1:]
	}
	if i := strings.IndexByte(p, '#'); i >= 0 {
		p = p[:i]
	}
	dec, err := url.PathUnescape(p)
	if err != nil {
		return "", "", fmt.Errorf("bad path escape: %w", err)
	}
	for _, r := range dec {
		if r < 0x20 || r == 0x7f || r == '\\' || r == '%' {
			return "", "", fmt.Errorf("illegal character %q in path", r)
		}
	}
	c := path.Clean("/" + dec)
	if c != "/api" && !strings.HasPrefix(c, "/api/") {
		c = path.Clean("/api" + c)
	}
	if c == "/api" {
		return "", "", fmt.Errorf("empty /api path")
	}
	if strings.ContainsAny(rawQuery, "\r\n #") {
		return "", "", fmt.Errorf("illegal character in query")
	}
	return c, rawQuery, nil
}

// escapePath percent-encodes a canonical (decoded) path for the wire.
func escapePath(canonical string) string {
	return (&url.URL{Path: canonical}).EscapedPath()
}
