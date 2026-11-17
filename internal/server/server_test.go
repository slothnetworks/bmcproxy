package server_test

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

const host = "10.9.9.9"

func fake() *servertest.Driver { return &servertest.Driver{NameV: "fake"} }

func TestTiersNotGated(t *testing.T) {
	// Disruptive tier is not gated: no flag or confirmation header needed.
	d := fake()
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	if r := h.Do(servertest.Req{Method: "DELETE", Path: "/api/v1/events", Host: host}); r.Status != 204 {
		t.Fatalf("events clear: %d %s", r.Status, r.Body)
	}
	// Write tier is not gated.
	h = servertest.New(t, servertest.Options{Drivers: []bmc.Driver{fake()}})
	if r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/led", Host: host, Body: map[string]string{"state": "on"}}); r.Status != 204 {
		t.Fatalf("led: %d %s", r.Status, r.Body)
	}
}

func TestErrorEnvelopeAndRequestID(t *testing.T) {
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{fake()}})
	r := h.Do(servertest.Req{Path: "/api/v1/boot", Host: host, Header: map[string]string{"X-Request-Id": "req-abc.123"}})
	if r.Status != 501 {
		t.Fatalf("status %d", r.Status)
	}
	if got := r.Header.Get("X-Request-Id"); got != "req-abc.123" {
		t.Fatalf("request id echo %q", got)
	}
	e := r.Envelope(t)
	if e.Error.Status != 501 || e.Error.Class != "unsupported" || e.Error.RequestID != "req-abc.123" || e.Error.Message == "" {
		t.Fatalf("envelope %+v", e)
	}
	if e.Error.Debug != nil {
		t.Fatal("debug block outside debug mode")
	}
	if !strings.Contains(string(r.Body), `"error":{`) {
		t.Fatalf("shape: %s", r.Body)
	}
	// Generated id when absent or invalid.
	r = h.Do(servertest.Req{Path: "/api/v1/boot", Host: host, Header: map[string]string{"X-Request-Id": "bad id with spaces"}})
	if id := r.Header.Get("X-Request-Id"); id == "" || id == "bad id with spaces" || r.Envelope(t).Error.RequestID != id {
		t.Fatalf("generated id %q", id)
	}
	// Unknown route -> JSON 404.
	r = h.Do(servertest.Req{Path: "/nope"})
	if r.Status != 404 || r.Envelope(t).Error.Class != "not_found" {
		t.Fatalf("404: %d %s", r.Status, r.Body)
	}
}

func TestDebugBlock(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Creds: map[string]string{"admin": "right"}}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}, DebugEndpoints: true})
	r := h.Do(servertest.Req{Path: "/api/v1/system", Host: host})
	e := r.Envelope(t)
	if r.Status != 401 || e.Error.Class != "auth" {
		t.Fatalf("status %d %+v", r.Status, e)
	}
	if e.Error.Debug == nil || e.Error.Debug.UpstreamStatus != 401 || e.Error.Debug.Backend != "redfish" || e.Error.Debug.Pkg != "fake" {
		t.Fatalf("debug block %+v", e.Error.Debug)
	}
}

func TestTargetAndCredsExtraction(t *testing.T) {
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{fake()}})
	if r := h.Do(servertest.Req{Path: "/api/v1/power"}); r.Status != 400 || r.Envelope(t).Error.Class != "invalid_argument" {
		t.Fatalf("missing host: %d %s", r.Status, r.Body)
	}
	if r := h.Do(servertest.Req{Path: "/api/v1/power", Host: host, NoAuth: true}); r.Status != 401 {
		t.Fatalf("missing auth: %d", r.Status)
	}
	if r := h.Do(servertest.Req{Path: "/api/v1/power", Host: host, Header: map[string]string{"X-BMC-Vendor": "nosuch"}}); r.Status != 400 {
		t.Fatalf("unknown vendor: %d %s", r.Status, r.Body)
	}
	// BMC rejecting credentials -> 401.
	d := &servertest.Driver{NameV: "strict", Creds: map[string]string{"admin": "right"}}
	h2 := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	if r := h2.Do(servertest.Req{Path: "/api/v1/power", Host: host, User: "admin", Pass: "wrong"}); r.Status != 401 || r.Envelope(t).Error.Class != "auth" {
		t.Fatalf("bad creds: %d %s", r.Status, r.Body)
	}
	if r := h2.Do(servertest.Req{Path: "/api/v1/power", Host: host, User: "admin", Pass: "right"}); r.Status != 200 {
		t.Fatalf("good creds: %d %s", r.Status, r.Body)
	}
}

func TestNoDriversIs501(t *testing.T) {
	h := servertest.New(t, servertest.Options{})
	if r := h.Do(servertest.Req{Path: "/api/v1/power", Host: host}); r.Status != 501 {
		t.Fatalf("status %d %s", r.Status, r.Body)
	}
}

func TestCredentialsNeverLogged(t *testing.T) {
	pass := "Zebra-Pa55word-XYZ"
	basic := base64.StdEncoding.EncodeToString([]byte("admin:" + pass))
	d := &servertest.Driver{NameV: "fake", Creds: map[string]string{"admin": pass}}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}, DebugEndpoints: true})
	h.Obs.Levels().SetDefault(-8) // TRACE everywhere
	h.Do(servertest.Req{Path: "/api/v1/sensors", Host: host, Pass: pass})
	h.Do(servertest.Req{Path: "/api/v1/sensors", Host: host, Pass: pass + "x"}) // auth failure path
	h.Do(servertest.Req{Method: "POST", Path: "/api/v1/power", Host: host, Pass: pass, Body: map[string]any{"action": "on", "password": pass}})
	logs := h.Logs.String()
	if !strings.Contains(logs, `"msg":"request"`) {
		t.Fatalf("no access log lines: %s", logs)
	}
	for _, secret := range []string{pass, basic} {
		if strings.Contains(logs, secret) {
			t.Fatalf("secret %q in logs:\n%s", secret, logs)
		}
	}
	r := h.Do(servertest.Req{Path: "/debug/sessions", NoAuth: true})
	if r.Status != 200 {
		t.Fatalf("debug sessions %d", r.Status)
	}
	if strings.Contains(string(r.Body), pass) || strings.Contains(string(r.Body), basic) || !strings.Contains(string(r.Body), `"user":"admin"`) {
		t.Fatalf("debug sessions body: %s", r.Body)
	}
}

func TestAccessLogLine(t *testing.T) {
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{fake()}})
	h.Do(servertest.Req{Path: "/api/v1/power", Host: host, Header: map[string]string{"X-Request-Id": "log-1"}})
	var line string
	for _, l := range strings.Split(h.Logs.String(), "\n") {
		if strings.Contains(l, `"msg":"request"`) && strings.Contains(l, "log-1") {
			line = l
		}
	}
	for _, want := range []string{`"subsys":"north"`, `"level":"INFO"`, `"status":200`, `"bmc":"10.9.9.9:443"`, `"bmc_user":"admin"`, `"req_id":"log-1"`} {
		if !strings.Contains(line, want) {
			t.Fatalf("access log %q missing %s", line, want)
		}
	}
}

func TestDebugEndpoints(t *testing.T) {
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{fake()}, DebugEndpoints: true})
	h.Do(servertest.Req{Path: "/api/v1/power", Host: host})
	for _, p := range []string{"/debug/targets", "/debug/sessions", "/debug/stats", "/debug/levels", "/debug/vars", "/debug/capabilities?host=" + host} {
		r := h.Do(servertest.Req{Path: p, NoAuth: true})
		if r.Status != 200 {
			t.Fatalf("%s: %d %s", p, r.Status, r.Body)
		}
	}
	r := h.Do(servertest.Req{Path: "/debug/capabilities?host=" + host, NoAuth: true})
	if !strings.Contains(string(r.Body), `"capability":"power","supported":true`) || !strings.Contains(string(r.Body), `"capability":"bootOverride","supported":false`) {
		t.Fatalf("capabilities: %s", r.Body)
	}
	// Disabled by default.
	h2 := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{fake()}})
	if r := h2.Do(servertest.Req{Path: "/debug/targets", NoAuth: true}); r.Status != 404 {
		t.Fatalf("debug without flag: %d", r.Status)
	}
}

func TestPanicRecovery(t *testing.T) {
	d := &servertest.Driver{NameV: "fake", Routes: []bmc.Route{{Method: "GET", Path: "boom", Tier: bmc.TierRead,
		Handler: func(bmc.Session, *http.Request) (any, error) { panic("kaboom") }}}}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	r := h.Do(servertest.Req{Path: "/api/v1/vendor/fake/boom", Host: host})
	if r.Status != 500 || r.Envelope(t).Error.Class != "internal" {
		t.Fatalf("panic: %d %s", r.Status, r.Body)
	}
	if !strings.Contains(h.Logs.String(), "panic in handler") {
		t.Fatal("panic not logged")
	}
}

func TestGracefulShutdownLogsOut(t *testing.T) {
	d := fake()
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = h.Server.Serve(ln) }()
	req, _ := http.NewRequest("GET", "http://"+ln.Addr().String()+"/api/v1/power", nil)
	req.Header.Set("X-BMC-Host", host)
	req.SetBasicAuth("admin", "pw")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.Server.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	for _, s := range d.Sessions() {
		if s.Closed.Load() != 1 {
			t.Fatal("session not logged out on shutdown")
		}
	}
}
