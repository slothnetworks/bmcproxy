package obs

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

// EnvDebug is the environment variable merged into Options.Debug.
const EnvDebug = "BMCPROXY_DEBUG"

// Log attribute keys added automatically.
const (
	KeySubsystem     = "subsys"
	KeyRequestID     = "req_id"
	KeyCorrelationID = "corr_id"
)

// Options configures an Obs. The zero value logs text at INFO to stderr with
// redaction on.
type Options struct {
	// Level is the default level (--log-level). Default INFO.
	Level slog.Level
	// Debug is the --debug CSV (see Levels.ApplyDebug). Merged with
	// BMCPROXY_DEBUG by ApplyEnv.
	Debug string
	// LogSecrets disables redaction in logs and HAR output (--log-secrets).
	LogSecrets bool
	// TraceHTTP raises the transport subsystem to TRACE (--trace-http).
	TraceHTTP bool
	// HAROut, when non-empty, is a directory that receives one HAR file per
	// BMC host (--har-out).
	HAROut string
	// JSON selects the JSON handler instead of text (--log-format=json).
	JSON bool
	// AddSource adds file:line to records.
	AddSource bool
	// Writer receives log output. Default os.Stderr.
	Writer io.Writer
}

// RegisterFlags binds the standard instrumentation flags on fs:
// --log-level, --debug, --log-secrets, --trace-http, --har-out, --log-format.
// Both cmd/bmcproxy and cmd/bmcprobe use this so the flags match.
func (o *Options) RegisterFlags(fs *flag.FlagSet) {
	fs.Func("log-level", "default log level: trace|debug|info|warn|error (default info)", func(s string) error {
		l, err := ParseLevel(s)
		if err != nil {
			return err
		}
		o.Level = l
		return nil
	})
	fs.StringVar(&o.Debug, "debug", o.Debug, "comma-separated subsystems to raise to DEBUG (name, name=trace, or all); also "+EnvDebug+". Subsystems: "+strings.Join(KnownSubsystems(), ","))
	fs.BoolVar(&o.LogSecrets, "log-secrets", o.LogSecrets, "DANGEROUS: disable redaction of credentials/cookies/tokens in logs and HAR output")
	fs.BoolVar(&o.TraceHTTP, "trace-http", o.TraceHTTP, "dump every southbound HTTP request/response (transport=trace)")
	fs.StringVar(&o.HAROut, "har-out", o.HAROut, "directory to write a HAR file of southbound traffic per BMC host")
	fs.Func("log-format", "log format: text|json (default text)", func(s string) error {
		switch strings.ToLower(s) {
		case "text", "":
			o.JSON = false
		case "json":
			o.JSON = true
		default:
			return fmt.Errorf("invalid log format %q", s)
		}
		return nil
	})
}

// ApplyEnv merges BMCPROXY_DEBUG into o.Debug.
func (o *Options) ApplyEnv() {
	if v := strings.TrimSpace(os.Getenv(EnvDebug)); v != "" {
		if o.Debug == "" {
			o.Debug = v
		} else {
			o.Debug += "," + v
		}
	}
}

// Obs is the root of instrumentation: it owns the level registry, the
// (redacting) base handler and the optional HAR writer. Create one at
// startup with New and install it in contexts with WithObs.
type Obs struct {
	levels     *Levels
	base       slog.Handler // emits everything >= LevelTrace; gating is per subsystem
	logSecrets bool
	har        *HARWriter
}

// New builds an Obs from opts. It returns an error for a bad --debug value or
// an unwritable --har-out directory. When LogSecrets is set a WARN line is
// logged immediately.
func New(opts Options) (*Obs, error) {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	ho := &slog.HandlerOptions{Level: LevelTrace, AddSource: opts.AddSource, ReplaceAttr: replaceLevel}
	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(w, ho)
	} else {
		h = slog.NewTextHandler(w, ho)
	}
	o := &Obs{
		levels:     NewLevels(opts.Level),
		base:       NewRedactingHandler(h, !opts.LogSecrets),
		logSecrets: opts.LogSecrets,
	}
	if err := o.levels.ApplyDebug(opts.Debug); err != nil {
		return nil, err
	}
	if opts.TraceHTTP {
		o.levels.Set(SubTransport, LevelTrace)
	}
	if opts.HAROut != "" {
		hw, err := NewHARWriter(opts.HAROut, !opts.LogSecrets)
		if err != nil {
			return nil, err
		}
		o.har = hw
	}
	if opts.LogSecrets {
		o.Logger("").Warn("--log-secrets is set: credentials, cookies and tokens will be written to logs and HAR files UNREDACTED")
	}
	return o, nil
}

// NewForHandler builds an Obs on a caller-supplied handler (tests, custom
// sinks). The handler should accept LevelTrace; per-subsystem gating is done
// by Obs. Redaction is applied unless logSecrets.
func NewForHandler(h slog.Handler, def slog.Level, logSecrets bool) *Obs {
	return &Obs{levels: NewLevels(def), base: NewRedactingHandler(h, !logSecrets), logSecrets: logSecrets}
}

// Discard returns an Obs that drops everything.
func Discard() *Obs {
	return NewForHandler(slog.DiscardHandler, slog.LevelError+100, false)
}

func replaceLevel(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.LevelKey && len(a.Value.String()) > 0 {
		if l, ok := a.Value.Any().(slog.Level); ok && l == LevelTrace {
			return slog.String(slog.LevelKey, "TRACE")
		}
	}
	return a
}

// Levels returns the level registry (for runtime changes and /debug/levels).
func (o *Obs) Levels() *Levels { return o.levels }

// Redacting reports whether secrets are redacted (i.e. !LogSecrets).
func (o *Obs) Redacting() bool { return !o.logSecrets }

// HAR returns the HAR writer, or nil when --har-out is not set.
func (o *Obs) HAR() *HARWriter { return o.har }

// SetHAR installs (or with nil, removes) a HAR writer.
func (o *Obs) SetHAR(h *HARWriter) { o.har = h }

// Close flushes the HAR writer, if any.
func (o *Obs) Close() error {
	if o.har != nil {
		return o.har.Close()
	}
	return nil
}

// Logger returns the named subsystem logger. Records are gated by the
// subsystem's level, carry subsys=<sub>, and, when logged with a *Context
// method, the request and correlation ids from that context. sub "" is the
// root logger (default level, no subsys attr).
func (o *Obs) Logger(sub string) *slog.Logger {
	var h slog.Handler = o.base
	if sub != "" {
		h = h.WithAttrs([]slog.Attr{slog.String(KeySubsystem, sub)})
	}
	return slog.New(&ctxHandler{next: h, levels: o.levels, sub: sub})
}

// Enabled reports whether sub logs at lv (cheap check before building
// expensive TRACE dumps).
func (o *Obs) Enabled(sub string, lv slog.Level) bool { return o.levels.Enabled(sub, lv) }

// ctxHandler gates by subsystem level and appends ids from the context.
type ctxHandler struct {
	next      slog.Handler
	levels    *Levels
	sub       string
	reqBaked  bool
	corrBaked bool
}

func (h *ctxHandler) Enabled(_ context.Context, l slog.Level) bool {
	return h.levels.Enabled(h.sub, l)
}

func (h *ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if ctx != nil {
		if !h.reqBaked {
			if id := RequestID(ctx); id != "" {
				r.AddAttrs(slog.String(KeyRequestID, id))
			}
		}
		if !h.corrBaked {
			if id := CorrelationID(ctx); id != "" {
				r.AddAttrs(slog.String(KeyCorrelationID, id))
			}
		}
		if as := ctxAttrs(ctx); len(as) > 0 {
			r.AddAttrs(as...)
		}
	}
	return h.next.Handle(ctx, r)
}

func (h *ctxHandler) WithAttrs(as []slog.Attr) slog.Handler {
	c := *h
	for _, a := range as {
		switch a.Key {
		case KeyRequestID:
			c.reqBaked = true
		case KeyCorrelationID:
			c.corrBaked = true
		}
	}
	c.next = h.next.WithAttrs(as)
	return &c
}

func (h *ctxHandler) WithGroup(name string) slog.Handler {
	c := *h
	c.next = h.next.WithGroup(name)
	return &c
}

// ---- process default ----

var defaultObs atomic.Pointer[Obs]

func init() {
	o, _ := New(Options{})
	defaultObs.Store(o)
}

// Default returns the process-wide fallback Obs used when a context carries
// none. Initially INFO text to stderr with redaction on.
func Default() *Obs { return defaultObs.Load() }

// SetDefault replaces the process-wide fallback Obs (call once at startup).
func SetDefault(o *Obs) {
	if o != nil {
		defaultObs.Store(o)
	}
}
