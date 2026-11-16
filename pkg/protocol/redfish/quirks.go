package redfish

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
)

// Quirks are per-board/firmware deviations a driver's board profile fills
// in. The zero value is standards behaviour.
type Quirks struct {
	// StripQuery never sends client query options ($select, $top, ...),
	// whatever the ServiceRoot advertises. Server-generated nextLinks are
	// still followed verbatim.
	StripQuery bool
	// QueryCaps, when non-nil, replaces the capabilities derived from
	// ProtocolFeaturesSupported (e.g. B650 4.10 honours $top/$skip without
	// advertising TopSkipQuery).
	QueryCaps *QueryCaps
	// NullStrings are string values treated as JSON null when decoding into
	// typed structs (e.g. "NA", "N/A"). Matching is case-insensitive and
	// whitespace-trimmed. See NAAsNull.
	NullStrings []string
	// NAAsNull is shorthand for NullStrings = {"NA", "N/A"}.
	NAAsNull bool
	// PathOverrides maps a path the client would request to the one the
	// board actually serves (exact match after trimming a trailing slash),
	// or a logical key ("system", "manager", "chassis", "sel", "auditlog",
	// "updateservice", "accountservice", "sessions") to a path used instead
	// of discovery.
	PathOverrides map[string]string
	// NoPathOverride disables PathOverrides for raw passthrough clients.
	NoPathOverride bool
	// NoIfMatch never sends an automatic If-Match on PATCH.
	NoIfMatch bool
	// RequireIfMatch GETs the resource for its ETag before a PATCH when none
	// is cached (services that answer 428 to unconditional writes).
	RequireIfMatch bool
	// NoRedirects disables following same-host GET redirects.
	NoRedirects bool
	// NoExpand never uses $expand for member collection, even if advertised
	// (expensive on single-threaded servers).
	NoExpand bool
	// ResetTypes overrides the allowable ResetType values for an action
	// name ("#ComputerSystem.Reset", "#Manager.Reset") when the service
	// does not advertise them.
	ResetTypes map[string][]string
	// SkipPageSize, when > 0, continues a collection walk with $skip when a
	// page carries no Members@odata.nextLink but holds exactly SkipPageSize
	// members. Gigabyte MC13-LE0 (AMI 13.04.15) truncates collections at 50
	// members without a nextLink, reports Members@odata.count as the page
	// length and answers 400 to $skip=0 (docs/gigabyte/mc13-le0-live-probe.md).
	// The walk stops at the first short or empty page (or MaxPages).
	SkipPageSize int
}

// Logical path keys usable in Quirks.PathOverrides.
const (
	PathSystem         = "system"
	PathManager        = "manager"
	PathChassis        = "chassis"
	PathSEL            = "sel"
	PathAuditLog       = "auditlog"
	PathUpdateService  = "updateservice"
	PathAccountService = "accountservice"
	PathSessions       = "sessions"
	PathServiceRoot    = "serviceroot"
)

// LogicalPath returns the override for a logical key, or "".
func (q Quirks) LogicalPath(key string) string {
	if q.PathOverrides == nil {
		return ""
	}
	return q.PathOverrides[key]
}

func (q Quirks) overridePath(p string) string {
	if len(q.PathOverrides) == 0 || !strings.HasPrefix(p, "/") {
		return p
	}
	path, query := p, ""
	if i := strings.IndexByte(p, '?'); i >= 0 {
		path, query = p[:i], p[i:]
	}
	key := path
	if len(key) > 1 {
		key = strings.TrimRight(key, "/")
	}
	if v, ok := q.PathOverrides[key]; ok && strings.HasPrefix(v, "/") {
		return v + query
	}
	return p
}

func (q Quirks) nullStrings() []string {
	ns := q.NullStrings
	if q.NAAsNull {
		ns = append(append([]string{}, ns...), "NA", "N/A")
	}
	return ns
}

// decodeLenient decodes JSON into out:
//   - unknown fields are ignored;
//   - a value whose JSON type doesn't match the Go type (e.g. "NA" where a
//     number is expected) is treated as null and the rest of the object
//     still decodes, instead of failing the whole call;
//   - strings listed in the quirks' null set become null.
//
// Only syntactically invalid JSON is an error.
func decodeLenient(data []byte, out any, q Quirks) error {
	if out == nil {
		return nil
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil
	}
	if raw, ok := out.(*json.RawMessage); ok {
		*raw = append((*raw)[:0], data...)
		return nil
	}
	ns := q.nullStrings()
	if len(ns) == 0 {
		err := json.Unmarshal(data, out)
		var te *json.UnmarshalTypeError
		if !errors.As(err, &te) {
			return err
		}
	}
	// Slow path: work on a generic tree, null the offending values, retry.
	var v any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}
	if len(ns) > 0 {
		v = nullify(v, ns)
	}
	for i := 0; i < 64; i++ {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		resetValue(out)
		err = json.Unmarshal(b, out)
		var te *json.UnmarshalTypeError
		if !errors.As(err, &te) {
			return err
		}
		if te.Field == "" || !nullPath(v, te.Field) {
			return nil // can't locate it: keep the partial decode
		}
	}
	return nil
}

// resetValue zeroes *out so a retry doesn't keep values from a failed pass.
func resetValue(out any) {
	rv := reflect.ValueOf(out)
	if rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv.Elem().Set(reflect.Zero(rv.Elem().Type()))
	}
}

// nullPath sets the value at a json.UnmarshalTypeError.Field path
// ("A.0.B.C"; keys may themselves contain dots) to null.
func nullPath(v any, path string) bool {
	switch t := v.(type) {
	case map[string]any:
		// longest key that prefixes the path
		best := ""
		for k := range t {
			if (path == k || strings.HasPrefix(path, k+".")) && len(k) > len(best) {
				best = k
			}
		}
		if best == "" {
			return false
		}
		if path == best {
			if t[best] == nil {
				return false
			}
			t[best] = nil
			return true
		}
		return nullPath(t[best], path[len(best)+1:])
	case []any:
		idx, rest, _ := strings.Cut(path, ".")
		n, err := strconv.Atoi(idx)
		if err != nil || n < 0 || n >= len(t) {
			return false
		}
		if rest == "" {
			if t[n] == nil {
				return false
			}
			t[n] = nil
			return true
		}
		return nullPath(t[n], rest)
	}
	return false
}

func nullify(v any, ns []string) any {
	switch t := v.(type) {
	case map[string]any:
		for k, e := range t {
			t[k] = nullify(e, ns)
		}
		return t
	case []any:
		for i, e := range t {
			t[i] = nullify(e, ns)
		}
		return t
	case string:
		s := strings.TrimSpace(t)
		for _, n := range ns {
			if strings.EqualFold(s, n) {
				return nil
			}
		}
	}
	return v
}

// DecodeLenient is the exported lenient decoder (see Quirks).
func DecodeLenient(data []byte, out any, q Quirks) error { return decodeLenient(data, out, q) }
