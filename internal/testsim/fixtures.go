package testsim

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"sync"
)

// The fixtures are generated from docs/*/har-endpoints.json (and the X470
// live Redfish inventory) by cmd/extract, which re-applies sanitization.
//
//go:generate go run ./cmd/extract -docs ../../docs -out fixtures

//go:embed fixtures
var fixtureFS embed.FS

// Endpoint is one captured /api request/response pair from a HAR digest.
type Endpoint struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Query       map[string]string `json:"query,omitempty"`
	Status      int               `json:"status"`
	Capture     string            `json:"capture,omitempty"`
	ContentType string            `json:"contentType,omitempty"`
	RequestBody json.RawMessage   `json:"requestBody,omitempty"`
	Response    json.RawMessage   `json:"response"`
	Note        string            `json:"note,omitempty"`
}

// FixtureSet is the sanitized capture of one board family.
type FixtureSet struct {
	Board     string     `json:"board"`
	Source    string     `json:"source"`
	Sanitized string     `json:"sanitized"`
	Endpoints []Endpoint `json:"endpoints"`
}

var (
	fixtureMu    sync.Mutex
	fixtureCache = map[string]*FixtureSet{}
)

// LoadFixtures returns the embedded fixture set for board ("x470" or
// "b650"). The result is shared; do not modify it.
func LoadFixtures(board string) (*FixtureSet, error) {
	fixtureMu.Lock()
	defer fixtureMu.Unlock()
	if fs, ok := fixtureCache[board]; ok {
		return fs, nil
	}
	b, err := fixtureFS.ReadFile(path.Join("fixtures", board, "endpoints.json"))
	if err != nil {
		return nil, fmt.Errorf("testsim: no fixtures for board %q: %w", board, err)
	}
	var fs FixtureSet
	if err := json.Unmarshal(b, &fs); err != nil {
		return nil, fmt.Errorf("testsim: decode %s fixtures: %w", board, err)
	}
	fixtureCache[board] = &fs
	return &fs, nil
}

// Find returns the endpoint for method+path, preferring captures in the
// given order (B650 has two captures: b650d4u.har and b650d4u-2t.har).
func (fs *FixtureSet) Find(method, p string, captures ...string) (Endpoint, bool) {
	var cands []Endpoint
	for _, e := range fs.Endpoints {
		if e.Method == method && e.Path == p {
			cands = append(cands, e)
		}
	}
	if len(cands) == 0 {
		return Endpoint{}, false
	}
	for _, c := range captures {
		for _, e := range cands {
			if e.Capture == c {
				return e, true
			}
		}
	}
	return cands[0], true
}

// rawFixture reads another embedded fixture file (e.g. the X470 ServiceRoot).
func rawFixture(board, name string) ([]byte, error) {
	return fixtureFS.ReadFile(path.Join("fixtures", board, name))
}

// decodeAny decodes JSON preserving numbers exactly (json.Number), so a
// replayed X470 threshold stays 0.0 rather than becoming 0.
func decodeAny(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	err := dec.Decode(&v)
	return v, err
}

func cloneJSON(v any) any {
	b, _ := json.Marshal(v)
	out, _ := decodeAny(b)
	return out
}
