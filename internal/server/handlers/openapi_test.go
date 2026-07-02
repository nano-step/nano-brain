package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
)

// TestOpenAPISpecHandler covers the new GET /api/openapi.json route: it must
// return 200, an application/json Content-Type, and a body that parses as
// JSON with a root "openapi" key starting "3.0" and no "swagger" key
// (guards Pitfall 1 — swag v1's native Swagger 2.0 output — at the handler
// level, not just in internal/openapigen's spec-level tests).
func TestOpenAPISpecHandler(t *testing.T) {
	h := handlers.OpenAPISpec()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("OpenAPISpec: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want prefix application/json", ct)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}

	if _, hasSwagger := body["swagger"]; hasSwagger {
		t.Fatalf("served spec has a top-level \"swagger\" key — must be OpenAPI 3.0, not raw swag v1 output")
	}

	openapiRaw, ok := body["openapi"]
	if !ok {
		t.Fatalf("served spec has no top-level \"openapi\" key")
	}
	var openapiVersion string
	if err := json.Unmarshal(openapiRaw, &openapiVersion); err != nil {
		t.Fatalf("unmarshal openapi version: %v", err)
	}
	if !strings.HasPrefix(openapiVersion, "3.0") {
		t.Fatalf("root \"openapi\" version = %q, want prefix \"3.0\"", openapiVersion)
	}
}
