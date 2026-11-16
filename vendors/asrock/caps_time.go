package asrock

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
)

// dateTimePath is the AMI clock/NTP object. PUT must carry the whole object
// (X470 500s on a partial body) and keep timezone/utc_minutes untouched.
const dateTimePath = "settings/date-time"

// dateTimeReadOnly are settings/date-time keys the GET returns but the web
// UI never sends back (docs/api/legacy-api-endpoints.json: request_fields
// vs response_fields). The live X470 2.32 answered 500 to a PUT that echoed
// the GET object verbatim, id and localized_timestamp included
// (docs/x470/scripts/rt.py "date-time:write_same",
// docs/x470/endpoint-reference.json), so they are dropped from the PUT.
// UNVERIFIED: no PUT settings/date-time has succeeded on hardware yet; the
// UI's field set is the best evidence of an accepted body.
var dateTimeReadOnly = map[string]bool{
	"id": true, "localized_timestamp": true, "ntp_server_status": true, "interface_name": true,
}

// amiDateTime is one read of settings/date-time: the raw object (numbers
// preserved) plus the measured clock offset.
type amiDateTime struct {
	raw    map[string]any
	parsed amimegarac.DateTime
	offset time.Duration
	at     time.Time
}

func (s *Session) readDateTime(ctx context.Context) (amiDateTime, error) {
	now := s.d.opts.Now
	start := now()
	resp, err := s.api.Do(ctx, &amimegarac.Request{Method: http.MethodGet, Path: dateTimePath, Idempotent: true, Op: "Time"})
	if err != nil {
		return amiDateTime{}, err
	}
	end := now()
	if err := resp.Err(); err != nil {
		return amiDateTime{}, err
	}
	var out amiDateTime
	dec := json.NewDecoder(bytes.NewReader(resp.Body))
	dec.UseNumber()
	if err := dec.Decode(&out.raw); err != nil {
		return amiDateTime{}, bmc.Errorf(bmc.ErrUpstream, Pkg, "Time", "decode date-time: %v", err)
	}
	if err := resp.Decode(&out.parsed); err != nil {
		return amiDateTime{}, err
	}
	out.at = start.Add(end.Sub(start) / 2)
	if out.parsed.Timestamp.Valid && out.parsed.Timestamp.V > 0 {
		out.offset = time.Unix(out.parsed.Timestamp.V, 0).Sub(out.at).Truncate(time.Second)
	}
	return out, nil
}

// autoDateKey returns the NTP mode field present in the object: auto_date
// (X470) or ntp_auto_date (B650/B550), falling back to the profile.
func (s *Session) autoDateKey(raw map[string]any) string {
	for _, k := range []string{s.prof.AutoDateField, "ntp_auto_date", "auto_date"} {
		if k == "" {
			continue
		}
		if _, ok := raw[k]; ok {
			return k
		}
	}
	if s.prof.AutoDateField != "" {
		return s.prof.AutoDateField
	}
	return "auto_date"
}

func (s *Session) timeConfigFromAMI(dt amiDateTime) bmc.TimeConfig {
	p := dt.parsed
	tc := bmc.TimeConfig{
		ProxyTime:     dt.at.UTC(),
		OffsetSeconds: dt.offset.Seconds(),
		Timezone:      p.Timezone.Clean(),
	}
	if p.Timestamp.Valid && p.Timestamp.V > 0 {
		tc.BMCTime = time.Unix(p.Timestamp.V, 0).UTC()
	}
	if p.UTCMinutes.Valid {
		tc.UTCOffsetMinutes = int(p.UTCMinutes.V)
	}
	mode := p.AutoDate
	if s.autoDateKey(dt.raw) == "ntp_auto_date" {
		mode = p.NTPAutoDate
	}
	// auto_date/ntp_auto_date: 1 = NTP sync (X470 HAR: auto_date 1 with NTP
	// servers set; docs/x470/har-analysis.md), 0 = off (B550-2T HAR,
	// docs/b650/har-analysis.md). UNVERIFIED: the live B650 4.10 reports
	// ntp_auto_date 2, read as "servers configured, auto-sync off"
	// (docs/b650/live-probe.md §5); it is reported as NTP disabled, and the
	// proxy only ever writes 0 or 1.
	tc.NTPEnabled = mode.Valid && mode.V == 1
	for _, srv := range []string{p.PrimaryNTP.Clean(), p.SecondaryNTP.Clean()} {
		if srv != "" && !amimegarac.IsNA(srv) {
			tc.NTPServers = append(tc.NTPServers, srv)
		}
	}
	return tc
}

// Time implements bmc.TimeManager. AMI: GET settings/date-time (also
// refreshes the session clock offset used for SEL/audit timestamps).
func (s *Session) Time(ctx context.Context) (bmc.TimeConfig, error) {
	switch s.backend(bmc.CapTime) {
	case bmc.BackendRedfish:
		return s.Session.Time(ctx)
	case bmc.BackendAMI:
		dt, err := s.readDateTime(ctx)
		if err != nil {
			return bmc.TimeConfig{}, err
		}
		if dt.parsed.Timestamp.Valid && dt.parsed.Timestamp.V > 0 {
			s.setClockOffset(dt.offset)
		}
		return s.timeConfigFromAMI(dt), nil
	}
	return bmc.TimeConfig{}, s.unsupported("Time")
}

// SetTime implements bmc.TimeManager. AMI: read the full date-time object,
// change only timestamp/localized_timestamp, the NTP mode field and the NTP
// servers, and PUT the whole object back (timezone, utc_minutes, daylight
// and B650 ptp_* fields are preserved verbatim).
func (s *Session) SetTime(ctx context.Context, u bmc.TimeUpdate) error {
	const op = "SetTime"
	switch s.backend(bmc.CapTime) {
	case bmc.BackendRedfish:
		return s.Session.SetTime(ctx, u)
	case bmc.BackendAMI:
	default:
		return s.unsupported(op)
	}
	if u.Time == nil && u.NTPEnabled == nil && len(u.NTPServers) == 0 {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "nothing to change (time, ntpEnabled or ntpServers)")
	}
	if u.Time != nil && u.NTPEnabled != nil && *u.NTPEnabled {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "cannot set the clock manually while enabling NTP")
	}
	if len(u.NTPServers) > 2 {
		return bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "at most 2 NTP servers (primary, secondary)")
	}
	dt, err := s.readDateTime(ctx)
	if err != nil {
		return err
	}
	body := make(map[string]any, len(dt.raw))
	for k, v := range dt.raw {
		if dateTimeReadOnly[k] {
			continue
		}
		body[k] = v
	}
	key := s.autoDateKey(dt.raw)
	if u.NTPEnabled != nil {
		body[key] = b2i(*u.NTPEnabled)
	}
	if len(u.NTPServers) > 0 {
		body["primary_ntp"] = strings.TrimSpace(u.NTPServers[0])
		if len(u.NTPServers) > 1 {
			body["secondary_ntp"] = strings.TrimSpace(u.NTPServers[1])
		}
	}
	if u.Time != nil {
		if u.NTPEnabled == nil {
			body[key] = 0 // a manual clock set needs NTP off
		}
		ts := u.Time.UTC().Unix()
		body["timestamp"] = json.Number(strconv.FormatInt(ts, 10))
	}
	if err := s.api.DoJSON(ctx, http.MethodPut, dateTimePath, body, nil); err != nil {
		return err
	}
	if u.Time != nil {
		s.setClockOffset(0)
	}
	return nil
}

// SyncTime implements bmc.TimeManager: set the BMC clock to the proxy's UTC
// (NTP is switched off by the manual set) and return the new config.
func (s *Session) SyncTime(ctx context.Context) (bmc.TimeConfig, error) {
	switch s.backend(bmc.CapTime) {
	case bmc.BackendRedfish:
		return s.Session.SyncTime(ctx)
	case bmc.BackendAMI:
		now := s.d.opts.Now().UTC()
		if err := s.SetTime(ctx, bmc.TimeUpdate{Time: &now}); err != nil {
			return bmc.TimeConfig{}, err
		}
		return s.Time(ctx)
	}
	return bmc.TimeConfig{}, s.unsupported("SyncTime")
}
