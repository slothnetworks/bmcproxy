package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// ErrorBody is the JSON error envelope: {"error": {...}}.
type ErrorBody struct {
	Error ErrorInfo `json:"error"`
}

// ErrorInfo is the envelope's error object.
type ErrorInfo struct {
	// HTTP status (mirrors the response status).
	Status int `json:"status" example:"501"`
	// Stable error class for programmatic handling.
	Class string `json:"class" example:"unsupported" enums:"invalid_argument,auth,disruptive_denied,forbidden,not_found,unsupported,upstream,busy,timeout,canceled,internal"`
	// Human-readable message naming the originating package and operation (redacted).
	Message string `json:"message" example:"api.v1 BootOverride: capability not supported"`
	// The request id (also the X-Request-Id response header).
	RequestID string `json:"request_id" example:"01J9Z3V8Q4K2M5N7P0R6S8T1W3"`
	// Present only in debug mode (--debug-endpoints or north=debug).
	Debug *ErrorDebug `json:"debug,omitempty"`
}

// ErrorDebug is present only in debug mode.
type ErrorDebug struct {
	// Southbound backend tried.
	Backend string `json:"backend,omitempty" example:"ami"`
	// Raw upstream HTTP status.
	UpstreamStatus int `json:"upstream_status,omitempty" example:"401"`
	// Vendor detail or decode error (redacted).
	Detail string `json:"detail,omitempty" example:"{\"cc\":7}"`
	// Operation.
	Op string `json:"op,omitempty" example:"GET /api/chassis-status"`
	// Originating package.
	Pkg string `json:"pkg,omitempty" example:"amimegarac"`
}

// mergeOpErrors walks every *bmc.OpError in the chain, taking the first
// non-empty value of each field (outermost first).
func mergeOpErrors(err error) (ErrorDebug, bool) {
	var d ErrorDebug
	found := false
	for e := err; e != nil; {
		oe, ok := bmc.AsOpError(e)
		if !ok {
			break
		}
		found = true
		if d.Pkg == "" {
			d.Pkg = oe.Pkg
		}
		if d.Op == "" {
			d.Op = oe.Op
		}
		if d.Backend == "" {
			d.Backend = oe.Backend
		}
		if d.UpstreamStatus == 0 {
			d.UpstreamStatus = oe.UpstreamStatus
		}
		if d.Detail == "" {
			d.Detail = oe.Detail
		}
		e = oe.Err
	}
	return d, found
}

// ErrorMessage renders err for the envelope: redacted always; the vendor
// Detail is only included in debug mode.
func ErrorMessage(err error, debug bool) string {
	if !debug {
		if oe, ok := bmc.AsOpError(err); ok {
			cp := *oe
			cp.Detail = ""
			return obs.RedactString(cp.Error())
		}
	}
	return obs.RedactString(err.Error())
}

// Envelope builds the error envelope for err.
func (s *Server) Envelope(r *http.Request, err error) ErrorBody {
	debug := s.DebugMode()
	info := ErrorInfo{
		Status:    bmc.HTTPStatus(err),
		Class:     bmc.Class(err),
		Message:   ErrorMessage(err, debug),
		RequestID: obs.RequestID(r.Context()),
	}
	if debug {
		if d, ok := mergeOpErrors(err); ok {
			d.Detail = obs.RedactString(d.Detail)
			info.Debug = &d
		}
	}
	return ErrorBody{Error: info}
}

// WriteError writes the JSON error envelope for err.
func (s *Server) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	body := s.Envelope(r, err)
	st := stateFrom(r.Context())
	st.class = body.Error.Class
	st.errMsg = ErrorMessage(err, false)
	writeJSON(w, body.Error.Status, body)
}

// WriteJSON writes v as JSON with status.
func WriteJSON(w http.ResponseWriter, status int, v any) { writeJSON(w, status, v) }

func writeJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		status = http.StatusInternalServerError
		b = []byte(`{"error":{"status":500,"class":"internal","message":"response encoding failed"}}`)
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(b)
	_, _ = w.Write([]byte("\n"))
}

// WriteResult encodes a handler result per the CONTRACTS HandlerFunc
// contract: error -> envelope; nil -> 204; bmc.Responder -> as-is; else 200
// JSON.
func (s *Server) WriteResult(w http.ResponseWriter, r *http.Request, v any, err error) {
	if err != nil {
		s.WriteError(w, r, err)
		return
	}
	if isNil(v) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if rs, ok := v.(bmc.Responder); ok {
		if err := rs.WriteResponse(w); err != nil {
			s.log(r.Context()).DebugContext(r.Context(), "responder write failed", "err", obs.RedactString(err.Error()))
		}
		return
	}
	if st, ok := v.(statusResult); ok {
		writeJSON(w, st.status, st.v)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// statusResult lets generic handlers pick a non-200 success status.
type statusResult struct {
	status int
	v      any
}

// WithStatus wraps a JSON result with an explicit success status (e.g. 202).
func WithStatus(status int, v any) any { return statusResult{status: status, v: v} }

func isNil(v any) bool {
	if v == nil {
		return true
	}
	// A typed nil pointer is "no result"; nil slices/maps stay JSON values
	// (an empty sensor list is [] not 204).
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.IsNil()
}
