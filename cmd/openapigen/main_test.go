package main

import (
	"bytes"
	"testing"

	"github.com/slothnetworks/bmcproxy/internal/apidocs"
)

// The committed openapi.json must be exactly the conversion of the committed
// swagger.json (deterministic output, no hand edits).
func TestConvertMatchesCommitted(t *testing.T) {
	b, err := Convert(apidocs.Swagger)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, apidocs.OpenAPI) {
		t.Fatal("openapi.json is stale or non-deterministic: run `make generate`")
	}
	b2, _ := Convert(apidocs.Swagger)
	if !bytes.Equal(b, b2) {
		t.Fatal("Convert is not deterministic")
	}
}
