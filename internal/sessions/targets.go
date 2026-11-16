package sessions

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// StatsSource is the subset of *transport.Manager the session layer (and the
// health/debug/metrics layers) read. Any field may be nil in tests.
type StatsSource interface {
	HostStats(host string) (transport.HostStats, bool)
	Stats() []transport.HostStats
	BreakerState(host string) transport.BreakerState
}

// TargetInfo is the non-secret per-host target cache entry.
type TargetInfo struct {
	Host string `json:"host"`
	// Resolution (from bmc.Resolve / Detect and SessionInfo).
	Vendor       string            `json:"vendor,omitempty"`
	Manufacturer string            `json:"manufacturer,omitempty"`
	Board        string            `json:"board,omitempty"`
	Firmware     string            `json:"firmware,omitempty"`
	Score        int               `json:"score,omitempty"`
	Detected     []string          `json:"detectedBackends,omitempty"` // Match.Backends
	Details      map[string]string `json:"details,omitempty"`          // Match.Details
	ResolvedAt   time.Time         `json:"resolvedAt,omitzero"`
	// Authentication / backend state from the last opened session.
	Backends     []bmc.BackendStatus `json:"backends,omitempty"`
	LastOpenAt   time.Time           `json:"lastOpenAt,omitzero"`
	LastProbe    time.Time           `json:"lastProbe,omitzero"`
	LastSeen     time.Time           `json:"lastSeen,omitzero"` // last successful operation
	LastError    string              `json:"lastError,omitempty"`
	LastClass    string              `json:"lastErrorClass,omitempty"`
	LastErrorAt  time.Time           `json:"lastErrorAt,omitzero"`
	SessionDegr  string              `json:"sessionDegraded,omitempty"` // SessionInfo.Degraded
	Relogins     uint64              `json:"relogins"`
	Opens        uint64              `json:"opens"`
	OpenFailures uint64              `json:"openFailures"`
	// Clock skew: offset = bmc - proxy.
	ClockOffset     time.Duration `json:"-"`
	ClockOffsetSecs float64       `json:"clockOffsetSeconds"`
	ClockKnown      bool          `json:"clockKnown"`
	ClockMeasuredAt time.Time     `json:"clockMeasuredAt,omitzero"`
	// Degraded marks the target as temporarily degraded (e.g. relogin after
	// an unexpected 401) until DegradedUntil.
	Degraded      string    `json:"degraded,omitempty"`
	DegradedUntil time.Time `json:"degradedUntil,omitzero"`
	// TLS is filled from transport stats at snapshot time.
	TLS *transport.TLSInfo `json:"tls,omitempty"`
}

// Targets is the per-host target cache. It is safe for concurrent use.
type Targets struct {
	mu    sync.Mutex
	m     map[string]*TargetInfo
	now   func() time.Time
	stats StatsSource
}

func newTargets(now func() time.Time, stats StatsSource) *Targets {
	return &Targets{m: map[string]*TargetInfo{}, now: now, stats: stats}
}

func (ts *Targets) entryLocked(host string) *TargetInfo {
	t, ok := ts.m[host]
	if !ok {
		t = &TargetInfo{Host: host}
		ts.m[host] = t
	}
	return t
}

func (ts *Targets) update(host string, fn func(t *TargetInfo)) {
	host = normHost(host)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	fn(ts.entryLocked(host))
}

// RecordMatch stores a resolution result.
func (ts *Targets) RecordMatch(host string, m bmc.Match) {
	now := ts.now()
	ts.update(host, func(t *TargetInfo) {
		t.Vendor, t.Score = m.Vendor, m.Score
		if m.Manufacturer != "" {
			t.Manufacturer = m.Manufacturer
		}
		if m.Board != "" {
			t.Board = m.Board
		}
		if m.Firmware != "" {
			t.Firmware = m.Firmware
		}
		t.Detected = slices.Clone(m.Backends)
		t.Details = maps.Clone(m.Details)
		t.ResolvedAt, t.LastProbe = now, now
	})
}

func (ts *Targets) recordOpen(host string, info bmc.SessionInfo) {
	now := ts.now()
	ts.update(host, func(t *TargetInfo) {
		if info.Driver != "" {
			t.Vendor = info.Driver
		}
		if info.Board != "" {
			t.Board = info.Board
		}
		if info.Firmware != "" {
			t.Firmware = info.Firmware
		}
		t.Backends = slices.Clone(info.Backends)
		t.SessionDegr = info.Degraded
		t.LastOpenAt, t.LastSeen = now, now
		t.Opens++
		if info.ClockOffset != 0 {
			t.setClock(info.ClockOffset, now)
		}
	})
}

func (t *TargetInfo) setClock(off time.Duration, at time.Time) {
	t.ClockOffset, t.ClockOffsetSecs, t.ClockKnown, t.ClockMeasuredAt = off, off.Seconds(), true, at
}

// SetClockOffset records a measured offset (bmc - proxy) for host.
func (ts *Targets) SetClockOffset(host string, off time.Duration) {
	now := ts.now()
	ts.update(host, func(t *TargetInfo) { t.setClock(off, now) })
}

// ClockOffset returns the measured offset for host and whether one is known.
func (ts *Targets) ClockOffset(host string) (time.Duration, bool) {
	host = normHost(host)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if t, ok := ts.m[host]; ok && t.ClockKnown {
		return t.ClockOffset, true
	}
	return 0, false
}

// MarkDegraded marks host degraded for d.
func (ts *Targets) MarkDegraded(host, reason string, d time.Duration) {
	until := ts.now().Add(d)
	ts.update(host, func(t *TargetInfo) { t.Degraded, t.DegradedUntil = reason, until })
}

// RecordSuccess notes a successful operation against host.
func (ts *Targets) RecordSuccess(host string) {
	now := ts.now()
	ts.update(host, func(t *TargetInfo) { t.LastSeen = now })
}

// RecordError notes a failed operation (message must already be non-secret).
func (ts *Targets) RecordError(host string, err error) {
	if err == nil {
		return
	}
	now := ts.now()
	ts.update(host, func(t *TargetInfo) {
		t.LastError, t.LastClass, t.LastErrorAt = redactErr(err), bmc.Class(err), now
	})
}

func (ts *Targets) recordOpenFailure(host string, err error) {
	ts.RecordError(host, err)
	ts.update(host, func(t *TargetInfo) { t.OpenFailures++ })
}

func (ts *Targets) recordRelogin(host string) {
	ts.update(host, func(t *TargetInfo) { t.Relogins++ })
}

// cachedVendor returns the resolved vendor for host if resolved within ttl.
func (ts *Targets) cachedVendor(host string, ttl time.Duration) string {
	host = normHost(host)
	ts.mu.Lock()
	defer ts.mu.Unlock()
	t, ok := ts.m[host]
	if !ok || t.Vendor == "" || t.ResolvedAt.IsZero() || ts.now().Sub(t.ResolvedAt) > ttl {
		return ""
	}
	return t.Vendor
}

func (ts *Targets) forgetResolution(host string) {
	ts.update(host, func(t *TargetInfo) { t.ResolvedAt = time.Time{} })
}

func (ts *Targets) snapshotLocked(t *TargetInfo) TargetInfo {
	c := *t
	c.Detected = slices.Clone(t.Detected)
	c.Details = maps.Clone(t.Details)
	c.Backends = slices.Clone(t.Backends)
	if !c.DegradedUntil.IsZero() && ts.now().After(c.DegradedUntil) {
		c.Degraded, c.DegradedUntil = "", time.Time{}
	}
	return c
}

func (ts *Targets) withTLS(c TargetInfo) TargetInfo {
	if ts.stats != nil {
		if hs, ok := ts.stats.HostStats(c.Host); ok && hs.TLS != nil {
			tls := *hs.TLS
			c.TLS = &tls
		}
	}
	return c
}

// Get returns a snapshot of host's entry.
func (ts *Targets) Get(host string) (TargetInfo, bool) {
	host = normHost(host)
	ts.mu.Lock()
	t, ok := ts.m[host]
	var c TargetInfo
	if ok {
		c = ts.snapshotLocked(t)
	}
	ts.mu.Unlock()
	if !ok {
		return TargetInfo{}, false
	}
	return ts.withTLS(c), true
}

// Snapshot returns all entries sorted by host (for /debug/targets).
func (ts *Targets) Snapshot() []TargetInfo {
	ts.mu.Lock()
	out := make([]TargetInfo, 0, len(ts.m))
	for _, t := range ts.m {
		out = append(out, ts.snapshotLocked(t))
	}
	ts.mu.Unlock()
	for i := range out {
		out[i] = ts.withTLS(out[i])
	}
	slices.SortFunc(out, func(a, b TargetInfo) int {
		switch {
		case a.Host < b.Host:
			return -1
		case a.Host > b.Host:
			return 1
		}
		return 0
	})
	return out
}
