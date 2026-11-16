package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizer(t *testing.T) {
	s := NewSanitizer()
	in := map[string]any{
		"password":               "hunter2",
		"CSRFToken":              "Ab3dEf9h",
		"racsession_id":          "7",
		"SerialNumber":           "M80-ABC1234",
		"privilege_limit_serial": "none", // benign value kept
		"mac_address":            "aa:bb:cc:dd:ee:ff",
		"UUID":                   "NA",
		"email_id":               "ops@example.com",
		"note":                   "BMC 10.3.10.207 talked to 10.0.0.252 and 172.29.0.6; mask 255.255.255.0, usb 169.254.0.17, mac 00:11:22:33:44:55",
		"active_session":         json.Number("129"),
		"nested":                 []any{map[string]any{"passwd": "x", "remote_addr": "10.0.0.252"}},
	}
	out := s.Value("", in).(map[string]any)
	want := map[string]string{
		"password": "<REDACTED_PASSWORD>", "CSRFToken": "<REDACTED_CSRF>", "racsession_id": "<REDACTED_SESSION_ID>",
		"SerialNumber": "<REDACTED_SERIAL>", "privilege_limit_serial": "none", "mac_address": "<REDACTED_MAC>",
		"UUID": "NA", "email_id": "<REDACTED_EMAIL>",
	}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("%s = %v, want %v", k, out[k], v)
		}
	}
	note := out["note"].(string)
	for _, leak := range []string{"10.3.10.207", "10.0.0.252", "172.29.0.6", "00:11:22:33:44:55"} {
		if strings.Contains(note, leak) {
			t.Errorf("note leaks %s: %s", leak, note)
		}
	}
	for _, keep := range []string{"255.255.255.0", "169.254.0.17"} {
		if !strings.Contains(note, keep) {
			t.Errorf("note lost non-identifying %s: %s", keep, note)
		}
	}
	nested := out["nested"].([]any)[0].(map[string]any)
	if nested["passwd"] != "<REDACTED_PASSWORD>" {
		t.Errorf("nested passwd %v", nested["passwd"])
	}
	// Stable mapping: the same address maps to the same placeholder.
	if !strings.Contains(note, nested["remote_addr"].(string)) {
		t.Errorf("unstable mapping: %s vs %s", nested["remote_addr"], note)
	}
	if out["active_session"] != json.Number("129") {
		t.Errorf("number changed: %v", out["active_session"])
	}
}

// TestFixturesUpToDate regenerates the fixtures from docs/ and compares
// them with the committed copies (run `go generate ./internal/testsim`).
func TestFixturesUpToDate(t *testing.T) {
	docs := filepath.Join("..", "..", "..", "..", "docs")
	if _, err := os.Stat(filepath.Join(docs, "x470", "har-endpoints.json")); err != nil {
		t.Skip("docs captures not available")
	}
	tmp := t.TempDir()
	if err := run(docs, tmp); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, b := range Boards {
		files = append(files, b+"/endpoints.json")
	}
	for _, b := range ServiceRootBoards {
		files = append(files, b+"/redfish-serviceroot.json")
	}
	for _, g := range GigabyteBoards {
		if g.HAR != "" {
			files = append(files, g.Board+"/endpoints.json")
		}
		files = append(files, g.Board+"/redfish.json")
	}
	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(tmp, f))
		if err != nil {
			t.Fatal(err)
		}
		have, err := os.ReadFile(filepath.Join("..", "..", "fixtures", f))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, have) {
			t.Errorf("fixtures/%s is stale: run go generate ./internal/testsim", f)
		}
	}
}
