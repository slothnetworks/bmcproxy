package server

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Request headers of the request model.
const (
	HeaderHost      = "X-BMC-Host"
	HeaderVendor    = "X-BMC-Vendor"
	HeaderRequestID = "X-Request-Id"
)

type ctxKey int

const (
	keyState ctxKey = iota
	keyTarget
)

// reqState is per-request mutable bookkeeping for the access log/metrics.
type reqState struct {
	host   string
	user   string
	driver string
	class  string
	errMsg string
}

func stateFrom(ctx context.Context) *reqState {
	if st, ok := ctx.Value(keyState).(*reqState); ok {
		return st
	}
	return &reqState{}
}

type targetVal struct {
	t bmc.Target
	c bmc.Creds
}

// TargetFrom returns the request's resolved target (set for routes that need
// one). The credentials are deliberately not exposed.
func TargetFrom(ctx context.Context) (bmc.Target, bool) {
	tv, ok := ctx.Value(keyTarget).(*targetVal)
	if !ok {
		return bmc.Target{}, false
	}
	return tv.t, true
}

func getTarget(ctx context.Context) (*targetVal, bool) {
	tv, ok := ctx.Value(keyTarget).(*targetVal)
	return tv, ok
}

// statusWriter records the status and size.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func validRequestID(s string) bool {
	if s == "" || len(s) > 128 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune("-_.:", r):
		default:
			return false
		}
	}
	return true
}

// requestID assigns (or accepts) X-Request-Id, echoes it and installs it and
// the Obs into the request context.
func (s *Server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(HeaderRequestID)
		if !validRequestID(id) {
			id = obs.NewID()
		}
		w.Header().Set(HeaderRequestID, id)
		ctx := r.Context()
		if obs.ObsFrom(ctx) == nil {
			ctx = obs.WithObs(ctx, s.cfg.Obs)
		}
		ctx = obs.WithRequestID(ctx, id)
		ctx = context.WithValue(ctx, keyState, &reqState{})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// recoverer turns a handler panic into a 500 envelope and an ERROR log.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			s.log(r.Context()).ErrorContext(r.Context(), "panic in handler", "panic", obs.RedactString(fmt.Sprint(rec)),
				"stack", string(debug.Stack()))
			sw, _ := w.(*statusWriter)
			if sw == nil || sw.status == 0 {
				s.WriteError(w, r, fmt.Errorf("internal error: handler panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// quietPaths are logged at DEBUG instead of INFO (scrapes, probes).
var quietPaths = map[string]bool{"/metrics": true, "/api/v1/health": true}

// accessLog emits one line per request (subsystem north) and feeds metrics.
func (s *Server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			// recoverer is inside; a panic here would only come from logging.
			st := stateFrom(r.Context())
			d := time.Since(start)
			status := sw.status
			if status == 0 {
				status = http.StatusOK
			}
			route := r.Pattern
			if route == "" || route == "/" {
				route = "unmatched"
			}
			lvl := slog.LevelInfo
			if quietPaths[r.URL.Path] && status < 400 {
				lvl = slog.LevelDebug
			}
			attrs := []any{
				"method", r.Method, "path", r.URL.Path, "status", status,
				"dur_ms", float64(d.Microseconds()) / 1000, "bytes", sw.bytes, "remote", r.RemoteAddr,
			}
			if st.host != "" {
				attrs = append(attrs, "bmc", st.host)
			}
			if st.user != "" {
				attrs = append(attrs, "bmc_user", st.user)
			}
			if st.driver != "" {
				attrs = append(attrs, "driver", st.driver)
			}
			if st.class != "" {
				attrs = append(attrs, "class", st.class)
			}
			if st.errMsg != "" {
				attrs = append(attrs, "err", st.errMsg)
			}
			s.log(r.Context()).Log(r.Context(), lvl, "request", attrs...)
			if s.cfg.Observer != nil {
				s.cfg.Observer.ObserveRequest(st.host, route, r.Method, status, st.class, d)
			}
		}()
		next.ServeHTTP(sw, r)
	})
}

// ExtractTarget parses X-BMC-Host, Authorization: Basic and X-BMC-Vendor.
// Errors wrap ErrInvalidArgument (bad/missing host) or ErrAuth (missing or
// malformed credentials). Credentials are never logged.
func ExtractTarget(r *http.Request) (bmc.Target, bmc.Creds, error) {
	hv := strings.TrimSpace(r.Header.Get(HeaderHost))
	if hv == "" {
		return bmc.Target{}, bmc.Creds{}, bmc.Errorf(bmc.ErrInvalidArgument, "server", "target", "missing %s header", HeaderHost)
	}
	t, err := bmc.ParseTarget(hv)
	if err != nil {
		return bmc.Target{}, bmc.Creds{}, &bmc.OpError{Pkg: "server", Op: "target", Err: err}
	}
	t.VendorHint = strings.ToLower(strings.TrimSpace(r.Header.Get(HeaderVendor)))
	c, err := basicCreds(r.Header.Get("Authorization"))
	if err != nil {
		return t, bmc.Creds{}, err
	}
	return t, c, nil
}

func basicCreds(h string) (bmc.Creds, error) {
	missing := bmc.Errorf(bmc.ErrAuth, "server", "credentials", "missing or malformed Authorization: Basic (the BMC's own credentials)")
	scheme, rest, ok := strings.Cut(strings.TrimSpace(h), " ")
	if !ok || !strings.EqualFold(scheme, "Basic") {
		return bmc.Creds{}, missing
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(rest))
	if err != nil {
		return bmc.Creds{}, missing
	}
	u, p, ok := strings.Cut(string(raw), ":")
	if !ok || u == "" {
		return bmc.Creds{}, missing
	}
	return bmc.Creds{User: u, Pass: p}, nil
}

// withTarget extracts the target into the context and the access-log state.
func (s *Server) withTarget(r *http.Request) (*http.Request, error) {
	t, c, err := ExtractTarget(r)
	st := stateFrom(r.Context())
	if t.Host != "" {
		st.host = t.Key()
	}
	if err != nil {
		return r, err
	}
	st.user = c.User
	ctx := context.WithValue(r.Context(), keyTarget, &targetVal{t: t, c: c})
	ctx = obs.WithAttrs(ctx, "bmc", t.Key())
	return r.WithContext(ctx), nil
}

var errNoTarget = errors.New("server: route has no target (Route.NeedsTarget false)")
