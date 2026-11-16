package gigabyte

import (
	"net/http"
	"testing"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/redfish/redfishtest"
	"github.com/slothnetworks/bmcproxy/vendors/asrock"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
)

func TestRegistered(t *testing.T) {
	d, ok := bmc.Lookup(Name)
	if !ok || d.(bmc.Prioritized).Priority() != Priority {
		t.Fatalf("gigabyte not registered with priority %d: %v", Priority, d)
	}
	if _, ok := d.(bmc.CredDetector); !ok {
		t.Fatal("not a CredDetector")
	}
	if _, ok := d.(bmc.VendorRouter); !ok {
		t.Fatal("not a VendorRouter")
	}
}

func noAuthSent(t *testing.T, srv *redfishtest.Server) {
	t.Helper()
	for _, r := range srv.Reqs() {
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Auth-Token") != "" {
			t.Fatalf("credentials sent to %s %s", r.Method, r.Path)
		}
		if r.Path == "/api/session" {
			t.Fatalf("AMI login attempted")
		}
	}
}

func TestDetectMC13LE0Unauthenticated(t *testing.T) {
	f := newFake(t)
	m, err := f.driver().Detect(ctx, f.tg)
	if err != nil {
		t.Fatal(err)
	}
	if m.Score != scoreUnauth || m.Score <= 35 || m.Vendor != Name || m.Details["signal"] != "$metadata GBT OEM namespaces" ||
		m.Details["stack"] != "AMI MegaRAC" || m.Details["redfishVersion"] != "1.11.0" {
		t.Fatalf("match: %+v", m)
	}
	noAuthSent(t, f.srv)
}

func TestDetectWithCredsCertain(t *testing.T) {
	f := newFake(t)
	m, err := f.driver().DetectWithCreds(ctx, f.tg, bmc.Creds{User: testUser, Pass: testPass})
	if err != nil {
		t.Fatal(err)
	}
	if m.Score != bmc.ScoreCertain || m.Manufacturer != "Giga Computing" || m.Board != "MC13-LE0-000" ||
		m.Firmware != "13.04.15" || m.Details["profile"] != "mc13le0" || m.Details["verified"] != "true" {
		t.Fatalf("match: %+v", m)
	}
	if f.ami.logins != 0 {
		t.Fatal("DetectWithCreds logged in to the AMI /api")
	}
	// Bad creds: unauthenticated match kept, no error.
	m, err = f.driver().DetectWithCreds(ctx, f.tg, bmc.Creds{User: testUser, Pass: "wrong"})
	if err != nil || m.Score != scoreUnauth || m.Details["redfishAuth"] != "rejected" {
		t.Fatalf("bad creds: %+v %v", m, err)
	}
}

func TestRootMarkersWithoutMetadata(t *testing.T) {
	// A plain (non-AMI) Gigabyte Redfish root, e.g. a board whose service
	// names the vendor in the ServiceRoot.
	f := newFake(t)
	f.srv.Lock()
	root := f.srv.Resources["/redfish/v1"]
	root["Vendor"] = "GIGABYTE"
	root["Product"] = "GIGABYTE Redfish Server"
	delete(root, "Oem")
	delete(f.srv.Handlers, "/redfish/v1/$metadata")
	f.srv.Unlock()
	m, err := New(Options{Transport: f.tm, NoMetadataProbe: true}).Detect(ctx, f.tg)
	if err != nil || m.Score != scoreUnauth || m.Details["signal"] != "ServiceRoot Vendor/Product" || m.Details["stack"] != "" {
		t.Fatalf("match: %+v %v", m, err)
	}
	if f.srv.Count(http.MethodGet, "/redfish/v1/$metadata") != 0 {
		t.Fatal("NoMetadataProbe ignored")
	}
}

// ASRock (B650D4U 4.10 fixture, the redfishtest default) is not claimed and
// never sees credentials from this driver.
func TestASRockNotClaimed(t *testing.T) {
	srv := redfishtest.New(testUser, testPass)
	t.Cleanup(srv.Close)
	f := newFake(t) // only for its transport manager
	tg, _ := bmc.ParseTarget(srv.Host())
	d := New(Options{Transport: f.tm})
	m, err := d.Detect(ctx, tg)
	if err != nil || m.Score != 0 {
		t.Fatalf("B650 Detect: %+v %v", m, err)
	}
	m, err = d.DetectWithCreds(ctx, tg, bmc.Creds{User: testUser, Pass: testPass})
	if err != nil || m.Score != 0 {
		t.Fatalf("B650 DetectWithCreds: %+v %v", m, err)
	}
	noAuthSent(t, srv)

	// X470-like: AMI root whose $metadata has only ASRock/AMI namespaces.
	srv.Lock()
	srv.Handlers["/redfish/v1/$metadata"] = func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<Schema Namespace="AMIManager"/><Schema Namespace="AsrockRack.v1_0_0"/>`))
	}
	srv.Unlock()
	if m, err := d.DetectWithCreds(ctx, tg, bmc.Creds{User: testUser, Pass: testPass}); err != nil || m.Score != 0 {
		t.Fatalf("X470-like: %+v %v", m, err)
	}
	noAuthSent(t, srv)

	// Not Redfish at all -> zero, no error.
	srv.Lock()
	srv.Handlers["/redfish/v1/"] = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>login</html>"))
	}
	srv.Handlers["/redfish/v1"] = srv.Handlers["/redfish/v1/"]
	srv.Unlock()
	if m, err := New(Options{Transport: f.tm}).Detect(ctx, tg); err != nil || m.Score != 0 {
		t.Fatalf("non-redfish: %+v %v", m, err)
	}
}

// With the real ASRock and generic drivers registered, the MC13-LE0
// resolves to gigabyte, and ASRock's credentialed /api probe never runs.
func TestResolvePrefersGigabyte(t *testing.T) {
	f := newFake(t)
	reg := bmc.NewRegistry()
	reg.Register(f.driver())
	reg.Register(asrock.New(asrock.Options{Transport: f.tm}))
	reg.Register(genericredfish.New(f.tm))

	d, m, err := reg.Resolve(ctx, f.tg, &bmc.Creds{User: testUser, Pass: testPass})
	if err != nil || d.Name() != Name || m.Score != bmc.ScoreCertain {
		t.Fatalf("resolve with creds: %v %+v %v", d, m, err)
	}
	if f.ami.logins != 0 {
		t.Fatalf("an AMI /api login happened during resolve (%d)", f.ami.logins)
	}
	d, m, err = reg.Resolve(ctx, f.tg, nil)
	if err != nil || d.Name() != Name || m.Score != scoreUnauth {
		t.Fatalf("resolve without creds: %v %+v %v", d, m, err)
	}
}

func TestIsGigabyteManufacturer(t *testing.T) {
	for s, want := range map[string]bool{"Giga Computing": true, "GIGABYTE": true, "Giga-Byte Technology": true,
		"ASRockRack": false, "AMI": false, "": false, "Supermicro": false} {
		if IsGigabyteManufacturer(s) != want {
			t.Errorf("%q: want %v", s, want)
		}
	}
}
