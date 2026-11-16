// Package redfish is a vendor-neutral Redfish client built on
// pkg/transport. It handles HTTP Basic and SessionService authentication,
// lenient JSON decoding, ETag capture and If-Match on PATCH, bounded
// @odata.nextLink paging, query options gated by the service's advertised
// ProtocolFeaturesSupported, task monitors and per-board quirk hooks.
//
// Every southbound call goes through a *transport.Endpoint (normally
// transport.LaneRedfish), so the breaker, the per-host limiter, retry and
// TRACE/HAR capture apply.
package redfish

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// Pkg is the OpError.Pkg value used by this package.
const Pkg = "redfish"

// DefaultServiceRoot is the ServiceRoot path. The trailing slash avoids the
// X470's 302 from /redfish/v1 -> /redfish/v1/ (B650 accepts both).
const DefaultServiceRoot = "/redfish/v1/"

// AuthMode selects how the client authenticates.
type AuthMode int

const (
	// AuthBasic sends HTTP Basic credentials on every request. It is the
	// default: B650 SessionTimeout is 30 s, so sessions go stale quickly,
	// and Basic never consumes a server-side session slot.
	AuthBasic AuthMode = iota
	// AuthSession creates a SessionService session (X-Auth-Token), re-logs
	// in once on 401 and DELETEs the session on Close.
	AuthSession
	// AuthNone sends no credentials (ServiceRoot probing).
	AuthNone
)

func (m AuthMode) String() string {
	switch m {
	case AuthBasic:
		return "basic"
	case AuthSession:
		return "session"
	case AuthNone:
		return "none"
	}
	return fmt.Sprintf("AuthMode(%d)", int(m))
}

// Options configure a Client. The zero value (plus Creds) is a Basic-auth
// client against the endpoint's https base URL.
type Options struct {
	Creds bmc.Creds
	Auth  AuthMode
	// BaseURL overrides ep.BaseURL() ("https://host:port"), e.g. for tests.
	BaseURL string
	Quirks  Quirks
	// MaxPages bounds every nextLink walk (default 64).
	MaxPages int
	// TaskPollInterval / TaskTimeout are the task monitor defaults
	// (default 2 s / 5 min).
	TaskPollInterval time.Duration
	TaskTimeout      time.Duration
	// WaitActionTasks makes the typed action helpers wait for a 202 task
	// to finish (default false: the 202 Response is returned and the caller
	// may call WaitTask).
	WaitActionTasks bool
	// SessionRefreshMargin: in AuthSession mode, a token idle for longer
	// than SessionTimeout minus this margin is proactively replaced before
	// the next request instead of waiting for a 401 (default 5 s).
	SessionRefreshMargin time.Duration
	// Now is the clock (tests). Default time.Now.
	Now func() time.Time
}

// Client is a Redfish client for one BMC and one set of credentials. It is
// safe for concurrent use.
type Client struct {
	ep   *transport.Endpoint
	base *url.URL
	opts Options

	mu       sync.Mutex
	root     *ServiceRoot
	rootRaw  json.RawMessage
	caps     QueryCaps
	capsSet  bool
	etags    map[string]string // path -> last seen ETag
	paths    map[string]string // discovered logical paths (system, manager, chassis)
	session  *sessionState
	loginMu  sync.Mutex // serialises login/relogin
	closed   bool
	actInfos map[string]*ActionInfo
}

// New returns a Client for ep. It does no I/O.
func New(ep *transport.Endpoint, opts Options) (*Client, error) {
	if ep == nil {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "New", "nil endpoint")
	}
	base := opts.BaseURL
	if base == "" {
		base = ep.BaseURL()
	}
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "New", "bad base URL %q", base)
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = 64
	}
	if opts.TaskPollInterval <= 0 {
		opts.TaskPollInterval = 2 * time.Second
	}
	if opts.TaskTimeout <= 0 {
		opts.TaskTimeout = 5 * time.Minute
	}
	if opts.SessionRefreshMargin <= 0 {
		opts.SessionRefreshMargin = 5 * time.Second
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Client{ep: ep, base: u, opts: opts, etags: map[string]string{}, paths: map[string]string{}, actInfos: map[string]*ActionInfo{}}, nil
}

// Endpoint returns the transport endpoint.
func (c *Client) Endpoint() *transport.Endpoint { return c.ep }

// BaseURL returns the scheme://host[:port] the client talks to.
func (c *Client) BaseURL() string { return c.base.String() }

// User returns the configured user name.
func (c *Client) User() string { return c.opts.Creds.User }

// AuthMode returns the configured auth mode.
func (c *Client) AuthMode() AuthMode { return c.opts.Auth }

// Quirks returns the active quirks.
func (c *Client) Quirks() Quirks { return c.opts.Quirks }

// Response is a buffered Redfish response.
type Response struct {
	Method   string
	Path     string // request path+query as sent
	Status   int
	Header   http.Header
	Body     []byte
	ETag     string // ETag header, else body @odata.etag
	Location string // Location header (resolved to a path when same-host)
}

// Decode leniently decodes the body into out (see Client.Decode for the
// quirks-aware variant).
func (r *Response) Decode(out any) error { return decodeLenient(r.Body, out, Quirks{}) }

// IsSuccess reports a 2xx status.
func (r *Response) IsSuccess() bool { return r != nil && r.Status >= 200 && r.Status < 300 }

// ---- request plumbing ----

// RequestOption customises one request.
type RequestOption func(*reqConfig)

type reqConfig struct {
	header  http.Header
	ifMatch string
	noAuth  bool
	op      string
	query   *Query
	noQuirk bool
}

// WithHeader adds a request header.
func WithHeader(k, v string) RequestOption {
	return func(rc *reqConfig) {
		if rc.header == nil {
			rc.header = http.Header{}
		}
		rc.header.Add(k, v)
	}
}

// WithIfMatch sets If-Match explicitly ("" leaves the automatic behaviour;
// use IfMatchNone to suppress it).
func WithIfMatch(etag string) RequestOption { return func(rc *reqConfig) { rc.ifMatch = etag } }

// IfMatchNone suppresses the automatic If-Match on a PATCH.
const IfMatchNone = "-"

// IfMatchAny sends If-Match: * (unconditional but satisfies services that
// require a precondition header).
const IfMatchAny = "*"

// WithoutAuth sends the request with no credentials.
func WithoutAuth() RequestOption { return func(rc *reqConfig) { rc.noAuth = true } }

// WithOp names the operation for transport logs/HAR/hooks.
func WithOp(op string) RequestOption { return func(rc *reqConfig) { rc.op = op } }

// WithQuery attaches client query options (filtered by capability).
func WithQuery(q Query) RequestOption { return func(rc *reqConfig) { rc.query = &q } }

// resolve turns a path or same-host URL into an absolute URL, applying the
// path overrides quirk.
func (c *Client) resolve(p string) (string, error) {
	if p == "" {
		return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "resolve", "empty path")
	}
	if !c.opts.Quirks.NoPathOverride {
		p = c.opts.Quirks.overridePath(p)
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		u, err := url.Parse(p)
		if err != nil {
			return "", bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "resolve", "bad URL %q: %v", p, err)
		}
		if !strings.EqualFold(u.Host, c.base.Host) {
			return "", bmc.Errorf(bmc.ErrForbidden, Pkg, "resolve", "refusing cross-host URL %q", u.Host)
		}
		u.Scheme = c.base.Scheme
		return u.String(), nil
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return c.base.String() + p, nil
}

// pathOf strips scheme/host from a same-host URL, returning path?query.
func (c *Client) pathOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.Host != "" && !strings.EqualFold(u.Host, c.base.Host) {
		return raw
	}
	s := u.EscapedPath()
	if u.RawQuery != "" {
		s += "?" + u.RawQuery
	}
	return s
}

func cleanPath(p string) string {
	if i := strings.IndexAny(p, "?#"); i >= 0 {
		p = p[:i]
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	return p
}

func bodyBytes(body any) ([]byte, error) {
	switch b := body.(type) {
	case nil:
		return nil, nil
	case []byte:
		return b, nil
	case string:
		return []byte(b), nil
	case json.RawMessage:
		return b, nil
	case io.Reader:
		return io.ReadAll(b)
	default:
		return json.Marshal(b)
	}
}

// Do performs one request and returns the buffered response whatever its
// status (the raw passthrough primitive). Authentication is applied per the
// configured mode; in session mode a 401 triggers one relogin and retry.
// Same-host GET/HEAD redirects are followed (max 3). Only transport errors
// (busy, timeout, connection, cancel) and a failed relogin are returned as
// errors.
func (c *Client) Do(ctx context.Context, method, path string, body any, opts ...RequestOption) (*Response, error) {
	var rc reqConfig
	for _, o := range opts {
		o(&rc)
	}
	return c.do(ctx, method, path, body, &rc)
}

func (c *Client) do(ctx context.Context, method, path string, body any, rc *reqConfig) (*Response, error) {
	method = strings.ToUpper(method)
	b, err := bodyBytes(body)
	if err != nil {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, method+" "+path, "encode body: %v", err)
	}
	full, err := c.resolve(path)
	if err != nil {
		return nil, err
	}
	sentQuery := false
	if rc.query != nil && (method == http.MethodGet || method == http.MethodHead) {
		qs, dropped := rc.query.encode(c.QueryCaps(ctx))
		if len(dropped) > 0 {
			obs.FromContext(ctx, obs.SubRedfish).DebugContext(ctx, "redfish query options dropped (not advertised)",
				"path", path, "dropped", dropped)
		}
		if qs != "" {
			full = appendQuery(full, qs)
			sentQuery = true
		}
	}
	resp, err := c.send(ctx, method, full, b, rc)
	if err != nil {
		return nil, err
	}
	// A service that 501s on query options it advertised (X470 style): retry
	// once without them and stop sending them.
	if sentQuery && (resp.Status == http.StatusNotImplemented || isQueryNotSupported(resp)) {
		obs.FromContext(ctx, obs.SubRedfish).DebugContext(ctx, "redfish query rejected; retrying without options",
			"path", path, "status", resp.Status)
		c.disableQueryCaps(rc.query)
		plain, _ := c.resolve(path)
		return c.send(ctx, method, plain, b, rc)
	}
	return resp, nil
}

func appendQuery(u, qs string) string {
	if strings.Contains(u, "?") {
		return u + "&" + qs
	}
	return u + "?" + qs
}

func isQueryNotSupported(r *Response) bool {
	if r.Status != http.StatusBadRequest && r.Status != http.StatusNotImplemented {
		return false
	}
	id := messageID(r.Body)
	return strings.Contains(id, "QueryNotSupported") || strings.Contains(id, "QueryParameter")
}

// send performs auth, redirects and the 401 relogin for one URL.
func (c *Client) send(ctx context.Context, method, full string, body []byte, rc *reqConfig) (*Response, error) {
	if c.opts.Auth == AuthSession && !rc.noAuth {
		if err := c.ensureSession(ctx); err != nil {
			return nil, err
		}
	}
	var gen uint64
	for attempt := 0; ; attempt++ {
		resp, g, err := c.sendOnce(ctx, method, full, body, rc)
		if err != nil {
			return nil, err
		}
		gen = g
		if resp.Status == http.StatusUnauthorized && c.opts.Auth == AuthSession && !rc.noAuth && attempt == 0 {
			obs.FromContext(ctx, obs.SubRedfish).DebugContext(ctx, "redfish 401 in session mode; re-login", "path", resp.Path)
			if err := c.relogin(ctx, gen); err != nil {
				return nil, err
			}
			continue
		}
		return resp, nil
	}
}

func (c *Client) sendOnce(ctx context.Context, method, full string, body []byte, rc *reqConfig) (*Response, uint64, error) {
	var gen uint64
	for hops := 0; ; hops++ {
		var rd io.Reader
		if body != nil {
			rd = bytes.NewReader(body)
		}
		op := rc.op
		if op == "" {
			op = "redfish." + method
		}
		rctx := transport.WithOp(ctx, op)
		req, err := c.ep.NewRequest(rctx, method, full, rd)
		if err != nil {
			return nil, 0, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, method, "build request: %v", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("OData-Version", "4.0")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, vs := range rc.header {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		if rc.ifMatch != "" && rc.ifMatch != IfMatchNone {
			req.Header.Set("If-Match", rc.ifMatch)
		}
		if !rc.noAuth {
			switch c.opts.Auth {
			case AuthBasic:
				req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.opts.Creds.User+":"+c.opts.Creds.Pass)))
			case AuthSession:
				tok, g := c.token()
				gen = g
				if tok != "" {
					req.Header.Set("X-Auth-Token", tok)
				}
			}
		}
		hresp, err := c.ep.Do(req)
		if err != nil {
			return nil, gen, err
		}
		data, rerr := io.ReadAll(hresp.Body)
		hresp.Body.Close()
		if rerr != nil {
			return nil, gen, bmc.Wrap(Pkg, method+" "+c.pathOf(full), rerr)
		}
		c.touchSession()
		resp := &Response{Method: method, Path: c.pathOf(full), Status: hresp.StatusCode, Header: hresp.Header, Body: data}
		resp.ETag = hresp.Header.Get("ETag")
		if resp.ETag == "" && len(data) > 0 && data[0] == '{' {
			var e struct {
				ETag string `json:"@odata.etag"`
			}
			if json.Unmarshal(data, &e) == nil {
				resp.ETag = e.ETag
			}
		}
		if loc := hresp.Header.Get("Location"); loc != "" {
			resp.Location = c.pathOf(loc)
		}
		if resp.ETag != "" && (method == http.MethodGet) && resp.Status == http.StatusOK {
			c.rememberETag(resp.Path, resp.ETag)
		}
		if (method == http.MethodGet || method == http.MethodHead) && isRedirect(resp.Status) && resp.Location != "" && hops < 3 && !c.opts.Quirks.NoRedirects {
			next, err := c.resolve(resp.Location)
			if err != nil {
				return resp, gen, nil
			}
			full = next
			continue
		}
		return resp, gen, nil
	}
}

func isRedirect(s int) bool {
	return s == http.StatusMovedPermanently || s == http.StatusFound || s == http.StatusSeeOther ||
		s == http.StatusTemporaryRedirect || s == http.StatusPermanentRedirect
}

// ---- ETags ----

func (c *Client) rememberETag(path, etag string) {
	p := cleanPath(path)
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.etags) > 1024 {
		c.etags = map[string]string{}
	}
	c.etags[p] = etag
}

// ETag returns the last ETag seen on a 200 GET of path ("" if unknown).
func (c *Client) ETag(path string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.etags[cleanPath(c.opts.Quirks.overridePath(path))]
}

func (c *Client) forgetETag(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.etags, cleanPath(path))
}

// ---- typed verbs (status -> error mapping) ----

// Get GETs path and decodes the body into out (may be nil). Non-2xx
// statuses become *bmc.OpError values (401 ErrAuth, 404 ErrNotFound, ...).
func (c *Client) Get(ctx context.Context, path string, out any, opts ...RequestOption) (*Response, error) {
	resp, err := c.Do(ctx, http.MethodGet, path, nil, opts...)
	if err != nil {
		return nil, err
	}
	if err := StatusError(resp); err != nil {
		return resp, err
	}
	if out != nil {
		if err := c.Decode(resp, out); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

// GetQuery is Get with client query options.
func (c *Client) GetQuery(ctx context.Context, path string, q Query, out any, opts ...RequestOption) (*Response, error) {
	return c.Get(ctx, path, out, append(opts, WithQuery(q))...)
}

// Patch PATCHes body to path. Unless suppressed (WithIfMatch(IfMatchNone)
// or Quirks.NoIfMatch) it sends If-Match with the last ETag seen for path;
// with Quirks.RequireIfMatch it GETs the resource first when no ETag is
// known. A 412 (stale ETag) or 428 (precondition required) is retried once
// after re-reading the ETag.
func (c *Client) Patch(ctx context.Context, path string, body any, opts ...RequestOption) (*Response, error) {
	var rc reqConfig
	for _, o := range opts {
		o(&rc)
	}
	explicit := rc.ifMatch != ""
	if !explicit && !c.opts.Quirks.NoIfMatch {
		et := c.ETag(path)
		if et == "" && c.opts.Quirks.RequireIfMatch {
			if r, err := c.Get(ctx, path, nil); err == nil {
				et = r.ETag
			}
		}
		rc.ifMatch = et
	}
	resp, err := c.do(ctx, http.MethodPatch, path, body, &rc)
	if err != nil {
		return nil, err
	}
	if !explicit && !c.opts.Quirks.NoIfMatch && (resp.Status == http.StatusPreconditionFailed || resp.Status == http.StatusPreconditionRequired) {
		c.forgetETag(path)
		r, gerr := c.Get(ctx, path, nil)
		if gerr == nil && r.ETag != "" {
			rc.ifMatch = r.ETag
			resp, err = c.do(ctx, http.MethodPatch, path, body, &rc)
			if err != nil {
				return nil, err
			}
		}
	}
	if resp.IsSuccess() {
		c.forgetETag(path) // the resource changed
		if resp.ETag != "" {
			c.rememberETag(path, resp.ETag)
		}
	}
	return resp, StatusError(resp)
}

// Post POSTs body to path.
func (c *Client) Post(ctx context.Context, path string, body any, opts ...RequestOption) (*Response, error) {
	if body == nil {
		body = map[string]any{}
	}
	resp, err := c.Do(ctx, http.MethodPost, path, body, opts...)
	if err != nil {
		return nil, err
	}
	return resp, StatusError(resp)
}

// Delete DELETEs path.
func (c *Client) Delete(ctx context.Context, path string, opts ...RequestOption) (*Response, error) {
	resp, err := c.Do(ctx, http.MethodDelete, path, nil, opts...)
	if err != nil {
		return nil, err
	}
	return resp, StatusError(resp)
}

// Decode leniently decodes resp's body into out applying the client's
// quirks (e.g. "NA" -> null).
func (c *Client) Decode(resp *Response, out any) error {
	if err := decodeLenient(resp.Body, out, c.opts.Quirks); err != nil {
		return &bmc.OpError{Pkg: Pkg, Op: resp.Method + " " + resp.Path, Backend: bmc.BackendRedfish,
			UpstreamStatus: resp.Status, Detail: "decode: " + err.Error(), Err: bmc.ErrUpstream}
	}
	return nil
}

// ---- errors ----

// StatusError maps a non-2xx Redfish response to a *bmc.OpError (nil for
// 1xx/2xx/3xx):
//
//	401 -> ErrAuth, 403 -> ErrForbidden, 404/410 -> ErrNotFound,
//	405/501 -> ErrUnsupported, 400/422 -> ErrInvalidArgument,
//	409/412/428 -> ErrBusy (concurrent modification / precondition),
//	503 -> ErrBusy, 504 -> ErrTimeout, other -> ErrUpstream.
func StatusError(resp *Response) error {
	if resp == nil || resp.Status < 400 {
		return nil
	}
	var sentinel error
	switch resp.Status {
	case http.StatusUnauthorized:
		sentinel = bmc.ErrAuth
	case http.StatusForbidden:
		sentinel = bmc.ErrForbidden
	case http.StatusNotFound, http.StatusGone:
		sentinel = bmc.ErrNotFound
	case http.StatusMethodNotAllowed, http.StatusNotImplemented:
		sentinel = bmc.ErrUnsupported
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		sentinel = bmc.ErrInvalidArgument
	case http.StatusConflict, http.StatusPreconditionFailed, http.StatusPreconditionRequired, http.StatusServiceUnavailable:
		sentinel = bmc.ErrBusy
	case http.StatusGatewayTimeout:
		sentinel = bmc.ErrTimeout
	default:
		sentinel = bmc.ErrUpstream
	}
	return &bmc.OpError{Pkg: Pkg, Op: resp.Method + " " + cleanPath(resp.Path), Backend: bmc.BackendRedfish,
		UpstreamStatus: resp.Status, Detail: errorDetail(resp.Body), Err: sentinel}
}

// ErrorInfo is the decoded DMTF error envelope.
type ErrorInfo struct {
	Code         string        `json:"code"`
	Message      string        `json:"message"`
	ExtendedInfo []MessageInfo `json:"@Message.ExtendedInfo"`
}

// MessageInfo is one @Message.ExtendedInfo entry.
type MessageInfo struct {
	MessageID   string   `json:"MessageId"`
	Message     string   `json:"Message"`
	MessageArgs []string `json:"MessageArgs"`
	Severity    string   `json:"Severity"`
	Resolution  string   `json:"Resolution"`
}

// ParseError decodes the DMTF error envelope from body (nil if absent).
func ParseError(body []byte) *ErrorInfo {
	var env struct {
		Error *ErrorInfo `json:"error"`
	}
	if len(body) == 0 || json.Unmarshal(body, &env) != nil {
		return nil
	}
	return env.Error
}

func messageID(body []byte) string {
	e := ParseError(body)
	if e == nil {
		return ""
	}
	for _, m := range e.ExtendedInfo {
		if m.MessageID != "" {
			return m.MessageID
		}
	}
	return e.Code
}

func errorDetail(body []byte) string {
	e := ParseError(body)
	if e == nil {
		s := strings.TrimSpace(string(body))
		if len(s) > 200 {
			s = s[:200] + "..."
		}
		return obs.RedactString(s)
	}
	parts := []string{}
	if id := messageID(body); id != "" {
		parts = append(parts, id)
	}
	msg := e.Message
	for _, m := range e.ExtendedInfo {
		if m.Message != "" {
			msg = m.Message
			break
		}
	}
	if msg != "" {
		parts = append(parts, msg)
	}
	return strings.Join(parts, ": ")
}

// IsStatus reports whether err is an OpError carrying upstream status s.
func IsStatus(err error, s int) bool {
	var oe *bmc.OpError
	return errors.As(err, &oe) && oe.UpstreamStatus == s
}
