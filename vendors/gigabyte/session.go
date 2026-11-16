package gigabyte

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
	rf "github.com/slothnetworks/bmcproxy/pkg/protocol/redfish"
	"github.com/slothnetworks/bmcproxy/pkg/transport"
	genericredfish "github.com/slothnetworks/bmcproxy/vendors/generic/redfish"
	"github.com/slothnetworks/bmcproxy/vendors/gigabyte/boards"
)

// Session is the Gigabyte bmc.Session: the generic Redfish session (every
// capability it discovered is served from Redfish) plus Gigabyte
// normalisation overrides and a lazily logged-in AMI /api client for the
// vendor routes. Safe for concurrent use.
type Session struct {
	*genericredfish.Session

	d     *Driver
	t     bmc.Target
	creds bmc.Creds
	prof  boards.Profile
	id    identity

	ami *amimegarac.Client // nil when the profile has no AMI stack

	closeOnce sync.Once
	closeErr  error
}

// Compile-time checks for the overridden capabilities.
var (
	_ bmc.Session     = (*Session)(nil)
	_ bmc.EventLog    = (*Session)(nil)
	_ bmc.FRUReader   = (*Session)(nil)
	_ bmc.Inventory   = (*Session)(nil)
	_ bmc.BondManager = (*Session)(nil)
)

func (d *Driver) openSession(ctx context.Context, t bmc.Target, c bmc.Creds, p boards.Profile, id identity) (*Session, error) {
	cl, err := d.client(t, c, rf.AuthBasic, p.Redfish)
	if err != nil {
		return nil, err
	}
	gs, err := genericredfish.NewSession(ctx, cl, genericredfish.SessionOptions{
		DriverName: Name, Target: t, Board: id.Board, Firmware: id.Firmware, CloseClient: true, Now: d.opts.Now,
	})
	if err != nil {
		_ = cl.Close(ctx)
		return nil, err
	}
	s := &Session{Session: gs, d: d, t: t, creds: c, prof: p, id: id}
	if p.AMI {
		cfg := d.opts.AMIConfig
		cfg.Endpoint = d.Transport().Endpoint(t.Addr(), transport.LaneAMI)
		cfg.Target, cfg.Creds = t, c
		cfg.ExtraDeny = append(append([]amimegarac.DenyRule(nil), cfg.ExtraDeny...), p.ExtraDeny...)
		cfg.ReportSessionLost = true
		if cfg.Now == nil {
			cfg.Now = d.opts.Now
		}
		ami, err := amimegarac.New(cfg)
		if err != nil {
			_ = gs.Close(ctx)
			return nil, err
		}
		s.ami = ami
	}
	d.log(ctx).DebugContext(ctx, "gigabyte session opened", "bmc", t.Addr(), "user", c.User, "board", id.Board,
		"firmware", id.Firmware, "profile", p.Key, "verified", p.Verified)
	return s, nil
}

// Profile returns the board profile in use.
func (s *Session) Profile() boards.Profile { return s.prof }

// RedfishClient returns the session's Redfish client.
func (s *Session) RedfishClient() *rf.Client { return s.C }

// AMI returns the lazily-logged-in AMI /api client, or nil when the board
// has no MegaRAC /api.
func (s *Session) AMI() *amimegarac.Client { return s.ami }

// Info implements bmc.Session.
func (s *Session) Info() bmc.SessionInfo {
	in := s.Session.Info()
	in.Driver = Name
	if s.ami != nil && in.Capabilities != nil {
		caps := make(map[bmc.Capability]string, len(in.Capabilities)+1)
		for k, v := range in.Capabilities {
			caps[k] = v
		}
		// NIC bonding is the AMI settings/network-bond handler.
		caps[bmc.CapBond] = bmc.BackendAMI
		if _, ok := caps[bmc.CapEventLog]; ok && s.prof.SELSource == boards.SELAMI {
			// Events reads the decoded SEL from /api/logs/event (Redfish
			// only when the /api does not serve it): report that backend.
			caps[bmc.CapEventLog] = bmc.BackendAMI
		}
		in.Capabilities = caps
	}
	if s.ami != nil {
		st := bmc.BackendStatus{Name: bmc.BackendAMI, Available: true, Detail: "lazy: logged in on first vendor-route use"}
		if _, ok := s.ami.Session(); ok {
			st.Authenticated, st.Detail = true, "logged in"
		}
		if lost, why := s.ami.HostLost(); lost {
			st.Detail = "backend restarted: " + why
		}
		in.Backends = append(in.Backends, st)
	}
	var deg []string
	if in.Degraded != "" {
		deg = append(deg, in.Degraded)
	}
	switch {
	case s.prof.Key == "unknown":
		deg = append(deg, "unknown Gigabyte board/firmware: capability probing")
	case !s.prof.Verified:
		deg = append(deg, "firmware "+s.prof.Firmware+" not verified for "+s.prof.Key)
	}
	in.Degraded = strings.Join(deg, "; ")
	return in
}

// Close logs out of the AMI /api (when it was used) and closes the Redfish
// client. Idempotent.
func (s *Session) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		var errs []error
		if s.ami != nil {
			if _, ok := s.ami.Session(); ok {
				if err := s.ami.Logout(ctx); err != nil {
					errs = append(errs, err)
				}
			}
		}
		if err := s.Session.Close(ctx); err != nil {
			errs = append(errs, err)
		}
		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}

// ---- EventLog ----

// selMsgTime matches the "Timestamp : 2025-03-25T16:07:47+00:00" field the
// AMI 13.x sync agent writes into SEL LogEntry.Message.
var selMsgTime = regexp.MustCompile(`Timestamp\s*:\s*([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}(?:Z|[+-][0-9]{2}:[0-9]{2})?)`)

// SELEventTime returns the real event time of a Gigabyte SEL entry: the
// Message "Timestamp :" field, else EventTimestamp with its offset dropped
// (13.04.15 writes garbage offsets such as "+00:05"; the BMC clock is UTC).
// ok is false when neither parses.
func SELEventTime(message, eventTimestamp string) (time.Time, bool) {
	if m := selMsgTime.FindStringSubmatch(message); m != nil {
		if t, err := rf.ParseTime(m[1]); err == nil && !t.IsZero() {
			return t.UTC(), true
		}
	}
	ts := strings.TrimSpace(eventTimestamp)
	if len(ts) >= 19 {
		if t, err := time.Parse("2006-01-02T15:04:05", ts[:19]); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// Events implements bmc.EventLog: the generic Redfish SEL read (paged past
// 50 by the profile's SkipPageSize quirk) with the event time taken from
// the event itself on boards whose LogEntry.Created is the import time.
func (s *Session) Events(ctx context.Context, q bmc.EventQuery) (bmc.EventPage, error) {
	if s.prof.SELSource == boards.SELAMI && s.ami != nil {
		page, err := s.amiEvents(ctx, q)
		if err == nil || !errors.Is(err, bmc.ErrUnsupported) && !errors.Is(err, bmc.ErrNotFound) {
			return page, err
		}
		s.d.log(ctx).DebugContext(ctx, "gigabyte: /api/logs/event unavailable, using Redfish SEL", "err", err)
	}
	page, err := s.Session.Events(ctx, q)
	if err != nil || s.prof.SELTime != boards.SELTimeEvent {
		return page, err
	}
	off := s.ClockOffset()
	for i := range page.Entries {
		e := &page.Entries[i]
		ets, _ := e.Oem["eventTimestamp"].(string)
		if t, ok := SELEventTime(e.Message, ets); ok {
			if e.Oem == nil {
				e.Oem = map[string]any{}
			}
			e.Oem["created"] = e.Time.Raw.UTC().Format(time.RFC3339)
			e.Time = bmc.NewTimestamp(t, off)
		}
	}
	return page, nil
}

// ---- FRU / Baseboard ----

// boardSerial reads Chassis Oem.GBTChassisOemProperty."Board Serial Number".
func (s *Session) boardSerial(ctx context.Context) string {
	if !s.prof.BoardSerialOem {
		return ""
	}
	_, _, chassis, _, _ := s.Paths()
	if chassis == "" {
		return ""
	}
	var ch struct {
		Oem map[string]map[string]any `json:"Oem"`
	}
	if _, err := s.C.Get(ctx, chassis, &ch); err != nil {
		return ""
	}
	for k, v := range ch.Oem {
		if !strings.HasPrefix(strings.ToLower(k), "gbt") {
			continue
		}
		for f, x := range v {
			if strings.EqualFold(strings.ReplaceAll(f, " ", ""), "BoardSerialNumber") {
				if str, ok := x.(string); ok && usable(str) {
					return strings.TrimSpace(str)
				}
			}
		}
	}
	return ""
}

func usable(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.EqualFold(s, "To be filled by O.E.M.") && !strings.EqualFold(s, "N/A") &&
		!strings.EqualFold(s, "Default string") && !strings.HasPrefix(s, "0123456789")
}

// FRU implements bmc.FRUReader: the generic Chassis/System FRU with the
// real baseboard serial from the Gigabyte chassis OEM block.
func (s *Session) FRU(ctx context.Context) ([]bmc.FRU, error) {
	frus, err := s.Session.FRU(ctx)
	if err != nil {
		return nil, err
	}
	if bs := s.boardSerial(ctx); bs != "" && len(frus) > 0 {
		frus[0].Board.Serial = bs
	}
	return frus, nil
}

// Baseboard implements bmc.Inventory with the Gigabyte board serial.
func (s *Session) Baseboard(ctx context.Context) (bmc.Baseboard, error) {
	bb, err := s.Session.Baseboard(ctx)
	if err != nil {
		return bb, err
	}
	if bs := s.boardSerial(ctx); bs != "" {
		bb.Serial = bs
	}
	return bb, nil
}
