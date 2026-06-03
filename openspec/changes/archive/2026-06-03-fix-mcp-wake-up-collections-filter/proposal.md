# Fix MCP wake_up Collections Filter

## Issue
[#356 — fix(mcp): memory_wake_up returns empty recent_memories — Collections filter missing in MCP handler](https://github.com/nano-step/nano-brain/issues/356)

## Lane
**normal** — bug fix in MCP hot path. Touches one file (`internal/mcp/tools.go`) plus regression test. No public API change, no migration, no schema change. Requires OpenSpec proposal per harness rules because it changes a tool surface behaviour observable to MCP clients.

## Why
PR #340 (fix #338) added a **required** collection filter to the shared `RecentDocuments` SQL query so that `wake_up` no longer returns irrelevant `code` collection symbols:

```sql
WHERE workspace_hash = $1
  AND collection = ANY($3::text[])   -- ← required after #338
```

The HTTP handler `internal/server/handlers/wakeup.go` was updated to pass `Collections: []string{"memory", "session-summary"}`. The **MCP handler at `internal/mcp/tools.go:818`** was missed — it still passes `Collections: nil`.

When the SQL receives `pq.Array(nil)`, PostgreSQL evaluates `collection = ANY('{}'::text[])` which is always **false**. Every call to the `memory_wake_up` MCP tool therefore returns `recent_memories: []`, even on workspaces with hundreds of memory documents.

Verified on this workspace (hash `7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f`, 71 docs in `memory`, multiple in `session-summary`):

| Path | Result |
|---|---|
| `POST /api/v1/wake-up` | ✅ 10 summaries returned |
| `memory_wake_up` MCP tool | ❌ empty array |

## Desired Outcome
- The `memory_wake_up` MCP tool returns the same `recent_memories` list as the HTTP `POST /api/v1/wake-up` endpoint when called with the same workspace and limit.
- A new regression test asserts the MCP handler invokes `RecentDocuments` with `Collections = ["memory", "session-summary"]`, preventing the next refactor from dropping the filter again.

## Constraints
- Backward compatible — no change to MCP tool input schema or response shape.
- No new dependency.
- No SQL or migration change — the filter was already added in #340.
- Existing tests (`TestMemoryWakeUp_RejectsWorkspaceAll` and friends in `internal/mcp/tools_test.go`) must pass unchanged.
- Fix is restricted to `internal/mcp/tools.go` + `internal/mcp/tools_test.go`. No drive-by refactor.

## Out of Scope
- **Service-layer extraction** to share wake-up logic between HTTP and MCP handlers (would structurally prevent this class of bug, but is a separate cleanup change).
- **Configurable collections list** (issue #356 only restores parity with the HTTP handler; making the list configurable is a separate feature).
- **Integration test against real PG for the MCP path** — HTTP integration tests already cover the SQL query end-to-end; an additional MCP-level integration test adds cost without new signal.

## Acceptance Criteria
1. **Parity**: `memory_wake_up` MCP tool returns the same number of `recent_memories` (with same IDs in same order) as `POST /api/v1/wake-up` for any workspace and limit, given identical state.
2. **Unit test**: New test in `internal/mcp/tools_test.go` invokes the MCP `memory_wake_up` handler with a mock `WakeUpQuerier`-like adapter and asserts the mock receives `RecentDocumentsParams.Collections == ["memory", "session-summary"]`.
3. **No regression**: `TestMemoryWakeUp_RejectsWorkspaceAll` and every other existing test in `internal/mcp/tools_test.go` passes unchanged.
4. **validate:quick passes**: `go build ./... && go test -race -short ./...` exits 0.
5. **smoke:e2e SKIPPED with reason**: MCP tool surface unchanged (input schema + output shape identical); behavioural correctness covered by unit test + manual repro on running daemon.
