// Package servertest provides fake BMC drivers and a full northbound server
// harness (httptest) for tests. Nothing here contacts a real BMC.
package servertest

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
)

// Driver is a configurable fake bmc.Driver.
type Driver struct {
	NameV  string
	Prio   int
	Score  int               // Detect score (default ScoreCertain)
	Creds  map[string]string // user -> pass accepted by Open (nil = any)
	Routes []bmc.Route
	// Minimal makes Open return a session implementing only Identity.
	Minimal bool
	// OpenDelay slows Open (singleflight tests).
	OpenDelay time.Duration
	// Configure, if set, mutates each new session.
	Configure func(*Session)

	Opens   atomic.Int32
	Detects atomic.Int32

	mu       sync.Mutex
	sessions []*Session
}

func (d *Driver) Name() string  { return d.NameV }
func (d *Driver) Priority() int { return d.Prio }

func (d *Driver) Detect(ctx context.Context, t bmc.Target) (bmc.Match, error) {
	d.Detects.Add(1)
	sc := d.Score
	if sc == 0 {
		sc = bmc.ScoreCertain
	}
	if sc < 0 {
		return bmc.Match{}, nil
	}
	return bmc.Match{Score: sc, Vendor: d.NameV, Manufacturer: "FakeCo", Board: "FAKE1", Firmware: "1.0", Backends: []string{"redfish"}}, nil
}

func (d *Driver) Open(ctx context.Context, t bmc.Target, c bmc.Creds) (bmc.Session, error) {
	d.Opens.Add(1)
	if d.OpenDelay > 0 {
		time.Sleep(d.OpenDelay)
	}
	if d.Creds != nil {
		if p, ok := d.Creds[c.User]; !ok || p != c.Pass {
			return nil, &bmc.OpError{Pkg: "fake", Op: "Open", Backend: "redfish", UpstreamStatus: 401, Detail: "AccessDenied", Err: bmc.ErrAuth}
		}
	}
	s := &Session{DriverName: d.NameV, Target: t.Key(), User: c.User, State: bmc.PowerOn, ConvergeAfter: 1, LED: bmc.LEDOff, calls: map[string]int{}}
	if d.Configure != nil {
		d.Configure(s)
	}
	d.mu.Lock()
	d.sessions = append(d.sessions, s)
	d.mu.Unlock()
	if d.Minimal {
		return &MinimalSession{s}, nil
	}
	return s, nil
}

// VendorRoutes implements bmc.VendorRouter when Routes is set.
func (d *Driver) VendorRoutes() []bmc.Route { return d.Routes }

// Sessions returns all sessions opened so far.
func (d *Driver) Sessions() []*Session {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]*Session(nil), d.sessions...)
}

// Session is a fake session implementing many capabilities (not
// BootOverride, so that route yields 501).
type Session struct {
	DriverName, Target, User string

	mu            sync.Mutex
	calls         map[string]int
	State         bmc.PowerState
	pending       bmc.PowerState
	pollsLeft     int
	ConvergeAfter int // polls until a power action takes effect; <0 never
	LED           bmc.LEDState
	Clock         time.Duration // BMC clock offset reported by Time()
	SEL           []bmc.SELEntry
	// SensorsGate, if non-nil, blocks Sensors until closed.
	SensorsGate chan struct{}
	// FailAuth makes the next N capability calls fail with ErrAuth.
	FailAuth atomic.Int32
	Closed   atomic.Int32
}

func (s *Session) count(op string) error {
	s.mu.Lock()
	s.calls[op]++
	s.mu.Unlock()
	if s.FailAuth.Load() > 0 {
		s.FailAuth.Add(-1)
		return &bmc.OpError{Pkg: "fake", Op: op, UpstreamStatus: 401, Detail: `{"cc":7}`, Err: bmc.ErrAuth}
	}
	return nil
}

// Calls returns how often op was invoked.
func (s *Session) Calls(op string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[op]
}

func (s *Session) Close(ctx context.Context) error { s.Closed.Add(1); return nil }

func (s *Session) Info() bmc.SessionInfo {
	return bmc.SessionInfo{Driver: s.DriverName, Target: s.Target, User: s.User, Board: "FAKE1", Firmware: "1.0",
		Backends: []bmc.BackendStatus{{Name: "redfish", Available: true, Authenticated: true}}}
}

func (s *Session) SystemInfo(ctx context.Context) (bmc.SystemInfo, error) {
	if err := s.count("SystemInfo"); err != nil {
		return bmc.SystemInfo{}, err
	}
	return bmc.SystemInfo{Vendor: s.DriverName, Board: "FAKE1", BMCFirmware: "1.0"}, nil
}

func (s *Session) PowerState(ctx context.Context) (bmc.PowerState, error) {
	if err := s.count("PowerState"); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != "" {
		if s.pollsLeft > 0 {
			s.pollsLeft--
		}
		if s.pollsLeft == 0 {
			s.State, s.pending = s.pending, ""
		}
	}
	return s.State, nil
}

func (s *Session) Power(ctx context.Context, a bmc.PowerAction) error {
	if err := s.count("Power"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target := bmc.PowerOn
	if a == bmc.ActionOff || a == bmc.ActionShutdown {
		target = bmc.PowerOff
	}
	if s.ConvergeAfter < 0 {
		return nil // silently ignored
	}
	if s.ConvergeAfter == 0 {
		s.State = target
		return nil
	}
	s.pending, s.pollsLeft = target, s.ConvergeAfter
	return nil
}

func (s *Session) GetIndicatorLED(ctx context.Context) (bmc.LEDState, error) {
	if err := s.count("GetIndicatorLED"); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LED, nil
}

func (s *Session) SetIndicatorLED(ctx context.Context, st bmc.LEDState) error {
	if err := s.count("SetIndicatorLED"); err != nil {
		return err
	}
	s.mu.Lock()
	s.LED = st
	s.mu.Unlock()
	return nil
}

func (s *Session) Sensors(ctx context.Context) ([]bmc.Sensor, error) {
	if err := s.count("Sensors"); err != nil {
		return nil, err
	}
	if s.SensorsGate != nil {
		select {
		case <-s.SensorsGate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []bmc.Sensor{{ID: "1", Name: "CPU Temp", Type: bmc.SensorTemperature, Reading: bmc.F64(42), Unit: "C", Health: bmc.HealthOK}}, nil
}

func (s *Session) Events(ctx context.Context, q bmc.EventQuery) (bmc.EventPage, error) {
	if err := s.count("Events"); err != nil {
		return bmc.EventPage{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return bmc.EventPage{Entries: append([]bmc.SELEntry(nil), s.SEL...), LastID: "2"}, nil
}

func (s *Session) ClearEvents(ctx context.Context) error { return s.count("ClearEvents") }

func (s *Session) Time(ctx context.Context) (bmc.TimeConfig, error) {
	if err := s.count("Time"); err != nil {
		return bmc.TimeConfig{}, err
	}
	s.mu.Lock()
	clk := s.Clock
	s.mu.Unlock()
	now := time.Now().UTC()
	return bmc.TimeConfig{BMCTime: now.Add(clk), ProxyTime: now, OffsetSeconds: clk.Seconds()}, nil
}

func (s *Session) SetTime(ctx context.Context, u bmc.TimeUpdate) error { return s.count("SetTime") }

func (s *Session) SyncTime(ctx context.Context) (bmc.TimeConfig, error) {
	if err := s.count("SyncTime"); err != nil {
		return bmc.TimeConfig{}, err
	}
	s.mu.Lock()
	s.Clock = 0
	s.mu.Unlock()
	now := time.Now().UTC()
	return bmc.TimeConfig{BMCTime: now, ProxyTime: now}, nil
}

func (s *Session) ResetBMC(ctx context.Context, k bmc.BMCResetKind) error { return s.count("ResetBMC") }

func (s *Session) NetworkInterfaces(ctx context.Context) ([]bmc.NetworkInterface, error) {
	return []bmc.NetworkInterface{{ID: "1", Name: "eth0"}}, s.count("NetworkInterfaces")
}

func (s *Session) SetNetworkInterface(ctx context.Context, u bmc.NetworkInterfaceUpdate) error {
	return s.count("SetNetworkInterface")
}

// MinimalSession implements only Identity.
type MinimalSession struct{ s *Session }

func (m *MinimalSession) Close(ctx context.Context) error { return m.s.Close(ctx) }
func (m *MinimalSession) Info() bmc.SessionInfo           { return m.s.Info() }
func (m *MinimalSession) SystemInfo(ctx context.Context) (bmc.SystemInfo, error) {
	return m.s.SystemInfo(ctx)
}

// SampleRoutes returns vendor routes covering the HandlerFunc contract and tiers.
func SampleRoutes() []bmc.Route {
	return []bmc.Route{
		{Method: "GET", Path: "bonding", Tier: bmc.TierRead, Summary: "bond config",
			Handler: func(s bmc.Session, r *http.Request) (any, error) {
				return map[string]any{"enabled": true, "user": s.Info().User}, nil
			}},
		{Method: "POST", Path: "disableBonding", Tier: bmc.TierDisruptive,
			Handler: func(s bmc.Session, r *http.Request) (any, error) { return nil, nil }},
		{Method: "POST", Path: "setFanMode", Tier: bmc.TierWrite,
			Handler: func(s bmc.Session, r *http.Request) (any, error) {
				var b struct {
					Mode string `json:"mode"`
				}
				if err := bmc.DecodeJSON(r, &b); err != nil {
					return nil, err
				}
				return map[string]string{"mode": b.Mode}, nil
			}},
		{Method: "GET", Path: "componentStatus/{component}", Tier: bmc.TierRead,
			Handler: func(s bmc.Session, r *http.Request) (any, error) {
				return map[string]string{"component": r.PathValue("component")}, nil
			}},
		{Path: "raw/api/{path...}",
			TierFor: func(r *http.Request) (bmc.Tier, error) {
				if strings.Contains(r.PathValue("path"), "help-data") {
					return 0, bmc.Errorf(bmc.ErrForbidden, "fake", "raw", "denylisted")
				}
				if r.Method == http.MethodGet {
					return bmc.TierRead, nil
				}
				return bmc.TierDisruptive, nil
			},
			Handler: func(s bmc.Session, r *http.Request) (any, error) {
				return &bmc.RawResponse{Status: 207, Header: http.Header{"X-Raw": {"1"}}, Body: []byte("raw:" + r.PathValue("path"))}, nil
			}},
		{Method: "GET", Path: "unsupportedThing", Tier: bmc.TierRead,
			Handler: func(s bmc.Session, r *http.Request) (any, error) {
				return nil, bmc.Unsupported("fake", "unsupportedThing")
			}},
	}
}
