package gigabyte

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// The fake is a redfishtest.Server loaded with the sanitized MC13-LE0
// 13.04.15 capture (testdata/mc13le0-13.04.15.json, generated from the live
// probe by the scratchpad gen.py) and the live-observed quirks: collections
// truncated at 50 without nextLink ($skip=0 -> 400), an unauthenticated
// $metadata declaring the GBT namespaces, and the AMI /api on the same port
// with CSRF enforced on every method (a bad token kills the session).

var ctx = context.Background()

const (
	fixtureFile = "testdata/mc13le0-13.04.15.json"
	testUser    = "admin"
	testPass    = "test-only-secret"
	fakeSerial  = "TESTBOARDSN01"
)

type fixture struct {
	Redfish map[string]map[string]any `json:"redfish"`
	API     map[string]struct {
		Status int `json:"status"`
		Body   any `json:"body"`
	} `json:"api"`
}

func loadFixture(t testing.TB, file string) fixture {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// fakeAMI is the MegaRAC /api half of the fake.
type fakeAMI struct {
	mu       sync.Mutex
	user     string
	pass     string
	sessions map[string]string // QSESSIONID -> CSRF token
	mode     string
	logins   int
	logouts  int
	posts    []string // bodies of POST settings/fanprofile/mode
	api      map[string]any
	srv      *redfishtest.Server
}

// body returns the request body: redfishtest.Server reads it before calling
// a Handler, so take it from the request log (requests are sequential).
func (a *fakeAMI) body(r *http.Request) string {
	reqs := a.srv.Reqs()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Path == r.URL.Path && reqs[i].Method == r.Method {
			return reqs[i].Body
		}
	}
	return ""
}

func newToken() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// check enforces cookie + CSRF on every request (MC13-LE0 13.04.15: even
// GETs; a bad token logs the session out).
func (a *fakeAMI) check(w http.ResponseWriter, r *http.Request) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	ck, err := r.Cookie("QSESSIONID")
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"cc": 7})
		return false
	}
	tok, ok := a.sessions[ck.Value]
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"cc": 7})
		return false
	}
	if r.Header.Get("X-CSRFTOKEN") != tok {
		delete(a.sessions, ck.Value)
		writeJSON(w, http.StatusUnauthorized, map[string]any{"cc": 7})
		return false
	}
	return true
}

func (a *fakeAMI) session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		b := a.body(r)
		if formValue(b, "username") != a.user || formValue(b, "password") != a.pass {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"cc": 1, "code": 15000, "error": "Authentication Failed"})
			return
		}
		sid, csrf := newToken(), newToken()
		a.mu.Lock()
		a.sessions[sid] = csrf
		a.logins++
		a.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: "QSESSIONID", Value: sid, Path: "/", Secure: true, HttpOnly: true})
		writeJSON(w, http.StatusOK, map[string]any{"ok": 0, "privilege": 4, "extendedpriv": 259, "racsession_id": 3,
			"remote_addr": "192.0.2.10", "server_name": "192.0.2.20", "server_addr": "192.0.2.20", "HTTPSEnabled": 1,
			"CSRFToken": csrf, "channel": 1, "passwordStatus": 0})
	case http.MethodDelete:
		if !a.check(w, r) {
			return
		}
		ck, _ := r.Cookie("QSESSIONID")
		a.mu.Lock()
		delete(a.sessions, ck.Value)
		a.logouts++
		a.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": 0})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"cc": 1})
	}
}

func (a *fakeAMI) fanMode(w http.ResponseWriter, r *http.Request) {
	if !a.check(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.mu.Lock()
		m := a.mode
		a.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"strMode": m})
	case http.MethodPost, http.MethodPut:
		var body struct {
			StrMode string `json:"strMode"`
		}
		raw := strings.TrimSpace(a.body(r))
		_ = json.Unmarshal([]byte(raw), &body)
		a.mu.Lock()
		a.posts = append(a.posts, raw)
		a.mode = body.StrMode
		a.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"strMode": body.StrMode})
	}
}

func (a *fakeAMI) static(w http.ResponseWriter, r *http.Request) {
	if !a.check(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"cc": 1})
		return
	}
	b, ok := a.api[r.URL.Path]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Invalid API Call", "code": 1010})
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (a *fakeAMI) liveSessions() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sessions)
}

type fake struct {
	srv *redfishtest.Server
	ami *fakeAMI
	tm  *transport.Manager
	tg  bmc.Target
}

const metadataXML = `<?xml version="1.0" encoding="UTF-8"?>
<edmx:Edmx xmlns:edmx="http://docs.oasis-open.org/odata/ns/edmx" Version="4.0">
<edmx:DataServices>
<Schema xmlns="http://docs.oasis-open.org/odata/ns/edm" Namespace="GBTChassisOemProperty"/>
<Schema xmlns="http://docs.oasis-open.org/odata/ns/edm" Namespace="GBTFanprofileService.v1_0_0"/>
<Schema xmlns="http://docs.oasis-open.org/odata/ns/edm" Namespace="GbtPowerLimit"/>
</edmx:DataServices>
</edmx:Edmx>`

// newFake starts the MC13-LE0 fake.
func newFake(t testing.TB) *fake { return newFakeFrom(t, fixtureFile, true) }

// newFakeFrom starts a fake from a fixture file; truncated selects the
// MC13-LE0 13.x paging bug (else standard nextLink paging, as MZ32-AR0).
func newFakeFrom(t testing.TB, file string, truncated bool) *fake {
	t.Helper()
	fx := loadFixture(t, file)
	srv := redfishtest.New(testUser, testPass)
	t.Cleanup(srv.Close)
	srv.Lock()
	srv.Resources = fx.Redfish
	srv.ETags = map[string]string{}
	srv.TruncatedPaging = truncated
	// The capture redacts the serial; give the test a real-looking one.
	if ch, ok := srv.Resources["/redfish/v1/Chassis/Self"]; ok {
		ch["Oem"].(map[string]any)["GBTChassisOemProperty"].(map[string]any)["Board Serial Number"] = fakeSerial
	}
	srv.Handlers["/redfish/v1/$metadata"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(metadataXML))
	}
	a := &fakeAMI{user: testUser, pass: testPass, sessions: map[string]string{}, mode: "NEW_PROFILE", api: map[string]any{}, srv: srv}
	srv.Handlers["/api/session"] = a.session
	srv.Handlers["/api/settings/fanprofile/mode"] = a.fanMode
	for k, v := range fx.API {
		p := strings.TrimPrefix(k, "GET ")
		if p == "/api/settings/fanprofile/mode" {
			continue
		}
		a.api[p] = v.Body
		srv.Handlers[p] = a.static
	}
	srv.Unlock()
	tm := transport.NewManager(transport.Config{Retry: transport.RetryPolicy{MaxAttempts: 1}, Breaker: transport.BreakerConfig{Disabled: true}})
	t.Cleanup(tm.Close)
	tg, err := bmc.ParseTarget(srv.Host())
	if err != nil {
		t.Fatal(err)
	}
	return &fake{srv: srv, ami: a, tm: tm, tg: tg}
}

func (f *fake) driver() *Driver {
	return New(Options{Transport: f.tm, RedfishOptions: rf.Options{TaskPollInterval: time.Millisecond}})
}

func (f *fake) open(t testing.TB) *Session {
	t.Helper()
	s, err := f.driver().Open(ctx, f.tg, bmc.Creds{User: testUser, Pass: testPass})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })
	return s.(*Session)
}

// formValue is url.Values.Get for a raw body.
func formValue(body, k string) string {
	v, _ := url.ParseQuery(body)
	return v.Get(k)
}
