// Command openapigen converts swag's Swagger 2.0 output to a validated
// OpenAPI 3 document.
//
//	openapigen -in internal/apidocs/swagger.json -out internal/apidocs/openapi.json
//
// Besides the conversion (kin-openapi openapi2conv) it hoists the request
// model's repeated header parameters (X-BMC-Host, X-BMC-Vendor) into shared components/parameters, validates the result
// with the kin-openapi loader, and writes deterministic, pretty-printed JSON
// (encoding/json sorts map keys) so the committed file only changes when the
// API does. It is run by `go generate ./internal/apidocs`.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
)

// shared header parameters: name -> component key, required flag, text,
// example value, and whether operations reference it.
//
//   - X-BMC-Host's example is the client-side variable {{bmcHost}}, so tools
//     that import the spec (Postman, Bruno, Insomnia) fill the header from an
//     environment variable instead of a hard-coded address.
//   - X-BMC-Vendor is defined in components/parameters (and described in the
//     API description) but deliberately NOT attached to operations: importers
//     add optional header parameters switched on by default, which would send
//     an empty vendor override on every request. Auto-detection is the
//     default; clients add the header by hand when they need to force a driver.
var shared = []struct {
	name, key   string
	required    bool
	description string
	example     string
	attach      bool
}{
	{"X-BMC-Host", "XBMCHost", true, "BMC address: ip, host or host:port (IPv6 in brackets).", "{{bmcHost}}", true},
	{"X-BMC-Vendor", "XBMCVendor", false, "Optional: force a driver (e.g. asrock, gigabyte, redfish) instead of auto-detection. Not attached to operations by default; add the header manually when needed.", "", false},
}

func main() {
	in := flag.String("in", "swagger.json", "swag Swagger 2.0 input")
	out := flag.String("out", "openapi.json", "OpenAPI 3 output")
	flag.Parse()
	if err := run(*in, *out); err != nil {
		fmt.Fprintln(os.Stderr, "openapigen:", err)
		os.Exit(1)
	}
}

func run(in, out string) error {
	raw, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	b, err := Convert(raw)
	if err != nil {
		return err
	}
	if old, err := os.ReadFile(out); err == nil && bytes.Equal(old, b) {
		return nil // unchanged: keep mtime stable
	}
	return os.WriteFile(out, b, 0o644)
}

// Convert turns Swagger 2.0 JSON into validated, deterministic OpenAPI 3 JSON.
func Convert(swagger []byte) ([]byte, error) {
	var doc2 openapi2.T
	if err := json.Unmarshal(swagger, &doc2); err != nil {
		return nil, fmt.Errorf("parse swagger: %w", err)
	}
	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return nil, fmt.Errorf("convert: %w", err)
	}
	hoistHeaders(doc3)

	b, err := json.MarshalIndent(doc3, "", "  ")
	if err != nil {
		return nil, err
	}
	// Re-marshal through a generic value: guarantees sorted keys everywhere.
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	b, err = json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	if err := Validate(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Validate loads an OpenAPI 3 document (resolving refs) and validates it.
func Validate(b []byte) error {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(b)
	if err != nil {
		return fmt.Errorf("load openapi: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return fmt.Errorf("validate openapi: %w", err)
	}
	return nil
}

func hoistHeaders(doc *openapi3.T) {
	if doc.Components == nil {
		doc.Components = &openapi3.Components{}
	}
	if doc.Components.Parameters == nil {
		doc.Components.Parameters = openapi3.ParametersMap{}
	}
	used := map[string]bool{}
	if doc.Paths == nil {
		return
	}
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			kept := op.Parameters[:0]
		params:
			for _, pr := range op.Parameters {
				if pr != nil && pr.Value != nil && pr.Value.In == openapi3.ParameterInHeader {
					for _, sh := range shared {
						if pr.Value.Name == sh.name && pr.Value.Required == sh.required {
							used[sh.key] = true
							if !sh.attach {
								continue params // defined in components only
							}
							pr = &openapi3.ParameterRef{Ref: "#/components/parameters/" + sh.key}
							break
						}
					}
				}
				kept = append(kept, pr)
			}
			op.Parameters = kept
		}
	}
	for _, sh := range shared {
		if !used[sh.key] {
			continue
		}
		prm := &openapi3.Parameter{
			Name: sh.name, In: openapi3.ParameterInHeader, Required: sh.required, Description: sh.description,
			Schema: openapi3.NewSchemaRef("", openapi3.NewStringSchema()),
		}
		if sh.example != "" {
			prm.Example = sh.example
		}
		doc.Components.Parameters[sh.key] = &openapi3.ParameterRef{Value: prm}
	}
}
