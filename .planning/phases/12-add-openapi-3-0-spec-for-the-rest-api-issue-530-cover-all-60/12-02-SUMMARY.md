---
phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60
plan: 02
subsystem: internal/server/handlers
tags: [openapi, swag, go, annotations, workspaces, documents, protocol-tunnel]

# Dependency graph
requires:
  - phase: 12-01
    provides: internal/openapigen.Generate() pipeline, internal/server/doc.go securityDefinitions (WorkspaceAuth/WorkspaceRegisteredAuth/CSRFToken), confirmed Assumption A1 (swag resolves unexported same-package structs)
provides:
  - swag doc-comment annotations on all 18 core-group handler files (workspace, config, doctor, collection, document, symbol, tag, wakeup, ticket, harvest, reload, events)
  - internal/server/handlers/protocol_doc.go — placeholder-anchor documentation for /mcp and /sse protocol-tunnel routes
  - Regenerated docs/openapi.json reflecting this plan's 21 new/updated routes (26 total paths after Plan 01 + Plan 02, before Plan 03's graph/search group lands)
affects:
  - phase: 12-04
    reason: final spec regeneration and drift/count reconciliation happens after both 12-02 and 12-03 land; this plan's route annotations, the /ui exclusion, and Assumption A2 outcome are inputs to that reconciliation

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "swag doc-comments placed directly above exported handler constructors, referencing real struct names for @Param/@Success (never handlers.ErrorResponse, which does not exist in this codebase — map[string]string used for error bodies instead)"
    - "@Router paths always the full mounted path from routes.go group nesting (e.g. /api/v1/collections/{name}, not /collections/:name) — swag has no visibility into echo.Group() nesting"
    - "Never-called placeholder-anchor functions (protocol_doc.go) as the swag doc-comment attachment point for echo.WrapHandler-registered routes with no named handler function"
    - "Dual @Router lines on a single doc-comment block (WakeUpHandler) when one Go function backs two routes with different middleware tiers; differing per-method @Security is not expressible on a single block, so the unsecured method is left unannotated and the secured method's requirement is called out in @Description instead"

key-files:
  created:
    - internal/server/handlers/protocol_doc.go
  modified:
    - internal/server/handlers/workspace.go
    - internal/server/handlers/workspace_remove.go
    - internal/server/handlers/workspace_resolve.go
    - internal/server/handlers/reset_workspace.go
    - internal/server/handlers/config.go
    - internal/server/handlers/doctor.go
    - internal/server/handlers/collection.go
    - internal/server/handlers/documents.go
    - internal/server/handlers/get_document.go
    - internal/server/handlers/multi_get.go
    - internal/server/handlers/document.go
    - internal/server/handlers/symbols.go
    - internal/server/handlers/tags.go
    - internal/server/handlers/wakeup.go
    - internal/server/handlers/ticket.go
    - internal/server/handlers/harvest.go
    - internal/server/handlers/reload.go
    - internal/server/handlers/events.go
    - docs/openapi.json

key-decisions:
  - "Regenerated and committed docs/openapi.json after each task (not deferred entirely to Plan 04) — the repo's pre-commit harness-check.sh runs go test -race -short ./... on every commit, which includes internal/openapigen's TestOpenAPISpec_NoDrift. Since that test regenerates from working-tree annotations and diffs against the committed spec, any new annotation without a matching regen fails the commit gate. Plan 04 still owns the FINAL full-coverage regen after Plan 03's routes also land — these interim regens are idempotent and don't preclude that."
  - "Assumption A2 (RESEARCH.md) CONFIRMED PASS: the placeholder-anchor function pattern for /mcp and /sse needed no fallback — swag correctly parsed mcpProtocolDoc()/sseProtocolDoc() and produced complete path entries for all 5 method registrations."
  - "/ui deliberately excluded from the spec — it's a static HTML redirect page, not a JSON REST endpoint, per PATTERNS.md's planner-discretion note."
  - "WakeUpHandler's GET method left @Security-free (its real registration IS unauthenticated, api group) while its POST method's workspace requirement is documented only in @Description, since swag cannot express differing security per @Router line within one doc-comment block — this is the plan's own pre-approved fallback (AC-2 'document presence' floor, D-03)."

patterns-established:
  - "Every handler in this plan's 18 files got at minimum @Summary + @Router + method; handlers with typed structs also got @Param/@Success referencing the real (often unexported) struct name, continuing Plan 01's proven Assumption A1 pattern."

requirements-completed: [D-03, D-04]

coverage:
  - id: D1
    description: "Core-group handler routes (workspace/config/doctor/collection/document/symbol/tag/wakeup/ticket group, ~21 routes) carry @Summary, @Router (full mounted path), method, and request/response schema where a typed struct exists"
    requirement: "D-03"
    verification:
      - kind: unit
        ref: "go build ./internal/server/... (task 1+2 verify commands)"
        status: pass
      - kind: unit
        ref: "internal/openapigen#TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema"
        status: pass
    human_judgment: false
  - id: D2
    description: "@Security tiers (WorkspaceAuth / WorkspaceRegisteredAuth+CSRFToken) applied per real middleware group from routes.go, not guessed"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "manual inspection of generated docs/openapi.json security arrays for /api/v1/write (WorkspaceRegisteredAuth+CSRFToken), /api/v1/collections and /api/v1/events (WorkspaceAuth), /api/v1/init and /api/harvest (none)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Protocol-tunnel routes /mcp (GET/POST/DELETE) and /sse (GET/POST) documented via placeholder-anchor, presence only, no full schema"
    requirement: "D-03"
    verification:
      - kind: unit
        ref: "go vet ./internal/server/handlers/... plus generated docs/openapi.json containing /mcp and /sse paths with all 5 methods"
        status: pass
    human_judgment: false
  - id: D4
    description: "Whole project builds and existing test suite stays green — annotations are pure comments, zero behavior change"
    verification:
      - kind: unit
        ref: "go test -race -short ./... -count=1 (full repo sweep, run after each task)"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-07-02
status: complete
---

# Phase 12 Plan 02: Core Handler Group + Protocol-Tunnel OpenAPI Annotations Summary

**Annotated all 18 core-group REST handlers (workspaces, config, doctor, collections, documents, symbols, tags, wake-up, tickets, harvest, reload, events) with swag doc-comments carrying correct full paths and per-tier @Security, plus a placeholder-anchor documenting the /mcp and /sse protocol-tunnel routes — bringing the committed OpenAPI 3.0 spec to 26 paths.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3 (all automated)
- **Files modified:** 18 handler files + docs/openapi.json (regenerated 3 times)
- **Files created:** 1 (internal/server/handlers/protocol_doc.go)

## Accomplishments

- Annotated `InitWorkspace`, `ListWorkspaces`, `RemoveWorkspace`, `ResolveWorkspace`, `ResetWorkspace`, `GetConfig`, `PatchConfig`, `Doctor` with full `/api/v1/*` paths, no `@Security` (unauthenticated `api` group per routes.go)
- Annotated `AddCollection`, `ListCollectionsHandler`, `RenameCollectionHandler`, `RemoveCollection`, `ListTags`, `ListDocuments`, `DeleteDocument`, `GetDocument`, `MultiGet`, `ListSymbols`, `EventsHandler` with `@Security WorkspaceAuth` (data group)
- Annotated `WriteDocument` with both `@Security WorkspaceRegisteredAuth` and `@Security CSRFToken` (write group — the only write-tier route in this plan)
- Annotated `WakeUpHandler` with dual `@Router` lines (GET and POST, same handler backs both) — GET left unauthenticated per its real registration, POST's workspace requirement documented in `@Description` since swag can't express differing per-method security on one doc block
- Annotated `TicketHandler` (unauthenticated, cross-workspace query) and `TriggerHarvest`/`ReloadConfig` with their real top-level paths (`/api/harvest`, `/api/reload-config` — explicitly NOT under `/api/v1`)
- Created `internal/server/handlers/protocol_doc.go` with never-called `mcpProtocolDoc()`/`sseProtocolDoc()` placeholder functions documenting `/mcp` (GET/POST/DELETE) and `/sse` (GET/POST) presence per D-03 — no full schema, since these are protocol tunnels not JSON REST endpoints
- Confirmed Assumption A2 PASS: swag parsed the placeholder-anchor pattern with zero fallback needed
- Regenerated `docs/openapi.json` after each task to keep the repo's `TestOpenAPISpec_NoDrift` and `TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema` tests green under the pre-commit harness gate — spec grew from 3 paths (Plan 01) to 24 (after Task 2) to 26 (after Task 3, adding /mcp and /sse)
- `/ui` deliberately excluded (static HTML redirect, not JSON REST) — documented in deferred-items.md for Plan 04's reconciliation

## Task Commits

1. **Task 1: Annotate workspace, config, doctor, and status routes** - `5f76524` (feat)
2. **Task 2: Annotate collection, document, symbol, tag, wakeup, ticket, harvest, reload, events routes** - `48729a5` (feat)
3. **Task 3: Document protocol-tunnel routes via placeholder-anchor function** - `5556293` (feat)

## Files Created/Modified

- `internal/server/handlers/workspace.go` - `InitWorkspace`/`ListWorkspaces` annotated
- `internal/server/handlers/workspace_remove.go` - `RemoveWorkspace` annotated, `{hash}` path-param syntax
- `internal/server/handlers/workspace_resolve.go` - `ResolveWorkspace` annotated
- `internal/server/handlers/reset_workspace.go` - `ResetWorkspace` annotated
- `internal/server/handlers/config.go` - `GetConfig`/`PatchConfig` annotated
- `internal/server/handlers/doctor.go` - `Doctor` annotated
- `internal/server/handlers/collection.go` - `AddCollection`/`ListCollectionsHandler`/`RenameCollectionHandler`/`RemoveCollection` annotated with `WorkspaceAuth`
- `internal/server/handlers/documents.go` - `ListDocuments`/`DeleteDocument` annotated with `WorkspaceAuth`
- `internal/server/handlers/get_document.go` - `GetDocument` annotated with `WorkspaceAuth`
- `internal/server/handlers/multi_get.go` - `MultiGet` annotated with `WorkspaceAuth`
- `internal/server/handlers/document.go` - `WriteDocument` annotated with `WorkspaceRegisteredAuth`+`CSRFToken`
- `internal/server/handlers/symbols.go` - `ListSymbols` annotated with `WorkspaceAuth`
- `internal/server/handlers/tags.go` - `ListTags` annotated with `WorkspaceAuth`
- `internal/server/handlers/wakeup.go` - `WakeUpHandler` dual-router annotation
- `internal/server/handlers/ticket.go` - `TicketHandler` annotated (unauthenticated, cross-workspace)
- `internal/server/handlers/harvest.go` - `TriggerHarvest` annotated with real `/api/harvest` path
- `internal/server/handlers/reload.go` - `ReloadConfig` annotated with real `/api/reload-config` path
- `internal/server/handlers/events.go` - `EventsHandler` annotated as `text/event-stream`, `WorkspaceAuth`
- `internal/server/handlers/protocol_doc.go` - NEW: placeholder-anchor functions for `/mcp` and `/sse`
- `docs/openapi.json` - regenerated, now 26 paths (health/status/version from Plan 01 + this plan's 21 core-group routes + /mcp + /sse)

## Decisions Made

- Regenerated and committed `docs/openapi.json` incrementally (once per task) rather than waiting entirely for Plan 04. The plan's own task-level `<verify>` blocks scope to `internal/server/...` and never touch `internal/openapigen`, but this repo's pre-commit hook (`scripts/harness-check.sh in-progress`) runs the full `go test -race -short ./...` sweep on every commit, including `internal/openapigen`'s drift-detection test (`TestOpenAPISpec_NoDrift`), which regenerates from working-tree annotations and byte-diffs against the committed spec. Any handler annotation without a matching regen fails that gate. Confirmed via a scratch worktree checkout of pre-Task-1 HEAD that the drift test passed before my edits and only failed after adding new annotations — this is an unavoidable consequence of annotation-only work under this repo's commit-time enforcement, not a plan defect. Plan 04 still performs the FINAL full-coverage regen once Plan 03's graph/search routes also land; these interim regens are idempotent (deterministic JSON marshaling, confirmed in Plan 01) and simply keep intermediate commits green.
- Added a minimal `@Success 200` line to both protocol-tunnel placeholder anchors (not in the plan's literal example) after `TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema` failed with "the responses object MUST contain at least one response code" — OpenAPI 3.0 requires every operation to declare at least one response, and the plan's own Pattern 2 example omitted `@Success`. This is a Rule 1 (auto-fix bug) fix, not a schema-completeness violation of D-03: the response has no schema body, only a generic description ("protocol response (not JSON — see MCP spec)"), preserving the presence-only intent.
- No struct-exporting or architectural changes needed — all annotations reference existing (often unexported) struct names, consistent with Plan 01's confirmed Assumption A1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Regenerated and committed docs/openapi.json per task to keep the repo-wide pre-commit hook green**
- **Found during:** Task 1 commit attempt
- **Issue:** `git commit` was blocked by `scripts/harness-check.sh in-progress` (check 2.3 "Validation ladder passes" — `go test -race -short ./...`). The repo-wide sweep includes `internal/openapigen.TestOpenAPISpec_NoDrift`, which failed because my new handler annotations changed what `Generate()` produces versus the committed `docs/openapi.json` (still Plan 01's health-only spec).
- **Fix:** Ran `make generate-openapi` and committed the regenerated `docs/openapi.json` alongside each task's handler changes. Verified via a scratch `git worktree add --detach` checkout of pre-Task-1 HEAD that the drift test passed there, confirming my edits (not a pre-existing issue) caused the drift, and that regeneration was the correct, minimal fix.
- **Files modified:** `docs/openapi.json` (all 3 task commits)
- **Verification:** `go test -race -short ./... -count=1` green after each regeneration; `go build ./...` green
- **Committed in:** `5f76524`, `48729a5`, `5556293` (part of each task commit)

**2. [Rule 1 - Bug] Added @Success to protocol-tunnel placeholder anchors**
- **Found during:** Task 3
- **Issue:** `TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema` failed after adding `protocol_doc.go` per the plan's literal example (which had no `@Success` line): "invalid operation DELETE: the responses object MUST contain at least one response code" — OpenAPI 3.0 requires every operation to declare a response.
- **Fix:** Added `@Success 200 "protocol response (not JSON — see MCP spec)"` to both `mcpProtocolDoc()` and `sseProtocolDoc()` — a generic description-only response, no schema body, preserving D-03's presence-only intent.
- **Files modified:** `internal/server/handlers/protocol_doc.go`
- **Verification:** `go test -race -short ./internal/openapigen/... -count=1` green (`TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema` passes)
- **Committed in:** `5556293` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking — commit-gate drift, 1 bug — missing required OpenAPI response)
**Impact on plan:** Both fixes necessary to keep every commit green under this repo's mandatory pre-commit harness; neither expands scope beyond the plan's stated files (`docs/openapi.json` is a generated artifact this plan's changes naturally affect, and `protocol_doc.go`'s fix is a one-line addition matching the plan's own presence-only intent). No architectural changes, no scope creep.

## Issues Encountered

- Pre-existing gofmt formatting issue in `internal/server/handlers/reset_workspace.go` and `internal/server/handlers/workspace_remove.go` (an if/else block indented one tab short of gofmt's expectation), confirmed present before this plan's edits via a scratch worktree diff against HEAD. Out of scope per the deviation rules' scope boundary (not caused by this plan's changes) — logged to `deferred-items.md`, not fixed.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 12-03 (graph/search handler group) runs in parallel in a separate worktree, touching disjoint handler files — no conflict expected on handler source, but both plans independently regenerate `docs/openapi.json`; Plan 04 must reconcile/re-regenerate the final merged spec after both land (this was always Plan 04's job per the phase plan).
- `docs/openapi.json` after this plan: 26 paths (Plan 01's health/status/version + this plan's 21 core-group routes + 2 protocol-tunnel entries).
- `/ui` exclusion and Assumption A2 PASS outcome are recorded in `deferred-items.md` for Plan 04's route-count reconciliation.
- Both `TestOpenAPISpec_NoDrift` and `TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema` pass against this plan's committed spec; Plan 04 will need to re-run `make generate-openapi` once more after merging with Plan 03's routes to produce the truly final spec.

## Self-Check: PASSED

All 18 modified handler files and the new `internal/server/handlers/protocol_doc.go` verified present on disk. All 3 task commits (`5f76524`, `48729a5`, `5556293`) verified present via `git log --oneline`. `go build ./internal/server/...` and `go test -race -short ./internal/server/... -count=1` green. Full-repo `go build ./...` and `go test -race -short ./... -count=1` green (including `internal/openapigen`'s drift and schema-validation tests). Generated `docs/openapi.json` inspected directly: 26 paths, all Task 1/2/3 routes present with correct methods, `@Security` tiers verified via JSON inspection for `/api/v1/write` (WorkspaceRegisteredAuth+CSRFToken), `/api/v1/collections`/`/api/v1/events` (WorkspaceAuth), and `/api/v1/init`/`/api/harvest` (none).

---
*Phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60*
*Completed: 2026-07-02*
