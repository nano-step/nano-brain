---
phase: 09-mcp-workspace-config-binding-bind-a-default-workspace-to-the
verified: 2026-07-01T12:36:46Z
status: passed
score: 10/10 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 9: MCP workspace config binding — bind a default workspace to the MCP connection Verification Report

**Phase Goal:** Let a `.mcp.json` MCP server entry bind a single default workspace via a `?workspace=<name-or-hash>` URL query param, so tool calls from that connection can omit the `workspace` argument and still resolve correctly; explicit `workspace` args continue to win, and no-arg + no-default still errors exactly as today.
**Verified:** 2026-07-01T12:36:46Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A tool call that omits the workspace arg but arrives on a connection whose URL had `?workspace=<name-or-hash>` resolves to that workspace (D-01/D-02) | ✓ VERIFIED | `internal/mcp/streamable.go:47-54` (`WrapStreamableHandler` injects query param into `r.Context()`); behavioral proof via `TestStreamableHTTP_ConnectionDefaultWorkspace/query_param_default_lets_tool_call_omit_workspace_arg` — PASS |
| 2 | An explicit workspace arg always overrides the connection default (D-03) | ✓ VERIFIED | `internal/mcp/tools.go:155-163` (`requireWorkspace` checks explicit arg first); behavioral proof via `TestRequireWorkspace_ExplicitArgWins` — PASS |
| 3 | When neither an explicit arg nor a connection default is present, the exact "workspace is required" error is returned, unchanged from today (D-04) | ✓ VERIFIED | Behavioral proof via `TestRequireWorkspace_NoArgNoDefaultErrors` (read path) — PASS; `TestStreamableHTTP_ConnectionDefaultWorkspace/no_query_param_and_no_arg_still_requires_workspace` (real HTTP path) — PASS; `TestRequireRegisteredWorkspace_UsesConnectionDefault/no_default_and_no_arg_still_requires_workspace` (write path) — PASS |
| 4 | The fallback applies uniformly to both read tools (`requireWorkspace`) and write tools (`requireRegisteredWorkspace`) (D-05) | ✓ VERIFIED | `internal/mcp/tools.go:187-204` (`requireRegisteredWorkspace` delegates its empty-check to `requireWorkspace`, no duplicate early check); behavioral proof via `TestRequireRegisteredWorkspace_UsesConnectionDefault/context_default_resolves_to_registered_workspace` — PASS |
| 5 | An LLM agent is no longer forced to supply the `workspace` argument for the 14 tools that previously required it (D-06) | ✓ VERIFIED | Negative grep on `internal/mcp/tools.go` and `internal/mcp/flowchart.go` for `}, []string{...."workspace"...}` — no matches; `TestToolSchema_WorkspaceNotRequired` — PASS |
| 6 | The `workspace` parameter still EXISTS in every tool's schema (still overridable per-call) — only its required-ness is dropped (D-06) | ✓ VERIFIED | `TestToolSchema_WorkspaceNotRequired` asserts `workspace` remains in `properties` for all 14 edited tools — PASS |
| 7 | The 4 tools that never took a `workspace` param (`memory_status`, `memory_workspaces_resolve`, `memory_workspaces_list`, `memory_ticket`) are untouched (RESEARCH Pitfall 3) | ✓ VERIFIED | `TestToolSchema_WorkspaceNotRequired` asserts their required-fields contracts are unchanged — PASS |
| 8 | A real HTTP request to `/mcp?workspace=<ws>` with the `workspace` arg OMITTED resolves through the actual `echo.WrapHandler`-wrapped handler and succeeds (D-01/D-07, catches Pitfall 1) | ✓ VERIFIED | `internal/mcp/streamable_http_integration_test.go` reproduces the exact `routes.go` wiring (`echo.WrapHandler(WrapStreamableHandler(streamableHandler))`); `TestStreamableHTTP_ConnectionDefaultWorkspace` — PASS |
| 9 | A write tool (`memory_write`/`memory_update` path) with a connection default configured and no workspace arg succeeds against a registered workspace (D-05 write-path) | ✓ VERIFIED | `TestRequireRegisteredWorkspace_UsesConnectionDefault/context_default_resolves_to_registered_workspace` calls `requireRegisteredWorkspace` directly with a ctx default and asserts the registration check (`GetWorkspaceByHash`) passes — PASS |
| 10 | The `.mcp.json` config docs show the new `?workspace=` example (D-01) | ✓ VERIFIED | `grep -l 'mcp?workspace=' docs/SETUP_AGENT.md docs/reference-readme.md README.md` — 3/3 matched |

**Score:** 10/10 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/mcp/streamable.go` | Unexported context-key type + exported `WrapStreamableHandler` wrapper | ✓ VERIFIED | `ctxKeyDefaultWorkspace` (line 28), `WrapStreamableHandler` (line 47) present and wired |
| `internal/mcp/tools.go` | `requireWorkspace`/`requireRegisteredWorkspace` read the context-key fallback | ✓ VERIFIED | `ctx.Value(ctxKeyDefaultWorkspace{})` read at line 158; `requireRegisteredWorkspace` delegates via `a.requireWorkspace` |
| `internal/server/routes.go` | `streamableHandler` wrapped by the new middleware BEFORE `echo.WrapHandler` | ✓ VERIFIED | Lines 140-143: `wrappedStreamable := mcp.WrapStreamableHandler(streamableHandler)` applied before all 3 `echo.WrapHandler(...)` calls |
| `internal/mcp/tools_internal_test.go` | Precedence unit tests (D-03/D-04/D-05 read path) | ✓ VERIFIED | `TestRequireWorkspace_ExplicitArgWins`, `TestRequireWorkspace_ContextFallback`, `TestRequireWorkspace_NoArgNoDefaultErrors` present and passing |
| `internal/mcp/tools.go` (13 sites) | `toolSchema` required-fields lists no longer contain `"workspace"` | ✓ VERIFIED | Negative grep confirms no `}, []string{...."workspace"...}` matches |
| `internal/mcp/flowchart.go` | `memory_flowchart` required-fields list no longer contains `"workspace"` | ✓ VERIFIED | Negative grep confirms no match |
| `internal/mcp/tools_schema_test.go` | Asserts required-fields for the 14 edited tools + 4 excluded tools | ✓ VERIFIED | File exists, `TestToolSchema_WorkspaceNotRequired` covers both sets |
| `internal/mcp/streamable_http_integration_test.go` | Full HTTP round-trip test using `httptest.NewServer` + real Echo + `?workspace=` query string | ✓ VERIFIED | File exists, uses `echo.New()` + `httptest.NewServer` + `mcpsdk.StreamableClientTransport`, mirrors production wiring exactly |
| Write-path context-fallback test | Extended write-path test (D-05) | ✓ VERIFIED (relocated) | Plan's artifact list named `internal/mcp/tools_security_test.go`; executor placed `TestRequireRegisteredWorkspace_UsesConnectionDefault` in `internal/mcp/tools_internal_test.go` instead, per the plan's own `<action>` text which explicitly pre-authorized this relocation ("the executor may add it to tools_internal_test.go instead of tools_security_test.go if that keeps the unexported-key access clean"). Documented as a deliberate decision in 09-03-SUMMARY.md, not a gap. |
| `docs/SETUP_AGENT.md`, `docs/reference-readme.md`, `README.md` | `?workspace=` config example added | ✓ VERIFIED | All 3 contain `mcp?workspace=` example with D-03 precedence note and D-02 name-or-hash/not-"all" constraint |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `WrapStreamableHandler` (streamable.go) | `mcpsdk.NewStreamableHTTPHandler`'s `ServeHTTP` | `r.WithContext(context.WithValue(...))` before `next.ServeHTTP` | ✓ WIRED | Confirmed by reading streamable.go:47-54; SDK reads `req.Context()` per its own documented extension point (verified in RESEARCH.md against vendored source) |
| `routes.go` | `mcp.WrapStreamableHandler` | Direct call, result passed to `echo.WrapHandler` | ✓ WIRED | `wrappedStreamable := mcp.WrapStreamableHandler(streamableHandler)` then `echo.WrapHandler(wrappedStreamable)` on all 3 verbs |
| `requireRegisteredWorkspace` | `requireWorkspace` | Direct delegation for empty-check + context fallback | ✓ WIRED | `ws, errRes := a.requireWorkspace(ctx, args)` at tools.go:191, no duplicate early check |
| `httptest.NewServer(echo instance)` | Real `/mcp` route with real `WrapStreamableHandler` wrapping | Reproduces `routes.go`'s exact registration | ✓ WIRED | `streamable_http_integration_test.go` registers `/mcp` via `echo.WrapHandler(wrappedStreamable)`, identical to production |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Explicit arg wins over context default (D-03) | `go test ./internal/mcp/... -run '^TestRequireWorkspace_ExplicitArgWins$'` | PASS | ✓ PASS |
| No arg + no default still errors (D-04, read path) | `go test ./internal/mcp/... -run '^TestRequireWorkspace_NoArgNoDefaultErrors$'` | PASS | ✓ PASS |
| Full HTTP round-trip with `?workspace=` (Pitfall 1, D-01/D-07) | `go test -tags=integration ./internal/mcp/... -run '^TestStreamableHTTP_ConnectionDefaultWorkspace$'` | PASS (both subtests) | ✓ PASS |
| Write-path connection default + registration check (D-05) | `go test ./internal/mcp/... -run '^TestRequireRegisteredWorkspace_UsesConnectionDefault$'` | PASS (both subtests) | ✓ PASS |

### Requirements Coverage

No formal REQ IDs mapped to this phase (`requirements: none` — feature phase, per ROADMAP.md and all 3 plan frontmatters). No orphaned requirements found in REQUIREMENTS.md for Phase 9.

### Anti-Patterns Found

None. Scanned all 10 phase-modified files for `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` — zero matches.

### Human Verification Required

None. All 10 truths resolved to ✓ VERIFIED with behavioral test evidence; zero items routed to human verification.

### Gaps Summary

No gaps. All must-haves from all 3 plans (09-01, 09-02, 09-03) are verified against the actual codebase with passing behavioral tests, not just presence/wiring checks. The single deviation from a plan's literal artifact path (write-path test relocated from `tools_security_test.go` to `tools_internal_test.go`) was explicitly pre-authorized by the plan's own text and does not affect goal achievement — the test exists, passes, and proves the D-05 write-path claim.

Full test evidence: `go build ./...`, `go vet ./...`, `go build -tags=integration ./...`, and `go test -race -short ./...` (whole project) all pass with zero failures. Independent code review (`.planning/phases/09-mcp-workspace-config-binding-bind-a-default-workspace-to-the/09-REVIEW.md`) found 0 critical, 0 warning issues.

---

_Verified: 2026-07-01T12:36:46Z_
_Verifier: Claude (gsd-verifier)_
