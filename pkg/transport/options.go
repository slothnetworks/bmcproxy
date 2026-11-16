package transport

import (
	"context"
	"net/http"
	"time"
)

// RequestOptions tune one southbound call. Attach them to the request's
// context with WithOptions (or the WithIdempotent shorthand) before calling
// Endpoint.Do or using Endpoint.HTTPClient.
type RequestOptions struct {
	// Idempotent marks a non-GET request as safe to retry on transient
	// failures (503, evasive 403, reset after send, attempt timeout). Without
	// it only GET/HEAD/OPTIONS are retried; a write is retried only if the
	// failure happened before the request was written (dial/TLS errors).
	Idempotent bool
	// NoRetry disables retries for this call entirely.
	NoRetry bool
	// Priority overrides the automatic (method-based) queue priority.
	Priority Priority
	// Stream returns the response body unbuffered: the limiter/pool slot is
	// held until Body.Close, responses are not retried on status, and the
	// HAR/TRACE body is omitted. Use for large downloads.
	Stream bool
	// Timeout is the per-attempt timeout (0 = Config.AttemptTimeout,
	// negative = none beyond the caller's context).
	Timeout time.Duration
	// RawStatus returns a final 503 / evasive-403 response as-is instead of
	// converting it to a bmc.ErrBusy error.
	RawStatus bool
	// Op is a logical operation name for logs, stats and HAR comments
	// (e.g. "amimegarac.Login").
	Op string
}

type optsKey struct{}

// WithOptions returns ctx carrying o.
func WithOptions(ctx context.Context, o RequestOptions) context.Context {
	return context.WithValue(ctx, optsKey{}, o)
}

// OptionsFrom returns the options in ctx (zero value if none).
func OptionsFrom(ctx context.Context) RequestOptions {
	o, _ := ctx.Value(optsKey{}).(RequestOptions)
	return o
}

// WithIdempotent marks calls made with ctx as safe to retry.
func WithIdempotent(ctx context.Context) context.Context {
	o := OptionsFrom(ctx)
	o.Idempotent = true
	return WithOptions(ctx, o)
}

// WithOp sets RequestOptions.Op on ctx.
func WithOp(ctx context.Context, op string) context.Context {
	o := OptionsFrom(ctx)
	o.Op = op
	return WithOptions(ctx, o)
}

// IsSafeMethod reports whether m is GET, HEAD or OPTIONS (retried by
// default, read priority).
func IsSafeMethod(m string) bool {
	switch m {
	case "", http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}
