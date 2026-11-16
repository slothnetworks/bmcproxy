package asrock

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
)

// amiEvent is one SEL record of GET /api/logs/event?LASTEVENTID=n (and of
// logs/eventlog). B650/B550 append a trailer element {"event_log_count":N}
// that has no id: it is skipped.
type amiEvent struct {
	ID               *amimegarac.NullableInt `json:"id"`
	RecordType       amimegarac.FlexString   `json:"record_type"`
	Timestamp        amimegarac.NullableInt  `json:"timestamp"`
	SensorType       amimegarac.FlexString   `json:"sensor_type"`
	SensorName       amimegarac.FlexString   `json:"sensor_name"`
	SensorNumber     amimegarac.NullableInt  `json:"sensor_number"`
	GeneratorType    amimegarac.FlexString   `json:"generator_type"`
	EventDirection   amimegarac.FlexString   `json:"event_direction"`
	EventReading     amimegarac.FlexString   `json:"event_reading_class"`
	EventDescription amimegarac.FlexString   `json:"event_description"`
	EventData1       amimegarac.NullableInt  `json:"EventData1"`
	EventData2       amimegarac.NullableInt  `json:"EventData2"`
	EventData3       amimegarac.NullableInt  `json:"EventData3"`
	ED1Desc          amimegarac.FlexString   `json:"EventData1_Description"`
	ED2Desc          amimegarac.FlexString   `json:"EventData2_Description"`
	ED3Desc          amimegarac.FlexString   `json:"EventData3_Description"`
	EventLogCount    *amimegarac.NullableInt `json:"event_log_count"`
}

func eventSeverity(desc string) bmc.Health {
	d := strings.ToLower(strings.ReplaceAll(desc, "-", " "))
	d = strings.ReplaceAll(d, "_", " ")
	switch {
	case strings.Contains(d, "non recoverable"), strings.Contains(d, "critical") && !strings.Contains(d, "non critical"):
		return bmc.HealthCritical
	case strings.Contains(d, "non critical"), strings.Contains(d, "fail"), strings.Contains(d, "lost"):
		return bmc.HealthWarning
	}
	return bmc.HealthOK
}

func selFromAMI(e amiEvent, off time.Duration) bmc.SELEntry {
	msg := e.ED1Desc.Clean()
	if msg == "" {
		msg = e.EventDescription.Clean()
	}
	if d2 := e.ED2Desc.Clean(); d2 != "" {
		msg += "; " + d2
	}
	if d3 := e.ED3Desc.Clean(); d3 != "" {
		msg += "; " + d3
	}
	dir := strings.ToLower(e.EventDirection.Clean())
	switch dir {
	case "asserted", "assertion":
		dir = "assertion"
	case "deasserted", "deassertion":
		dir = "deassertion"
	}
	en := bmc.SELEntry{
		ID:         strconv.FormatInt(e.ID.V, 10),
		Severity:   eventSeverity(e.EventDescription.Clean() + " " + e.ED1Desc.Clean()),
		SensorType: e.SensorType.Clean(),
		SensorName: e.SensorName.Clean(),
		EventType:  e.EventReading.Clean(),
		Direction:  dir,
		Message:    msg,
		EventData:  fmt.Sprintf("%02x%02x%02x", byte(e.EventData1.V), byte(e.EventData2.V), byte(e.EventData3.V)),
		Generator:  e.GeneratorType.Clean(),
		Backend:    bmc.BackendAMI,
	}
	if e.SensorNumber.Valid {
		n := int(e.SensorNumber.V)
		en.SensorNumber = &n
	}
	if e.Timestamp.Valid && e.Timestamp.V > 0 {
		en.Time = bmc.NewTimestamp(time.Unix(e.Timestamp.V, 0).UTC(), off)
	}
	if desc := e.EventDescription.Clean(); desc != "" {
		en.Oem = map[string]any{"eventDescription": desc}
	}
	return en
}

// parseSince validates an AMI record id cursor.
func parseSince(op, since string) (int64, error) {
	since = strings.TrimSpace(since)
	if since == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(since, 10, 64)
	if err != nil || n < 0 {
		return 0, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, op, "since must be a non-negative record id, got %q", since)
	}
	return n, nil
}

// Events implements bmc.EventLog. AMI: the incremental
// GET /api/logs/event?LASTEVENTID=n (ids > n), never the 589 KB / 4.7 s full
// logs/eventlog dump. Entries are returned oldest first; LastID is the
// cursor for the next call.
func (s *Session) Events(ctx context.Context, q bmc.EventQuery) (bmc.EventPage, error) {
	switch s.backend(bmc.CapEventLog) {
	case bmc.BackendRedfish:
		return s.Session.Events(ctx, q)
	case bmc.BackendAMI:
		since, err := parseSince("Events", q.SinceID)
		if err != nil {
			return bmc.EventPage{}, err
		}
		var raw []amiEvent
		path := "logs/event?" + url.Values{"LASTEVENTID": {strconv.FormatInt(since, 10)}}.Encode()
		if err := s.api.Get(ctx, path, &raw); err != nil {
			return bmc.EventPage{}, err
		}
		off := s.ClockOffset()
		entries := make([]amiEvent, 0, len(raw))
		for _, e := range raw {
			if e.ID == nil || !e.ID.Valid {
				continue // B650 trailer {"event_log_count": N}
			}
			if e.ID.V <= since {
				continue // defensive: firmware that ignores LASTEVENTID
			}
			entries = append(entries, e)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].ID.V < entries[j].ID.V })
		page := bmc.EventPage{Entries: []bmc.SELEntry{}, LastID: q.SinceID}
		if q.Limit > 0 && len(entries) > q.Limit {
			entries, page.More = entries[:q.Limit], true
		}
		for _, e := range entries {
			page.Entries = append(page.Entries, selFromAMI(e, off))
		}
		if n := len(page.Entries); n > 0 {
			page.LastID = page.Entries[n-1].ID
		}
		return page, nil
	}
	return bmc.EventPage{}, s.unsupported("Events")
}

// ClearEvents implements bmc.EventLogClearer (disruptive). AMI: DELETE
// /api/logs/event.
func (s *Session) ClearEvents(ctx context.Context) error {
	switch s.backend(bmc.CapEventLogClear) {
	case bmc.BackendRedfish:
		return s.Session.ClearEvents(ctx)
	case bmc.BackendAMI:
		return s.api.DoJSON(ctx, http.MethodDelete, "logs/event", nil, nil)
	}
	return s.unsupported("ClearEvents")
}

type amiAudit struct {
	ID         *amimegarac.NullableInt `json:"id"`
	Timestamp  amimegarac.NullableInt  `json:"timestamp"`
	Hostname   amimegarac.FlexString   `json:"hostname"`
	Message    amimegarac.FlexString   `json:"message"`
	AuditCount *amimegarac.NullableInt `json:"audit_count"`
}

var (
	reAuditIP   = regexp.MustCompile(`(?i)\bIP:\s*([0-9A-Fa-f:.]+)`)
	reAuditUser = regexp.MustCompile(`(?i)\buser:\s*([^\s-][^\s]*)`)
	reAuditProc = regexp.MustCompile(`^([A-Za-z0-9_.-]+):`)
)

// AuditLog implements bmc.AuditLog. AMI: GET /api/logs/audit
// [{id, timestamp, hostname, message}]; user and source IP are parsed from
// the syslog message ("... Login from IP:10.0.0.1 user:admin").
func (s *Session) AuditLog(ctx context.Context, q bmc.AuditQuery) ([]bmc.AuditEntry, error) {
	switch s.backend(bmc.CapAuditLog) {
	case bmc.BackendRedfish:
		return s.Session.AuditLog(ctx, q)
	case bmc.BackendAMI:
		since, err := parseSince("AuditLog", q.SinceID)
		if err != nil {
			return nil, err
		}
		var raw []amiAudit
		if err := s.api.Get(ctx, "logs/audit", &raw); err != nil {
			return nil, err
		}
		off := s.ClockOffset()
		out := []bmc.AuditEntry{}
		sort.Slice(raw, func(i, j int) bool { return idOf(raw[i].ID) < idOf(raw[j].ID) })
		for _, a := range raw {
			if a.ID == nil || !a.ID.Valid || a.ID.V <= since {
				continue // ?cmd=1 style {"audit_count":N} rows, or older than the cursor
			}
			msg := a.Message.Clean()
			e := bmc.AuditEntry{ID: strconv.FormatInt(a.ID.V, 10), Message: msg, Severity: bmc.HealthOK}
			if a.Timestamp.Valid && a.Timestamp.V > 0 {
				e.Time = bmc.NewTimestamp(time.Unix(a.Timestamp.V, 0).UTC(), off)
			}
			if m := reAuditIP.FindStringSubmatch(msg); m != nil {
				e.Source = m[1]
			}
			if m := reAuditUser.FindStringSubmatch(msg); m != nil {
				e.User = strings.TrimRight(m[1], " -")
			}
			if m := reAuditProc.FindStringSubmatch(msg); m != nil {
				e.Facility = m[1]
			}
			if h := a.Hostname.Clean(); h != "" {
				e.Oem = map[string]any{"hostname": h}
			}
			out = append(out, e)
			if q.Limit > 0 && len(out) >= q.Limit {
				break
			}
		}
		return out, nil
	}
	return nil, s.unsupported("AuditLog")
}

func idOf(n *amimegarac.NullableInt) int64 {
	if n == nil {
		return -1
	}
	return n.V
}
