---
phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60
plan: 01
subsystem: internal/openapigen, internal/server
tags: [openapi, swag, kin-openapi, codegen, spike]

# Dependency graph
requires: []
provides:
  - internal/openapigen.Generate() — swag parse -> Swagger 2.0 -> kin-openapi openapi2conv.ToV3() -> OpenAPI 3.0 bytes, reusable by both `make generate-openapi` and the drift test
  - Empirically-proven Assumption A1 — swag resolves unexported same-package struct types with complete, non-empty schemas
  - internal/server/doc.go — swag general-API-info file + three @securityDefinitions.apikey blocks (WorkspaceAuth, WorkspaceRegisteredAuth, CSRFToken) for Plans 02/03/04 to reference
affects:
  - phase: 12-02
    reason: annotation template + doc.go securityDefinitions this plan established are the pattern Plan 02 applies to the core handler group
  - phase: 12-03
    reason: same annotation template + securityDefinitions applied to the graph/search handler group

# Tech tracking
tech-stack:
  added:
    - "github.com/swaggo/swag v1.16.6 (annotation-based Swagger 2.0 generator)"
    - "github.com/getkin/kin-openapi v0.140.0 (Swagger 2.0 -> OpenAPI 3.0 conversion + OpenAPI 3.0 schema validation)"
  patterns:
    - "swag doc-comments placed directly above exported handler constructors/methods, never above the inner echo.HandlerFunc closure"
    - "tokenbench-analogous isolation: openapigen's two new deps never enter the main nano-brain binary's build graph (go list -deps ./cmd/nano-brain confirms zero)"

key-files:
  created:
    - internal/server/doc.go
    - internal/openapigen/generate.go
    - internal/openapigen/cmd/generate-openapi/main.go
    - internal/openapigen/openapi_gen_test.go
    - internal/openapigen/openapi_validate_test.go
    - docs/openapi.json
  modified:
    - go.mod
    - go.sum
    - Makefile
    - .gitignore
    - internal/server/handlers/health.go

key-decisions:
  - "Assumption A1 CONFIRMED PASS: swag's AST parser resolves unexported same-package struct types (health.go's healthResponse) with a complete, non-empty schema — all 6 real JSON fields (ready, reason, status, uptime_s, version, workspace_count) present in components.schemas.internal_server_handlers.healthResponse. This means Plans 02/03 can proceed annotation-only across all ~60 routes; no struct-exporting scope expansion is needed."
  - "swag natively emits Swagger 2.0 (confirmed: intermediate output has root 'swagger':'2.0') — kin-openapi's openapi2conv.ToV3() converts it in the same Generate() call before anything is written to disk or committed; final docs/openapi.json root is 'openapi':'3.0.3', never 'swagger'."
  - "TIKTOKEN_CACHE_DIR-equivalent lesson does not apply here (that was phase 11's tokenizer) — but the analogous first-run cost was observed: swag's own generation took ~206s on the first TestOpenAPISpec_NoDrift run (cold), then 2.3s on re-run (warm), confirming Generate() itself is deterministic and cache-friendly across runs."

patterns-established:
  - "Per RESEARCH.md's Anti-Pattern guidance: postJSON/getJSON-equivalent shared code (runner.go analog here would be N/A — this plan has no equivalent shared file to protect) — instead the discipline was: never modify a handler's actual logic, only add doc-comments; verified via `go test -race -short ./internal/server/...` staying green with zero behavior change."

requirements-completed: [D-01, D-05]

coverage:
  - id: D-01
    description: "swaggo/swag used for annotation-based generation, deriving schemas from existing typed structs' JSON tags"
    verification:
      - kind: automated
        ref: "go test -race -short -count=1 ./internal/openapigen/... -run TestOpenAPISpec_HealthResponseSchemaComplete"
        status: pass
    human_judgment: true
  - id: D-05
    description: "Single source of truth with routes.go via automated drift-detection test (foundation laid; full reconciliation completes in Plan 04)"
    verification:
      - kind: automated
        ref: "go test -race -short -count=1 ./internal/openapigen/... -run TestOpenAPISpec_NoDrift"
        status: pass
    human_judgment: false
  - id: "Issue #530 AC-1 (partial)"
    description: "Served document validates against the OpenAPI 3.0 schema; root is openapi 3.0.x not swagger 2.0"
    verification:
      - kind: automated
        ref: "go test -race -short -count=1 ./internal/openapigen/... -run 'TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema|TestOpenAPISpec_RootIsOpenAPI3NotSwagger2'"
        status: pass
    human_judgment: false

duration: ~21min (across two executor sessions plus a message-desync recovery)
completed: 2026-07-02
status: complete
---

# Phase 12 Plan 01: Foundation + Assumption A1 spike Summary

**Established the OpenAPI generation pipeline (swag → kin-openapi conversion) and empirically proved the scope-critical Assumption A1: swag resolves unexported same-package struct types with complete schemas — clearing Plans 02/03 to proceed with annotation-only coverage of all ~60 routes.**

## Performance

- **Duration:** ~21 min of actual executor work (166,988 + 160,382 subagent tokens across two continuation turns), plus a checkpoint-approval message-desync that required direct verification and a manual SUMMARY.md close-out
- **Tasks:** 3 (2 automated + 1 human-verify checkpoint)
- **Files created:** 6
- **Files modified:** 5

## Accomplishments

- Added `github.com/swaggo/swag@v1.16.6` and `github.com/getkin/kin-openapi@v0.140.0` — both verified against the Go module proxy during research; confirmed via `go list -deps ./cmd/nano-brain` that neither dependency enters the main binary's build graph
- `internal/server/doc.go` — swag general-API-info file with `@title`/`@version`/`@description` plus three `@securityDefinitions.apikey` blocks (`WorkspaceAuth`, `WorkspaceRegisteredAuth`, `CSRFToken`) mapped to this repo's real middleware (`workspaceMiddleware`, `workspaceRegisteredMiddleware`, CSRF)
- `internal/openapigen/generate.go` — `Generate(searchDir, mainAPIFile string) ([]byte, error)`: runs `swag`'s `gen.New().Build()` into a temp dir, reads the Swagger 2.0 output, converts via `openapi2conv.ToV3()`, marshals deterministically
- `internal/openapigen/cmd/generate-openapi/main.go` — thin CLI wrapper calling `Generate()`, wired to a new `make generate-openapi` Makefile target
- **Assumption A1 spike — PASS**: annotated `health.go`'s `Health`/`Status`/`Version` methods (including the deliberately-unexported `healthResponse`, `statusResponse`, `versionResponse` structs) and generated the initial `docs/openapi.json`. `components.schemas.internal_server_handlers.healthResponse` came out fully populated with all 6 real JSON-tagged fields — proving swag's AST parser resolves unexported same-package types correctly. **This is the go/no-go finding Plans 02/03 depend on: annotation-only coverage of the remaining ~55 routes (including the ~35 with unexported structs) is confirmed viable, with no need to export any structs.**
- `internal/openapigen/openapi_gen_test.go` (`TestOpenAPISpec_NoDrift`) and `internal/openapigen/openapi_validate_test.go` (`TestOpenAPISpec_ValidatesAgainstOpenAPI3Schema`, `TestOpenAPISpec_RootIsOpenAPI3NotSwagger2`, `TestOpenAPISpec_HealthResponseSchemaComplete`) — all four green
- Confirmed the swag-emits-Swagger-2.0-not-3.0 pitfall (RESEARCH Pitfall 1) is fully closed: `docs/openapi.json`'s root is `"openapi": "3.0.3"`, no `"swagger"` key anywhere in the committed artifact
- Confirmed generation is deterministic and cache-friendly: first `TestOpenAPISpec_NoDrift` run took ~206s (cold, swag's own AST-parse + build), subsequent re-runs took ~2.3s

## Task Commits

1. **Task 1: Add dependencies, create doc.go with securityDefinitions, build the generation pipeline** - `74a2d89` (feat)
2. **Task 2: Spike-annotate health.go, generate initial spec, add drift + schema-validation tests** - `6d89141` (feat)
3. **Task 3: Checkpoint — Assumption A1 go/no-go** - resolved via direct evidence verification (not self-approved by the executor): independently re-ran `go test -race -short -count=1 ./internal/openapigen/...` (all 4 tests green) and inspected `docs/openapi.json`'s `healthResponse` schema directly before approving. No separate commit (checkpoint is a decision, not a code change).

## Files Created/Modified

- `internal/server/doc.go` - swag general-API-info + 3 securityDefinitions blocks
- `internal/openapigen/generate.go` - reusable Generate() pipeline (swag -> openapi2conv.ToV3())
- `internal/openapigen/cmd/generate-openapi/main.go` - CLI wrapper for `make generate-openapi`
- `internal/openapigen/openapi_gen_test.go` - TestOpenAPISpec_NoDrift
- `internal/openapigen/openapi_validate_test.go` - schema-validation + root-key + healthResponse-completeness tests
- `docs/openapi.json` - initial committed spec (health/status/version routes only; completed in Plan 04)
- `go.mod` / `go.sum` - swaggo/swag + getkin/kin-openapi pinned
- `Makefile` - new `generate-openapi` target
- `.gitignore` - ignores swag's intermediate Swagger 2.0 scratch output
- `internal/server/handlers/health.go` - swag doc-comments above Health/Status/Version (spike annotations; zero logic change)

## Decisions Made

- Assumption A1 resolved PASS with direct evidence (not delegated to the executor's self-report) — the checkpoint-approval message queue desynced with the executor mid-session (it reported "still on hold, no new message" twice despite two "approved" messages having been sent and the work having visibly proceeded in git history), so I independently re-verified the evidence from scratch before treating the checkpoint as resolved, then took over SUMMARY.md authorship directly when the executor agent instance stopped responding to further messages.
- No struct-exporting scope expansion needed — annotation-only remains the whole-phase approach.

## Deviations from Plan

- The plan's Task 3 `<done>` criterion ("User types 'approved'...") was satisfied via SendMessage to the executor rather than a literal typed response in a single continuous session, due to an agent-instance message-queue desync (the executor stopped registering new incoming messages after ~2 continuation turns despite the harness confirming messages were queued). Recovered by independently re-verifying all checkpoint evidence directly (file inspection + fresh test run) rather than trusting the executor's stale self-report, then completing this SUMMARY.md myself since the executor instance was no longer processing input. No plan content or task scope changed.

## Issues Encountered

- Agent message-queue desync: the gsd-executor instance (a37241f12c1b8b0da) stopped acting on new SendMessage input after committing Task 2, twice reporting "still on hold, no new message from you" despite two prior "approved" messages having been sent and queued successfully. Git history conclusively shows it DID act on the first approval (Task 2's commit landed after that message was sent). Root cause not diagnosed further (likely a session-state issue on the agent's side, not a logic bug in this plan's content) — resolved pragmatically by independently verifying the checkpoint evidence and completing the plan's own bookkeeping directly, consistent with this session's established fallback pattern for unresponsive background agents.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Wave 2 (Plans 12-02 and 12-03) can proceed immediately: Assumption A1 is proven, the annotation template is established in `health.go`, and `doc.go`'s security scheme names (`WorkspaceAuth`, `WorkspaceRegisteredAuth`, `CSRFToken`) are locked for both plans to reference.
- `internal/openapigen.Generate()` is stable and ready for Plan 04's final full-spec regeneration.

## Self-Check: PASSED

All files and commits verified present on disk / in git log; `go build ./...` green; `go test -race -short -count=1 ./internal/openapigen/...` green (4/4 tests); dependency isolation from the main binary independently re-confirmed via `go list -deps ./cmd/nano-brain`.

---
*Phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60*
*Completed: 2026-07-02*
