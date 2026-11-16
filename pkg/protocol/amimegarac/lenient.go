package amimegarac

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// IsNA reports the MegaRAC spellings of "no value": "", "NA", "N/A", "N\/A"
// (once JSON-unescaped), "nil", "null" and whitespace. The check ignores
// case.
func IsNA(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "na", "n/a", "nil", "null":
		return true
	}
	return false
}

// CleanString trims whitespace and the literal two-character `\n` suffix
// that FRU and certificate dates carry ("Thu Jan 13 06:21:00 2022\n").
// It returns "" for IsNA values.
func CleanString(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasSuffix(s, `\n`) {
		s = strings.TrimSpace(strings.TrimSuffix(s, `\n`))
	}
	if IsNA(s) {
		return ""
	}
	return s
}

// ParseFloat converts a loosely typed JSON value (float64, json.Number,
// string, int, bool) to a float. ok is false for "NA"-style values, nil,
// NaN/Inf, and anything it cannot parse.
func ParseFloat(v any) (f float64, ok bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case float64:
		f = x
	case float32:
		f = float64(x)
	case int:
		f = float64(x)
	case int64:
		f = float64(x)
	case json.Number:
		var err error
		if f, err = x.Float64(); err != nil {
			return 0, false
		}
	case string:
		if IsNA(x) {
			return 0, false
		}
		var err error
		if f, err = strconv.ParseFloat(strings.TrimSpace(x), 64); err != nil {
			return 0, false
		}
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// NullableFloat accepts a JSON number, a numeric string, or an NA marker
// ("NA", "N/A", "", null). NA markers decode to Valid=false. It never fails
// on an unparseable string: that also gives Valid=false. Use it for sensor
// readings and thresholds.
type NullableFloat struct {
	V     float64
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullableFloat) UnmarshalJSON(b []byte) error {
	*n = NullableFloat{}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		n.V, n.Valid = ParseFloat(s)
		return nil
	}
	if b[0] == 't' || b[0] == 'f' {
		var x bool
		if err := json.Unmarshal(b, &x); err != nil {
			return err
		}
		n.V, n.Valid = ParseFloat(x)
		return nil
	}
	if b[0] == '{' || b[0] == '[' {
		return nil // tolerate garbage shapes
	}
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return fmt.Errorf("amimegarac: NullableFloat: %w", err)
	}
	n.V, n.Valid = f, !math.IsNaN(f) && !math.IsInf(f, 0)
	return nil
}

// MarshalJSON writes a number or null.
func (n NullableFloat) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(n.V)
}

// Ptr returns &V, or nil when the value is not valid (the bmc.Thresholds
// convention).
func (n NullableFloat) Ptr() *float64 {
	if !n.Valid {
		return nil
	}
	v := n.V
	return &v
}

// Or returns V, or def when the value is not valid.
func (n NullableFloat) Or(def float64) float64 {
	if !n.Valid {
		return def
	}
	return n.V
}

// NullableInt is the integer form of NullableFloat. It accepts 3, 3.0, "3"
// and "0x1F". NA markers decode to Valid=false. A non-integral number is
// truncated.
type NullableInt struct {
	V     int64
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler.
func (n *NullableInt) UnmarshalJSON(b []byte) error {
	*n = NullableInt{}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if IsNA(s) {
			return nil
		}
		if i, err := strconv.ParseInt(s, 0, 64); err == nil {
			n.V, n.Valid = i, true
			return nil
		}
		if f, ok := ParseFloat(s); ok {
			n.V, n.Valid = int64(f), true
		}
		return nil
	}
	if b[0] == 't' || b[0] == 'f' {
		n.V, n.Valid = 0, true
		if b[0] == 't' {
			n.V = 1
		}
		return nil
	}
	if b[0] == '{' || b[0] == '[' {
		return nil
	}
	if i, err := strconv.ParseInt(string(b), 10, 64); err == nil {
		n.V, n.Valid = i, true
		return nil
	}
	f, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return fmt.Errorf("amimegarac: NullableInt: %w", err)
	}
	n.V, n.Valid = int64(f), true
	return nil
}

// MarshalJSON writes a number or null.
func (n NullableInt) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}
	return strconv.AppendInt(nil, n.V, 10), nil
}

// Ptr returns &V, or nil when the value is not valid.
func (n NullableInt) Ptr() *int64 {
	if !n.Valid {
		return nil
	}
	v := n.V
	return &v
}

// Int returns V as an int, or 0 when the value is not valid.
func (n NullableInt) Int() int { return int(n.V) }

// FlexString accepts a string, number, bool or null and keeps the text
// form. Numbers keep their JSON spelling, and null becomes "". It does not
// clean the value; call CleanString for that.
type FlexString string

// UnmarshalJSON implements json.Unmarshaler.
func (s *FlexString) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	switch {
	case len(b) == 0 || bytes.Equal(b, []byte("null")):
		*s = ""
	case b[0] == '"':
		var x string
		if err := json.Unmarshal(b, &x); err != nil {
			return err
		}
		*s = FlexString(x)
	case b[0] == '{' || b[0] == '[':
		*s = FlexString(b)
	default:
		*s = FlexString(b) // number or bool literal
	}
	return nil
}

// String returns the raw text.
func (s FlexString) String() string { return string(s) }

// Clean returns CleanString(s).
func (s FlexString) Clean() string { return CleanString(string(s)) }

// FlexBool accepts true/false, 0/1 (any non-zero number is true), and the
// strings "0"/"1"/"true"/"false"/"yes"/"no"/"on"/"off"/"enabled"/"disabled".
// null and NA leave it false.
type FlexBool bool

// UnmarshalJSON implements json.Unmarshaler.
func (fb *FlexBool) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	*fb = false
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		return nil
	}
	var s string
	if b[0] == '"' {
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
	} else {
		s = string(b)
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "on", "enabled", "enable":
		*fb = true
		return nil
	case "false", "no", "off", "disabled", "disable":
		return nil
	}
	if f, ok := ParseFloat(s); ok {
		*fb = f != 0
	}
	return nil
}

// DecodePort decodes a MegaRAC service port. Port fields with bit 31 set
// (X470 single-port mode: -2147476070 = 0x80000000|7578) or set to -1 (B650)
// mean "disabled". It returns the port number, if known, and whether the
// port is disabled.
func DecodePort(v int64) (port int, disabled bool) {
	if v == -1 {
		return 0, true
	}
	u := uint32(v)
	if u&0x80000000 != 0 {
		return int(u &^ 0x80000000), true
	}
	return int(u), false
}

// Decode unmarshals body into dst. Unknown fields are ignored and missing
// fields keep their zero value. An empty body, or a nil dst, is a no-op.
// Use the Nullable and Flex types in dst for loosely typed fields.
func Decode(body []byte, dst any) error {
	if dst == nil {
		return nil
	}
	b := bytes.TrimSpace(body)
	if len(b) == 0 {
		return nil
	}
	if raw, ok := dst.(*[]byte); ok {
		*raw = append((*raw)[:0], body...)
		return nil
	}
	return json.Unmarshal(b, dst)
}
