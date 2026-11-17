package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/cache"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// Route is a northbound route served through the standard wrapper: tier
// gate, optional target extraction, body cap, write-invalidation and result
// encoding.
type Route struct {
	// Method ("GET", ...; "" = any) and Path (ServeMux path pattern,
	// absolute, e.g. "/api/v1/inventory/{kind}").
	Method string
	Path   string
	// Tier is the static tier; TierFor, if set, overrides per request (an
	// error rejects the request).
	Tier    bmc.Tier
	TierFor func(*http.Request) (bmc.Tier, error)
	// NeedsTarget requires X-BMC-Host + Authorization: Basic.
	NeedsTarget bool
	// MaxBody overrides Config.MaxBodyBytes (-1 = unlimited).
	MaxBody int64
	// Group and Summary describe the route for listings.
	Group   string
	Summary string
	// Handler returns a result encoded by WriteResult.
	Handler func(r *http.Request) (any, error)
}

// RouteInfo describes a mounted route.
type RouteInfo struct {
	Method  string   `json:"method,omitempty"`
	Path    string   `json:"path"`
	Tier    bmc.Tier `json:"tier" swaggertype:"string" enums:"read,write,disruptive"`
	Dynamic bool     `json:"dynamicTier,omitempty"`
	Group   string   `json:"group,omitempty"`
	Summary string   `json:"summary,omitempty"`
}

// Pattern returns the ServeMux pattern.
func (rt Route) Pattern() string {
	if rt.Method == "" {
		return rt.Path
	}
	return rt.Method + " " + rt.Path
}

// Handle mounts rt. It returns an error (instead of panicking) when the
// pattern is invalid or conflicts with an existing one.
func (s *Server) Handle(rt Route) (err error) {
	if rt.Handler == nil {
		return fmt.Errorf("server: route %s has no handler", rt.Pattern())
	}
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("server: mount %s: %v", rt.Pattern(), rec)
		}
	}()
	s.mux.Handle(rt.Pattern(), s.wrap(rt))
	s.mu.Lock()
	s.routes = append(s.routes, RouteInfo{Method: rt.Method, Path: rt.Path, Tier: rt.Tier, Dynamic: rt.TierFor != nil, Group: rt.Group, Summary: rt.Summary})
	s.mu.Unlock()
	return nil
}

// MustHandle is Handle that panics on error (static generic routes).
func (s *Server) MustHandle(rt Route) {
	if err := s.Handle(rt); err != nil {
		panic(err)
	}
}

// Routes lists mounted routes in registration order.
func (s *Server) Routes() []RouteInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.routes)
}

func (s *Server) wrap(rt Route) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tier := rt.Tier
		if rt.TierFor != nil {
			t, err := rt.TierFor(r)
			if err != nil {
				s.WriteError(w, r, err)
				return
			}
			tier = t
		}
		if limit := rt.MaxBody; limit >= 0 && r.Body != nil {
			if limit == 0 {
				limit = s.cfg.MaxBodyBytes
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		if rt.NeedsTarget {
			var err error
			if r, err = s.withTarget(r); err != nil {
				s.WriteError(w, r, err)
				return
			}
		}
		v, err := rt.Handler(r)
		if rt.NeedsTarget && (tier >= bmc.TierWrite || !isReadMethod(r.Method)) {
			// Never serve a pre-write read after a write through this host
			// (even a failed one may have partially applied).
			if t, ok := TargetFrom(r.Context()); ok {
				s.cfg.Cache.InvalidateHost(t.Key())
			}
		}
		s.WriteResult(w, r, v, err)
	})
}

func isReadMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

// Call acquires the request's session (with relogin-once on auth failure of
// a cached session) and runs fn. The route must have NeedsTarget.
func (s *Server) Call(r *http.Request, fn func(ctx context.Context, h *sessions.Handle) (any, error)) (any, error) {
	tv, ok := getTarget(r.Context())
	if !ok {
		return nil, errNoTarget
	}
	// The relogin retry re-runs fn, so the request body must be replayable:
	// buffer it (bounded). Bodies too large to buffer get no retry.
	body, replayable, err := bufferBody(r)
	if err != nil {
		return nil, err
	}
	var out any
	run := func(ctx context.Context, h *sessions.Handle) error {
		stateFrom(r.Context()).driver = h.Driver.Name()
		if replayable && body != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		v, err := fn(ctx, h)
		out = v
		return err
	}
	if !replayable {
		h, err := s.cfg.Sessions.Acquire(r.Context(), tv.t, tv.c)
		if err != nil {
			return nil, err
		}
		defer h.Release()
		err = run(r.Context(), h)
		return out, err
	}
	err = s.cfg.Sessions.Do(r.Context(), tv.t, tv.c, run)
	return out, err
}

// maxReplayBody is the largest body buffered for a relogin retry.
const maxReplayBody = 8 << 20

func bufferBody(r *http.Request) ([]byte, bool, error) {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return nil, true, nil
	}
	if r.ContentLength > maxReplayBody {
		return nil, false, nil
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, maxReplayBody+1))
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, false, bmc.Errorf(bmc.ErrInvalidArgument, "server", "body", "request body exceeds %d bytes", mbe.Limit)
		}
		return nil, false, bmc.Errorf(bmc.ErrInvalidArgument, "server", "body", "reading request body: %v", err)
	}
	if len(b) > maxReplayBody {
		// Unknown length and too large: stitch back what was read and stream.
		r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(b), r.Body))
		return nil, false, nil
	}
	return b, true, nil
}

// Read is Call for a read: concurrent identical reads (same host, credential,
// vendor hint, op and params) are coalesced into one driver call and the
// result cached for ttl (<0 = cache default, 0 = coalesce only).
func (s *Server) Read(r *http.Request, op, params string, ttl time.Duration, fn func(ctx context.Context, sess bmc.Session) (any, error)) (any, error) {
	tv, ok := getTarget(r.Context())
	if !ok {
		return nil, errNoTarget
	}
	key := cache.Key{
		Host:   tv.t.Key(),
		User:   sessions.KeyFor(tv.t, tv.c).ID() + "|" + tv.t.VendorHint,
		Op:     op,
		Params: params,
	}
	return s.cfg.Cache.Get(r.Context(), key, ttl, func(ctx context.Context) (any, error) {
		var out any
		err := s.cfg.Sessions.Do(ctx, tv.t, tv.c, func(ctx context.Context, h *sessions.Handle) error {
			v, err := fn(ctx, h.Session)
			out = v
			return err
		})
		return out, err
	})
}

// SetDriver records the driver name for the access log (handlers that
// acquire sessions themselves).
func SetDriver(ctx context.Context, name string) { stateFrom(ctx).driver = name }

// CanonicalParams renders selected query parameters as a stable cache-key
// string.
func CanonicalParams(r *http.Request, names ...string) string {
	q := r.URL.Query()
	var b strings.Builder
	for _, n := range names {
		b.WriteString(n)
		b.WriteByte('=')
		b.WriteString(q.Get(n))
		b.WriteByte('&')
	}
	return b.String()
}
