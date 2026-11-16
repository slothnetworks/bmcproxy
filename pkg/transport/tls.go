package transport

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http/httptrace"
	"sync"
	"time"
)

// TLSProfile selects the TLS configuration used towards a BMC. Every profile
// sets InsecureSkipVerify: BMCs ship the shared AMI firmware certificate
// (private key public in firmware, often expired), so verification gives no
// identity assurance; network isolation is the control (see PLAN).
type TLSProfile int

const (
	// ProfileAuto tries ProfileModern first and falls back to ProfileLegacy
	// when the handshake fails, remembering the result per host. Default.
	ProfileAuto TLSProfile = iota
	// ProfileModern uses Go's defaults (TLS 1.2+, ECDHE/AEAD). B650D4U.
	ProfileModern
	// ProfileLegacy allows TLS 1.0–1.2 and re-enables RSA key-exchange and
	// CBC/3DES suites that Go no longer offers by default. X470D4U, whose
	// BMC only offers RSA key exchange.
	ProfileLegacy
)

func (p TLSProfile) String() string {
	switch p {
	case ProfileAuto:
		return "auto"
	case ProfileModern:
		return "modern"
	case ProfileLegacy:
		return "legacy"
	}
	return fmt.Sprintf("profile(%d)", int(p))
}

// ParseTLSProfile parses "auto", "modern" or "legacy".
func ParseTLSProfile(s string) (TLSProfile, error) {
	switch s {
	case "", "auto":
		return ProfileAuto, nil
	case "modern":
		return ProfileModern, nil
	case "legacy":
		return ProfileLegacy, nil
	}
	return 0, fmt.Errorf("transport: unknown TLS profile %q", s)
}

// legacySuites: modern ECDHE suites first, then the RSA key-exchange suites
// the X470 needs (Go disables these by default; listing them explicitly
// re-enables them).
var legacySuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
}

// TLSConfig returns a fresh *tls.Config for p (ProfileAuto returns the
// modern config; the fallback is handled by the dialer).
func TLSConfig(p TLSProfile) *tls.Config {
	c := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // deliberate, see TLSProfile
	if p == ProfileLegacy {
		c.MinVersion = tls.VersionTLS10
		c.MaxVersion = tls.VersionTLS12
		c.CipherSuites = append([]uint16(nil), legacySuites...)
	}
	return c
}

// TLSInfo describes the negotiated TLS of the last new connection to a host.
type TLSInfo struct {
	Profile     TLSProfile `json:"profile"` // profile that succeeded
	Version     string     `json:"version"`
	CipherSuite string     `json:"cipherSuite"`
	At          time.Time  `json:"at"`
}

// tlsDialer dials TLS with per-host profile selection and Auto fallback.
type tlsDialer struct {
	dialer           net.Dialer
	handshakeTimeout time.Duration
	profileFor       func(addr string) TLSProfile
	onConn           func(addr string, info TLSInfo)

	mu      sync.Mutex
	learned map[string]TLSProfile
}

func (d *tlsDialer) learnedProfile(addr string) (TLSProfile, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, ok := d.learned[addr]
	return p, ok
}

func (d *tlsDialer) learn(addr string, p TLSProfile) {
	d.mu.Lock()
	if d.learned == nil {
		d.learned = map[string]TLSProfile{}
	}
	d.learned[addr] = p
	d.mu.Unlock()
}

func (d *tlsDialer) forget(addr string) {
	d.mu.Lock()
	delete(d.learned, addr)
	d.mu.Unlock()
}

// errHandshake marks a failure after TCP connect (eligible for fallback).
type errHandshake struct{ err error }

func (e *errHandshake) Error() string { return "tls handshake: " + e.err.Error() }
func (e *errHandshake) Unwrap() error { return e.err }

// IsTLSHandshakeError reports whether err came from a failed TLS handshake
// (as opposed to TCP connect or HTTP).
func IsTLSHandshakeError(err error) bool {
	var h *errHandshake
	return errors.As(err, &h)
}

func (d *tlsDialer) DialTLSContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	var order []TLSProfile
	switch p := d.profileFor(addr); p {
	case ProfileAuto:
		first := ProfileModern
		if lp, ok := d.learnedProfile(addr); ok {
			first = lp
		}
		second := ProfileLegacy
		if first == ProfileLegacy {
			second = ProfileModern
		}
		order = []TLSProfile{first, second}
	default:
		order = []TLSProfile{p}
	}
	var lastErr error
	for i, p := range order {
		conn, err := d.dialOne(ctx, network, addr, host, p)
		if err == nil {
			if len(order) > 1 {
				d.learn(addr, p)
			}
			return conn, nil
		}
		lastErr = err
		if !IsTLSHandshakeError(err) || ctx.Err() != nil {
			return nil, err
		}
		if i == 0 && len(order) > 1 {
			d.forget(addr)
		}
	}
	return nil, lastErr
}

func (d *tlsDialer) dialOne(ctx context.Context, network, addr, host string, p TLSProfile) (net.Conn, error) {
	raw, err := d.dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}
	cfg := TLSConfig(p)
	if net.ParseIP(host) == nil {
		cfg.ServerName = host
	}
	tc := tls.Client(raw, cfg)
	trace := httptrace.ContextClientTrace(ctx)
	if trace != nil && trace.TLSHandshakeStart != nil {
		trace.TLSHandshakeStart()
	}
	hctx := ctx
	if d.handshakeTimeout > 0 {
		var cancel context.CancelFunc
		hctx, cancel = context.WithTimeout(ctx, d.handshakeTimeout)
		defer cancel()
	}
	herr := tc.HandshakeContext(hctx)
	st := tc.ConnectionState()
	if trace != nil && trace.TLSHandshakeDone != nil {
		trace.TLSHandshakeDone(st, herr)
	}
	if herr != nil {
		_ = raw.Close()
		return nil, &errHandshake{err: fmt.Errorf("profile %s: %w", p, herr)}
	}
	if d.onConn != nil {
		d.onConn(addr, TLSInfo{Profile: p, Version: tls.VersionName(st.Version), CipherSuite: tls.CipherSuiteName(st.CipherSuite), At: time.Now()})
	}
	return tc, nil
}
