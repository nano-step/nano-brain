---
phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the
plan: 03
subsystem: api
tags: [mcp, http, integration-test, security, docs]

# Dependency graph
requires: ["09-01", "09-02"]
provides:
  - "Full-HTTP integration test proving ?workspace= on the connection URL reaches requireWorkspace through the real echo.WrapHandler-wrapped handler (catches Pitfall 1)"
  - "Write-path connection-default test proving requireRegisteredWorkspace honors the fallback (D-05)"
  - "?workspace= config examples in docs/SETUP_AGENT.md, docs/reference-readme.md, README.md"
affects: []

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Real Echo instance + httptest.NewServer + go-sdk streamable HTTP client transport for wiring-level integration tests, mirroring internal/server/handlers/events_integration_test.go"

key-files:
  created:
    - internal/mcp/streamable_http_integration_test.go
  modified:
    - internal/mcp/tools_security_test.go
    - docs/SETUP_AGENT.md
    - docs/reference-readme.md
    - README.md

key-decisions:
  - "Integration test registers /mcp on a real Echo instance using the exact production wiring (echo.WrapHandler(mcp.WrapStreamableHandler(streamableHandler))) rather than the in-memory MCP transport, so it fails if the middleware is ever reverted to an echo.MiddlewareFunc or dropped (RESEARCH Pitfall 1)"
  - "Both the full-HTTP test and the write-path test include a negative control (no query param / no default) asserting the unchanged 'workspace is required' error, proving D-04 holds at both the transport and write-path layers"
  - "Docs updates are additive examples only — the existing plain-workspace .mcp.json examples are left in place, with the ?workspace= binding shown as an alternative for the single-project case"

patterns-established: []

requirements-completed: []

coverage:
  - id: D1-D7-http
    description: "?workspace= URL query param reaches requireWorkspace through the real HTTP/Echo layer; explicit arg still overrides; no-default+no-arg still errors over HTTP"
    verification:
      - kind: integration
        ref: "go test -race -tags=integration ./internal/mcp/... -run TestStreamableHTTP_ConnectionDefaultWorkspace"
        status: pass
    human_judgment: false
  - id: D5-write
    description: "Connection default applies to requireRegisteredWorkspace (write path); no-default+no-arg still errors on write path"
    verification:
      - kind: integration
        ref: "go test -race -tags=integration ./internal/mcp/... -run TestRequireRegisteredWorkspace_UsesConnectionDefault"
        status: pass
    human_judgment: false
  - id: docs-example
    description: "All 3 MCP config docs show the ?workspace= connection-default pattern"
    verification:
      - kind: manual
        ref: "grep -rl 'mcp?workspace=' docs/SETUP_AGENT.md docs/reference-readme.md README.md (3/3 matched)"
        status: pass
    human_judgment: false

duration: ~25min
completed: 2026-07-01
status: complete
---

# Phase 9 Plan 3: MCP workspace config binding — integration tests + docs Summary

**Added the full-HTTP integration test that is the only test in the phase capable of catching a middleware-wiring regression (Pitfall 1), a write-path connection-default test, and updated all 3 MCP config docs with the `?workspace=` example.**

## Performance

- **Duration:** ~25 min (commit-to-commit, 16c43ae → 0a3e00a)
- **Tasks:** 3
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments
- `internal/mcp/streamable_http_integration_test.go` (new, `//go:build integration`): registers a real workspace against the isolated `nanobrain_test` schema, stands up a real Echo instance wired exactly as `routes.go` does, and drives the go-sdk streamable HTTP client against `httptest.NewServer` — proving `?workspace=<name>` on the connection URL lets a tool call omit the `workspace` arg, and that a plain `/mcp` URL (no query param) still returns "workspace is required"
- `internal/mcp/tools_security_test.go`: added `TestRequireRegisteredWorkspace_UsesConnectionDefault`, proving the write-path (`requireRegisteredWorkspace`) honors the same context-default fallback as reads (D-05), with a no-default/no-arg negative control
- `docs/SETUP_AGENT.md`, `docs/reference-readme.md`, `README.md`: each now shows a `.mcp.json` example with `"url": "http://localhost:3100/mcp?workspace=<name>"` alongside the existing plain-URL example

## Task Commits

Each task was committed atomically:

1. **Task 1: Full-HTTP integration test for the connection-default workspace (Pitfall 1)** - `16c43ae` (test)
2. **Task 2: Write-path connection-default test (D-05) in tools_security_test.go** - `b18b095` (test)
3. **Task 3: Add the ?workspace= config example to the three docs** - `0a3e00a` (docs)

**Plan metadata:** (this commit, following SUMMARY.md write)

## Files Created/Modified
- `internal/mcp/streamable_http_integration_test.go` (new) - `TestStreamableHTTP_ConnectionDefaultWorkspace`, the Pitfall-1 regression guard
- `internal/mcp/tools_security_test.go` - `TestRequireRegisteredWorkspace_UsesConnectionDefault` added
- `docs/SETUP_AGENT.md`, `docs/reference-readme.md`, `README.md` - `?workspace=` config example added

## Decisions Made
- Used the SDK's streamable HTTP client transport (not `NewInMemoryTransports`) specifically because the in-memory transport bypasses the real HTTP/Echo layer entirely and provably cannot catch a Pitfall-1-class wiring regression
- Kept both new tests `-short`-skipped since they require a reachable `nanobrain_test` Postgres instance, consistent with the rest of the `tools_security_test.go` suite

## Deviations from Plan

None — plan executed as written.

## Issues Encountered

None. Verified independently against a live `nanobrain_test` instance: `go build ./...`, `go build -tags=integration ./...`, `go test -race -short ./internal/mcp/... ./internal/server/...`, and both new integration tests (`TestStreamableHTTP_ConnectionDefaultWorkspace`, `TestRequireRegisteredWorkspace_UsesConnectionDefault`) all pass.

## User Setup Required

None - no external service configuration required. Users who want the connection-default behavior add `?workspace=<name-or-hash>` to their existing `.mcp.json` URL.

## Next Phase Readiness
- Phase 9 is now functionally complete: all 3 plans (middleware + runtime fallback, schema visibility, integration coverage + docs) are merged
- `go build ./...`, `go build -tags=integration ./...`, and the full `-short` + tagged-integration test suites are green
- Ready for `/gsd-verify-work` or an independent code-review pass before shipping

---
*Phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the*
*Completed: 2026-07-01*

## Self-Check: PASSED

All created/modified files exist on disk (streamable_http_integration_test.go, tools_security_test.go, docs/SETUP_AGENT.md, docs/reference-readme.md, README.md, this SUMMARY.md) and all three task commits (16c43ae, b18b095, 0a3e00a) are present in git history. Independently re-ran the full verification suite (build, integration build, unit tests, both new integration tests) — all pass.
