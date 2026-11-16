package main

import (
	"encoding/json"
	"testing"

	"github.com/slothnetworks/bmcproxy/internal/apidocs"
)

// TestSharedHeaderDefaults pins the import-friendly header setup: X-BMC-Host
// carries the {{bmcHost}} client variable, and X-BMC-Vendor is defined in
// components but attached to no operation (importers would otherwise add it
// switched on by default).
func TestSharedHeaderDefaults(t *testing.T) {
	var doc struct {
		Components struct {
			Parameters map[string]struct {
				Name     string `json:"name"`
				Required bool   `json:"required"`
				Example  any    `json:"example"`
			} `json:"parameters"`
		} `json:"components"`
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(apidocs.OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	host, ok := doc.Components.Parameters["XBMCHost"]
	if !ok || !host.Required || host.Example != "{{bmcHost}}" {
		t.Fatalf("XBMCHost component = %+v (present=%v), want required with example {{bmcHost}}", host, ok)
	}
	vendor, ok := doc.Components.Parameters["XBMCVendor"]
	if !ok || vendor.Required || vendor.Example != nil {
		t.Fatalf("XBMCVendor component = %+v (present=%v), want optional with no example", vendor, ok)
	}
	for path, ops := range doc.Paths {
		for method, raw := range ops {
			var op struct {
				Parameters []struct {
					Ref  string `json:"$ref"`
					Name string `json:"name"`
				} `json:"parameters"`
			}
			if json.Unmarshal(raw, &op) != nil {
				continue // path-level non-operation keys
			}
			for _, p := range op.Parameters {
				if p.Ref == "#/components/parameters/XBMCVendor" || p.Name == "X-BMC-Vendor" {
					t.Errorf("%s %s attaches X-BMC-Vendor; it must stay components-only", method, path)
				}
			}
		}
	}
}
