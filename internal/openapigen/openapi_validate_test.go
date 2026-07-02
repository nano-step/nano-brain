package openapigen_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

// TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema satisfies issue #530
// acceptance criterion AC-1: the committed docs/openapi.json must validate
// against the OpenAPI 3.0 schema.
func TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema(t *testing.T) {
	const specPath = "../../docs/openapi.json"

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("loading %s: %v", specPath, err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("%s failed OpenAPI 3.0 validation: %v", specPath, err)
	}
}

// TestOpenAPISpec_RootIsOpenAPI3NotSwagger2 guards Pitfall 1 (RESEARCH.md):
// swag v1 natively emits Swagger 2.0; the committed spec must be the
// converted OpenAPI 3.0 document, never the raw swag output.
func TestOpenAPISpec_RootIsOpenAPI3NotSwagger2(t *testing.T) {
	const specPath = "../../docs/openapi.json"

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading %s: %v", specPath, err)
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatalf("unmarshal %s: %v", specPath, err)
	}

	if _, hasSwagger := root["swagger"]; hasSwagger {
		t.Fatalf("%s has a top-level \"swagger\" key — this must be the converted OpenAPI 3.0 document, not raw swag v1 output", specPath)
	}

	openapiRaw, ok := root["openapi"]
	if !ok {
		t.Fatalf("%s has no top-level \"openapi\" key", specPath)
	}

	var openapiVersion string
	if err := json.Unmarshal(openapiRaw, &openapiVersion); err != nil {
		t.Fatalf("unmarshal openapi version: %v", err)
	}
	if !strings.HasPrefix(openapiVersion, "3.0") {
		t.Fatalf("%s root \"openapi\" version = %q, want prefix \"3.0\"", specPath, openapiVersion)
	}
}

// TestOpenAPISpec_HealthResponseSchemaComplete is the empirical Assumption
// A1 gate (12-RESEARCH.md Open Question #1 / Pitfall 2): swag's AST parser
// must resolve the UNEXPORTED same-package struct healthResponse
// (internal/server/handlers/health.go) into a complete, non-empty schema.
// If this fails, ~35 of the 60 routes would need their structs exported
// before Plans 02/03 can proceed on an annotation-only basis.
func TestOpenAPISpec_HealthResponseSchemaComplete(t *testing.T) {
	const specPath = "../../docs/openapi.json"

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("reading %s: %v", specPath, err)
	}

	var doc struct {
		Components struct {
			Schemas map[string]struct {
				Type       string                     `json:"type"`
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal %s: %v", specPath, err)
	}

	// swag names same-package unexported types with a mangled
	// path-qualified key (e.g. "internal_server_handlers.healthResponse")
	// rather than the bare type name — find it by suffix.
	var found *struct {
		Type       string
		Properties map[string]json.RawMessage
	}
	var matchedKey string
	for key, schema := range doc.Components.Schemas {
		if strings.HasSuffix(key, ".healthResponse") || key == "healthResponse" {
			matchedKey = key
			found = &struct {
				Type       string
				Properties map[string]json.RawMessage
			}{Type: schema.Type, Properties: schema.Properties}
			break
		}
	}

	if found == nil {
		t.Fatalf("Assumption A1 FAILED: no schema found for healthResponse in %s (schemas present: %v) — swag did not resolve the unexported same-package struct", specPath, schemaKeys(doc.Components.Schemas))
	}

	if len(found.Properties) == 0 {
		t.Fatalf("Assumption A1 FAILED: schema %q for healthResponse is empty (no properties) — swag resolved the type name but not its fields", matchedKey)
	}

	wantFields := []string{"status", "ready", "version", "uptime_s", "workspace_count", "reason"}
	for _, field := range wantFields {
		if _, ok := found.Properties[field]; !ok {
			t.Errorf("Assumption A1: schema %q missing expected field %q (properties present: %v)", matchedKey, field, propertyKeys(found.Properties))
		}
	}
}

func schemaKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func propertyKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
