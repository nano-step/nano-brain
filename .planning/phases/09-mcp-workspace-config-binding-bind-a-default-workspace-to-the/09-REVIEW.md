---
phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the
reviewed: 2026-07-01T19:30:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - README.md
  - docs/SETUP_AGENT.md
  - docs/reference-readme.md
  - internal/mcp/flowchart.go
  - internal/mcp/streamable.go
  - internal/mcp/streamable_http_integration_test.go
  - internal/mcp/tools.go
  - internal/mcp/tools_internal_test.go
  - internal/mcp/tools_schema_test.go
  - internal/server/routes.go
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
status: clean
---

# Phase 9: Code Review Report

**Reviewed:** 2026-07-01T19:30:00Z
**Depth:** standard
**Files Reviewed:** 10
**Status:** clean

## Summary

Reviewed the full diff across all 3 plans of Phase 9 (MCP workspace config binding): the `WrapStreamableHandler` middleware and `ctxKeyDefaultWorkspace` context-fallback mechanism (`internal/mcp/streamable.go`, `internal/mcp/tools.go`), the `routes.go` wiring change, the 14-tool schema required-fields edit (`internal/mcp/tools.go`, `internal/mcp/flowchart.go`), the new schema-assertion test (`internal/mcp/tools_schema_test.go`), the full-HTTP integration test and write-path connection-default test (`internal/mcp/streamable_http_integration_test.go`, `internal/mcp/tools_internal_test.go`), and the `?workspace=` documentation added to three docs.

Assessment: clean implementation, no correctness or security defects found. The RESEARCH.md's own flagged pitfalls (echo.MiddlewareFunc-vs-plain-http.Handler wiring; requireRegisteredWorkspace's duplicate empty-check shadowing the context fallback) were both correctly avoided in the implementation and are now covered by dedicated tests. Traced `requireRegisteredWorkspace`'s two call sites (`memory_write`, `memory_update`) — signature and calling convention unchanged. Verified the `workspace_not_registered` error message's interpolation change (from raw `input` to resolved `ws`) does not break either existing test that substring-matches only the literal error code, not the interpolated value. `go build ./...`, `go vet ./...`, `go build -tags=integration ./...`, and `go test -race -short ./...` all pass cleanly with no output.

## Info

### IN-01: Postgres test harness duplicated between tools_internal_test.go and tools_security_test.go

**File:** `internal/mcp/tools_internal_test.go:1-72`
**Issue:** `setupInternalTestPG`/`mcpInternalTestDSN` duplicate `setupMCPSecTestPG`/`mcpSecTestDSN` from `tools_security_test.go` almost verbatim (schema-per-test Postgres bootstrap, goose migration, cleanup). This is a real, acknowledged duplication — but it exists because `tools_internal_test.go` is package `mcp` (needs the unexported `ctxKeyDefaultWorkspace` type) while `tools_security_test.go` is package `mcp_test` (external), and Go does not allow sharing unexported test helpers across the internal/external test package split within the same directory without introducing a new non-test shared file.
**Fix:** No action needed for this phase — the duplication was explicitly anticipated and pre-authorized by the 09-03-PLAN.md's own `<action>` text for Task 2 ("the executor may add it to tools_internal_test.go... Reconcile by adding a small internal-package test"). If this pattern needs a third occurrence in a future phase, consider extracting a small non-test helper package (e.g. `internal/mcp/mcptest`) that both `mcp` and `mcp_test` can import.

---

_Reviewed: 2026-07-01T19:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
