// Package health serves the northbound health surface:
//
//	GET /api/v1/health            proxy liveness, version, uptime, drivers
//	GET /api/v1/bmcs/{host}/status per-BMC status from the target cache and
//	                               transport stats (no BMC contact unless
//	                               ?probe=true, which runs unauthenticated
//	                               detection)
package health

import (
	"math"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// Config tunes health reporting.
type Config struct {
	// SkewWarn is the |clock offset| above which status warns. Default 2m.
	SkewWarn time.Duration
}

// DriverInfo is one registered driver.
type DriverInfo struct {
	// Driver name (X-BMC-Vendor value, vendor route prefix).
	Name string `json:"name" example:"asrock"`
	// Detection priority (higher first; generic fallbacks negative).
	Priority int `json:"priority" example:"10"`
	// Number of vendor routes under /api/v1/vendor/{name}/.
	VendorRoutes int `json:"vendorRoutes" example:"24"`
}

// Health is the body of GET /api/v1/health.
type Health struct {
	// Always "ok" when the proxy answers.
	Status string `json:"status" example:"ok"`
	// Proxy version.
	Version string `json:"version" example:"1.0.0"`
	// Go runtime version.
	GoVersion string `json:"goVersion" example:"go1.27.3"`
	// Process start time.
	StartedAt time.Time `json:"startedAt"`
	// Seconds since start.
	UptimeSeconds float64 `json:"uptimeSeconds" example:"3600.5"`
	// Compiled-in drivers in detection order.
	Drivers []DriverInfo `json:"drivers"`
	// Live cached BMC sessions.
	Sessions int `json:"sessions" example:"3"`
	// Hosts in the target cache.
	Targets int `json:"targets" example:"2"`
}

// Status is the body of GET /api/v1/bmcs/{host}/status.
type Status struct {
	// Canonical host:port.
	Host string `json:"host" example:"10.0.0.5:443"`
	// The proxy has seen this host before (target cache entry exists).
	Known bool `json:"known" example:"true"`
	// Whether the BMC answered recently; null = never contacted.
	Reachable *bool `json:"reachable" example:"true"`
	// Resolved driver.
	Vendor string `json:"vendor,omitempty" example:"asrock"`
	// Reported manufacturer.
	Manufacturer string `json:"manufacturer,omitempty" example:"ASRockRack"`
	// Board model.
	Board string `json:"board,omitempty" example:"X470D4U"`
	// Live-reported BMC firmware version.
	Firmware string `json:"firmware,omitempty" example:"2.32.00"`
	// Southbound backends of the last session (authenticated or not).
	Backends []bmc.BackendStatus `json:"backends,omitempty"`
	// Backends found reachable during detection.
	DetectedBackends []string `json:"detectedBackends,omitempty" example:"ami,ipmi"`
	// Live cached sessions to this host.
	Sessions int `json:"sessions" example:"1"`
	// Circuit-breaker state.
	Circuit string `json:"circuit" example:"closed" enums:"closed,half-open,open"`
	// Times the breaker has tripped.
	CircuitTrips uint64 `json:"circuitTrips,omitempty" example:"0"`
	// Measured BMC clock offset (bmc - proxy), if known.
	ClockOffsetSeconds *float64 `json:"clockOffsetSeconds,omitempty" example:"-7200"`
	// When the clock offset was measured.
	ClockMeasuredAt time.Time `json:"clockMeasuredAt,omitzero"`
	// Reason the target is degraded (e.g. relogin after auth failure, BMC reset).
	Degraded string `json:"degraded,omitempty"`
	// Degraded state expiry.
	DegradedUntil time.Time `json:"degradedUntil,omitzero"`
	// Last successful operation.
	LastSeen time.Time `json:"lastSeen,omitzero"`
	// Last error (redacted).
	LastError string `json:"lastError,omitempty"`
	// Class of the last error.
	LastErrorClass string `json:"lastErrorClass,omitempty" example:"timeout"`
	// Time of the last error.
	LastErrorAt time.Time `json:"lastErrorAt,omitzero"`
	// Last detection probe.
	LastProbe time.Time `json:"lastProbe,omitzero"`
	// Negotiated TLS parameters.
	TLS *transport.TLSInfo `json:"tls,omitempty"`
	// Human-readable warnings (clock skew, open circuit, rejected backends).
	Warnings []string `json:"warnings,omitempty"`
	// Summary health.
	Health bmc.Health `json:"health" example:"ok" enums:"ok,warning,critical,unknown"`
}

// Register mounts the health routes on s.
func Register(s *server.Server, cfg Config) {
	if cfg.SkewWarn <= 0 {
		cfg.SkewWarn = 2 * time.Minute
	}
	h := &handlers{s: s, cfg: cfg}
	s.MustHandle(server.Route{Method: "GET", Path: "/api/v1/health", Tier: bmc.TierRead, Group: "health",
		Summary: "proxy liveness, version, uptime, drivers", Handler: h.getHealth})
	s.MustHandle(server.Route{Method: "GET", Path: "/api/v1/bmcs/{host}/status", Tier: bmc.TierRead, Group: "health",
		Summary: "per-BMC status from the target cache", Handler: h.getStatus})
}

type handlers struct {
	s   *server.Server
	cfg Config
}

// getHealth reports proxy liveness.
//
//	@Summary		Proxy health
//	@Description	Tier: **read**. Proxy liveness: version, uptime, compiled-in drivers (with priority and vendor-route count), live session and known-target counts. Never contacts a BMC; needs no credentials.
//	@Tags			health
//	@Produce		json
//	@Success		200	{object}	Health
//	@Router			/api/v1/health [get]
func (h *handlers) getHealth(r *http.Request) (any, error) { return Snapshot(h.s), nil }

// getStatus reports one BMC's status from the caches.
//
//	@Summary		BMC status
//	@Description	Tier: **read**. Per-BMC status built from the proxy's target cache and transport stats: reachability (most recent success vs failure; an auth failure still counts as reachable), authenticated backends, board and firmware, circuit-breaker state, measured clock skew (warning above --skew-warn), degraded reason, negotiated TLS. Does not contact the BMC unless `probe=true`, which runs unauthenticated, non-mutating driver detection. Needs no credentials.
//	@Tags			health
//	@Produce		json
//	@Param			host	path		string	true	"BMC address: ip, host or host:port"
//	@Param			probe	query		bool	false	"Run unauthenticated detection first"
//	@Success		200		{object}	Status
//	@Failure		400		{object}	server.ErrorBody	"invalid host"
//	@Router			/api/v1/bmcs/{host}/status [get]
func (h *handlers) getStatus(r *http.Request) (any, error) {
	s := h.s
	t, err := bmc.ParseTarget(r.PathValue("host"))
	if err != nil {
		return nil, err
	}
	if probe, _ := strconv.ParseBool(r.URL.Query().Get("probe")); probe {
		_, m, err := s.Registry().Resolve(r.Context(), t, nil)
		if err != nil {
			s.Sessions().Targets().RecordError(t.Key(), err)
		} else {
			if m.Vendor == "" {
				m.Vendor = "unknown"
			}
			s.Sessions().Targets().RecordMatch(t.Key(), m)
		}
	}
	return HostStatus(s, t.Key(), h.cfg.SkewWarn), nil
}

// Snapshot builds the /api/v1/health body.
func Snapshot(s *server.Server) Health {
	var drivers []DriverInfo
	for _, d := range s.Registry().Drivers() {
		di := DriverInfo{Name: d.Name(), VendorRoutes: len(server.VendorRoutesOf(d))}
		if p, ok := d.(bmc.Prioritized); ok {
			di.Priority = p.Priority()
		}
		drivers = append(drivers, di)
	}
	if drivers == nil {
		drivers = []DriverInfo{}
	}
	return Health{
		Status: "ok", Version: s.Version(), GoVersion: runtime.Version(),
		StartedAt: s.Started(), UptimeSeconds: math.Round(time.Since(s.Started()).Seconds()*1000) / 1000,
		Drivers: drivers, Sessions: s.Sessions().Len(), Targets: len(s.Sessions().Targets().Snapshot()),
	}
}

// HostStatus builds the per-BMC status from caches only.
func HostStatus(s *server.Server, host string, skewWarn time.Duration) Status {
	st := Status{Host: host, Circuit: transport.BreakerClosed.String(), Health: bmc.HealthUnknown}
	ti, known := s.Sessions().Targets().Get(host)
	st.Known = known
	st.Sessions = s.Sessions().HostSessions(host)
	if known {
		st.Vendor, st.Manufacturer, st.Board, st.Firmware = ti.Vendor, ti.Manufacturer, ti.Board, ti.Firmware
		st.Backends, st.DetectedBackends = ti.Backends, ti.Detected
		st.Degraded, st.DegradedUntil = ti.Degraded, ti.DegradedUntil
		if st.Degraded == "" && ti.SessionDegr != "" {
			st.Degraded = ti.SessionDegr
		}
		st.LastSeen, st.LastError, st.LastErrorClass, st.LastErrorAt, st.LastProbe = ti.LastSeen, ti.LastError, ti.LastClass, ti.LastErrorAt, ti.LastProbe
		st.TLS = ti.TLS
		if ti.ClockKnown {
			secs := ti.ClockOffsetSecs
			st.ClockOffsetSeconds = &secs
			st.ClockMeasuredAt = ti.ClockMeasuredAt
			if w := skewWarning(ti.ClockOffset, skewWarn); w != "" {
				st.Warnings = append(st.Warnings, w)
			}
		}
	}
	lastOK, lastErr := st.LastSeen, st.LastErrorAt
	if src := s.Stats(); src != nil {
		if hs, ok := src.HostStats(host); ok {
			st.Circuit = hs.Breaker.State.String()
			st.CircuitTrips = hs.Breaker.Trips
			if hs.LastSuccessAt.After(lastOK) {
				lastOK = hs.LastSuccessAt
			}
			if hs.LastErrorAt.After(lastErr) {
				lastErr = hs.LastErrorAt
				if st.LastError == "" || hs.LastErrorAt.After(st.LastErrorAt) {
					st.LastError, st.LastErrorAt = hs.LastError, hs.LastErrorAt
				}
			}
			if st.TLS == nil && hs.TLS != nil {
				tls := *hs.TLS
				st.TLS = &tls
			}
		} else {
			st.Circuit = src.BreakerState(host).String()
		}
	}
	// Reachability: the most recent signal wins. An auth failure still means
	// the BMC answered.
	switch {
	case !lastOK.IsZero() && !lastOK.Before(lastErr):
		st.Reachable = ptr(true)
	case !lastErr.IsZero():
		answered := st.LastErrorClass == "auth" || st.LastErrorClass == "unsupported" || st.LastErrorClass == "invalid_argument"
		st.Reachable = ptr(answered)
	}
	if st.Circuit == transport.BreakerOpen.String() {
		st.Reachable = ptr(false)
		st.Warnings = append(st.Warnings, "circuit breaker open: requests to this BMC fail fast with 503 until it cools down")
	}
	if st.Degraded != "" {
		st.Warnings = append(st.Warnings, "degraded: "+st.Degraded)
	}
	for _, b := range st.Backends {
		if b.Available && !b.Authenticated {
			st.Warnings = append(st.Warnings, "backend "+b.Name+" reachable but credentials rejected")
		}
	}
	slices.Sort(st.Warnings)
	switch {
	case st.Reachable == nil:
		st.Health = bmc.HealthUnknown
	case !*st.Reachable:
		st.Health = bmc.HealthCritical
	case len(st.Warnings) > 0:
		st.Health = bmc.HealthWarning
	default:
		st.Health = bmc.HealthOK
	}
	return st
}

func ptr[T any](v T) *T { return &v }

// skewWarning mirrors v1.SkewWarning (kept local to avoid a dependency on
// the API package).
func skewWarning(off, limit time.Duration) string {
	abs := time.Duration(math.Abs(float64(off)))
	if limit <= 0 || abs <= limit {
		return ""
	}
	dir := "ahead of"
	if off < 0 {
		dir = "behind"
	}
	return "clock skew: BMC is " + abs.Round(time.Second).String() + " " + dir + " the proxy (POST /api/v1/time/sync corrects it)"
}
