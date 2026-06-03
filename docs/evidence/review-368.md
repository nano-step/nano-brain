# Review Gate — Issue #368 / PR #369

Review Verdict: PASS
Reviewer: oracle (independent, not implementing agent)
Date: 2026-06-03
Commit reviewed: e15997c

## Per-criterion verdicts

| Criterion | Verdict | Evidence |
| --- | --- | --- |
| AC1 — memory_graph relative input | PASS | `resolveNodeAgainstWorkspace` called at tools.go:1240 after `argString` and before SQL queries; error returned via `errResult` at line 1242; `TestMemoryGraph_RelativeNodeInputResolvesToAbsolute` passes |
| AC2 — memory_trace + memory_impact relative input | PASS | Same `resolveNodeAgainstWorkspace` pattern at tools.go:1342 (trace) and tools.go:1436 (impact); both `TestMemoryTrace_RelativeInputAndOutput` and `TestMemoryImpact_RelativeInputAndOutput` pass |
| AC3 — paths=relative strips workspace prefix | PASS | `wsRoot` computed via `lookupWorkspaceRoot` only when `pathStyle == "relative"` (tools.go:1296, 1393, 1481); `stripWorkspacePrefix(wsRoot, ...)` applied to all output nodes/edges; `TestMemoryGraph_RelativeOutputStripsPrefix` passes |
| AC4 — paths=relative does NOT strip imports | PASS | `stripWorkspacePrefix` (graph_paths.go:60) requires `strings.HasPrefix(node, prefix)` where `prefix = wsRoot + "/"` — tokens like "context" pass through unchanged; test asserts `contextSeen == true` at line 177-179 |
| AC5 — Default paths returns absolute | PASS | When `paths` omitted, `argString` returns ""; `pathStyle != "relative"` so `wsRoot = ""`; `stripWorkspacePrefix("", node)` returns node unchanged (graph_paths.go:53-54); test lines 131-145 assert `/tmp/test-ws-` prefix preserved |
| AC6 — Invalid workspace hash errors clearly | PASS | `resolveNodeAgainstWorkspace` returns `fmt.Errorf("workspace lookup failed: %w", err)` at graph_paths.go:43; handler converts to `errResult(err.Error())` setting `IsError: true`; `TestMemoryGraph_InvalidWorkspaceHashErrorsClearly` passes |
| AC7 — All bugs covered by integration tests | PASS | 6 integration tests in `graph_paths_integration_test.go` covering B1-B4 and backward compat: RelativeNodeInput, AbsoluteNodeInputUnchanged, RelativeOutputStripsPrefix, InvalidWorkspaceHash, TraceRelativeIO, ImpactRelativeIO; plus 3 unit test functions in `graph_paths_test.go` |

## Backward compatibility audit

- **Absolute input unchanged**: `resolveNodeAgainstWorkspace` returns absolute paths unchanged (`filepath.IsAbs` check at graph_paths.go:35-37). Verified by `TestMemoryGraph_AbsoluteNodeInputUnchanged`.
- **Default output unchanged**: When `paths` param omitted, `pathStyle` = "" → `wsRoot` = "" → `stripWorkspacePrefix("", node)` is a no-op (graph_paths.go:53-54). Response is byte-identical to pre-change for existing callers.
- **Non-path tokens preserved**: Import tokens like "context" (no file extension) bypass resolution entirely (graph_paths.go:38-40). Verified by unit test and integration test `contextSeen` assertion.
- **No changes to SQL queries or sqlc generated code**: `git diff master..HEAD -- internal/storage/sqlc/` produces empty output.

## Full validation

| Step | Result |
| --- | --- |
| `go build ./...` | PASS — clean, exit 0 |
| `go test -race -short ./...` | PASS — all packages ok |
| `go test -race -count=1 -tags=integration ./internal/mcp/` | PASS — 3.823s, no failures |
| `gofmt -l graph_paths.go graph_paths_test.go graph_paths_integration_test.go` | PASS — no output (all new files are gofmt-clean) |
| `gofmt -l tools.go` | NOTE — pre-existing alignment diffs (present on master before this PR); not introduced by this change |
| `_ = err` check in changed code | PASS — no instances found |
| No edits to `internal/storage/sqlc/*` | PASS — confirmed via git diff |
| Error wrapping convention (`fmt.Errorf("...: %w", err)`) | PASS — graph_paths.go:43 follows convention |

## Findings

1. **Minor (pre-existing, not a blocker)**: `internal/mcp/tools.go` has gofmt alignment diffs in schema map declarations (e.g. extra spaces before `{"type":...}` values). This predates PR #369 — `git show master:internal/mcp/tools.go | gofmt -l` also reports the file. Not introduced by this change, not blocking merge.

## Recommendation

Ready to merge. All 7 acceptance criteria verified by code inspection and test execution. Backward compatibility confirmed — existing callers passing absolute paths receive byte-identical responses. New functionality (relative input resolution, relative output stripping) is opt-in and well-tested.
