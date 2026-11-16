package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Lane is a per-host concurrency class. Each (host, lane) pair has its own
// Limiter; lanes on the same host share one circuit breaker.
type Lane struct {
	Name        string
	Concurrency int
}

// Standard lanes.
var (
	// LaneAMI serialises AMI MegaRAC /api (single FastCGI worker).
	LaneAMI = Lane{Name: "ami", Concurrency: 1}
	// LaneRedfish allows up to 4 concurrent Redfish calls (B650 caps
	// /redfish at 10 connections per IP via mod_evasive).
	LaneRedfish = Lane{Name: "redfish", Concurrency: 4}
	// LaneDefault serialises anything else (detection probes).
	LaneDefault = Lane{Name: "default", Concurrency: 1}
)

// Config configures a Manager. Zero values pick the documented defaults.
type Config struct {
	// GlobalConcurrency caps in-flight southbound HTTP attempts across all
	// hosts. Default 64.
	GlobalConcurrency int
	// DefaultProfile is the TLS profile for hosts without SetProfile.
	// Default ProfileAuto.
	DefaultProfile TLSProfile
	// DialTimeout (default 10s), TLSHandshakeTimeout (default 15s),
	// IdleConnTimeout (default 20s, below typical BMC keep-alive),
	// MaxIdleConnsPerHost (default 4).
	DialTimeout         time.Duration
	TLSHandshakeTimeout time.Duration
	IdleConnTimeout     time.Duration
	MaxIdleConnsPerHost int
	// AttemptTimeout bounds one HTTP attempt including body read. Default
	// 60s. RequestOptions.Timeout overrides per call.
	AttemptTimeout time.Duration
	// MaxBodyBytes caps response buffering (default DefaultMaxBodyBytes).
	MaxBodyBytes int64
	Retry        RetryPolicy
	Breaker      BreakerConfig
	// IsEvasive403 classifies 403s (default DefaultIsEvasive403).
	IsEvasive403 Evasive403Func
	// Obs provides loggers (when the request context has none) and the HAR
	// writer. Default obs.Default().
	Obs *obs.Obs
	// HAR overrides Obs.HAR().
	HAR   *obs.HARWriter
	Hooks Hooks
	// Now is the clock for the breakers (tests). Default time.Now.
	Now func() time.Time
	// BaseTransport replaces the internal *http.Transport (tests/fakes). TLS
	// profiles then do not apply.
	BaseTransport http.RoundTripper
}

// Manager is the fleet-wide southbound HTTP layer: one per process. It owns
// the keep-alive pool, TLS profile selection, per-host limiters and breakers,
// the global worker pool, retry policy and stats. Safe for concurrent use.
type Manager struct {
	cfg    Config
	base   http.RoundTripper
	ht     *http.Transport // nil when BaseTransport supplied
	dialer *tlsDialer
	tracer *Tracer
	global *semaphore.Weighted

	mu    sync.Mutex
	hosts map[string]*hostState
}

type hostState struct {
	host    string
	breaker *Breaker
	profile TLSProfile
	hasProf bool
	lanes   map[string]*Endpoint
	stats   statsMu
}

// NewManager builds a Manager.
func NewManager(cfg Config) *Manager {
	if cfg.GlobalConcurrency <= 0 {
		cfg.GlobalConcurrency = 64
	}
	if cfg.DialTimeout <= 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if cfg.TLSHandshakeTimeout <= 0 {
		cfg.TLSHandshakeTimeout = 15 * time.Second
	}
	if cfg.IdleConnTimeout <= 0 {
		cfg.IdleConnTimeout = 20 * time.Second
	}
	if cfg.MaxIdleConnsPerHost <= 0 {
		cfg.MaxIdleConnsPerHost = 4
	}
	if cfg.AttemptTimeout == 0 {
		cfg.AttemptTimeout = 60 * time.Second
	}
	if cfg.IsEvasive403 == nil {
		cfg.IsEvasive403 = DefaultIsEvasive403
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	cfg.Retry = cfg.Retry.withDefaults()
	m := &Manager{cfg: cfg, global: semaphore.NewWeighted(int64(cfg.GlobalConcurrency)), hosts: map[string]*hostState{}}
	if cfg.BaseTransport != nil {
		m.base = cfg.BaseTransport
	} else {
		m.dialer = &tlsDialer{
			dialer:           net.Dialer{Timeout: cfg.DialTimeout, KeepAlive: 30 * time.Second},
			handshakeTimeout: cfg.TLSHandshakeTimeout,
			profileFor:       m.Profile,
			onConn:           m.recordTLS,
		}
		m.ht = &http.Transport{
			Proxy:                 nil, // BMC networks are direct; never honour HTTP(S)_PROXY
			DialContext:           m.dialer.dialer.DialContext,
			DialTLSContext:        m.dialer.DialTLSContext,
			MaxIdleConns:          1024,
			MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
			IdleConnTimeout:       cfg.IdleConnTimeout,
			ExpectContinueTimeout: time.Second,
			DisableCompression:    true, // keep wire bytes identical for HAR/TRACE
		}
		m.base = m.ht
	}
	m.tracer = &Tracer{Base: m.base, Obs: cfg.Obs, HAR: cfg.HAR, MaxBodyBytes: cfg.MaxBodyBytes}
	return m
}

// NewHTTPClient returns a plain *http.Client with the given TLS profile, the
// keep-alive pool and tracing, but WITHOUT limiter, breaker or retry. Use
// Endpoint for BMC traffic; this is for one-shot tools and tests.
func NewHTTPClient(p TLSProfile, o *obs.Obs) *http.Client {
	m := NewManager(Config{DefaultProfile: p, Obs: o})
	return &http.Client{Transport: m.tracer}
}

// CanonicalHost normalises "host" / "host:port" to "host:port" (443 default).
func CanonicalHost(host string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	h := strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	return net.JoinHostPort(h, "443")
}

func (m *Manager) host(host string) *hostState {
	host = CanonicalHost(host)
	m.mu.Lock()
	defer m.mu.Unlock()
	hs := m.hosts[host]
	if hs == nil {
		hs = &hostState{host: host, lanes: map[string]*Endpoint{}}
		hs.stats.c.classes = map[string]uint64{}
		hs.stats.c.latency = newHistogram()
		hs.breaker = NewBreaker(m.cfg.Breaker, m.cfg.Now, func(from, to BreakerState) {
			lvl := slog.LevelInfo
			if to == BreakerOpen {
				lvl = slog.LevelWarn
			}
			obs.LoggerFrom(context.Background(), m.cfg.Obs, obs.SubTransport).Log(context.Background(), lvl,
				"circuit breaker state change", "bmc", host, "from", from.String(), "to", to.String())
			if m.cfg.Hooks.OnBreakerChange != nil {
				m.cfg.Hooks.OnBreakerChange(host, from, to)
			}
		})
		m.hosts[host] = hs
	}
	return hs
}

// Endpoint returns the (cached) endpoint for host and lane. host is
// "host[:port]" — use bmc.Target.Addr(). Calling again with a different
// lane.Concurrency adjusts the limit.
func (m *Manager) Endpoint(host string, lane Lane) *Endpoint {
	if lane.Name == "" {
		lane = LaneDefault
	}
	if lane.Concurrency < 1 {
		lane.Concurrency = 1
	}
	hs := m.host(host)
	m.mu.Lock()
	defer m.mu.Unlock()
	ep := hs.lanes[lane.Name]
	if ep == nil {
		ep = &Endpoint{m: m, hs: hs, lane: lane, lim: NewLimiter(lane.Concurrency)}
		hs.lanes[lane.Name] = ep
	} else if ep.lim.Limit() != lane.Concurrency {
		ep.lim.SetLimit(lane.Concurrency)
		ep.lane.Concurrency = lane.Concurrency
	}
	return ep
}

// HTTPClient is shorthand for m.Endpoint(host, lane).HTTPClient().
func (m *Manager) HTTPClient(host string, lane Lane) *http.Client {
	return m.Endpoint(host, lane).HTTPClient()
}

// SetProfile pins the TLS profile for host (e.g. from a board profile:
// X470 -> ProfileLegacy). Existing idle connections are kept.
func (m *Manager) SetProfile(host string, p TLSProfile) {
	hs := m.host(host)
	m.mu.Lock()
	hs.profile, hs.hasProf = p, true
	m.mu.Unlock()
}

// Profile returns the configured TLS profile for host.
func (m *Manager) Profile(host string) TLSProfile {
	hs := m.host(host)
	m.mu.Lock()
	defer m.mu.Unlock()
	if hs.hasProf {
		return hs.profile
	}
	return m.cfg.DefaultProfile
}

func (m *Manager) recordTLS(addr string, info TLSInfo) {
	hs := m.host(addr)
	hs.stats.Lock()
	hs.stats.c.tls = &info
	hs.stats.Unlock()
}

// Breaker returns the circuit breaker for host.
func (m *Manager) Breaker(host string) *Breaker { return m.host(host).breaker }

// BreakerState returns the breaker state for host.
func (m *Manager) BreakerState(host string) BreakerState { return m.host(host).breaker.State() }

// HostStats returns a snapshot for one host (ok=false if never used).
func (m *Manager) HostStats(host string) (HostStats, bool) {
	host = CanonicalHost(host)
	m.mu.Lock()
	hs := m.hosts[host]
	m.mu.Unlock()
	if hs == nil {
		return HostStats{}, false
	}
	return m.snapshot(hs), true
}

// Stats returns snapshots for every host, sorted by host.
func (m *Manager) Stats() []HostStats {
	m.mu.Lock()
	hss := make([]*hostState, 0, len(m.hosts))
	for _, hs := range m.hosts {
		hss = append(hss, hs)
	}
	m.mu.Unlock()
	out := make([]HostStats, 0, len(hss))
	for _, hs := range hss {
		out = append(out, m.snapshot(hs))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

func (m *Manager) snapshot(hs *hostState) HostStats {
	s := HostStats{Host: hs.host, Breaker: hs.breaker.Snapshot(), Profile: m.Profile(hs.host)}
	m.mu.Lock()
	for _, ep := range hs.lanes {
		s.Lanes = append(s.Lanes, LaneStats{Name: ep.lane.Name, Limit: ep.lim.Limit(), InFlight: ep.lim.InFlight(), Queued: ep.lim.Waiting()})
	}
	m.mu.Unlock()
	sort.Slice(s.Lanes, func(i, j int) bool { return s.Lanes[i].Name < s.Lanes[j].Name })
	hs.stats.Lock()
	c := &hs.stats.c
	s.Requests, s.Attempts, s.Retries = c.requests, c.attempts, c.retries
	s.Classes = make(map[string]uint64, len(c.classes))
	for k, v := range c.classes {
		s.Classes[k] = v
	}
	s.Latency = c.latency.snapshot()
	s.QueueWaitSeconds, s.QueueWaits = c.queueWait.Seconds(), c.queueWaits
	s.LastError, s.LastErrorAt, s.LastSuccessAt = c.lastErr, c.lastErrAt, c.lastOKAt
	if c.tls != nil {
		t := *c.tls
		s.TLS = &t
	}
	hs.stats.Unlock()
	return s
}

// GlobalConcurrency returns the global in-flight attempt cap.
func (m *Manager) GlobalConcurrency() int { return m.cfg.GlobalConcurrency }

// Close drops idle keep-alive connections.
func (m *Manager) Close() {
	if m.ht != nil {
		m.ht.CloseIdleConnections()
	}
}

// ---------------------------------------------------------------------------

// Endpoint is the handle protocol clients use for one (host, lane). All
// southbound HTTP for that host/lane must go through Do (or HTTPClient()).
type Endpoint struct {
	m    *Manager
	hs   *hostState
	lane Lane
	lim  *Limiter
}

// Host returns the canonical "host:port".
func (e *Endpoint) Host() string { return e.hs.host }

// Lane returns the lane.
func (e *Endpoint) Lane() Lane { return e.lane }

// Limiter exposes the lane limiter (read-only use: stats).
func (e *Endpoint) Limiter() *Limiter { return e.lim }

// BaseURL returns "https://host:port".
func (e *Endpoint) BaseURL() string { return "https://" + e.hs.host }

// NewRequest builds a request for path (absolute "/api/..." or full URL)
// against this endpoint. A []byte, string or io.Reader body is accepted (nil
// for none).
func (e *Endpoint) NewRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var r io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		r = bytes.NewReader(b)
	case string:
		r = strings.NewReader(b)
	case io.Reader:
		r = b
	default:
		return nil, fmt.Errorf("transport: unsupported body type %T", body)
	}
	u := path
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		u = e.BaseURL() + "/" + strings.TrimPrefix(path, "/")
	}
	return http.NewRequestWithContext(ctx, method, u, r)
}

// HTTPClient returns an *http.Client whose every request goes through Do.
// It has no cookie jar and no overall timeout (use contexts). Redirects are
// not followed (BMC redirects are protocol signals, e.g. to /login).
func (e *Endpoint) HTTPClient() *http.Client {
	return &http.Client{
		Transport:     endpointRT{e},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type endpointRT struct{ e *Endpoint }

func (rt endpointRT) RoundTrip(r *http.Request) (*http.Response, error) { return rt.e.Do(r) }

type heldKey struct{ l *Limiter }

// Exclusive runs fn while holding this lane's slot, so a multi-request
// sequence (e.g. the X470 fan table two-step select+read) cannot be
// interleaved with other requests to the same host/lane. Requests made
// through this endpoint with the ctx passed to fn reuse the held slot.
// Re-entrant. fn must not leak the ctx to other goroutines.
func (e *Endpoint) Exclusive(ctx context.Context, fn func(ctx context.Context) error) error {
	if ctx.Value(heldKey{e.lim}) != nil {
		return fn(ctx)
	}
	start := time.Now()
	if err := e.lim.Acquire(ctx, PriorityWrite); err != nil {
		return e.ctxErr("Exclusive", err)
	}
	e.recordWait(time.Since(start))
	defer e.lim.Release()
	return fn(context.WithValue(ctx, heldKey{e.lim}, true))
}

func (e *Endpoint) recordWait(d time.Duration) {
	e.hs.stats.Lock()
	e.hs.stats.c.queueWait += d
	e.hs.stats.c.queueWaits++
	e.hs.stats.Unlock()
	if e.m.cfg.Hooks.OnQueueWait != nil {
		e.m.cfg.Hooks.OnQueueWait(e.hs.host, e.lane.Name, d)
	}
}

func (e *Endpoint) ctxErr(op string, err error) error {
	sent := bmc.ErrTimeout
	if errors.Is(err, context.Canceled) {
		return &bmc.OpError{Pkg: "transport", Op: op, Detail: e.hs.host, Err: err}
	}
	return &bmc.OpError{Pkg: "transport", Op: op, Detail: e.hs.host, Err: fmt.Errorf("%w: %w", sent, err)}
}

// Do performs req through the breaker, the per-host lane limiter (priority
// FIFO), the global worker pool and the retry loop, with TRACE/HAR capture.
//
// Unless RequestOptions.Stream is set the returned response body is fully
// buffered (the limiter slot is already released) — still Close it.
//
// Errors are *bmc.OpError values wrapping bmc.ErrBusy (breaker open, final
// 503 or evasive 403), bmc.ErrTimeout, bmc.ErrUpstream (connection errors)
// or context.Canceled. Other HTTP statuses (401, 404, 500, ...) are returned
// as a normal response for the protocol client to interpret.
func (e *Endpoint) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	opts := OptionsFrom(ctx)
	ctx, _ = obs.EnsureCorrelationID(ctx)
	log := obs.LoggerFrom(ctx, e.m.cfg.Obs, obs.SubTransport)
	if req.URL.Host == "" {
		u := *req.URL
		u.Scheme, u.Host = "https", e.hs.host
		req = req.Clone(ctx)
		req.URL = &u
	}

	// Make the body replayable.
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		b, err := io.ReadAll(req.Body)
		_ = req.Body.Close()
		if err != nil {
			return nil, &bmc.OpError{Pkg: "transport", Op: opName(req, opts), Err: fmt.Errorf("%w: reading request body: %w", bmc.ErrInvalidArgument, err)}
		}
		req = req.Clone(ctx)
		req.Body = io.NopCloser(bytes.NewReader(b))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }
		req.ContentLength = int64(len(b))
	}

	safe := IsSafeMethod(req.Method)
	retryable := (safe || opts.Idempotent) && !opts.NoRetry
	pri := opts.Priority
	if pri == PriorityAuto {
		pri = PriorityRead
		if !safe {
			pri = PriorityWrite
		}
	}
	maxTries := e.m.cfg.Retry.MaxAttempts
	if opts.NoRetry {
		maxTries = 1
	}

	start := time.Now()
	var (
		resp    *http.Response
		err     error
		class   string
		attempt int
	)
	for attempt = 1; ; attempt++ {
		var r attemptResult
		r = e.attempt(ctx, req, opts, pri, attempt, maxTries)
		resp, err, class = r.resp, r.err, r.class

		canRetry := attempt < maxTries && r.transient && (retryable || (!r.wrote && r.err != nil && !opts.NoRetry)) &&
			ctx.Err() == nil && !(opts.Stream && resp != nil)
		if e.m.cfg.Hooks.OnAttempt != nil {
			e.m.cfg.Hooks.OnAttempt(AttemptInfo{Host: e.hs.host, Lane: e.lane.Name, Method: req.Method, Path: req.URL.Path, Op: opts.Op,
				Attempt: attempt, Status: statusOf(resp), Class: class, Duration: r.dur, Err: err, WillRetry: canRetry})
		}
		e.hs.stats.Lock()
		e.hs.stats.c.attempts++
		if canRetry {
			e.hs.stats.c.retries++
		}
		e.hs.stats.Unlock()
		if !canRetry {
			break
		}
		delay := e.m.cfg.Retry.Backoff(attempt)
		if ra := retryAfter(resp); ra > 0 {
			delay = min(ra, e.m.cfg.Retry.MaxDelay)
		}
		if resp != nil {
			_ = resp.Body.Close()
			resp = nil
		}
		log.DebugContext(ctx, "retrying southbound request", "bmc", e.hs.host, "method", req.Method, "path", req.URL.Path,
			"attempt", attempt, "class", class, "status", r.status, "delay", delay, "err", err)
		if serr := sleepCtx(ctx, delay); serr != nil {
			err, class = e.ctxErr(opName(req, opts), serr), ctxClass(serr)
			break
		}
	}

	// Convert a final busy status into an error unless the caller wants it.
	if err == nil && resp != nil && class == ClassBusy && !opts.RawStatus {
		st := resp.StatusCode
		_ = resp.Body.Close()
		resp = nil
		detail := "BMC returned 503"
		if st == http.StatusForbidden {
			detail = "connection cap (mod_evasive 403)"
		}
		err = &bmc.OpError{Pkg: "transport", Op: opName(req, opts), UpstreamStatus: st, Detail: detail, Err: bmc.ErrBusy}
	}

	dur := time.Since(start)
	e.hs.stats.Lock()
	c := &e.hs.stats.c
	c.requests++
	c.classes[class]++
	c.latency.observe(dur)
	if err != nil || class == ClassServerError {
		c.lastErrAt = time.Now()
		if err != nil {
			c.lastErr = obs.RedactString(err.Error())
		} else {
			c.lastErr = fmt.Sprintf("%s %s: HTTP %d", req.Method, req.URL.Path, statusOf(resp))
		}
	} else {
		c.lastOKAt = time.Now()
	}
	e.hs.stats.Unlock()
	if e.m.cfg.Hooks.OnRequest != nil {
		e.m.cfg.Hooks.OnRequest(RequestInfo{Host: e.hs.host, Lane: e.lane.Name, Method: req.Method, Path: req.URL.Path, Op: opts.Op,
			Attempts: attempt, Status: statusOf(resp), Class: class, Duration: dur, Err: err})
	}
	return resp, err
}

type attemptResult struct {
	resp      *http.Response
	err       error
	class     string
	status    int
	transient bool // eligible for retry
	wrote     bool // request reached the wire
	dur       time.Duration
}

func statusOf(r *http.Response) int {
	if r == nil {
		return 0
	}
	return r.StatusCode
}

func opName(req *http.Request, o RequestOptions) string {
	if o.Op != "" {
		return o.Op
	}
	return req.Method + " " + req.URL.Path
}

func ctxClass(err error) string {
	if errors.Is(err, context.Canceled) {
		return ClassCanceled
	}
	return ClassTimeout
}

func (e *Endpoint) attempt(ctx context.Context, req *http.Request, opts RequestOptions, pri Priority, n, maxTries int) attemptResult {
	op := opName(req, opts)
	// 1. circuit breaker
	done, berr := e.hs.breaker.Allow()
	if berr != nil {
		return attemptResult{class: ClassBreakerOpen, err: &bmc.OpError{Pkg: "transport", Op: op, Detail: e.hs.host + ": circuit breaker open", Err: fmt.Errorf("%w: %w", bmc.ErrBusy, berr)}}
	}
	// 2. per-host lane limiter (unless held by Exclusive)
	held := ctx.Value(heldKey{e.lim}) != nil
	release := func() {}
	if !held {
		ws := time.Now()
		if err := e.lim.Acquire(ctx, pri); err != nil {
			done(OutcomeIgnore)
			return attemptResult{class: ctxClass(err), err: e.ctxErr(op, err)}
		}
		if w := time.Since(ws); w > time.Millisecond {
			e.recordWait(w)
			obs.LoggerFrom(ctx, e.m.cfg.Obs, obs.SubTransport).DebugContext(ctx, "limiter wait", "bmc", e.hs.host, "lane", e.lane.Name, "wait", w, "priority", pri.String())
		}
		release = e.lim.Release
	}
	// 3. global pool
	if err := e.m.global.Acquire(ctx, 1); err != nil {
		release()
		done(OutcomeIgnore)
		return attemptResult{class: ctxClass(err), err: e.ctxErr(op, err)}
	}
	relAll := func() { e.m.global.Release(1); release() }

	// 4. the attempt itself
	actx := ctx
	cancel := context.CancelFunc(func() {})
	to := opts.Timeout
	if to == 0 {
		to = e.m.cfg.AttemptTimeout
	}
	if to > 0 {
		actx, cancel = context.WithTimeout(ctx, to)
	}
	st := &attemptState{attempt: n, maxTries: maxTries}
	actx = context.WithValue(actx, attemptKey{}, st)
	r2 := req.Clone(actx)
	if req.GetBody != nil {
		b, err := req.GetBody()
		if err != nil {
			cancel()
			relAll()
			done(OutcomeIgnore)
			return attemptResult{class: ClassConn, err: &bmc.OpError{Pkg: "transport", Op: op, Err: fmt.Errorf("%w: %w", bmc.ErrUpstream, err)}}
		}
		r2.Body = b
	}
	// AMI honours X-HTTP-Method-Override; never let it (or its common
	// aliases) through.
	for _, h := range MethodOverrideHeaders {
		r2.Header.Del(h)
	}
	t0 := time.Now()
	resp, err := e.m.tracer.RoundTrip(r2)
	dur := time.Since(t0)
	st.mu.Lock()
	streaming := st.streaming
	st.mu.Unlock()
	res := attemptResult{resp: resp, dur: dur, wrote: st.didWrite()}

	if err == nil && streaming {
		// hold slots until the caller closes the body
		var once sync.Once
		body := resp.Body
		resp.Body = &onClose{ReadCloser: body, fn: func() { once.Do(func() { cancel(); relAll() }) }}
	} else {
		cancel()
		relAll()
	}

	if err != nil {
		res.resp = nil
		switch {
		case errors.Is(ctx.Err(), context.Canceled):
			done(OutcomeIgnore)
			res.class, res.err = ClassCanceled, &bmc.OpError{Pkg: "transport", Op: op, Detail: e.hs.host, Err: ctx.Err()}
		case ctx.Err() != nil: // caller deadline
			done(OutcomeFailure)
			res.class, res.err = ClassTimeout, &bmc.OpError{Pkg: "transport", Op: op, Detail: e.hs.host, Err: fmt.Errorf("%w: %w", bmc.ErrTimeout, err)}
		case isTimeout(err):
			done(OutcomeFailure)
			res.class, res.transient = ClassTimeout, true
			res.err = &bmc.OpError{Pkg: "transport", Op: op, Detail: e.hs.host, Err: fmt.Errorf("%w: %w", bmc.ErrTimeout, err)}
		default:
			done(OutcomeFailure)
			res.class, res.transient = ClassConn, IsTransientNetError(err)
			res.err = &bmc.OpError{Pkg: "transport", Op: op, Detail: e.hs.host, Err: fmt.Errorf("%w: %w", bmc.ErrUpstream, err)}
		}
		return res
	}

	res.status = resp.StatusCode
	var body []byte
	if !streaming {
		body, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewReader(body))
	}
	switch sc := resp.StatusCode; {
	case sc == http.StatusServiceUnavailable:
		done(OutcomeFailure)
		res.class, res.transient = ClassBusy, true
	case sc == http.StatusForbidden && !streaming && e.m.cfg.IsEvasive403(resp, body):
		done(OutcomeFailure)
		res.class, res.transient = ClassBusy, true
	case sc == http.StatusBadGateway || sc == http.StatusGatewayTimeout:
		done(OutcomeFailure)
		res.class = ClassServerError
	case sc >= 500:
		done(OutcomeSuccess) // the BMC answered; app-level failure
		res.class = ClassServerError
	case sc == http.StatusUnauthorized:
		done(OutcomeSuccess)
		res.class = ClassAuth
	case sc >= 400:
		done(OutcomeSuccess)
		res.class = ClassClientError
	default:
		done(OutcomeSuccess)
		res.class = ClassOK
	}
	return res
}

type onClose struct {
	io.ReadCloser
	fn func()
}

func (o *onClose) Close() error {
	err := o.ReadCloser.Close()
	o.fn()
	return err
}

// MethodOverrideHeaders are stripped from every outgoing request: the AMI
// /api backend honours X-HTTP-Method-Override (a GET could become a DELETE),
// and X-HTTP-Method / X-Method-Override are common aliases of it.
var MethodOverrideHeaders = []string{"X-HTTP-Method-Override", "X-HTTP-Method", "X-Method-Override"}
