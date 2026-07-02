---
phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60
reviewed: 2026-07-02T00:00:00Z
depth: deep
files_reviewed: 43
files_reviewed_list:
  - internal/config/defaults.go
  - internal/openapigen/generate.go
  - internal/openapigen/cmd/generate-openapi/main.go
  - internal/openapigen/openapi_gen_test.go
  - internal/openapigen/openapi_validate_test.go
  - internal/server/doc.go
  - internal/server/routes.go
  - internal/server/middleware.go
  - internal/server/middleware/auth.go
  - internal/server/handlers/openapi.go
  - internal/server/handlers/openapi_test.go
  - internal/server/handlers/openapi.json
  - internal/server/handlers/bm25.go
  - internal/server/handlers/code_summarize.go
  - internal/server/handlers/code_summarize_failures.go
  - internal/server/handlers/code_summarize_retry.go
  - internal/server/handlers/code_summarize_status.go
  - internal/server/handlers/collection.go
  - internal/server/handlers/config.go
  - internal/server/handlers/doctor.go
  - internal/server/handlers/document.go
  - internal/server/handlers/documents.go
  - internal/server/handlers/embed.go
  - internal/server/handlers/events.go
  - internal/server/handlers/flow.go
  - internal/server/handlers/flowchart.go
  - internal/server/handlers/get_document.go
  - internal/server/handlers/graph.go
  - internal/server/handlers/graph_neighborhood.go
  - internal/server/handlers/graph_overview.go
  - internal/server/handlers/graph_pagerank.go
  - internal/server/handlers/harvest.go
  - internal/server/handlers/health.go
  - internal/server/handlers/impact.go
  - internal/server/handlers/links.go
  - internal/server/handlers/multi_get.go
  - internal/server/handlers/protocol_doc.go
  - internal/server/handlers/query.go
  - internal/server/handlers/reindex.go
  - internal/server/handlers/reindex_cfg.go
  - internal/server/handlers/reload.go
  - internal/server/handlers/reset_workspace.go
  - internal/server/handlers/search.go
  - internal/server/handlers/stats.go
  - internal/server/handlers/summarize.go
  - internal/server/handlers/symbols.go
  - internal/server/handlers/tags.go
  - internal/server/handlers/ticket.go
  - internal/server/handlers/trace.go
  - internal/server/handlers/wakeup.go
  - internal/server/handlers/workspace.go
  - internal/server/handlers/workspace_remove.go
  - internal/server/handlers/workspace_resolve.go
  - docs/openapi.json
  - Makefile
  - .gitignore
  - go.mod
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 12: Code Review Report

**Reviewed:** 2026-07-02T00:00:00Z
**Depth:** deep
**Files Reviewed:** 43 (source) + generated artifacts
**Status:** issues_found

## Summary

Reviewed the full `feat/openapi-spec` diff against `origin/master` (27 commits, 74 files changed, +12941/-83). The generation pipeline (`swag` → `kin-openapi.openapi2conv.ToV3`) is correctly implemented and isolated: `swaggo/swag` and `getkin/kin-openapi` do not appear in `go list -deps ./cmd/nano-brain/...` or `./internal/server/handlers/...` — confirmed isolated to `internal/openapigen` and its `cmd/generate-openapi` subcommand, which the main binary never imports. `go build ./...` succeeds. `go test -race -short ./internal/openapigen/... ./internal/server/handlers/...` passes. The colocated `docs/openapi.json` and `internal/server/handlers/openapi.json` are byte-identical (verified via `diff`/`md5`), and the route-reconciliation slice's 55 entries were independently cross-checked against every path present in the generated spec and every registration in `routes.go` — the slice is accurate and complete. The ~34 "comment-only" handler diffs were verified line-by-line: all non-comment additions are `gofmt` realignment artifacts (struct tag/field padding) from adding doc-comments and one `swaggertype:"object"` tag, with zero logic, import, or signature changes.

However, one genuine, high-value finding emerged from focus area #1 (spot-checking `@Security` against real middleware): **`WakeUpHandler` shares a single swag doc-comment block for two differently-authenticated routes** (`GET /api/v1/wake-up`, no middleware, vs `POST /api/v1/wake-up`, gated by `workspaceMiddleware` on the `data` group) and carries **no `@Security` tag at all** — confirmed by inspecting the generated `docs/openapi.json`, where `paths["/api/v1/wake-up"].post.security` is absent. This causes the self-describing spec to misrepresent the POST route as requiring no authentication/workspace context, which is exactly the failure mode this phase's review focus area was designed to catch. Neither `TestOpenAPISpec_NoDrift` nor `TestOpenAPISpec_RouteReconciliation` would catch this class of bug, since neither asserts per-route `security` correctness against `routes.go`'s middleware groups.

All other spot-checked `@Security` annotations (write-group CSRF+WorkspaceRegisteredAuth handlers, data-group WorkspaceAuth handlers, api-group unauthenticated handlers, protocol-tunnel placeholder anchors) were verified accurate against their actual route group in `routes.go`.

## Critical Issues

### CR-01: `WakeUpHandler`'s shared doc-comment omits `@Security` for the authenticated POST route

**File:** `internal/server/handlers/wakeup.go:56-65`
**Issue:** `WakeUpHandler` is mounted twice in `routes.go`: `api.GET("/wake-up", wakeUp)` (line 124, unauthenticated — no workspace middleware) and `data.POST("/wake-up", wakeUp)` (line 125, inside the `data` group gated by `workspaceMiddleware(s.db)`). Both routes share one swag doc-comment block with two `@Router` lines. The `@Description` field mentions in prose that the POST path "requires @Security WorkspaceAuth", but no actual `@Security` swag tag is present. Verified directly against the generated artifact:
```
$ python3 -c "import json; d=json.load(open('docs/openapi.json')); print(d['paths']['/api/v1/wake-up']['post'].get('security'))"
None
```
Any client trusting this self-describing spec (the entire stated purpose of this phase) will believe `POST /api/v1/wake-up` needs no workspace/auth context, when in reality the request will 400/401 without it. This is exactly the "wrong/missing `@Security` tag misrepresents the API's real access requirements" risk the phase brief called out as the top security concern.
**Fix:** Split into two handler wrappers (or two swag anchor functions, following the `protocol_doc.go` placeholder-anchor pattern already used elsewhere in this phase) so each `@Router` line gets its own accurate `@Security` tag:
```go
// WakeUpHandlerPublic godoc
// @Summary  Get a session-start context summary (no workspace scope)
// @Router   /api/v1/wake-up [get]
func wakeUpPublicDoc() {}

// WakeUpHandlerScoped godoc
// @Summary  Get a session-start context summary (workspace-scoped)
// @Security WorkspaceAuth
// @Router   /api/v1/wake-up [post]
func wakeUpScopedDoc() {}
```
Then regenerate via `make generate-openapi` and add an assertion (e.g. in `TestOpenAPISpec_RouteReconciliation` or a new test) that `paths["/api/v1/wake-up"].post.security` is non-empty, so this class of drift is caught mechanically rather than by manual spot-check next time.

## Warnings

### WR-01: Route-reconciliation and drift tests cannot detect `@Security` mismatches

**File:** `internal/openapigen/openapi_gen_test.go:64-154`
**Issue:** `TestOpenAPISpec_RouteReconciliation` only asserts path-string presence; `TestOpenAPISpec_NoDrift` only asserts byte-equality against the committed spec. Neither test cross-references a route's actual middleware group in `routes.go` against its documented `security` requirements in the spec. This is precisely how CR-01 shipped undetected through the phase's own test suite (both tests pass on the current `docs/openapi.json`, wakeup.go bug included).
**Fix:** Add a `TestOpenAPISpec_SecurityMatchesMiddleware` (or extend the reconciliation test) that maps each `expectedRoutePaths` entry to its expected security scheme name (based on which of `api`/`data`/`write` group it's registered under) and asserts `paths[path][method].security` matches. This converts the "spot-check several by hand" review step into a permanent regression guard.

### WR-02: `securityDefinitions` in `doc.go` do not document the global HTTP Basic/Bearer auth layer

**File:** `internal/server/doc.go:14-28`
**Issue:** `middleware.Auth` (`internal/server/middleware/auth.go`) is applied globally via `s.echo.Use(...)` in `internal/server/middleware.go:31` and is the actual outer gate on every route (including ones tagged `WorkspaceAuth`/`WorkspaceRegisteredAuth`/`CSRFToken`) whenever `Server.Auth.Enabled=true` in config. The three `@securityDefinitions.apikey` blocks in `doc.go` document only the workspace/CSRF layer, never this Basic/Bearer HTTP auth layer. A client relying solely on the generated spec would not learn that the whole API can additionally require `Authorization: Basic`/`Bearer` credentials.
**Fix:** Add a fourth `@securityDefinitions.basic` (and/or `.apikey` with `@in header @name Authorization`) block documenting the global auth layer, and apply it via a top-level `@Security` covering all non-bypassed paths (or via swag's global-security mechanism), so the spec reflects that config-gated global auth exists independent of the per-route workspace/CSRF layers.

### WR-03: `OpenAPISpec()` silently discards the embed read error

**File:** `internal/server/handlers/openapi.go:29-34`
**Issue:** `specJSON, _ := openapiSpecFS.ReadFile("openapi.json")` discards the error. Because the file is `//go:embed`-ed, a missing file would fail at compile time, so this can't fail in a normally-built binary today — but if that invariant is ever broken (e.g. someone changes the embed pattern to a runtime path, or the embed FS is refactored), the handler would silently return `200 OK` with an empty body instead of a clear 500, making the failure mode much harder to diagnose than it needs to be.
**Fix:**
```go
func OpenAPISpec() echo.HandlerFunc {
	specJSON, err := openapiSpecFS.ReadFile("openapi.json")
	return func(c echo.Context) error {
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "openapi spec unavailable")
		}
		return c.Blob(http.StatusOK, "application/json", specJSON)
	}
}
```

## Info

### IN-01: `TestOpenAPISpec_RouteReconciliation`'s maintained path slice needs an explicit maintenance trigger

**File:** `internal/openapigen/openapi_gen_test.go:46-63`
**Issue:** The comment acknowledges `expectedRoutePaths` is hand-maintained and must be "cross-checked against routes.go on every edit to either file," but nothing enforces this beyond developer discipline — verified correct today (all 55 routes.go paths match), but the mechanism that keeps it correct going forward is purely social convention, which is the same failure mode that produced CR-01 in a different form (correct-today artifacts silently drifting).
**Fix:** Consider a lightweight `go generate`-driven check that greps `routes.go` for route-registration call patterns and asserts the count/set matches `expectedRoutePaths`, so an added-but-forgotten route registration fails CI even before considering swag annotations.

### IN-02: `openapigen.Generate` calls are position-independent on `mainAPIFile` but not documented as CWD-relative

**File:** `internal/openapigen/generate.go:39-50`
**Issue:** `Generate(searchDir, mainAPIFile)` is called with `"../.."`/`"internal/server/doc.go"` from the test (CWD = `internal/openapigen/`) and `"."`/`"internal/server/doc.go"` from `cmd/generate-openapi/main.go` (CWD = repo root via `make generate-openapi`). Both are correct today, but the function's doc-comment doesn't state that `mainAPIFile` is resolved relative to `searchDir` (per swag's `gen.Config` semantics) — a future caller invoking `Generate` from a different working directory without reading `gen.Config`'s upstream docs could pass an inconsistent pair and get a confusing swag error rather than a clear "these two args are wired together" contract violation.
**Fix:** Add a doc-comment note: "mainAPIFile is resolved relative to searchDir, matching swag's gen.Config.MainAPIFile semantics."

---

_Reviewed: 2026-07-02T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
