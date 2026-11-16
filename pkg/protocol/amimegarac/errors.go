package amimegarac

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Pkg is the OpError.Pkg value for errors from this package.
const Pkg = "amimegarac"

// Well-known MegaRAC /api error codes (the "code" field of {"error","code"}
// bodies, or "cc" on auth failures).
const (
	CodeInvalidAPICall    = 1010  // 404: unknown route on this firmware
	CodeFeatureNotEnabled = 8500  // 405: plugin or feature absent
	CodeMaxSessions       = 15000 // login: MAX_NUM_SESSIONS_ALREADY_IN_USE
	CCInvalidAuth         = 7     // 401 {"cc":7,"error":"Invalid Authentication"}
)

// excerptLimit bounds body excerpts in error details.
const excerptLimit = 256

// APIError describes a non-2xx /api response. OpError.Err wraps it next to
// a bmc sentinel, so drivers can inspect it with errors.As (for example
// Code 11272, "no host inventory", returned while the host is off).
type APIError struct {
	Status      int    // HTTP status
	Code        int    // "code" field (0 if absent)
	CC          int    // "cc" field (0 if absent)
	HasCC       bool   // true when the body had a "cc" field
	Message     string // "error" field
	HTML        bool   // body was HTML/non-JSON (lighttpd error page)
	BodyExcerpt string // redacted, truncated body
}

func (e *APIError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP %d", e.Status)
	if e.HasCC {
		fmt.Fprintf(&b, " cc=%d", e.CC)
	}
	if e.Code != 0 {
		fmt.Fprintf(&b, " code=%d", e.Code)
	}
	if e.Message != "" {
		fmt.Fprintf(&b, " %q", e.Message)
	}
	if e.HTML {
		b.WriteString(" (html body)")
	}
	return b.String()
}

// ParseAPIError builds an APIError from a status and body.
func ParseAPIError(status int, header http.Header, body []byte) *APIError {
	e := &APIError{Status: status, BodyExcerpt: excerpt(body)}
	trim := bytes.TrimSpace(body)
	if len(trim) == 0 || trim[0] != '{' {
		ct := ""
		if header != nil {
			ct = header.Get("Content-Type")
		}
		e.HTML = len(trim) > 0 && (trim[0] == '<' || strings.Contains(ct, "html"))
		return e
	}
	var m struct {
		CC    *NullableInt `json:"cc"`
		Code  NullableInt  `json:"code"`
		Error FlexString   `json:"error"`
	}
	if json.Unmarshal(trim, &m) == nil {
		if m.CC != nil && m.CC.Valid {
			e.CC, e.HasCC = int(m.CC.V), true
		}
		e.Code = int(m.Code.V)
		e.Message = string(m.Error)
	}
	return e
}

// isAuthGateReject reports whether a 401 body is the AMI auth-gate
// rejection {"cc":7,...}. That body proves the request was refused before
// any handler ran.
func isAuthGateReject(body []byte) bool {
	e := ParseAPIError(http.StatusUnauthorized, nil, body)
	return e.HasCC && e.CC == CCInvalidAuth
}

// isBackendCrash reports responses that mean the FastCGI backend died or
// restarted: any 502, or a 500 with an HTML/non-JSON body.
func isBackendCrash(status int, header http.Header, body []byte) (bool, string) {
	switch {
	case status == http.StatusBadGateway:
		return true, "502 from lighttpd: FastCGI backend went away"
	case status == http.StatusInternalServerError:
		if e := ParseAPIError(status, header, body); e.HTML {
			return true, "HTML 500 from lighttpd: backend crash"
		}
	}
	return false, ""
}

// StatusError maps a non-2xx /api response to an *bmc.OpError. It returns
// nil for 2xx.
//
//	400                         -> ErrInvalidArgument
//	401                         -> ErrAuth
//	403                         -> ErrForbidden (BMC privilege check)
//	404 code 1010 (bad route)   -> ErrUnsupported
//	404 otherwise               -> ErrNotFound
//	405 (feature not enabled)   -> ErrUnsupported
//	503                         -> ErrBusy
//	other 4xx, 5xx              -> ErrUpstream (detail: cc/code + body excerpt)
func StatusError(op string, status int, header http.Header, body []byte) error {
	if status >= 200 && status < 300 {
		return nil
	}
	ae := ParseAPIError(status, header, body)
	var sentinel error
	switch {
	case status == http.StatusBadRequest:
		sentinel = bmc.ErrInvalidArgument
	case status == http.StatusUnauthorized:
		sentinel = bmc.ErrAuth
	case status == http.StatusForbidden:
		sentinel = bmc.ErrForbidden
	case status == http.StatusNotFound && (ae.Code == CodeInvalidAPICall || ae.Code == 0 && !ae.HasCC && ae.Message == ""):
		sentinel = bmc.ErrUnsupported
	case status == http.StatusNotFound:
		sentinel = bmc.ErrNotFound
	case status == http.StatusMethodNotAllowed:
		sentinel = bmc.ErrUnsupported
	case status == http.StatusServiceUnavailable:
		sentinel = bmc.ErrBusy
	default:
		sentinel = bmc.ErrUpstream
	}
	detail := ae.Error()
	if ae.BodyExcerpt != "" {
		detail += ": " + ae.BodyExcerpt
	}
	return &bmc.OpError{
		Pkg: Pkg, Op: op, Backend: bmc.BackendAMI,
		UpstreamStatus: status, Detail: detail,
		Err: fmt.Errorf("%w: %w", sentinel, ae),
	}
}

func excerpt(b []byte) string {
	b = bytes.TrimSpace(b)
	if len(b) > excerptLimit {
		b = append(append([]byte(nil), b[:excerptLimit]...), "..."...)
	}
	return obs.RedactString(string(b))
}

func opErr(op string, status int, sentinel error, format string, args ...any) *bmc.OpError {
	return &bmc.OpError{
		Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, UpstreamStatus: status,
		Detail: fmt.Sprintf(format, args...), Err: sentinel,
	}
}
