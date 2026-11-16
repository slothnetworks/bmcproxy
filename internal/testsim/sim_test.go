package testsim

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// ---- tiny direct client (no proxy) ----

type amiSess struct {
	cookie, csrf string
}

func client() *http.Client {
	return &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		TLSClientConfig: transport.TLSConfig(transport.ProfileLegacy), MaxIdleConnsPerHost: 32},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func do(t *testing.T, s *Sim, method, path string, body string, sess *amiSess, hdr map[string]string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, s.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if sess != nil {
		req.AddCookie(&http.Cookie{Name: "QSESSIONID", Value: sess.cookie})
		req.Header.Set("X-CSRFTOKEN", sess.csrf)
	}
	for k, v := range hdr {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	resp, err := client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

func login(t *testing.T, s *Sim) *amiSess {
	t.Helper()
	form := url.Values{"username": {s.opts.User}, "password": {s.opts.Pass}}.Encode()
	st, h, b := do(t, s, "POST", "/api/session", form, nil, map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if st != 200 {
		t.Fatalf("login: %d %s", st, b)
	}
	var lr struct {
		CSRFToken string `json:"CSRFToken"`
	}
	_ = json.Unmarshal(b, &lr)
	resp := http.Response{Header: h}
	for _, c := range resp.Cookies() {
		if c.Name == "QSESSIONID" {
			return &amiSess{cookie: c.Value, csrf: lr.CSRFToken}
		}
	}
	t.Fatalf("no QSESSIONID in %v", h)
	return nil
}

func isCC7(b []byte) bool { return strings.Contains(string(b), `"cc":7`) }

func boards(t *testing.T, fn func(t *testing.T, s *Sim)) {
	t.Run("x470", func(t *testing.T) { fn(t, StartX470(t)) })
	t.Run("b650", func(t *testing.T) { fn(t, StartB650(t)) })
}

// ---- fixtures ----

func TestFixturesAreSanitized(t *testing.T) {
	private := regexp.MustCompile(`\b(10\.\d+\.\d+\.\d+|172\.(1[6-9]|2\d|3[01])\.\d+\.\d+|192\.168\.\d+\.\d+)\b`)
	mac := regexp.MustCompile(`\b([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}\b`)
	for _, f := range []string{"x470/endpoints.json", "b650/endpoints.json", "x470/redfish-serviceroot.json"} {
		b, err := fixtureFS.ReadFile("fixtures/" + f)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if m := private.FindString(s); m != "" {
			t.Errorf("%s: private address %s leaked", f, m)
		}
		if m := mac.FindString(s); m != "" {
			t.Errorf("%s: MAC %s leaked", f, m)
		}
		for _, real := range []string{"10.3.10.207", "10.7.12.55"} {
			if strings.Contains(s, real) {
				t.Errorf("%s: real BMC address %s present", f, real)
			}
		}
		for _, key := range []string{"CSRFToken", "password", "racsession_id", "SerialNumber", "mac_address"} {
			re := regexp.MustCompile(`"` + key + `":\s*"([^"]*)"`)
			for _, m := range re.FindAllStringSubmatch(s, -1) {
				v := m[1]
				if v != "" && v != " " && v != "NA" && v != "N/A" && v != "Not Available" && !strings.HasPrefix(v, "<REDACTED") {
					t.Errorf("%s: %s=%q not a placeholder", f, key, v)
				}
			}
		}
	}
	for _, b := range []string{"x470", "b650"} {
		fs, err := LoadFixtures(b)
		if err != nil || len(fs.Endpoints) < 30 {
			t.Fatalf("%s fixtures: %v (%d endpoints)", b, err, len(fs.Endpoints))
		}
	}
}

// ---- /api session model ----

func TestLoginCookieAndCSRF(t *testing.T) {
	boards(t, func(t *testing.T, s *Sim) {
		form := url.Values{"username": {DefaultUser}, "password": {DefaultPass}}.Encode()
		st, h, b := do(t, s, "POST", "/api/session", form, nil, nil)
		if st != 200 || !strings.Contains(string(b), `"CSRFToken"`) {
			t.Fatalf("login %d %s", st, b)
		}
		ck := h.Get("Set-Cookie")
		if !strings.HasPrefix(ck, "QSESSIONID=") || strings.Contains(ck, "secure") != s.profile.SecureCookie {
			t.Fatalf("cookie %q", ck)
		}
		if strings.Contains(string(b), `"user_id"`) != s.profile.LoginExtras {
			t.Fatalf("login extras: %s", b)
		}
		sess := login(t, s)
		if len(sess.cookie) != 30 || len(sess.csrf) != 8 {
			t.Fatalf("session shape %+v", sess)
		}

		// GETs: any non-empty token passes; a missing one does not.
		if st, _, _ := do(t, s, "GET", "/api/chassis-status", "", &amiSess{sess.cookie, "AAAAAAAA"}, nil); st != 200 {
			t.Fatalf("GET bogus csrf = %d, want 200", st)
		}
		if st, _, b := do(t, s, "GET", "/api/chassis-status", "", &amiSess{sess.cookie, ""}, nil); st != 401 || !isCC7(b) {
			t.Fatalf("GET no csrf = %d %s", st, b)
		}
		// Writes: exact token required.
		body := `{"power_command":1}`
		if st, _, b := do(t, s, "POST", "/api/actions/power", body, &amiSess{sess.cookie, "AAAAAAAA"}, nil); st != 401 || !isCC7(b) {
			t.Fatalf("write bogus csrf = %d %s", st, b)
		}
		if st, _, _ := do(t, s, "POST", "/api/actions/power", body, sess, nil); st != 200 {
			t.Fatalf("write good csrf = %d", st)
		}
		// Bad cookie: 401 plus a fresh cookie.
		st, h, b = do(t, s, "GET", "/api/chassis-status", "", &amiSess{"bogus", sess.csrf}, nil)
		if st != 401 || !isCC7(b) || h.Get("Set-Cookie") == "" {
			t.Fatalf("bogus cookie = %d %s %v", st, b, h)
		}
		// Bad password.
		bad := url.Values{"username": {DefaultUser}, "password": {"nope"}}.Encode()
		if st, _, _ := do(t, s, "POST", "/api/session", bad, nil, nil); st != 401 {
			t.Fatalf("bad password = %d", st)
		}
		// Logout invalidates.
		if st, _, _ := do(t, s, "DELETE", "/api/session", "", sess, nil); st != 200 {
			t.Fatalf("logout = %d", st)
		}
		if st, _, _ := do(t, s, "GET", "/api/chassis-status", "", sess, nil); st != 401 {
			t.Fatalf("after logout = %d", st)
		}
		stt := s.Stats()
		if stt.Logins != 2 || stt.Logouts != 1 || stt.LoginFailures != 1 || stt.CSRFRejects != 2 {
			t.Fatalf("stats %+v", stt)
		}
	})
}

func TestMethodOverrideHonoured(t *testing.T) {
	s := StartX470(t)
	sess := login(t, s)
	// A POST with X-HTTP-Method-Override: DELETE logs the session out.
	st, _, _ := do(t, s, "POST", "/api/session", "", sess, map[string]string{"X-HTTP-Method-Override": "DELETE"})
	if st != 200 || s.APISessions() != 0 || s.Stats().MethodOverrides != 1 {
		t.Fatalf("override: %d sessions=%d stats=%+v", st, s.APISessions(), s.Stats())
	}
}

func TestCrashEndpointsInvalidateAllSessions(t *testing.T) {
	for _, tc := range []struct {
		path   string
		status int
		opt    func(*Options)
	}{
		{"/api/logs/help-data", 500, nil},
		{"/api/logs/help-data", 502, func(o *Options) { o.HelpDataStatus = 502 }},
		{"/api/settings/download_image", 502, nil},
	} {
		var mods []func(*Options)
		if tc.opt != nil {
			mods = append(mods, tc.opt)
		}
		s := StartX470(t, mods...)
		a, b := login(t, s), login(t, s)
		st, h, body := do(t, s, "GET", tc.path, "", a, nil)
		if st != tc.status || !strings.Contains(h.Get("Content-Type"), "html") || !strings.HasPrefix(string(body), "<") {
			t.Fatalf("%s = %d %q", tc.path, st, body)
		}
		for _, x := range []*amiSess{a, b} {
			if st, _, _ := do(t, s, "GET", "/api/chassis-status", "", x, nil); st != 401 {
				t.Fatalf("session survived crash: %d", st)
			}
		}
		if s.Stats().Restarts != 1 {
			t.Fatalf("restarts %d", s.Stats().Restarts)
		}
	}
}

func TestRestartAndSlotCap(t *testing.T) {
	s := StartB650(t, func(o *Options) { o.MaxSessions = 2 })
	a := login(t, s)
	login(t, s)
	form := url.Values{"username": {DefaultUser}, "password": {DefaultPass}}.Encode()
	st, _, b := do(t, s, "POST", "/api/session", form, nil, nil)
	if st == 200 || !strings.Contains(string(b), "15000") {
		t.Fatalf("slot cap: %d %s", st, b)
	}
	s.Restart()
	if st, _, _ := do(t, s, "GET", "/api/chassis-status", "", a, nil); st != 401 {
		t.Fatal("session survived Restart")
	}
	login(t, s) // slots freed
}

func TestSessionIdleExpiry(t *testing.T) {
	s := StartX470(t, func(o *Options) { o.SessionIdle = 50 * time.Millisecond })
	a := login(t, s)
	time.Sleep(80 * time.Millisecond)
	if st, _, _ := do(t, s, "GET", "/api/chassis-status", "", a, nil); st != 401 {
		t.Fatalf("idle session still valid: %d", st)
	}
}

func TestUnknownRouteAndOptions(t *testing.T) {
	s := StartX470(t)
	a := login(t, s)
	st, _, b := do(t, s, "GET", "/api/no/such", "", a, nil)
	if st != 404 || !strings.Contains(string(b), "1010") {
		t.Fatalf("unknown = %d %s", st, b)
	}
	if st, _, _ := do(t, s, "OPTIONS", "/api/no/such", "", nil, nil); st != 200 {
		t.Fatalf("OPTIONS = %d", st)
	}
}

// ---- board dialects ----

func TestSensorThresholdDialects(t *testing.T) {
	x := StartX470(t)
	_, _, xb := do(t, x, "GET", "/api/sensors", "", login(t, x), nil)
	if !strings.Contains(string(xb), `"lower_non_critical_threshold":0.0`) || strings.Contains(string(xb), `"NA"`) {
		t.Fatalf("X470 sensors should carry 0.0 thresholds: %.300s", xb)
	}
	if !strings.Contains(string(xb), "settable_readable_threshMask") {
		t.Fatal("X470 field name")
	}
	b := StartB650(t)
	_, _, bb := do(t, b, "GET", "/api/sensors", "", login(t, b), nil)
	if !strings.Contains(string(bb), `"lower_non_critical_threshold":"NA"`) || !strings.Contains(string(bb), "settable_flag") {
		t.Fatalf("B650 sensors should carry \"NA\": %.300s", bb)
	}
}

func TestIdentityOverlay(t *testing.T) {
	boards(t, func(t *testing.T, s *Sim) {
		a := login(t, s)
		_, _, b := do(t, s, "GET", "/api/asrr/fw-info", "", a, nil)
		if !strings.Contains(string(b), `"BMC_fw_version":"`+s.profile.Firmware+`"`) {
			t.Fatalf("fw-info %s", b)
		}
		_, _, b = do(t, s, "GET", "/api/asrr/model-name", "", a, nil)
		if !strings.Contains(string(b), s.profile.BoardName) {
			t.Fatalf("model-name %s", b)
		}
	})
}

func TestEventLogIncrementalAndTrailer(t *testing.T) {
	boards(t, func(t *testing.T, s *Sim) {
		a := login(t, s)
		_, _, b := do(t, s, "GET", "/api/logs/eventlog", "", a, nil)
		var all []map[string]any
		_ = json.Unmarshal(b, &all)
		n := s.EventCount()
		if n == 0 {
			t.Fatal("no events from fixture")
		}
		last := all[len(all)-1]
		_, trailer := last["event_log_count"]
		if trailer != s.profile.EventLogTrailer {
			t.Fatalf("trailer=%v want %v: %v", trailer, s.profile.EventLogTrailer, last)
		}
		top, _ := toInt(all[0]["id"])
		id := s.AddEvent("CPU Temp", "Upper Critical - going high")
		if id != top+1 {
			t.Fatalf("new id %d after %d", id, top)
		}
		_, _, b = do(t, s, "GET", "/api/logs/event?LASTEVENTID="+strconv.Itoa(top), "", a, nil)
		var inc []map[string]any
		_ = json.Unmarshal(b, &inc)
		recs := 0
		for _, r := range inc {
			if _, ok := r["id"]; ok {
				recs++
				if got, _ := toInt(r["id"]); got != id {
					t.Fatalf("incremental returned id %d", got)
				}
				ts, _ := toInt(r["timestamp"])
				if d := time.Since(time.Unix(int64(ts), 0)); d > 5*time.Second || d < -5*time.Second {
					t.Fatalf("new event timestamp %d not BMC-now", ts)
				}
			}
		}
		if recs != 1 {
			t.Fatalf("incremental: %s", b)
		}
	})
}

func TestFanTwoStepInterleaving(t *testing.T) {
	s := StartX470(t)
	a, b := login(t, s), login(t, s)
	sel := func(x *amiSess, n int) {
		if st, _, body := do(t, s, "POST", "/api/asrr/settings/setopenlooptablenumber", `{"table_numble":`+strconv.Itoa(n)+`}`, x, nil); st != 200 {
			t.Fatalf("select %d: %d %s", n, st, body)
		}
	}
	read := func(x *amiSess) int {
		_, _, body := do(t, s, "GET", "/api/asrr/settings/getfanopenloopcontroltable", "", x, nil)
		return FanTableFromBody(body)
	}
	// Proper two-step: select+read per client.
	sel(a, 2)
	if got := read(a); got != 2 {
		t.Fatalf("read table %d want 2", got)
	}
	sel(b, 3)
	if got := read(b); got != 3 {
		t.Fatalf("read table %d want 3", got)
	}
	if s.Stats().FanInterleaves != 0 {
		t.Fatalf("false interleave: %+v", s.Stats())
	}
	// Interleaved: A selects, B selects, A reads B's table.
	sel(a, 4)
	sel(b, 5)
	if got := read(a); got != 5 {
		t.Fatalf("cursor is server-side: got %d", got)
	}
	if s.Stats().FanInterleaves != 2 {
		t.Fatalf("interleaves %d want 2", s.Stats().FanInterleaves)
	}
	if got := read(b); got != 5 {
		t.Fatalf("b read %d", got)
	}
	if s.Stats().FanInterleaves != 2 {
		t.Fatalf("own read counted: %d", s.Stats().FanInterleaves)
	}
}

func TestDateTimeAndClockSkew(t *testing.T) {
	s := StartX470(t, func(o *Options) { o.ClockOffset = -2 * time.Hour })
	a := login(t, s)
	_, h, b := do(t, s, "GET", "/api/settings/date-time", "", a, nil)
	var dt map[string]any
	_ = json.Unmarshal(b, &dt)
	ts, _ := toInt(dt["timestamp"])
	if d := time.Unix(int64(ts), 0).Sub(time.Now()); d > -2*time.Hour+5*time.Second || d < -2*time.Hour-5*time.Second {
		t.Fatalf("timestamp offset %v", d)
	}
	date, err := http.ParseTime(h.Get("Date"))
	if err != nil || time.Until(date) > -2*time.Hour+5*time.Second {
		t.Fatalf("Date header %q not skewed", h.Get("Date"))
	}
	// Partial body -> 500 (JSON, not a crash).
	st, hh, body := do(t, s, "PUT", "/api/settings/date-time", `{"timestamp":`+strconv.Itoa(int(time.Now().Unix()))+`}`, a, nil)
	if st != 500 || !strings.Contains(hh.Get("Content-Type"), "json") || s.Stats().DateTime500s != 1 || s.Stats().Restarts != 0 {
		t.Fatalf("partial PUT = %d %s", st, body)
	}
	// Full object with auto_date off and the right time fixes the clock.
	dt["auto_date"] = 0
	dt["timestamp"] = time.Now().Unix()
	full, _ := json.Marshal(dt)
	if st, _, body := do(t, s, "PUT", "/api/settings/date-time", string(full), a, nil); st != 200 {
		t.Fatalf("full PUT = %d %s", st, body)
	}
	if off := s.ClockOffset(); off > 2*time.Second || off < -2*time.Second {
		t.Fatalf("offset after set %v", off)
	}
}

func TestPowerStateMachine(t *testing.T) {
	s := StartX470(t)
	a := login(t, s)
	power := func() int {
		_, _, b := do(t, s, "GET", "/api/chassis-status", "", a, nil)
		var m map[string]int
		_ = json.Unmarshal(b, &m)
		return m["power_status"]
	}
	act := func(n int) {
		if st, _, b := do(t, s, "POST", "/api/actions/power", `{"power_command":`+strconv.Itoa(n)+`}`, a, nil); st != 200 {
			t.Fatalf("power %d: %d %s", n, st, b)
		}
	}
	if power() != 1 {
		t.Fatal("initially on")
	}
	act(PowerCmdOff)
	if power() != 0 {
		t.Fatal("off failed")
	}
	act(PowerCmdOn)
	s.SetPowerIgnoresSoftOff(true)
	act(PowerCmdSoftOff)
	if power() != 1 || s.Stats().IgnoredSoftOffs != 1 {
		t.Fatal("soft-off should be ignored")
	}
	s.SetPowerIgnoresSoftOff(false)
	s.SetPowerDelay(60 * time.Millisecond)
	act(PowerCmdSoftOff)
	if power() != 1 {
		t.Fatal("transition should take PowerDelay")
	}
	time.Sleep(80 * time.Millisecond)
	if power() != 0 {
		t.Fatal("soft-off did not complete")
	}
}

// ---- resilience knobs ----

func TestEvasive403(t *testing.T) {
	s := StartB650(t)
	s.InjectEvasive403(2)
	for i := 0; i < 2; i++ {
		st, h, b := do(t, s, "GET", "/redfish/v1/", "", nil, nil)
		if st != 403 || !strings.Contains(h.Get("Content-Type"), "html") || !transport.DefaultIsEvasive403(&http.Response{StatusCode: st, Header: h}, b) {
			t.Fatalf("injected 403 #%d: %d", i, st)
		}
	}
	if st, _, _ := do(t, s, "GET", "/redfish/v1/", "", nil, nil); st != 200 {
		t.Fatalf("after injection: %d", st)
	}

	// Concurrent cap on /redfish.
	s.SetEvasiveMaxConcurrent(3)
	s.SetLatency(100 * time.Millisecond)
	var wg sync.WaitGroup
	var mu sync.Mutex
	codes := map[int]int{}
	c := client()
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := c.Get(s.URL + "/redfish/v1/")
			if err != nil {
				return
			}
			resp.Body.Close()
			mu.Lock()
			codes[resp.StatusCode]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	if codes[403] == 0 || codes[200] == 0 || s.Stats().MaxInFlightRF < 4 {
		t.Fatalf("concurrency cap: %v %+v", codes, s.Stats())
	}
}

func TestWedgeAndLatency(t *testing.T) {
	s := StartB650(t)
	s.Wedge()
	c := &http.Client{Timeout: 100 * time.Millisecond, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	if _, err := c.Get(s.URL + "/redfish/v1/"); err == nil {
		t.Fatal("wedged request answered")
	}
	s.Unwedge()
	if st, _, _ := do(t, s, "GET", "/redfish/v1/", "", nil, nil); st != 200 {
		t.Fatalf("after unwedge %d", st)
	}
	s.SetLatency(50 * time.Millisecond)
	start := time.Now()
	do(t, s, "GET", "/redfish/v1/", "", nil, nil)
	if time.Since(start) < 50*time.Millisecond {
		t.Fatal("latency not applied")
	}
}

func TestInFlightCounterSeesConcurrency(t *testing.T) {
	s := StartX470(t)
	a := login(t, s)
	s.SetLatency(30 * time.Millisecond)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest("GET", s.URL+"/api/chassis-status", nil)
			req.AddCookie(&http.Cookie{Name: "QSESSIONID", Value: a.cookie})
			req.Header.Set("X-CSRFTOKEN", a.csrf)
			if resp, err := client().Do(req); err == nil {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
	if s.Stats().MaxInFlightAPI < 2 {
		t.Fatalf("MaxInFlightAPI %d: counter must detect parallel /api clients", s.Stats().MaxInFlightAPI)
	}
}

// ---- Redfish ----

func TestRedfishX470AccessDenied(t *testing.T) {
	s := StartX470(t)
	st, h, _ := do(t, s, "GET", "/redfish/v1", "", nil, nil)
	if st != 302 || h.Get("Location") != "/redfish/v1/" {
		t.Fatalf("no-slash = %d %v", st, h)
	}
	st, _, b := do(t, s, "GET", "/redfish/v1/", "", nil, nil)
	if st != 200 || !strings.Contains(string(b), "ServiceRoot.v1_3_1") || !strings.Contains(string(b), `"1.5.0"`) {
		t.Fatalf("root = %d %s", st, b)
	}
	if st, _, _ := do(t, s, "GET", "/redfish/v1/?foo=1", "", nil, nil); st != 501 {
		t.Fatalf("unknown query = %d", st)
	}
	auth := map[string]string{"Authorization": "Basic YWRtaW46c2ltLVBhc3N3MHJk"} // admin:sim-Passw0rd
	st, _, b = do(t, s, "GET", "/redfish/v1/Systems", "", nil, auth)
	if st != 401 || !strings.Contains(string(b), "Security.1.0.AccessDenied") {
		t.Fatalf("Systems with valid /api creds = %d %s", st, b)
	}
	st, _, _ = do(t, s, "POST", "/redfish/v1/SessionService/Sessions", `{"UserName":"admin","Password":"sim-Passw0rd"}`, nil, nil)
	if st != 401 {
		t.Fatalf("session login = %d", st)
	}
	if st, _, _ := do(t, s, "GET", "/redfish/v1/GlobalLuaTables", "", nil, nil); st != 200 {
		t.Fatalf("GlobalLuaTables = %d", st)
	}
}

func TestRedfishB650PowerAndSessions(t *testing.T) {
	s := StartB650(t)
	if st, _, _ := do(t, s, "GET", "/redfish/v1/GlobalLuaTables", "", nil, nil); st != 200 {
		t.Fatalf("GlobalLuaTables = %d", st)
	}
	auth := map[string]string{"Authorization": "Basic YWRtaW46c2ltLVBhc3N3MHJk", "Content-Type": "application/json"}
	st, _, b := do(t, s, "GET", "/redfish/v1/Systems/Self", "", nil, auth)
	if st != 200 || !strings.Contains(string(b), `"PowerState":"On"`) {
		t.Fatalf("system = %d %.200s", st, b)
	}
	reset := func(rt string) {
		if st, _, b := do(t, s, "POST", "/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset", `{"ResetType":"`+rt+`"}`, nil, auth); st/100 != 2 {
			t.Fatalf("reset %s = %d %s", rt, st, b)
		}
	}
	s.SetPowerIgnoresSoftOff(true)
	reset("GracefulShutdown")
	if !s.PowerOn() {
		t.Fatal("graceful shutdown should be ignored")
	}
	_, _, b = do(t, s, "GET", "/redfish/v1/Systems/Self", "", nil, auth)
	if !strings.Contains(string(b), `"PowerState":"On"`) {
		t.Fatalf("Redfish view diverged from state machine: %.200s", b)
	}
	reset("ForceOff")
	if s.PowerOn() {
		t.Fatal("ForceOff")
	}
	// Session login + Restart drops it.
	st, h, _ := do(t, s, "POST", "/redfish/v1/SessionService/Sessions", `{"UserName":"admin","Password":"sim-Passw0rd"}`, nil, map[string]string{"Content-Type": "application/json"})
	if st != 201 || s.Logins() != 1 || s.LiveSessions() != 1 {
		t.Fatalf("redfish login = %d logins=%d live=%d", st, s.Logins(), s.LiveSessions())
	}
	tok := map[string]string{"X-Auth-Token": h.Get("X-Auth-Token")}
	if st, _, _ := do(t, s, "GET", "/redfish/v1/Systems/Self", "", nil, tok); st != 200 {
		t.Fatalf("token = %d", st)
	}
	s.Restart()
	if st, _, _ := do(t, s, "GET", "/redfish/v1/Systems/Self", "", nil, tok); st != 401 {
		t.Fatalf("token survived restart = %d", st)
	}
	// Clock skew reaches Manager.DateTime.
	s.SetClockOffset(3 * time.Hour)
	_, _, b = do(t, s, "GET", "/redfish/v1/Managers/Self", "", nil, auth)
	var m struct{ DateTime string }
	_ = json.Unmarshal(b, &m)
	bt, err := time.Parse(time.RFC3339, m.DateTime)
	if err != nil || time.Until(bt) < 3*time.Hour-5*time.Second {
		t.Fatalf("DateTime %q not skewed", m.DateTime)
	}
}

// ---- TLS ----

func TestX470LegacyTLS(t *testing.T) {
	s := StartX470(t)
	// A default (modern) Go client cannot negotiate RSA key exchange.
	modern := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: transport.TLSConfig(transport.ProfileModern)}}
	if resp, err := modern.Get(s.URL + "/redfish/v1/"); err == nil {
		resp.Body.Close()
		t.Fatal("modern TLS profile should fail against the X470 sim")
	}
	// The transport's auto profile falls back to legacy and records it.
	tm := transport.NewManager(transport.Config{Retry: transport.RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}})
	defer tm.Close()
	ep := tm.Endpoint(s.Host(), transport.LaneDefault)
	req, _ := ep.NewRequest(context.Background(), "GET", "/redfish/v1/", nil)
	resp, err := ep.Do(req)
	if err != nil {
		t.Fatalf("auto profile: %v", err)
	}
	resp.Body.Close()
	hs, _ := tm.HostStats(s.Host())
	if hs.TLS == nil || hs.TLS.Profile != transport.ProfileLegacy || hs.TLS.Version == "TLS 1.3" {
		t.Fatalf("tls %+v", hs.TLS)
	}
	b := StartB650(t)
	resp, err = modern.Get(b.URL + "/redfish/v1/")
	if err != nil {
		t.Fatalf("B650 modern TLS: %v", err)
	}
	resp.Body.Close()
}

func TestB650NeverServesLiveAddresses(t *testing.T) {
	s := StartB650(t)
	auth := map[string]string{"Authorization": "Basic YWRtaW46c2ltLVBhc3N3MHJk"}
	for _, path := range s.Redfish().Paths() {
		_, _, b := do(t, s, "GET", path, "", nil, auth)
		for _, real := range []string{"10.7.12.", "10.3.10."} {
			if strings.Contains(string(b), real) {
				t.Fatalf("%s serves a live address: %.200s", path, b)
			}
		}
	}
}

func TestCrossBoardFRUFallback(t *testing.T) {
	s := StartB650(t)
	st, _, b := do(t, s, "GET", "/api/fru", "", login(t, s), nil)
	if st != 200 || !strings.Contains(string(b), "B650D4U") || strings.Contains(string(b), "X470D4U") {
		t.Fatalf("B650 /api/fru = %d %.300s", st, b)
	}
}
