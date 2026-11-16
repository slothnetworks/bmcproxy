package server

import (
	"net"
	"net/http"
	"strings"

	"github.com/slothnetworks/bmcproxy/internal/cache"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// VendorRouteInfo describes one vendor route of a driver.
type VendorRouteInfo struct {
	// HTTP method ("" = any).
	Method string `json:"method,omitempty" example:"POST"`
	// Absolute path pattern.
	Path string `json:"path" example:"/api/v1/vendor/asrock/disableBonding"`
	// Static risk tier.
	Tier bmc.Tier `json:"tier" swaggertype:"string" enums:"read,write,disruptive" example:"disruptive"`
	// The tier is decided per request (e.g. raw passthrough).
	Dynamic bool `json:"dynamicTier,omitempty"`
	// One-line description.
	Summary string `json:"summary,omitempty"`
}

// VendorPrefix is the mount prefix of a driver's vendor routes.
func VendorPrefix(driver string) string { return "/api/v1/vendor/" + driver }

// VendorRoutesOf lists d's vendor routes (nil if it has none).
func VendorRoutesOf(d bmc.Driver) []VendorRouteInfo {
	vr, ok := d.(bmc.VendorRouter)
	if !ok {
		return nil
	}
	var out []VendorRouteInfo
	for _, rt := range vr.VendorRoutes() {
		out = append(out, VendorRouteInfo{
			Method: rt.Method, Path: VendorPrefix(d.Name()) + "/" + strings.TrimPrefix(rt.Path, "/"),
			Tier: rt.Tier, Dynamic: rt.TierFor != nil, Summary: rt.Summary,
		})
	}
	return out
}

// loopbackOnly rejects non-loopback clients (debug endpoints expose
// internals; they must only be reachable locally).
func loopbackOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if ip := net.ParseIP(host); err != nil || ip == nil || !ip.IsLoopback() {
			http.Error(w, "debug endpoints are only served to loopback clients", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) mountDebug() {
	sub := http.NewServeMux()
	obs.RegisterDebugHandlers(sub, s.cfg.Obs)
	// Re-register /debug/levels through an annotated wrapper (more specific
	// method patterns win over RegisterDebugHandlers' catch-all).
	sub.HandleFunc("GET /debug/levels", s.debugLevels)
	sub.HandleFunc("POST /debug/levels", s.debugLevels)
	sub.HandleFunc("GET /debug/targets", s.debugTargets)
	sub.HandleFunc("GET /debug/sessions", s.debugSessions)
	sub.HandleFunc("GET /debug/capabilities", s.debugCapabilities)
	sub.HandleFunc("GET /debug/stats", s.debugStats)
	s.mux.Handle("/debug/", loopbackOnly(sub))
}

// DebugTargets is the body of GET /debug/targets.
type DebugTargets struct {
	// Per-host target cache entries (non-secret).
	Targets []sessions.TargetInfo `json:"targets"`
}

// DebugSessions is the body of GET /debug/sessions.
type DebugSessions struct {
	// Configured idle TTL.
	IdleTTLSeconds float64 `json:"idleTtlSeconds" example:"300"`
	// Cumulative session-cache counters.
	Counters sessions.Counters `json:"counters"`
	// Live sessions (host, user, age, TTL; never tokens or passwords).
	Sessions []sessions.SessionSnapshot `json:"sessions"`
}

// DebugStats is the body of GET /debug/stats.
type DebugStats struct {
	// Session-cache counters (opens, reuses, relogins, evictions).
	Sessions sessions.Counters `json:"sessions"`
	// Read-cache counters and hit rate.
	Cache cache.Stats `json:"cache"`
	// Per-host southbound stats: requests, error classes, limiter queues, breaker, latency histogram.
	Transport []transport.HostStats `json:"transport,omitempty"`
	// Additional sections (Config.ExtraStats).
	Extra map[string]any `json:"extra,omitempty"`
}

// debugTargets serves the target cache.
//
//	@Summary		Target cache
//	@Description	Requires `--debug-endpoints`; loopback clients only. Per-host detected vendor, board, firmware, negotiated TLS, authenticated backends, last probe, clock skew, degraded state.
//	@Tags			debug
//	@Produce		json
//	@Success		200	{object}	DebugTargets
//	@Failure		403	{string}	string	"non-loopback client"
//	@Router			/debug/targets [get]
func (s *Server) debugTargets(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, DebugTargets{Targets: s.cfg.Sessions.Targets().Snapshot()})
}

// debugSessions serves the session cache.
//
//	@Summary		Session cache
//	@Description	Requires `--debug-endpoints`; loopback clients only. Live sessions with host, user, driver, age, idle time and remaining TTL. Never includes passwords, tokens or credential hashes (`id` is an opaque per-credential identifier).
//	@Tags			debug
//	@Produce		json
//	@Success		200	{object}	DebugSessions
//	@Failure		403	{string}	string	"non-loopback client"
//	@Router			/debug/sessions [get]
func (s *Server) debugSessions(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, DebugSessions{
		IdleTTLSeconds: s.cfg.Sessions.IdleTTL().Seconds(),
		Counters:       s.cfg.Sessions.Counters(),
		Sessions:       s.cfg.Sessions.Snapshot(),
	})
}

// debugStats serves aggregate counters.
//
//	@Summary		Proxy statistics
//	@Description	Requires `--debug-endpoints`; loopback clients only. Per-host southbound counters (requests, errors by class, retries, limiter queue depth, breaker, latency histogram), session counters (relogins), cache hit rate.
//	@Tags			debug
//	@Produce		json
//	@Success		200	{object}	DebugStats
//	@Failure		403	{string}	string	"non-loopback client"
//	@Router			/debug/stats [get]
func (s *Server) debugStats(w http.ResponseWriter, r *http.Request) {
	out := DebugStats{Sessions: s.cfg.Sessions.Counters(), Cache: s.cfg.Cache.Stats()}
	if s.cfg.Stats != nil {
		out.Transport = s.cfg.Stats.Stats()
	}
	if s.cfg.ExtraStats != nil {
		out.Extra = s.cfg.ExtraStats()
	}
	WriteJSON(w, http.StatusOK, out)
}

// debugLevels reads or changes per-subsystem log levels.
//
//	@Summary		Log levels
//	@Description	Requires `--debug-endpoints`; loopback clients only. GET returns the per-subsystem levels. POST changes them at runtime: `?set=transport=trace,redfish` (same syntax as --debug; may also lower), `?default=debug`, `?reset=transport`. pprof is served under /debug/pprof/ and expvar at /debug/vars (not documented here).
//	@Tags			debug
//	@Produce		json
//	@Param			set		query		string	false	"Comma list name[=level]"
//	@Param			default	query		string	false	"New default level"
//	@Param			reset	query		string	false	"Comma list of subsystems to reset"
//	@Success		200		{object}	map[string]string
//	@Failure		400		{string}	string	"bad level"
//	@Router			/debug/levels [get]
//	@Router			/debug/levels [post]
func (s *Server) debugLevels(w http.ResponseWriter, r *http.Request) {
	obs.LevelsHandler(s.cfg.Obs.Levels()).ServeHTTP(w, r)
}

// CapabilitiesReport is the body of /debug/capabilities and
// /api/v1/capabilities.
type CapabilitiesReport struct {
	// Canonical host:port.
	Host string `json:"host" example:"10.0.0.5:443"`
	// Resolved driver.
	Driver string `json:"driver" example:"asrock"`
	// Board model.
	Board string `json:"board,omitempty" example:"B650D4U"`
	// Live-reported BMC firmware.
	Firmware string `json:"firmware,omitempty" example:"4.10.00"`
	// Southbound backends and whether they authenticated.
	Backends []bmc.BackendStatus `json:"backends,omitempty"`
	// Reduced-mode reason (e.g. Redfish auth failed, fell back to /api).
	Degraded string `json:"degraded,omitempty"`
	// Every generic capability: supported (else 501) and the serving backend.
	Capabilities []bmc.CapabilityReport `json:"capabilities"`
	// The driver's vendor routes.
	VendorRoutes []VendorRouteInfo `json:"vendorRoutes,omitempty"`
	// Target cache entry (debug endpoint only).
	Target *sessions.TargetInfo `json:"target,omitempty"`
}

// BuildCapabilities describes a session.
func BuildCapabilities(h *sessions.Handle) CapabilitiesReport {
	info := h.Session.Info()
	return CapabilitiesReport{
		Host: h.Key.Host, Driver: h.Driver.Name(), Board: info.Board, Firmware: info.Firmware,
		Backends: info.Backends, Degraded: info.Degraded,
		Capabilities: bmc.Describe(h.Session), VendorRoutes: VendorRoutesOf(h.Driver),
	}
}

// debugCapabilities: ?host= (or X-BMC-Host). With credentials it opens or
// reuses the caller's session; without, it describes any cached session to
// the host.
//
//	@Summary		Resolved driver and capability map
//	@Description	Requires `--debug-endpoints`; loopback clients only. Resolved driver, per-capability backend choice and supported/501 map, vendor routes and the target cache entry. With Basic credentials it opens or reuses that session; without, it describes any cached session to the host (404 if none).
//	@Tags			debug
//	@Produce		json
//	@Param			host			query		string	false	"BMC address (or use X-BMC-Host)"
//	@Param			X-BMC-Host		header		string	false	"BMC address: ip, host or host:port"
//	@Param			X-BMC-Vendor	header		string	false	"Force a driver instead of auto-detection (e.g. asrock)"
//	@Success		200				{object}	CapabilitiesReport
//	@Failure		400				{object}	ErrorBody
//	@Failure		401				{object}	ErrorBody
//	@Failure		404				{object}	ErrorBody	"no cached session and no credentials"
//	@Security		BMCBasicAuth
//	@Router			/debug/capabilities [get]
func (s *Server) debugCapabilities(w http.ResponseWriter, r *http.Request) {
	if hq := r.URL.Query().Get("host"); hq != "" && r.Header.Get(HeaderHost) == "" {
		r.Header.Set(HeaderHost, hq)
	}
	t, c, err := ExtractTarget(r)
	if t.Host == "" {
		s.WriteError(w, r, err)
		return
	}
	var h *sessions.Handle
	if err == nil {
		h, err = s.cfg.Sessions.Acquire(r.Context(), t, c)
		if err != nil {
			s.WriteError(w, r, err)
			return
		}
	} else {
		var ok bool
		if h, ok = s.cfg.Sessions.Peek(t.Key()); !ok {
			s.WriteError(w, r, bmc.Errorf(bmc.ErrNotFound, "server", "debug/capabilities",
				"no cached session for %s; pass Authorization: Basic to open one", t.Key()))
			return
		}
	}
	defer h.Release()
	rep := BuildCapabilities(h)
	if ti, ok := s.cfg.Sessions.Targets().Get(t.Key()); ok {
		rep.Target = &ti
	}
	WriteJSON(w, http.StatusOK, rep)
}
