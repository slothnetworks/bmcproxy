package bmc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Sentinel errors. Every error returned across a package boundary should wrap
// one of these (usually inside an *OpError) so the northbound server can map
// it to an HTTP status with HTTPStatus and a class with Class.
var (
	// ErrUnsupported: the capability is not supported by this BMC, board,
	// firmware or driver. -> 501
	ErrUnsupported = errors.New("capability not supported")
	// ErrAuth: the BMC rejected the credentials. -> 401
	ErrAuth = errors.New("BMC rejected credentials")
	// ErrBusy: the BMC is busy, connection-capped (B650 mod_evasive 403),
	// returned 503, or its circuit breaker is open. -> 503
	ErrBusy = errors.New("BMC busy")
	// ErrTimeout: the BMC did not answer in time. -> 504
	ErrTimeout = errors.New("BMC timeout")
	// ErrUpstream: the BMC returned an error or an undecodable response, or
	// could not be reached. -> 502
	ErrUpstream = errors.New("BMC error")
	// ErrDisruptiveDenied: a disruptive operation was requested without
	// --allow-disruptive and X-BMC-Confirm: true. -> 403
	ErrDisruptiveDenied = errors.New("disruptive operation denied")
	// ErrForbidden: the operation/endpoint is blocked by proxy policy (e.g.
	// the logs/help-data denylist, X-HTTP-Method-Override). -> 403
	ErrForbidden = errors.New("operation forbidden by proxy policy")
	// ErrInvalidArgument: the northbound request is malformed. -> 400
	ErrInvalidArgument = errors.New("invalid argument")
	// ErrNotFound: the addressed object (sensor, user id, slot) does not
	// exist. -> 404
	ErrNotFound = errors.New("not found")
	// ErrSessionLost: the BMC backend restarted (or otherwise dropped every
	// session on the host, e.g. after the logs/help-data crash or a 502).
	// It wraps ErrUpstream so it still maps to 502, but the session cache
	// reacts by invalidating ALL sessions for the host and retrying once.
	ErrSessionLost = fmt.Errorf("%w: BMC backend restarted, all sessions lost", ErrUpstream)
)

// OpError carries context for a failed operation. It wraps one of the
// sentinels (Err) so errors.Is works through it.
type OpError struct {
	Pkg            string // originating package, e.g. "amimegarac", "redfish", "driver.asrock"
	Op             string // operation/capability, e.g. "PowerState", "GET /api/chassis-status"
	Backend        string // backend tried: "redfish", "ami", "ipmi" (may be empty)
	UpstreamStatus int    // raw upstream HTTP status (0 if none)
	Detail         string // vendor detail, e.g. `{"cc":7}` or a decode error
	Err            error  // a sentinel, possibly wrapping a cause (use fmt.Errorf("%w: %w", ErrX, cause))
}

func (e *OpError) Error() string {
	var b strings.Builder
	if e.Pkg != "" {
		b.WriteString(e.Pkg)
	}
	if e.Op != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(e.Op)
	}
	if e.Backend != "" {
		fmt.Fprintf(&b, " [%s]", e.Backend)
	}
	if b.Len() > 0 {
		b.WriteString(": ")
	}
	if e.Err != nil {
		b.WriteString(e.Err.Error())
	} else {
		b.WriteString("error")
	}
	if e.UpstreamStatus != 0 {
		fmt.Fprintf(&b, " (upstream %d)", e.UpstreamStatus)
	}
	if e.Detail != "" {
		b.WriteString(": ")
		b.WriteString(e.Detail)
	}
	return b.String()
}

func (e *OpError) Unwrap() error { return e.Err }

// Errorf builds an *OpError wrapping sentinel with a formatted Detail.
func Errorf(sentinel error, pkg, op, format string, args ...any) *OpError {
	return &OpError{Pkg: pkg, Op: op, Detail: fmt.Sprintf(format, args...), Err: sentinel}
}

// Wrap returns nil for a nil err; otherwise an *OpError{Pkg, Op, Err: err}.
// If err does not already wrap a known sentinel it is classified first
// (timeouts -> ErrTimeout, network errors -> ErrUpstream) so that HTTPStatus
// gives a sensible answer.
func Wrap(pkg, op string, err error) error {
	if err == nil {
		return nil
	}
	if !hasSentinel(err) {
		err = fmt.Errorf("%w: %w", classify(err), err)
	}
	return &OpError{Pkg: pkg, Op: op, Err: err}
}

// Unsupported is shorthand for an *OpError wrapping ErrUnsupported.
func Unsupported(pkg, op string) *OpError {
	return &OpError{Pkg: pkg, Op: op, Err: ErrUnsupported}
}

var allSentinels = []error{ErrDisruptiveDenied, ErrForbidden, ErrUnsupported, ErrAuth, ErrInvalidArgument, ErrNotFound, ErrBusy, ErrTimeout, ErrUpstream}

func hasSentinel(err error) bool {
	for _, s := range allSentinels {
		if errors.Is(err, s) {
			return true
		}
	}
	return false
}

func classify(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return ErrTimeout
	}
	return ErrUpstream
}

// StatusClientClosed is returned by HTTPStatus for context.Canceled (the
// northbound client went away); nginx convention.
const StatusClientClosed = 499

// HTTPStatus maps err to the northbound HTTP status per the PLAN error table:
//
//	nil                    200
//	ErrInvalidArgument     400
//	ErrAuth                401
//	ErrDisruptiveDenied    403
//	ErrForbidden           403
//	ErrNotFound            404
//	ErrUnsupported         501
//	ErrUpstream            502
//	ErrBusy                503
//	ErrTimeout             504  (also context.DeadlineExceeded, net timeouts)
//	context.Canceled       499
//	anything else          500
//
// When an error wraps several sentinels the first match in the order
// DisruptiveDenied, Forbidden, Unsupported, Auth, InvalidArgument, NotFound,
// Busy, Timeout, Upstream wins.
func HTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrDisruptiveDenied), errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrUnsupported):
		return http.StatusNotImplemented
	case errors.Is(err, ErrAuth):
		return http.StatusUnauthorized
	case errors.Is(err, ErrInvalidArgument):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrBusy):
		return http.StatusServiceUnavailable
	case errors.Is(err, ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return http.StatusGatewayTimeout
	case errors.Is(err, ErrUpstream):
		return http.StatusBadGateway
	case errors.Is(err, context.Canceled):
		return StatusClientClosed
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return http.StatusGatewayTimeout
	}
	return http.StatusInternalServerError
}

// Class returns a short stable error class for envelopes and metrics labels:
// "" (nil), "invalid_argument", "auth", "disruptive_denied", "forbidden",
// "not_found", "unsupported", "upstream", "busy", "timeout", "canceled",
// "internal".
func Class(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrDisruptiveDenied):
		return "disruptive_denied"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	}
	switch HTTPStatus(err) {
	case 400:
		return "invalid_argument"
	case 401:
		return "auth"
	case 404:
		return "not_found"
	case 501:
		return "unsupported"
	case 502:
		return "upstream"
	case 503:
		return "busy"
	case 504:
		return "timeout"
	case StatusClientClosed:
		return "canceled"
	}
	return "internal"
}

// AsOpError returns the outermost *OpError in err's chain, if any.
func AsOpError(err error) (*OpError, bool) {
	var oe *OpError
	ok := errors.As(err, &oe)
	return oe, ok
}
