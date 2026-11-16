package testsim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// Gigabyte (Giga Computing) AMI MegaRAC SP-X profiles.
//
// MC13-LE0 (BMC 13.04.15, docs/gigabyte/mc13-le0-live-probe.md): Redfish
// from vendors/gigabyte/testdata/mc13le0-13.04.15.json and /api from the
// live read-only probe digest (docs/gigabyte/mc13-le0-har-endpoints.json),
// both via cmd/extract. Quirks:
//
//   - Redfish collections stop at 50 members without a nextLink
//     (Members@odata.count is the page length) and $skip<1 answers 400;
//   - an unauthenticated $metadata declaring the GBT*/Gbt* namespaces;
//   - Chassis/Self/Sensors lists only Energy and Power (the readings are in
//     Thermal/Power);
//   - SEL LogEntry.Created is the sync-agent import time, the real event
//     time is the "Timestamp :" field of Message;
//   - Chassis.SerialNumber is the "0123456789..." FRU placeholder, the real
//     board serial is Chassis Oem.GBTChassisOemProperty."Board Serial
//     Number" (GigabyteBoardSerial here);
//   - /api enforces the exact CSRF token on GETs too, and a missing or
//     wrong token logs the session out; there is no asrr namespace (404).
//
// MZ32-AR0 (BMC 12.61.39, docs/gigabyte/mz32-ar0-live-probe.md): Redfish
// from vendors/gigabyte/testdata/mz32ar0-12.61.39.json plus the
// SubProcessors crawl trap from docs/gigabyte/mz32-ar0-redfish-inventory.json.
// No raw /api capture exists, so the /api bodies are synthesized below
// from the field values in docs/gigabyte/mz32-ar0-frontend.md §4. Quirks:
//
//   - normal paging at 50 with a literal "?$skip=50" nextLink;
//   - query options are honoured only with a literal "$": percent-encoded
//     "%24skip" etc. are silently ignored (the full resource is returned);
//   - $filter is applied to the current page only (and the nextLink
//     drops it);
//   - an unknown "$" parameter answers 501 Base.1.0.QueryNotSupported;
//   - ResetType On/ForceOff/GracefulShutdown/ForceRestart only (no
//     PowerCycle, no Nmi), IndicatorLED only (no LocationIndicatorActive),
//     a Bios @Redfish.Settings stub;
//   - /api enforces CSRF on GETs (a wrong token is a 401, the session
//     survives) and serves the decoded SEL at logs/event?LASTEVENTID=n.

// Gigabyte boards.
const (
	BoardMC13LE0 Board = "mc13le0"
	BoardMZ32AR0 Board = "mz32ar0"
)

// GigabyteBoardSerial is the real-looking baseboard serial the Gigabyte
// sims put in Chassis Oem.GBTChassisOemProperty."Board Serial Number" (the
// capture redacts it); Chassis.SerialNumber keeps the FRU placeholder.
const GigabyteBoardSerial = "SIMBOARDSN0001"

// GigabytePlaceholderSerial is the FRU placeholder of Chassis.SerialNumber.
const GigabytePlaceholderSerial = "01234567890123456789AB"

// MC13LE0 is the Gigabyte MC13-LE0-000 on BMC 13.04.15.
func MC13LE0() Profile {
	return Profile{Board: BoardMC13LE0, Vendor: "gigabyte", BoardName: "MC13-LE0-000", Firmware: "13.04.15",
		CookieAttrs: "; secure;HttpOnly;Samesite=Lax", CSRFOnGET: true, CSRFKillsSession: true, LoginExtras: true,
		RedfishAuth: true, RedfishTree: true, NoASRR: true, RedfishTruncatedPaging: true, MaxSessions: 20,
		MetadataNamespaces: []string{"GBTChassisOemProperty.v1_0_0", "GBTFanprofileService.v1_0_0", "GBTSystemsOemProperty.v1_0_0",
			"GBTThermal.v1_0_0", "GbtGPUManager.v1_0_0", "GbtMemoryOemProperty.v1_0_0", "GbtPowerLimit.v1_0_0"}}
}

// MZ32AR0 is the Gigabyte MZ32-AR0-00 on BMC 12.61.39.
func MZ32AR0() Profile {
	return Profile{Board: BoardMZ32AR0, Vendor: "gigabyte", BoardName: "MZ32-AR0-00", Firmware: "12.61.39",
		CookieAttrs: "; secure;HttpOnly;Samesite=Lax", CSRFOnGET: true, LoginExtras: true,
		RedfishAuth: true, RedfishTree: true, NoASRR: true, SyntheticAPI: true,
		RedfishLiteralDollar: true, RedfishFilterPageLocal: true, RedfishStrictDollar: true, MaxSessions: 20,
		MetadataNamespaces: []string{"GBTChassisOemProperty.v1_0_0", "GBTFanprofileService.v1_0_0", "GBTThermal.v1_0_0"}}
}

// StartMC13LE0 starts a Gigabyte MC13-LE0 13.04.15 Sim.
func StartMC13LE0(t testing.TB, mod ...func(*Options)) *Sim { return startProfile(t, MC13LE0(), mod) }

// StartMZ32AR0 starts a Gigabyte MZ32-AR0 12.61.39 Sim.
func StartMZ32AR0(t testing.TB, mod ...func(*Options)) *Sim { return startProfile(t, MZ32AR0(), mod) }

// gigabyteRedfish applies the sim-side edits to a Gigabyte Redfish tree.
func gigabyteRedfish(r map[string]map[string]any) {
	if ch := r["/redfish/v1/Chassis/Self"]; ch != nil {
		if sn, _ := ch["SerialNumber"].(string); sn == "" || strings.HasPrefix(sn, "<REDACTED") {
			ch["SerialNumber"] = GigabytePlaceholderSerial
		}
		if oem, ok := ch["Oem"].(map[string]any); ok {
			if gbt, ok := oem["GBTChassisOemProperty"].(map[string]any); ok {
				gbt["Board Serial Number"] = GigabyteBoardSerial
			}
		}
	}
}

// ---- $metadata ----

// metadataXML is the unauthenticated $metadata of a Redfish-capable
// profile: stock AMI OEM namespaces plus the profile's (Gigabyte GBT*).
func (s *Sim) metadataXML() []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" +
		`<edmx:Edmx xmlns:edmx="http://docs.oasis-open.org/odata/ns/edmx" Version="4.0">` + "\n")
	for _, ns := range append([]string{"ServiceRoot.v1_5_0", "AMIServiceRoot.v1_0_0", "AMIManager.v1_0_0"}, s.profile.MetadataNamespaces...) {
		name := ns
		if i := strings.Index(ns, ".v"); i > 0 {
			name = ns[:i]
		}
		fmt.Fprintf(&b, ` <edmx:Reference Uri="/redfish/v1/schemas/%s.xml"><edmx:Include Namespace="%s"/></edmx:Reference>`+"\n", name, ns)
	}
	b.WriteString(` <edmx:DataServices><Schema xmlns="http://docs.oasis-open.org/odata/ns/edm" Namespace="Service">` +
		`<EntityContainer Name="Service" Extends="ServiceRoot.v1_5_0.ServiceContainer"/></Schema></edmx:DataServices>` + "\n</edmx:Edmx>\n")
	return b.Bytes()
}

// ---- SP-X 12.x query quirks ----

var sp12Known = map[string]bool{"$select": true, "$expand": true, "$filter": true, "$top": true, "$skip": true}

// gbtQuery rewrites r's query the way an AMI SP-X 12.x service reads it:
// percent-encoded "%24" options are dropped (ignored), an unknown "$"
// option is a 501, and $filter is taken out so it can be applied to the
// returned page only. handled is true when a response was written.
func (s *Sim) gbtQuery(w http.ResponseWriter, r *http.Request) (filter string, handled bool) {
	p := s.profile
	if r.URL.RawQuery == "" || !(p.RedfishLiteralDollar || p.RedfishStrictDollar || p.RedfishFilterPageLocal) {
		return "", false
	}
	var keep []string
	for _, part := range strings.Split(r.URL.RawQuery, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		if p.RedfishLiteralDollar && strings.HasPrefix(strings.ToLower(k), "%24") {
			continue // never decoded: silently ignored
		}
		if p.RedfishStrictDollar && strings.HasPrefix(k, "$") && !sp12Known[k] {
			writeJSON(w, http.StatusNotImplemented, map[string]any{"error": map[string]any{"code": "Base.1.0.QueryNotSupported",
				"message": "Querying is not supported by the implementation.",
				"@Message.ExtendedInfo": []any{map[string]any{"MessageId": "Base.1.0.QueryNotSupported",
					"Message": "Querying is not supported by the implementation.", "MessageArgs": []any{k}, "Severity": "Warning"}}}})
			return "", true
		}
		if p.RedfishFilterPageLocal && k == "$filter" {
			if dv, err := url.QueryUnescape(v); err == nil {
				filter = dv
			} else {
				filter = v
			}
			continue
		}
		keep = append(keep, part)
	}
	r.URL.RawQuery = strings.Join(keep, "&")
	r.RequestURI = r.URL.RequestURI()
	return filter, false
}

// serveFiltered runs the Redfish handler and applies filter to the
// Members of the returned page only (nextLink and count untouched).
func (s *Sim) serveFiltered(w http.ResponseWriter, r *http.Request, filter string) {
	rec := httptest.NewRecorder()
	s.rf.Config.Handler.ServeHTTP(rec, r)
	body := rec.Body.Bytes()
	if rec.Code == http.StatusOK {
		var m map[string]any
		if json.Unmarshal(body, &m) == nil {
			if ms, ok := m["Members"].([]any); ok {
				out := []any{}
				for _, x := range ms {
					mm, _ := x.(map[string]any)
					res := mm
					if len(mm) <= 1 {
						id, _ := mm["@odata.id"].(string)
						res = s.rf.Get(id)
					}
					if matchFilter(res, filter) {
						out = append(out, x)
					}
				}
				m["Members"] = out
				body, _ = json.Marshal(m)
			}
		}
	}
	for k, vs := range rec.Header() {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(rec.Code)
	_, _ = w.Write(body)
}

// matchFilter evaluates a single "Prop op value" comparison (eq, ne, gt,
// ge, lt, le; quoted string or number). Anything else matches.
func matchFilter(res map[string]any, expr string) bool {
	f := strings.Fields(expr)
	if len(f) < 3 || res == nil {
		return true
	}
	prop, op, lit := f[0], strings.ToLower(f[1]), strings.Join(f[2:], " ")
	got, ok := res[prop]
	if !ok {
		return false
	}
	var c int
	if strings.HasPrefix(lit, "'") {
		c = strings.Compare(fmt.Sprint(got), strings.Trim(lit, "'"))
	} else {
		want, err := strconv.ParseFloat(lit, 64)
		g, ok := got.(float64)
		if err != nil || !ok {
			return false
		}
		switch {
		case g < want:
			c = -1
		case g > want:
			c = 1
		}
	}
	switch op {
	case "eq":
		return c == 0
	case "ne":
		return c != 0
	case "gt":
		return c > 0
	case "ge":
		return c >= 0
	case "lt":
		return c < 0
	case "le":
		return c <= 0
	}
	return true
}

// ---- MZ32-AR0 synthetic /api ----

// mz32Events is the number of synthetic SEL records (the live SEL held
// 1,022; a smaller log keeps tests fast while still exceeding a Redfish
// page).
const mz32Events = 60

// mz32APIFixtures synthesizes the MZ32-AR0 12.61.39 /api bodies from the
// values documented in docs/gigabyte/mz32-ar0-frontend.md §2 and §4.
func mz32APIFixtures() *FixtureSet {
	ep := func(method, path string, v any) Endpoint {
		b, _ := json.Marshal(v)
		return Endpoint{Method: method, Path: path, Status: http.StatusOK, Response: b, Note: "synthesized from docs/gigabyte/mz32-ar0-frontend.md"}
	}
	events := make([]any, 0, mz32Events)
	for i := 1; i <= mz32Events; i++ {
		sev, dir, desc := "OK", "Asserted", "Log Area Reset/Cleared"
		typ, name, num, class := "Event Logging Disabled", "SEL", 225, "sensor-specific"
		if i > 1 {
			typ, name, num, class = "Temperature", "CPU0_TEMP", 1, "threshold"
			desc = "Upper Critical - going high"
			if i%2 == 0 {
				sev = "Critical"
			} else {
				dir = "Deasserted"
			}
		}
		events = append(events, map[string]any{"id": i, "record_type": "system event record", "timestamp": 1754000000 + i*600,
			"sensor_type": typ, "sensor_name": name, "sensor_number": num, "generator_type": "BMC", "event_direction": dir,
			"event_reading_class": class, "event_description": desc, "event_assertSeverity": sev})
	}
	return &FixtureSet{Board: string(BoardMZ32AR0), Source: "synthesized (no /api capture of the MZ32-AR0)",
		Sanitized: "synthetic values only; addresses are 192.0.2.0/24",
		Endpoints: []Endpoint{
			ep(http.MethodPost, "/api/session", map[string]any{"ok": 0, "privilege": 4, "extendedpriv": 259, "racsession_id": 1,
				"remote_addr": "192.0.2.250", "server_name": "192.0.2.68", "server_addr": "192.0.2.68", "HTTPSEnabled": 1,
				"CSRFToken": "<REDACTED_CSRF>", "channel": 1, "passwordStatus": 0}),
			ep(http.MethodDelete, "/api/session", map[string]any{"ok": 0}),
			ep(http.MethodGet, "/api/firmware-info", map[string]any{"device_id": 32, "revision": 129, "fw_rev_1": 12, "fw_rev_2": 97,
				"ipmi_version": 2, "mfg_id_0": 10, "mfg_id_1": 60, "mfg_id_2": 0, "prod_id": 4104, "aux": 39, "fw_ver": "12.61.39",
				"bios_ver": "", "date": "Jul 2 2025", "time": "11:14:05 CST", "active_image": 1, "sku_ver": "1.0"}),
			ep(http.MethodGet, "/api/chassis-status", map[string]any{"power_status": 1, "led_status": 0}),
			ep(http.MethodGet, "/api/status/uptime", map[string]any{"minutes_per_count": 60, "poh_counter_reading": 1992}),
			ep(http.MethodGet, "/api/settings/fanprofile/mode", map[string]any{"strMode": "NEW_PROFILE"}),
			ep(http.MethodGet, "/api/settings/date-time", map[string]any{"id": 1, "primary_ntp": "pool.ntp.org", "secondary_ntp": "time.nist.gov",
				"ntp_auto_date": 0, "timestamp": 0, "localized_timestamp": 0, "utc_minutes": 0, "timezone": "Etc/GMT"}),
			ep(http.MethodGet, "/api/system_inventory_gbt/bios_info", map[string]any{"strManufacturer": "GIGABYTE",
				"strProductName": "MZ32-AR0-00", "strReleaseDate": "06/23/2026", "strVersion": "M32"}),
			ep(http.MethodGet, "/api/logs/event", events),
		}}
}
