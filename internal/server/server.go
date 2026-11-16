// Package server is the northbound HTTP server: stdlib ServeMux routing,
// middleware (request id, panic recovery, access log, target and credential
// extraction, tier gate), the JSON error envelope, session/cache helpers for
// handlers, graceful shutdown and the /debug endpoints.
//
// Handler packages (internal/api/v1, internal/api/vendorroutes,
// internal/health, internal/metrics) register onto a *Server; the server
// itself knows only pkg/bmc interfaces.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/cache"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// DefaultAddr is the default listen address (localhost only: there is no
// proxy API auth yet).
const DefaultAddr = "127.0.0.1:8080"

// RequestObserver receives one call per northbound request (metrics).
type RequestObserver interface {
	ObserveRequest(host, route, method string, status int, class string, d time.Duration)
}

// Config configures a Server.
type Config struct {
	// Addr is the listen address. Default DefaultAddr.
	Addr string
	// AllowDisruptive enables disruptive-tier operations (each request must
	// still carry X-BMC-Confirm: true).
	AllowDisruptive bool
	// DebugEndpoints mounts /debug/* (localhost clients only) and enables
	// the envelope's debug block.
	DebugEndpoints bool
	// Debug forces the envelope's debug block on. It is also on whenever
	// the "north" subsystem logs at DEBUG.
	Debug bool
	// Obs is the instrumentation root (required in production; default
	// obs.Default()).
	Obs *obs.Obs
	// Sessions is the session cache (required).
	Sessions *sessions.Manager
	// Cache is the read cache (required).
	Cache *cache.Cache
	// Stats is the transport stats source (usually *transport.Manager). May be nil.
	Stats sessions.StatsSource
	// Observer receives request metrics. May be nil.
	Observer RequestObserver
	// Version is reported by /api/v1/health.
	Version string
	// MaxBodyBytes caps request bodies (per route override). Default 1 MiB.
	MaxBodyBytes int64
	// ReadHeaderTimeout for the http.Server. Default 10s.
	ReadHeaderTimeout time.Duration
	// IdleTimeout for the http.Server. Default 120s.
	IdleTimeout time.Duration
	// ExtraStats, if set, contributes extra sections to /debug/stats.
	ExtraStats func() map[string]any
}

// Server is the northbound server.
type Server struct {
	cfg     Config
	mux     *http.ServeMux
	handler http.Handler
	started time.Time

	mu      sync.Mutex
	httpSrv *http.Server
	routes  []RouteInfo
}

// New creates a Server and mounts the debug endpoints if enabled. Handler
// packages then register their routes.
func New(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = DefaultAddr
	}
	if cfg.Obs == nil {
		cfg.Obs = obs.Default()
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 1 << 20
	}
	if cfg.ReadHeaderTimeout <= 0 {
		cfg.ReadHeaderTimeout = 10 * time.Second
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = 120 * time.Second
	}
	if cfg.Sessions == nil {
		panic("server: Config.Sessions is required")
	}
	if cfg.Cache == nil {
		panic("server: Config.Cache is required")
	}
	s := &Server{cfg: cfg, mux: http.NewServeMux(), started: time.Now()}
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.WriteError(w, r, bmc.Errorf(bmc.ErrNotFound, "server", r.Method+" "+r.URL.Path, "no such route"))
	})
	if cfg.DebugEndpoints {
		s.mountDebug()
	}
	s.handler = s.requestID(s.recoverer(s.accessLog(s.mux)))
	return s
}

// Handler returns the root handler (middleware + mux), e.g. for httptest.
func (s *Server) Handler() http.Handler { return s.handler }

// Mux exposes the underlying mux for raw (non-BMC) handlers such as /metrics.
func (s *Server) Mux() *http.ServeMux { return s.mux }

// Sessions returns the session cache.
func (s *Server) Sessions() *sessions.Manager { return s.cfg.Sessions }

// Cache returns the read cache.
func (s *Server) Cache() *cache.Cache { return s.cfg.Cache }

// Stats returns the transport stats source (may be nil).
func (s *Server) Stats() sessions.StatsSource { return s.cfg.Stats }

// Registry returns the driver registry.
func (s *Server) Registry() *bmc.Registry { return s.cfg.Sessions.Registry() }

// Obs returns the instrumentation root.
func (s *Server) Obs() *obs.Obs { return s.cfg.Obs }

// Version returns the configured version string.
func (s *Server) Version() string { return s.cfg.Version }

// Started returns the server construction time.
func (s *Server) Started() time.Time { return s.started }

// AllowDisruptive reports whether --allow-disruptive is set.
func (s *Server) AllowDisruptive() bool { return s.cfg.AllowDisruptive }

// DebugMode reports whether error envelopes carry the debug block.
func (s *Server) DebugMode() bool {
	return s.cfg.Debug || s.cfg.DebugEndpoints || s.cfg.Obs.Enabled(obs.SubNorth, slog.LevelDebug)
}

// Addr returns the configured listen address.
func (s *Server) Addr() string { return s.cfg.Addr }

func (s *Server) log(ctx context.Context) *slog.Logger {
	return obs.LoggerFrom(ctx, s.cfg.Obs, obs.SubNorth)
}

// ListenAndServe listens on Config.Addr and serves until Shutdown.
func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve serves on ln until Shutdown. It returns nil after a clean shutdown.
func (s *Server) Serve(ln net.Listener) error {
	hs := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return obs.WithObs(context.Background(), s.cfg.Obs)
		},
		ErrorLog: slog.NewLogLogger(s.cfg.Obs.Logger(obs.SubNorth).Handler(), slog.LevelWarn),
	}
	s.mu.Lock()
	s.httpSrv = hs
	s.mu.Unlock()
	s.log(context.Background()).Info("listening", "addr", ln.Addr().String(),
		"allow_disruptive", s.cfg.AllowDisruptive, "debug_endpoints", s.cfg.DebugEndpoints)
	if !isLoopbackAddr(ln.Addr()) {
		s.log(context.Background()).Warn("listening on a non-loopback address: there is no proxy API auth; any client can drive BMCs with credentials it holds")
	}
	err := hs.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown stops accepting connections, drains in-flight requests (until ctx
// expires), then logs out every cached session. If the drain deadline is
// hit, logout still gets a short budget of its own.
func (s *Server) Shutdown(ctx context.Context) error {
	log := s.log(ctx)
	s.mu.Lock()
	hs := s.httpSrv
	s.mu.Unlock()
	var drainErr error
	if hs != nil {
		log.Info("shutdown: draining in-flight requests")
		drainErr = hs.Shutdown(ctx)
		if drainErr != nil {
			log.Warn("shutdown: drain incomplete", "err", drainErr)
		}
	}
	lctx := ctx
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		lctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
	}
	log.Info("shutdown: logging out sessions", "count", s.cfg.Sessions.Len())
	err := s.cfg.Sessions.Shutdown(lctx)
	if err != nil {
		log.Warn("shutdown: session logout incomplete", "err", err)
	}
	return errors.Join(drainErr, err)
}

func isLoopbackAddr(a net.Addr) bool {
	host, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
