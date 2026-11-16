// Package sessions is the in-memory BMC session cache and the per-host,
// non-secret target cache.
//
// Sessions are keyed by host + user + sha256(salt||password), opened once
// per key (singleflight), shared by concurrent requests, evicted after an
// idle TTL (shorter than the BMC's own timeout) and logged out (Session.Close)
// on eviction, invalidation and Shutdown. Nothing here ever stores or exposes
// a password or token: snapshots carry host, user, driver and timings only.
package sessions

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Config configures a Manager. Zero values pick the defaults noted.
type Config struct {
	// Registry resolves drivers. Default bmc.DefaultRegistry.
	Registry *bmc.Registry
	// IdleTTL evicts (and logs out) a session unused for this long.
	// Default 5m: well below the AMI web session timeout (30 min default).
	IdleTTL time.Duration
	// MaxAge, if > 0, evicts a session this long after it was opened even if
	// busy-idle, to bound how long stale backend state can live.
	MaxAge time.Duration
	// SweepInterval is how often idle sessions are swept. Default IdleTTL/4
	// (min 1s). Negative disables the background sweeper (call Sweep).
	SweepInterval time.Duration
	// TargetTTL is how long a resolved driver for a host is trusted before
	// detection runs again. Default 10m.
	TargetTTL time.Duration
	// OpenTimeout bounds a (shared, singleflighted) Driver.Open. Default 60s.
	OpenTimeout time.Duration
	// CloseTimeout bounds each Session.Close on eviction. Default 10s.
	CloseTimeout time.Duration
	// DegradedFor is how long a target stays "degraded" after a relogin.
	// Default 1m.
	DegradedFor time.Duration
	// SkewRefresh: re-measure clock skew via TimeManager at most this often
	// per host. Default 1h. Negative disables measuring.
	SkewRefresh time.Duration
	// SkewSync measures clock skew synchronously inside Open (tests).
	// Default: asynchronously after the session is cached.
	SkewSync bool
	// Stats provides TLS info for the target cache (usually the
	// *transport.Manager). May be nil.
	Stats StatsSource
	// Obs is the fallback Obs for background logging. Default obs.Default().
	Obs *obs.Obs
	// Now is the clock. Default time.Now.
	Now func() time.Time
}

func (c *Config) defaults() {
	if c.Registry == nil {
		c.Registry = bmc.DefaultRegistry
	}
	if c.IdleTTL <= 0 {
		c.IdleTTL = 5 * time.Minute
	}
	if c.SweepInterval == 0 {
		c.SweepInterval = max(c.IdleTTL/4, time.Second)
	}
	if c.TargetTTL <= 0 {
		c.TargetTTL = 10 * time.Minute
	}
	if c.OpenTimeout <= 0 {
		c.OpenTimeout = 60 * time.Second
	}
	if c.CloseTimeout <= 0 {
		c.CloseTimeout = 10 * time.Second
	}
	if c.DegradedFor <= 0 {
		c.DegradedFor = time.Minute
	}
	if c.SkewRefresh == 0 {
		c.SkewRefresh = time.Hour
	}
	if c.Obs == nil {
		c.Obs = obs.Default()
	}
	if c.Now == nil {
		c.Now = time.Now
	}
}

// ErrShuttingDown is returned by Acquire after Shutdown started (503).
var ErrShuttingDown = fmt.Errorf("%w: proxy shutting down", bmc.ErrBusy)

type entry struct {
	key       Key
	sess      bmc.Session
	drv       bmc.Driver
	openedAt  time.Time
	lastUsed  time.Time
	refs      int
	uses      uint64
	removed   bool // no longer in the map
	closePend bool // close when refs drops to 0
	closed    bool
}

// Handle is a leased cached session. Call Release when done.
type Handle struct {
	Session bmc.Session
	Driver  bmc.Driver
	Key     Key
	// Reused is true when the session came from the cache rather than a
	// fresh Open (an auth failure on a reused session triggers a relogin).
	Reused bool

	m    *Manager
	e    *entry
	once sync.Once
}

// Release returns the lease. It is idempotent.
func (h *Handle) Release() {
	if h == nil || h.m == nil {
		return
	}
	h.once.Do(func() { h.m.release(h.e) })
}

// Counters are cumulative session-cache statistics.
type Counters struct {
	Live      int    `json:"live"`
	Opens     uint64 `json:"opens"`
	OpenFails uint64 `json:"openFailures"`
	Hits      uint64 `json:"hits"`
	Evictions uint64 `json:"evictions"`
	Relogins  uint64 `json:"relogins"`
	Closes    uint64 `json:"closes"`
	CloseErrs uint64 `json:"closeErrors"`
}

// Manager is the session cache.
type Manager struct {
	cfg     Config
	targets *Targets
	sf      singleflight.Group

	mu       sync.Mutex
	entries  map[string]*entry
	shutdown bool
	ctr      Counters

	bg        sync.WaitGroup // background closes / skew measurements
	stopOnce  sync.Once
	stop      chan struct{}
	sweepDone chan struct{}
}

// New creates a Manager and starts its idle sweeper.
func New(cfg Config) *Manager {
	cfg.defaults()
	m := &Manager{
		cfg:       cfg,
		targets:   newTargets(cfg.Now, cfg.Stats),
		entries:   map[string]*entry{},
		stop:      make(chan struct{}),
		sweepDone: make(chan struct{}),
	}
	if cfg.SweepInterval > 0 {
		go m.sweeper()
	} else {
		close(m.sweepDone)
	}
	return m
}

// Targets returns the per-host target cache.
func (m *Manager) Targets() *Targets { return m.targets }

// Registry returns the driver registry in use.
func (m *Manager) Registry() *bmc.Registry { return m.cfg.Registry }

// IdleTTL returns the configured idle TTL.
func (m *Manager) IdleTTL() time.Duration { return m.cfg.IdleTTL }

func (m *Manager) log(ctx context.Context) *slog.Logger {
	return obs.LoggerFrom(ctx, m.cfg.Obs, obs.SubSessions)
}

// Acquire returns a leased session for t/c, opening (once per key, shared by
// concurrent callers) when none is cached. If t.VendorHint names a different
// driver than the cached session's, the cached one is replaced.
func (m *Manager) Acquire(ctx context.Context, t bmc.Target, c bmc.Creds) (*Handle, error) {
	key := KeyFor(t, c)
	mk := key.mapKey()
	for attempt := 0; attempt < 3; attempt++ {
		m.mu.Lock()
		if m.shutdown {
			m.mu.Unlock()
			return nil, ErrShuttingDown
		}
		if e, ok := m.entries[mk]; ok {
			if t.VendorHint != "" && !strings.EqualFold(e.drv.Name(), t.VendorHint) {
				m.removeLocked(e)
				m.mu.Unlock()
				m.log(ctx).DebugContext(ctx, "session vendor hint mismatch; replacing", "key", key, "cached", e.drv.Name(), "hint", t.VendorHint)
				m.closeIfIdle(e, "vendor hint changed")
			} else {
				e.refs++
				e.uses++
				e.lastUsed = m.cfg.Now()
				m.ctr.Hits++
				m.mu.Unlock()
				m.log(ctx).DebugContext(ctx, "session reuse", "key", key, "driver", e.drv.Name())
				return &Handle{Session: e.sess, Driver: e.drv, Key: key, Reused: true, m: m, e: e}, nil
			}
		} else {
			m.mu.Unlock()
		}

		sfKey := mk + "\x00" + strings.ToLower(t.VendorHint)
		ch := m.sf.DoChan(sfKey, func() (any, error) {
			return m.open(ctx, t, c, key)
		})
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case res := <-ch:
			if res.Err != nil {
				return nil, res.Err
			}
			// The opener inserted the entry; loop round to lease it. If it
			// was evicted in between (tiny window), we open again.
		}
	}
	return nil, bmc.Errorf(bmc.ErrBusy, "sessions", "Acquire", "session churn for %s", key)
}

// open runs once per key under singleflight; it inserts the entry.
func (m *Manager) open(ctx context.Context, t bmc.Target, c bmc.Creds, key Key) (any, error) {
	// Detach from the first caller's cancellation (others share the result)
	// but keep its values (logger, request id) and bound the time.
	octx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.cfg.OpenTimeout)
	defer cancel()
	log := m.log(octx)
	host := key.Host

	drv, err := m.resolve(octx, t, c)
	if err != nil {
		m.targets.recordOpenFailure(host, err)
		m.mu.Lock()
		m.ctr.OpenFails++
		m.mu.Unlock()
		return nil, err
	}
	start := m.cfg.Now()
	sess, err := drv.Open(octx, t, c)
	if err != nil {
		log.InfoContext(octx, "session open failed", "key", key, "driver", drv.Name(), "class", bmc.Class(err), "err", redactErr(err))
		m.targets.recordOpenFailure(host, err)
		m.mu.Lock()
		m.ctr.OpenFails++
		m.mu.Unlock()
		if errors.Is(err, bmc.ErrUnsupported) || bmc.HTTPStatus(err) >= 500 {
			// The cached resolution may be wrong (firmware changed); re-detect next time.
			m.targets.forgetResolution(host)
		}
		return nil, err
	}
	if sess == nil {
		err := bmc.Errorf(bmc.ErrUpstream, "sessions", "Open", "driver %s returned a nil session", drv.Name())
		m.targets.recordOpenFailure(host, err)
		return nil, err
	}
	info := sess.Info()
	m.targets.recordOpen(host, info)
	log.InfoContext(octx, "session opened", "key", key, "driver", drv.Name(), "board", info.Board,
		"firmware", info.Firmware, "took", m.cfg.Now().Sub(start).Round(time.Millisecond))

	now := m.cfg.Now()
	e := &entry{key: key, sess: sess, drv: drv, openedAt: now, lastUsed: now}
	m.mu.Lock()
	if m.shutdown {
		m.mu.Unlock()
		m.closeEntry(octx, e, "opened during shutdown")
		return nil, ErrShuttingDown
	}
	mk := key.mapKey()
	var old *entry
	if prev, ok := m.entries[mk]; ok {
		old = prev
		m.removeLocked(prev)
	}
	m.entries[mk] = e
	m.ctr.Opens++
	m.mu.Unlock()
	if old != nil {
		m.closeIfIdle(old, "replaced")
	}

	if info.ClockOffset == 0 {
		m.maybeMeasureSkew(octx, e)
	}
	return nil, nil
}

// resolve picks the driver: the vendor hint, a recently resolved vendor from
// the target cache, or full detection via the registry.
func (m *Manager) resolve(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Driver, error) {
	reg := m.cfg.Registry
	if t.VendorHint == "" {
		if v := m.targets.cachedVendor(t.Key(), m.cfg.TargetTTL); v != "" {
			if d, ok := reg.Lookup(v); ok {
				m.log(ctx).DebugContext(ctx, "driver from target cache", "host", t.Key(), "driver", v)
				return d, nil
			}
		}
	}
	d, match, err := reg.Resolve(ctx, t, &c)
	if err != nil {
		m.log(ctx).InfoContext(ctx, "driver resolution failed", "host", t.Key(), "class", bmc.Class(err), "err", redactErr(err))
		return nil, err
	}
	if match.Vendor == "" {
		match.Vendor = d.Name()
	}
	m.targets.RecordMatch(t.Key(), match)
	m.log(ctx).DebugContext(ctx, "driver resolved", "host", t.Key(), "driver", d.Name(), "score", match.Score,
		"board", match.Board, "firmware", match.Firmware)
	return d, nil
}

// maybeMeasureSkew records bmc-proxy clock offset using TimeManager (if the
// session implements it) unless measured recently.
func (m *Manager) maybeMeasureSkew(ctx context.Context, e *entry) {
	if m.cfg.SkewRefresh < 0 {
		return
	}
	tm, ok := e.sess.(bmc.TimeManager)
	if !ok {
		return
	}
	if ti, ok := m.targets.Get(e.key.Host); ok && ti.ClockKnown && m.cfg.Now().Sub(ti.ClockMeasuredAt) < m.cfg.SkewRefresh {
		return
	}
	measure := func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		tc, err := tm.Time(ctx)
		if err != nil {
			m.log(ctx).DebugContext(ctx, "clock skew measurement failed", "host", e.key.Host, "class", bmc.Class(err))
			return
		}
		if off, ok := OffsetFromTime(tc, m.cfg.Now()); ok {
			m.targets.SetClockOffset(e.key.Host, off)
			m.log(ctx).DebugContext(ctx, "clock skew measured", "host", e.key.Host, "offset", off)
		}
	}
	if m.cfg.SkewSync {
		measure(ctx)
		return
	}
	// Hold a lease so the session is not closed underneath the measurement.
	m.mu.Lock()
	e.refs++
	m.mu.Unlock()
	m.bg.Add(1)
	go func() {
		defer m.bg.Done()
		defer m.release(e)
		measure(context.WithoutCancel(ctx))
	}()
}

// OffsetFromTime derives bmc-proxy offset from a TimeConfig.
func OffsetFromTime(tc bmc.TimeConfig, now time.Time) (time.Duration, bool) {
	if tc.OffsetSeconds != 0 {
		return tc.Offset(), true
	}
	if tc.BMCTime.IsZero() {
		return 0, false
	}
	ref := tc.ProxyTime
	if ref.IsZero() {
		ref = now
	}
	return tc.BMCTime.Sub(ref), true
}

// Do acquires a session and runs fn. If fn fails with ErrAuth on a reused
// session (the BMC dropped it: backend restart, timeout), the session is
// invalidated, the target marked degraded briefly, and fn retried once on a
// fresh session before the error is surfaced. If fn fails with
// bmc.ErrSessionLost (backend restart), every session for the host is
// invalidated before the single retry.
func (m *Manager) Do(ctx context.Context, t bmc.Target, c bmc.Creds, fn func(ctx context.Context, h *Handle) error) error {
	h, err := m.Acquire(ctx, t, c)
	if err != nil {
		return err
	}
	err = fn(ctx, h)
	reused := h.Reused
	h.Release()
	if err == nil {
		m.targets.RecordSuccess(h.Key.Host)
		return nil
	}
	lost := errors.Is(err, bmc.ErrSessionLost)
	if !lost && (!reused || !errors.Is(err, bmc.ErrAuth)) {
		m.recordOpErr(h.Key.Host, err)
		return err
	}
	if lost {
		m.log(ctx).InfoContext(ctx, "BMC backend restart; invalidating all host sessions", "host", h.Key.Host)
		m.InvalidateHost(h.Key.Host)
	} else {
		m.log(ctx).InfoContext(ctx, "auth failure on cached session; relogin", "key", h.Key)
		m.Invalidate(h.Key)
	}
	m.targets.recordRelogin(h.Key.Host)
	m.targets.MarkDegraded(h.Key.Host, "relogin after unexpected auth failure", m.cfg.DegradedFor)
	m.mu.Lock()
	m.ctr.Relogins++
	m.mu.Unlock()

	h2, err := m.Acquire(ctx, t, c)
	if err != nil {
		return err
	}
	defer h2.Release()
	err = fn(ctx, h2)
	if err == nil {
		m.targets.RecordSuccess(h2.Key.Host)
	} else {
		m.recordOpErr(h2.Key.Host, err)
	}
	return err
}

func (m *Manager) recordOpErr(host string, err error) {
	switch bmc.Class(err) {
	case "invalid_argument", "unsupported", "not_found", "disruptive_denied", "forbidden", "canceled":
		// The BMC answered (or was never asked); not a target health signal.
	default:
		m.targets.RecordError(host, err)
	}
}

// Invalidate drops the session for key (logging it out once no request is
// using it). Used for a single relogin.
func (m *Manager) Invalidate(key Key) bool {
	m.mu.Lock()
	e, ok := m.entries[key.mapKey()]
	if ok {
		m.removeLocked(e)
	}
	m.mu.Unlock()
	if ok {
		m.closeIfIdle(e, "invalidated")
	}
	return ok
}

// InvalidateHost drops every session to host (backend restart, BMC reset)
// and forgets its resolution so the next request re-detects. It returns
// the number of sessions dropped.
func (m *Manager) InvalidateHost(host string) int {
	host = normHost(host)
	m.mu.Lock()
	var victims []*entry
	for _, e := range m.entries {
		if e.key.Host == host {
			m.removeLocked(e)
			victims = append(victims, e)
		}
	}
	m.mu.Unlock()
	for _, e := range victims {
		m.closeIfIdle(e, "host invalidated")
	}
	m.targets.forgetResolution(host)
	return len(victims)
}

func (m *Manager) removeLocked(e *entry) {
	if e.removed {
		return
	}
	e.removed = true
	delete(m.entries, e.key.mapKey())
}

// closeIfIdle closes e now if unused, else when its last lease is released.
func (m *Manager) closeIfIdle(e *entry, why string) {
	m.mu.Lock()
	if e.refs > 0 {
		e.closePend = true
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	m.closeEntry(context.Background(), e, why)
}

func (m *Manager) release(e *entry) {
	m.mu.Lock()
	e.refs--
	e.lastUsed = m.cfg.Now()
	doClose := e.refs <= 0 && e.closePend
	m.mu.Unlock()
	if doClose {
		m.bg.Add(1)
		go func() {
			defer m.bg.Done()
			m.closeEntry(context.Background(), e, "deferred")
		}()
	}
}

// closeEntry calls Session.Close exactly once.
func (m *Manager) closeEntry(ctx context.Context, e *entry, why string) {
	m.mu.Lock()
	if e.closed {
		m.mu.Unlock()
		return
	}
	e.closed = true
	m.mu.Unlock()
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.cfg.CloseTimeout)
	defer cancel()
	err := e.sess.Close(cctx)
	m.mu.Lock()
	m.ctr.Closes++
	if err != nil {
		m.ctr.CloseErrs++
	}
	m.mu.Unlock()
	log := m.log(cctx)
	if err != nil {
		log.WarnContext(cctx, "session logout failed", "key", e.key, "why", why, "class", bmc.Class(err), "err", redactErr(err))
		return
	}
	log.DebugContext(cctx, "session logged out", "key", e.key, "why", why)
}

func (m *Manager) sweeper() {
	defer close(m.sweepDone)
	t := time.NewTicker(m.cfg.SweepInterval)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			m.Sweep()
		}
	}
}

// Sweep evicts and logs out sessions idle longer than IdleTTL (or older than
// MaxAge) that are not in use. It returns the number evicted. The background
// sweeper calls it; tests may call it directly.
func (m *Manager) Sweep() int {
	now := m.cfg.Now()
	m.mu.Lock()
	var victims []*entry
	for _, e := range m.entries {
		if e.refs > 0 {
			continue
		}
		idle := now.Sub(e.lastUsed) >= m.cfg.IdleTTL
		old := m.cfg.MaxAge > 0 && now.Sub(e.openedAt) >= m.cfg.MaxAge
		if idle || old {
			m.removeLocked(e)
			victims = append(victims, e)
			m.ctr.Evictions++
		}
	}
	m.mu.Unlock()
	for _, e := range victims {
		m.closeEntry(context.Background(), e, "idle eviction")
	}
	return len(victims)
}

// Shutdown stops the sweeper, refuses new sessions and logs out every cached
// session (in parallel), waiting until done or ctx expires.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.stopOnce.Do(func() { close(m.stop) })
	<-m.sweepDone
	m.mu.Lock()
	m.shutdown = true
	var all []*entry
	for _, e := range m.entries {
		m.removeLocked(e)
		all = append(all, e)
	}
	m.mu.Unlock()
	m.log(ctx).InfoContext(ctx, "logging out cached sessions", "count", len(all))

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for _, e := range all {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m.closeEntry(ctx, e, "shutdown")
			}()
		}
		wg.Wait()
		m.bg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Counters returns cumulative statistics.
func (m *Manager) Counters() Counters {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.ctr
	c.Live = len(m.entries)
	return c
}

// Len returns the number of live cached sessions.
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

// SessionSnapshot is one /debug/sessions row. It never contains secrets.
type SessionSnapshot struct {
	ID           string              `json:"id"` // opaque Key.ID()
	Host         string              `json:"host"`
	User         string              `json:"user"`
	Driver       string              `json:"driver"`
	Board        string              `json:"board,omitempty"`
	Firmware     string              `json:"firmware,omitempty"`
	OpenedAt     time.Time           `json:"openedAt"`
	LastUsed     time.Time           `json:"lastUsed"`
	AgeSeconds   float64             `json:"ageSeconds"`
	IdleSeconds  float64             `json:"idleSeconds"`
	TTLRemaining float64             `json:"ttlRemainingSeconds"`
	InUse        int                 `json:"inUse"`
	Uses         uint64              `json:"uses"`
	Backends     []bmc.BackendStatus `json:"backends,omitempty"`
	Degraded     string              `json:"degraded,omitempty"`
}

// Snapshot lists live sessions sorted by host then user.
func (m *Manager) Snapshot() []SessionSnapshot {
	now := m.cfg.Now()
	m.mu.Lock()
	type row struct {
		e    *entry
		refs int
		uses uint64
		lu   time.Time
	}
	rows := make([]row, 0, len(m.entries))
	for _, e := range m.entries {
		rows = append(rows, row{e, e.refs, e.uses, e.lastUsed})
	}
	m.mu.Unlock()
	out := make([]SessionSnapshot, 0, len(rows))
	for _, r := range rows {
		info := r.e.sess.Info()
		ttl := m.cfg.IdleTTL - now.Sub(r.lu)
		if ttl < 0 {
			ttl = 0
		}
		out = append(out, SessionSnapshot{
			ID: r.e.key.ID(), Host: r.e.key.Host, User: r.e.key.User, Driver: r.e.drv.Name(),
			Board: info.Board, Firmware: info.Firmware, OpenedAt: r.e.openedAt, LastUsed: r.lu,
			AgeSeconds: now.Sub(r.e.openedAt).Seconds(), IdleSeconds: now.Sub(r.lu).Seconds(),
			TTLRemaining: ttl.Seconds(), InUse: r.refs, Uses: r.uses,
			Backends: slices.Clone(info.Backends), Degraded: info.Degraded,
		})
	}
	slices.SortFunc(out, func(a, b SessionSnapshot) int {
		if c := strings.Compare(a.Host, b.Host); c != 0 {
			return c
		}
		return strings.Compare(a.User, b.User)
	})
	return out
}

// HostSessions returns the number of live sessions to host.
func (m *Manager) HostSessions(host string) int {
	host = normHost(host)
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, e := range m.entries {
		if e.key.Host == host {
			n++
		}
	}
	return n
}

// Peek leases any live session to host without opening one (for
// /debug/capabilities without credentials). ok is false if none is cached.
func (m *Manager) Peek(host string) (*Handle, bool) {
	host = normHost(host)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.key.Host == host {
			e.refs++
			return &Handle{Session: e.sess, Driver: e.drv, Key: e.key, Reused: true, m: m, e: e}, true
		}
	}
	return nil, false
}

func redactErr(err error) string { return obs.RedactString(err.Error()) }
