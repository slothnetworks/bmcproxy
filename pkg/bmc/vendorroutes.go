package bmc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Tier classifies an operation's risk. It is informational (reported in route
// listings and docs); the server does not gate requests on it.
type Tier int

const (
	// TierRead never changes BMC state.
	TierRead Tier = iota
	// TierWrite is reversible (LED, boot override, fan mode, time).
	TierWrite
	// TierDisruptive can cut connectivity or lose data (network/bond, BMC
	// reset, firmware, factory reset, log clear, user changes).
	TierDisruptive
)

func (t Tier) String() string {
	switch t {
	case TierRead:
		return "read"
	case TierWrite:
		return "write"
	case TierDisruptive:
		return "disruptive"
	}
	return fmt.Sprintf("tier(%d)", int(t))
}

// ParseTier parses "read", "write" or "disruptive".
func ParseTier(s string) (Tier, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "read":
		return TierRead, nil
	case "write":
		return TierWrite, nil
	case "disruptive":
		return TierDisruptive, nil
	}
	return 0, fmt.Errorf("%w: unknown tier %q", ErrInvalidArgument, s)
}

func (t Tier) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

func (t *Tier) UnmarshalText(b []byte) error {
	v, err := ParseTier(string(b))
	if err != nil {
		return err
	}
	*t = v
	return nil
}

// HandlerFunc implements a vendor route. s is the resolved, authenticated
// Session for the request's target (type-assert it to the driver's concrete
// session type); r is the northbound request (use r.Context(), r.PathValue,
// r.Body). The server encodes the result:
//
//   - error != nil  -> JSON error envelope, status from HTTPStatus(err)
//   - result == nil -> 204 No Content
//   - result implements Responder -> it writes the response itself
//     (raw passthrough, binary downloads)
//   - otherwise     -> 200 with JSON-encoded result
type HandlerFunc func(s Session, r *http.Request) (any, error)

// Route is one vendor-specific endpoint mounted at
// /api/v1/vendor/{driver.Name()}/{Path}.
type Route struct {
	// Method is the HTTP method ("GET", "POST", ...); "" matches any method.
	Method string
	// Path is relative to the vendor prefix, without a leading slash, in
	// net/http ServeMux pattern syntax: "bonding",
	// "componentStatus/{component}", "raw/api/{path...}".
	Path string
	// Tier is the static risk tier.
	Tier Tier
	// TierFor, if set, overrides Tier per request (e.g. raw passthrough
	// classifying by method and path). It runs before the session is
	// opened; returning an error rejects the request (e.g. ErrForbidden for
	// a denylisted path).
	TierFor func(r *http.Request) (Tier, error)
	// Handler serves the route.
	Handler HandlerFunc
	// Summary is a one-line description for /capabilities.
	Summary string
}

// Pattern returns the ServeMux pattern for the route under prefix (e.g.
// "/api/v1/vendor/asrock"): "POST /api/v1/vendor/asrock/disableBonding".
func (r Route) Pattern(prefix string) string {
	p := strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(r.Path, "/")
	if r.Method == "" {
		return p
	}
	return r.Method + " " + p
}

// EffectiveTier returns TierFor(req) if set, else Tier.
func (r Route) EffectiveTier(req *http.Request) (Tier, error) {
	if r.TierFor != nil {
		return r.TierFor(req)
	}
	return r.Tier, nil
}

// VendorRouter is optionally implemented by a Driver to expose vendor routes.
// Routes are static per driver (mounted at startup); handlers receive the
// per-request Session.
type VendorRouter interface {
	VendorRoutes() []Route
}

// Responder lets a HandlerFunc result write its own response (status,
// headers, raw body). The server calls WriteResponse instead of JSON
// encoding.
type Responder interface {
	WriteResponse(w http.ResponseWriter) error
}

// RawResponse is a ready-made Responder for passthrough routes.
type RawResponse struct {
	Status int
	Header http.Header
	Body   []byte
}

// WriteResponse implements Responder.
func (r *RawResponse) WriteResponse(w http.ResponseWriter) error {
	for k, vs := range r.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	st := r.Status
	if st == 0 {
		st = http.StatusOK
	}
	w.WriteHeader(st)
	_, err := w.Write(r.Body)
	return err
}

// DecodeJSON decodes r.Body into v, rejecting unknown fields; errors wrap
// ErrInvalidArgument. Convenience for route handlers.
func DecodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("%w: empty request body", ErrInvalidArgument)
	}
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: decode request body: %v", ErrInvalidArgument, err)
	}
	return nil
}
