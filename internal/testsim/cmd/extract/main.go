// Command extract turns the research captures in docs/ into the sanitized
// fixture files embedded by internal/testsim. It is run by
//
//	go generate ./internal/testsim
//
// Inputs (read-only):
//
//	docs/x470/har-endpoints.json          X470D4U HAR digest (BMC 2.32.00)
//	docs/b650/har-endpoints.json          B650D4U + B550D4U-2T HAR digests (BMC 1.10.00)
//	docs/x570/har-endpoints.json          X570D4U live read-only probe digest (BMC 1.20.00)
//	docs/x470/live-redfish-inventory.json X470 unauthenticated Redfish ServiceRoot
//	docs/x570/live-redfish-inventory.json X570 Redfish ServiceRoot
//	docs/gigabyte/mc13-le0-har-endpoints.json   Gigabyte MC13-LE0 live read-only probe digest (BMC 13.04.15; /api entries only)
//	docs/gigabyte/mz32-ar0-redfish-inventory.json Gigabyte MZ32-AR0 Redfish crawl (BMC 12.61.39; the SubProcessors crawl trap)
//	vendors/gigabyte/testdata/{mc13le0-13.04.15,mz32ar0-12.61.39}.json Redfish trees (docs/../vendors)
//
// Outputs:
//
//	<out>/{x470,b650,x570,mc13le0}/endpoints.json
//	<out>/{x470,x570}/redfish-serviceroot.json
//	<out>/{mc13le0,mz32ar0}/redfish.json
//
// The docs captures are already redacted, but sanitization is re-applied here
// so a future, less carefully redacted capture cannot leak into the test tree:
// passwords, CSRF tokens, session ids, serials, MACs, UUIDs, host names and
// e-mail addresses become <REDACTED_*> placeholders, and every private IPv4
// address is rewritten to a stable RFC 5737 documentation address
// (192.0.2.0/24). The output is deterministic (sorted keys, stable address
// mapping), so regenerating produces no diff unless the inputs changed.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	docs := flag.String("docs", "../../docs", "docs directory (relative to internal/testsim when run via go generate)")
	out := flag.String("out", "fixtures", "output directory")
	flag.Parse()
	if err := run(*docs, *out); err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(1)
	}
}

func run(docs, out string) error {
	for _, board := range Boards {
		src := filepath.Join(docs, board, "har-endpoints.json")
		raw, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		fx, err := Extract(raw, board)
		if err != nil {
			return fmt.Errorf("%s: %w", src, err)
		}
		if err := writeJSON(filepath.Join(out, board, "endpoints.json"), fx); err != nil {
			return err
		}
	}
	for _, board := range ServiceRootBoards {
		inv, err := os.ReadFile(filepath.Join(docs, board, "live-redfish-inventory.json"))
		if err != nil {
			return err
		}
		root, err := ExtractServiceRoot(inv)
		if err != nil {
			return err
		}
		if err := writeJSON(filepath.Join(out, board, "redfish-serviceroot.json"), root); err != nil {
			return err
		}
	}
	testdata := filepath.Join(docs, "..", "vendors", "gigabyte", "testdata")
	for _, g := range GigabyteBoards {
		if g.HAR != "" {
			raw, err := os.ReadFile(filepath.Join(docs, g.HAR))
			if err != nil {
				return err
			}
			fx, err := Extract(raw, g.Board, "/api/")
			if err != nil {
				return fmt.Errorf("%s: %w", g.HAR, err)
			}
			fx.Sanitized = strings.ReplaceAll(fx.Sanitized, "docs/<board>/har-endpoints.json", "docs/"+g.HAR)
			if err := writeJSON(filepath.Join(out, g.Board, "endpoints.json"), fx); err != nil {
				return err
			}
		}
		raw, err := os.ReadFile(filepath.Join(testdata, g.Redfish))
		if err != nil {
			return err
		}
		var inv []byte
		if g.Inventory != "" {
			if inv, err = os.ReadFile(filepath.Join(docs, g.Inventory)); err != nil {
				return err
			}
		}
		tree, err := ExtractRedfishTree(raw, inv, g.InventoryPrefixes)
		if err != nil {
			return fmt.Errorf("%s: %w", g.Redfish, err)
		}
		if err := writeJSON(filepath.Join(out, g.Board, "redfish.json"), tree); err != nil {
			return err
		}
	}
	return nil
}

// GigabyteBoard describes one Gigabyte fixture set: an optional /api HAR
// digest (docs-relative), the Redfish tree in vendors/gigabyte/testdata, and
// optionally a docs Redfish inventory whose resources under
// InventoryPrefixes are merged in (the MZ32-AR0 testdata drops the
// SubProcessors crawl trap, which the simulator must still serve so tests
// can prove the driver never walks it).
type GigabyteBoard struct {
	Board, HAR, Redfish, Inventory string
	InventoryPrefixes              []string
}

// GigabyteBoards are the Gigabyte simulator fixtures.
var GigabyteBoards = []GigabyteBoard{
	{Board: "mc13le0", HAR: "gigabyte/mc13-le0-har-endpoints.json", Redfish: "mc13le0-13.04.15.json"},
	{Board: "mz32ar0", Redfish: "mz32ar0-12.61.39.json", Inventory: "gigabyte/mz32-ar0-redfish-inventory.json",
		InventoryPrefixes: []string{"/redfish/v1/Systems/Self/Processors/CPU0/SubProcessors"}},
}

// RedfishTree is the redfish.json fixture format: path -> body.
type RedfishTree struct {
	Source    string                    `json:"source"`
	Sanitized string                    `json:"sanitized"`
	Resources map[string]map[string]any `json:"resources"`
}

const redfishSanitizedNote = "Derived from vendors/gigabyte/testdata (and docs/gigabyte inventories) by internal/testsim/cmd/extract: serials, MACs, UUIDs, host names, e-mails and secrets are <REDACTED_*> placeholders (the SMBIOS 0123456789... placeholder is kept); private IPv4 addresses are mapped to 192.0.2.0/24."

// ExtractRedfishTree sanitizes a vendors/gigabyte/testdata Redfish tree
// ({"_comment", "redfish": {path: body}}) and merges the inventory
// resources (docs/gigabyte/*-redfish-inventory.json, {"resources":
// [{uri, status, body}]}) whose path starts with one of prefixes; "?$skip"
// pages are merged back into their collection.
func ExtractRedfishTree(raw, inv []byte, prefixes []string) (*RedfishTree, error) {
	var td struct {
		Comment string                    `json:"_comment"`
		Redfish map[string]map[string]any `json:"redfish"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&td); err != nil {
		return nil, err
	}
	if len(td.Redfish) == 0 {
		return nil, fmt.Errorf("no redfish resources")
	}
	res := map[string]map[string]any{}
	for k, v := range td.Redfish {
		res[strings.TrimRight(k, "/")] = v
	}
	if len(inv) > 0 && len(prefixes) > 0 {
		var d struct {
			Resources []struct {
				URI    string         `json:"uri"`
				Status int            `json:"status"`
				Body   map[string]any `json:"body"`
			} `json:"resources"`
		}
		dec := json.NewDecoder(bytes.NewReader(inv))
		dec.UseNumber()
		if err := dec.Decode(&d); err != nil {
			return nil, err
		}
		var pages []map[string]any
		var pagePaths []string
		for _, r := range d.Resources {
			if r.Status != 200 || r.Body == nil || !hasAnyPrefix(r.URI, prefixes) {
				continue
			}
			if i := strings.IndexByte(r.URI, '?'); i >= 0 {
				pages, pagePaths = append(pages, r.Body), append(pagePaths, strings.TrimRight(r.URI[:i], "/"))
				continue
			}
			res[strings.TrimRight(r.URI, "/")] = r.Body
		}
		for i, pg := range pages {
			coll := res[pagePaths[i]]
			if coll == nil {
				continue
			}
			ms, _ := coll["Members"].([]any)
			more, _ := pg["Members"].([]any)
			coll["Members"] = append(ms, more...)
			coll["Members@odata.count"] = json.Number(fmt.Sprint(len(ms) + len(more)))
			delete(coll, "Members@odata.nextLink")
		}
	}
	s := NewSanitizer()
	out := &RedfishTree{Source: s.String(td.Comment), Sanitized: redfishSanitizedNote, Resources: map[string]map[string]any{}}
	for _, k := range sortedKeys(res) {
		out.Resources[k] = s.Value("", res[k]).(map[string]any)
	}
	return out, nil
}

func hasAnyPrefix(s string, ps []string) bool {
	for _, p := range ps {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// Boards are the docs/<board>/har-endpoints.json digests turned into
// fixtures; ServiceRootBoards also get redfish-serviceroot.json.
var (
	Boards            = []string{"x470", "b650", "x570"}
	ServiceRootBoards = []string{"x470", "x570"}
)

// File is the fixture file format (mirrored by testsim.fixtureFile).
type File struct {
	Board     string     `json:"board"`
	Source    string     `json:"source"`
	Sanitized string     `json:"sanitized"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint is one captured request/response pair.
type Endpoint struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Query       map[string]string `json:"query,omitempty"`
	Status      int               `json:"status"`
	Capture     string            `json:"capture,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	RequestBody any               `json:"requestBody,omitempty"`
	Response    any               `json:"response"`
	Note        string            `json:"note,omitempty"`
}

type harDigest struct {
	Source    json.RawMessage `json:"source"` // string (X470, X570) or []string (B650)
	Endpoints []struct {
		Method         string            `json:"method"`
		Path           string            `json:"path"`
		Query          []json.RawMessage `json:"query"` // {"k":"v"} objects, or "k=v" strings (Gigabyte digests)
		Status         map[string]int    `json:"status"`
		Capture        string            `json:"capture"`
		ContentType    string            `json:"response_content_type"`
		RequestExample struct {
			Body any `json:"body"`
		} `json:"request_example"`
		ResponseExample any    `json:"response_example"`
		Note            string `json:"note"`
	} `json:"endpoints"`
}

const sanitizedNote = "Derived from docs/<board>/har-endpoints.json by internal/testsim/cmd/extract: passwords, CSRF tokens, session ids, serials, MACs, UUIDs, host names and e-mails are <REDACTED_*> placeholders; private IPv4 addresses are mapped to 192.0.2.0/24."

// Extract converts one har-endpoints.json digest into a sanitized fixture.
// With pathPrefix, only endpoints whose path starts with it are kept (the
// Gigabyte digests also hold the Redfish crawl, which comes from testdata).
func Extract(raw []byte, board string, pathPrefix ...string) (*File, error) {
	var d harDigest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&d); err != nil {
		return nil, err
	}
	s := NewSanitizer()
	f := &File{Board: board, Source: s.String(sourceString(d.Source)), Sanitized: sanitizedNote}
	for _, e := range d.Endpoints {
		if len(pathPrefix) > 0 && !hasAnyPrefix(e.Path, pathPrefix) {
			continue
		}
		ep := Endpoint{
			Method:      e.Method,
			Path:        e.Path,
			Status:      pickStatus(e.Status),
			Capture:     e.Capture,
			ContentType: e.ContentType,
			RequestBody: s.Value("", e.RequestExample.Body),
			Response:    s.Value("", e.ResponseExample),
			Note:        s.String(e.Note),
		}
		if ep.Status == 200 && isAuthGate(ep.Response) {
			// The digest kept the auth-gate example of an endpoint that
			// also answered 200 (MC13-LE0 chassis-status): say so.
			ep.Status = 401
			if ep.Note == "" {
				ep.Note = "example is the 401 {cc:7} auth-gate answer; the endpoint also answered 200"
			}
		}
		if len(e.Query) > 0 {
			ep.Query = map[string]string{}
			for _, raw := range e.Query {
				var q map[string]string
				if json.Unmarshal(raw, &q) != nil {
					var kv string
					if json.Unmarshal(raw, &kv) != nil {
						continue
					}
					k, v, _ := strings.Cut(kv, "=")
					q = map[string]string{k: v}
				}
				for k, v := range q {
					ep.Query[k] = s.String(v)
				}
			}
		}
		f.Endpoints = append(f.Endpoints, ep)
	}
	sort.SliceStable(f.Endpoints, func(i, j int) bool {
		a, b := f.Endpoints[i], f.Endpoints[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		return a.Capture < b.Capture
	})
	return f, nil
}

// isAuthGate reports the AMI {"cc":7,"error":"Invalid Authentication"} body.
func isAuthGate(v any) bool {
	m, ok := v.(map[string]any)
	if !ok || len(m) != 2 {
		return false
	}
	_, cc := m["cc"]
	_, er := m["error"]
	return cc && er
}

func sourceString(raw json.RawMessage) string {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		return one
	}
	var many []string
	_ = json.Unmarshal(raw, &many)
	return strings.Join(many, "; ")
}

// pickStatus prefers 200 (the example body is the successful one), else the
// most frequent status.
func pickStatus(m map[string]int) int {
	if _, ok := m["200"]; ok {
		return 200
	}
	best, bestN := 200, -1
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var st int
		if _, err := fmt.Sscanf(k, "%d", &st); err == nil && m[k] > bestN {
			best, bestN = st, m[k]
		}
	}
	return best
}

// ExtractServiceRoot pulls the ServiceRoot out of a live Redfish inventory.
func ExtractServiceRoot(raw []byte) (any, error) {
	var inv struct {
		ServiceRoot any `json:"service_root"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&inv); err != nil {
		return nil, err
	}
	if inv.ServiceRoot == nil {
		return nil, fmt.Errorf("no service_root in inventory")
	}
	return NewSanitizer().Value("", inv.ServiceRoot), nil
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
