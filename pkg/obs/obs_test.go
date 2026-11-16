package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	secretPass   = "S3cr3tPassw0rd!"
	secretBasic  = "YWRtaW46UzNjcjN0UGFzc3cwcmQh" // admin:S3cr3tPassw0rd!
	secretCSRF   = "csrfTOKENvalue123"
	secretSessID = "qsessVALUE987654"
	secretXAuth  = "xauthTOKENabcdef"
)

var secrets = []string{secretPass, secretBasic, secretCSRF, secretSessID, secretXAuth}

func credentialedRequest() (*http.Request, []byte, http.Header, []byte) {
	body := []byte(`{"username":"admin","password":"` + secretPass + `"}`)
	req := httptest.NewRequest("POST", "https://admin:"+secretPass+"@10.0.0.1/api/session?password="+secretPass, bytes.NewReader(body))
	req.Header.Set("Authorization", "Basic "+secretBasic)
	req.Header.Set("Cookie", "QSESSIONID="+secretSessID+"; lang=en")
	req.Header.Set("X-CSRFTOKEN", secretCSRF)
	req.Header.Set("Content-Type", "application/json")
	respHdr := http.Header{}
	respHdr.Set("Set-Cookie", "QSESSIONID="+secretSessID+"; path=/")
	respHdr.Set("X-Auth-Token", secretXAuth)
	respHdr.Set("Content-Type", "application/json")
	respBody := []byte(`{"ok":0,"CSRFToken":"` + secretCSRF + `","privilege":4,"extendedpriv":259}`)
	return req, body, respHdr, respBody
}

// traceLog logs a credentialed exchange the way pkg/transport does at TRACE.
func traceLog(t *testing.T, logSecrets bool) string {
	t.Helper()
	var buf bytes.Buffer
	o, err := New(Options{Writer: &buf, Debug: "transport=trace", LogSecrets: logSecrets})
	if err != nil {
		t.Fatal(err)
	}
	req, body, rh, rb := credentialedRequest()
	ctx := WithRequestID(WithObs(context.Background(), o), "req-1")
	l := FromContext(ctx, SubTransport)
	l.Log(ctx, LevelTrace, "southbound exchange",
		"method", req.Method, "url", req.URL.String(),
		"req_headers", req.Header, "req_body", string(body),
		"resp_headers", rh, "resp_body", string(rb),
		"password", secretPass,
		slog.Group("creds", "user", "admin", "pass", secretPass),
		"dump", "POST /api/session HTTP/1.1\r\nAuthorization: Basic "+secretBasic+"\r\nCookie: QSESSIONID="+secretSessID+"\r\n\r\nusername=admin&password="+secretPass,
	)
	return buf.String()
}

func TestTraceRedactionOn(t *testing.T) {
	out := traceLog(t, false)
	if !strings.Contains(out, "TRACE") || !strings.Contains(out, "southbound exchange") {
		t.Fatalf("trace line not emitted: %s", out)
	}
	for _, s := range secrets {
		if strings.Contains(out, s) {
			t.Errorf("secret %q leaked with redaction on:\n%s", s, out)
		}
	}
	for _, want := range []string{"req_id=req-1", "subsys=transport", "Basic " + Redacted, "username", "admin", "privilege"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestTraceRedactionOffWithLogSecrets(t *testing.T) {
	out := traceLog(t, true)
	for _, s := range secrets {
		if !strings.Contains(out, s) {
			t.Errorf("secret %q missing with LogSecrets:\n%s", s, out)
		}
	}
	if !strings.Contains(out, "log-secrets") {
		t.Errorf("expected startup warning, got:\n%s", out)
	}
}

func TestSubsystemLevels(t *testing.T) {
	var buf bytes.Buffer
	o, err := New(Options{Writer: &buf, Level: slog.LevelWarn, Debug: "redfish,driver=trace"})
	if err != nil {
		t.Fatal(err)
	}
	o.Logger(SubRedfish).Debug("rf-debug")
	o.Logger(SubRedfish).Log(context.Background(), LevelTrace, "rf-trace")
	o.Logger(SubAMI).Info("ami-info")
	o.Logger(SubAMI).Warn("ami-warn")
	o.Logger(SubDriverASRock).Log(context.Background(), LevelTrace, "asrock-trace")
	out := buf.String()
	for _, w := range []string{"rf-debug", "ami-warn", "asrock-trace"} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %s", w)
		}
	}
	for _, nw := range []string{"rf-trace", "ami-info"} {
		if strings.Contains(out, nw) {
			t.Errorf("unexpected %s", nw)
		}
	}
	// runtime change
	o.Levels().Set(SubAMI, slog.LevelInfo)
	o.Logger(SubAMI).Info("ami-info-2")
	if !strings.Contains(buf.String(), "ami-info-2") {
		t.Error("runtime level change not applied")
	}
}

func TestApplyDebugAll(t *testing.T) {
	l := NewLevels(slog.LevelInfo)
	if err := l.ApplyDebug("all"); err != nil {
		t.Fatal(err)
	}
	if l.Level("anything") != slog.LevelDebug {
		t.Fatal("all should set default to debug")
	}
	if err := l.ApplyDebug("transport:trace, ipmi=bogus"); err == nil {
		t.Fatal("expected error for bogus level")
	}
	if l.Level(SubTransport) != LevelTrace {
		t.Fatal("transport:trace not applied")
	}
}

func TestEnvDebug(t *testing.T) {
	t.Setenv(EnvDebug, "sessions")
	o := Options{Debug: "north"}
	o.ApplyEnv()
	if o.Debug != "north,sessions" {
		t.Fatalf("got %q", o.Debug)
	}
}

func TestContextIDs(t *testing.T) {
	var buf bytes.Buffer
	o := NewForHandler(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: LevelTrace}), slog.LevelInfo, false)
	ctx := WithObs(context.Background(), o)
	ctx = WithRequestID(ctx, "R1")
	l := FromContext(ctx, SubNorth)
	cctx, cid := EnsureCorrelationID(ctx)
	cctx = WithAttrs(cctx, "bmc", "10.0.0.9")
	l.InfoContext(cctx, "hello")
	l.Info("plain")
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines: %v", lines)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &m)
	if m[KeyRequestID] != "R1" || m[KeyCorrelationID] != cid || m["bmc"] != "10.0.0.9" || m[KeySubsystem] != SubNorth {
		t.Fatalf("missing ids: %v", m)
	}
	if strings.Count(lines[0], `"req_id"`) != 1 {
		t.Fatalf("duplicate req_id: %s", lines[0])
	}
	m = nil
	_ = json.Unmarshal([]byte(lines[1]), &m)
	if m[KeyRequestID] != "R1" {
		t.Fatalf("baked req id missing on non-context call: %v", m)
	}
}

func TestRedactHelpers(t *testing.T) {
	h := RedactHeader(http.Header{"Set-Cookie": {"QSESSIONID=abc; path=/; HttpOnly"}, "Accept": {"x"}})
	if h.Get("Set-Cookie") != "QSESSIONID="+Redacted+"; path=/; HttpOnly" || h.Get("Accept") != "x" {
		t.Fatalf("got %v", h)
	}
	for _, k := range []string{"password", "Password", "new_password", "X-CSRFTOKEN", "QSESSIONID", "pwd", "PasswordConfirm", "SessionToken"} {
		if !IsSensitiveKey(k) {
			t.Errorf("%s should be sensitive", k)
		}
	}
	for _, k := range []string{"username", "user", "host", "status", "sessions"} {
		if IsSensitiveKey(k) {
			t.Errorf("%s should not be sensitive", k)
		}
	}
	if got := RedactString(`{"Password": "x y", "UserName":"root"}`); strings.Contains(got, "x y") || !strings.Contains(got, "root") {
		t.Errorf("json redact: %s", got)
	}
	if got := RedactString(`err: open https://u:pw@h/x`); strings.Contains(got, ":pw@") {
		t.Errorf("userinfo: %s", got)
	}
}

func harExchange(t *testing.T, redact bool) (HAR, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := NewHARWriter(dir, redact)
	if err != nil {
		t.Fatal(err)
	}
	req, body, rh, rb := credentialedRequest()
	resp := &http.Response{StatusCode: 200, Status: "200 OK", Proto: "HTTP/1.1", Header: rh}
	start := time.Now()
	if err := w.Record(HARRecord{Host: "10.0.0.1:443", Request: req, ReqBody: body, Response: resp, RespBody: rb, Started: start, Duration: 12 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if err := w.Record(HARRecord{Host: "10.0.0.1:443", Request: httptest.NewRequest("GET", "https://10.0.0.1/api/sensors", nil), Err: errors.New("connection reset"), Started: start}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "10.0.0.1_443.har")
	if w.Path("10.0.0.1:443") != p {
		t.Fatalf("path %s", w.Path("10.0.0.1:443"))
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var h HAR
	if err := json.Unmarshal(raw, &h); err != nil {
		t.Fatalf("HAR does not parse: %v", err)
	}
	return h, string(raw)
}

func TestHARShapeAndRedaction(t *testing.T) {
	h, raw := harExchange(t, true)
	if h.Log.Version != "1.2" || h.Log.Creator.Name == "" || len(h.Log.Entries) != 2 {
		t.Fatalf("bad HAR log: %+v", h.Log)
	}
	// generic shape check against the spec's required members
	var generic map[string]any
	_ = json.Unmarshal([]byte(raw), &generic)
	log := generic["log"].(map[string]any)
	e := log["entries"].([]any)[0].(map[string]any)
	for _, k := range []string{"startedDateTime", "time", "request", "response", "cache", "timings"} {
		if _, ok := e[k]; !ok {
			t.Errorf("entry missing %s", k)
		}
	}
	reqm := e["request"].(map[string]any)
	for _, k := range []string{"method", "url", "httpVersion", "cookies", "headers", "queryString", "headersSize", "bodySize"} {
		if _, ok := reqm[k]; !ok {
			t.Errorf("request missing %s", k)
		}
	}
	respm := e["response"].(map[string]any)
	for _, k := range []string{"status", "statusText", "httpVersion", "cookies", "headers", "content", "redirectURL", "headersSize", "bodySize"} {
		if _, ok := respm[k]; !ok {
			t.Errorf("response missing %s", k)
		}
	}
	if h.Log.Entries[0].Request.Method != "POST" || h.Log.Entries[0].Response.Status != 200 {
		t.Errorf("entry 0: %+v", h.Log.Entries[0])
	}
	if h.Log.Entries[1].Response.Status != 0 || !strings.Contains(h.Log.Entries[1].Response.Comment, "reset") {
		t.Errorf("error entry: %+v", h.Log.Entries[1].Response)
	}
	for _, s := range secrets {
		if strings.Contains(raw, s) {
			t.Errorf("secret %q in redacted HAR", s)
		}
	}
}

func TestHARLogSecrets(t *testing.T) {
	_, raw := harExchange(t, false)
	for _, s := range []string{secretPass, secretBasic, secretCSRF, secretSessID, secretXAuth} {
		if !strings.Contains(raw, s) {
			t.Errorf("secret %q missing from unredacted HAR", s)
		}
	}
}

func TestLevelsHandler(t *testing.T) {
	l := NewLevels(slog.LevelInfo)
	mux := http.NewServeMux()
	RegisterDebugHandlers(mux, NewForHandler(slog.DiscardHandler, slog.LevelInfo, false))
	mux.Handle("/lv", LevelsHandler(l))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest("POST", "/lv?set=transport=trace,ipmi", nil))
	var m map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &m)
	if m["transport"] != "TRACE" || m["ipmi"] != "DEBUG" || m["*"] != "INFO" {
		t.Fatalf("got %v", m)
	}
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest("GET", "/debug/vars", nil))
	if rr.Code != 200 {
		t.Fatalf("expvar %d", rr.Code)
	}
}
