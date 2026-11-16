package testsim

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
)

// Default credentials accepted by a Sim (placeholders, never real ones).
const (
	DefaultUser = "admin"
	DefaultPass = "sim-Passw0rd"
)

// Board identifies a board family (and its fixture directory).
type Board string

// Supported boards.
const (
	BoardX470 Board = "x470"
	BoardB650 Board = "b650"
	BoardX570 Board = "x570"
)

// Profile captures a board+firmware's documented behaviour.
type Profile struct {
	Board Board
	// Vendor is the driver the board belongs to ("asrock", "gigabyte").
	Vendor    string
	BoardName string // asrr/model-name mb_name (ASRock); Redfish ComputerSystem.Model (Gigabyte)
	Firmware  string // live-reported BMC firmware
	// Captures is the preferred HAR capture order for fixture lookup.
	Captures []string
	// SecureCookie: B650 sets QSESSIONID with Secure; HttpOnly; SameSite=Lax.
	SecureCookie bool
	// CookieAttrs, when set, replaces the cookie attributes after
	// "path=/" verbatim (X570: "; secure;HttpOnly", no SameSite).
	CookieAttrs string
	// CSRFOnGET: GETs need the session's exact X-CSRFTOKEN too (X570 1.20;
	// X470/B650 accept any non-empty token on GETs).
	CSRFOnGET bool
	// RedfishIgnoresQuery: $select and $expand are silently ignored (X570
	// 1.20 answers the full, unexpanded resource although the ServiceRoot
	// advertises both).
	RedfishIgnoresQuery bool
	// LoginExtras: B650 login response adds user_id, channel, passwordStatus.
	LoginExtras bool
	// EventLogTrailer: B650 appends {"event_log_count":N} to SEL arrays.
	EventLogTrailer bool
	// ChassisCycleInterval: B650 chassis-status carries power_cycle_interval.
	ChassisCycleInterval bool
	// LegacyTLS: X470 offers only RSA key exchange on TLS 1.0-1.2.
	LegacyTLS bool
	// RedfishAuth: B650 accepts the admin account on Redfish; the X470 2.32
	// answers 401 Security.1.0.AccessDenied to everything below the root.
	RedfishAuth bool
	// EvasiveMaxConcurrent: lighttpd mod_evasive cap on /redfish (B650:
	// 10 per IP). Requests beyond it get an HTML 403. 0 = no cap.
	EvasiveMaxConcurrent int
	// MaxSessions is the number of free web session slots (the live boards
	// report maximum_sessions 148 with a 128 baseline, i.e. 20 usable).
	MaxSessions int

	// CSRFKillsSession: a missing or wrong X-CSRFTOKEN on a request with a
	// valid session cookie also logs that session out (MC13-LE0 13.04.15).
	CSRFKillsSession bool
	// NoASRR: the /api has no ASRock asrr namespace; every asrr/* path is
	// 404 {"code":1010} (Gigabyte).
	NoASRR bool
	// SyntheticAPI: no /api capture exists; the bodies are synthesized
	// (mz32APIFixtures).
	SyntheticAPI bool
	// RedfishTree: the Redfish resources come from
	// fixtures/<board>/redfish.json instead of the B650 tree.
	RedfishTree bool
	// MetadataNamespaces are OEM CSDL namespaces declared by the
	// unauthenticated $metadata, on top of the stock AMI ones.
	MetadataNamespaces []string
	// RedfishTruncatedPaging: collections stop at 50 members without a
	// nextLink and $skip<1 is a 400 (MC13-LE0 13.04.15).
	RedfishTruncatedPaging bool
	// RedfishLiteralDollar: query options are only recognised with a
	// literal "$"; percent-encoded "%24..." options are silently ignored
	// (MZ32-AR0 12.61.39).
	RedfishLiteralDollar bool
	// RedfishFilterPageLocal: $filter is applied to the returned page only
	// (MZ32-AR0 12.61.39).
	RedfishFilterPageLocal bool
	// RedfishStrictDollar: an unknown "$" query option is a 501
	// (MZ32-AR0 12.61.39).
	RedfishStrictDollar bool
}

// X470 is the X470D4U on BMC 2.32.00 (docs/x470/live-probe.md).
func X470() Profile {
	return Profile{Board: BoardX470, Vendor: "asrock", BoardName: "X470D4U", Firmware: "2.32.00", LegacyTLS: true, MaxSessions: 20}
}

// B650 is the B650D4U on BMC 4.10.00 (docs/b650/live-probe.md); /api
// bodies come from the 1.10.00 HAR with the firmware version overridden.
func B650() Profile {
	return Profile{Board: BoardB650, Vendor: "asrock", BoardName: "B650D4U", Firmware: "4.10.00",
		Captures:     []string{"b650d4u.har", "b650d4u-2t.har"},
		SecureCookie: true, LoginExtras: true, EventLogTrailer: true, ChassisCycleInterval: true,
		RedfishAuth: true, EvasiveMaxConcurrent: 10, MaxSessions: 20}
}

// X570 is the X570D4U on BMC 1.20.00 (docs/x570/live-probe.md); /api
// bodies come from the live read-only probe (docs/x570/har-endpoints.json),
// Redfish from the B650 tree reshaped to the X570 quirks (x570.go).
func X570() Profile {
	return Profile{Board: BoardX570, Vendor: "asrock", BoardName: "X570D4U", Firmware: "1.20.00",
		CookieAttrs: "; secure;HttpOnly", CSRFOnGET: true, LoginExtras: true,
		RedfishAuth: true, RedfishIgnoresQuery: true, EvasiveMaxConcurrent: 10, MaxSessions: 20}
}

// Options configure a Sim. Zero values pick the defaults noted.
type Options struct {
	Profile Profile
	// User/Pass accepted by /api and Redfish (default DefaultUser/DefaultPass).
	User, Pass string
	// Latency is added to every request.
	Latency time.Duration
	// ClockOffset is BMC clock - real time (reflected in the Date header,
	// settings/date-time, Redfish Manager.DateTime and new SEL records).
	ClockOffset time.Duration
	// PowerOn is the initial host power state.
	PowerOn bool
	// PowerDelay is how long a power transition takes to show up.
	PowerDelay time.Duration
	// PowerIgnoresSoftOff makes soft-off (code 5 / GracefulShutdown) a no-op.
	PowerIgnoresSoftOff bool
	// SessionIdle expires /api sessions idle this long (0 = never).
	SessionIdle time.Duration
	// MaxSessions overrides Profile.MaxSessions (free web session slots).
	MaxSessions int
	// HelpDataStatus is the status of the logs/help-data crash (default 500
	// with the lighttpd HTML page, as observed live on the X470; set 502 to
	// emulate the FastCGI-gone variant). Either one restarts the backend.
	HelpDataStatus int
}

// Request is one recorded request.
type Request struct {
	Time     time.Time
	Method   string // as received (before X-HTTP-Method-Override)
	Path     string
	RawQuery string
	Header   http.Header
	Body     string
	Status   int
	Session  string // AMI session id the request authenticated with, if any
}

// Stats are cumulative counters (see ResetStats).
type Stats struct {
	Requests        int
	Logins          int // successful /api + Redfish session logins
	LoginFailures   int
	Logouts         int
	Restarts        int // backend restarts (Restart, help-data, 502 handlers)
	CSRFRejects     int // 401 {cc:7} because of a missing/wrong CSRF token
	AuthRejects     int // 401 for a missing/invalid session
	MethodOverrides int // requests carrying X-HTTP-Method-Override (or X-HTTP-Method / X-Method-Override)
	Evasive403s     int
	MaxInFlight     int // all paths
	MaxInFlightAPI  int // /api/*
	MaxInFlightRF   int // /redfish/*
	FanInterleaves  int // X470 two-step fan select/read interleavings
	PowerActions    int
	IgnoredSoftOffs int
	DateTime500s    int // partial-body PUT settings/date-time
}

// Sim is a HAR-replay BMC simulator (an httptest TLS server).
type Sim struct {
	*httptest.Server
	opts     Options
	profile  Profile
	fixtures *FixtureSet
	rf       *redfishtest.Server // Redfish backend (B650, X570; nil on X470)
	x470Root []byte

	apiMu sync.Mutex // the single FastCGI worker: /api is serialised

	mu       sync.Mutex
	stats    Stats
	reqs     []Request
	inflight struct{ all, api, rf int }
	latency  time.Duration
	offset   atomic.Int64 // time.Duration; atomic so the Redfish fake can set it under its own lock
	evasive  int          // pending injected 403s
	evasiveN int          // concurrent cap
	wedged   chan struct{}
	closed   chan struct{}
	ami      amiState
	power    powerState
	events   eventStore
	fan      fanState
	oem      oemState
	dateTime map[string]any
	handlers map[string]http.HandlerFunc

	now     func() time.Time
	nextReq atomic.Int64
}

// Start creates a Sim and closes it when the test ends.
func Start(t testing.TB, o Options) *Sim {
	t.Helper()
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

// StartX470 / StartB650 / StartX570 start a Sim with the board's profile; mod adjusts
// the options first.
func StartX470(t testing.TB, mod ...func(*Options)) *Sim { return startProfile(t, X470(), mod) }

// StartB650 starts a B650D4U 4.10.00 Sim.
func StartB650(t testing.TB, mod ...func(*Options)) *Sim { return startProfile(t, B650(), mod) }

// StartX570 starts an X570D4U 1.20.00 Sim.
func StartX570(t testing.TB, mod ...func(*Options)) *Sim { return startProfile(t, X570(), mod) }

func startProfile(t testing.TB, p Profile, mod []func(*Options)) *Sim {
	o := Options{Profile: p, PowerOn: true}
	for _, m := range mod {
		m(&o)
	}
	return Start(t, o)
}

// New starts a Sim. Call Close when done.
func New(o Options) (*Sim, error) {
	if o.Profile.Board == "" {
		o.Profile = B650()
	}
	if o.User == "" {
		o.User = DefaultUser
	}
	if o.Pass == "" {
		o.Pass = DefaultPass
	}
	if o.MaxSessions == 0 {
		o.MaxSessions = o.Profile.MaxSessions
	}
	if o.HelpDataStatus == 0 {
		o.HelpDataStatus = http.StatusInternalServerError
	}
	var fx *FixtureSet
	if o.Profile.SyntheticAPI {
		fx = mz32APIFixtures()
	} else {
		var err error
		if fx, err = LoadFixtures(string(o.Profile.Board)); err != nil {
			return nil, err
		}
	}
	s := &Sim{opts: o, profile: o.Profile, fixtures: fx, latency: o.Latency,
		evasiveN: o.Profile.EvasiveMaxConcurrent, closed: make(chan struct{}),
		handlers: map[string]http.HandlerFunc{}, now: time.Now}
	s.offset.Store(int64(o.ClockOffset))
	s.ami.sessions = map[string]*amiSession{}
	s.power = powerState{on: o.PowerOn}
	if err := s.initEvents(); err != nil {
		return nil, err
	}
	if err := s.initDateTime(); err != nil {
		return nil, err
	}
	var err error
	if o.Profile.RedfishAuth {
		var tree map[string]map[string]any
		if o.Profile.RedfishTree {
			if tree, err = LoadRedfishTree(string(o.Profile.Board)); err != nil {
				return nil, err
			}
		}
		s.rf = redfishtest.New(o.User, o.Pass)
		s.rf.Lock()
		switch {
		case tree != nil:
			s.rf.Resources = tree
			s.rf.ETags = map[string]string{}
			s.rf.TruncatedPaging = o.Profile.RedfishTruncatedPaging
			gigabyteRedfish(s.rf.Resources)
		case o.Profile.Board == BoardX570:
			x570Redfish(s.rf.Resources)
		}
		s.rf.ClockSkew = o.ClockOffset
		// A Redfish DateTime PATCH moves the whole BMC clock.
		s.rf.OnClockSet = func(skew time.Duration) { s.offset.Store(int64(skew)) }
		// redfishtest's B650 tree carries the live board's real address;
		// never serve it from the sim.
		for path, res := range s.rf.Resources {
			s.rf.Resources[path] = scrubAddrs(res).(map[string]any)
		}
		s.rf.Unlock()
	} else {
		s.x470Root, err = rawFixture("x470", "redfish-serviceroot.json")
		if err != nil {
			return nil, err
		}
	}
	s.Server = httptest.NewUnstartedServer(http.HandlerFunc(s.serve))
	if o.Profile.LegacyTLS {
		// X470: RSA key exchange only, TLS 1.0-1.2 (Go clients must opt in).
		s.Server.TLS = &tls.Config{
			MinVersion: tls.VersionTLS10,
			MaxVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		}
	}
	s.Server.Config.ErrorLog = log.New(io.Discard, "", 0) // TLS handshake failures are expected (X470 vs modern clients)
	s.StartTLS()
	return s, nil
}

// Close stops the server, releasing any wedged requests first.
func (s *Sim) Close() {
	s.mu.Lock()
	select {
	case <-s.closed:
		s.mu.Unlock()
		return
	default:
		close(s.closed)
	}
	s.mu.Unlock()
	s.Server.CloseClientConnections()
	s.Server.Close()
	if s.rf != nil {
		s.rf.Close()
	}
}

// Profile returns the board profile.
func (s *Sim) Profile() Profile { return s.profile }

// Host returns "127.0.0.1:port" (an X-BMC-Host value).
func (s *Sim) Host() string { return s.Listener.Addr().String() }

// Target returns the bmc.Target of the sim.
func (s *Sim) Target() bmc.Target {
	t, err := bmc.ParseTarget(s.Host())
	if err != nil {
		panic(err)
	}
	return t
}

// Creds returns the accepted credentials.
func (s *Sim) Creds() bmc.Creds { return bmc.Creds{User: s.opts.User, Pass: s.opts.Pass} }

// Redfish returns the B650/X570 Redfish backend (nil on X470) for direct
// inspection (LiveSessions, AddSELEntry, Count...).
func (s *Sim) Redfish() *redfishtest.Server { return s.rf }

// Fixtures returns the board's fixture set.
func (s *Sim) Fixtures() *FixtureSet { return s.fixtures }

// ---- fault injection ----

// InjectEvasive403 makes the next n requests (any path) answer the lighttpd
// mod_evasive HTML 403.
func (s *Sim) InjectEvasive403(n int) {
	s.mu.Lock()
	s.evasive += n
	s.mu.Unlock()
}

// SetEvasiveMaxConcurrent sets the concurrent /redfish cap (0 = none).
func (s *Sim) SetEvasiveMaxConcurrent(n int) {
	s.mu.Lock()
	s.evasiveN = n
	s.mu.Unlock()
}

// Restart emulates a web backend restart (spx_restservice crash): every
// /api session and every Redfish session is dropped.
func (s *Sim) Restart() {
	s.mu.Lock()
	s.restartLocked()
	s.mu.Unlock()
}

func (s *Sim) restartLocked() {
	s.ami.sessions = map[string]*amiSession{}
	s.ami.pid++
	s.fan = fanState{}
	s.stats.Restarts++
	if s.rf != nil {
		s.rf.ExpireSessions()
	}
}

// SetLatency sets the per-request latency.
func (s *Sim) SetLatency(d time.Duration) {
	s.mu.Lock()
	s.latency = d
	s.mu.Unlock()
}

// SetPowerIgnoresSoftOff makes soft-off silently ignored (mid-POST).
func (s *Sim) SetPowerIgnoresSoftOff(v bool) {
	s.mu.Lock()
	s.opts.PowerIgnoresSoftOff = v
	s.mu.Unlock()
}

// SetPowerDelay sets how long power transitions take.
func (s *Sim) SetPowerDelay(d time.Duration) {
	s.mu.Lock()
	s.opts.PowerDelay = d
	s.mu.Unlock()
}

// SetPower forces the host power state.
func (s *Sim) SetPower(on bool) {
	s.mu.Lock()
	s.power = powerState{on: on}
	s.mu.Unlock()
}

// PowerOn reports the current host power state.
func (s *Sim) PowerOn() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.power.isOn(s.now())
}

// SetClockOffset sets BMC clock - real time.
func (s *Sim) SetClockOffset(d time.Duration) {
	s.mu.Lock()
	s.offset.Store(int64(d))
	s.mu.Unlock()
	if s.rf != nil {
		s.rf.Lock()
		s.rf.ClockSkew = d
		s.rf.Unlock()
	}
}

func (s *Sim) clockOffset() time.Duration { return time.Duration(s.offset.Load()) }

// ClockOffset returns BMC clock - real time.
func (s *Sim) ClockOffset() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clockOffset()
}

// BMCNow returns the BMC's notion of now.
func (s *Sim) BMCNow() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.now().Add(s.clockOffset())
}

// Wedge makes every request hang (no response) until Unwedge, the client
// gives up, or the Sim closes: for timeout and circuit-breaker tests.
func (s *Sim) Wedge() {
	s.mu.Lock()
	if s.wedged == nil {
		s.wedged = make(chan struct{})
	}
	s.mu.Unlock()
}

// Unwedge releases wedged requests (they then proceed normally).
func (s *Sim) Unwedge() {
	s.mu.Lock()
	if s.wedged != nil {
		close(s.wedged)
		s.wedged = nil
	}
	s.mu.Unlock()
}

// Handle overrides one route ("GET /api/sensors"); nil removes it.
func (s *Sim) Handle(methodPath string, h http.HandlerFunc) {
	s.mu.Lock()
	if h == nil {
		delete(s.handlers, methodPath)
	} else {
		s.handlers[methodPath] = h
	}
	s.mu.Unlock()
}

// ---- introspection ----

// Stats returns a snapshot of the counters.
func (s *Sim) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

// ResetStats zeroes the counters and the request log.
func (s *Sim) ResetStats() {
	s.mu.Lock()
	s.stats = Stats{}
	s.reqs = nil
	s.mu.Unlock()
	if s.rf != nil {
		s.rf.ResetLog()
	}
}

// Logins returns the number of successful logins.
func (s *Sim) Logins() int { return s.Stats().Logins }

// Requests returns a copy of the request log.
func (s *Sim) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.reqs...)
}

// Count returns how many logged requests match method ("" = any) and path
// prefix.
func (s *Sim) Count(method, pathPrefix string) int {
	n := 0
	for _, r := range s.Requests() {
		if (method == "" || r.Method == method) && strings.HasPrefix(r.Path, pathPrefix) {
			n++
		}
	}
	return n
}

// LiveSessions returns the number of live /api sessions plus live Redfish
// sessions.
func (s *Sim) LiveSessions() int {
	s.mu.Lock()
	n := len(s.ami.sessions)
	s.mu.Unlock()
	if s.rf != nil {
		n += s.rf.LiveSessions()
	}
	return n
}

// APISessions returns the number of live /api sessions.
func (s *Sim) APISessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.ami.sessions)
}

// ---- request pipeline ----

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(c int) {
	if w.status == 0 {
		w.status = c
	}
	w.ResponseWriter.WriteHeader(c)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

const evasiveBody = `<?xml version="1.0" encoding="iso-8859-1"?>
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en" lang="en">
 <head><title>403 - Forbidden</title></head>
 <body><h1>403 - Forbidden</h1></body>
</html>
`

func (s *Sim) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	sw := &statusWriter{ResponseWriter: w}
	rec := Request{Time: time.Now(), Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery, Header: r.Header.Clone(), Body: string(body)}
	isAPI := strings.HasPrefix(r.URL.Path, "/api/")
	isRF := strings.HasPrefix(r.URL.Path, "/redfish")

	s.mu.Lock()
	s.stats.Requests++
	if r.Header.Get("X-HTTP-Method-Override") != "" || r.Header.Get("X-HTTP-Method") != "" || r.Header.Get("X-Method-Override") != "" {
		s.stats.MethodOverrides++
	}
	s.inflight.all++
	s.stats.MaxInFlight = max(s.stats.MaxInFlight, s.inflight.all)
	if isAPI {
		s.inflight.api++
		s.stats.MaxInFlightAPI = max(s.stats.MaxInFlightAPI, s.inflight.api)
	}
	if isRF {
		s.inflight.rf++
		s.stats.MaxInFlightRF = max(s.stats.MaxInFlightRF, s.inflight.rf)
	}
	wedged, latency := s.wedged, s.latency
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.inflight.all--
		if isAPI {
			s.inflight.api--
		}
		if isRF {
			s.inflight.rf--
		}
		rec.Status = sw.status
		s.reqs = append(s.reqs, rec)
		s.mu.Unlock()
	}()

	if wedged != nil {
		select {
		case <-wedged:
		case <-r.Context().Done():
			return
		case <-s.closed:
			return
		}
	}
	if latency > 0 {
		t := time.NewTimer(latency)
		select {
		case <-t.C:
		case <-r.Context().Done():
			t.Stop()
			return
		case <-s.closed:
			t.Stop()
			return
		}
	}

	s.mu.Lock()
	sw.Header().Set("Date", s.now().Add(s.clockOffset()).UTC().Format(http.TimeFormat))
	sw.Header().Set("Server", "lighttpd")
	evasive := false
	if s.evasive > 0 {
		s.evasive--
		evasive = true
	} else if isRF && s.evasiveN > 0 && s.inflight.rf > s.evasiveN {
		evasive = true
	}
	if evasive {
		s.stats.Evasive403s++
	}
	h := s.handlers[r.Method+" "+r.URL.Path]
	s.mu.Unlock()

	switch {
	case evasive:
		sw.Header().Set("Content-Type", "text/html")
		sw.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(sw, evasiveBody)
	case h != nil:
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		h(sw, r)
	case isAPI:
		s.apiMu.Lock()
		rec.Session = s.serveAPI(sw, r, body)
		s.apiMu.Unlock()
	case isRF:
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		s.serveRedfish(sw, r, body)
	default:
		sw.Header().Set("Content-Type", "text/html")
		sw.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(sw, "<html><body><h1>404 - Not Found</h1></body></html>\n")
	}
}

var liveAddrRe = regexp.MustCompile(`\b10\.(?:7\.12|3\.10)\.(\d{1,3})\b`)

// scrubAddrs rewrites the live BMC subnets (10.7.12.0/24, 10.3.10.0/24) in
// every string to 192.0.2.0/24.
func scrubAddrs(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, vv := range x {
			x[k] = scrubAddrs(vv)
		}
		return x
	case []any:
		for i, vv := range x {
			x[i] = scrubAddrs(vv)
		}
		return x
	case string:
		return liveAddrRe.ReplaceAllString(x, "192.0.2.$1")
	}
	return v
}

func (s *Sim) String() string {
	return fmt.Sprintf("testsim(%s %s @ %s)", s.profile.BoardName, s.profile.Firmware, s.Host())
}
