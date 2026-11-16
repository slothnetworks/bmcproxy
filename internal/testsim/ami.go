package testsim

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// AMI MegaRAC /api emulation: form login (QSESSIONID + CSRFToken), the auth
// gate (cc:7), CSRF enforced on writes but not on GETs, honoured
// X-HTTP-Method-Override, crash endpoints, slot cap, and fixture replay with
// dynamic overlays for power, LED, date-time, SEL and the fan cursor.

type amiSession struct {
	id, csrf, user string
	racID          int
	created, used  time.Time
}

type amiState struct {
	sessions map[string]*amiSession
	nextRac  int
	pid      int
}

// Crash endpoints (docs/x470/endpoint-behaviour.json).
var (
	html500Crashers = map[string]bool{"/api/logs/help-data": true, "/api/hpm/upgradestatus": true, "/api/hpm/verifyimagestatus": true,
		"/api/maintenance/hpm/upgradestatus": true, "/api/maintenance/hpm/verifyimagestatus": true}
	badGateway502 = map[string]bool{"/api/asrr/mca-file": true, "/api/maintenance/download_config": true, "/api/settings/download_image": true}
)

const html500 = "<?xml version=\"1.0\" encoding=\"iso-8859-1\"?>\n<html><head><title>500 - Internal Server Error</title></head><body><h1>500 - Internal Server Error</h1></body></html>\n"
const html502 = "<?xml version=\"1.0\" encoding=\"iso-8859-1\"?>\n<html><head><title>502 - Bad Gateway</title></head><body><h1>502 - Bad Gateway</h1></body></html>\n"

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.WriteHeader(status)
	b, _ := json.Marshal(v)
	_, _ = w.Write(b)
}

// writeRaw replays a fixture body, compacted like the BMC sends it.
func writeRaw(w http.ResponseWriter, status int, body []byte) {
	var buf bytes.Buffer
	if json.Compact(&buf, body) == nil {
		body = buf.Bytes()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

var ccInvalidAuth = map[string]any{"cc": 7, "error": "Invalid Authentication"}

const alnum = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		x, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alnum))))
		b[i] = alnum[x.Int64()]
	}
	return string(b)
}

func (s *Sim) setCookie(w http.ResponseWriter, v string) {
	c := "QSESSIONID=" + v + "; path=/"
	if s.profile.SecureCookie {
		c += "; secure; HttpOnly; Samesite=Lax"
	}
	w.Header().Add("Set-Cookie", c)
}

func isWrite(m string) bool {
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// serveAPI handles /api/*. It returns the session id used (for the log).
func (s *Sim) serveAPI(w http.ResponseWriter, r *http.Request, body []byte) string {
	method := r.Method
	if ov := r.Header.Get("X-HTTP-Method-Override"); ov != "" {
		method = strings.ToUpper(ov) // the backend honours it (a proxy must strip it)
	}
	p := r.URL.Path
	if method == http.MethodOptions || method == http.MethodHead {
		w.WriteHeader(http.StatusOK) // always 200, empty, even for unknown paths
		return ""
	}
	if method == http.MethodPost && p == "/api/session" {
		s.login(w, r, body)
		return ""
	}

	s.mu.Lock()
	sess, reason := s.authLocked(r, method)
	if sess == nil {
		if reason == "csrf" {
			s.stats.CSRFRejects++
		} else {
			s.stats.AuthRejects++
			s.setCookie(w, randString(30)) // a fresh unauthenticated cookie
		}
		s.mu.Unlock()
		writeJSON(w, http.StatusUnauthorized, ccInvalidAuth)
		return ""
	}
	sid := sess.id

	// Crash endpoints: the FastCGI backend dies and takes every session.
	if html500Crashers[p] {
		st := s.opts.HelpDataStatus
		s.restartLocked()
		s.mu.Unlock()
		if st == http.StatusBadGateway {
			writeHTML(w, st, html502)
		} else {
			writeHTML(w, st, html500)
		}
		return sid
	}
	if badGateway502[p] {
		s.restartLocked()
		s.mu.Unlock()
		writeHTML(w, http.StatusBadGateway, html502)
		return sid
	}

	if method == http.MethodDelete && p == "/api/session" {
		delete(s.ami.sessions, sess.id)
		s.stats.Logouts++
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": 0})
		return sid
	}
	s.mu.Unlock()

	if s.dynamicAPI(w, r, method, p, body, sess) || s.oemAPI(w, method, p, body) {
		return sid
	}
	if e, ok := s.fixtures.Find(method, p, s.profile.Captures...); ok {
		writeRaw(w, e.Status, e.Response)
		return sid
	}
	if b, ok := s.crossBoard(method, p); ok {
		writeJSON(w, http.StatusOK, b)
		return sid
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"error": "Invalid API Call", "code": 1010})
	return sid
}

// authLocked validates the session cookie and CSRF token. GETs need a valid
// cookie and any non-empty token; writes need the session's exact token.
func (s *Sim) authLocked(r *http.Request, method string) (*amiSession, string) {
	c, err := r.Cookie("QSESSIONID")
	if err != nil || c.Value == "" {
		return nil, "session"
	}
	sess, ok := s.ami.sessions[c.Value]
	if !ok {
		return nil, "session"
	}
	now := s.now()
	if s.opts.SessionIdle > 0 && now.Sub(sess.used) > s.opts.SessionIdle {
		delete(s.ami.sessions, sess.id)
		return nil, "session"
	}
	tok := r.Header.Get("X-CSRFTOKEN")
	if tok == "" || (isWrite(method) && tok != sess.csrf) {
		return nil, "csrf"
	}
	sess.used = now
	return sess, ""
}

func (s *Sim) login(w http.ResponseWriter, r *http.Request, body []byte) {
	form, _ := url.ParseQuery(string(body))
	user, pass := form.Get("username"), form.Get("password")
	s.mu.Lock()
	if user != s.opts.User || pass != s.opts.Pass {
		s.stats.LoginFailures++
		s.mu.Unlock()
		s.setCookie(w, randString(30))
		writeJSON(w, http.StatusUnauthorized, ccInvalidAuth)
		return
	}
	if s.opts.MaxSessions > 0 && len(s.ami.sessions) >= s.opts.MaxSessions {
		s.stats.LoginFailures++
		s.mu.Unlock()
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "MAX_NUM_SESSIONS_ALREADY_IN_USE", "code": 15000})
		return
	}
	s.ami.nextRac++
	now := s.now()
	sess := &amiSession{id: randString(30), csrf: randString(8), user: user, racID: s.ami.nextRac, created: now, used: now}
	s.ami.sessions[sess.id] = sess
	s.stats.Logins++
	s.mu.Unlock()

	resp := map[string]any{}
	if e, ok := s.fixtures.Find(http.MethodPost, "/api/session", s.profile.Captures...); ok {
		if v, err := decodeAny(e.Response); err == nil {
			if m, ok := v.(map[string]any); ok {
				resp = m
			}
		}
	}
	remote, _, _ := net.SplitHostPort(r.RemoteAddr)
	local := strings.Split(s.Host(), ":")[0]
	resp["ok"] = 0
	resp["privilege"] = 4
	resp["extendedpriv"] = 259
	resp["racsession_id"] = sess.racID
	resp["remote_addr"] = remote
	resp["server_name"] = local
	resp["server_addr"] = local
	resp["HTTPSEnabled"] = 1
	resp["CSRFToken"] = sess.csrf
	if s.profile.LoginExtras {
		resp["user_id"] = 2
		resp["channel"] = 1
		resp["passwordStatus"] = 0
	} else {
		delete(resp, "user_id")
		delete(resp, "channel")
		delete(resp, "passwordStatus")
	}
	s.setCookie(w, sess.id)
	writeJSON(w, http.StatusOK, resp)
}

// crossBoardFallback lists GETs missing from a board's HAR whose handler
// the firmware catalog (docs/api/legacy-api-endpoints.json handler_present)
// shows on every board, with an identical AMI shape: they are served from
// the other board's capture with the board name substituted.
var crossBoardFallback = map[string]bool{
	"GET /api/fru": true,
	// Not in the X470 HAR; live-read on the X470 2.32 with the same shape
	// (docs/x470/endpoint-reference.json: services array with id,
	// service_name, ports, time_out; audit array of {id, timestamp,
	// hostname, message}).
	"GET /api/settings/services": true,
	"GET /api/logs/audit":        true,
}

func (s *Sim) crossBoard(method, p string) (any, bool) {
	if !crossBoardFallback[method+" "+p] {
		return nil, false
	}
	other := BoardX470
	if s.profile.Board == BoardX470 {
		other = BoardB650
	}
	fs, err := LoadFixtures(string(other))
	if err != nil {
		return nil, false
	}
	e, ok := fs.Find(method, p)
	if !ok {
		return nil, false
	}
	b := strings.ReplaceAll(string(e.Response), X470().BoardName, s.profile.BoardName)
	v, err := decodeAny([]byte(b))
	return v, err == nil
}

// fixtureObject returns a fresh decoded copy of a GET fixture body.
func (s *Sim) fixtureObject(method, p string) any {
	e, ok := s.fixtures.Find(method, p, s.profile.Captures...)
	if !ok {
		return nil
	}
	v, _ := decodeAny(e.Response)
	return v
}

// dynamicAPI serves endpoints whose state the sim models. It returns false
// to fall through to fixture replay.
func (s *Sim) dynamicAPI(w http.ResponseWriter, r *http.Request, method, p string, body []byte, sess *amiSession) bool {
	switch method + " " + p {
	case "GET /api/chassis-status", "GET /api/chassis":
		s.mu.Lock()
		on := s.power.isOn(s.now())
		led := s.power.led
		s.mu.Unlock()
		m, _ := s.fixtureObject(http.MethodGet, p).(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		m["power_status"] = b2i(on)
		m["led_status"] = led
		if p == "/api/chassis-status" && s.profile.ChassisCycleInterval {
			m["power_cycle_interval"] = 2000
		}
		writeJSON(w, http.StatusOK, m)
	case "POST /api/actions/power":
		var req struct {
			Cmd *int `json:"power_command"`
		}
		if json.Unmarshal(body, &req) != nil || req.Cmd == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid data", "code": 1001})
			return true
		}
		if !s.powerAction(*req.Cmd) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid power command", "code": 1001})
			return true
		}
		writeJSON(w, http.StatusOK, map[string]any{"power_command": *req.Cmd})
	case "POST /api/actions/chassis-led", "PUT /api/actions/chassis-led":
		var req map[string]any
		_ = json.Unmarshal(body, &req)
		v, ok := req["uid_command"]
		if !ok {
			v = req["led_status"]
		}
		n, _ := toInt(v)
		s.mu.Lock()
		s.power.led = n
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, req)
	case "GET /api/asrr/fw-info":
		m, _ := s.fixtureObject(http.MethodGet, p).(map[string]any)
		if m == nil {
			return false
		}
		m["BMC_fw_version"] = s.profile.Firmware
		writeJSON(w, http.StatusOK, m)
	case "GET /api/asrr/model-name":
		m, _ := s.fixtureObject(http.MethodGet, p).(map[string]any)
		if m == nil {
			m = map[string]any{}
		}
		m["mb_name"] = s.profile.BoardName
		writeJSON(w, http.StatusOK, m)
	case "GET /api/settings/date-time":
		writeJSON(w, http.StatusOK, s.dateTimeNow())
	case "PUT /api/settings/date-time":
		s.putDateTime(w, body)
	case "GET /api/logs/eventlog":
		writeJSON(w, http.StatusOK, s.eventsAfter(-1))
	case "GET /api/logs/event":
		last := -1
		if v := r.URL.Query().Get("LASTEVENTID"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid data", "code": 1001})
				return true
			}
			last = n
		}
		writeJSON(w, http.StatusOK, s.eventsAfter(last))
	case "DELETE /api/logs/event", "DELETE /api/logs/eventlog":
		s.mu.Lock()
		s.events.recs = nil
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": 0})
	case "POST /api/asrr/settings/setopenlooptablenumber", "PUT /api/asrr/settings/setopenlooptablenumber":
		var req struct {
			N *int `json:"table_numble"` // sic: the firmware's spelling
		}
		if json.Unmarshal(body, &req) != nil || req.N == nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Invalid data", "code": 1001})
			return true
		}
		s.fanSelect(sess.id, *req.N)
		writeRaw(w, http.StatusOK, []byte("{ }"))
	case "GET /api/asrr/settings/getfanopenloopcontroltable":
		writeJSON(w, http.StatusOK, s.fanRead(sess.id))
	default:
		return false
	}
	return true
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		return int(n), err == nil
	case string:
		n, err := strconv.Atoi(x)
		return n, err == nil
	case int:
		return x, true
	}
	return 0, false
}

// ---- date-time ----

func (s *Sim) initDateTime() error {
	m, _ := s.fixtureObject(http.MethodGet, "/api/settings/date-time").(map[string]any)
	if m == nil {
		m = map[string]any{"id": json.Number("1"), "primary_ntp": "pool.ntp.org", "secondary_ntp": "time.nist.gov",
			"auto_date": json.Number("0"), "utc_minutes": json.Number("0"), "timezone": "GMT+0"}
	}
	s.dateTime = m
	return nil
}

func (s *Sim) dateTimeNow() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := cloneJSON(s.dateTime).(map[string]any)
	ts := s.now().Add(s.clockOffset()).Unix()
	um, _ := toInt(out["utc_minutes"])
	out["timestamp"] = ts
	out["localized_timestamp"] = ts + int64(um)*60
	return out
}

// putDateTime requires the full writable object (every field the UI sends:
// all GET keys except the response-only id and localized_timestamp, plus
// timestamp) and answers a partial body with a JSON 500. The live X470 2.32
// 500'd on a PUT of the verbatim GET object (docs/x470/scripts/rt.py
// "date-time:write_same"); which keys it objects to is unknown, so the sim
// tolerates id/localized_timestamp either way. A manual "timestamp" re-sets
// the BMC clock when automatic (NTP) dating is off.
func (s *Sim) putDateTime(w http.ResponseWriter, body []byte) {
	v, err := decodeAny(body)
	m, _ := v.(map[string]any)
	s.mu.Lock()
	defer s.mu.Unlock()
	missing := err != nil || m == nil
	if !missing {
		for k := range s.dateTime {
			if k == "localized_timestamp" || k == "id" {
				continue
			}
			if _, ok := m[k]; !ok {
				missing = true
				break
			}
		}
		if _, ok := m["timestamp"]; !ok {
			missing = true
		}
	}
	if missing {
		s.stats.DateTime500s++
		// JSON body: a handler error, not a backend crash.
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Error in setting date time", "code": 1001})
		return
	}
	auto := 0
	for _, k := range []string{"auto_date", "ntp_auto_date"} {
		if n, ok := toInt(m[k]); ok && n == 1 {
			auto = 1
		}
	}
	if ts, ok := toInt(m["timestamp"]); ok && auto == 0 {
		s.offset.Store(int64(time.Unix(int64(ts), 0).Sub(s.now()).Truncate(time.Second)))
		if s.rf != nil {
			s.rf.Lock()
			s.rf.ClockSkew = s.clockOffset()
			s.rf.Unlock()
		}
	}
	for k, v := range m {
		if k == "timestamp" || k == "localized_timestamp" {
			continue
		}
		s.dateTime[k] = v
	}
	out := cloneJSON(s.dateTime).(map[string]any)
	ts := s.now().Add(s.clockOffset()).Unix()
	out["timestamp"] = ts
	writeJSON(w, http.StatusOK, out)
}

// ---- SEL ----

type eventStore struct {
	recs []map[string]any // newest first
}

func (s *Sim) initEvents() error {
	v := s.fixtureObject(http.MethodGet, "/api/logs/eventlog")
	arr, _ := v.([]any)
	seen := map[int]bool{}
	for _, e := range arr {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		id, ok := toInt(m["id"])
		if !ok || seen[id] {
			continue // trailer {"event_log_count":N} or duplicate
		}
		seen[id] = true
		s.events.recs = append(s.events.recs, m)
	}
	sortEvents(s.events.recs)
	return nil
}

func sortEvents(recs []map[string]any) {
	sort.SliceStable(recs, func(i, j int) bool {
		a, _ := toInt(recs[i]["id"])
		b, _ := toInt(recs[j]["id"])
		return a > b
	})
}

// AddEvent appends a SEL record (BMC-clock timestamp) and returns its id.
func (s *Sim) AddEvent(sensorName, description string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := 1
	if len(s.events.recs) > 0 {
		top, _ := toInt(s.events.recs[0]["id"])
		id = top + 1
	}
	rec := map[string]any{"id": id, "record_type": "system_event_record", "timestamp": s.now().Add(s.clockOffset()).Unix(),
		"sensor_type": "temperature", "sensor_name": sensorName, "sensor_number": 1, "generator_type": "ipmb_slave_address",
		"event_direction": "asserted", "event_reading_class": "threshold", "event_description": description,
		"EventData1_Description": description, "EventData2_Description": "N/A", "EventData3_Description": "N/A"}
	s.events.recs = append([]map[string]any{rec}, s.events.recs...)
	return id
}

// EventCount returns the number of SEL records.
func (s *Sim) EventCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events.recs)
}

// eventsAfter returns records with id > last, newest first, with the B650
// trailer element.
func (s *Sim) eventsAfter(last int) []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []any{}
	for _, r := range s.events.recs {
		if id, _ := toInt(r["id"]); id > last {
			out = append(out, r)
		}
	}
	if s.profile.EventLogTrailer {
		out = append(out, map[string]any{"event_log_count": len(s.events.recs)})
	}
	return out
}

// ---- fan two-step (server-side cursor) ----

type fanState struct {
	table   int
	owner   string
	pending bool // selected but not yet read
}

func (s *Sim) fanSelect(owner string, n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fan.pending {
		// A select whose table was not read yet: another request (or another
		// client) slipped in between a select and its read. One proxy
		// session serves many northbound requests, so the owner alone
		// cannot tell them apart.
		s.stats.FanInterleaves++
	}
	s.fan = fanState{table: n, owner: owner, pending: true}
}

func (s *Sim) fanRead(owner string) []any {
	s.mu.Lock()
	if s.fan.owner != "" && s.fan.owner != owner {
		s.stats.FanInterleaves++ // reading a table another client selected
	}
	table := s.fan.table
	if table == 0 {
		table = 1
	}
	s.fan.pending = false
	s.mu.Unlock()

	base, _ := s.fixtureObject(http.MethodGet, "/api/asrr/settings/getfanopenloopcontroltable").([]any)
	var m map[string]any
	if len(base) > 0 {
		m, _ = base[0].(map[string]any)
	}
	if m == nil {
		m = map[string]any{}
	}
	if table != 1 {
		// Table 1 is the captured (empty, all-255) customized table; other
		// tables get distinct synthetic points so tests can tell them apart.
		for k := 1; k <= 24; k++ {
			m["Get_Temp"+strconv.Itoa(k)] = 20 + 3*k
			m["Get_Duty"+strconv.Itoa(k)] = min(100, (table&0x7f)*10+k)
		}
	}
	return []any{m}
}

// FanTableFromBody recovers which open-loop table (1..8, factory bit
// dropped) a getfanopenloopcontroltable body belongs to (test helper):
// table 1 is all 255; table N has Get_Duty1 = N*10+1.
func FanTableFromBody(b []byte) int {
	var arr []map[string]any
	if json.Unmarshal(b, &arr) != nil || len(arr) == 0 {
		return 0
	}
	d, ok := toInt(arr[0]["Get_Duty1"])
	if !ok {
		return 0
	}
	if d == 255 {
		return 1
	}
	return (d - 1) / 10
}
