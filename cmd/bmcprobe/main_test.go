package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/internal/testsim"
	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

func runProbe(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(context.Background(), args, &out, &errb)
	return code, out.String(), errb.String()
}

func withPassword(t *testing.T, sim *testsim.Sim) []string {
	t.Setenv("BMCPROBE_TEST_PASS", sim.Creds().Pass)
	return []string{"--host", sim.Host(), "--user", sim.Creds().User, "--password-env", "BMCPROBE_TEST_PASS"}
}

func TestDetectB650(t *testing.T) {
	sim := testsim.StartB650(t)

	// Unauthenticated: driver + TLS only.
	code, out, errs := runProbe(t, "--host", sim.Host(), "--json", "detect")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var rep DetectReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	// Unauthenticated, the asrock driver sees an AMI MegaRAC /api plus an
	// AMI Redfish root and outranks the generic Redfish fallback.
	if rep.Driver != "asrock" || rep.Score <= 10 || rep.Details["redfishVersion"] != "1.15.1" || rep.TLS == nil || rep.Session != nil {
		t.Fatalf("detect: %+v", rep)
	}
	// The asrock auth-gate probe (an unauthenticated GET that answers 401
	// cc:7) is expected; no credentials may be sent.
	if sim.Logins() != 0 || sim.Stats().LoginFailures != 0 || sim.Count("POST", "/api/session") != 0 {
		t.Fatal("unauthenticated detect must not try credentials")
	}
	for _, r := range sim.Requests() {
		if r.Header.Get("Authorization") != "" || r.Header.Get("X-Auth-Token") != "" || r.Header.Get("Cookie") != "" {
			t.Fatalf("unauthenticated detect sent credentials on %s %s", r.Method, r.Path)
		}
	}

	// With credentials (flags after the command work too).
	args := append([]string{"detect", "--json"}, withPassword(t, sim)...)
	code, out, errs = runProbe(t, args...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	rep = DetectReport{}
	_ = json.Unmarshal([]byte(out), &rep)
	if rep.Session == nil || !rep.Session.Authenticated || rep.Session.Board != "B650D4U" || !strings.HasPrefix(rep.Session.Firmware, "4.10") {
		t.Fatalf("session: %+v", rep.Session)
	}
	// With credentials asrr/model-name identifies the board: certain match.
	if rep.Driver != "asrock" || rep.Score != 100 || rep.Board != "B650D4U" {
		t.Fatalf("credentialed detect: %+v", rep)
	}
	if !backendAuthenticated(rep.Session.Backends, "redfish") || !backendAuthenticated(rep.Session.Backends, "ami") {
		t.Fatalf("B650 backends: %+v", rep.Session.Backends)
	}
	if strings.Contains(out+errs, sim.Creds().Pass) {
		t.Fatal("password printed")
	}

	// Text output.
	code, out, _ = runProbe(t, append([]string{"detect"}, withPassword(t, sim)...)...)
	if code != 0 || !strings.Contains(out, "driver") || !strings.Contains(out, "authenticated") || !strings.Contains(out, "tls") {
		t.Fatalf("text detect: %d %s", code, out)
	}
}

func backendAuthenticated(bs []bmc.BackendStatus, name string) bool {
	for _, b := range bs {
		if b.Name == name {
			return b.Authenticated
		}
	}
	return false
}

// TestDetectGigabyte: detect picks gigabyte on both Gigabyte sims (over
// asrock's MegaRAC guess) without credentials and without an /api login;
// with credentials the match is certain with the board name, and the
// password is never printed.
func TestDetectGigabyte(t *testing.T) {
	for name, start := range map[string]func(testing.TB, ...func(*testsim.Options)) *testsim.Sim{
		"MC13-LE0-000": testsim.StartMC13LE0, "MZ32-AR0-00": testsim.StartMZ32AR0} {
		t.Run(name, func(t *testing.T) {
			sim := start(t)
			code, out, errs := runProbe(t, "--host", sim.Host(), "--json", "detect")
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errs)
			}
			var rep DetectReport
			if err := json.Unmarshal([]byte(out), &rep); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			if rep.Driver != "gigabyte" || rep.Score <= 35 || sim.Count("POST", "/api/session") != 0 {
				t.Fatalf("unauthenticated detect: %+v", rep)
			}
			code, out, errs = runProbe(t, append([]string{"detect", "--json"}, withPassword(t, sim)...)...)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, errs)
			}
			rep = DetectReport{}
			_ = json.Unmarshal([]byte(out), &rep)
			if rep.Driver != "gigabyte" || rep.Score != 100 || rep.Board != name || rep.Firmware != sim.Profile().Firmware {
				t.Fatalf("credentialed detect: %+v", rep)
			}
			if sim.Count("POST", "/api/session") != 0 {
				t.Fatal("asrock logged in to a Gigabyte /api during detection")
			}
			if strings.Contains(out+errs, sim.Creds().Pass) {
				t.Fatal("password printed")
			}
		})
	}
}

// TestDetectX570: unauthenticated detect picks asrock on the X570's AMI
// Redfish 1.8.0 root; with credentials the board profile is the verified
// X570D4U one (not "unknown"), and every request went out one at a time.
func TestDetectX570(t *testing.T) {
	sim := testsim.StartX570(t)
	code, out, errs := runProbe(t, "--host", sim.Host(), "--json", "detect")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var rep DetectReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if rep.Driver != "asrock" || rep.Score <= 10 || rep.Details["redfishVersion"] != "1.8.0" || sim.Logins() != 0 {
		t.Fatalf("detect: %+v", rep)
	}

	code, out, errs = runProbe(t, append([]string{"detect", "--json"}, withPassword(t, sim)...)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	rep = DetectReport{}
	_ = json.Unmarshal([]byte(out), &rep)
	if rep.Driver != "asrock" || rep.Score != 100 || rep.Board != "X570D4U" || rep.Firmware != "1.20.00" {
		t.Fatalf("credentialed detect: %+v", rep)
	}
	if rep.Session == nil || rep.Session.Board != "X570D4U" || rep.Session.Degraded != "" ||
		!backendAuthenticated(rep.Session.Backends, "redfish") || !backendAuthenticated(rep.Session.Backends, "ami") {
		t.Fatalf("session: %+v", rep.Session)
	}
	if st := sim.Stats(); st.MaxInFlight > 1 || st.CSRFRejects != 0 {
		t.Fatalf("max in-flight %d (default --max-inflight 1), CSRF rejects %d", st.MaxInFlight, st.CSRFRejects)
	}
	if strings.Contains(out+errs, sim.Creds().Pass) {
		t.Fatal("password printed")
	}
	testsim.Eventually(t, 2*time.Second, func() bool { return sim.LiveSessions() == 0 }, "X570 sessions logged out")
}

func TestDetectX470CredentialsRejected(t *testing.T) {
	sim := testsim.StartX470(t)
	code, out, errs := runProbe(t, append([]string{"--json", "detect"}, withPassword(t, sim)...)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var rep DetectReport
	_ = json.Unmarshal([]byte(out), &rep)
	if rep.TLS == nil || rep.TLS.Profile.String() != "legacy" {
		t.Fatalf("tls: %+v", rep.TLS)
	}
	// The asrock driver picks the X470 and serves it over /api (+ IPMI);
	// Redfish is not used on this profile.
	if rep.Driver != "asrock" || rep.Board != "X470D4U" || rep.Firmware != "2.32.00" {
		t.Fatalf("detect: %+v", rep)
	}
	if rep.Session == nil || !rep.Session.Authenticated || !backendAuthenticated(rep.Session.Backends, "ami") ||
		backendAuthenticated(rep.Session.Backends, "redfish") {
		t.Fatalf("X470 session: %+v", rep.Session)
	}

	// Forcing the generic Redfish driver shows the X470's Redfish rejecting
	// the admin account.
	code, out, errs = runProbe(t, append([]string{"--json", "--vendor", "redfish", "detect"}, withPassword(t, sim)...)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	rep = DetectReport{}
	_ = json.Unmarshal([]byte(out), &rep)
	if rep.Driver != "redfish" || rep.Session == nil || rep.Session.Authenticated || rep.Session.Error == "" {
		t.Fatalf("X470 Redfish must reject admin: %+v", rep.Session)
	}
	testsim.Eventually(t, 2*time.Second, func() bool { return sim.APISessions() == 0 }, "X470 /api sessions logged out")
}

func TestCapabilities(t *testing.T) {
	sim := testsim.StartB650(t)
	code, out, errs := runProbe(t, append([]string{"capabilities", "--json"}, withPassword(t, sim)...)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var rep CapabilitiesReport
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range rep.Capabilities {
		got[string(c.Capability)] = c.Supported
		if c.Capability == "sensors" && c.Backend != "redfish" {
			t.Fatalf("sensors backend %q", c.Backend)
		}
	}
	if !got["sensors"] || !got["power"] || !got["eventLog"] || got["kvm"] {
		t.Fatalf("capability map: %v", got)
	}
	if sim.LiveSessions() != 0 {
		t.Fatal("session left open")
	}

	code, out, _ = runProbe(t, append([]string{"capabilities"}, withPassword(t, sim)...)...)
	if code != 0 || !strings.Contains(out, "CAPABILITY") || !strings.Contains(out, "501") {
		t.Fatalf("text capabilities: %s", out)
	}
}

func TestGetSensorsEventsPower(t *testing.T) {
	sim := testsim.StartB650(t)
	code, out, errs := runProbe(t, append([]string{"get", "sensors"}, withPassword(t, sim)...)...)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var sens struct {
		Sensors []struct {
			Name    string   `json:"name"`
			Reading *float64 `json:"reading"`
		} `json:"sensors"`
	}
	if err := json.Unmarshal([]byte(out), &sens); err != nil || len(sens.Sensors) <= 50 {
		t.Fatalf("sensors (%v): %d", err, len(sens.Sensors))
	}

	code, out, _ = runProbe(t, append([]string{"get", "power"}, withPassword(t, sim)...)...)
	if code != 0 || !strings.Contains(out, `"state": "on"`) {
		t.Fatalf("power: %s", out)
	}

	code, out, _ = runProbe(t, append([]string{"get", "events"}, withPassword(t, sim)...)...)
	var ev struct {
		Entries []struct{ ID string } `json:"entries"`
		LastID  string                `json:"lastId"`
	}
	_ = json.Unmarshal([]byte(out), &ev)
	if code != 0 || len(ev.Entries) == 0 || ev.LastID == "" {
		t.Fatalf("events: %s", out)
	}
	id := sim.Redfish().AddSELEntry("new", "Warning")
	code, out, _ = runProbe(t, append([]string{"get", "events", "--since", ev.LastID}, withPassword(t, sim)...)...)
	ev.Entries = nil
	_ = json.Unmarshal([]byte(out), &ev)
	if code != 0 || len(ev.Entries) != 1 || ev.Entries[0].ID != id {
		t.Fatalf("events --since: %s", out)
	}

	code, out, _ = runProbe(t, append([]string{"get", "inventory", "cpus"}, withPassword(t, sim)...)...)
	if code != 0 || !strings.Contains(out, "Ryzen") {
		t.Fatalf("inventory: %s", out)
	}
}

func TestPasswordStdinAndHAR(t *testing.T) {
	sim := testsim.StartB650(t)
	old := stdin
	stdin = strings.NewReader(sim.Creds().Pass + "\n")
	defer func() { stdin = old }()
	har := t.TempDir()
	code, _, errs := runProbe(t, "--host", sim.Host(), "--user", sim.Creds().User, "--password-stdin", "--har-out", har, "get", "system")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	files, _ := filepath.Glob(filepath.Join(har, "*.har"))
	if len(files) != 1 {
		t.Fatalf("har files %v", files)
	}
	b, _ := os.ReadFile(files[0])
	var h obs.HAR
	if err := json.Unmarshal(b, &h); err != nil || len(h.Log.Entries) == 0 {
		t.Fatalf("HAR unparseable or empty: %v", err)
	}
	if strings.Contains(string(b), sim.Creds().Pass) {
		t.Fatal("HAR leaks the password with redaction on")
	}
}

func TestUsageErrors(t *testing.T) {
	sim := testsim.StartB650(t)
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"--host", sim.Host(), "--password", "x", "detect"}, "refusing a password"},
		{[]string{"detect"}, "--host is required"},
		{[]string{"--host", sim.Host(), "capabilities"}, "needs credentials"},
		{[]string{"--host", sim.Host(), "frobnicate"}, "unknown command"},
		{[]string{"--host", sim.Host(), "--user", "admin", "--password-env", "BMCPROBE_UNSET_VAR", "get", "power"}, "is not set"},
		{[]string{}, "usage"},
		{[]string{"--host", sim.Host(), "--max-inflight", "0", "detect"}, "--max-inflight must be at least 1"},
	}
	for _, c := range cases {
		code, _, errs := runProbe(t, c.args...)
		if code != 2 || !strings.Contains(errs, c.want) {
			t.Errorf("%v: exit %d, stderr %q (want %q)", c.args, code, errs, c.want)
		}
	}
	code, _, errs := runProbe(t, append([]string{"get", "nope"}, withPassword(t, sim)...)...)
	if code != 2 || !strings.Contains(errs, "unknown op") {
		t.Errorf("unknown op: %d %s", code, errs)
	}
}
