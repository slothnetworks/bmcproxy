package vendorroutes_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

const host = "10.4.4.4"

func TestVendorRoutes(t *testing.T) {
	d := &servertest.Driver{NameV: "acme", Routes: servertest.SampleRoutes()}
	other := &servertest.Driver{NameV: "zeta", Score: -1, Routes: []bmc.Route{{Method: "GET", Path: "ping", Handler: func(s bmc.Session, _ *http.Request) (any, error) { return "pong", nil }}}}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d, other}})

	r := h.Do(servertest.Req{Path: "/api/v1/vendor/acme/bonding", Host: host})
	if r.Status != 200 || !strings.Contains(string(r.Body), `"user":"admin"`) {
		t.Fatalf("read route: %d %s", r.Status, r.Body)
	}
	r = h.Do(servertest.Req{Path: "/api/v1/vendor/acme/componentStatus/BIOS", Host: host})
	if r.Status != 200 || !strings.Contains(string(r.Body), "BIOS") {
		t.Fatalf("path value: %d %s", r.Status, r.Body)
	}
	r = h.Do(servertest.Req{Method: "POST", Path: "/api/v1/vendor/acme/setFanMode", Host: host, Body: map[string]string{"mode": "full"}})
	if r.Status != 200 || !strings.Contains(string(r.Body), "full") {
		t.Fatalf("write route: %d %s", r.Status, r.Body)
	}
	// Disruptive gate.
	r = h.Do(servertest.Req{Method: "POST", Path: "/api/v1/vendor/acme/disableBonding", Host: host, Header: map[string]string{"X-BMC-Confirm": "true"}})
	if r.Status != 403 || r.Envelope(t).Error.Class != "disruptive_denied" {
		t.Fatalf("disruptive: %d %s", r.Status, r.Body)
	}
	// Dynamic tier: GET passthrough is read, POST is disruptive, denylist is 403 forbidden.
	r = h.Do(servertest.Req{Path: "/api/v1/vendor/acme/raw/api/chassis-status", Host: host})
	if r.Status != 207 || r.Header.Get("X-Raw") != "1" || string(r.Body) != "raw:chassis-status" {
		t.Fatalf("raw responder: %d %s", r.Status, r.Body)
	}
	if r = h.Do(servertest.Req{Method: "POST", Path: "/api/v1/vendor/acme/raw/api/x", Host: host}); r.Status != 403 {
		t.Fatalf("raw POST: %d", r.Status)
	}
	if r = h.Do(servertest.Req{Path: "/api/v1/vendor/acme/raw/api/logs/help-data", Host: host}); r.Status != 403 || r.Envelope(t).Error.Class != "forbidden" {
		t.Fatalf("denylist: %d %s", r.Status, r.Body)
	}
	if r = h.Do(servertest.Req{Path: "/api/v1/vendor/acme/unsupportedThing", Host: host}); r.Status != 501 {
		t.Fatalf("unsupported: %d", r.Status)
	}
	// Vendor mismatch: target resolves to acme, URL says zeta.
	if r = h.Do(servertest.Req{Path: "/api/v1/vendor/zeta/ping", Host: host}); r.Status != 404 || r.Envelope(t).Error.Class != "not_found" {
		t.Fatalf("mismatch: %d %s", r.Status, r.Body)
	}
	// Header conflicting with URL vendor -> 400.
	if r = h.Do(servertest.Req{Path: "/api/v1/vendor/acme/bonding", Host: host, Header: map[string]string{"X-BMC-Vendor": "zeta"}}); r.Status != 400 {
		t.Fatalf("header conflict: %d %s", r.Status, r.Body)
	}
	// Unknown vendor prefix -> 404.
	if r = h.Do(servertest.Req{Path: "/api/v1/vendor/nobody/x", Host: host}); r.Status != 404 {
		t.Fatalf("unknown vendor: %d", r.Status)
	}
	if d.Opens.Load() != 1 {
		t.Fatalf("opens %d", d.Opens.Load())
	}
}

func TestVendorRoutesDisruptiveAllowed(t *testing.T) {
	d := &servertest.Driver{NameV: "acme", Routes: servertest.SampleRoutes()}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}, AllowDisruptive: true})
	r := h.Do(servertest.Req{Method: "POST", Path: "/api/v1/vendor/acme/disableBonding", Host: host, Header: map[string]string{"X-BMC-Confirm": "true"}})
	if r.Status != 204 {
		t.Fatalf("nil result -> 204, got %d %s", r.Status, r.Body)
	}
	// Vendor routes appear in /capabilities.
	r = h.Do(servertest.Req{Path: "/api/v1/capabilities", Host: host})
	if !strings.Contains(string(r.Body), "/api/v1/vendor/acme/disableBonding") || !strings.Contains(string(r.Body), `"tier":"disruptive"`) {
		t.Fatalf("capabilities: %s", r.Body)
	}
}

func TestBadRouteSkipped(t *testing.T) {
	d := &servertest.Driver{NameV: "acme", Routes: []bmc.Route{
		{Method: "GET", Path: "ok", Handler: func(bmc.Session, *http.Request) (any, error) { return 1, nil }},
		{Method: "GET", Path: "ok", Handler: func(bmc.Session, *http.Request) (any, error) { return 2, nil }}, // duplicate
		{Method: "GET", Path: "nohandler"},
	}}
	h := servertest.New(t, servertest.Options{Drivers: []bmc.Driver{d}})
	if r := h.Do(servertest.Req{Path: "/api/v1/vendor/acme/ok", Host: host}); r.Status != 200 {
		t.Fatalf("%d", r.Status)
	}
	if !strings.Contains(h.Logs.String(), "vendor route not mounted") {
		t.Fatal("conflict not logged")
	}
}
