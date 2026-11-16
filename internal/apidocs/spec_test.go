package apidocs_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/slothnetworks/bmcproxy/internal/apidocs"
	"github.com/slothnetworks/bmcproxy/internal/server"
	"github.com/slothnetworks/bmcproxy/internal/server/servertest"
)

// TestSpecCoversRoutes fails when a mounted route has no annotation (or the
// committed spec is stale): run `make generate`.
func TestSpecCoversRoutes(t *testing.T) {
	var sw map[string]any
	if err := json.Unmarshal(apidocs.Swagger, &sw); err != nil {
		t.Fatalf("swagger.json: %v", err)
	}
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(apidocs.OpenAPI)
	if err != nil {
		t.Fatalf("load openapi.json: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("openapi.json invalid: %v", err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		t.Fatalf("openapi version %q", doc.OpenAPI)
	}

	h := servertest.New(t, servertest.Options{DebugEndpoints: true})
	routes := h.Server.Routes()
	if len(routes) < 30 {
		t.Fatalf("only %d routes registered", len(routes))
	}
	type mp struct{ method, path string }
	want := []mp{{"GET", "/metrics"}, {"GET", "/debug/targets"}, {"GET", "/debug/sessions"}, {"GET", "/debug/stats"}, {"GET", "/debug/capabilities"}, {"GET", "/debug/levels"}}
	for _, r := range routes {
		want = append(want, mp{r.Method, strings.ReplaceAll(r.Path, "...}", "}")})
	}
	for _, w := range want {
		item := doc.Paths.Find(w.path)
		if item == nil || item.GetOperation(w.method) == nil {
			t.Errorf("openapi.json lacks %s %s (annotate the handler and run `make generate`)", w.method, w.path)
			continue
		}
		op := item.GetOperation(w.method)
		if op.Summary == "" || op.Description == "" || len(op.Responses.Map()) == 0 {
			t.Errorf("%s %s: incomplete annotation", w.method, w.path)
		}
	}
	// Generic BMC routes reference the shared X-BMC-Host header parameter.
	op := doc.Paths.Find("/api/v1/sensors").Get
	found := false
	for _, p := range op.Parameters {
		if p.Ref == "#/components/parameters/XBMCHost" {
			found = true
		}
	}
	if !found {
		t.Error("X-BMC-Host not hoisted to a shared component")
	}
}

func TestDocsServed(t *testing.T) {
	h := servertest.New(t, servertest.Options{})
	server.MountAPIDocs(h.Server, apidocs.OpenAPI, apidocs.Swagger)
	for path, want := range map[string]string{
		"/openapi.json":       `"openapi": "3.`,
		"/swagger/doc.json":   `"swagger": "2.0"`,
		"/swagger/index.html": "swagger-ui",
	} {
		r := h.Do(servertest.Req{Path: path, NoAuth: true})
		if r.Status != 200 || !strings.Contains(string(r.Body), want) {
			t.Errorf("%s: %d %.200s", path, r.Status, r.Body)
		}
	}
	r := h.Do(servertest.Req{Path: "/swagger/index.html", NoAuth: true})
	if !strings.Contains(string(r.Body), "/openapi.json") {
		t.Errorf("swagger UI not pointed at /openapi.json: %.300s", r.Body)
	}
}
