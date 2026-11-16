package asrock

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/ipmi"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// Test credentials (fake only).
const (
	testUser = "admin"
	testPass = "s3cret-Pa55"
)

// X470D4U 2.32 Redfish ServiceRoot shape (docs/x470/live-probe.md): v1_3_1,
// RedfishVersion 1.5.0, no Vendor.
const x470ServiceRoot = `{"@odata.context":"/redfish/v1/$metadata#ServiceRoot.ServiceRoot","@odata.id":"/redfish/v1/","@odata.type":"#ServiceRoot.v1_3_1.ServiceRoot","Id":"RootService","Name":"Root Service","RedfishVersion":"1.5.0","UUID":"00000000-0000-0000-0000-000000000000","Systems":{"@odata.id":"/redfish/v1/Systems"},"Managers":{"@odata.id":"/redfish/v1/Managers"},"SessionService":{"@odata.id":"/redfish/v1/SessionService"}}`

const ccInvalidAuth = `{ "cc": 7, "error": "Invalid Authentication" }`

type fixResp struct {
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

func loadFixture(t testing.TB, name string) map[string]fixResp {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Responses map[string]fixResp `json:"responses"`
	}
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f.Responses
}

type seenReq struct {
	Method, Path, Query, Body string
	At                        time.Time
}

// fakeBMC emulates an ASRock Rack MegaRAC: the /api session layer (cookie +
// CSRF, cc:7 on bad auth), fixture replay from testdata, and Redfish either
// as X470 (root only, 401 for /api users) or delegated to redfishtest (B650).
type fakeBMC struct {
	t   testing.TB
	srv *httptest.Server

	user, pass string
	certlogin  bool // require certlogin=0 (X470)
	trailer    bool // append {"event_log_count":N} to logs/event (B650)

	mu        sync.Mutex
	sessions  map[string]string
	reqs      []seenReq
	responses map[string]fixResp
	handlers  map[string]http.HandlerFunc

	rf *redfishtest.Server // nil: X470-style Redfish

	logins, logouts atomic.Int32
}

func newFake(t testing.TB, fixture string) *fakeBMC {
	f := &fakeBMC{t: t, user: testUser, pass: testPass, sessions: map[string]string{},
		responses: loadFixture(t, fixture), handlers: map[string]http.HandlerFunc{}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

// newX470 is the X470D4U 2.32 fake.
func newX470(t testing.TB) *fakeBMC {
	f := newFake(t, "x470d4u-2.32.json")
	f.certlogin = true
	return f
}

// newB650 is the B650D4U fake with Redfish from redfishtest. fw sets
// asrr/fw-info BMC_fw_version ("" keeps the 1.10 capture).
func newB650(t testing.TB, fw string) *fakeBMC {
	f := newFake(t, "b650d4u-1.10.json")
	f.trailer = true
	f.rf = redfishtest.New(testUser, testPass)
	t.Cleanup(f.rf.Close)
	if fw != "" {
		var m map[string]any
		_ = json.Unmarshal(f.responses["GET /api/asrr/fw-info"].Body, &m)
		m["BMC_fw_version"] = fw
		f.setJSON("GET /api/asrr/fw-info", 200, m)
	}
	return f
}

func (f *fakeBMC) host() string { return f.srv.Listener.Addr().String() }

func (f *fakeBMC) target(t testing.TB) bmc.Target {
	tg, err := bmc.ParseTarget(f.host())
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

func (f *fakeBMC) handle(key string, h http.HandlerFunc) {
	f.mu.Lock()
	f.handlers[key] = h
	f.mu.Unlock()
}

func (f *fakeBMC) setJSON(key string, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		f.t.Fatal(err)
	}
	f.mu.Lock()
	f.responses[key] = fixResp{Status: status, Body: b}
	f.mu.Unlock()
}

func (f *fakeBMC) killAll() {
	f.mu.Lock()
	f.sessions = map[string]string{}
	f.mu.Unlock()
}

func (f *fakeBMC) seen() []seenReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seenReq(nil), f.reqs...)
}

func (f *fakeBMC) seenPath(method, p string) []seenReq {
	var out []seenReq
	for _, r := range f.seen() {
		if r.Method == method && r.Path == p {
			out = append(out, r)
		}
	}
	return out
}

func randAlnum(n int) string {
	const a = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = a[int(b[i])%len(a)]
	}
	return string(b)
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (f *fakeBMC) unauth(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: randAlnum(30), Path: "/"})
	writeJSON(w, http.StatusUnauthorized, ccInvalidAuth)
}

func (f *fakeBMC) serve(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/redfish") {
		f.serveRedfish(w, r)
		return
	}
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, seenReq{r.Method, r.URL.Path, r.URL.RawQuery, string(body), time.Now()})
	f.mu.Unlock()
	w.Header().Set("Server", "lighttpd")

	if r.URL.Path == "/api/session" && r.Method == http.MethodPost {
		f.login(w, r, body)
		return
	}
	ck, _ := r.Cookie("QSESSIONID")
	f.mu.Lock()
	csrf, ok := "", false
	if ck != nil {
		csrf, ok = f.sessions[ck.Value]
	}
	f.mu.Unlock()
	if !ok || r.Header.Get("X-CSRFTOKEN") != csrf {
		f.unauth(w)
		return
	}
	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	h := f.handlers[key]
	resp, haveQ := f.responses[key+"?"+r.URL.RawQuery]
	if !haveQ {
		resp, ok = f.responses[key]
	} else {
		ok = true
	}
	f.mu.Unlock()
	if h != nil {
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		h(w, r)
		return
	}
	switch {
	case key == "DELETE /api/session":
		f.logouts.Add(1)
		f.mu.Lock()
		delete(f.sessions, ck.Value)
		f.mu.Unlock()
		writeJSON(w, 200, `{ "ok": 0 }`)
		return
	case key == "GET /api/logs/event":
		f.serveEvents(w, r)
		return
	case ok:
		st := resp.Status
		if st == 0 {
			st = 200
		}
		writeJSON(w, st, string(resp.Body))
		return
	}
	writeJSON(w, 404, `{ "error": "Invalid API Call", "code": 1010 }`)
}

// serveEvents implements logs/event?LASTEVENTID=n from the logs/eventlog
// fixture (ids > n), with the B650 trailer when enabled.
func (f *fakeBMC) serveEvents(w http.ResponseWriter, r *http.Request) {
	var all []map[string]any
	f.mu.Lock()
	for k, v := range f.responses {
		if strings.HasPrefix(k, "GET /api/logs/eventlog") {
			_ = json.Unmarshal(v.Body, &all)
			break
		}
	}
	f.mu.Unlock()
	last, _ := strconv.Atoi(r.URL.Query().Get("LASTEVENTID"))
	out := []any{}
	for _, e := range all {
		id, _ := e["id"].(float64)
		if int(id) > last {
			out = append(out, e)
		}
	}
	if f.trailer {
		out = append(out, map[string]any{"event_log_count": len(all)})
	}
	b, _ := json.Marshal(out)
	writeJSON(w, 200, string(b))
}

func (f *fakeBMC) serveRedfish(w http.ResponseWriter, r *http.Request) {
	if f.rf != nil {
		f.rf.Config.Handler.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/redfish/v1/" || r.URL.Path == "/redfish/v1" {
		writeJSON(w, 200, x470ServiceRoot)
		return
	}
	// X470: the /api users are not Redfish accounts.
	w.Header().Set("WWW-Authenticate", `Basic realm="Redfish"`)
	writeJSON(w, 401, `{"error":{"code":"Base.1.4.GeneralError","message":"Unauthorized"}}`)
}

func (f *fakeBMC) login(w http.ResponseWriter, r *http.Request, body []byte) {
	f.logins.Add(1)
	form, _ := url.ParseQuery(string(body))
	if form.Get("username") != f.user || form.Get("password") != f.pass || (f.certlogin && form.Get("certlogin") != "0") {
		f.unauth(w)
		return
	}
	sid, csrf := randAlnum(30), randAlnum(8)
	f.mu.Lock()
	f.sessions[sid] = csrf
	n := len(f.sessions)
	f.mu.Unlock()
	w.Header().Add("Set-Cookie", "QSESSIONID="+sid+"; path=/; secure; HttpOnly")
	writeJSON(w, 200, fmt.Sprintf(`{"ok": 0, "privilege": 4, "user_id": 2, "extendedpriv": 259, "racsession_id": %d, "CSRFToken": %q, "channel": 1}`, n+2, csrf))
}

// ---- fake IPMI ----

type fakeIPMI struct {
	mu       sync.Mutex
	powerOn  bool
	restore  ipmi.RestorePolicyCode
	boot     ipmi.BootFlags
	controls []ipmi.ChassisControl
	dials    int
	resets   []string
}

func (f *fakeIPMI) dial(context.Context, string, bmc.Creds, ipmi.Options) (ipmi.Conn, error) {
	f.mu.Lock()
	f.dials++
	f.mu.Unlock()
	return f, nil
}

func (f *fakeIPMI) ChassisStatus(context.Context) (ipmi.ChassisStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return ipmi.ChassisStatus{PowerOn: f.powerOn, RestorePolicy: f.restore}, nil
}
func (f *fakeIPMI) ChassisControl(_ context.Context, c ipmi.ChassisControl) error {
	f.mu.Lock()
	f.controls = append(f.controls, c)
	f.mu.Unlock()
	return nil
}
func (f *fakeIPMI) ChassisIdentify(context.Context, uint8, bool) error { return nil }
func (f *fakeIPMI) SetBootDevice(_ context.Context, d ipmi.BootDevice, efi, persist bool) error {
	f.mu.Lock()
	f.boot = ipmi.BootFlags{Valid: true, Device: d, EFI: efi, Persist: persist}
	f.mu.Unlock()
	return nil
}
func (f *fakeIPMI) BootFlags(context.Context) (ipmi.BootFlags, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.boot, nil
}
func (f *fakeIPMI) SetPowerRestorePolicy(_ context.Context, p ipmi.RestorePolicyCode) error {
	f.mu.Lock()
	f.restore = p
	f.mu.Unlock()
	return nil
}
func (f *fakeIPMI) Sensors(context.Context) ([]ipmi.SensorRecord, error) { return nil, nil }
func (f *fakeIPMI) SEL(context.Context) ([]ipmi.SELRecord, error)        { return nil, nil }
func (f *fakeIPMI) ClearSEL(context.Context) error                       { return nil }
func (f *fakeIPMI) FRUs(context.Context) ([]bmc.FRU, error)              { return nil, nil }
func (f *fakeIPMI) DeviceID(context.Context) (ipmi.DeviceInfo, error)    { return ipmi.DeviceInfo{}, nil }
func (f *fakeIPMI) ColdReset(context.Context) error {
	f.mu.Lock()
	f.resets = append(f.resets, "cold")
	f.mu.Unlock()
	return nil
}
func (f *fakeIPMI) WarmReset(context.Context) error {
	f.mu.Lock()
	f.resets = append(f.resets, "warm")
	f.mu.Unlock()
	return nil
}
func (f *fakeIPMI) LANConfig(context.Context, uint8) (ipmi.LANConfig, error) {
	return ipmi.LANConfig{}, nil
}
func (f *fakeIPMI) Users(context.Context, uint8) ([]ipmi.UserRecord, error) { return nil, nil }
func (f *fakeIPMI) Close(context.Context) error                             { return nil }

// ---- driver helpers ----

func newTransport(t testing.TB) *transport.Manager {
	m := transport.NewManager(transport.Config{
		Obs:     obs.Discard(),
		Retry:   transport.RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond},
		Breaker: transport.BreakerConfig{Disabled: true},
	})
	t.Cleanup(m.Close)
	return m
}

func newTestDriver(t testing.TB, ip *fakeIPMI) *Driver {
	if ip == nil {
		ip = &fakeIPMI{}
	}
	return New(Options{
		Transport:  newTransport(t),
		IPMIDialer: ip.dial,
		AMIConfig:  amimegarac.Config{LoginFailureBackoff: time.Millisecond},
	})
}

// openSession opens a driver session against f.
func openSession(t testing.TB, d *Driver, f *fakeBMC) *Session {
	t.Helper()
	ctx := context.Background()
	s, err := d.Open(ctx, f.target(t), bmc.Creds{User: testUser, Pass: testPass})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s.(*Session)
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
