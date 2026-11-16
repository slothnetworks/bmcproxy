// Package obs is the shared instrumentation layer for bmcproxy: named
// subsystem loggers built on log/slog, a per-subsystem level registry driven
// by --log-level / --debug / BMCPROXY_DEBUG, request and correlation ids
// carried in context.Context, a secret-redacting slog.Handler, a HAR 1.2
// writer for southbound traffic, and helpers to mount pprof/expvar.
//
// No package in bmcproxy uses the global slog logger. Code obtains a logger
// with FromContext(ctx, obs.SubTransport) (or (*Obs).Logger) and logs with the
// *Context methods so request/correlation ids are attached.
package obs

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

// LevelTrace is below slog.LevelDebug and is used for full wire dumps
// (headers and bodies). It renders as "TRACE".
const LevelTrace = slog.Level(-8)

// Well-known subsystem names. Each package owns exactly one; the --debug flag
// maps 1:1 to these. Drivers use "driver.<vendor>".
const (
	SubTransport     = "transport"
	SubAMI           = "amimegarac"
	SubRedfish       = "redfish"
	SubIPMI          = "ipmi"
	SubSessions      = "sessions"
	SubNorth         = "north" // northbound HTTP server / API handlers
	SubCache         = "cache"
	SubHealth        = "health"
	SubMetrics       = "metrics"
	SubCore          = "core" // registry / resolution
	SubTestsim       = "testsim"
	SubDriverASRock  = "driver.asrock"
	SubDriverRedfish = "driver.redfish"
)

// KnownSubsystems lists the built-in subsystem names (for --help and
// /debug/levels). Packages may use other names; they are created on demand.
func KnownSubsystems() []string {
	return []string{SubTransport, SubAMI, SubRedfish, SubIPMI, SubSessions, SubNorth,
		SubCache, SubHealth, SubMetrics, SubCore, SubTestsim, SubDriverASRock, SubDriverRedfish}
}

// ParseLevel parses trace|debug|info|warn|warning|error (case-insensitive) or
// any form accepted by slog.Level.UnmarshalText (e.g. "DEBUG-2").
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return LevelTrace, nil
	case "debug":
		return slog.LevelDebug, nil
	case "", "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	}
	var l slog.Level
	if err := l.UnmarshalText([]byte(s)); err != nil {
		return 0, fmt.Errorf("obs: invalid log level %q", s)
	}
	return l, nil
}

// LevelString renders a level, using "TRACE" for LevelTrace.
func LevelString(l slog.Level) string {
	if l == LevelTrace {
		return "TRACE"
	}
	return l.String()
}

// Levels is a concurrency-safe registry of per-subsystem log levels. A
// subsystem without an explicit level uses the default. Levels may be changed
// at runtime; loggers created earlier observe the change immediately.
type Levels struct {
	mu   sync.RWMutex
	def  slog.Level
	subs map[string]slog.Level
}

// NewLevels returns a registry whose default level is def.
func NewLevels(def slog.Level) *Levels {
	return &Levels{def: def, subs: map[string]slog.Level{}}
}

// SetDefault sets the level used by subsystems without an override.
func (l *Levels) SetDefault(lv slog.Level) {
	l.mu.Lock()
	l.def = lv
	l.mu.Unlock()
}

// Default returns the default level.
func (l *Levels) Default() slog.Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.def
}

// Set overrides the level for one subsystem.
func (l *Levels) Set(sub string, lv slog.Level) {
	l.mu.Lock()
	l.subs[sub] = lv
	l.mu.Unlock()
}

// Reset removes a subsystem override.
func (l *Levels) Reset(sub string) {
	l.mu.Lock()
	delete(l.subs, sub)
	l.mu.Unlock()
}

// Level returns the effective level of sub. A subsystem "a.b" without its own
// override inherits the override of "a" if any ("driver" covers
// "driver.asrock"). An empty sub returns the default.
func (l *Levels) Level(sub string) slog.Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for s := sub; s != ""; {
		if lv, ok := l.subs[s]; ok {
			return lv
		}
		i := strings.LastIndexByte(s, '.')
		if i < 0 {
			break
		}
		s = s[:i]
	}
	return l.def
}

// Enabled reports whether a record at lv would be emitted for sub.
func (l *Levels) Enabled(sub string, lv slog.Level) bool { return lv >= l.Level(sub) }

// Leveler returns a dynamic slog.Leveler for sub.
func (l *Levels) Leveler(sub string) slog.Leveler { return subLeveler{l, sub} }

type subLeveler struct {
	l   *Levels
	sub string
}

func (s subLeveler) Level() slog.Level { return s.l.Level(s.sub) }

// Snapshot returns the default ("*") and every override, rendered as strings.
func (l *Levels) Snapshot() map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	m := map[string]string{"*": LevelString(l.def)}
	for k, v := range l.subs {
		m[k] = LevelString(v)
	}
	return m
}

// ApplyDebug parses a --debug / BMCPROXY_DEBUG value and raises the named
// subsystems. Syntax: comma-separated items, each "name" (DEBUG),
// "name=level" or "name:level". The name "all" sets the default level
// (affecting every subsystem without its own override). Levels are only ever
// lowered (made more verbose) relative to the current effective level, so
// "--log-level=trace --debug=redfish" leaves redfish at TRACE.
func (l *Levels) ApplyDebug(csv string) error {
	for _, item := range strings.Split(csv, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		name, lvs, found := strings.Cut(item, "=")
		if !found {
			name, lvs, found = strings.Cut(item, ":")
		}
		lv := slog.LevelDebug
		if found {
			var err error
			if lv, err = ParseLevel(lvs); err != nil {
				return fmt.Errorf("obs: --debug item %q: %w", item, err)
			}
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return fmt.Errorf("obs: --debug item %q has no subsystem", item)
		}
		if name == "all" || name == "*" {
			if lv < l.Default() {
				l.SetDefault(lv)
			}
			continue
		}
		if lv < l.Level(name) {
			l.Set(name, lv)
		}
	}
	return nil
}

// SortedSubsystems returns the names with explicit overrides, sorted.
func (l *Levels) SortedSubsystems() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]string, 0, len(l.subs))
	for k := range l.subs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
