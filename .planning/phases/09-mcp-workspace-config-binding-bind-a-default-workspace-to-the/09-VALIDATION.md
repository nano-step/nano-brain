---
phase: 09
slug: mcp-workspace-config-binding-bind-a-default-workspace-to-the
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-01
---

# Phase 09 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `httptest`, project convention (`-race -short` for unit; integration-tagged tests skip in `-short` via `testing.Short()` guard) |
| **Config file** | none — Go's built-in test runner |
| **Quick run command** | `go test -race -short ./internal/mcp/... ./internal/server/...` |
| **Full suite command** | `go test -race -tags=integration ./internal/mcp/... ./internal/server/...` (against `nanobrain_test`, never the dev DB) |
| **Estimated runtime** | ~30s quick, ~90s full (integration tests hit Postgres) |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -short ./internal/mcp/... ./internal/server/...`
- **After every plan wave:** Run `go test -race -tags=integration ./internal/mcp/... ./internal/server/...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | D-01/D-02 | — | Raw query value injected into context only when non-empty | unit (compile-check) | `go build ./internal/mcp/...` | ❌ W0 | ⬜ pending |
| 09-01-02 | 01 | 1 | D-03/D-04/D-05 | T-09-01/T-09-02/T-09-03 | Explicit arg wins; context fallback used only when arg empty; no-arg+no-default still errors; write path shares the same fallback | unit | `go test -race -short ./internal/mcp/... -run 'TestRequireWorkspace'` | ❌ W0 | ⬜ pending |
| 09-01-03 | 01 | 1 | D-07 | T-09-01 | Middleware wired before `echo.WrapHandler`, not as `echo.MiddlewareFunc` | unit (compile+grep-check) | `go build ./... && grep -c 'WrapStreamableHandler(streamableHandler)' internal/server/routes.go` | ❌ W0 | ⬜ pending |
| 09-02-01 | 02 | 2 | D-06 | — | 13 tools.go sites drop `"workspace"` from required, property retained | unit (compile+grep-check) | `go build ./internal/mcp/... && ! grep -nE '\}, \[\]string\{[^}]*"workspace"' internal/mcp/tools.go` | ❌ W0 | ⬜ pending |
| 09-02-02 | 02 | 2 | D-06 | — | `memory_flowchart` (separate file) drops `"workspace"` from required | unit (compile+grep-check) | `go build ./internal/mcp/... && ! grep -nE '\}, \[\]string\{[^}]*"workspace"' internal/mcp/flowchart.go` | ❌ W0 | ⬜ pending |
| 09-02-03 | 02 | 2 | D-06 | T-09-05 | 14 edited tools have `workspace` optional; 4 excluded tools (`memory_status`, `memory_workspaces_resolve`, `memory_workspaces_list`, `memory_ticket`) unaffected | unit | `go test -race -short ./internal/mcp/... -run 'TestToolSchema_WorkspaceNotRequired'` | ❌ W0 | ⬜ pending |
| 09-03-01 | 03 | 2 | D-01/D-07 | T-09-01/T-09-02 | Full HTTP round-trip: real Echo instance, `?workspace=` on the URL reaches the tool handler (catches Pitfall 1 — the single test that would catch an `echo.MiddlewareFunc`-vs-plain-`http.Handler` wiring mistake) | integration | `go test -race -tags=integration ./internal/mcp/... -run 'TestStreamableHTTP_ConnectionDefaultWorkspace'` | ❌ W0 | ⬜ pending |
| 09-03-02 | 03 | 2 | D-05 | T-09-01 | Write-path (`requireRegisteredWorkspace`) honors the connection default via MCP transport | integration | `go test -race -tags=integration ./internal/mcp/... -run 'TestRequireRegisteredWorkspace_UsesConnectionDefault'` | ❌ W0 | ⬜ pending |
| 09-03-03 | 03 | 2 | — | — | `.mcp.json` config examples in all 3 docs show the `?workspace=` pattern | docs (grep-check) | `test "$(grep -rl 'mcp?workspace=' docs/SETUP_AGENT.md docs/reference-readme.md README.md \| wc -l \| tr -d ' ')" = "3"` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/mcp/tools_internal_test.go` — new unit tests for `requireWorkspace`/`requireRegisteredWorkspace` context-fallback precedence (D-03, D-04, D-05 read-path)
- [ ] Extend `internal/mcp/tools_security_test.go` — write-path connection-default variant (D-05)
- [ ] `internal/mcp/tools_schema_test.go` — schema-assertion test for the 14 edited + 4 excluded tools (D-06)
- [ ] New integration test file (`internal/mcp/streamable_http_integration_test.go`) exercising the real HTTP path against an `httptest.NewServer`-hosted Echo instance (D-01/D-07, catches Pitfall 1)

All four are created within this phase's own plans (09-01 Task 2, 09-02 Task 3, 09-03 Tasks 1-2) — no separate Wave 0 plan needed; the phase is small enough that Wave 0 test-scaffolding and the feature itself land in the same waves.

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify (every task across all 3 plans has an `<automated>` block)
- [x] Wave 0 covers all MISSING references (see Wave 0 Requirements above, all satisfied within-phase)
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-07-01
