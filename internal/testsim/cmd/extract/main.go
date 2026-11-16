// Command extract turns the research captures in docs/ into the sanitized
// fixture files embedded by internal/testsim. It is run by
//
//	go generate ./internal/testsim
//
// Inputs (read-only):
//
//	docs/x470/har-endpoints.json          X470D4U HAR digest (BMC 2.32.00)
//	docs/b650/har-endpoints.json          B650D4U + B550D4U-2T HAR digests (BMC 1.10.00)
//	docs/x470/live-redfish-inventory.json X470 unauthenticated Redfish ServiceRoot
//
// Outputs:
//
//	<out>/x470/endpoints.json, <out>/b650/endpoints.json
//	<out>/x470/redfish-serviceroot.json
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
	for _, board := range []string{"x470", "b650"} {
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
	inv, err := os.ReadFile(filepath.Join(docs, "x470", "live-redfish-inventory.json"))
	if err != nil {
		return err
	}
	root, err := ExtractServiceRoot(inv)
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(out, "x470", "redfish-serviceroot.json"), root)
}

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
	Source    json.RawMessage `json:"source"` // string (X470) or []string (B650)
	Endpoints []struct {
		Method         string              `json:"method"`
		Path           string              `json:"path"`
		Query          []map[string]string `json:"query"`
		Status         map[string]int      `json:"status"`
		Capture        string              `json:"capture"`
		ContentType    string              `json:"response_content_type"`
		RequestExample struct {
			Body any `json:"body"`
		} `json:"request_example"`
		ResponseExample any    `json:"response_example"`
		Note            string `json:"note"`
	} `json:"endpoints"`
}

const sanitizedNote = "Derived from docs/<board>/har-endpoints.json by internal/testsim/cmd/extract: passwords, CSRF tokens, session ids, serials, MACs, UUIDs, host names and e-mails are <REDACTED_*> placeholders; private IPv4 addresses are mapped to 192.0.2.0/24."

// Extract converts one har-endpoints.json digest into a sanitized fixture.
func Extract(raw []byte, board string) (*File, error) {
	var d harDigest
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&d); err != nil {
		return nil, err
	}
	s := NewSanitizer()
	f := &File{Board: board, Source: s.String(sourceString(d.Source)), Sanitized: sanitizedNote}
	for _, e := range d.Endpoints {
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
		if len(e.Query) > 0 {
			ep.Query = map[string]string{}
			for _, q := range e.Query {
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

// ExtractServiceRoot pulls the X470 unauthenticated ServiceRoot out of the
// live Redfish inventory.
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
