package transport

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// RetryPolicy bounds retries of transient southbound failures.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts including the first.
	// Default 3; 1 disables retries.
	MaxAttempts int
	// BaseDelay is the backoff before the 2nd attempt; it doubles per
	// attempt. Default 250ms.
	BaseDelay time.Duration
	// MaxDelay caps one backoff (and a server Retry-After). Default 5s.
	MaxDelay time.Duration
}

func (p RetryPolicy) withDefaults() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.BaseDelay <= 0 {
		p.BaseDelay = 250 * time.Millisecond
	}
	if p.MaxDelay <= 0 {
		p.MaxDelay = 5 * time.Second
	}
	return p
}

// Backoff returns the jittered delay before attempt n+1 (n >= 1 is the
// attempt that just failed): uniformly in [d/2, d] where
// d = min(MaxDelay, BaseDelay*2^(n-1)).
func (p RetryPolicy) Backoff(n int) time.Duration {
	p = p.withDefaults()
	d := p.BaseDelay
	for i := 1; i < n && d < p.MaxDelay; i++ {
		d *= 2
	}
	if d > p.MaxDelay {
		d = p.MaxDelay
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(d-half)+1))
}

// retryAfter parses a Retry-After header in seconds (dates ignored).
func retryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	if s := resp.Header.Get("Retry-After"); s != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return 0
}

// Evasive403Func decides whether a 403 response is a connection-cap
// rejection (lighttpd mod_evasive on B650-class firmware) rather than a real
// authorisation failure. body is the buffered response body (may be
// truncated or nil).
type Evasive403Func func(resp *http.Response, body []byte) bool

// DefaultIsEvasive403 treats a 403 as evasive when it is not a JSON API
// error: lighttpd's mod_evasive answers with its stock HTML/empty page,
// whereas Redfish and AMI /api privilege errors are JSON.
func DefaultIsEvasive403(resp *http.Response, body []byte) bool {
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		return false
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "json") {
		return false
	}
	b := strings.TrimSpace(string(body))
	return b == "" || strings.HasPrefix(b, "<") || strings.Contains(strings.ToLower(b), "403")
}

// sleepCtx waits d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isTimeout reports net/context timeouts.
func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ETIMEDOUT) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// IsTransientNetError reports connection resets, refused/unreachable,
// unexpected EOFs, broken pipes, timeouts and TLS handshake failures — the
// network errors the retry loop treats as transient.
func IsTransientNetError(err error) bool {
	if err == nil {
		return false
	}
	if isTimeout(err) || IsTLSHandshakeError(err) {
		return true
	}
	for _, e := range []error{syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.ECONNABORTED, syscall.EPIPE,
		syscall.EHOSTUNREACH, syscall.ENETUNREACH, io.EOF, io.ErrUnexpectedEOF, net.ErrClosed} {
		if errors.Is(err, e) {
			return true
		}
	}
	var oe *net.OpError
	if errors.As(err, &oe) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "connection reset") || strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "server closed idle connection") || strings.Contains(s, "EOF")
}
