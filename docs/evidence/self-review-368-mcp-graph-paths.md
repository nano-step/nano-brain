# Self-Review: Issue #368 / PR (fix/368-mcp-graph-paths)

**Change Type:** bug-fix
**Story:** #368 — graph tools accept relative paths + optional relative output
**Lane:** normal (3 risk flags: public-contracts, existing-behavior, multi-domain)
**Date:** 2026-06-03
**Reviewer (Self):** Sisyphus (implementing — independent review delegated separately per R: no self-review)

## Summary

Three MCP code-intelligence tools (`memory_graph`, `memory_trace`, `memory_impact`) silently returned empty results when agents passed workspace-relative `node` paths (B1-B3), and always returned absolute paths regardless of agent need, wasting ~55 chars × N edges per call (B4). This PR adds:

1. **Input resolution** — relative paths are joined against `workspaces.path` server-side. Absolute paths and import tokens (no slash / no extension) pass through unchanged. Invalid workspace hash returns a clear error instead of silent zero.
2. **Opt-in relative output** — new `paths: "relative"` parameter strips the workspace root prefix from every node/source/target in the response. Imports (e.g. `context`) without the prefix pass through unchanged. Default behavior (`paths` omitted or `"absolute"`) is byte-identical to pre-change.

## Acceptance Criteria

- [x] **AC1** — `memory_graph(node="internal/x.go::F")` returns same edges as `memory_graph(node="/abs/.../internal/x.go::F")` (test: `TestMemoryGraph_RelativeNodeInputResolvesToAbsolute`)
- [x] **AC2** — Same for `memory_trace`, `memory_impact` (tests: `TestMemoryTrace_RelativeInputAndOutput`, `TestMemoryImpact_RelativeInputAndOutput`)
- [x] **AC3** — `paths="relative"` strips workspace root prefix from `edges[].source` and `edges[].target` (test: `TestMemoryGraph_RelativeOutputStripsPrefix`)
- [x] **AC4** — `paths="relative"` does NOT strip imports — `"context"` passes through unchanged (same test, asserts `contextSeen`)
- [x] **AC5** — Default (`paths` omitted) returns absolute paths exactly as today (same test, first half asserts `/tmp/` prefix preserved)
- [x] **AC6** — Invalid workspace hash → 1 error result with clear message, not silent zero (test: `TestMemoryGraph_InvalidWorkspaceHashErrorsClearly`)
- [x] **AC7** — Integration tests cover all 4 bugs + the new opt-in

## Diff scope

Files touched:
1. `internal/mcp/graph_paths.go` (new, 75 lines) — 4 shared helpers (`splitNodeSymbol`, `resolveNodeAgainstWorkspace`, `stripWorkspacePrefix`, `lookupWorkspaceRoot`)
2. `internal/mcp/graph_paths_test.go` (new) — unit tests for split + strip + resolve pass-through rules (no DB needed)
3. `internal/mcp/graph_paths_integration_test.go` (new) — 6 integration tests covering all 3 tools end-to-end
4. `internal/mcp/tools.go` — 3 targeted Edit patches:
   - `registerMemoryGraph` (~25 lines): add `paths` schema field, call `resolveNodeAgainstWorkspace` after input parse, apply `stripWorkspacePrefix` when `paths=relative`
   - `registerMemoryTrace` (~20 lines): same pattern
   - `registerMemoryImpact` (~18 lines): same pattern
5. `.opencode/skills/nano-brain/SKILL.md` — update 3 tool schemas to document the new `paths` param and relative-input support

No `sqlc/*` files edited. No migration. No new dependencies. tools.go grew by ~60 lines net (still well under the 1206-line warning threshold).

## Risk audit per FEATURE_INTAKE.md

| Flag | Applied? | Mitigation |
|---|---|---|
| Auth / authorization / data-model / audit-security / external-systems / search-quality / embedding/vector / cross-platform / weak-proof | No | n/a |
| Public contracts | **Yes** | Additive-only: input gains relative-form support, output gains opt-in flag. Default behavior unchanged. Existing absolute-path callers and existing fixtures pass without modification. |
| Existing behavior | **Yes** | Backward compat verified: `TestMemoryGraph_RelativeOutputStripsPrefix` asserts default still returns absolute paths. |
| Multi-domain | **Yes** | 3 MCP tools + 1 new helper file. Same root-cause fix applied uniformly. |

**Total: 3 risk flags → Normal lane confirmed.**

## Validation

| Step | Result |
|---|---|
| `go build ./...` | clean |
| `go test -race -short ./...` | all packages pass (28+ packages) |
| `go test -race -count=1 -tags=integration ./internal/mcp/` | pass — 6 new tests included |
| `go test -race -count=1 -run TestSplitNodeSymbol\|TestStripWorkspacePrefix\|TestResolveNodeAgainstWorkspace ./internal/mcp/` | pass (unit, no DB) |
| `bash scripts/harness-check.sh pre-work --issue 368` | PASS (6/6) |

## Scope discipline

Every changed line traces directly to one of B1-B4:
- `resolveNodeAgainstWorkspace` calls → B1, B2, B3
- `stripWorkspacePrefix` calls + `paths` schema entries → B4
- New helper file + tests → support for B1-B4
- SKILL.md updates → keep docs in sync with new behavior

No adjacent refactoring. No formatting cleanups. No "improvements" outside scope.
