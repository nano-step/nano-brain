---
phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60
plan: 03
subsystem: internal/server/handlers, internal/openapigen
tags: [openapi, swag, go, annotations, graph, search, embed, reindex, summarize]

# Dependency graph
requires:
  - phase: 12-01
    provides: swag annotation template, doc.go @securityDefinitions (WorkspaceAuth, WorkspaceRegisteredAuth, CSRFToken), internal/openapigen.Generate() pipeline
provides:
  - swag doc-comment annotations above all 21 exported handler constructors/methods in the graph/search handler group (search, graph, flow, links, embed, reindex, summarize, code-summarize, stats)
  - flowchartResponse.CFG swaggertype:"object" fix so swag resolves json.RawMessage fields
  - interim regenerated docs/openapi.json covering this plan's + Plan 01's annotated routes (29 paths total after this plan)
affects:
  - phase: 12-04
    reason: Plan 04 does the final regen once Plan 02 also lands; this plan's annotations are additive input to that final spec

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "swag doc-comments placed directly above exported handler constructors/methods, never above the inner echo.HandlerFunc closure (established in 12-01, applied here to 21 more handlers)"
    - "json.RawMessage fields need an explicit swaggertype:\"object\" struct tag — swag's AST parser cannot resolve json.RawMessage's underlying type on its own"

key-files:
  created: []
  modified:
    - internal/server/handlers/query.go
    - internal/server/handlers/search.go
    - internal/server/handlers/bm25.go
    - internal/server/handlers/graph.go
    - internal/server/handlers/graph_overview.go
    - internal/server/handlers/graph_neighborhood.go
    - internal/server/handlers/graph_pagerank.go
    - internal/server/handlers/impact.go
    - internal/server/handlers/trace.go
    - internal/server/handlers/flow.go
    - internal/server/handlers/flowchart.go
    - internal/server/handlers/links.go
    - internal/server/handlers/embed.go
    - internal/server/handlers/reindex.go
    - internal/server/handlers/reindex_cfg.go
    - internal/server/handlers/summarize.go
    - internal/server/handlers/code_summarize.go
    - internal/server/handlers/code_summarize_status.go
    - internal/server/handlers/code_summarize_failures.go
    - internal/server/handlers/code_summarize_retry.go
    - internal/server/handlers/stats.go
    - docs/openapi.json

key-decisions:
  - "flowchartResponse.CFG (json.RawMessage) required swaggertype:\"object\" — swag's ParseComment errored with 'cannot find type definition: json.RawMessage' without it. Fixed inline (Rule 3 blocking-issue) rather than deferring to Plan 04."
  - "Regenerated docs/openapi.json after each task (via make generate-openapi) to keep internal/openapigen.TestOpenAPISpec_NoDrift green under the repo-wide pre-commit hook (go test -race -short ./...), even though this plan's files_modified list does not include docs/openapi.json. This is an interim, additive regeneration — Plan 04's own dedicated regen (after Plan 02 also lands) supersedes it; no conflict expected since both regenerations are pure functions of the current annotation state."
  - "GraphPageRankCompute and other no-request-body handlers omit @Param request body (they take no JSON payload, only workspace from context) — @Success/@Security/@Router only."

patterns-established:
  - "Per-route @Security tier derived directly from routes.go group membership (data vs write), never guessed from route name — neighborhood/pagerank/flow-materialize are graph-shaped but write-tier because routes.go registers them on the `write` group."

requirements-completed: [D-03, D-04]

coverage:
  - id: D1
    description: "Search + graph read routes (Query, VectorSearch, BM25Search, GraphQuery, GraphOverview, GraphImpact, GraphTrace, GraphFlowchart) annotated with @Summary/@Router/@Security WorkspaceAuth and real request/response struct schemas"
    requirement: "D-03"
    verification:
      - kind: unit
        ref: "go build ./internal/server/... && gofmt -l (Task 1 verify command)"
        status: pass
      - kind: unit
        ref: "go test -race -short -count=1 ./internal/openapigen/... (post-regen drift + validation tests)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Flow/links/neighborhood/pagerank routes annotated with correct per-route tier — GraphFlow/ListFlowEndpoints/Backlinks/ResolveLink as data-tier WorkspaceAuth; FlowMaterialize/GraphNeighborhood/GraphPageRankCompute as write-tier WorkspaceRegisteredAuth+CSRFToken"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "go test -race -short -count=1 ./internal/server/... (Task 2 verify command)"
        status: pass
      - kind: other
        ref: "python3 spec inspection of docs/openapi.json security arrays per path (manual check documented in this SUMMARY)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Embed/reindex/reindex-cfg/summarize/code-summarize (9 write routes) annotated with WorkspaceRegisteredAuth+CSRFToken; stats annotated with WorkspaceAuth on the (h *StatsHandler) Handle method receiver idiom"
    requirement: "D-04"
    verification:
      - kind: unit
        ref: "go test -race -short -count=1 ./internal/server/... (Task 3 verify command)"
        status: pass
      - kind: other
        ref: "grep -rn handlers.ErrorResponse internal/server/handlers/*.go returns no matches"
        status: pass
    human_judgment: false

duration: ~35min
completed: 2026-07-02
status: complete
---

# Phase 12 Plan 03: Graph/Search Handler Group Annotations Summary

**Added swag doc-comment annotations to all 21 graph/search/embed/summarize/stats handler files (search, graph traversal, flow, links, embed, reindex, code-summarize, stats), correctly splitting @Security tiers between data (WorkspaceAuth) and write (WorkspaceRegisteredAuth + CSRFToken) per each route's actual routes.go group membership.**

## Performance

- **Duration:** ~35 min
- **Tasks:** 3 (all automated)
- **Files modified:** 21 handler files + docs/openapi.json (regenerated 3 times, once per task)

## Accomplishments

- **Task 1 (search + graph read routes):** Annotated `Query`, `VectorSearch`, `BM25Search`, `GraphQuery`, `GraphOverview`, `GraphImpact`, `GraphTrace`, `GraphFlowchart` — all data-tier (`WorkspaceAuth`), full `/api/v1/...` `@Router` paths cross-referenced against `routes.go`, real request/response struct names (`QueryRequest`/`SearchResponse`, `graphQueryRequest`/`graphQueryResponse`, etc.), `@Failure 400 {object} map[string]string` (no `handlers.ErrorResponse`, which does not exist).
- **Task 2 (flow/links/neighborhood/pagerank, mixed tiers):** Annotated `GraphFlow`, `ListFlowEndpoints`, `Backlinks`, `ResolveLink` as data-tier; `FlowMaterialize`, `GraphNeighborhood`, `GraphPageRankCompute` as write-tier (`WorkspaceRegisteredAuth` + `CSRFToken`) per `routes.go`'s actual `write` group registrations (lines 102, 115, 116) — confirming the plan's warning that not all graph routes are data-tier. `Backlinks` uses swag's `{doc_id}` path-param syntax matching Echo's `:doc_id`.
- **Task 3 (embed/reindex/summarize/code-summarize/stats):** Annotated all 9 write-tier routes (`TriggerEmbed`, `TriggerReindex`, `TriggerUpdate`, `ReindexCFG`, `TriggerSummarize`, `TriggerCodeSummarize`, `GetCodeSummarizeStatus`, `GetCodeSummarizeFailures`, `RetryCodeSummarize`, `RetryAllCodeSummarize`) with `WorkspaceRegisteredAuth` + `CSRFToken`, and `stats.go`'s `(h *StatsHandler) Handle` method (data-tier `WorkspaceAuth`) using the method-receiver idiom matching `health.go`, not the constructor-closure idiom used elsewhere.
- Fixed a swag parse failure on `flowchartResponse.CFG` (`json.RawMessage`) by adding `swaggertype:"object"` — swag's AST parser cannot resolve `json.RawMessage`'s underlying type without this hint.
- Regenerated `docs/openapi.json` after each task via `make generate-openapi` to keep `internal/openapigen.TestOpenAPISpec_NoDrift` green under the repo's pre-commit hook, which runs `go test -race -short ./...` across the whole repository (not scoped to this plan's `internal/server/...`).

## Task Commits

1. **Task 1: Annotate search + graph read routes (data group)** - `892744d` (feat)
2. **Task 2: Annotate flow, links, neighborhood, pagerank routes (mixed tiers)** - `47eaeef` (feat)
3. **Task 3: Annotate embed, reindex, summarize, code-summarize, and stats routes** - `d57c4d9` (feat)

## Files Created/Modified

- `internal/server/handlers/query.go` - `@Router /api/v1/query [post]`, WorkspaceAuth
- `internal/server/handlers/search.go` - `@Router /api/v1/vsearch [post]`, WorkspaceAuth
- `internal/server/handlers/bm25.go` - `@Router /api/v1/search [post]`, WorkspaceAuth
- `internal/server/handlers/graph.go` - `@Router /api/v1/graph/query [post]`, WorkspaceAuth
- `internal/server/handlers/graph_overview.go` - `@Router /api/v1/graph/overview [post]`, WorkspaceAuth
- `internal/server/handlers/impact.go` - `@Router /api/v1/graph/impact [post]`, WorkspaceAuth
- `internal/server/handlers/trace.go` - `@Router /api/v1/graph/trace [post]`, WorkspaceAuth
- `internal/server/handlers/flowchart.go` - `@Router /api/v1/graph/flowchart [post]`, WorkspaceAuth; `swaggertype:"object"` fix on CFG field
- `internal/server/handlers/flow.go` - GraphFlow/ListFlowEndpoints (WorkspaceAuth), FlowMaterialize (write-tier)
- `internal/server/handlers/links.go` - Backlinks/ResolveLink (WorkspaceAuth), `{doc_id}` path param
- `internal/server/handlers/graph_neighborhood.go` - GraphNeighborhood (write-tier)
- `internal/server/handlers/graph_pagerank.go` - GraphPageRankCompute (write-tier)
- `internal/server/handlers/embed.go` - TriggerEmbed (write-tier)
- `internal/server/handlers/reindex.go` - TriggerReindex, TriggerUpdate (write-tier)
- `internal/server/handlers/reindex_cfg.go` - ReindexCFG (write-tier)
- `internal/server/handlers/summarize.go` - TriggerSummarize (write-tier)
- `internal/server/handlers/code_summarize.go` - TriggerCodeSummarize (write-tier)
- `internal/server/handlers/code_summarize_status.go` - GetCodeSummarizeStatus (write-tier)
- `internal/server/handlers/code_summarize_failures.go` - GetCodeSummarizeFailures (write-tier)
- `internal/server/handlers/code_summarize_retry.go` - RetryCodeSummarize, RetryAllCodeSummarize (write-tier)
- `internal/server/handlers/stats.go` - StatsHandler.Handle (data-tier, method receiver)
- `docs/openapi.json` - interim regenerated spec (29 paths after this plan; final complete regen is Plan 04's job)

## Decisions Made

- Fixed `json.RawMessage` swag resolution failure inline via `swaggertype:"object"` (Rule 3 — blocking issue, prevented `go build`/`go test` of `internal/openapigen` from passing).
- Regenerated `docs/openapi.json` after each task despite it not being in this plan's `files_modified` — necessary because the repo's pre-commit hook runs the whole-repo test suite (`go test -race -short ./...`), which includes `internal/openapigen.TestOpenAPISpec_NoDrift`. Without regenerating, every commit in this plan would be blocked. This is additive/interim; Plan 04 performs the authoritative final regen after Plan 02 also lands.
- Fixed pre-existing `gofmt` misalignment (struct field whitespace) in every file touched by this plan's edits, since the plan's own Task 1 acceptance criteria requires `gofmt -l` clean on touched files.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added `swaggertype:"object"` to flowchartResponse.CFG**
- **Found during:** Task 1 (verification step — `go test -race -short ./...` at the repo level, run before committing)
- **Issue:** `swag`'s AST parser failed with `ParseComment error ... flowchartResponse: [cfg]: cannot find type definition: json.RawMessage` — the plan's Task 1 didn't anticipate this field type would block generation.
- **Fix:** Added `swaggertype:"object"` struct tag to the `CFG json.RawMessage` field in `flowchart.go`, telling swag to treat it as an opaque JSON object in the generated schema.
- **Files modified:** `internal/server/handlers/flowchart.go`
- **Verification:** `go test -race -short -count=1 ./internal/openapigen/...` passes; `go build ./...` passes.
- **Committed in:** `892744d` (Task 1 commit)

**2. [Rule 3 - Blocking] Regenerated docs/openapi.json in each task's commit**
- **Found during:** Task 1 (first commit attempt blocked by pre-commit hook: `harness-check.sh in-progress` runs `go build ./... && go test -race -short ./...` across the whole repo)
- **Issue:** `internal/openapigen.TestOpenAPISpec_NoDrift` compares the committed `docs/openapi.json` against a fresh regeneration from current annotations; adding annotations without regenerating makes the test fail, blocking every commit.
- **Fix:** Ran `make generate-openapi` before each task's commit to keep the committed spec in sync with the annotations added so far.
- **Files modified:** `docs/openapi.json` (not in this plan's declared `files_modified`, but required to keep the repo-wide test suite green)
- **Verification:** `go test -race -short -count=1 ./...` green after each regen.
- **Committed in:** `892744d`, `47eaeef`, `d57c4d9` (all three task commits)

**3. [Rule 1 - Bug] gofmt cleanup of pre-existing struct-field misalignment**
- **Found during:** Task 1, 2, 3 verification (`gofmt -l` flagged files before any of my edits ran cleanly)
- **Issue:** Several handler files had pre-existing struct-tag column misalignment (e.g. `QueryRequest`, `VSearchRequest`, `BM25SearchRequest`, `impactResponse`, `backlinksResponse`, `neighborhoodNode`, `reindexCFGRequest`) that predated this plan but was in files this plan touches, and the plan's own Task 1 acceptance criteria requires `gofmt -l` clean on touched files.
- **Fix:** Ran `gofmt -w` on each affected file.
- **Files modified:** `query.go`, `search.go`, `bm25.go`, `impact.go`, `links.go`, `graph_neighborhood.go`, `reindex_cfg.go`
- **Verification:** `gofmt -l` returns empty for all touched files.
- **Committed in:** `892744d`, `47eaeef`, `d57c4d9` (part of each task's commit)

---

**Total deviations:** 3 auto-fixed (2 Rule 3 blocking, 1 Rule 1 bug)
**Impact on plan:** All fixes were necessary to keep the build/test suite green and unblock commits under the repo's pre-commit hook; none changed handler behavior (annotations and struct tags are comment/tag-only). No scope creep — the `docs/openapi.json` regeneration is interim and will be superseded by Plan 04's final, complete regen.

## Issues Encountered

- The pre-commit hook (`scripts/harness-check.sh in-progress`) runs `go build ./... && go test -race -short ./...` across the entire repository, not scoped to this plan's `files_modified`. Since `internal/openapigen.TestOpenAPISpec_NoDrift` asserts the committed spec matches a fresh regeneration, any annotation change anywhere in the repo makes that test fail until `docs/openapi.json` is regenerated. Resolved by regenerating after each task (see Deviation #2 above). This is a structural characteristic of the Wave 2 parallel-plan design (Plans 02 and 03 both annotate handlers without owning `docs/openapi.json`), not a bug introduced by this plan — Plan 04's frontmatter explicitly designates it as the only plan that finalizes `docs/openapi.json`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All 21 graph/search-group handler files are fully annotated with correct `@Router` paths and per-route `@Security` tiers, matching `routes.go`'s actual group membership.
- Plan 02 (parallel, core handler group) is expected to have made equivalent progress on its disjoint file set.
- Plan 04 (Wave 3) can proceed once Plan 02 also completes: it will do the final `make generate-openapi` regen covering all ~60 routes, add the `GET /api/openapi.json` serving route, the drift/reconciliation test, and documentation updates. The interim `docs/openapi.json` committed by this plan (29 paths: Plan 01's 3 health/status/version routes + this plan's 26 route registrations across the 8 handler-annotation points — some paths carry both GET/POST) is safe for Plan 04 to overwrite; no manual reconciliation needed since regeneration is a pure function of the current annotation state.

## Self-Check: PASSED

All files and commits verified present on disk / in git log; `go build ./...` green; `go test -race -short -count=1 ./...` green (all packages including `internal/openapigen` and `internal/server/...`); `gofmt -l` clean on all 21 files this plan touched; `grep -rn handlers.ErrorResponse internal/server/handlers/*.go` returns no matches.

---
*Phase: 12-add-openapi-3-0-spec-for-the-rest-api-issue-530-cover-all-60*
*Completed: 2026-07-02*
