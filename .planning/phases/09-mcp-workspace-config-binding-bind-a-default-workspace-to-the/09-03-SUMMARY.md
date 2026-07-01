---
phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the
plan: 03
subsystem: api
tags: [mcp, http-middleware, integration-test, echo, docs]

# Dependency graph
requires: ["09-01", "09-02"]
provides:
  - "TestStreamableHTTP_ConnectionDefaultWorkspace — full-HTTP integration test proving ?workspace= reaches the tool handler through the real echo.WrapHandler(WrapStreamableHandler(...)) wiring (Pitfall 1 coverage)"
  - "TestRequireRegisteredWorkspace_UsesConnectionDefault — write-path D-05 test proving the connection default resolves through requireRegisteredWorkspace's registration check"
  - "?workspace= config examples in docs/SETUP_AGENT.md, docs/reference-readme.md, README.md"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Full-HTTP integration tests for MCP mirror events_integration_test.go's echo.New() + httptest.NewServer(e) shape, using mcpsdk.StreamableClientTransport{Endpoint: ts.URL+...} instead of NewInMemoryTransports to exercise the real transport boundary"
    - "Internal-package (package mcp) Postgres test harness duplicated from tools_security_test.go's package-mcp_test harness, when a test needs both an unexported context-key type and real DB access — the two test-package boundaries in the same directory cannot share unexported symbols"

key-files:
  created:
    - internal/mcp/streamable_http_integration_test.go
  modified:
    - internal/mcp/tools_internal_test.go
    - docs/SETUP_AGENT.md
    - docs/reference-readme.md
    - README.md

key-decisions:
  - "TestRequireRegisteredWorkspace_UsesConnectionDefault placed in tools_internal_test.go (package mcp, internal) rather than tools_security_test.go (package mcp_test), per the plan's own guidance — it needs the unexported ctxKeyDefaultWorkspace type, which package mcp_test cannot reach"
  - "Since tools_security_test.go's Postgres harness (setupMCPSecTestPG) lives in package mcp_test and cannot be imported into package mcp, added a small duplicate (setupInternalTestPG/mcpInternalTestDSN) in tools_internal_test.go — a sanctioned, plan-anticipated duplication rather than a shared non-test helper file"
  - "Used memory_tags as the read-tool probe in the full-HTTP integration test: it only requires workspace (no other args), and its handler doesn't hit the registration-check path, keeping the test focused purely on whether requireWorkspace's context fallback reached through the real HTTP layer"
  - "Logged a pre-existing, unrelated test failure (TestMemoryTrace_RelativeInputAndOutput, graph_paths_integration_test.go) to deferred-items.md instead of fixing it — confirmed out of scope via git log (last touched in PR #423, predates this phase's branch, no overlap with any of Phase 9's 3 plans)"

patterns-established: []

requirements-completed: []

coverage:
  - id: D1-http-integration
    description: "Full HTTP round-trip proves ?workspace= on the connection URL reaches the tool handler through the real echo.WrapHandler(WrapStreamableHandler(...)) wiring, and the same call over a bare /mcp URL still requires the workspace arg (D-01/D-04/D-07, Pitfall 1)"
    verification:
      - kind: integration
        ref: "internal/mcp/streamable_http_integration_test.go#TestStreamableHTTP_ConnectionDefaultWorkspace"
        status: pass
    human_judgment: false
  - id: D2-write-path-default
    description: "Connection default applies uniformly to the write path (requireRegisteredWorkspace) and still enforces the registration check; no default + no arg still errors (D-04/D-05 write-path)"
    verification:
      - kind: integration
        ref: "internal/mcp/tools_internal_test.go#TestRequireRegisteredWorkspace_UsesConnectionDefault"
        status: pass
    human_judgment: false
  - id: D3-docs
    description: "docs/SETUP_AGENT.md, docs/reference-readme.md, and README.md all document the ?workspace= URL query param, precedence (D-03), and name-or-hash/not-\"all\" constraint (D-02), without removing the plain no-query examples"
    verification:
      - kind: other
        ref: "grep -rl 'mcp?workspace=' docs/SETUP_AGENT.md docs/reference-readme.md README.md | wc -l == 3"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-07-01
status: complete
---

# Phase 9 Plan 3: MCP workspace config binding — HTTP integration proof + docs Summary

**Added the full-HTTP integration test that is the only test capable of catching an `echo.MiddlewareFunc`-vs-plain-`http.Handler` wiring regression in the `?workspace=` connection-default feature, extended write-path test coverage for `requireRegisteredWorkspace`, and documented the `?workspace=` config pattern across all three user-facing docs.**

## Performance

- **Duration:** ~8 min (commit-to-commit, 260b9ea → 0a3e00a)
- **Started:** 2026-07-01T19:19:00Z
- **Completed:** 2026-07-01T19:24:21Z
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments
- `TestStreamableHTTP_ConnectionDefaultWorkspace` (new file, `//go:build integration`): stands up a real `echo.New()` instance with `/mcp` wired exactly as `routes.go` does (`echo.WrapHandler(internalmcp.WrapStreamableHandler(streamableHandler))`), serves it via `httptest.NewServer`, connects with the SDK's real `StreamableClientTransport` (not the in-memory transport), and proves a tool call with the `workspace` arg omitted succeeds when `?workspace=<name>` is on the connection URL — plus a negative control proving the same call over a bare `/mcp` URL still returns "workspace is required" (D-01/D-04/D-07)
- `TestRequireRegisteredWorkspace_UsesConnectionDefault` (added to `tools_internal_test.go`, package `mcp`): proves the write-path function (`requireRegisteredWorkspace`, backing `memory_write`/`memory_update`) resolves a context-injected connection default through to a real registration check (`GetWorkspaceByHash`) against the isolated `nanobrain_test` schema, and that no-default+no-arg still errors (D-04/D-05 write-path)
- Added a minimal internal-package Postgres test harness (`setupInternalTestPG`/`mcpInternalTestDSN`) to `tools_internal_test.go` since the existing harness in `tools_security_test.go` lives in the external `mcp_test` package and cannot be shared with `mcp`'s unexported `ctxKeyDefaultWorkspace`
- Added the `?workspace=<name-or-hash>` config example to all three docs (`docs/SETUP_AGENT.md` Step 9, `README.md`, `docs/reference-readme.md`), each explaining that an explicit `workspace` argument still overrides the connection default (D-03) and that the value must be a name or full hash, never `"all"` (D-02); the plain no-query examples remain untouched

## Task Commits

Each task was committed atomically:

1. **Task 1: Full-HTTP integration test for the connection-default workspace (Pitfall 1)** - `16c43ae` (test)
2. **Task 2: Write-path connection-default test (D-05) in tools_internal_test.go** - `b18b095` (test)
3. **Task 3: Add the ?workspace= config example to the three docs** - `0a3e00a` (docs)

**Plan metadata:** (this commit, following SUMMARY.md write)

## Files Created/Modified
- `internal/mcp/streamable_http_integration_test.go` (new) - `TestStreamableHTTP_ConnectionDefaultWorkspace`, the Pitfall-1 full-HTTP-path test
- `internal/mcp/tools_internal_test.go` - added `setupInternalTestPG`/`mcpInternalTestDSN` Postgres harness and `TestRequireRegisteredWorkspace_UsesConnectionDefault`
- `docs/SETUP_AGENT.md` - new "Binding a default workspace (optional)" subsection under Step 9
- `README.md` - added `?workspace=` note under the MCP client config block
- `docs/reference-readme.md` - added the same `?workspace=` note under MCP Configuration, kept in sync with README.md

## Decisions Made
- Placed the D-05 write-path test in `tools_internal_test.go` (package `mcp`) rather than `tools_security_test.go` (package `mcp_test`) — the plan explicitly anticipated this reconciliation because the test needs both the unexported `ctxKeyDefaultWorkspace` type and a real Postgres registration check, and the two test-package boundaries in the same directory cannot share unexported symbols
- Duplicated a small Postgres test-harness (`setupInternalTestPG`) into the internal test file rather than refactoring `tools_security_test.go`'s harness into a shared non-test file — kept the diff smaller and matched the plan's own suggested reconciliation path
- Chose `memory_tags` as the read-tool probe for the HTTP integration test: it requires only `workspace` and has no registration-check side effect, isolating the test to purely the context-fallback-through-HTTP question
- Logged the pre-existing `TestMemoryTrace_RelativeInputAndOutput` failure (unrelated `graph_paths_integration_test.go`, last touched in PR #423 before this phase existed) to `deferred-items.md` instead of fixing it — out of scope per the executor's scope-boundary rule

## Deviations from Plan

### Auto-fixed Issues

None — no bugs, missing functionality, or blocking issues required deviation. The plan's own `<behavior>`/`<action>` blocks for Task 2 explicitly anticipated and pre-authorized the internal-package placement choice made here, so this is not treated as a deviation.

---

**Total deviations:** 0
**Impact on plan:** None — plan executed as written, including its own built-in discretion for Task 2's placement.

## Issues Encountered

- `go test -race -tags=integration ./internal/mcp/... ./internal/server/...` (the broader smoke command, not a plan-mandated verification gate) surfaced a pre-existing panic in `TestMemoryTrace_RelativeInputAndOutput` (`graph_paths_integration_test.go:218`, nil-vs-empty-slice interface conversion). Confirmed via `git log`/`git diff --stat` across all three of Phase 9's plan commit ranges that this file was never touched by this phase and the failure predates the `feat/mcp-workspace-config-binding` branch. Logged to `.planning/phases/09-.../deferred-items.md`; not fixed (out of scope). All of this plan's own specific verification commands pass cleanly in isolation.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 9 is now fully implemented end-to-end: Plan 01 (middleware + context-fallback), Plan 02 (schema required-fields drop), Plan 03 (HTTP-path proof + write-path proof + docs)
- The single highest-risk regression (an `echo.MiddlewareFunc`-vs-plain-`http.Handler` wiring mistake) now has a dedicated, real-HTTP integration test that would fail immediately if reintroduced
- `go build ./...`, `go build -tags=integration ./...`, and `go test -race -short ./internal/mcp/... ./internal/server/...` are all green
- Deferred: `TestMemoryTrace_RelativeInputAndOutput` pre-existing failure (unrelated to this phase) needs a separate bugfix — see `deferred-items.md`

---
*Phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the*
*Completed: 2026-07-01*

## Self-Check: PASSED

All created/modified files exist on disk (streamable_http_integration_test.go, tools_internal_test.go, docs/SETUP_AGENT.md, docs/reference-readme.md, README.md, deferred-items.md, this SUMMARY.md) and all three task commits (16c43ae, b18b095, 0a3e00a) are present in git history. Independently re-ran the full verification suite (build, integration build, both new integration tests, docs grep gate) — all pass.
