package testsim

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var simBasic = map[string]string{"Authorization": "Basic YWRtaW46c2ltLVBhc3N3MHJk"} // admin:sim-Passw0rd

func gigabyteSims(t *testing.T, fn func(t *testing.T, s *Sim)) {
	t.Run("mc13le0", func(t *testing.T) { fn(t, StartMC13LE0(t)) })
	t.Run("mz32ar0", func(t *testing.T) { fn(t, StartMZ32AR0(t)) })
}

func TestGigabyteFixturesAreSanitized(t *testing.T) {
	private := regexp.MustCompile(`\b(10\.\d+\.\d+\.\d+|172\.(1[6-9]|2\d|3[01])\.\d+\.\d+|192\.168\.\d+\.\d+)\b`)
	mac := regexp.MustCompile(`\b([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}\b`)
	for _, f := range []string{"mc13le0/endpoints.json", "mc13le0/redfish.json", "mz32ar0/redfish.json"} {
		b, err := fixtureFS.ReadFile("fixtures/" + f)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if m := private.FindString(s); m != "" {
			t.Errorf("%s: private address %s leaked", f, m)
		}
		if m := mac.FindString(s); m != "" {
			t.Errorf("%s: MAC %s leaked", f, m)
		}
		for _, key := range []string{"CSRFToken", "password", "SerialNumber", "UUID", "MACAddress", "PermanentMACAddress"} {
			re := regexp.MustCompile(`"` + key + `":\s*"([^"]*)"`)
			for _, m := range re.FindAllStringSubmatch(s, -1) {
				v := m[1]
				if v != "" && v != " " && v != "NA" && v != "N/A" && v != "Unknown" && !strings.HasPrefix(v, "<REDACTED") && !strings.HasPrefix(v, "0123456789") {
					t.Errorf("%s: %s=%q not a placeholder", f, key, v)
				}
			}
		}
	}
}

func TestGigabyteCommon(t *testing.T) {
	gigabyteSims(t, func(t *testing.T, s *Sim) {
		// Unauthenticated $metadata with the GBT namespaces.
		st, h, b := do(t, s, "GET", "/redfish/v1/$metadata", "", nil, nil)
		if st != 200 || !strings.Contains(string(b), `Namespace="GBT`) || !strings.Contains(h.Get("Content-Type"), "xml") {
			t.Fatalf("$metadata %d %.200s", st, b)
		}
		// Sensors collection: Energy + Power only.
		st, _, b = do(t, s, "GET", "/redfish/v1/Chassis/Self/Sensors", "", nil, simBasic)
		var coll struct{ Members []map[string]any }
		_ = json.Unmarshal(b, &coll)
		if st != 200 || len(coll.Members) != 2 {
			t.Fatalf("sensors %d %s", st, b)
		}
		// FRU placeholder in Chassis.SerialNumber, real serial in the GBT OEM block.
		ch := s.Redfish().Get("/redfish/v1/Chassis/Self")
		if ch["SerialNumber"] != GigabytePlaceholderSerial ||
			ch["Oem"].(map[string]any)["GBTChassisOemProperty"].(map[string]any)["Board Serial Number"] != GigabyteBoardSerial {
			t.Fatalf("chassis serials: %v / %v", ch["SerialNumber"], ch["Oem"])
		}
		// /api: no asrr namespace; CSRF enforced on GETs.
		sess := login(t, s)
		if st, _, _ := do(t, s, "GET", "/api/asrr/model-name", "", sess, nil); st != 404 {
			t.Fatalf("asrr/model-name %d", st)
		}
		if st, _, b := do(t, s, "GET", "/api/settings/fanprofile/mode", "", sess, nil); st != 200 || !strings.Contains(string(b), "strMode") {
			t.Fatalf("fanprofile/mode %d %s", st, b)
		}
		if st, _, b := do(t, s, "GET", "/api/firmware-info", "", sess, nil); st != 200 || !strings.Contains(string(b), s.Profile().Firmware) {
			t.Fatalf("firmware-info %d %s", st, b)
		}
		bad := &amiSess{cookie: sess.cookie, csrf: "wrong"}
		if st, _, b := do(t, s, "GET", "/api/chassis-status", "", bad, nil); st != 401 || !isCC7(b) {
			t.Fatalf("bad CSRF on GET: %d %s", st, b)
		}
		if s.Stats().CSRFRejects != 1 {
			t.Fatal("CSRF reject not counted")
		}
		// MC13-LE0: the bad token killed the session; MZ32-AR0: it survived.
		st, _, _ = do(t, s, "GET", "/api/chassis-status", "", sess, nil)
		if want := map[Board]int{BoardMC13LE0: 401, BoardMZ32AR0: 200}[s.Profile().Board]; st != want {
			t.Fatalf("after bad token: %d want %d", st, want)
		}
	})
}

func TestMC13LE0Paging(t *testing.T) {
	s := StartMC13LE0(t)
	const sel = "/redfish/v1/Managers/Self/LogServices/SEL/Entries"
	st, _, b := do(t, s, "GET", sel, "", nil, simBasic)
	var page struct {
		Members  []map[string]any
		Count    int    `json:"Members@odata.count"`
		NextLink string `json:"Members@odata.nextLink"`
	}
	_ = json.Unmarshal(b, &page)
	if st != 200 || len(page.Members) != 50 || page.Count != 50 || page.NextLink != "" {
		t.Fatalf("truncated page: %d members=%d count=%d next=%q", st, len(page.Members), page.Count, page.NextLink)
	}
	// SEL Created is the import time; the real time is in Message.
	if m, _ := page.Members[0]["Message"].(string); !strings.Contains(m, "Timestamp : ") {
		t.Fatalf("SEL message: %v", page.Members[0])
	}
	if st, _, _ := do(t, s, "GET", sel+"?$skip=0", "", nil, simBasic); st != 400 {
		t.Fatalf("$skip=0 = %d, want 400", st)
	}
	page.Members = nil
	_, _, b = do(t, s, "GET", sel+"?$skip=100", "", nil, simBasic)
	_ = json.Unmarshal(b, &page)
	if len(page.Members) != 50 {
		t.Fatalf("$skip=100: %d members", len(page.Members))
	}
}

func TestMZ32AR0Queries(t *testing.T) {
	s := StartMZ32AR0(t)
	const sel = "/redfish/v1/Managers/Self/LogServices/SEL/Entries"
	type page struct {
		Members  []map[string]any
		Count    int    `json:"Members@odata.count"`
		NextLink string `json:"Members@odata.nextLink"`
	}
	get := func(q string) (int, page) {
		st, _, b := do(t, s, "GET", sel+q, "", nil, simBasic)
		var p page
		_ = json.Unmarshal(b, &p)
		return st, p
	}
	st, p := get("")
	if st != 200 || len(p.Members) != 50 || p.Count != 57 || p.NextLink != sel+"?$skip=50" {
		t.Fatalf("page 1: %d %d %d %q", st, len(p.Members), p.Count, p.NextLink)
	}
	if _, p = get("?$skip=50"); len(p.Members) != 7 || p.NextLink != "" {
		t.Fatalf("page 2: %d %q", len(p.Members), p.NextLink)
	}
	// Percent-encoded options are ignored: the first page again.
	if _, p = get("?%24skip=50"); len(p.Members) != 50 {
		t.Fatalf("%%24skip honoured: %d members", len(p.Members))
	}
	// Unknown $ option: 501.
	if st, _ = get("?$orderby=Id"); st != 501 {
		t.Fatalf("$orderby = %d", st)
	}
	// $filter: page-local (only the first page is filtered; the nextLink stays).
	_, p = get("?$filter=" + strings.ReplaceAll("Id eq '1'", " ", "%20"))
	if len(p.Members) != 1 || p.NextLink == "" || strings.Contains(p.NextLink, "filter") {
		t.Fatalf("$filter page-local: %d members, next %q", len(p.Members), p.NextLink)
	}
	// ResetType limited; Bios settings stub; SubProcessors trap present.
	if st, _, _ := do(t, s, "POST", "/redfish/v1/Systems/Self/Actions/ComputerSystem.Reset", `{"ResetType":"PowerCycle"}`, nil, simBasic); st != 400 {
		t.Fatalf("PowerCycle = %d, want 400", st)
	}
	if _, ok := s.Redfish().Get("/redfish/v1/Systems/Self/Bios")["@Redfish.Settings"]; !ok {
		t.Fatal("no Bios @Redfish.Settings stub")
	}
	if st, _, _ := do(t, s, "GET", "/redfish/v1/Systems/Self/Processors/CPU0/SubProcessors", "", nil, simBasic); st != 200 {
		t.Fatalf("SubProcessors = %d", st)
	}
	// Decoded SEL over /api with LASTEVENTID.
	sess := login(t, s)
	st, _, b := do(t, s, "GET", "/api/logs/event?LASTEVENTID=58", "", sess, nil)
	var evs []map[string]any
	_ = json.Unmarshal(b, &evs)
	if st != 200 || len(evs) != 2 || s.EventCount() != mz32Events {
		t.Fatalf("logs/event: %d %d (%d)", st, len(evs), s.EventCount())
	}
}
