package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/obs"
)

// Default body limits.
const (
	DefaultMaxBodyBytes   = 32 << 20 // buffered response cap; beyond it the rest is streamed
	DefaultTraceBodyLimit = 64 << 10 // bytes of each body included in TRACE logs
)

// Tracer is an http.RoundTripper wrapper that
//   - buffers the response body (up to MaxBodyBytes) so retries, TRACE dumps
//     and HAR capture see the exact bytes,
//   - measures DNS/connect/TLS/send/wait/receive timings via httptrace,
//   - logs one DEBUG line per exchange and, when the transport subsystem is
//     at TRACE, the full request/response headers and bodies (redacted by
//     the obs handler unless --log-secrets),
//   - records the exchange in the HAR writer (--har-out).
//
// Endpoint uses a Tracer internally; it is exported for callers that want
// tracing on a plain *http.Client (e.g. bmcprobe one-shots).
type Tracer struct {
	// Base performs the request. Required.
	Base http.RoundTripper
	// Obs is used when the request context carries no Obs. May be nil.
	Obs *obs.Obs
	// HAR overrides Obs.HAR(). May be nil.
	HAR *obs.HARWriter
	// MaxBodyBytes caps buffering (default DefaultMaxBodyBytes).
	MaxBodyBytes int64
	// TraceBodyLimit caps bodies in TRACE logs (default DefaultTraceBodyLimit).
	TraceBodyLimit int
}

// attemptState is shared between Endpoint and Tracer through the context.
type attemptState struct {
	mu        sync.Mutex
	wrote     bool // request fully written to the wire
	streaming bool // body not (fully) buffered; slot must be held until Close
	attempt   int
	maxTries  int
	serverIP  string
}

type attemptKey struct{}

func stateFrom(ctx context.Context) *attemptState {
	s, _ := ctx.Value(attemptKey{}).(*attemptState)
	return s
}

func (s *attemptState) setWrote() {
	if s != nil {
		s.mu.Lock()
		s.wrote = true
		s.mu.Unlock()
	}
}

func (s *attemptState) didWrite() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.wrote
}

type timings struct {
	start, dnsStart, dnsDone, connStart, connDone, tlsStart, tlsDone, gotConn, wrote, firstByte, done time.Time
	reused                                                                                            bool
	remote                                                                                            string
}

func span(a, b time.Time) float64 {
	if a.IsZero() || b.IsZero() || b.Before(a) {
		return -1
	}
	return obs.HARMillis(b.Sub(a))
}

func (t *timings) har() obs.HARTimings {
	h := obs.HARTimings{
		Blocked: -1,
		DNS:     span(t.dnsStart, t.dnsDone),
		Connect: span(t.connStart, t.connDone),
		SSL:     span(t.tlsStart, t.tlsDone),
		Send:    0,
		Wait:    span(t.wrote, t.firstByte),
		Receive: span(t.firstByte, t.done),
	}
	if !t.gotConn.IsZero() {
		first := t.gotConn
		for _, x := range []time.Time{t.dnsStart, t.connStart} {
			if !x.IsZero() && x.Before(first) {
				first = x
			}
		}
		h.Blocked = span(t.start, first)
		h.Send = max(span(t.gotConn, t.wrote), 0)
	}
	if h.Wait < 0 {
		h.Wait = 0
	}
	if h.Receive < 0 {
		h.Receive = 0
	}
	// HAR: connect includes ssl
	if h.Connect >= 0 && h.SSL >= 0 && !t.tlsDone.Before(t.connDone) {
		h.Connect += h.SSL
	}
	return h
}

func (t *Tracer) logger(ctx context.Context) *slog.Logger {
	return obs.LoggerFrom(ctx, t.Obs, obs.SubTransport)
}

func (t *Tracer) har(ctx context.Context) *obs.HARWriter {
	if t.HAR != nil {
		return t.HAR
	}
	if o := t.Obs; o != nil && o.HAR() != nil {
		return o.HAR()
	}
	return obs.ObsFrom(ctx).HAR()
}

// RoundTrip implements http.RoundTripper.
func (t *Tracer) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	opts := OptionsFrom(ctx)
	st := stateFrom(ctx)
	log := t.logger(ctx)
	har := t.har(ctx)
	traceOn := log.Enabled(ctx, obs.LevelTrace)

	// capture request body bytes (Endpoint always sets GetBody)
	var reqBody []byte
	if req.Body != nil && req.Body != http.NoBody && (traceOn || har != nil) {
		if req.GetBody != nil {
			if rc, err := req.GetBody(); err == nil {
				reqBody, _ = io.ReadAll(rc)
				_ = rc.Close()
			}
		} else {
			b, err := io.ReadAll(req.Body)
			_ = req.Body.Close()
			if err != nil {
				return nil, err
			}
			reqBody = b
			req = req.Clone(ctx)
			req.Body = io.NopCloser(bytes.NewReader(b))
			req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(b)), nil }
		}
	}

	tm := &timings{start: time.Now()}
	var tmu sync.Mutex
	set := func(p *time.Time) { tmu.Lock(); *p = time.Now(); tmu.Unlock() }
	ct := &httptrace.ClientTrace{
		DNSStart:     func(httptrace.DNSStartInfo) { set(&tm.dnsStart) },
		DNSDone:      func(httptrace.DNSDoneInfo) { set(&tm.dnsDone) },
		ConnectStart: func(string, string) { set(&tm.connStart) },
		ConnectDone:  func(string, string, error) { set(&tm.connDone) },
		TLSHandshakeStart: func() {
			set(&tm.tlsStart)
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) { set(&tm.tlsDone) },
		GotConn: func(i httptrace.GotConnInfo) {
			tmu.Lock()
			tm.gotConn = time.Now()
			tm.reused = i.Reused
			if i.Conn != nil {
				tm.remote = i.Conn.RemoteAddr().String()
			}
			tmu.Unlock()
		},
		WroteRequest: func(i httptrace.WroteRequestInfo) {
			set(&tm.wrote)
			if i.Err == nil {
				st.setWrote()
			}
		},
		GotFirstResponseByte: func() { set(&tm.firstByte) },
	}
	treq := req.WithContext(httptrace.WithClientTrace(ctx, ct))

	resp, err := t.Base.RoundTrip(treq)

	var respBody []byte
	truncated := false
	if err == nil && resp.Body != nil {
		if opts.Stream {
			if st != nil {
				st.mu.Lock()
				st.streaming = true
				st.mu.Unlock()
			}
		} else {
			limit := t.MaxBodyBytes
			if limit <= 0 {
				limit = DefaultMaxBodyBytes
			}
			b, rerr := io.ReadAll(io.LimitReader(resp.Body, limit))
			respBody = b
			switch {
			case rerr != nil:
				_ = resp.Body.Close()
				err = fmt.Errorf("transport: reading response body: %w", rerr)
				resp = nil
			case int64(len(b)) == limit:
				// possibly more: hand the rest through as a stream
				truncated = true
				rest := resp.Body
				resp.Body = struct {
					io.Reader
					io.Closer
				}{io.MultiReader(bytes.NewReader(b), rest), rest}
				if st != nil {
					st.mu.Lock()
					st.streaming = true
					st.mu.Unlock()
				}
			default:
				_ = resp.Body.Close()
				resp.Body = io.NopCloser(bytes.NewReader(b))
				resp.ContentLength = int64(len(b))
			}
		}
	}
	tmu.Lock()
	tm.done = time.Now()
	ht := tm.har()
	dur := tm.done.Sub(tm.start)
	reused, remote := tm.reused, tm.remote
	tmu.Unlock()
	if st != nil {
		st.mu.Lock()
		st.serverIP = remote
		st.mu.Unlock()
	}

	attempt := ""
	if st != nil && st.maxTries > 1 {
		attempt = fmt.Sprintf("%d/%d", st.attempt, st.maxTries)
	}
	status := 0
	if resp != nil {
		status = resp.StatusCode
	}
	if log.Enabled(ctx, slog.LevelDebug) {
		args := []any{"method", req.Method, "url", req.URL.String(), "status", status, "dur", dur.Round(time.Microsecond), "reused", reused}
		if attempt != "" {
			args = append(args, "attempt", attempt)
		}
		if opts.Op != "" {
			args = append(args, "op", opts.Op)
		}
		if err != nil {
			args = append(args, "err", err)
		}
		log.DebugContext(ctx, "southbound", args...)
	}
	if traceOn {
		args := []any{"method", req.Method, "url", req.URL.String(), "req_headers", req.Header, "req_body", t.clip(reqBody),
			"status", status, "dur", dur, "timings", fmt.Sprintf("%+v", ht)}
		if resp != nil {
			body := t.clip(respBody)
			if opts.Stream {
				body = "[streamed]"
			} else if truncated {
				body += " [truncated]"
			}
			args = append(args, "resp_proto", resp.Proto, "resp_headers", resp.Header, "resp_body", body)
		}
		if err != nil {
			args = append(args, "err", err)
		}
		log.Log(ctx, obs.LevelTrace, "southbound dump", args...)
	}
	if har != nil {
		comment := ""
		if id := obs.CorrelationID(ctx); id != "" {
			comment = "corr_id=" + id
		}
		if attempt != "" {
			comment += " attempt=" + attempt
		}
		if opts.Op != "" {
			comment += " op=" + opts.Op
		}
		if opts.Stream {
			comment += " body=streamed"
		}
		_ = har.Record(obs.HARRecord{Host: req.URL.Host, Request: req, ReqBody: reqBody, Response: resp, RespBody: respBody,
			Started: tm.start, Duration: dur, Timings: ht, ServerIP: hostOnly(remote), Err: err, Comment: comment})
	}
	return resp, err
}

func (t *Tracer) clip(b []byte) string {
	lim := t.TraceBodyLimit
	if lim <= 0 {
		lim = DefaultTraceBodyLimit
	}
	if len(b) > lim {
		return string(b[:lim]) + fmt.Sprintf("...[%d bytes total]", len(b))
	}
	return string(b)
}

func hostOnly(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			h := addr[:i]
			if len(h) > 1 && h[0] == '[' {
				h = h[1 : len(h)-1]
			}
			return h
		}
	}
	return addr
}
