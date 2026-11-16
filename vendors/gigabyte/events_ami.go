package gigabyte

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/slothnetworks/bmcproxy/pkg/bmc"
	"github.com/slothnetworks/bmcproxy/pkg/protocol/amimegarac"
)

// AMI /api decoded SEL (profiles with SELSource SELAMI, MZ32-AR0).
//
// GET /api/logs/event?LASTEVENTID=n returns the records with id > n as
// decoded JSON ({id, record_type, timestamp, sensor_type, sensor_number,
// event_direction, event_reading_class, event_description, ...}: the
// classic SP-X shape, docs/gigabyte/mz32-ar0-frontend.md §4). Never the
// unfiltered logs/event (550 KB / 1.8 s on MZ32-AR0) when a cursor is known.
//
// TODO(refactor): amiEvent/selFromAMI/eventSeverity duplicate
// vendors/asrock/caps_logs.go; move them to pkg/protocol/amimegarac.

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
	AssertSeverity   amimegarac.FlexString   `json:"event_assertSeverity"`
	EventData1       amimegarac.NullableInt  `json:"EventData1"`
	EventData2       amimegarac.NullableInt  `json:"EventData2"`
	EventData3       amimegarac.NullableInt  `json:"EventData3"`
	ED1Desc          amimegarac.FlexString   `json:"EventData1_Description"`
	ED2Desc          amimegarac.FlexString   `json:"EventData2_Description"`
	ED3Desc          amimegarac.FlexString   `json:"EventData3_Description"`
}

func eventSeverity(desc string) bmc.Health {
	d := strings.ToLower(strings.NewReplacer("-", " ", "_", " ").Replace(desc))
	switch {
	case strings.Contains(d, "non recoverable"), strings.Contains(d, "critical") && !strings.Contains(d, "non critical"):
		return bmc.HealthCritical
	case strings.Contains(d, "non critical"), strings.Contains(d, "warning"), strings.Contains(d, "fail"), strings.Contains(d, "lost"):
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
		Severity:   eventSeverity(e.AssertSeverity.Clean() + " " + e.EventDescription.Clean() + " " + e.ED1Desc.Clean()),
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

func (s *Session) amiEvents(ctx context.Context, q bmc.EventQuery) (bmc.EventPage, error) {
	var since int64
	if v := strings.TrimSpace(q.SinceID); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return bmc.EventPage{}, bmc.Errorf(bmc.ErrInvalidArgument, Pkg, "Events", "since must be a non-negative record id, got %q", v)
		}
		since = n
	}
	var raw []amiEvent
	path := "logs/event?" + url.Values{"LASTEVENTID": {strconv.FormatInt(since, 10)}}.Encode()
	if err := s.ami.Get(ctx, path, &raw); err != nil {
		return bmc.EventPage{}, err
	}
	entries := make([]amiEvent, 0, len(raw))
	for _, e := range raw {
		if e.ID == nil || !e.ID.Valid || e.ID.V <= since {
			continue // trailer records / firmware ignoring LASTEVENTID
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID.V < entries[j].ID.V })
	page := bmc.EventPage{Entries: []bmc.SELEntry{}, LastID: q.SinceID}
	if q.Limit > 0 && len(entries) > q.Limit {
		entries, page.More = entries[:q.Limit], true
	}
	off := s.ClockOffset()
	for _, e := range entries {
		page.Entries = append(page.Entries, selFromAMI(e, off))
	}
	if n := len(page.Entries); n > 0 {
		page.LastID = page.Entries[n-1].ID
	}
	return page, nil
}

// BMCFirmwareInfo is the AMI /api firmware-info view (IPMI Get Device ID
// plus the dual-image indicator Redfish lacks).
type BMCFirmwareInfo struct {
	Version string `json:"version" example:"12.61.39"`
	Date    string `json:"date,omitempty" example:"Jul 2 2025"`
	Time    string `json:"time,omitempty" example:"11:14:05 CST"`
	// ActiveImage is the running image on dual-image BMCs (0 = not reported).
	ActiveImage int `json:"activeImage,omitempty" example:"1"`
	// ManufacturerID is the IPMI IANA number (15370 = Giga-Byte).
	ManufacturerID int `json:"manufacturerId,omitempty" example:"15370"`
	ProductID      int `json:"productId,omitempty" example:"4104"`
	DeviceID       int `json:"deviceId,omitempty" example:"32"`
	// Raw is the BMC body with nothing removed (it holds no secrets).
	Raw map[string]any `json:"raw,omitempty"`
}

// FirmwareInfo reads GET /api/firmware-info.
func (s *Session) FirmwareInfo(ctx context.Context) (BMCFirmwareInfo, error) {
	if s.ami == nil {
		return BMCFirmwareInfo{}, bmc.Unsupported(Pkg, "FirmwareInfo (no AMI /api on "+s.prof.Key+")")
	}
	var raw map[string]any
	if err := s.ami.Get(ctx, "firmware-info", &raw); err != nil {
		return BMCFirmwareInfo{}, err
	}
	out := BMCFirmwareInfo{Raw: raw}
	str := func(k string) string { v, _ := raw[k].(string); return strings.TrimSpace(v) }
	num := func(k string) int {
		switch v := raw[k].(type) {
		case float64:
			return int(v)
		case string:
			v = strings.TrimSpace(v)
			if n, err := strconv.ParseInt(v, 0, 64); err == nil {
				return int(n)
			}
		}
		return 0
	}
	out.Version, out.Date, out.Time = str("fw_ver"), str("date"), str("time")
	out.ActiveImage, out.ProductID, out.DeviceID = num("active_image"), num("prod_id"), num("device_id")
	out.ManufacturerID = mfgID(raw["mfg_id"])
	if out.ManufacturerID == 0 {
		if _, ok := raw["mfg_id_0"]; ok { // 13.04.15: mfg_id_0..2 bytes
			out.ManufacturerID = mfgID([]any{raw["mfg_id_0"], raw["mfg_id_1"], raw["mfg_id_2"]})
		}
	}
	return out, nil
}

// mfgID decodes mfg_id: a number, a "0x3C0A" string, or the 3 little-endian
// IANA bytes ([10, 60, 0] / ["0A","3C","00"]).
func mfgID(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(x), 0, 64); err == nil {
			return int(n)
		}
	case []any:
		n := 0
		for i := len(x) - 1; i >= 0; i-- {
			b := 0
			switch y := x[i].(type) {
			case float64:
				b = int(y)
			case string:
				if v, err := strconv.ParseInt(strings.TrimPrefix(strings.TrimSpace(y), "0x"), 16, 64); err == nil {
					b = int(v)
				}
			}
			n = n<<8 | (b & 0xff)
		}
		return n
	}
	return 0
}
