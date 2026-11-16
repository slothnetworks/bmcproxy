package obs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
)

type ctxKey int

const (
	keyObs ctxKey = iota
	keyLogger
	keyRequestID
	keyCorrelationID
	keyAttrs
)

// WithObs returns a context carrying o. FromContext derives subsystem loggers
// from it. The server installs its Obs on every request context.
func WithObs(ctx context.Context, o *Obs) context.Context {
	return context.WithValue(ctx, keyObs, o)
}

// ObsFrom returns the Obs in ctx, or Default() if none.
func ObsFrom(ctx context.Context) *Obs {
	if ctx != nil {
		if o, ok := ctx.Value(keyObs).(*Obs); ok && o != nil {
			return o
		}
	}
	return Default()
}

// WithLogger returns a context whose FromContext calls return l (plus a
// subsys attr) instead of an Obs-derived logger. Per-subsystem level gating
// then does not apply; this is intended for tests and embedding.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, keyLogger, l)
}

// FromContext returns the logger for subsystem sub, derived from the Obs in
// ctx (or the logger set by WithLogger, or Default()). The returned logger
// has the context's request id, correlation id and WithAttrs attrs baked in,
// so even non-Context log calls carry them; ids added to a later context are
// still picked up when logging with the *Context methods.
func FromContext(ctx context.Context, sub string) *slog.Logger {
	if ctx == nil {
		ctx = context.Background()
	}
	var l *slog.Logger
	if ol, ok := ctx.Value(keyLogger).(*slog.Logger); ok && ol != nil {
		l = ol
		if sub != "" {
			l = l.With(KeySubsystem, sub)
		}
	} else {
		l = ObsFrom(ctx).Logger(sub)
	}
	return bake(ctx, l)
}

// LoggerFrom is like FromContext but uses fallback (if non-nil) instead of
// Default() when ctx carries no Obs.
func LoggerFrom(ctx context.Context, fallback *Obs, sub string) *slog.Logger {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Value(keyObs) == nil && ctx.Value(keyLogger) == nil && fallback != nil {
		return bake(ctx, fallback.Logger(sub))
	}
	return FromContext(ctx, sub)
}

func bake(ctx context.Context, l *slog.Logger) *slog.Logger {
	var as []any
	if id := RequestID(ctx); id != "" {
		as = append(as, slog.String(KeyRequestID, id))
	}
	if id := CorrelationID(ctx); id != "" {
		as = append(as, slog.String(KeyCorrelationID, id))
	}
	if len(as) == 0 {
		return l
	}
	return l.With(as...)
}

// NewID returns a random 16-hex-character id suitable for request and
// correlation ids.
func NewID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// WithRequestID stores the northbound request id (echoed as X-Request-Id).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyRequestID, id)
}

// RequestID returns the request id in ctx, or "".
func RequestID(ctx context.Context) string {
	s, _ := ctx.Value(keyRequestID).(string)
	return s
}

// WithCorrelationID stores the correlation id of one southbound BMC call.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, keyCorrelationID, id)
}

// CorrelationID returns the correlation id in ctx, or "".
func CorrelationID(ctx context.Context) string {
	s, _ := ctx.Value(keyCorrelationID).(string)
	return s
}

// EnsureCorrelationID returns ctx with a correlation id, creating one if ctx
// has none, plus the id.
func EnsureCorrelationID(ctx context.Context) (context.Context, string) {
	if id := CorrelationID(ctx); id != "" {
		return ctx, id
	}
	id := NewID()
	return WithCorrelationID(ctx, id), id
}

// WithAttrs returns a context whose log records (from any subsystem logger,
// when logged with a *Context method) also carry the given attrs, e.g.
// obs.WithAttrs(ctx, "bmc", host, "vendor", "asrock"). Arguments follow
// slog's key/value convention.
func WithAttrs(ctx context.Context, args ...any) context.Context {
	rec := slog.Record{}
	rec.Add(args...)
	add := make([]slog.Attr, 0, rec.NumAttrs())
	rec.Attrs(func(a slog.Attr) bool { add = append(add, a); return true })
	prev := ctxAttrs(ctx)
	all := make([]slog.Attr, 0, len(prev)+len(add))
	all = append(all, prev...)
	all = append(all, add...)
	return context.WithValue(ctx, keyAttrs, all)
}

func ctxAttrs(ctx context.Context) []slog.Attr {
	as, _ := ctx.Value(keyAttrs).([]slog.Attr)
	return as
}
