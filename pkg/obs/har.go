package obs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// HAR 1.2 types (http://www.softwareishard.com/blog/har-12-spec/). They are
// exported so tests and the HAR-replay simulator can parse HAR files.
type (
	HAR struct {
		Log HARLog `json:"log"`
	}
	HARLog struct {
		Version string     `json:"version"`
		Creator HARCreator `json:"creator"`
		Pages   []any      `json:"pages,omitempty"`
		Entries []HAREntry `json:"entries"`
	}
	HARCreator struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	HAREntry struct {
		StartedDateTime string      `json:"startedDateTime"`
		Time            float64     `json:"time"` // ms
		Request         HARRequest  `json:"request"`
		Response        HARResponse `json:"response"`
		Cache           struct{}    `json:"cache"`
		Timings         HARTimings  `json:"timings"`
		ServerIPAddress string      `json:"serverIPAddress,omitempty"`
		Connection      string      `json:"connection,omitempty"`
		Comment         string      `json:"comment,omitempty"`
	}
	HARRequest struct {
		Method      string       `json:"method"`
		URL         string       `json:"url"`
		HTTPVersion string       `json:"httpVersion"`
		Cookies     []HARCookie  `json:"cookies"`
		Headers     []HARNV      `json:"headers"`
		QueryString []HARNV      `json:"queryString"`
		PostData    *HARPostData `json:"postData,omitempty"`
		HeadersSize int          `json:"headersSize"`
		BodySize    int          `json:"bodySize"`
	}
	HARResponse struct {
		Status      int         `json:"status"`
		StatusText  string      `json:"statusText"`
		HTTPVersion string      `json:"httpVersion"`
		Cookies     []HARCookie `json:"cookies"`
		Headers     []HARNV     `json:"headers"`
		Content     HARContent  `json:"content"`
		RedirectURL string      `json:"redirectURL"`
		HeadersSize int         `json:"headersSize"`
		BodySize    int         `json:"bodySize"`
		Comment     string      `json:"comment,omitempty"`
	}
	HARNV struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	HARCookie struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	HARPostData struct {
		MimeType string `json:"mimeType"`
		Text     string `json:"text"`
	}
	HARContent struct {
		Size     int    `json:"size"`
		MimeType string `json:"mimeType"`
		Text     string `json:"text,omitempty"`
		Encoding string `json:"encoding,omitempty"`
	}
	// HARTimings are in milliseconds; -1 means not applicable.
	HARTimings struct {
		Blocked float64 `json:"blocked"`
		DNS     float64 `json:"dns"`
		Connect float64 `json:"connect"`
		SSL     float64 `json:"ssl"`
		Send    float64 `json:"send"`
		Wait    float64 `json:"wait"`
		Receive float64 `json:"receive"`
	}
)

// HARRecord is one southbound exchange handed to HARWriter.Record.
type HARRecord struct {
	Host     string         // BMC host[:port]; selects the output file. Default Request.URL.Host.
	Request  *http.Request  // required
	ReqBody  []byte         // request body as sent (nil if none)
	Response *http.Response // nil when Err != nil
	RespBody []byte         // response body (nil if none or streamed)
	Started  time.Time
	Duration time.Duration
	Timings  HARTimings
	ServerIP string
	Err      error  // transport error; recorded as status 0 with a comment
	Comment  string // free text, e.g. "attempt 2/3" or "corr_id=..."
}

// HARWriter writes one HAR 1.2 file per BMC host into a directory. Each Record
// rewrites the host's file atomically, so the file is always valid JSON even
// if the process dies. When redact is true, headers, cookies, URLs and bodies
// are redacted exactly as logs are. Safe for concurrent use.
type HARWriter struct {
	dir        string
	redact     bool
	maxEntries int

	mu    sync.Mutex
	hosts map[string]*harFile
}

type harFile struct {
	seq     int
	entries []HAREntry
}

// DefaultHARMaxEntries is the per-file entry cap before rotation.
const DefaultHARMaxEntries = 5000

// NewHARWriter creates dir (0700) and returns a writer. redact should be
// !LogSecrets.
func NewHARWriter(dir string, redact bool) (*HARWriter, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("obs: har-out: %w", err)
	}
	return &HARWriter{dir: dir, redact: redact, maxEntries: DefaultHARMaxEntries, hosts: map[string]*harFile{}}, nil
}

// SetMaxEntries changes the rotation threshold (n <= 0 means unlimited).
func (w *HARWriter) SetMaxEntries(n int) {
	w.mu.Lock()
	w.maxEntries = n
	w.mu.Unlock()
}

// Dir returns the output directory.
func (w *HARWriter) Dir() string { return w.dir }

// Path returns the current file path for host.
func (w *HARWriter) Path(host string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pathLocked(host, w.fileFor(host))
}

func (w *HARWriter) fileFor(host string) *harFile {
	f := w.hosts[host]
	if f == nil {
		f = &harFile{}
		w.hosts[host] = f
	}
	return f
}

func (w *HARWriter) pathLocked(host string, f *harFile) string {
	name := sanitizeFile(host)
	if f.seq > 0 {
		name = fmt.Sprintf("%s.%d", name, f.seq)
	}
	return filepath.Join(w.dir, name+".har")
}

func sanitizeFile(s string) string {
	if s == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		}
		return '_'
	}, s)
}

// Record appends one exchange and rewrites the host's HAR file.
func (w *HARWriter) Record(rec HARRecord) error {
	if w == nil || rec.Request == nil {
		return nil
	}
	host := rec.Host
	if host == "" && rec.Request.URL != nil {
		host = rec.Request.URL.Host
	}
	e := w.entry(rec)
	w.mu.Lock()
	defer w.mu.Unlock()
	f := w.fileFor(host)
	if w.maxEntries > 0 && len(f.entries) >= w.maxEntries {
		f.seq++
		f.entries = nil
	}
	f.entries = append(f.entries, e)
	return w.flushLocked(host, f)
}

func (w *HARWriter) flushLocked(host string, f *harFile) error {
	doc := HAR{Log: HARLog{Version: "1.2", Creator: HARCreator{Name: "bmcproxy", Version: "1"}, Entries: f.entries}}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	p := w.pathLocked(host, f)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Close is a no-op kept for symmetry (files are flushed on every Record).
func (w *HARWriter) Close() error { return nil }

func (w *HARWriter) str(s string) string {
	if w.redact {
		return RedactString(s)
	}
	return s
}

func (w *HARWriter) headers(h http.Header) []HARNV {
	if w.redact {
		h = RedactHeader(h)
	}
	out := []HARNV{}
	for k, vs := range h {
		for _, v := range vs {
			out = append(out, HARNV{Name: k, Value: v})
		}
	}
	return out
}

func (w *HARWriter) entry(rec HARRecord) HAREntry {
	r := rec.Request
	started := rec.Started
	if started.IsZero() {
		started = time.Now()
	}
	reqHdr := r.Header.Clone()
	if reqHdr == nil {
		reqHdr = http.Header{}
	}
	if r.Host != "" && reqHdr.Get("Host") == "" {
		reqHdr.Set("Host", r.Host)
	}
	u := ""
	if r.URL != nil {
		u = r.URL.String()
	}
	hr := HARRequest{
		Method:      r.Method,
		URL:         w.str(u),
		HTTPVersion: protoOr(r.Proto, "HTTP/1.1"),
		Cookies:     w.cookies(r.Cookies()),
		Headers:     w.headers(reqHdr),
		QueryString: []HARNV{},
		HeadersSize: -1,
		BodySize:    len(rec.ReqBody),
	}
	if r.URL != nil {
		for k, vs := range r.URL.Query() {
			for _, v := range vs {
				if w.redact && IsSensitiveKey(k) {
					v = Redacted
				}
				hr.QueryString = append(hr.QueryString, HARNV{Name: k, Value: v})
			}
		}
	}
	if rec.ReqBody != nil {
		hr.PostData = &HARPostData{MimeType: r.Header.Get("Content-Type"), Text: w.body(rec.ReqBody)}
	}
	e := HAREntry{
		StartedDateTime: started.UTC().Format(time.RFC3339Nano),
		Time:            ms(rec.Duration),
		Request:         hr,
		Timings:         rec.Timings,
		ServerIPAddress: rec.ServerIP,
		Comment:         rec.Comment,
	}
	if e.Timings == (HARTimings{}) {
		e.Timings = HARTimings{Blocked: -1, DNS: -1, Connect: -1, SSL: -1, Send: 0, Wait: ms(rec.Duration), Receive: 0}
	}
	if rec.Response == nil {
		msg := "no response"
		if rec.Err != nil {
			msg = w.str(rec.Err.Error())
		}
		e.Response = HARResponse{Status: 0, StatusText: "", HTTPVersion: "HTTP/1.1", Cookies: []HARCookie{}, Headers: []HARNV{},
			Content: HARContent{MimeType: "x-unknown"}, HeadersSize: -1, BodySize: -1, Comment: "error: " + msg}
		return e
	}
	resp := rec.Response
	text, enc := w.content(rec.RespBody)
	e.Response = HARResponse{
		Status:      resp.StatusCode,
		StatusText:  strings.TrimSpace(strings.TrimPrefix(resp.Status, fmt.Sprint(resp.StatusCode))),
		HTTPVersion: protoOr(resp.Proto, "HTTP/1.1"),
		Cookies:     w.cookies(resp.Cookies()),
		Headers:     w.headers(resp.Header),
		Content:     HARContent{Size: len(rec.RespBody), MimeType: resp.Header.Get("Content-Type"), Text: text, Encoding: enc},
		RedirectURL: resp.Header.Get("Location"),
		HeadersSize: -1,
		BodySize:    len(rec.RespBody),
	}
	return e
}

func (w *HARWriter) cookies(cs []*http.Cookie) []HARCookie {
	out := []HARCookie{}
	for _, c := range cs {
		v := c.Value
		if w.redact {
			v = Redacted
		}
		out = append(out, HARCookie{Name: c.Name, Value: v})
	}
	return out
}

func (w *HARWriter) body(b []byte) string {
	if !utf8.Valid(b) {
		return base64.StdEncoding.EncodeToString(b)
	}
	return w.str(string(b))
}

func (w *HARWriter) content(b []byte) (string, string) {
	if len(b) == 0 {
		return "", ""
	}
	if !utf8.Valid(b) {
		return base64.StdEncoding.EncodeToString(b), "base64"
	}
	return w.str(string(b)), ""
}

func protoOr(p, d string) string {
	if p == "" {
		return d
	}
	return p
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// HARMillis converts a duration to HAR milliseconds (helper for callers
// building HARTimings).
func HARMillis(d time.Duration) float64 { return ms(d) }
