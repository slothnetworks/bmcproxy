package amimegarac

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
)

// Defaults for Config fields left zero.
const (
	DefaultLoginTimeout        = 30 * time.Second
	DefaultDegradedFor         = 60 * time.Second
	DefaultLoginFailureBackoff = 5 * time.Second
)

// CookieName is the MegaRAC web session cookie.
const CookieName = "QSESSIONID"

// CSRFHeader is the header that carries the CSRF token from the login
// response.
const CSRFHeader = "X-CSRFTOKEN"

// Config configures a Client.
type Config struct {
	// Endpoint is the transport endpoint for the BMC, normally
	// tm.Endpoint(target.Addr(), transport.LaneAMI). Required.
	Endpoint *transport.Endpoint
	// Target identifies the BMC (for logs and hooks).
	Target bmc.Target
	// Creds are the web/IPMI credentials.
	Creds bmc.Creds
	// LoginExtra holds extra login form fields. X470-class (SPX 2.x/3.x)
	// firmware sends certlogin=0 (see X470LoginExtra); B650 sends none.
	LoginExtra url.Values
	// ExtraDeny adds board-specific deny rules on top of DefaultDenyRules.
	ExtraDeny []DenyRule
	// Hooks receive session lifecycle events.
	Hooks Hooks
	// Obs is the fallback logger source when the request context has none.
	Obs *obs.Obs
	// LoginTimeout bounds one login attempt. The login runs detached from
	// the caller's cancellation, because other goroutines may be waiting on
	// it. Default DefaultLoginTimeout.
	LoginTimeout time.Duration
	// DegradedFor is how long Degraded stays true after a re-login or a
	// backend restart. Default DefaultDegradedFor.
	DegradedFor time.Duration
	// LoginFailureBackoff caches a credential rejection for this long, so a
	// burst of requests does not hammer the BMC with bad logins (and trip
	// its lockout). Default DefaultLoginFailureBackoff; negative disables.
	LoginFailureBackoff time.Duration
	// LogoutRetryDelay is the pause before Logout retries a 5xx answer
	// (default 1s).
	LogoutRetryDelay time.Duration
	// Now is the clock (tests). Default time.Now.
	Now func() time.Time
	// ReportSessionLost makes a backend restart a hard, sticky signal: once
	// the client sees one (Hooks.OnHostInvalidated) or MarkHostLost is
	// called, the current session is dropped and Do (and everything built
	// on it: DoJSON, Get, Clock, ...) fails with an error wrapping
	// bmc.ErrSessionLost, without contacting the BMC, until an explicit
	// Login establishes a fresh session. The request that saw the restart
	// fails the same way. A session cache uses this to invalidate every
	// session on the host and retry once (sessions.Manager.Do). Without it
	// (the default) the call that saw the restart fails with
	// bmc.ErrUpstream and the next request logs in again on its own.
	ReportSessionLost bool
}

// X470LoginExtra returns the extra login field that X470-class firmware's
// web UI sends.
func X470LoginExtra() url.Values { return url.Values{"certlogin": {"0"}} }

// Hooks are optional callbacks. They are called synchronously, without any
// Client lock held, and must be fast and concurrency-safe. A hook may call
// back into the Client.
type Hooks struct {
	// OnLogin runs after every successful login, including re-logins.
	OnLogin func(SessionInfo)
	// OnRelogin runs after every re-login attempt triggered by an invalid
	// session. Err is nil on success.
	OnRelogin func(ReloginEvent)
	// OnHostInvalidated runs when a response suggests the BMC web backend
	// restarted (502, or an HTML 500), which invalidates every web session
	// on the host. A session cache should drop all sessions for Host.
	OnHostInvalidated func(host, reason string)
}

// ReloginEvent describes one re-login attempt.
type ReloginEvent struct {
	Host     string
	Reason   string // what triggered it, e.g. "401 cc:7 on GET /api/sensors"
	Op       string // the request that saw the invalid session
	At       time.Time
	Duration time.Duration
	Err      error
}

// SessionInfo is the metadata from a successful login.
type SessionInfo struct {
	Host       string        `json:"host"`
	User       string        `json:"user"`
	LoginAt    time.Time     `json:"loginAt"`
	LoginTook  time.Duration `json:"loginTook"`
	Generation uint64        `json:"generation"` // increases by one per login on this Client
	// Privilege is the IPMI privilege level (4 = administrator).
	Privilege     int           `json:"privilege"`
	PrivilegeName bmc.Privilege `json:"privilegeName,omitempty"`
	ExtendedPriv  int           `json:"extendedPriv"`
	RACSessionID  int           `json:"racSessionId"`
	// UserID, Channel and PasswordStatus are only present on newer
	// firmware (B650).
	UserID         int    `json:"userId,omitempty"`
	Channel        int    `json:"channel,omitempty"`
	PasswordStatus int    `json:"passwordStatus,omitempty"`
	RemoteAddr     string `json:"remoteAddr,omitempty"`
	ServerName     string `json:"serverName,omitempty"`
	ServerAddr     string `json:"serverAddr,omitempty"`
	HTTPSEnabled   bool   `json:"httpsEnabled"`
	// BMCDate is the login response's Date header (the BMC's clock, 1 s
	// resolution). It is zero if the header was missing.
	BMCDate time.Time `json:"bmcDate,omitzero"`
	// ClockOffset is a coarse estimate of bmc − proxy, taken from BMCDate
	// against the midpoint of the login round trip. Use Client.Clock for a
	// dedicated sample.
	ClockOffset time.Duration `json:"clockOffset"`
	// Raw is the full login body, minus CSRFToken.
	Raw map[string]any `json:"raw,omitempty"`
}

// LoginResponse is the JSON body of POST /api/session. The X470 and B650
// fields are shown in docs/x470/har-endpoints.json and
// docs/b650/har-endpoints.json, entry "POST /api/session".
type LoginResponse struct {
	OK             NullableInt `json:"ok"`
	Privilege      NullableInt `json:"privilege"`
	ExtendedPriv   NullableInt `json:"extendedpriv"`
	RACSessionID   NullableInt `json:"racsession_id"`
	UserID         NullableInt `json:"user_id"`
	Channel        NullableInt `json:"channel"`
	PasswordStatus NullableInt `json:"passwordStatus"`
	RemoteAddr     FlexString  `json:"remote_addr"`
	ServerName     FlexString  `json:"server_name"`
	ServerAddr     FlexString  `json:"server_addr"`
	HTTPSEnabled   FlexBool    `json:"HTTPSEnabled"`
	CSRFToken      FlexString  `json:"CSRFToken"`
}

// PrivilegeFromIPMI maps an IPMI privilege level to bmc.Privilege.
func PrivilegeFromIPMI(p int) bmc.Privilege {
	switch p {
	case 1:
		return bmc.PrivCallback
	case 2:
		return bmc.PrivUser
	case 3:
		return bmc.PrivOperator
	case 4:
		return bmc.PrivAdministrator
	case 5:
		return bmc.PrivOEM
	case 15:
		return bmc.PrivNoAccess
	}
	return ""
}

// Stats is a snapshot of the client's session counters.
type Stats struct {
	LoggedIn          bool      `json:"loggedIn"`
	Generation        uint64    `json:"generation"`
	Logins            uint64    `json:"logins"`        // successful logins, including re-logins
	Relogins          uint64    `json:"relogins"`      // re-login attempts after an invalid session
	LoginFailures     uint64    `json:"loginFailures"` // failed login attempts
	Logouts           uint64    `json:"logouts"`
	HostInvalidations uint64    `json:"hostInvalidations"`
	DenylistBlocks    uint64    `json:"denylistBlocks"`
	Degraded          bool      `json:"degraded"`
	LastReloginAt     time.Time `json:"lastReloginAt,omitzero"`
}

type session struct {
	id   string
	csrf string
	gen  uint64
	info SessionInfo
}

// Client is a MegaRAC /api client bound to one BMC and one set of
// credentials. Safe for concurrent use.
type Client struct {
	ep    *transport.Endpoint
	host  string
	creds bmc.Creds
	cfg   Config
	deny  *Denylist
	now   func() time.Time

	sf singleflight.Group

	mu            sync.Mutex
	sess          *session
	gen           uint64
	degradedUntil time.Time
	lastRelogin   time.Time
	failErr       error
	failUntil     time.Time
	lost          bool   // see Config.ReportSessionLost / MarkHostLost
	lostGen       uint64 // sessions up to this generation are gone
	lostReason    string

	logins, relogins, loginFailures, logouts, hostInval, denyBlocks atomic.Uint64
}

// New builds a Client. It does not contact the BMC: the first request (or
// an explicit Login) logs in.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == nil {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "New", "nil transport endpoint")
	}
	if cfg.LoginTimeout <= 0 {
		cfg.LoginTimeout = DefaultLoginTimeout
	}
	if cfg.DegradedFor <= 0 {
		cfg.DegradedFor = DefaultDegradedFor
	}
	if cfg.LoginFailureBackoff == 0 {
		cfg.LoginFailureBackoff = DefaultLoginFailureBackoff
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	host := cfg.Endpoint.Host()
	if cfg.Target.Host != "" {
		host = cfg.Target.Addr()
	}
	return &Client{
		ep:    cfg.Endpoint,
		host:  host,
		creds: cfg.Creds,
		cfg:   cfg,
		deny:  NewDenylist(cfg.ExtraDeny...),
		now:   now,
	}, nil
}

// Host returns the BMC "host:port".
func (c *Client) Host() string { return c.host }

// Endpoint returns the transport endpoint, for example to use Exclusive
// around a multi-step sequence. Requests made through the Client with the
// ctx that Exclusive passes reuse the held lane slot. Call Login before
// entering Exclusive. Otherwise a login that another goroutine started
// (and so does not hold the slot) waits behind the section that is waiting
// for that login, until LoginTimeout expires.
func (c *Client) Endpoint() *transport.Endpoint { return c.ep }

// Denylist returns the client's add-only denylist.
func (c *Client) Denylist() *Denylist { return c.deny }

// Session returns the current session metadata, and false when there is no
// live session.
func (c *Client) Session() (SessionInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sess == nil {
		return SessionInfo{}, false
	}
	return c.sess.info, true
}

// Degraded reports whether the client re-logged in or saw a backend
// restart within the last Config.DegradedFor.
func (c *Client) Degraded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Before(c.degradedUntil)
}

// Stats returns a counters snapshot.
func (c *Client) Stats() Stats {
	c.mu.Lock()
	s := Stats{
		LoggedIn:      c.sess != nil,
		Generation:    c.gen,
		Degraded:      c.now().Before(c.degradedUntil),
		LastReloginAt: c.lastRelogin,
	}
	c.mu.Unlock()
	s.Logins = c.logins.Load()
	s.Relogins = c.relogins.Load()
	s.LoginFailures = c.loginFailures.Load()
	s.Logouts = c.logouts.Load()
	s.HostInvalidations = c.hostInval.Load()
	s.DenylistBlocks = c.denyBlocks.Load()
	return s
}

// DropSession forgets the current session without logging out, so the next
// request logs in again. A session cache calls this after
// OnHostInvalidated, to skip the 401 round trip.
func (c *Client) DropSession() {
	c.mu.Lock()
	c.sess = nil
	c.mu.Unlock()
}

func (c *Client) log(ctx context.Context) *slog.Logger {
	return obs.LoggerFrom(ctx, c.cfg.Obs, obs.SubAMI)
}

// ---- login / logout ----

// Login makes sure a session exists. With no session it logs in; otherwise
// it does nothing. After a host invalidation (Config.ReportSessionLost or
// MarkHostLost) it logs in afresh, which clears the session-lost state.
func (c *Client) Login(ctx context.Context) error {
	c.mu.Lock()
	lost, lostGen := c.lost, c.lostGen
	c.mu.Unlock()
	if lost {
		if _, err := c.acquire(ctx, lostGen, "", ""); err != nil {
			return err
		}
		c.mu.Lock()
		if c.sess != nil && c.sess.gen > c.lostGen {
			c.lost, c.lostReason = false, ""
		}
		c.mu.Unlock()
		return nil
	}
	_, err := c.ensureSession(ctx)
	return err
}

// MarkHostLost records that every web session on the host is gone (for
// example after the caller requested a BMC reset). The current session is
// forgotten without a logout, and Do fails with bmc.ErrSessionLost until
// the next explicit Login, whatever Config.ReportSessionLost says.
func (c *Client) MarkHostLost(reason string) {
	c.mu.Lock()
	c.markLostLocked(reason)
	c.degradedUntil = c.now().Add(c.cfg.DegradedFor)
	c.mu.Unlock()
}

func (c *Client) markLostLocked(reason string) {
	if !c.lost || c.lostReason == "" {
		c.lostReason = reason
	}
	c.lost = true
	c.lostGen = c.gen
	c.sess = nil
}

// HostLost reports whether the client is in the session-lost state, and why.
func (c *Client) HostLost() (bool, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lost && (c.sess == nil || c.sess.gen <= c.lostGen), c.lostReason
}

// SessionLostError returns the error Do would return for op while the host
// is in the session-lost state (an *bmc.OpError wrapping
// bmc.ErrSessionLost), or nil.
func (c *Client) SessionLostError(op string) error {
	lost, why := c.HostLost()
	if !lost {
		return nil
	}
	return c.lostErr(op, 0, why, nil)
}

func (c *Client) lostErr(op string, status int, why string, cause error) error {
	detail := "BMC web backend restarted; every session on the host is gone"
	if why != "" {
		detail += " (" + why + ")"
	}
	err := bmc.ErrSessionLost
	if cause != nil {
		err = fmt.Errorf("%w: %w", bmc.ErrSessionLost, cause)
	}
	return &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, UpstreamStatus: status, Detail: detail, Err: err}
}

func (c *Client) ensureSession(ctx context.Context) (*session, error) {
	c.mu.Lock()
	s, gen := c.sess, c.gen
	c.mu.Unlock()
	if s != nil {
		return s, nil
	}
	return c.acquire(ctx, gen, "", "")
}

// acquire returns a session newer than staleGen, logging in (singleflighted)
// if needed. reason is non-empty for re-logins.
func (c *Client) acquire(ctx context.Context, staleGen uint64, reason, op string) (*session, error) {
	if s := c.fresher(staleGen); s != nil {
		return s, nil
	}
	ch := c.sf.DoChan("login", func() (any, error) {
		if s := c.fresher(staleGen); s != nil {
			return s, nil
		}
		return c.loginOnce(ctx, reason, op)
	})
	select {
	case r := <-ch:
		if r.Err != nil {
			return nil, r.Err
		}
		return r.Val.(*session), nil
	case <-ctx.Done():
		return nil, bmc.Wrap(Pkg, "Login", ctx.Err())
	}
}

// fresher returns the current session if it is newer than staleGen.
func (c *Client) fresher(staleGen uint64) *session {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sess != nil && c.sess.gen > staleGen {
		return c.sess
	}
	return nil
}

// invalidate drops the session if it is still generation gen.
func (c *Client) invalidate(gen uint64) {
	c.mu.Lock()
	if c.sess != nil && c.sess.gen == gen {
		c.sess = nil
	}
	c.mu.Unlock()
}

// loginOnce performs one login. It always runs inside the singleflight.
func (c *Client) loginOnce(callerCtx context.Context, reason, op string) (*session, error) {
	relogin := reason != ""
	start := c.now()

	c.mu.Lock()
	if c.failErr != nil && start.Before(c.failUntil) {
		err := c.failErr
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	// Detach from the caller's cancellation (others may be waiting on this
	// login) but keep its values (obs logger, ids).
	ctx, cancel := context.WithTimeout(context.WithoutCancel(callerCtx), c.cfg.LoginTimeout)
	defer cancel()
	ctx = transport.WithOptions(ctx, transport.RequestOptions{Op: "amimegarac.Login"})
	log := c.log(ctx)
	if relogin {
		c.relogins.Add(1)
		log.DebugContext(ctx, "amimegarac relogin", "bmc", c.host, "reason", reason, "op", op)
	} else {
		log.DebugContext(ctx, "amimegarac login", "bmc", c.host, "user", c.creds.User)
	}

	s, err := c.doLogin(ctx)
	took := c.now().Sub(start)

	c.mu.Lock()
	if err != nil {
		if errors.Is(err, bmc.ErrAuth) && c.cfg.LoginFailureBackoff > 0 {
			c.failErr, c.failUntil = err, c.now().Add(c.cfg.LoginFailureBackoff)
		}
	} else {
		c.failErr = nil
		c.gen++
		s.gen = c.gen
		s.info.Generation = c.gen
		s.info.LoginAt = start
		s.info.LoginTook = took
		c.sess = s
	}
	if relogin {
		c.lastRelogin = start
		c.degradedUntil = c.now().Add(c.cfg.DegradedFor)
	}
	c.mu.Unlock()

	if err != nil {
		c.loginFailures.Add(1)
		log.DebugContext(ctx, "amimegarac login failed", "bmc", c.host, "relogin", relogin, "took", took, "err", err)
	} else {
		c.logins.Add(1)
		log.DebugContext(ctx, "amimegarac login ok", "bmc", c.host, "relogin", relogin,
			"generation", s.gen, "privilege", s.info.Privilege, "took", took)
		if h := c.cfg.Hooks.OnLogin; h != nil {
			h(s.info)
		}
	}
	if relogin {
		if h := c.cfg.Hooks.OnRelogin; h != nil {
			h(ReloginEvent{Host: c.host, Reason: reason, Op: op, At: start, Duration: took, Err: err})
		}
	}
	return s, err
}

func (c *Client) doLogin(ctx context.Context) (*session, error) {
	const op = "POST /api/session"
	form := url.Values{}
	form.Set("username", c.creds.User)
	form.Set("password", c.creds.Pass)
	for k, vs := range c.cfg.LoginExtra {
		if k == "username" || k == "password" {
			continue
		}
		form[k] = append([]string(nil), vs...)
	}
	req, err := c.ep.NewRequest(ctx, http.MethodPost, "/api/session", form.Encode())
	if err != nil {
		return nil, bmc.Wrap(Pkg, op, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	setCommonHeaders(req.Header)

	sent := c.now()
	resp, err := c.ep.Do(req)
	if err != nil {
		return nil, err // already a classified *bmc.OpError
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	recv := c.now()

	if crash, why := isBackendCrash(resp.StatusCode, resp.Header, body); crash {
		c.hostInvalidated(ctx, why)
	}
	if resp.StatusCode != http.StatusOK {
		ae := ParseAPIError(resp.StatusCode, resp.Header, body)
		switch {
		case ae.Code == CodeMaxSessions:
			return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, UpstreamStatus: resp.StatusCode,
				Detail: "BMC session table full: " + ae.Error(), Err: fmt.Errorf("%w: %w", bmc.ErrBusy, ae)}
		case resp.StatusCode >= 500:
			return nil, StatusError(op, resp.StatusCode, resp.Header, body)
		default:
			return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, UpstreamStatus: resp.StatusCode,
				Detail: "login rejected: " + ae.Error(), Err: fmt.Errorf("%w: %w", bmc.ErrAuth, ae)}
		}
	}

	var lr LoginResponse
	if err := json.Unmarshal(bytes.TrimSpace(body), &lr); err != nil {
		return nil, opErr(op, resp.StatusCode, bmc.ErrUpstream, "decode login response: %v", err)
	}
	if lr.OK.Valid && lr.OK.V != 0 {
		return nil, opErr(op, resp.StatusCode, bmc.ErrAuth, "login response ok=%d", lr.OK.V)
	}
	csrf := strings.TrimSpace(string(lr.CSRFToken))
	if csrf == "" {
		return nil, opErr(op, resp.StatusCode, bmc.ErrUpstream, "login response has no CSRFToken")
	}
	var sid string
	for _, ck := range resp.Cookies() {
		if ck.Name == CookieName && ck.Value != "" {
			sid = ck.Value
		}
	}
	if sid == "" {
		return nil, opErr(op, resp.StatusCode, bmc.ErrUpstream, "login response did not set %s", CookieName)
	}

	info := SessionInfo{
		Host:           c.host,
		User:           c.creds.User,
		Privilege:      lr.Privilege.Int(),
		PrivilegeName:  PrivilegeFromIPMI(lr.Privilege.Int()),
		ExtendedPriv:   lr.ExtendedPriv.Int(),
		RACSessionID:   lr.RACSessionID.Int(),
		UserID:         lr.UserID.Int(),
		Channel:        lr.Channel.Int(),
		PasswordStatus: lr.PasswordStatus.Int(),
		RemoteAddr:     lr.RemoteAddr.Clean(),
		ServerName:     lr.ServerName.Clean(),
		ServerAddr:     lr.ServerAddr.Clean(),
		HTTPSEnabled:   bool(lr.HTTPSEnabled),
	}
	var raw map[string]any
	if json.Unmarshal(bytes.TrimSpace(body), &raw) == nil {
		delete(raw, "CSRFToken")
		info.Raw = raw
	}
	if d, err := http.ParseTime(resp.Header.Get("Date")); err == nil {
		info.BMCDate = d
		mid := sent.Add(recv.Sub(sent) / 2)
		info.ClockOffset = d.Sub(mid).Truncate(time.Second)
	}
	return &session{id: sid, csrf: csrf, info: info}, nil
}

// Logout ends the session with DELETE /api/session. It is idempotent. A 401
// (session already gone) counts as success. The session is forgotten even
// if the call fails. A later request logs in again.
func (c *Client) Logout(ctx context.Context) error {
	const op = "DELETE /api/session"
	c.mu.Lock()
	s := c.sess
	c.sess = nil
	c.mu.Unlock()
	if s == nil {
		return nil
	}
	ctx = transport.WithOptions(ctx, transport.RequestOptions{Op: "amimegarac.Logout", NoRetry: true})
	req, err := c.ep.NewRequest(ctx, http.MethodDelete, "/api/session", nil)
	if err != nil {
		return bmc.Wrap(Pkg, op, err)
	}
	setCommonHeaders(req.Header)
	setAuth(req.Header, s)
	log := c.log(ctx)
	var (
		status int
		hdr    http.Header
		body   []byte
	)
	// X570D4U 1.20 answered DELETE /api/session with a lighttpd HTML 500
	// while the session stayed valid (docs/x570/live-probe.md), which leaks
	// one of the BMC's web session slots. Logout is idempotent (401 = gone),
	// so a 5xx is retried once after a short pause.
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := c.ep.Do(req.Clone(ctx))
		c.logouts.Add(1)
		if err != nil {
			log.DebugContext(ctx, "amimegarac logout failed", "bmc", c.host, "generation", s.gen, "err", err)
			return err
		}
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		status, hdr = resp.StatusCode, resp.Header
		log.DebugContext(ctx, "amimegarac logout", "bmc", c.host, "generation", s.gen, "status", status, "attempt", attempt)
		if status == http.StatusUnauthorized || status/100 == 2 {
			return nil
		}
		if status < 500 || attempt == 2 {
			break
		}
		t := time.NewTimer(c.logoutRetryDelay())
		select {
		case <-ctx.Done():
			t.Stop()
			return bmc.Wrap(Pkg, op, ctx.Err())
		case <-t.C:
		}
	}
	return StatusError(op, status, hdr, body)
}

// logoutRetryDelay is the pause before retrying a 5xx logout.
func (c *Client) logoutRetryDelay() time.Duration {
	if c.cfg.LogoutRetryDelay > 0 {
		return c.cfg.LogoutRetryDelay
	}
	return time.Second
}

// ---- requests ----

// Request is a raw /api request.
type Request struct {
	// Method defaults to GET. Only standard methods are allowed.
	Method string
	// Path is "/api/x", "api/x" or "x" (relative to /api), optionally with
	// "?query". See CanonicalPath.
	Path string
	// Query is merged into any query string already in Path.
	Query url.Values
	// Body is sent as-is. It is a []byte so that it can be resent after a
	// re-login.
	Body []byte
	// ContentType defaults to application/json when Body is non-empty.
	ContentType string
	// Header holds extra headers. Auth, cookie, hop-by-hop and
	// method-override headers are always removed.
	Header http.Header
	// Idempotent allows a write to be replayed after a re-login even when
	// the 401 body is not the cc:7 auth-gate rejection. It also allows
	// transport-level retries. Setting transport.WithIdempotent on the
	// context does the same.
	Idempotent bool
	// Op names the call for logs and HAR. Default "METHOD /api/path".
	Op string
}

// Response is a buffered /api response.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
	// Method and Path are the canonical request method and path.
	Method string
	Path   string
}

// Err maps a non-2xx status to an error (see StatusError). It returns nil
// for 2xx.
func (r *Response) Err() error {
	return StatusError(r.Method+" "+r.Path, r.Status, r.Header, r.Body)
}

// Decode checks Err, then decodes the body into dst.
func (r *Response) Decode(dst any) error {
	if err := r.Err(); err != nil {
		return err
	}
	if err := Decode(r.Body, dst); err != nil {
		return opErr(r.Method+" "+r.Path, r.Status, bmc.ErrUpstream, "decode: %v: %s", err, excerpt(r.Body))
	}
	return nil
}

var allowedMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodOptions: true,
}

// strippedHeaders are never forwarded from Request.Header.
var strippedHeaders = []string{
	"Cookie", "Authorization", CSRFHeader, "X-Auth-Token",
	"X-Http-Method-Override", "X-Http-Method", "X-Method-Override",
	"Host", "Content-Length", "Connection", "Keep-Alive", "Transfer-Encoding", "Te", "Trailer",
	"Upgrade", "Proxy-Authorization", "Proxy-Connection", "Origin", "Referer",
	"X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "Forwarded",
}

// Do sends a raw request with session handling, the denylist and the
// re-login rule (see the package doc). It returns the response for any
// status the BMC answers, including 4xx and 5xx, except for a final 401,
// which becomes an error wrapping bmc.ErrAuth. Errors from transport (busy,
// timeout, connection) pass through unchanged. Denied paths return
// bmc.ErrForbidden, and malformed requests return bmc.ErrInvalidArgument,
// both without contacting the BMC.
//
// This is the primitive behind the raw/api/{path...} passthrough route.
func (c *Client) Do(ctx context.Context, r *Request) (*Response, error) {
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = http.MethodGet
	}
	cpath, rawQuery, err := CanonicalPath(r.Path)
	if err != nil {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, method+" "+r.Path, "%v", err)
	}
	op := method + " " + cpath
	if !allowedMethods[method] {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "method %q not allowed", method)
	}
	if rule, denied := c.deny.Match(cpath); denied {
		c.denyBlocks.Add(1)
		c.log(ctx).InfoContext(ctx, "amimegarac denylist blocked request", "bmc", c.host, "op", op, "rule", rule.Pattern)
		return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI,
			Detail: fmt.Sprintf("endpoint denylisted (%s): %s", rule.Pattern, rule.Reason), Err: bmc.ErrForbidden}
	}
	if len(r.Query) > 0 {
		if rawQuery != "" {
			rawQuery += "&"
		}
		rawQuery += r.Query.Encode()
	}
	target := escapePath(cpath)
	if rawQuery != "" {
		target += "?" + rawQuery
	}

	if err := c.SessionLostError(op); err != nil {
		return nil, err
	}

	opts := transport.OptionsFrom(ctx)
	idempotent := r.Idempotent || opts.Idempotent
	if opts.Op == "" {
		opts.Op = r.Op
		if opts.Op == "" {
			opts.Op = "amimegarac " + op
		}
	}
	opts.Idempotent = idempotent
	ctx = transport.WithOptions(ctx, opts)

	s, err := c.ensureSession(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(ctx, s, method, target, cpath, r)
	if err != nil {
		return nil, err
	}

	if resp.Status == http.StatusUnauthorized {
		gate := isAuthGateReject(resp.Body)
		c.invalidate(s.gen)
		reason := "401 on " + op
		if gate {
			reason = "401 cc:7 on " + op
		}
		if err := c.SessionLostError(op); err != nil {
			return nil, err // a restart was seen meanwhile: no silent re-login
		}
		ns, lerr := c.acquire(ctx, s.gen, reason, op)
		if lerr != nil {
			if errors.Is(lerr, bmc.ErrAuth) {
				return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, UpstreamStatus: http.StatusUnauthorized,
					Detail: "session invalid and re-login failed", Err: fmt.Errorf("%w: %w", bmc.ErrAuth, lerr)}
			}
			return nil, lerr
		}
		if !(transport.IsSafeMethod(method) || idempotent || gate) {
			return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, UpstreamStatus: http.StatusUnauthorized,
				Detail: "session expired; write not replayed because the BMC may have acted on it (session renewed)",
				Err:    fmt.Errorf("%w: %w", bmc.ErrAuth, ParseAPIError(resp.Status, resp.Header, resp.Body))}
		}
		resp, err = c.send(ctx, ns, method, target, cpath, r)
		if err != nil {
			return nil, err
		}
		if resp.Status == http.StatusUnauthorized {
			c.invalidate(ns.gen)
			return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, UpstreamStatus: http.StatusUnauthorized,
				Detail: "401 again after re-login", Err: fmt.Errorf("%w: %w", bmc.ErrAuth, ParseAPIError(resp.Status, resp.Header, resp.Body))}
		}
	}

	if crash, why := isBackendCrash(resp.Status, resp.Header, resp.Body); crash {
		c.hostInvalidated(ctx, why+" ("+op+")")
		if c.cfg.ReportSessionLost {
			return nil, c.lostErr(op, resp.Status, why+" ("+op+")", resp.Err())
		}
	}
	return resp, nil
}

func (c *Client) send(ctx context.Context, s *session, method, target, cpath string, r *Request) (*Response, error) {
	var body any
	if len(r.Body) > 0 {
		body = r.Body
	}
	req, err := c.ep.NewRequest(ctx, method, target, body)
	if err != nil {
		return nil, bmc.Wrap(Pkg, method+" "+cpath, err)
	}
	for k, vs := range r.Header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for _, h := range strippedHeaders {
		req.Header.Del(h)
	}
	if len(r.Body) > 0 {
		ct := r.ContentType
		if ct == "" {
			ct = req.Header.Get("Content-Type")
		}
		if ct == "" {
			ct = "application/json"
		}
		req.Header.Set("Content-Type", ct)
	}
	setCommonHeaders(req.Header)
	setAuth(req.Header, s)

	resp, err := c.ep.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, rerr := io.ReadAll(resp.Body)
	if rerr != nil {
		return nil, bmc.Wrap(Pkg, method+" "+cpath, fmt.Errorf("read body: %w", rerr))
	}
	return &Response{Status: resp.StatusCode, Header: resp.Header, Body: b, Method: method, Path: cpath}, nil
}

func setCommonHeaders(h http.Header) {
	if h.Get("Accept") == "" {
		h.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	}
	h.Set("X-Requested-With", "XMLHttpRequest")
}

func setAuth(h http.Header, s *session) {
	h.Set("Cookie", CookieName+"="+s.id)
	h.Set(CSRFHeader, s.csrf)
}

func (c *Client) hostInvalidated(ctx context.Context, reason string) {
	c.hostInval.Add(1)
	c.mu.Lock()
	c.degradedUntil = c.now().Add(c.cfg.DegradedFor)
	if c.cfg.ReportSessionLost {
		c.markLostLocked(reason)
	}
	c.mu.Unlock()
	c.log(ctx).WarnContext(ctx, "amimegarac backend restart suspected; host sessions likely invalid",
		"bmc", c.host, "reason", reason)
	if h := c.cfg.Hooks.OnHostInvalidated; h != nil {
		h(c.host, reason)
	}
}

// ---- JSON helpers ----

// DoJSON sends body as JSON (nil means no body; a []byte or json.RawMessage
// is sent verbatim), checks the status (see StatusError), and decodes the
// response into dst (nil to discard).
func (c *Client) DoJSON(ctx context.Context, method, path string, body, dst any) error {
	req := &Request{Method: method, Path: path}
	switch b := body.(type) {
	case nil:
	case []byte:
		req.Body = b
	case json.RawMessage:
		req.Body = b
	default:
		enc, err := json.Marshal(body)
		if err != nil {
			return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, method+" "+path, "encode body: %v", err)
		}
		req.Body = enc
	}
	resp, err := c.Do(ctx, req)
	if err != nil {
		return err
	}
	return resp.Decode(dst)
}

// Get fetches path and decodes the JSON response into dst.
func (c *Client) Get(ctx context.Context, path string, dst any) error {
	return c.DoJSON(ctx, http.MethodGet, path, nil, dst)
}

// Post sends body as JSON and decodes the response into dst.
func (c *Client) Post(ctx context.Context, path string, body, dst any) error {
	return c.DoJSON(ctx, http.MethodPost, path, body, dst)
}

// Put sends body as JSON and decodes the response into dst.
func (c *Client) Put(ctx context.Context, path string, body, dst any) error {
	return c.DoJSON(ctx, http.MethodPut, path, body, dst)
}

// Delete sends a DELETE and decodes the response into dst.
func (c *Client) Delete(ctx context.Context, path string, dst any) error {
	return c.DoJSON(ctx, http.MethodDelete, path, nil, dst)
}
