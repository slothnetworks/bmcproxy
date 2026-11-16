package redfish

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// DefaultSessionsPath is used when the ServiceRoot does not link Sessions.
const DefaultSessionsPath = "/redfish/v1/SessionService/Sessions"

type sessionState struct {
	token    string
	location string // session resource path, for DELETE
	gen      uint64 // bumped on every successful login
	lastUsed time.Time
	timeout  time.Duration // SessionService.SessionTimeout (0 = unknown)
}

// Login authenticates. In AuthSession mode it creates a session (if none is
// live); in AuthBasic mode it verifies the credentials with a GET of the
// Systems collection. A 401 is returned as bmc.ErrAuth.
func (c *Client) Login(ctx context.Context) error {
	switch c.opts.Auth {
	case AuthSession:
		return c.ensureSession(ctx)
	case AuthBasic:
		p := DefaultServiceRoot + "Systems"
		if root, err := c.ServiceRoot(ctx); err == nil && root.Systems.ID != "" {
			p = root.Systems.ID
		}
		_, err := c.Get(ctx, p, nil, WithOp("redfish.VerifyAuth"))
		return err
	}
	return nil
}

// SessionToken reports whether a session is live and its resource path
// (never the token itself).
func (c *Client) SessionLocation() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return ""
	}
	return c.session.location
}

func (c *Client) token() (string, uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		return "", 0
	}
	return c.session.token, c.session.gen
}

func (c *Client) touchSession() {
	if c.opts.Auth != AuthSession {
		return
	}
	c.mu.Lock()
	if c.session != nil {
		c.session.lastUsed = c.opts.Now()
	}
	c.mu.Unlock()
}

// ensureSession logs in if there is no session, or if the token has been
// idle for longer than SessionTimeout - SessionRefreshMargin.
func (c *Client) ensureSession(ctx context.Context) error {
	c.mu.Lock()
	s := c.session
	closed := c.closed
	var stale bool
	var gen uint64
	if s != nil {
		gen = s.gen
		if s.timeout > 0 {
			stale = c.opts.Now().Sub(s.lastUsed) > s.timeout-c.opts.SessionRefreshMargin
		}
	}
	c.mu.Unlock()
	if closed {
		return bmc.Errorf(bmc.ErrUpstream, Pkg, "session", "client closed")
	}
	if s != nil && !stale {
		return nil
	}
	if stale {
		obs.FromContext(ctx, obs.SubRedfish).DebugContext(ctx, "redfish session idle past timeout; refreshing", "location", s.location)
	}
	return c.relogin(ctx, gen)
}

// relogin replaces the session whose generation is gen. Concurrent callers
// that saw the same generation share one login.
func (c *Client) relogin(ctx context.Context, gen uint64) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	c.mu.Lock()
	if c.session != nil && c.session.gen != gen {
		c.mu.Unlock()
		return nil // someone else already logged in again
	}
	old := c.session
	c.mu.Unlock()
	if old != nil && old.location != "" {
		// Best effort: the old token may still be live (proactive refresh).
		c.deleteSession(ctx, old)
	}
	return c.createSession(ctx, gen)
}

func (c *Client) sessionsPath(ctx context.Context) string {
	if root, err := c.ServiceRoot(ctx); err == nil {
		if root.Links.Sessions.ID != "" {
			return root.Links.Sessions.ID
		}
	}
	return DefaultSessionsPath
}

func (c *Client) createSession(ctx context.Context, prevGen uint64) error {
	log := obs.FromContext(ctx, obs.SubRedfish)
	path := c.sessionsPath(ctx)
	body, _ := json.Marshal(map[string]string{"UserName": c.opts.Creds.User, "Password": c.opts.Creds.Pass})
	resp, _, err := c.sendOnce(ctx, http.MethodPost, mustResolve(c, path), body, &reqConfig{noAuth: true, op: "redfish.Login"})
	if err != nil {
		return err
	}
	if resp.Status == http.StatusUnauthorized || resp.Status == http.StatusForbidden {
		return &bmc.OpError{Pkg: Pkg, Op: "Login", Backend: bmc.BackendRedfish, UpstreamStatus: resp.Status,
			Detail: errorDetail(resp.Body), Err: bmc.ErrAuth}
	}
	if err := StatusError(resp); err != nil {
		return err
	}
	tok := resp.Header.Get("X-Auth-Token")
	if tok == "" {
		return &bmc.OpError{Pkg: Pkg, Op: "Login", Backend: bmc.BackendRedfish, UpstreamStatus: resp.Status,
			Detail: "no X-Auth-Token in session response", Err: bmc.ErrUpstream}
	}
	loc := resp.Location
	if loc == "" {
		var b struct {
			ID string `json:"@odata.id"`
		}
		_ = json.Unmarshal(resp.Body, &b)
		loc = b.ID
	}
	c.mu.Lock()
	c.session = &sessionState{token: tok, location: loc, gen: prevGen + 1, lastUsed: c.opts.Now()}
	if prevGen+1 <= prevGen { // overflow paranoia
		c.session.gen = 1
	}
	c.mu.Unlock()
	log.DebugContext(ctx, "redfish session created", "location", loc, "user", c.opts.Creds.User)

	// Learn SessionTimeout for proactive refresh (best effort).
	var ss struct {
		SessionTimeout *float64 `json:"SessionTimeout"`
	}
	ssPath := DefaultServiceRoot + "SessionService"
	if root, err := c.ServiceRoot(ctx); err == nil && root.SessionService.ID != "" {
		ssPath = root.SessionService.ID
	}
	if r, _, err := c.sendOnce(ctx, http.MethodGet, mustResolve(c, ssPath), nil, &reqConfig{op: "redfish.SessionService"}); err == nil && r.Status == http.StatusOK {
		if decodeLenient(r.Body, &ss, c.opts.Quirks) == nil && ss.SessionTimeout != nil && *ss.SessionTimeout > 0 {
			c.mu.Lock()
			if c.session != nil {
				c.session.timeout = time.Duration(*ss.SessionTimeout * float64(time.Second))
			}
			c.mu.Unlock()
		}
	}
	return nil
}

func mustResolve(c *Client, p string) string {
	u, err := c.resolve(p)
	if err != nil {
		return c.base.String() + p
	}
	return u
}

func (c *Client) deleteSession(ctx context.Context, s *sessionState) {
	if s == nil || s.location == "" {
		return
	}
	full, err := c.resolve(s.location)
	if err != nil {
		return
	}
	rc := &reqConfig{noAuth: true, op: "redfish.Logout", header: http.Header{"X-Auth-Token": {s.token}}}
	resp, _, err := c.sendOnce(ctx, http.MethodDelete, full, nil, rc)
	log := obs.FromContext(ctx, obs.SubRedfish)
	if err != nil {
		log.DebugContext(ctx, "redfish session delete failed", "location", s.location, "err", err)
		return
	}
	log.DebugContext(ctx, "redfish session deleted", "location", s.location, "status", resp.Status)
}

// Close logs out (DELETE of the session resource) in session mode. It is
// idempotent; the client must not be used afterwards.
func (c *Client) Close(ctx context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	c.mu.Lock()
	s := c.session
	c.session = nil
	c.closed = true
	c.mu.Unlock()
	c.deleteSession(ctx, s)
	return nil
}
