# Phase 12: Add OpenAPI 3.0 spec for the REST API - Pattern Map

**Mapped:** 2026-07-02
**Files analyzed:** ~40 (34 handler files + 6 new files, grouped)
**Analogs found:** 5 strong matches covering all file groups

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/server/handlers/*.go` (34 files, add swag doc-comments above exported handler constructors) | controller (annotation-only edit) | request-response | `internal/server/handlers/query.go` | exact (same constructor-returns-`echo.HandlerFunc` idiom, typed Request/Response structs) |
| `internal/server/handlers/health.go` / `stats.go` (unexported struct case) | controller | request-response | `internal/server/handlers/health.go` (`healthResponse`, `statusResponse` — unexported, same-package) | exact — this IS the Pitfall-2 test case itself |
| `internal/server/doc.go` (NEW — general API info + `@securityDefinitions`) | config/doc-anchor | N/A (build-time metadata) | No direct analog exists (first swag "general API info" file in repo) — pattern must follow swag's documented convention exactly | no analog — see "No Analog Found" |
| `internal/server/handlers/openapi.go` (NEW — serves committed spec) | controller | file-I/O (serves embedded static bytes) | `internal/server/handlers/health.go` (`Health`/`Version` — simple `c.JSON`/handler-constructor pattern) + `migrations/migrations.go` (`//go:embed` precedent) | role-match (handler) + exact (embed precedent) |
| `internal/openapigen/*_test.go` (NEW — drift + schema validation tests) | test | transform/batch (regenerate-and-diff) | `internal/server/handlers/health_test.go` (table-style unit test conventions: `package x_test`, `zerolog.Nop()`, `httptest`) | role-match (test file conventions only; no drift-test analog exists in repo) |
| `internal/server/handlers/openapi_test.go` (NEW — handler-level test for `GET /api/openapi.json`) | test | request-response | `internal/server/handlers/health_test.go` | exact (handler test harness pattern: `echo.New()`, `httptest.NewRequest`, `httptest.NewRecorder()`, `e.NewContext`) |
| `internal/server/routes.go` (MODIFIED — register new route) | route | request-response | itself (existing file, extend in place) | exact — modify existing registration block |
| `go.mod` / `go.sum` (MODIFIED — add `swaggo/swag`, `getkin/kin-openapi`) | config | N/A | itself | exact — `go get` mechanically updates |
| `Makefile` (MODIFIED — new `generate-openapi` target) | config | batch | itself (existing `build`/`lint`/`test`/`test-integration`/`test-e2e` targets) | exact — same `.PHONY` + tab-indented recipe style |
| `README.md` / `docs/SETUP_AGENT.md` (MODIFIED — doc pointer per D-06) | doc | N/A | itself (existing "Development Setup" section in README.md at line 497) | exact — extend existing section |

## Pattern Assignments

### `internal/server/handlers/*.go` (34 files — annotation-only edits)

**Analog:** `internal/server/handlers/query.go` (113 lines, read in full)

**Imports pattern** (lines 1-13):
```go
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/telemetry"
	"github.com/rs/zerolog"
)
```
No import changes are needed for the annotation task itself — swag doc-comments are pure comments, not code. Do not add a swag import to handler files.

**Core pattern — constructor-returns-HandlerFunc idiom** (lines 15-31):
```go
type HybridSearcher interface { ... }

type QueryRequest struct {
	Query      string   `json:"query"`
	MaxResults int      `json:"max_results,omitempty"`
	...
}

func Query(searcher HybridSearcher, logger zerolog.Logger, rec ...*telemetry.Recorder) echo.HandlerFunc {
	return func(c echo.Context) error {
		...
	}
}
```
**Annotation placement rule (from RESEARCH.md Pattern 1):** the swag doc-comment block goes immediately above `func Query(...)`, the outer constructor — NEVER above the inner returned closure. Example annotation to add above this exact function:
```go
// Query godoc
// @Summary      Hybrid BM25 + vector search
// @Description  Runs the hybrid search pipeline scoped to a workspace
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        request body QueryRequest true "Search query"
// @Success      200 {object} SearchResponse
// @Failure      400 {object} handlers.ErrorResponse
// @Security     WorkspaceAuth
// @Router       /api/v1/query [post]
func Query(searcher HybridSearcher, logger zerolog.Logger, rec ...*telemetry.Recorder) echo.HandlerFunc {
```
**Critical caveat (Pitfall 3, confirmed against `routes.go` line 108):** `Query` is registered as `data.POST("/query", ...)` where `data := api.Group("", workspaceMiddleware(s.db))` and `api := s.echo.Group("/api/v1", ...)`. The literal `@Router` path MUST be the fully-resolved `/api/v1/query`, not `/query` — cross-reference every handler's registration line in `routes.go` (read in full below) before writing its `@Router` annotation.

**Error handling pattern** (lines 34-51): `echo.NewHTTPError(http.StatusBadRequest, "...")` for validation failures, `c.JSON(http.StatusBadRequest, map[string]string{...})` for structured param errors, `echo.NewHTTPError(http.StatusInternalServerError, "...")` after logging the underlying error. No changes needed — this task is annotation-only, error handling code is untouched.

---

### `internal/server/handlers/health.go` / `stats.go` (unexported-struct case — Wave 0 spike target)

**Analog:** `internal/server/handlers/health.go` (211 lines, read in full)

This file is the primary evidence for Assumption A1 (RESEARCH.md Open Question #1): `healthResponse`, `harvesterStatusResponse`, `statusResponse`, `versionResponse` (lines 86-93, 95-109, 111-123, 197-202) are all **unexported** structs referenced by exported handler methods (`Health`, `Status`, `Version`) in the *same* `handlers` package. This is the exact case the Wave-0 spike must verify swag resolves correctly before scaling to all 34 files:
```go
type healthResponse struct {
	Status         string `json:"status"`
	Ready          bool   `json:"ready"`
	Version        string `json:"version,omitempty"`
	UptimeS        int64  `json:"uptime_s,omitempty"`
	WorkspaceCount int    `json:"workspace_count,omitempty"`
	Reason         string `json:"reason,omitempty"`
}
...
func (h *Health) Health(c echo.Context) error {
	if err := h.pool.Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusOK, healthResponse{...})
	}
	return c.JSON(http.StatusOK, healthResponse{...})
}
```
Note this file uses **method receivers** (`func (h *Health) Health(c echo.Context) error`), not the constructor-returns-closure idiom of `query.go` — swag annotations for these should still go directly above the method declaration (`// @Router /health [get]` above `func (h *Health) Health(...)`), same placement rule, different Go idiom. `routes.go` line 27 confirms `/health` is a plain top-level route (no group prefix), so `@Router /health [get]` needs no prefix resolution — simpler than the grouped-route case.

---

### `internal/server/handlers/openapi.go` (NEW file — spec-serving handler)

**Analog 1 (handler shape):** `internal/server/handlers/health.go` `Version` method (lines 204-211) — the simplest handler in the codebase, useful as a template for a minimal `c.Blob`/`c.JSON`-only handler with no request parsing.

**Analog 2 (embed precedent):** `migrations/migrations.go` (read in full, 6 lines) — this repo's **only** existing `//go:embed` usage:
```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```
Mirror this exact shape for the committed OpenAPI spec, e.g.:
```go
package handlers

import (
	"embed"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed openapi.json
var openapiSpecFS embed.FS

// OpenAPISpec godoc
// @Summary      Serve the generated OpenAPI 3.0 specification
// @Tags         meta
// @Produce      json
// @Router       /api/openapi.json [get]
func OpenAPISpec() echo.HandlerFunc {
	specJSON, _ := openapiSpecFS.ReadFile("openapi.json")
	return func(c echo.Context) error {
		return c.Blob(http.StatusOK, "application/json", specJSON)
	}
}
```
Constructor-returns-closure idiom (matches `query.go`, not `health.go`'s method-receiver style) since this handler has no shared state beyond the embedded bytes — read once at construction, closed over by the returned `echo.HandlerFunc`, exactly like RESEARCH.md's Code Examples section recommends. Whether the embed source lives in `internal/server/handlers/openapi.json` or `docs/openapi.json` with the embed directive pointing there is the planner's call — RESEARCH.md's Code Examples section shows `docs/openapi.json` as the recommended location; `//go:embed` supports relative paths within the same or nested directory only (Go embed directives cannot reference paths outside the package directory), which constrains this: if `docs/openapi.json` is used, `internal/server/handlers/openapi.go` cannot embed it directly and would need either the file colocated in `internal/server/handlers/` or a small wrapper package under `docs/` exporting the embedded bytes — flag for planner.

---

### `internal/server/routes.go` (MODIFIED — register new route)

**Analog:** itself, read in full (159 lines).

**Registration pattern for new top-level GET route** (mirrors lines 27-29, the `/health`, `/api/status`, `/api/version` registrations — same tier, no auth group):
```go
s.echo.GET("/health", h.Health)
s.echo.GET("/api/status", h.Status)
s.echo.GET("/api/version", h.Version)
```
New route should be added in this same block or near it:
```go
s.echo.GET("/api/openapi.json", handlers.OpenAPISpec())
```
**Security note (from RESEARCH.md Security Domain):** confirm whether `/health`/`/api/version` bypass `middleware.Auth` (check `BypassPaths` in `internal/server/middleware.go`, not yet read in this pass — planner/implementer must verify) and apply the same bypass treatment to `/api/openapi.json` for consistency, since this is a discovery endpoint, not a protected data path.

**`@securityDefinitions` placement (D-04):** `routes.go` itself is NOT the annotation home — swag's `-g`/`MainAPIFile` flag expects one "general API info" file (RESEARCH.md line 250), which is `internal/server/doc.go` (new file, no analog — first of its kind in this repo). Middleware names to cross-reference for the `@Security` scheme descriptions:
```
internal/server/middleware.go:120  func workspaceMiddleware(db *sql.DB) echo.MiddlewareFunc
internal/server/middleware.go:213  func workspaceRegisteredMiddleware(db *sql.DB) echo.MiddlewareFunc
```
(CSRF middleware is constructed inline in `routes.go` line 59: `csrfMW := middleware.CSRF(boundAddr)` — defined in the `internal/server/middleware` package, not yet read in this pass; confirm exact file/line during implementation.)

**Route-to-group mapping table (for `@Router` path resolution across all 34+ handler files, extracted directly from `routes.go`):**

| Registration line | Full resolved path | Group chain |
|---|---|---|
| `s.echo.GET("/health", ...)` | `/health` | none (top-level) |
| `s.echo.GET("/api/status", ...)` | `/api/status` | none (top-level) |
| `s.echo.GET("/api/version", ...)` | `/api/version` | none (top-level) |
| `api.POST("/init", ...)` | `/api/v1/init` | `api = /api/v1` |
| `api.GET("/workspaces", ...)` | `/api/v1/workspaces` | `api` |
| `api.POST("/workspaces/resolve", ...)` | `/api/v1/workspaces/resolve` | `api` |
| `api.DELETE("/workspaces/:hash", ...)` | `/api/v1/workspaces/{hash}` | `api` |
| `api.POST("/reset-workspace", ...)` | `/api/v1/reset-workspace` | `api` |
| `api.GET("/config", ...)` / `api.POST("/config", ...)` | `/api/v1/config` | `api` |
| `api.GET("/doctor", ...)` | `/api/v1/doctor` | `api` |
| `data.GET("/events", ...)` | `/api/v1/events` | `data = api + workspaceMiddleware` |
| `write.POST("/write", ...)` etc. (all `write.*`) | `/api/v1/{path}` | `write = data + workspaceRegisteredMiddleware + csrfMW` |
| `data.POST("/collections", ...)` etc. (all `data.*`) | `/api/v1/{path}` | `data` |
| `api.GET("/wake-up", ...)` and `data.POST("/wake-up", ...)` | `/api/v1/wake-up` (both GET and POST, different middleware tiers) | `api` and `data` respectively |
| `api.GET("/sessions/by-ticket", ...)` | `/api/v1/sessions/by-ticket` | `api` |
| `s.echo.POST("/api/harvest", ...)` | `/api/harvest` | none (top-level, note: NOT under `/api/v1`) |
| `s.echo.POST("/api/reload-config", ...)` | `/api/reload-config` | none (top-level) |
| `s.echo.GET/POST("/sse", ...)` | `/sse` | none — protocol tunnel (Pattern 2, placeholder-anchor needed) |
| `s.echo.GET/POST/DELETE("/mcp", ...)` | `/mcp` | none — protocol tunnel (Pattern 2, placeholder-anchor needed) |
| `s.echo.GET("/ui", ...)` | `/ui` | none — static HTML redirect page, likely excluded from OpenAPI spec entirely (not a JSON REST endpoint; planner's call whether to annotate or exclude) |

Use this table directly when writing each handler's `@Router` line — it eliminates re-deriving group nesting per file.

---

### `internal/openapigen/*_test.go` (NEW — drift detection + schema validation)

**Analog (test file conventions only):** `internal/server/handlers/health_test.go` (lines 1-60 read)

```go
package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/rs/zerolog"
)
```
Convention to mirror: external `_test` package (black-box testing), `zerolog.Nop()` for silent test loggers, small mock types (`mockPool`, `mockQueue`, `mockCounter`) implementing the handler's narrow interfaces, one `newTestXxx` helper constructor. No `httptest` server is needed for the drift/validation tests themselves (RESEARCH.md's Code Examples show plain `os.ReadFile` + library calls), but the `openapi_test.go` handler-level test (see below) should use the same `httptest.NewRequest`/`httptest.NewRecorder`/`e.NewContext` pattern visible later in this file (not shown in this excerpt but standard across `*_test.go` in this package per the file list).

No existing analog exists in this repo for "regenerate into temp dir and diff against committed artifact" — RESEARCH.md's own Code Examples section is the authoritative template here (see RESEARCH.md lines 322-402), not a codebase analog. Follow it directly; naming convention (`package openapigen_test`, `TestOpenAPISpec_NoDrift`, `TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema`) should still follow this repo's `TestXxx_Description` convention visible in `health_test.go` (`TestStatusReturnsRealWorkspaceCount`) and `middleware_test.go` (`TestWorkspaceMiddleware_POST_WithWorkspace`, `TestWorkspaceMiddleware_POST_MissingWorkspace`).

---

### `internal/server/handlers/openapi_test.go` (NEW — handler-level test)

**Analog:** `internal/server/handlers/health_test.go` (full pattern, lines 55-60+ show the harness):
```go
func TestStatusReturnsRealWorkspaceCount(t *testing.T) {
	h := newTestHealth(&mockCounter{count: 3})
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// ... call h.Status(c), assert rec.Code / decodeJSON(t, rec.Body)
}
```
Mirror directly for `TestOpenAPISpecHandler`: construct via `handlers.OpenAPISpec()`, build `httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)`, assert `rec.Code == http.StatusOK` and `rec.Header().Get("Content-Type") == "application/json"`, and that the body is valid JSON with `"openapi": "3.0.x"` at the root (guards Pitfall 1 — the Swagger-2.0-vs-OpenAPI-3.0 root-key mixup) directly at the handler level, not just in the `internal/openapigen` package tests.

---

### `Makefile` (MODIFIED — new `generate-openapi` target)

**Analog:** itself, read in full (19 lines):
```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build lint test test-integration test-e2e clean

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o ./bin/nano-brain ./cmd/nano-brain/

lint:
	golangci-lint run ./...

test:
	go test -race -short ./...

test-integration:
	go test -race -tags=integration ./...

test-e2e:
	go test -race -tags=e2e ./...

clean:
	rm -rf ./bin/
```
New target must be added to the `.PHONY` line and follow the same tab-indented single-recipe-line style:
```makefile
.PHONY: build lint test test-integration test-e2e clean generate-openapi

generate-openapi:
	go run internal/openapigen/cmd/main.go   # or: swag init + conversion step, per planner's chosen trigger mechanism
```
Per RESEARCH.md Open Question #3: this repo has **no existing `go:generate` or Makefile codegen precedent** (the CONTEXT.md claim of a `sqlc generate` Makefile precedent does not hold — `sqlc generate` is manually invoked, documented only in CLAUDE.md's Quick Reference). The planner has full discretion on the exact recipe body; only the `.PHONY`/target-declaration *style* should mirror the existing 5 targets shown above.

---

### `README.md` / `docs/SETUP_AGENT.md` (MODIFIED — D-06 doc pointer)

**Analog:** itself — README.md has an existing "Development Setup" section (confirmed at line 497 via grep) and CLAUDE.md/AGENTS.md's "Quick Reference" convention (already includes `sqlc generate` as a one-line documented command per CLAUDE.md). Add a short section/line following the same one-liner style as the Quick Reference block in this project's CLAUDE.md:
```
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain   # Build
go test -race -short ./...                                 # Unit tests
go test -race -tags=integration ./...                      # Integration tests
sqlc generate                                              # SQL codegen
make generate-openapi                                      # OpenAPI spec regeneration   <- NEW LINE
```
And a short prose note near/in README.md's "Development Setup" section: `GET /api/openapi.json` serves the current OpenAPI 3.0 spec; regenerate via `make generate-openapi` after adding/changing routes.

---

## Shared Patterns

### Handler constructor idiom (constructor-returns-`echo.HandlerFunc`)
**Source:** `internal/server/handlers/query.go` lines 31-113
**Apply to:** All ~34 handler files being annotated, plus the new `openapi.go`
```go
func Query(searcher HybridSearcher, logger zerolog.Logger, rec ...*telemetry.Recorder) echo.HandlerFunc {
	return func(c echo.Context) error {
		// ...
	}
}
```
swag annotations always go above the **outer** named function, never the inner closure.

### `//go:embed` for committed generated artifacts
**Source:** `migrations/migrations.go` (full file, 6 lines)
**Apply to:** `internal/server/handlers/openapi.go` (serving the committed `openapi.json`)
```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

### Unexported same-package Request/Response structs
**Source:** `internal/server/handlers/health.go` lines 86-123, 197-202
**Apply to:** All handlers with lowercase struct names (~35 of 60 routes per RESEARCH.md Pitfall 2) — verify in Wave 0 that swag resolves these; do not export them preemptively.

### Route registration groups and their middleware stacks
**Source:** `internal/server/routes.go` lines 14-31, 61, 67
**Apply to:** Every `@Router` annotation and every `@Security` annotation — use the Route-to-group mapping table above as the canonical source for path prefixes and which of `workspaceMiddleware`/`workspaceRegisteredMiddleware`/`csrfMW` applies to each route.

### Test harness (mock interfaces + httptest)
**Source:** `internal/server/handlers/health_test.go` lines 1-60
**Apply to:** `internal/server/handlers/openapi_test.go`, and adapted for `internal/openapigen/*_test.go` (external `_test` package, `zerolog.Nop()`, `httptest.NewRequest`/`NewRecorder`/`e.NewContext`).

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `internal/server/doc.go` | config/doc-anchor | N/A | First swag "general API info" file in this repo — no prior art; must follow `swaggo/swag` README's documented `@title`/`@version`/`@securityDefinitions.apikey` syntax directly (see RESEARCH.md lines 231-250 for the exact block to adapt) |
| `internal/openapigen/openapi_gen_test.go` (drift detection) | test | transform/batch | No prior "regenerate and diff against committed artifact" test exists anywhere in this repo; follow RESEARCH.md's Code Examples section (lines 322-379) directly as the template, not a codebase analog |
| MCP-tunnel placeholder/anchor function for `/mcp` and `/sse` (Pattern 2, Pitfall 4) | doc-anchor | N/A | No existing unexported placeholder-function pattern in this codebase for documentation-only purposes; RESEARCH.md flags this needs empirical Wave-0 verification against swag's parser before locking the exact shape |

## Metadata

**Analog search scope:** `internal/server/handlers/`, `internal/server/`, `migrations/`, root (`Makefile`, `go.mod`, `README.md`)
**Files read in full:** `internal/server/handlers/query.go` (113 lines), `migrations/migrations.go` (6 lines), `internal/server/routes.go` (159 lines), `internal/server/handlers/health.go` (211 lines), `internal/server/handlers/health_test.go` (first 60 lines), `Makefile` (19 lines)
**Files grepped only (line refs, not full-read):** `internal/server/middleware.go` (function signatures at lines 28, 63, 111, 120, 177, 213), `internal/server/middleware_test.go`/`middleware_registered_test.go` (test naming convention)
**Pattern extraction date:** 2026-07-02
