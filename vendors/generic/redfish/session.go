package genericredfish

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/obs"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
)

// SessionOptions configure NewSession.
type SessionOptions struct {
	// DriverName goes into SessionInfo.Driver and SystemInfo.Vendor
	// (default "redfish").
	DriverName string
	Target     bmc.Target
	// Board / Firmware override the discovered values (Chassis/System
	// Model, Manager FirmwareVersion).
	Board, Firmware string
	// Path overrides (otherwise discovered from the ServiceRoot and the
	// client's Quirks.PathOverrides logical keys).
	SystemPath, ManagerPath, ChassisPath string
	SELPath, AuditLogPath                string
	// Exclude hides capabilities from Info().Capabilities (an embedding
	// driver serves them from another backend or not at all). The methods
	// still work if called directly.
	Exclude []bmc.Capability
	// CloseClient makes Close log the client out (set when the session owns
	// the client).
	CloseClient bool
	// Now is the clock (tests). Default time.Now.
	Now func() time.Time
}

// Session is a bmc.Session over Redfish implementing every capability the
// service can serve. It is safe for concurrent use. Vendor drivers can embed
// *Session and override methods; C exposes the client for extra calls.
type Session struct {
	C *rf.Client

	opts     SessionOptions
	openedAt time.Time

	mu          sync.Mutex
	sysPath     string
	mgrPath     string
	chassisPath string
	selPath     string
	auditPath   string
	vmPath      string
	board       string
	firmware    string
	clockOffset time.Duration
	caps        map[bmc.Capability]string
	degraded    string
	closeOnce   sync.Once
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
	_ bmc.UserReader         = (*Session)(nil)
	_ bmc.ServiceReader      = (*Session)(nil)
	_ bmc.TimeManager        = (*Session)(nil)
	_ bmc.FirmwareInventory  = (*Session)(nil)
	_ bmc.VirtualMedia       = (*Session)(nil)
	_ bmc.BMCReset           = (*Session)(nil)
)

// NewSession discovers the system/manager/chassis, log services and
// optional features of an authenticated client, measures the BMC clock
// offset, and returns the Session. Only the ComputerSystem is required.
func NewSession(ctx context.Context, c *rf.Client, o SessionOptions) (*Session, error) {
	if o.DriverName == "" {
		o.DriverName = Name
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	s := &Session{C: c, opts: o, openedAt: o.Now(), caps: map[bmc.Capability]string{}}
	log := obs.FromContext(ctx, obs.SubDriverRedfish)
	B := bmc.BackendRedfish

	// System (required).
	s.sysPath = o.SystemPath
	if s.sysPath == "" {
		p, err := c.SystemPath(ctx)
		if err != nil {
			return nil, err
		}
		s.sysPath = p
	}
	sys, err := c.GetSystem(ctx, s.sysPath)
	if err != nil {
		return nil, err
	}
	s.caps[bmc.CapIdentity] = B
	s.caps[bmc.CapPower] = B
	s.caps[bmc.CapInventory] = B
	if sys.PowerRestorePolicy != nil {
		s.caps[bmc.CapPowerRestorePolicy] = B
	}
	if sys.Boot != nil {
		s.caps[bmc.CapBootOverride] = B
	}

	// Manager (optional).
	var mgr *rf.Manager
	s.mgrPath = o.ManagerPath
	if s.mgrPath == "" {
		if len(sys.Links.ManagedBy) > 0 && c.Quirks().LogicalPath(rf.PathManager) == "" {
			s.mgrPath = sys.Links.ManagedBy[0].ID
		} else if p, err := c.ManagerPath(ctx); err == nil {
			s.mgrPath = p
		}
	}
	if s.mgrPath != "" {
		mreqStart := o.Now()
		if m, err := c.GetManager(ctx, s.mgrPath); err == nil {
			mgr = m
			s.caps[bmc.CapBMCReset] = B
			s.firmware = m.FirmwareVersion
			if m.EthernetInterfaces.ID != "" {
				s.caps[bmc.CapNetwork] = B
			}
			if m.NetworkProtocol.ID != "" {
				s.caps[bmc.CapServices] = B
			}
			if m.DateTime != "" {
				s.caps[bmc.CapTime] = B
				if bt, err := rf.ParseTime(m.DateTime); err == nil && !bt.IsZero() {
					mid := mreqStart.Add(o.Now().Sub(mreqStart) / 2)
					s.clockOffset = bt.Sub(mid).Truncate(time.Second)
				}
			}
			if m.VirtualMedia.ID != "" {
				s.vmPath = m.VirtualMedia.ID
			}
		} else {
			log.DebugContext(ctx, "redfish manager unavailable", "path", s.mgrPath, "err", err)
			s.mgrPath = ""
		}
	}
	if s.vmPath == "" && sys.VirtualMedia.ID != "" {
		s.vmPath = sys.VirtualMedia.ID
	}
	if s.vmPath != "" {
		s.caps[bmc.CapVirtualMedia] = B
	}

	// Chassis (optional).
	var ch *rf.Chassis
	s.chassisPath = o.ChassisPath
	if s.chassisPath == "" {
		if len(sys.Links.Chassis) > 0 && c.Quirks().LogicalPath(rf.PathChassis) == "" {
			s.chassisPath = sys.Links.Chassis[0].ID
		} else if p, err := c.ChassisPath(ctx); err == nil {
			s.chassisPath = p
		}
	}
	if s.chassisPath != "" {
		if cc, err := c.GetChassis(ctx, s.chassisPath); err == nil {
			ch = cc
			s.caps[bmc.CapFRU] = B
			if cc.Sensors.ID != "" || cc.Thermal.ID != "" || cc.Power.ID != "" {
				s.caps[bmc.CapSensors] = B
			}
			if cc.IndicatorLED != nil || cc.LocationIndicatorActive != nil {
				s.caps[bmc.CapIndicatorLED] = B
			}
		} else {
			s.chassisPath = ""
		}
	}
	if sys.IndicatorLED != nil || sys.LocationIndicatorActive != nil {
		s.caps[bmc.CapIndicatorLED] = B
	}

	// Log services.
	s.selPath, s.auditPath = o.SELPath, o.AuditLogPath
	if s.selPath == "" {
		s.selPath = c.Quirks().LogicalPath(rf.PathSEL)
	}
	if s.auditPath == "" {
		s.auditPath = c.Quirks().LogicalPath(rf.PathAuditLog)
	}
	if s.selPath == "" || s.auditPath == "" {
		var colls []string
		if mgr != nil && mgr.LogServices.ID != "" {
			colls = append(colls, mgr.LogServices.ID)
		}
		if sys.LogServices.ID != "" {
			colls = append(colls, sys.LogServices.ID)
		}
		sel, audit := discoverLogs(ctx, c, colls)
		if s.selPath == "" {
			s.selPath = sel
		}
		if s.auditPath == "" {
			s.auditPath = audit
		}
	}
	if s.selPath != "" {
		s.caps[bmc.CapEventLog] = B
		s.caps[bmc.CapEventLogClear] = B
	}
	if s.auditPath != "" {
		s.caps[bmc.CapAuditLog] = B
	}

	// Service-level features.
	if root, err := c.ServiceRoot(ctx); err == nil {
		if root.AccountService.ID != "" {
			s.caps[bmc.CapUsers] = B
		}
		if root.UpdateService.ID != "" {
			s.caps[bmc.CapFirmwareInventory] = B
		}
	}

	// Board.
	s.board = o.Board
	if s.board == "" {
		if ch != nil && usable(ch.Model) {
			s.board = ch.Model
		} else if usable(sys.Model) {
			s.board = sys.Model
		}
	}
	if o.Firmware != "" {
		s.firmware = o.Firmware
	}
	for _, x := range o.Exclude {
		delete(s.caps, x)
	}
	return s, nil
}

func usable(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.EqualFold(s, "To be filled by O.E.M.") && !strings.EqualFold(s, "N/A") && !strings.EqualFold(s, "Default string")
}

func discoverLogs(ctx context.Context, c *rf.Client, colls []string) (sel, audit string) {
	var eventLog string
	for _, coll := range colls {
		services, err := c.LogServices(ctx, coll)
		if err != nil {
			continue
		}
		for _, ls := range services {
			id := lower(ls.ID)
			switch {
			case sel == "" && (id == "sel" || lower(ls.LogEntryType) == "sel"):
				sel = ls.ODataID
			case audit == "" && strings.Contains(id, "audit"):
				audit = ls.ODataID
			case eventLog == "" && (id == "eventlog" || id == "log"):
				eventLog = ls.ODataID
			}
		}
	}
	if sel == "" {
		sel = eventLog
	}
	return sel, audit
}

// ---- bmc.Session ----

// Close logs the client out if the session owns it. Idempotent.
func (s *Session) Close(ctx context.Context) error {
	var err error
	s.closeOnce.Do(func() {
		if s.opts.CloseClient {
			err = s.C.Close(ctx)
		}
	})
	return err
}

// Info describes the session.
func (s *Session) Info() bmc.SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	caps := make(map[bmc.Capability]string, len(s.caps))
	for k, v := range s.caps {
		caps[k] = v
	}
	return bmc.SessionInfo{Driver: s.opts.DriverName, Target: s.opts.Target.Addr(), User: s.C.User(), Board: s.board,
		Firmware: s.firmware, OpenedAt: s.openedAt, ClockOffset: s.clockOffset,
		Backends:     []bmc.BackendStatus{{Name: bmc.BackendRedfish, Available: true, Authenticated: true, Detail: s.C.AuthMode().String()}},
		Capabilities: caps, Degraded: s.degraded}
}

// Capabilities returns the capability -> backend map (copy).
func (s *Session) Capabilities() map[bmc.Capability]string { return s.Info().Capabilities }

// Supports reports whether c was discovered (and not excluded).
func (s *Session) Supports(c bmc.Capability) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.caps[c] != ""
}

// Paths returns the discovered resource paths.
func (s *Session) Paths() (system, manager, chassis, sel, audit string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sysPath, s.mgrPath, s.chassisPath, s.selPath, s.auditPath
}

// ClockOffset returns bmcTime - proxyTime measured at open.
func (s *Session) ClockOffset() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clockOffset
}

func (s *Session) need(c bmc.Capability, op string) error {
	if !s.Supports(c) {
		// still honour explicit excludes as "supported elsewhere": only fail
		// when the resource is genuinely missing
		if slices.Contains(s.opts.Exclude, c) {
			return nil
		}
		return bmc.Unsupported(Pkg, op)
	}
	return nil
}

func (s *Session) system(ctx context.Context) (*rf.ComputerSystem, error) {
	return s.C.GetSystem(ctx, s.sysPath)
}

func (s *Session) manager(ctx context.Context, op string) (*rf.Manager, error) {
	if s.mgrPath == "" {
		return nil, bmc.Unsupported(Pkg, op)
	}
	return s.C.GetManager(ctx, s.mgrPath)
}

func (s *Session) chassis(ctx context.Context, op string) (*rf.Chassis, error) {
	if s.chassisPath == "" {
		return nil, bmc.Unsupported(Pkg, op)
	}
	return s.C.GetChassis(ctx, s.chassisPath)
}

// ---- Identity ----

// SystemInfo implements bmc.Identity.
func (s *Session) SystemInfo(ctx context.Context) (bmc.SystemInfo, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return bmc.SystemInfo{}, err
	}
	si := bmc.SystemInfo{Vendor: s.opts.DriverName, Manufacturer: sys.Manufacturer, Model: sys.Model, Serial: sys.SerialNumber,
		UUID: sys.UUID, BIOSVersion: sys.BiosVersion, Hostname: sys.HostName, PowerState: PowerState(sys.PowerState),
		Health: Health(sys.Status)}
	if !usable(si.Serial) {
		si.Serial = ""
	}
	s.mu.Lock()
	si.Board, si.BMCFirmware = s.board, s.firmware
	s.mu.Unlock()
	if mgr, err := s.manager(ctx, "SystemInfo"); err == nil {
		si.BMCFirmware = mgr.FirmwareVersion
		if s.opts.Firmware != "" {
			si.BMCFirmware = s.opts.Firmware
		}
		if ifs, err := s.C.EthernetInterfaces(ctx, mgr.EthernetInterfaces.ID); err == nil {
			for _, e := range ifs {
				if e.MACAddress != "" && (e.InterfaceEnabled == nil || *e.InterfaceEnabled) && !strings.HasPrefix(lower(e.ID), "usb") {
					si.BMCMAC = strings.ToLower(e.MACAddress)
					break
				}
			}
		}
	}
	return si, nil
}

// ---- PowerController ----

// PowerState implements bmc.PowerController.
func (s *Session) PowerState(ctx context.Context) (bmc.PowerState, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return bmc.PowerUnknown, err
	}
	return PowerState(sys.PowerState), nil
}

// Power implements bmc.PowerController (ComputerSystem.Reset with the
// ResetType chosen from the advertised allowable values). It does not wait
// for the state change; callers poll PowerState.
func (s *Session) Power(ctx context.Context, a bmc.PowerAction) error {
	sys, err := s.system(ctx)
	if err != nil {
		return err
	}
	allowed := s.C.AllowableValues(ctx, sys.Actions, "#ComputerSystem.Reset", "ResetType")
	rt, err := ResetTypeFor(a, allowed)
	if err != nil {
		return err
	}
	obs.FromContext(ctx, obs.SubDriverRedfish).DebugContext(ctx, "redfish power action", "action", a, "resetType", rt)
	_, err = s.C.ResetSystem(ctx, sys, rt)
	return err
}

// ---- PowerRestorePolicy ----

// GetPowerRestorePolicy implements bmc.PowerRestorePolicy.
func (s *Session) GetPowerRestorePolicy(ctx context.Context) (bmc.RestorePolicy, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return bmc.RestoreUnknown, err
	}
	if sys.PowerRestorePolicy == nil {
		return bmc.RestoreUnknown, bmc.Unsupported(Pkg, "GetPowerRestorePolicy")
	}
	return RestorePolicyFromRedfish(*sys.PowerRestorePolicy), nil
}

// SetPowerRestorePolicy implements bmc.PowerRestorePolicy.
func (s *Session) SetPowerRestorePolicy(ctx context.Context, p bmc.RestorePolicy) error {
	v, err := RestorePolicyToRedfish(p)
	if err != nil {
		return err
	}
	if err := s.need(bmc.CapPowerRestorePolicy, "SetPowerRestorePolicy"); err != nil {
		return err
	}
	_, err = s.C.SetPowerRestorePolicy(ctx, s.sysPath, v)
	return err
}

// ---- IndicatorLED ----

// ledTarget returns the path and current values of the resource carrying
// the indicator (chassis preferred, then system).
func (s *Session) ledTarget(ctx context.Context) (path string, led *string, active *bool, allow []string, err error) {
	if s.chassisPath != "" {
		ch, err := s.C.GetChassis(ctx, s.chassisPath)
		if err != nil {
			return "", nil, nil, nil, err
		}
		if ch.IndicatorLED != nil || ch.LocationIndicatorActive != nil {
			return ch.ODataID, ch.IndicatorLED, ch.LocationIndicatorActive, ch.IndicatorLEDAllowable, nil
		}
	}
	sys, err := s.system(ctx)
	if err != nil {
		return "", nil, nil, nil, err
	}
	if sys.IndicatorLED != nil || sys.LocationIndicatorActive != nil {
		return sys.ODataID, sys.IndicatorLED, sys.LocationIndicatorActive, nil, nil
	}
	return "", nil, nil, nil, bmc.Unsupported(Pkg, "IndicatorLED")
}

// GetIndicatorLED implements bmc.IndicatorLED.
func (s *Session) GetIndicatorLED(ctx context.Context) (bmc.LEDState, error) {
	_, led, active, _, err := s.ledTarget(ctx)
	if err != nil {
		return bmc.LEDUnknown, err
	}
	return LEDFromRedfish(led, active), nil
}

// SetIndicatorLED implements bmc.IndicatorLED. IndicatorLED is preferred
// (Lit/Blinking/Off); LocationIndicatorActive is used when only it exists
// (blinking -> true).
func (s *Session) SetIndicatorLED(ctx context.Context, st bmc.LEDState) error {
	path, led, _, allow, err := s.ledTarget(ctx)
	if err != nil {
		return err
	}
	if led != nil {
		var v string
		switch st {
		case bmc.LEDOn:
			v = "Lit"
		case bmc.LEDOff:
			v = "Off"
		case bmc.LEDBlinking:
			v = "Blinking"
		default:
			return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetIndicatorLED", "unknown LED state %q", st)
		}
		if len(allow) > 0 && !slices.Contains(allow, v) {
			return bmc.Errorf(bmc.ErrUnsupported, Pkg, "SetIndicatorLED", "%s not allowed (allowable: %s)", v, strings.Join(allow, ","))
		}
		_, err = s.C.SetIndicatorLED(ctx, path, v)
		return err
	}
	switch st {
	case bmc.LEDOn, bmc.LEDBlinking:
		_, err = s.C.SetLocationIndicator(ctx, path, true)
	case bmc.LEDOff:
		_, err = s.C.SetLocationIndicator(ctx, path, false)
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetIndicatorLED", "unknown LED state %q", st)
	}
	return err
}

// ---- BootOverride ----

// GetBootOverride implements bmc.BootOverride.
func (s *Session) GetBootOverride(ctx context.Context) (bmc.BootOverrideConfig, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return bmc.BootOverrideConfig{}, err
	}
	if sys.Boot == nil {
		return bmc.BootOverrideConfig{}, bmc.Unsupported(Pkg, "GetBootOverride")
	}
	cfg := bmc.BootOverrideConfig{Target: BootTargetFromRedfish(sys.Boot.BootSourceOverrideTarget),
		Mode: BootModeFromRedfish(sys.Boot.BootSourceOverrideEnabled)}
	switch sys.Boot.BootSourceOverrideMode {
	case "UEFI":
		cfg.BootMode = "uefi"
	case "Legacy":
		cfg.BootMode = "legacy"
	}
	return cfg, nil
}

// SetBootOverride implements bmc.BootOverride.
func (s *Session) SetBootOverride(ctx context.Context, b bmc.BootOverrideConfig) error {
	target, err := BootTargetToRedfish(b.Target)
	if err != nil {
		return err
	}
	bo := rf.BootOverride{Target: target}
	switch b.Mode {
	case bmc.BootOnce, "":
		bo.Enabled = "Once"
	case bmc.BootContinuous:
		bo.Enabled = "Continuous"
	case bmc.BootDisabled:
		bo.Enabled, bo.Target = "Disabled", "None"
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetBootOverride", "unknown mode %q", b.Mode)
	}
	if target == "None" {
		bo.Enabled = "Disabled"
	}
	switch lower(b.BootMode) {
	case "uefi":
		bo.Mode = "UEFI"
	case "legacy":
		bo.Mode = "Legacy"
	case "":
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetBootOverride", "unknown boot mode %q", b.BootMode)
	}
	if err := s.need(bmc.CapBootOverride, "SetBootOverride"); err != nil {
		return err
	}
	_, err = s.C.SetBootOverride(ctx, s.sysPath, bo)
	return err
}

// ---- SensorReader ----

// Sensors implements bmc.SensorReader: the modern Chassis Sensors
// collection when present, else the legacy Thermal + Power resources.
func (s *Session) Sensors(ctx context.Context) ([]bmc.Sensor, error) {
	ch, err := s.chassis(ctx, "Sensors")
	if err != nil {
		return nil, err
	}
	if ch.Sensors.ID != "" {
		list, err := s.C.Sensors(ctx, ch)
		if err == nil {
			out := make([]bmc.Sensor, 0, len(list))
			for _, x := range list {
				out = append(out, SensorFromRedfish(x))
			}
			return out, nil
		}
		if ch.Thermal.ID == "" && ch.Power.ID == "" {
			return nil, err
		}
	}
	var out []bmc.Sensor
	if ch.Thermal.ID != "" {
		th, err := s.C.Thermal(ctx, ch)
		if err != nil {
			return nil, err
		}
		out = append(out, SensorsFromThermal(th)...)
	}
	if ch.Power.ID != "" {
		p, err := s.C.Power(ctx, ch)
		if err != nil {
			return nil, err
		}
		out = append(out, SensorsFromPower(p)...)
	}
	if out == nil && ch.Thermal.ID == "" && ch.Power.ID == "" {
		return nil, bmc.Unsupported(Pkg, "Sensors")
	}
	return out, nil
}

// ---- EventLog / EventLogClearer / AuditLog ----

func (s *Session) logService(ctx context.Context, path, op string) (*rf.LogService, error) {
	if path == "" {
		return nil, bmc.Unsupported(Pkg, op)
	}
	var ls rf.LogService
	if _, err := s.C.Get(ctx, path, &ls); err != nil {
		return nil, err
	}
	if ls.ODataID == "" {
		ls.ODataID = path
	}
	return &ls, nil
}

func (s *Session) entriesSince(ctx context.Context, path, op, since string, limit int) ([]rf.LogEntry, bool, error) {
	ls, err := s.logService(ctx, path, op)
	if err != nil {
		return nil, false, err
	}
	all, err := s.C.LogEntries(ctx, ls)
	if err != nil {
		return nil, false, err
	}
	slices.SortStableFunc(all, func(a, b rf.LogEntry) int {
		switch {
		case idLess(a.ID, b.ID):
			return -1
		case idLess(b.ID, a.ID):
			return 1
		}
		return 0
	})
	var out []rf.LogEntry
	for _, e := range all {
		if idAfter(e.ID, since) {
			out = append(out, e)
		}
	}
	more := false
	if limit > 0 && len(out) > limit {
		out, more = out[:limit], true
	}
	return out, more, nil
}

// Events implements bmc.EventLog: SEL entries with Id > q.SinceID
// (numeric order), capped by q.Limit.
func (s *Session) Events(ctx context.Context, q bmc.EventQuery) (bmc.EventPage, error) {
	entries, more, err := s.entriesSince(ctx, s.selPath, "Events", q.SinceID, q.Limit)
	if err != nil {
		return bmc.EventPage{}, err
	}
	off := s.ClockOffset()
	page := bmc.EventPage{Entries: make([]bmc.SELEntry, 0, len(entries)), LastID: q.SinceID, More: more}
	for _, e := range entries {
		page.Entries = append(page.Entries, SELFromLogEntry(e, off))
		page.LastID = e.ID
	}
	return page, nil
}

// ClearEvents implements bmc.EventLogClearer (LogService.ClearLog).
func (s *Session) ClearEvents(ctx context.Context) error {
	ls, err := s.logService(ctx, s.selPath, "ClearEvents")
	if err != nil {
		return err
	}
	_, err = s.C.ClearLog(ctx, ls)
	return err
}

// AuditLog implements bmc.AuditLog from the manager's audit log service.
func (s *Session) AuditLog(ctx context.Context, q bmc.AuditQuery) ([]bmc.AuditEntry, error) {
	entries, _, err := s.entriesSince(ctx, s.auditPath, "AuditLog", q.SinceID, q.Limit)
	if err != nil {
		return nil, err
	}
	off := s.ClockOffset()
	out := make([]bmc.AuditEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, AuditFromLogEntry(e, off))
	}
	return out, nil
}

// ---- FRUReader ----

// FRU implements bmc.FRUReader from the Chassis (board/chassis areas) and
// ComputerSystem (product area).
func (s *Session) FRU(ctx context.Context) ([]bmc.FRU, error) {
	ch, err := s.chassis(ctx, "FRU")
	if err != nil {
		return nil, err
	}
	sys, err := s.system(ctx)
	if err != nil {
		return nil, err
	}
	keep := func(v string) string {
		if usable(v) {
			return v
		}
		return ""
	}
	f := bmc.FRU{ID: 0, Name: ch.Name,
		Chassis: bmc.FRUChassis{Type: ch.ChassisType, PartNumber: keep(ch.PartNumber), Serial: keep(ch.SerialNumber)},
		Board: bmc.FRUBoard{Manufacturer: keep(ch.Manufacturer), Product: keep(ch.Model), Serial: keep(ch.SerialNumber),
			PartNumber: keep(ch.PartNumber)},
		Product: bmc.FRUProduct{Manufacturer: keep(sys.Manufacturer), Name: keep(sys.Model), PartNumber: keep(sys.PartNumber),
			Serial: keep(sys.SerialNumber), AssetTag: keep(sys.AssetTag), Version: keep(sys.SKU)},
	}
	if f.Product.AssetTag == "" {
		f.Product.AssetTag = keep(ch.AssetTag)
	}
	return []bmc.FRU{f}, nil
}

// ---- Inventory ----

// CPUs implements bmc.Inventory.
func (s *Session) CPUs(ctx context.Context) ([]bmc.CPU, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return nil, err
	}
	ps, err := s.C.Processors(ctx, sys)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.CPU, 0, len(ps))
	for _, p := range ps {
		if p.ProcessorType != "" && p.ProcessorType != "CPU" {
			continue
		}
		out = append(out, CPUFromProcessor(p))
	}
	return out, nil
}

// Memory implements bmc.Inventory.
func (s *Session) Memory(ctx context.Context) ([]bmc.MemoryModule, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return nil, err
	}
	ms, err := s.C.MemoryModules(ctx, sys)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.MemoryModule, 0, len(ms))
	for _, m := range ms {
		out = append(out, MemoryFromRedfish(m))
	}
	return out, nil
}

// PCIeDevices implements bmc.Inventory.
func (s *Session) PCIeDevices(ctx context.Context) ([]bmc.PCIeDevice, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return nil, err
	}
	var ch *rf.Chassis
	if s.chassisPath != "" {
		ch, _ = s.C.GetChassis(ctx, s.chassisPath)
	}
	devs, err := s.C.PCIeDevices(ctx, sys, ch)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.PCIeDevice, 0, len(devs))
	for i := range devs {
		var fn *rf.PCIeFunction
		if fns, err := s.C.PCIeFunctions(ctx, &devs[i]); err == nil && len(fns) > 0 {
			fn = &fns[0]
		}
		out = append(out, PCIeFromRedfish(devs[i], fn))
	}
	return out, nil
}

// Storage implements bmc.Inventory (Storage drives, else SimpleStorage).
func (s *Session) Storage(ctx context.Context) ([]bmc.StorageDevice, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return nil, err
	}
	if sys.Storage.ID != "" {
		drives, err := s.C.StorageDrives(ctx, sys)
		if err != nil {
			return nil, err
		}
		out := make([]bmc.StorageDevice, 0, len(drives))
		for _, d := range drives {
			out = append(out, StorageFromDrive(d))
		}
		return out, nil
	}
	ss, err := s.C.SimpleStorages(ctx, sys)
	if err != nil {
		return nil, err
	}
	var out []bmc.StorageDevice
	for _, st := range ss {
		for i, d := range st.Devices {
			out = append(out, bmc.StorageDevice{ID: st.ID + "." + strconv.Itoa(i), Name: d.Name, Manufacturer: d.Manufacturer, Model: d.Model,
				CapacityBytes: int64(fval(d.CapacityBytes)), Health: Health(d.Status)})
		}
	}
	return out, nil
}

// Baseboard implements bmc.Inventory from the Chassis and System.
func (s *Session) Baseboard(ctx context.Context) (bmc.Baseboard, error) {
	sys, err := s.system(ctx)
	if err != nil {
		return bmc.Baseboard{}, err
	}
	bb := bmc.Baseboard{BIOSVersion: sys.BiosVersion}
	if ch, err := s.chassis(ctx, "Baseboard"); err == nil {
		keep := func(v string) string {
			if usable(v) {
				return v
			}
			return ""
		}
		bb.Manufacturer, bb.Product, bb.Serial, bb.PartNumber, bb.Version = keep(ch.Manufacturer), keep(ch.Model),
			keep(ch.SerialNumber), keep(ch.PartNumber), keep(ch.Version)
	}
	if bb.Manufacturer == "" {
		bb.Manufacturer = sys.Manufacturer
	}
	if bb.Product == "" {
		s.mu.Lock()
		bb.Product = s.board
		s.mu.Unlock()
	}
	return bb, nil
}

// ---- NetworkManager ----

// NetworkInterfaces implements bmc.NetworkManager (manager NICs).
func (s *Session) NetworkInterfaces(ctx context.Context) ([]bmc.NetworkInterface, error) {
	mgr, err := s.manager(ctx, "NetworkInterfaces")
	if err != nil {
		return nil, err
	}
	ifs, err := s.C.EthernetInterfaces(ctx, mgr.EthernetInterfaces.ID)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.NetworkInterface, 0, len(ifs))
	for _, e := range ifs {
		out = append(out, NetworkFromEthernet(e))
	}
	return out, nil
}

// SetNetworkInterface implements bmc.NetworkManager with a PATCH of the
// named manager EthernetInterface (disruptive: the BMC may drop off the
// network). Only non-nil fields are sent.
func (s *Session) SetNetworkInterface(ctx context.Context, u bmc.NetworkInterfaceUpdate) error {
	if u.Name == "" {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetNetworkInterface", "interface name required")
	}
	mgr, err := s.manager(ctx, "SetNetworkInterface")
	if err != nil {
		return err
	}
	ifs, err := s.C.EthernetInterfaces(ctx, mgr.EthernetInterfaces.ID)
	if err != nil {
		return err
	}
	var target *rf.EthernetInterface
	for i := range ifs {
		if ifs[i].ID == u.Name || ifs[i].Name == u.Name {
			target = &ifs[i]
		}
	}
	if target == nil {
		return bmc.Errorf(bmc.ErrNotFound, Pkg, "SetNetworkInterface", "no interface %q", u.Name)
	}
	body := map[string]any{}
	if u.Enabled != nil {
		body["InterfaceEnabled"] = *u.Enabled
	}
	if u.IPv4 != nil {
		body["DHCPv4"] = map[string]any{"DHCPEnabled": u.IPv4.DHCP}
		if !u.IPv4.DHCP {
			body["IPv4StaticAddresses"] = []rf.IPv4Address{{Address: u.IPv4.Address, SubnetMask: u.IPv4.Netmask, Gateway: u.IPv4.Gateway}}
		}
	}
	if u.IPv6 != nil {
		mode := "Disabled"
		if u.IPv6.Enabled {
			mode = "Stateful"
			if !u.IPv6.DHCP {
				mode = "Disabled"
			}
		}
		body["DHCPv6"] = map[string]any{"OperatingMode": mode}
	}
	if u.VLAN != nil {
		body["VLAN"] = map[string]any{"VLANEnable": u.VLAN.Enabled, "VLANId": u.VLAN.ID}
	}
	if u.DNS != nil {
		body["StaticNameServers"] = u.DNS
	}
	if len(body) == 0 {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetNetworkInterface", "nothing to change")
	}
	_, err = s.C.Patch(ctx, target.ODataID, body)
	return err
}

// ---- UserReader / ServiceReader ----

// Users implements bmc.UserReader.
func (s *Session) Users(ctx context.Context) ([]bmc.User, error) {
	accts, err := s.C.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.User, 0, len(accts))
	for _, a := range accts {
		if a.UserName == "" {
			continue
		}
		out = append(out, UserFromAccount(a))
	}
	return out, nil
}

// Services implements bmc.ServiceReader from NetworkProtocol.
func (s *Session) Services(ctx context.Context) ([]bmc.Service, error) {
	mgr, err := s.manager(ctx, "Services")
	if err != nil {
		return nil, err
	}
	np, err := s.C.GetNetworkProtocol(ctx, mgr)
	if err != nil {
		return nil, err
	}
	return ServicesFromNetworkProtocol(np), nil
}

// ---- TimeManager ----

// Time implements bmc.TimeManager (Manager DateTime + NetworkProtocol NTP).
func (s *Session) Time(ctx context.Context) (bmc.TimeConfig, error) {
	start := s.opts.Now()
	mgr, err := s.manager(ctx, "Time")
	if err != nil {
		return bmc.TimeConfig{}, err
	}
	end := s.opts.Now()
	mid := start.Add(end.Sub(start) / 2)
	tc := bmc.TimeConfig{ProxyTime: mid.UTC(), Timezone: mgr.DateTimeLocalOffset, UTCOffsetMinutes: rf.OffsetMinutes(mgr.DateTimeLocalOffset)}
	if bt, err := rf.ParseTime(mgr.DateTime); err == nil && !bt.IsZero() {
		tc.BMCTime = bt
		tc.OffsetSeconds = bt.Sub(mid).Truncate(time.Second).Seconds()
		s.mu.Lock()
		s.clockOffset = bt.Sub(mid).Truncate(time.Second)
		s.mu.Unlock()
	} else {
		return tc, bmc.Errorf(bmc.ErrUnsupported, Pkg, "Time", "manager has no DateTime")
	}
	if np, err := s.C.GetNetworkProtocol(ctx, mgr); err == nil && np.NTP != nil {
		tc.NTPEnabled = np.NTP.ProtocolEnabled != nil && *np.NTP.ProtocolEnabled
		for _, srv := range np.NTP.NTPServers {
			if srv != nil && *srv != "" {
				tc.NTPServers = append(tc.NTPServers, *srv)
			}
		}
	}
	return tc, nil
}

// SetTime implements bmc.TimeManager: Manager DateTime PATCH (keeping the
// BMC's DateTimeLocalOffset) and/or NetworkProtocol NTP PATCH.
func (s *Session) SetTime(ctx context.Context, u bmc.TimeUpdate) error {
	if u.Time == nil && u.NTPEnabled == nil && u.NTPServers == nil {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "SetTime", "nothing to change")
	}
	mgr, err := s.manager(ctx, "SetTime")
	if err != nil {
		return err
	}
	if u.NTPEnabled != nil || u.NTPServers != nil {
		if mgr.NetworkProtocol.ID == "" {
			return bmc.Unsupported(Pkg, "SetTime NTP")
		}
		if _, err := s.C.SetNTP(ctx, mgr.NetworkProtocol.ID, rf.NTPUpdate{ProtocolEnabled: u.NTPEnabled, NTPServers: u.NTPServers}); err != nil {
			return err
		}
	}
	if u.Time != nil {
		if _, err := s.C.SetDateTime(ctx, mgr, *u.Time); err != nil {
			return err
		}
	}
	return nil
}

// SyncTime implements bmc.TimeManager: sets the BMC clock to the proxy's
// current UTC and returns the resulting configuration.
func (s *Session) SyncTime(ctx context.Context) (bmc.TimeConfig, error) {
	now := s.opts.Now().UTC()
	if err := s.SetTime(ctx, bmc.TimeUpdate{Time: &now}); err != nil {
		return bmc.TimeConfig{}, err
	}
	return s.Time(ctx)
}

// ---- FirmwareInventory ----

// Firmware implements bmc.FirmwareInventory.
func (s *Session) Firmware(ctx context.Context) ([]bmc.FirmwareComponent, error) {
	inv, err := s.C.FirmwareInventory(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.FirmwareComponent, 0, len(inv))
	for _, i := range inv {
		out = append(out, FirmwareFromSoftware(i))
	}
	return out, nil
}

// ---- VirtualMedia ----

// VirtualMedia implements bmc.VirtualMedia.
func (s *Session) VirtualMedia(ctx context.Context) ([]bmc.VirtualMediaSlot, error) {
	if s.vmPath == "" {
		return nil, bmc.Unsupported(Pkg, "VirtualMedia")
	}
	slots, err := s.C.VirtualMediaSlots(ctx, s.vmPath)
	if err != nil {
		return nil, err
	}
	out := make([]bmc.VirtualMediaSlot, 0, len(slots))
	for _, v := range slots {
		out = append(out, VirtualMediaFromRedfish(v))
	}
	return out, nil
}

func (s *Session) vmSlot(ctx context.Context, id string) (*rf.VirtualMedia, error) {
	if s.vmPath == "" {
		return nil, bmc.Unsupported(Pkg, "VirtualMedia")
	}
	if id == "" {
		return nil, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "VirtualMedia", "slot id required")
	}
	var vm rf.VirtualMedia
	resp, err := s.C.Get(ctx, strings.TrimRight(s.vmPath, "/")+"/"+id, &vm)
	if err != nil {
		if resp != nil && resp.Status == http.StatusNotFound {
			return nil, bmc.Errorf(bmc.ErrNotFound, Pkg, "VirtualMedia", "no slot %q", id)
		}
		return nil, err
	}
	return &vm, nil
}

// InsertMedia implements bmc.VirtualMedia.
func (s *Session) InsertMedia(ctx context.Context, r bmc.VirtualMediaInsert) error {
	if r.Image == "" {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "InsertMedia", "image required")
	}
	vm, err := s.vmSlot(ctx, r.SlotID)
	if err != nil {
		return err
	}
	t, wp := true, r.WriteProtected
	_, err = s.C.InsertMedia(ctx, vm, rf.InsertMediaRequest{Image: r.Image, Inserted: &t, WriteProtected: &wp,
		TransferProtocolType: r.TransferProtocol, UserName: r.UserName, Password: r.Password})
	return err
}

// EjectMedia implements bmc.VirtualMedia.
func (s *Session) EjectMedia(ctx context.Context, slotID string) error {
	vm, err := s.vmSlot(ctx, slotID)
	if err != nil {
		return err
	}
	_, err = s.C.EjectMedia(ctx, vm)
	return err
}

// ---- BMCReset ----

// ResetBMC implements bmc.BMCReset (Manager.Reset: cold -> ForceRestart,
// warm -> GracefulRestart), honouring the advertised allowable values.
func (s *Session) ResetBMC(ctx context.Context, kind bmc.BMCResetKind) error {
	mgr, err := s.manager(ctx, "ResetBMC")
	if err != nil {
		return err
	}
	var want string
	switch kind {
	case bmc.BMCResetCold, "":
		want = "ForceRestart"
	case bmc.BMCResetWarm:
		want = "GracefulRestart"
	default:
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "ResetBMC", "unknown reset kind %q", kind)
	}
	if allowed := s.C.AllowableValues(ctx, mgr.Actions, "#Manager.Reset", "ResetType"); len(allowed) > 0 && !slices.Contains(allowed, want) {
		return bmc.Errorf(bmc.ErrUnsupported, Pkg, "ResetBMC", "%s not allowed (allowable: %s)", want, strings.Join(allowed, ","))
	}
	_, err = s.C.ResetManager(ctx, mgr, want)
	return err
}
