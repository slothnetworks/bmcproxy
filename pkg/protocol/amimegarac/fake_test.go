package amimegarac

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// Bodies below are taken from docs/b650/har-endpoints.json and
// docs/x470/har-endpoints.json (redacted values replaced).

const harChassisStatusB650 = `{"power_status": 1, "led_status": 0, "power_cycle_interval": 2000}`

// Two sensors from the B650 HAR /api/sensors sample: "NA" thresholds,
// integral values as ints, 17-digit floats.
const harSensorsB650 = `[{"id": 1, "sensor_number": 1, "name": "VOLT_3VSB", "owner_id": 32, "owner_lun": 0, "raw_reading": 167, "type": "voltage", "type_number": 2, "reading": 3.34, "sensor_state": 1, "discrete_state": 0, "accessible": 0, "settable_flag": 13878, "lower_non_recoverable_threshold": 2.86, "lower_critical_threshold": 3.04, "lower_non_critical_threshold": "NA", "higher_non_critical_threshold": "NA", "higher_critical_threshold": 3.72, "higher_non_recoverable_threshold": 3.88, "unit": "V"},
{"id": 2, "sensor_number": 2, "name": "VOLT_5VSB", "owner_id": 32, "owner_lun": 0, "raw_reading": 170, "type": "voltage", "type_number": 2, "reading": 5.1000000000000005, "sensor_state": 1, "discrete_state": 0, "accessible": 0, "settable_flag": 13878, "lower_non_recoverable_threshold": 4.26, "lower_critical_threshold": 4.5, "lower_non_critical_threshold": "NA", "higher_non_critical_threshold": "NA", "higher_critical_threshold": 5.49, "higher_non_recoverable_threshold": 5.76, "unit": "V"}]`

const harDateTimeX470 = `{"id": 1, "primary_ntp": "pool.ntp.org", "secondary_ntp": "time.nist.gov", "auto_date": 1, "localized_timestamp": 1794808815, "utc_minutes": 0, "daylight": 0, "timestamp": 1794808815, "timezone": "GMT+0"}`

const ccInvalidAuth = `{ "cc": 7, "error": "Invalid Authentication" }`

type seenReq struct {
	Method, Path string
	Header       http.Header
	Body         string
}

// fakeBMC emulates the MegaRAC /api session layer, from the live probes.
type fakeBMC struct {
	t   *testing.T
	srv *httptest.Server

	user, pass string
	certlogin  bool // require certlogin=0 (X470 style)

	mu          sync.Mutex
	sessions    map[string]string // QSESSIONID -> CSRF
	reqs        []seenReq
	rejectLogin bool
	writes      map[string]int // successful write count by path

	logins     atomic.Int32
	logouts    atomic.Int32
	hits       atomic.Int32 // all requests
	loginDelay time.Duration

	// handlers override routes (after the auth check), keyed by "METHOD /path".
	handlers map[string]http.HandlerFunc
}

func newFakeBMC(t *testing.T) *fakeBMC {
	f := &fakeBMC{t: t, user: "admin", pass: "s3cret", sessions: map[string]string{},
		writes: map[string]int{}, handlers: map[string]http.HandlerFunc{}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
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

func (f *fakeBMC) killAll() {
	f.mu.Lock()
	f.sessions = map[string]string{}
	f.mu.Unlock()
}

func (f *fakeBMC) setRejectLogin(v bool) {
	f.mu.Lock()
	f.rejectLogin = v
	f.mu.Unlock()
}

func (f *fakeBMC) handle(key string, h http.HandlerFunc) {
	f.mu.Lock()
	f.handlers[key] = h
	f.mu.Unlock()
}

func (f *fakeBMC) seen() []seenReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]seenReq(nil), f.reqs...)
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (f *fakeBMC) unauth(w http.ResponseWriter) {
	// Live behaviour: a 401 hands out a fresh, unauthenticated cookie.
	http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: randAlnum(30), Path: "/"})
	writeJSON(w, http.StatusUnauthorized, ccInvalidAuth)
}

func (f *fakeBMC) serve(w http.ResponseWriter, r *http.Request) {
	f.hits.Add(1)
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.reqs = append(f.reqs, seenReq{r.Method, r.URL.Path, r.Header.Clone(), string(body)})
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
	// No session, or a missing or wrong token -> 401 cc:7. Live, the X470
	// and B650 accept any non-empty token on GETs but enforce it on writes;
	// the X570 1.20 enforces it on every request, as this fake does.
	if !ok || r.Header.Get("X-CSRFTOKEN") != csrf {
		f.unauth(w)
		return
	}

	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	h := f.handlers[key]
	f.mu.Unlock()
	if h != nil {
		h(w, r)
		if r.Method != http.MethodGet {
			f.mu.Lock()
			f.writes[r.URL.Path]++
			f.mu.Unlock()
		}
		return
	}
	switch key {
	case "DELETE /api/session":
		f.logouts.Add(1)
		f.mu.Lock()
		delete(f.sessions, ck.Value)
		f.mu.Unlock()
		writeJSON(w, 200, `{ "ok": 0 }`)
	case "GET /api/chassis-status":
		writeJSON(w, 200, harChassisStatusB650)
	case "GET /api/sensors":
		writeJSON(w, 200, harSensorsB650)
	case "GET /api/settings/date-time":
		writeJSON(w, 200, harDateTimeX470)
	case "POST /api/actions/chassis-led":
		f.mu.Lock()
		f.writes[r.URL.Path]++
		f.mu.Unlock()
		writeJSON(w, 200, string(body))
	default:
		writeJSON(w, 404, `{ "error": "Invalid API Call", "code": 1010 }`)
	}
}

func (f *fakeBMC) login(w http.ResponseWriter, r *http.Request, body []byte) {
	f.logins.Add(1)
	if f.loginDelay > 0 {
		time.Sleep(f.loginDelay)
	}
	form, _ := url.ParseQuery(string(body))
	f.mu.Lock()
	reject := f.rejectLogin
	f.mu.Unlock()
	if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded; charset=UTF-8" ||
		reject || form.Get("username") != f.user || form.Get("password") != f.pass ||
		(f.certlogin && form.Get("certlogin") != "0") {
		f.unauth(w)
		return
	}
	sid, csrf := randAlnum(30), randAlnum(8)
	f.mu.Lock()
	f.sessions[sid] = csrf
	n := len(f.sessions)
	f.mu.Unlock()
	// B650 cookie attributes; X470 has only path=/.
	w.Header().Add("Set-Cookie", "QSESSIONID="+sid+"; path=/; secure; HttpOnly; Samesite=Lax")
	w.Header().Set("Date", time.Now().Add(-5*time.Second).UTC().Format(http.TimeFormat))
	// B650 login response shape (docs/b650/har-endpoints.json).
	writeJSON(w, 200, fmt.Sprintf(`{"ok": 0, "privilege": 4, "user_id": 2, "extendedpriv": 259, "racsession_id": %d, "remote_addr": "10.3.10.252", "server_name": "10.3.10.128", "server_addr": "10.3.10.128", "HTTPSEnabled": 1, "CSRFToken": %q, "channel": 1, "passwordStatus": 0}`, n+2, csrf))
}

// newClient builds a transport manager and a Client against f.
func newClient(t *testing.T, f *fakeBMC, mod func(*Config)) *Client {
	t.Helper()
	m := transport.NewManager(transport.Config{
		Obs:     obs.Discard(),
		Retry:   transport.RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Millisecond},
		Breaker: transport.BreakerConfig{Disabled: true},
	})
	t.Cleanup(m.Close)
	host := f.srv.Listener.Addr().String()
	tgt, err := bmc.ParseTarget(host)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Endpoint: m.Endpoint(tgt.Addr(), transport.LaneAMI),
		Target:   tgt,
		Creds:    bmc.Creds{User: f.user, Pass: f.pass},
		Obs:      obs.Discard(),
	}
	if mod != nil {
		mod(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
