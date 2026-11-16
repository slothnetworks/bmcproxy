package testsim

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	v1 "github.com/slothnetworks/bmcproxy/internal/api/v1"
	"github.com/slothnetworks/bmcproxy/internal/api/vendorroutes"
	"github.com/slothnetworks/bmcproxy/internal/cache"
	"github.com/slothnetworks/bmcproxy/internal/health"
	"github.com/slothnetworks/bmcproxy/internal/metrics"
	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/internal/sessions"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// TransportBinder is implemented by drivers that can be pointed at a
// specific transport.Manager (genericredfish.Driver.SetTransport). The
// proxy helper binds every registry driver that implements it.
type TransportBinder interface {
	SetTransport(*transport.Manager)
}

// ProxyOptions configure StartProxy. The helper is driver-agnostic: pass
// Drivers (preferred: a factory that builds fresh driver instances bound to
// the proxy's transport, e.g. genericredfish.New(tm)) or a ready Registry.
type ProxyOptions struct {
	// Drivers builds the drivers for this proxy, given its transport.
	Drivers func(tm *transport.Manager) []bmc.Driver
	// Registry is used as-is when Drivers is nil. Drivers in it that
	// implement TransportBinder are bound to the proxy's transport (do not
	// share a bound driver instance between parallel tests).
	Registry *bmc.Registry

	AllowDisruptive bool
	DebugEndpoints  bool
	// IdleTTL of cached sessions (default 5m).
	IdleTTL time.Duration
	// CacheTTL of read results (default 0: coalesce only, never stale).
	CacheTTL time.Duration
	// TargetTTL of resolved drivers (default 10m).
	TargetTTL time.Duration
	// V1 tunes the generic handlers (default power poll every 5ms, 1s
	// timeout, so power tests are fast).
	V1 v1.Config
	// Transport overrides the test-friendly transport defaults: attempt
	// timeout 2s, 3 attempts with 5-20ms backoff, breaker threshold 5 with a
	// 200ms cool-down. Obs and Hooks are always set by the helper.
	Transport transport.Config
	// SkewAsync measures clock skew after Open instead of inside it.
	SkewAsync bool
	// LogSecrets disables redaction.
	LogSecrets bool
	// LogLevel is the default level for captured logs (default INFO).
	LogLevel slog.Level
	// Debug is an --debug CSV applied to the captured logs.
	Debug string
	// ShutdownTimeout bounds the cleanup Shutdown (default 5s).
	ShutdownTimeout time.Duration
}

// SyncBuffer is a goroutine-safe log sink.
type SyncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *SyncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *SyncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// Proxy is a full bmcproxy (server, sessions, cache, /api/v1, vendor routes,
// health, metrics, transport) running in-process on a loopback listener.
type Proxy struct {
	T         testing.TB
	URL       string
	Server    *server.Server
	Sessions  *sessions.Manager
	Cache     *cache.Cache
	Transport *transport.Manager
	Metrics   *metrics.Metrics
	Registry  *bmc.Registry
	Obs       *obs.Obs
	Logs      *SyncBuffer
	Client    *http.Client

	serveErr chan error
	stopOnce sync.Once
	stopErr  error
}

// TestTransportConfig returns the transport defaults StartProxy uses.
func TestTransportConfig() transport.Config {
	return transport.Config{
		GlobalConcurrency:   64,
		DialTimeout:         time.Second,
		TLSHandshakeTimeout: 2 * time.Second,
		AttemptTimeout:      2 * time.Second,
		Retry:               transport.RetryPolicy{MaxAttempts: 3, BaseDelay: 5 * time.Millisecond, MaxDelay: 20 * time.Millisecond},
		Breaker:             transport.BreakerConfig{FailureThreshold: 5, OpenTimeout: 200 * time.Millisecond},
	}
}

// StartProxy builds and starts the proxy, wired like cmd/bmcproxy. It is
// shut down (draining requests and logging out every cached session) at
// test cleanup unless Shutdown was called first.
func StartProxy(t testing.TB, o ProxyOptions) *Proxy {
	t.Helper()
	logs := &SyncBuffer{}
	ob := obs.NewForHandler(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: obs.LevelTrace}), o.LogLevel, o.LogSecrets)
	if o.Debug != "" {
		if err := ob.Levels().ApplyDebug(o.Debug); err != nil {
			t.Fatal(err)
		}
	}

	tc := TestTransportConfig()
	merge := o.Transport
	if merge.GlobalConcurrency > 0 {
		tc.GlobalConcurrency = merge.GlobalConcurrency
	}
	if merge.DialTimeout > 0 {
		tc.DialTimeout = merge.DialTimeout
	}
	if merge.TLSHandshakeTimeout > 0 {
		tc.TLSHandshakeTimeout = merge.TLSHandshakeTimeout
	}
	if merge.AttemptTimeout != 0 {
		tc.AttemptTimeout = merge.AttemptTimeout
	}
	if merge.Retry != (transport.RetryPolicy{}) {
		tc.Retry = merge.Retry
	}
	if merge.Breaker != (transport.BreakerConfig{}) {
		tc.Breaker = merge.Breaker
	}
	tc.DefaultProfile = merge.DefaultProfile
	tc.IsEvasive403 = merge.IsEvasive403
	tc.Now = merge.Now
	tc.BaseTransport = merge.BaseTransport
	tc.MaxBodyBytes = merge.MaxBodyBytes
	tc.Obs = ob

	met := metrics.New(metrics.Sources{})
	tc.Hooks = met.Hooks()
	tm := transport.NewManager(tc)
	met.SetTransport(tm)

	reg := o.Registry
	if o.Drivers != nil {
		reg = bmc.NewRegistry()
		for _, d := range o.Drivers(tm) {
			reg.Register(d)
		}
	} else if reg == nil {
		t.Fatal("testsim.StartProxy: set ProxyOptions.Drivers or Registry")
	} else {
		for _, d := range reg.Drivers() {
			if b, ok := d.(TransportBinder); ok {
				b.SetTransport(tm)
			}
		}
	}

	if o.IdleTTL == 0 {
		o.IdleTTL = 5 * time.Minute
	}
	if o.V1.PowerPollInterval == 0 {
		o.V1.PowerPollInterval = 5 * time.Millisecond
	}
	if o.V1.PowerPollTimeout == 0 {
		o.V1.PowerPollTimeout = time.Second
	}
	sm := sessions.New(sessions.Config{Registry: reg, IdleTTL: o.IdleTTL, TargetTTL: o.TargetTTL, SweepInterval: -1,
		Stats: tm, Obs: ob, SkewSync: !o.SkewAsync, OpenTimeout: 10 * time.Second, CloseTimeout: 2 * time.Second})
	rc := cache.New(cache.Config{TTL: o.CacheTTL, Obs: ob})
	met.SetSources(metrics.Sources{Sessions: sm, Cache: rc})

	srv := server.New(server.Config{
		Addr:            "127.0.0.1:0",
		AllowDisruptive: o.AllowDisruptive,
		DebugEndpoints:  o.DebugEndpoints,
		Obs:             ob, Sessions: sm, Cache: rc, Stats: tm, Observer: met, Version: "testsim",
	})
	v1.Register(srv, o.V1)
	vendorroutes.Mount(srv, reg)
	health.Register(srv, health.Config{SkewWarn: o.V1.SkewWarn})
	met.Mount(srv.Mux())

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &Proxy{T: t, URL: "http://" + ln.Addr().String(), Server: srv, Sessions: sm, Cache: rc, Transport: tm,
		Metrics: met, Registry: reg, Obs: ob, Logs: logs, serveErr: make(chan error, 1),
		Client: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{MaxIdleConnsPerHost: 64}}}
	go func() { p.serveErr <- srv.Serve(ln) }()

	if o.ShutdownTimeout == 0 {
		o.ShutdownTimeout = 5 * time.Second
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), o.ShutdownTimeout)
		defer cancel()
		_ = p.Shutdown(ctx)
	})
	return p
}

// Shutdown performs the daemon's graceful shutdown: stop accepting, drain
// in-flight requests, log out every cached session, then close the
// transport. Idempotent.
func (p *Proxy) Shutdown(ctx context.Context) error {
	p.stopOnce.Do(func() {
		p.stopErr = p.Server.Shutdown(ctx)
		select {
		case err := <-p.serveErr:
			p.stopErr = errors.Join(p.stopErr, err)
		case <-ctx.Done():
		}
		p.Client.CloseIdleConnections()
		p.Transport.Close()
	})
	return p.stopErr
}

// Req describes a northbound request.
type Req struct {
	Method string
	Path   string
	Host   string // X-BMC-Host ("" = none)
	User   string // default DefaultUser
	Pass   string // default DefaultPass
	NoAuth bool
	Vendor string // X-BMC-Vendor
	Header map[string]string
	Body   any // JSON-marshalled unless []byte/string
}

// Resp is a buffered response.
type Resp struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into v (fatal on error).
func (r *Resp) JSON(t testing.TB, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %q: %v", r.Body, err)
	}
}

// Envelope decodes an error envelope.
func (r *Resp) Envelope(t testing.TB) server.ErrorBody {
	t.Helper()
	var e server.ErrorBody
	r.JSON(t, &e)
	return e
}

// Do performs a request (fatal on transport error).
func (p *Proxy) Do(rq Req) *Resp {
	p.T.Helper()
	r, err := p.Try(rq)
	if err != nil {
		p.T.Fatal(err)
	}
	return r
}

// Try performs a request and returns transport errors (safe from
// goroutines other than the test's).
func (p *Proxy) Try(rq Req) (*Resp, error) {
	var body io.Reader
	switch b := rq.Body.(type) {
	case nil:
	case []byte:
		body = bytes.NewReader(b)
	case string:
		body = strings.NewReader(b)
	default:
		j, err := json.Marshal(b)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(j)
	}
	if rq.Method == "" {
		rq.Method = http.MethodGet
	}
	req, err := http.NewRequest(rq.Method, p.URL+rq.Path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if rq.Host != "" {
		req.Header.Set(server.HeaderHost, rq.Host)
	}
	if rq.Vendor != "" {
		req.Header.Set("X-BMC-Vendor", rq.Vendor)
	}
	if !rq.NoAuth {
		u, pw := rq.User, rq.Pass
		if u == "" {
			u = DefaultUser
		}
		if pw == "" {
			pw = DefaultPass
		}
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u+":"+pw)))
	}
	for k, v := range rq.Header {
		req.Header.Set(k, v)
	}
	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return &Resp{Status: resp.StatusCode, Header: resp.Header, Body: b}, err
}

// Get is a GET against sim (X-BMC-Host = sim.Host()) with its credentials.
func (p *Proxy) Get(sim *Sim, path string) *Resp {
	p.T.Helper()
	c := sim.Creds()
	return p.Do(Req{Path: path, Host: sim.Host(), User: c.User, Pass: c.Pass})
}

// Eventually polls cond until true or the deadline.
func Eventually(t testing.TB, d time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out: %s", msg)
}
