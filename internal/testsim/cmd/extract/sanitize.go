package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/netip"
	"regexp"
	"slices"
	"strings"
)

// Sanitizer rewrites secrets and identifying values. Private IPv4 addresses
// are mapped to 192.0.2.N in first-seen order, so one Sanitizer per output
// file keeps a stable mapping.
type Sanitizer struct {
	ips map[string]string
}

// NewSanitizer returns a Sanitizer with an empty address map.
func NewSanitizer() *Sanitizer { return &Sanitizer{ips: map[string]string{}} }

// sensitiveKeys maps a key pattern to its placeholder. Only string values
// are replaced (numbers such as active_session counts stay).
var sensitiveKeys = []struct {
	re          *regexp.Regexp
	placeholder string
}{
	{regexp.MustCompile(`(?i)pass(word|wd)?$|^pwd$|password`), "<REDACTED_PASSWORD>"},
	{regexp.MustCompile(`(?i)csrf`), "<REDACTED_CSRF>"},
	{regexp.MustCompile(`(?i)token`), "<REDACTED_TOKEN>"},
	{regexp.MustCompile(`(?i)secret|private_?key`), "<REDACTED_SECRET>"},
	{regexp.MustCompile(`(?i)racsession_id|session_?id|qsessionid`), "<REDACTED_SESSION_ID>"},
	{regexp.MustCompile(`(?i)serial_?num|serialnumber|^serial$|^sn$|_sn$|asset_?tag`), "<REDACTED_SERIAL>"},
	{regexp.MustCompile(`(?i)mac_?addr|^mac$|_mac$|permanentmac`), "<REDACTED_MAC>"},
	{regexp.MustCompile(`(?i)uuid|guid`), "<REDACTED_UUID>"},
	{regexp.MustCompile(`(?i)^host_?name$|^hostname$`), "<REDACTED_HOSTNAME>"},
	{regexp.MustCompile(`(?i)e?mail(_id|_address)?$`), "<REDACTED_EMAIL>"},
}

// benign values are never treated as secrets (unset fields).
var benign = map[string]bool{
	"": true, " ": true, "na": true, "n/a": true, "none": true, "null": true, "not available": true,
	"to be filled by o.e.m.": true, "default string": true, "unknown": true, "0": true,
}

var (
	ipv4Re  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	macRe   = regexp.MustCompile(`\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b`)
	uuidRe  = regexp.MustCompile(`\b[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\b`)
	emailRe = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	// realIPs are the two live BMC addresses: never allowed through even
	// if a future capture embeds them in an unusual form.
	realIPs = []string{"10.3.10.207", "10.7.12.55"}
)

// Value sanitizes v (decoded JSON with json.Number) found under key.
func (s *Sanitizer) Value(key string, v any) any {
	switch x := v.(type) {
	case map[string]any:
		// Sorted traversal keeps the address mapping deterministic.
		out := make(map[string]any, len(x))
		for _, k := range slices.Sorted(maps.Keys(x)) {
			out[k] = s.Value(k, x[k])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = s.Value(key, vv)
		}
		return out
	case string:
		if p := placeholderFor(key, x); p != "" {
			return p
		}
		return s.String(x)
	case json.Number, bool, nil:
		return x
	default:
		return x
	}
}

func placeholderFor(key, val string) string {
	if key == "" || benign[strings.ToLower(strings.TrimSpace(val))] || strings.HasPrefix(val, "<REDACTED") ||
		strings.HasPrefix(strings.TrimSpace(val), "0123456789") { // SMBIOS/FRU placeholder, not identifying
		return ""
	}
	for _, sk := range sensitiveKeys {
		if sk.re.MatchString(key) {
			return sk.placeholder
		}
	}
	return ""
}

// String sanitizes free text: MACs, UUIDs, e-mails and private IPv4s.
func (s *Sanitizer) String(v string) string {
	v = macRe.ReplaceAllString(v, "<REDACTED_MAC>")
	v = uuidRe.ReplaceAllString(v, "<REDACTED_UUID>")
	v = emailRe.ReplaceAllString(v, "<REDACTED_EMAIL>")
	v = ipv4Re.ReplaceAllStringFunc(v, s.ip)
	for _, r := range realIPs {
		v = strings.ReplaceAll(v, r, "<BMC_HOST>")
	}
	return v
}

func (s *Sanitizer) ip(v string) string {
	a, err := netip.ParseAddr(v)
	if err != nil || !a.Is4() {
		return v
	}
	if !a.IsPrivate() {
		// Public, loopback, masks (255.x), 0.0.0.0 and the USB
		// host-interface 169.254.x firmware defaults are not identifying.
		return v
	}
	if m, ok := s.ips[v]; ok {
		return m
	}
	m := fmt.Sprintf("192.0.2.%d", len(s.ips)+10)
	s.ips[v] = m
	return m
}
