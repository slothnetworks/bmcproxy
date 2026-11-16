package asrock

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/ipmi"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/boards"
	"github.com/slothnetworks/bmcproxy/vendors/asrock/oem"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
)

// Session is the ASRock bmc.Session. It embeds the generic Redfish session
// when the board's Redfish accepts the credentials (B650); every capability
// method is overridden and dispatches to the backend chosen at Open (see
// Info().Capabilities), so the embedded pointer may be nil (X470).
// Safe for concurrent use.
type Session struct {
	*genericredfish.Session

	d        *Driver
	t        bmc.Target
	creds    bmc.Creds
	prof     boards.Profile
	ami      *amimegarac.Client
	api      *amiConn
	oem      *oem.Client
	rfClient *rf.Client
	openedAt time.Time

	ipmiMu sync.Mutex // serialises lazy IPMI connects

	mu          sync.Mutex
	ipmi        *ipmi.Client
	amiOK       bool
	clockOffset time.Duration
	caps        map[bmc.Capability]string
	backends    []bmc.BackendStatus
	degraded    string
	sysName     string
	fwInfo      map[string]any
	closeOnce   sync.Once
	closeErr    error
}

// Compile-time capability checks.
var (
	_ bmc.Session            = (*Session)(nil)
	_ bmc.Identity           = (*Session)(nil)
	_ bmc.PowerController    = (*Session)(nil)
	_ bmc.PowerRestorePolicy = (*Session)(nil)
	_ bmc.IndicatorLED       = (*Session)(nil)
	_ bmc.BootOverride       = (*Session)(nil)
	_ bmc.SensorReader       = (*Session)(nil)
	_ bmc.EventLog           = (*Session)(nil)
	_ bmc.EventLogClearer    = (*Session)(nil)
	_ bmc.AuditLog           = (*Session)(nil)
	_ bmc.FRUReader          = (*Session)(nil)
	_ bmc.Inventory          = (*Session)(nil)
	_ bmc.NetworkManager     = (*Session)(nil)
	_ bmc.BondManager        = (*Session)(nil)
	_ bmc.UserReader         = (*Session)(nil)
	_ bmc.ServiceReader      = (*Session)(nil)
	_ bmc.TimeManager        = (*Session)(nil)
	_ bmc.FirmwareInventory  = (*Session)(nil)
	_ bmc.VirtualMedia       = (*Session)(nil)
	_ bmc.BMCReset           = (*Session)(nil)
)

// capPrefs is the backend preference order per capability (first available
// wins). "redfish" counts only when the generic Redfish session discovered
// the resource (and it was not excluded); "ami" when /api login worked;
// "ipmi" when the profile allows the lazy IPMI backend.
func capPrefs(p boards.Profile) map[bmc.Capability][]string {
	R, A, I := bmc.BackendRedfish, bmc.BackendAMI, bmc.BackendIPMI
	restore := []string{R, A, I}
	switch p.Restore {
	case boards.RestoreASRR:
		restore = []string{A, I}
	case boards.RestoreIPMI:
		restore = []string{I}
	}
	return map[bmc.Capability][]string{
		bmc.CapIdentity:           {R, A},
		bmc.CapPower:              {R, A, I},
		bmc.CapPowerRestorePolicy: restore,
		bmc.CapIndicatorLED:       {R, A},
		bmc.CapBootOverride:       {R, I}, // no boot override on /api
		bmc.CapSensors:            {R, A},
		bmc.CapEventLog:           {R, A},
		bmc.CapEventLogClear:      {R, A},
		bmc.CapAuditLog:           {R, A},
		bmc.CapFRU:                {A, R}, // /api FRU has the real board/product areas
		bmc.CapInventory:          {R, A},
		bmc.CapNetwork:            {R, A},
		bmc.CapBond:               {A},
		bmc.CapUsers:              {R, A},
		bmc.CapServices:           {R, A},
		bmc.CapTime:               {R, A},
		bmc.CapFirmwareInventory:  {R, A},
		bmc.CapVirtualMedia:       {R}, // /api virtual media: later (PLAN milestone 5)
		bmc.CapBMCReset:           {R, A},
	}
}

// redfishExclude lists capabilities the AMI /api serves better on a
// Redfish-capable board (passed to genericredfish.NewSession as Exclude).
func redfishExclude(p boards.Profile) []bmc.Capability {
	ex := []bmc.Capability{bmc.CapFRU}
	if p.Restore != boards.RestoreRedfish {
		ex = append(ex, bmc.CapPowerRestorePolicy)
	}
	return ex
}

// openSession completes Open: login + identify (unless handed off), pick the
// profile, try Redfish when the profile allows it, plan the backends.
func (d *Driver) openSession(ctx context.Context, t bmc.Target, c bmc.Creds, cl *amimegarac.Client, id identity) (*Session, error) {
	log := d.log(ctx)
	s := &Session{d: d, t: t, creds: c, ami: cl, openedAt: d.opts.Now(), caps: map[bmc.Capability]string{}}
	s.api = &amiConn{c: cl, s: s}

	var amiErr error
	if id.Board == "" {
		if amiErr = cl.Login(ctx); amiErr == nil {
			id, amiErr = identify(ctx, cl)
		}
	}
	if amiErr != nil && !errors.Is(amiErr, bmc.ErrAuth) && !errors.Is(amiErr, bmc.ErrUnsupported) {
		// Transport/busy/timeout: surface it rather than half-open.
		_ = cl.Logout(ctx)
		return nil, amiErr
	}
	s.amiOK = amiErr == nil
	s.prof = boards.Lookup(id.Board, id.Firmware)
	s.sysName, s.fwInfo = id.SysName, id.FWInfo
	if s.amiOK {
		if s.prof.TLS != transport.ProfileAuto {
			d.Transport().SetProfile(t.Addr(), s.prof.TLS)
		}
		cl.Denylist().Add(s.prof.ExtraDeny...)
		d.setFamily(t.Addr(), familyForProfile(s.prof))
		if si, ok := cl.Session(); ok {
			s.clockOffset = si.ClockOffset
		}
	}
	s.oem = oem.New(s.api, s.prof)
	amiStatus := bmc.BackendStatus{Name: bmc.BackendAMI, Available: true, Authenticated: s.amiOK}
	if amiErr != nil {
		amiStatus.Detail = bmc.Class(amiErr) + ": " + amiErr.Error()
		if errors.Is(amiErr, bmc.ErrUnsupported) {
			amiStatus.Detail = "no asrr namespace (board unknown)"
		}
	}
	s.backends = append(s.backends, amiStatus)

	// Redfish.
	useRedfish := s.prof.Redfish != boards.RedfishNo || x470PAMRedfish(ctx, d, t, c, s.prof)
	if !s.amiOK {
		// /api refused us; Redfish is the only hope whatever the profile says.
		useRedfish = true
	}
	var rfErr error
	if useRedfish {
		rfErr = s.openRedfish(ctx)
		st := bmc.BackendStatus{Name: bmc.BackendRedfish, Available: true, Authenticated: rfErr == nil}
		if rfErr != nil {
			st.Detail = bmc.Class(rfErr) + ": " + rfErr.Error()
			if bmc.HTTPStatus(rfErr) >= 500 && !errors.Is(rfErr, bmc.ErrAuth) {
				st.Available = false
			}
			if s.prof.Redfish == boards.RedfishYes {
				s.degraded = "redfish unavailable (" + bmc.Class(rfErr) + "); serving from AMI /api"
			}
		} else {
			st.Detail = s.rfClient.AuthMode().String()
		}
		s.backends = append(s.backends, st)
	} else {
		s.backends = append(s.backends, bmc.BackendStatus{Name: bmc.BackendRedfish, Available: true, Authenticated: false,
			Detail: "not used on " + s.prof.Key + " (profile: /api + IPMI)"})
	}
	if !s.amiOK && rfErr != nil {
		err := amiErr
		if errors.Is(rfErr, bmc.ErrAuth) || !errors.Is(err, bmc.ErrAuth) {
			err = rfErr
		}
		_ = s.Close(ctx)
		return nil, err
	}
	if s.prof.IPMI {
		s.backends = append(s.backends, bmc.BackendStatus{Name: bmc.BackendIPMI, Available: true, Detail: "lazy: connected on first use"})
	}
	s.planCaps()
	if s.Session != nil && s.clockOffset == 0 {
		s.clockOffset = s.Session.ClockOffset()
	}
	if s.prof.Key == "unknown" {
		s.addDegraded("unknown board/firmware: capability probing")
	}
	log.DebugContext(ctx, "asrock session opened", "bmc", t.Addr(), "user", c.User, "board", s.prof.Board,
		"firmware", s.prof.Firmware, "profile", s.prof.Key, "verified", s.prof.Verified,
		"ami", s.amiOK, "redfish", s.Session != nil, "caps", len(s.caps))
	return s, nil
}

func (s *Session) addDegraded(why string) {
	if s.degraded == "" {
		s.degraded = why
	} else {
		s.degraded += "; " + why
	}
}

func (s *Session) openRedfish(ctx context.Context) error {
	o := s.d.opts.RedfishOptions
	o.Creds, o.Auth = s.creds, rf.AuthBasic
	cl, err := rf.New(s.d.Transport().Endpoint(s.t.Addr(), transport.LaneRedfish), o)
	if err != nil {
		return err
	}
	if err := cl.Login(ctx); err != nil {
		_ = cl.Close(ctx)
		return err
	}
	gs, err := genericredfish.NewSession(ctx, cl, genericredfish.SessionOptions{
		DriverName: Name, Target: s.t, Board: s.prof.Board, Firmware: s.prof.Firmware,
		Exclude: redfishExclude(s.prof), CloseClient: true, Now: s.d.opts.Now,
	})
	if err != nil {
		_ = cl.Close(ctx)
		return err
	}
	s.rfClient, s.Session = cl, gs
	return nil
}

// planCaps picks the backend per capability.
func (s *Session) planCaps() {
	var rfCaps map[bmc.Capability]string
	if s.Session != nil {
		rfCaps = s.Session.Capabilities()
	}
	for c, prefs := range capPrefs(s.prof) {
		for _, b := range prefs {
			ok := false
			switch b {
			case bmc.BackendRedfish:
				ok = rfCaps[c] != ""
			case bmc.BackendAMI:
				ok = s.amiOK
			case bmc.BackendIPMI:
				ok = s.prof.IPMI
			}
			if ok {
				s.caps[c] = b
				break
			}
		}
	}
}

// backend returns the backend chosen for c ("" = unsupported).
func (s *Session) backend(c bmc.Capability) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.caps[c]
}

// Profile returns the board profile in use.
func (s *Session) Profile() boards.Profile { return s.prof }

// AMI returns the AMI /api connection (session-loss aware) for raw use.
func (s *Session) AMI() *amiConn { return s.api }

// OEM returns the ASRock OEM operations client.
func (s *Session) OEM() *oem.Client { return s.oem }

// RedfishClient returns the Redfish client, or nil when Redfish is not in
// use on this session.
func (s *Session) RedfishClient() *rf.Client { return s.rfClient }

// Info implements bmc.Session.
func (s *Session) Info() bmc.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	caps := make(map[bmc.Capability]string, len(s.caps))
	for k, v := range s.caps {
		caps[k] = v
	}
	bs := append([]bmc.BackendStatus(nil), s.backends...)
	if s.ipmi != nil {
		for i := range bs {
			if bs[i].Name == bmc.BackendIPMI {
				bs[i].Authenticated, bs[i].Detail = true, "connected"
			}
		}
	}
	deg := s.degraded
	if s.amiOK && s.ami.Degraded() {
		if deg != "" {
			deg += "; "
		}
		deg += "AMI /api re-login or backend restart within the last minute"
	}
	if l, why := s.ami.HostLost(); l {
		if deg != "" {
			deg += "; "
		}
		deg += "AMI backend restarted (" + why + ")"
	}
	return bmc.SessionInfo{Driver: Name, Target: s.t.Addr(), User: s.creds.User, Board: s.prof.Board,
		Firmware: s.prof.Firmware, OpenedAt: s.openedAt, ClockOffset: s.clockOffset, Backends: bs,
		Capabilities: caps, Degraded: deg}
}

// Capabilities returns the capability -> backend map (copy).
func (s *Session) Capabilities() map[bmc.Capability]string { return s.Info().Capabilities }

// Supports reports whether capability c has a backend.
func (s *Session) Supports(c bmc.Capability) bool { return s.backend(c) != "" }

// ClockOffset returns bmcTime - proxyTime as last measured.
func (s *Session) ClockOffset() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clockOffset
}

func (s *Session) setClockOffset(d time.Duration) {
	s.mu.Lock()
	s.clockOffset = d
	s.mu.Unlock()
	s.mu.Lock()
	ic := s.ipmi
	s.mu.Unlock()
	if ic != nil {
		ic.SetClockOffset(d)
	}
}

// Close logs out of every backend (AMI DELETE /api/session, Redfish
// session/client close, IPMI close). Idempotent.
func (s *Session) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		var errs []error
		if s.ami != nil {
			if err := s.ami.Logout(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		if s.Session != nil {
			if err := s.Session.Close(ctx); err != nil {
				errs = append(errs, err)
			}
		} else if s.rfClient != nil {
			_ = s.rfClient.Close(ctx)
		}
		s.mu.Lock()
		ic := s.ipmi
		s.ipmi = nil
		s.mu.Unlock()
		if ic != nil {
			if err := ic.Close(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}

// ipmiClient connects the IPMI backend on first use.
func (s *Session) ipmiClient(ctx context.Context) (*ipmi.Client, error) {
	if !s.prof.IPMI {
		return nil, bmc.Unsupported(Pkg, "IPMI backend not enabled for "+s.prof.Key)
	}
	s.mu.Lock()
	ic := s.ipmi
	s.mu.Unlock()
	if ic != nil {
		return ic, nil
	}
	s.ipmiMu.Lock()
	defer s.ipmiMu.Unlock()
	s.mu.Lock()
	ic = s.ipmi
	off := s.clockOffset
	s.mu.Unlock()
	if ic != nil {
		return ic, nil
	}
	ic, err := ipmi.Connect(ctx, s.t.IPMIAddr(), s.creds, ipmi.Options{Dialer: s.d.opts.IPMIDialer, ClockOffset: off})
	if err != nil {
		return nil, err
	}
	s.d.log(ctx).DebugContext(ctx, "asrock ipmi backend connected", "bmc", s.t.IPMIAddr())
	s.mu.Lock()
	s.ipmi = ic
	s.mu.Unlock()
	return ic, nil
}

func (s *Session) unsupported(op string) error {
	return &bmc.OpError{Pkg: Pkg, Op: op, Detail: "no backend for this capability on " + s.prof.Key, Err: bmc.ErrUnsupported}
}

// ---- AMI connection wrapper ----

// amiConn wraps the amimegarac client with the profile's per-endpoint
// timeouts. It implements oem.API. Session loss is the client's job: it is
// built with amimegarac.Config.ReportSessionLost, so after a backend
// restart (502 / HTML 500) or MarkHostLost every call (Clock included)
// fails with bmc.ErrSessionLost until the session manager reopens.
type amiConn struct {
	c *amimegarac.Client
	s *Session
}

var _ oem.API = (*amiConn)(nil)

// Do sends a raw /api request (see amimegarac.Client.Do).
func (a *amiConn) Do(ctx context.Context, r *amimegarac.Request) (*amimegarac.Response, error) {
	op := r.Method + " " + r.Path
	if !a.s.amiOK {
		return nil, &bmc.OpError{Pkg: Pkg, Op: op, Backend: bmc.BackendAMI, Detail: "AMI /api login failed at session open", Err: bmc.ErrUnsupported}
	}
	if cp, _, err := amimegarac.CanonicalPath(r.Path); err == nil {
		if d, ok := a.s.prof.TimeoutFor(cp); ok {
			o := transport.OptionsFrom(ctx)
			o.Timeout, o.NoRetry = d, true
			ctx = transport.WithOptions(ctx, o)
		}
	}
	return a.c.Do(ctx, r)
}

// DoJSON implements oem.API.
func (a *amiConn) DoJSON(ctx context.Context, method, path string, body, dst any) error {
	req := &amimegarac.Request{Method: method, Path: path}
	switch b := body.(type) {
	case nil:
	case []byte:
		req.Body = b
	default:
		enc, err := jsonMarshal(body)
		if err != nil {
			return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, method+" "+path, "encode body: %v", err)
		}
		req.Body = enc
	}
	resp, err := a.Do(ctx, req)
	if err != nil {
		return err
	}
	return resp.Decode(dst)
}

// Get is DoJSON GET.
func (a *amiConn) Get(ctx context.Context, path string, dst any) error {
	return a.DoJSON(ctx, http.MethodGet, path, nil, dst)
}

// Exclusive implements oem.API: login first (a login started by another
// goroutine would otherwise wait behind the held slot), then hold the AMI
// lane for fn.
func (a *amiConn) Exclusive(ctx context.Context, fn func(ctx context.Context) error) error {
	// Login would start a fresh session; a lost one must surface instead.
	if err := a.c.SessionLostError("Exclusive"); err != nil {
		return err
	}
	if err := a.c.Login(ctx); err != nil {
		return err
	}
	return a.c.Endpoint().Exclusive(ctx, fn)
}
