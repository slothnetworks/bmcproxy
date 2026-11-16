package servertest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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

// Options configure a Harness.
type Options struct {
	Drivers         []bmc.Driver
	AllowDisruptive bool
	DebugEndpoints  bool
	IdleTTL         time.Duration
	CacheTTL        time.Duration // default 1 minute (so TTL never expires mid-test)
	V1              v1.Config
	Now             func() time.Time
	LogSecrets      bool
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

// Harness is a complete northbound server wired to fake drivers.
type Harness struct {
	T         testing.TB
	Registry  *bmc.Registry
	Sessions  *sessions.Manager
	Cache     *cache.Cache
	Server    *server.Server
	Metrics   *metrics.Metrics
	Transport *transport.Manager
	HTTP      *httptest.Server
	Logs      *SyncBuffer
	Obs       *obs.Obs
}

// New builds and starts a harness; it is torn down with t.Cleanup.
func New(t testing.TB, o Options) *Harness {
	t.Helper()
	reg := bmc.NewRegistry()
	for _, d := range o.Drivers {
		reg.Register(d)
	}
	logs := &SyncBuffer{}
	ob := obs.NewForHandler(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: obs.LevelTrace}), slog.LevelInfo, o.LogSecrets)
	if o.CacheTTL == 0 {
		o.CacheTTL = time.Minute
	}
	if o.V1.PowerPollInterval == 0 {
		o.V1.PowerPollInterval = 5 * time.Millisecond
	}
	if o.V1.PowerPollTimeout == 0 {
		o.V1.PowerPollTimeout = 500 * time.Millisecond
	}
	met := metrics.New(metrics.Sources{})
	tm := transport.NewManager(transport.Config{Obs: ob, Hooks: met.Hooks()})
	met.SetTransport(tm)
	sm := sessions.New(sessions.Config{Registry: reg, IdleTTL: o.IdleTTL, SweepInterval: -1, Stats: tm, Obs: ob, Now: o.Now, SkewSync: true})
	rc := cache.New(cache.Config{TTL: o.CacheTTL, Obs: ob})
	met.SetSources(metrics.Sources{Sessions: sm, Cache: rc})
	srv := server.New(server.Config{
		AllowDisruptive: o.AllowDisruptive, DebugEndpoints: o.DebugEndpoints,
		Obs: ob, Sessions: sm, Cache: rc, Stats: tm, Observer: met, Version: "test",
	})
	v1.Register(srv, o.V1)
	vendorroutes.Mount(srv, reg)
	health.Register(srv, health.Config{})
	met.Mount(srv.Mux())
	hs := httptest.NewServer(srv.Handler())
	h := &Harness{T: t, Registry: reg, Sessions: sm, Cache: rc, Server: srv, Metrics: met, Transport: tm, HTTP: hs, Logs: logs, Obs: ob}
	t.Cleanup(func() {
		hs.Close()
		_ = sm.Shutdown(t.Context())
		tm.Close()
	})
	return h
}

// Req describes a request.
type Req struct {
	Method string
	Path   string
	Host   string // X-BMC-Host ("" = none)
	User   string
	Pass   string
	NoAuth bool
	Header map[string]string
	Body   any // marshalled as JSON unless []byte/string
}

// Resp is a decoded response.
type Resp struct {
	Status int
	Header http.Header
	Body   []byte
}

// JSON decodes the body into v.
func (r *Resp) JSON(t testing.TB, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode %q: %v", r.Body, err)
	}
}

// Envelope decodes an error envelope.
func (r *Resp) Envelope(t testing.TB) server.ErrorBody {
	var e server.ErrorBody
	r.JSON(t, &e)
	return e
}

// Do performs a request against the harness.
func (h *Harness) Do(rq Req) *Resp {
	h.T.Helper()
	var body io.Reader
	switch b := rq.Body.(type) {
	case nil:
	case []byte:
		body = bytes.NewReader(b)
	case string:
		body = strings.NewReader(b)
	default:
		j, _ := json.Marshal(b)
		body = bytes.NewReader(j)
	}
	if rq.Method == "" {
		rq.Method = "GET"
	}
	req, err := http.NewRequest(rq.Method, h.HTTP.URL+rq.Path, body)
	if err != nil {
		h.T.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if rq.Host != "" {
		req.Header.Set(server.HeaderHost, rq.Host)
	}
	if !rq.NoAuth {
		u, p := rq.User, rq.Pass
		if u == "" {
			u = "admin"
		}
		if p == "" {
			p = "s3cret-Pa55"
		}
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(u+":"+p)))
	}
	for k, v := range rq.Header {
		req.Header.Set(k, v)
	}
	resp, err := h.HTTP.Client().Do(req)
	if err != nil {
		h.T.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return &Resp{Status: resp.StatusCode, Header: resp.Header, Body: b}
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
