---
phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the
plan: 01
subsystem: api
tags: [mcp, go-sdk, http-middleware, context, workspace-resolution]

# Dependency graph
requires: []
provides:
  - "ctxKeyDefaultWorkspace context key + WrapStreamableHandler middleware (internal/mcp/streamable.go)"
  - "requireWorkspace/requireRegisteredWorkspace context-fallback precedence (arg > connection-default > error)"
  - "routes.go wiring of the middleware in front of all three /mcp verb registrations"
affects: [09-02, 09-03]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Plain http.Handler middleware wrapping a vendored SDK handler BEFORE echo.WrapHandler, so req.Context() (not echo.Context) carries the injected value"
    - "Lazy resolution: middleware stores the raw query string; resolution to a hash happens inside requireWorkspace via the existing storage.ResolveWorkspaceParam, avoiding an extra DB round-trip per request"

key-files:
  created: []
  modified:
    - internal/mcp/streamable.go
    - internal/mcp/tools.go
    - internal/mcp/tools_internal_test.go
    - internal/server/routes.go

key-decisions:
  - "Context key type kept unexported (ctxKeyDefaultWorkspace struct{}); only WrapStreamableHandler is exported, avoiding an import cycle between package mcp and package server (RESEARCH.md Open Question 1)"
  - "requireRegisteredWorkspace's own input==\"\" early-check was removed entirely; it now delegates fully to requireWorkspace so the context-fallback is not shadowed for write tools (RESEARCH.md Pitfall 2)"
  - "workspace_not_registered error message now interpolates the resolved ws value instead of the raw input arg — verified safe because tools_security_test.go only substring-matches the literal error code"

patterns-established:
  - "MCP-transport-specific middleware lives in internal/mcp (colocated with the handler factory and the context key it reads), not in internal/server/middleware, per Go convention that a package should read its own context keys"

requirements-completed: []

coverage:
  - id: D1
    description: "WrapStreamableHandler middleware injects raw ?workspace= query value into r.Context(), no-op when absent/empty"
    verification:
      - kind: unit
        ref: "go build ./internal/mcp/... (compiles); manual code inspection — behavior covered indirectly via D2/D3 unit tests exercising the context key it writes"
        status: pass
    human_judgment: false
  - id: D2
    description: "requireWorkspace precedence: explicit arg wins, context-default is fallback, no-arg+no-default returns unchanged 'workspace is required' error (D-03/D-04)"
    verification:
      - kind: unit
        ref: "internal/mcp/tools_internal_test.go#TestRequireWorkspace_ExplicitArgWins"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_internal_test.go#TestRequireWorkspace_ContextFallback"
        status: pass
      - kind: unit
        ref: "internal/mcp/tools_internal_test.go#TestRequireWorkspace_NoArgNoDefaultErrors"
        status: pass
    human_judgment: false
  - id: D3
    description: "requireRegisteredWorkspace (write path) delegates its empty-check to requireWorkspace instead of shadowing the context-fallback with its own early check (D-05)"
    verification:
      - kind: unit
        ref: "grep -n 'workspace is required' internal/mcp/tools.go — confirms the error originates only from requireWorkspace"
        status: pass
    human_judgment: false
  - id: D4
    description: "routes.go wires WrapStreamableHandler(streamableHandler) before echo.WrapHandler on all three /mcp verbs (GET, POST, DELETE), avoiding the Echo-context pitfall (Pitfall 1)"
    verification:
      - kind: unit
        ref: "go build ./... ; grep -n wrappedStreamable internal/server/routes.go (3 route registrations use the shared local)"
        status: pass
    human_judgment: false

duration: 4min
completed: 2026-07-01
status: complete
---

# Phase 9 Plan 1: MCP workspace config binding — core mechanism Summary

**Per-connection default workspace via `?workspace=` query param: HTTP middleware injects it into `r.Context()`, and `requireWorkspace`/`requireRegisteredWorkspace` fall back to it when a tool call omits the `workspace` arg — explicit args still always win.**

## Performance

- **Duration:** ~4 min (commit-to-commit, d84c582 → 6300eb3)
- **Started:** 2026-07-01T19:01:39+07:00
- **Completed:** 2026-07-01T19:04:53+07:00
- **Tasks:** 3
- **Files modified:** 4

## Accomplishments
- Added the unexported `ctxKeyDefaultWorkspace` context-key type and exported `WrapStreamableHandler` middleware in `internal/mcp/streamable.go`, which reads `r.URL.Query().Get("workspace")` and injects the raw value into `r.Context()` (no-op when absent/empty)
- Gave `requireWorkspace` a context-fallback branch (tried only when the explicit arg is empty) and restructured `requireRegisteredWorkspace` to delegate its empty-check entirely to `requireWorkspace`, so the fallback applies uniformly to both read and write tools without a shadow check
- Wired the middleware into `internal/server/routes.go`, wrapping `streamableHandler` once into a shared `wrappedStreamable` local and passing it to all three `/mcp` `echo.WrapHandler(...)` registrations (GET/POST/DELETE), placed correctly *before* `echo.WrapHandler` per the SDK's `req.Context()` extension point (avoiding the Echo-context pitfall that would have made the feature silently inert)
- Added three unit tests (`TestRequireWorkspace_ExplicitArgWins`, `TestRequireWorkspace_ContextFallback`, `TestRequireWorkspace_NoArgNoDefaultErrors`) proving the arg > context-default > error precedence, all `-short`-safe (no live Postgres required — they exercise the pre-resolution `"all"` special-case branch)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add context key + WrapStreamableHandler middleware in streamable.go** - `d84c582` (feat)
2. **Task 2: Add context-fallback to requireWorkspace and requireRegisteredWorkspace** - `899d8e0` (feat)
3. **Task 3: Wire WrapStreamableHandler into routes.go before echo.WrapHandler** - `6300eb3` (feat)

**Plan metadata:** (this commit, following SUMMARY.md write)

## Files Created/Modified
- `internal/mcp/streamable.go` - Added `ctxKeyDefaultWorkspace` context-key type and `WrapStreamableHandler` middleware
- `internal/mcp/tools.go` - Added context-fallback to `requireWorkspace`; restructured `requireRegisteredWorkspace` to delegate its empty-check
- `internal/mcp/tools_internal_test.go` - Added three unit tests for the arg/context-default/error precedence
- `internal/server/routes.go` - Wired `WrapStreamableHandler` in front of the three `/mcp` route registrations

## Decisions Made
- Kept the context key type unexported and exported only the wrapper function, per RESEARCH.md's Open Question 1 recommendation, to avoid an import cycle between `internal/mcp` and `internal/server`
- Stored the raw query string in context rather than eagerly resolving to a hash, keeping resolution lazy inside `requireWorkspace` (avoids a DB round-trip on every request, including ones that don't need a workspace, e.g. `memory_status`)
- Test values for the three new unit tests use `"all"` as the workspace input specifically because it resolves without a DB round-trip, keeping the tests `-short`-safe as the plan required, while still distinguishing which input source (`arg` vs `ctx`) was selected

## Deviations from Plan

None - plan executed exactly as written. The three tasks' `<action>` blocks were followed precisely (exact function restructuring, exact wiring pattern, exact test names).

## Issues Encountered

During test authoring, an initial draft of `TestRequireWorkspace_NoArgNoDefaultErrors` used speculative helper types that don't exist in the codebase; this was caught and corrected in the same edit pass before running tests, switching to the established `errRes.Content[0].(*mcpsdk.TextContent).Text` pattern already used in `tools_security_test.go`. No test failures occurred as a result — this was caught during authoring, not during test execution.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- The core mechanism (middleware + context-fallback) is in place and verified with unit tests; Plan 02 (schema visibility, D-06) and Plan 03 (broader security/integration tests) now have something to exercise
- No blockers. `go build ./...` and `go test -race -short ./...` are green across the whole repo, not just `internal/mcp`/`internal/server`
- `go.mod` diff is empty for this plan — no new dependencies introduced, matching the plan's verification requirement

---
*Phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the*
*Completed: 2026-07-01*

## Self-Check: PASSED

All created/modified files exist on disk (streamable.go, tools.go, tools_internal_test.go, routes.go, SUMMARY.md) and all three task commits (d84c582, 899d8e0, 6300eb3) are present in git history.
