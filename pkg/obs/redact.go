package obs

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
)

// Redacted replaces secret values in logs and HAR files.
const Redacted = "[REDACTED]"

// sensitiveHeaders are matched case-insensitively against header names and
// log attribute keys.
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-csrftoken":         true,
	"x-csrf-token":        true,
	"x-auth-token":        true,
	"x-bmc-password":      true,
	"qsessionid":          true,
}

// sensitiveKeyParts: a key containing any of these (case-insensitive,
// ignoring '_' and '-') is treated as secret.
var sensitiveKeyParts = []string{"password", "passwd", "passphrase", "secret", "token", "csrf", "sessionid", "cookie", "authorization", "privatekey", "apikey"}

// sensitiveExactKeys are short keys that only count on an exact match.
var sensitiveExactKeys = map[string]bool{"pass": true, "pwd": true, "pw": true, "auth": true, "creds": true}

// IsSensitiveKey reports whether a header name, JSON key, form field or log
// attribute key names a secret (Authorization, Cookie, X-CSRFTOKEN,
// QSESSIONID, password, token, ...).
func IsSensitiveKey(k string) bool {
	lk := strings.ToLower(k)
	if sensitiveHeaders[lk] || sensitiveExactKeys[lk] {
		return true
	}
	norm := strings.NewReplacer("_", "", "-", "", ".", "").Replace(lk)
	for _, p := range sensitiveKeyParts {
		if strings.Contains(norm, p) {
			return true
		}
	}
	return false
}

// RedactHeader returns a copy of h with sensitive header values replaced.
// Cookie/Set-Cookie keep cookie names so traces stay useful.
func RedactHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	out := make(http.Header, len(h))
	for k, vs := range h {
		if IsSensitiveKey(k) {
			nv := make([]string, len(vs))
			for i, v := range vs {
				nv[i] = redactHeaderValue(k, v)
			}
			out[k] = nv
			continue
		}
		out[k] = append([]string(nil), vs...)
	}
	return out
}

func redactHeaderValue(name, v string) string {
	switch strings.ToLower(name) {
	case "authorization", "proxy-authorization":
		// keep the scheme ("Basic", "Bearer") for debugging
		if scheme, _, ok := strings.Cut(v, " "); ok {
			return scheme + " " + Redacted
		}
		return Redacted
	case "cookie":
		parts := strings.Split(v, ";")
		for i, p := range parts {
			if name, _, ok := strings.Cut(p, "="); ok {
				parts[i] = name + "=" + Redacted
			}
		}
		return strings.Join(parts, ";")
	case "set-cookie":
		name, rest, ok := strings.Cut(v, "=")
		if !ok {
			return Redacted
		}
		attrs := ""
		if _, a, ok := strings.Cut(rest, ";"); ok {
			attrs = ";" + a
		}
		return name + "=" + Redacted + attrs
	}
	return Redacted
}

var (
	// Header lines inside free text dumps: "Authorization: Basic xyz"
	reHeaderLine = regexp.MustCompile(`(?im)^(\s*(?:proxy-)?authorization|\s*cookie|\s*set-cookie|\s*x-csrf-?token|\s*x-auth-token)(\s*:\s*)([^\r\n]*)`)
	// JSON "key": "value" or "key": 123 where key is sensitive (checked in func)
	reJSONKV = regexp.MustCompile(`"([^"\\]{1,64})"(\s*:\s*)("(?:[^"\\]|\\.)*"|-?[0-9][0-9.eE+-]*|true|false)`)
	// form / query / cookie pairs: key=value
	reFormKV = regexp.MustCompile(`(^|[?&;\s,])([A-Za-z0-9_.\-]{1,64})=([^&;\s,"]*)`)
	// URL userinfo: scheme://user:pass@
	reUserinfo = regexp.MustCompile(`(://[^/:@\s]*):([^/@\s]*)@`)
	// bare scheme tokens: "Basic dXNlcjpwYXNz", "Bearer abc"
	reAuthScheme = regexp.MustCompile(`(?i)\b(Basic|Bearer)\s+[A-Za-z0-9+/=._~-]{6,}`)
)

// quickSensitive is a cheap pre-filter for RedactString.
var quickSensitive = []string{"pass", "auth", "cookie", "token", "csrf", "session", "secret", "basic", "bearer", "pwd", "@"}

// RedactString masks secrets in free text: HTTP header lines, JSON
// key/value pairs with sensitive keys, form/query/cookie pairs, URL
// userinfo, and Basic/Bearer tokens. It is used on log string attributes and
// on HAR bodies.
func RedactString(s string) string {
	ls := strings.ToLower(s)
	hit := false
	for _, q := range quickSensitive {
		if strings.Contains(ls, q) {
			hit = true
			break
		}
	}
	if !hit {
		return s
	}
	s = reHeaderLine.ReplaceAllString(s, "${1}${2}"+Redacted)
	s = reJSONKV.ReplaceAllStringFunc(s, func(m string) string {
		sub := reJSONKV.FindStringSubmatch(m)
		if !IsSensitiveKey(sub[1]) {
			return m
		}
		return `"` + sub[1] + `"` + sub[2] + `"` + Redacted + `"`
	})
	s = reFormKV.ReplaceAllStringFunc(s, func(m string) string {
		sub := reFormKV.FindStringSubmatch(m)
		if !IsSensitiveKey(sub[2]) || sub[3] == "" {
			return m
		}
		return sub[1] + sub[2] + "=" + Redacted
	})
	s = reUserinfo.ReplaceAllString(s, "${1}:"+Redacted+"@")
	s = reAuthScheme.ReplaceAllString(s, "${1} "+Redacted)
	return s
}

// RedactBytes is RedactString for byte slices.
func RedactBytes(b []byte) []byte {
	r := RedactString(string(b))
	if r == string(b) {
		return b
	}
	return []byte(r)
}

// redactingHandler wraps another handler and masks secrets in every attr.
type redactingHandler struct {
	next slog.Handler
}

// NewRedactingHandler wraps next so that every attribute (including those
// added via With/WithGroup and nested groups) is redacted: attrs with a
// sensitive key get Redacted, http.Header values are passed through
// RedactHeader, and string values through RedactString. If enabled is false
// next is returned unchanged (this is the --log-secrets mode).
func NewRedactingHandler(next slog.Handler, enabled bool) slog.Handler {
	if !enabled {
		return next
	}
	return &redactingHandler{next: next}
}

func (h *redactingHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.next.Enabled(ctx, l)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, RedactString(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		nr.AddAttrs(RedactAttr(a))
		return true
	})
	return h.next.Handle(ctx, nr)
}

func (h *redactingHandler) WithAttrs(as []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(as))
	for i, a := range as {
		out[i] = RedactAttr(a)
	}
	return &redactingHandler{next: h.next.WithAttrs(out)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{next: h.next.WithGroup(name)}
}

// RedactAttr returns a redacted copy of a (recursing into groups and
// resolving LogValuers).
func RedactAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	if a.Key != "" && IsSensitiveKey(a.Key) && v.Kind() != slog.KindGroup {
		// header maps under a sensitive key still get structured redaction
		if hv, ok := v.Any().(http.Header); ok && v.Kind() == slog.KindAny {
			return slog.Any(a.Key, RedactHeader(hv))
		}
		return slog.String(a.Key, Redacted)
	}
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, RedactString(v.String()))
	case slog.KindGroup:
		gs := v.Group()
		out := make([]slog.Attr, len(gs))
		for i, g := range gs {
			out[i] = RedactAttr(g)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	case slog.KindAny:
		switch x := v.Any().(type) {
		case http.Header:
			return slog.Any(a.Key, RedactHeader(x))
		case []byte:
			return slog.String(a.Key, RedactString(string(x)))
		case error:
			return slog.String(a.Key, RedactString(x.Error()))
		case map[string]string:
			m := make(map[string]string, len(x))
			for k, s := range x {
				if IsSensitiveKey(k) {
					m[k] = Redacted
				} else {
					m[k] = RedactString(s)
				}
			}
			return slog.Any(a.Key, m)
		}
	}
	return slog.Attr{Key: a.Key, Value: v}
}
