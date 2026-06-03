# Design — Fix MCP wake_up Collections Filter

## Approach

Single 1-line edit + 1 regression test. No alternative considered because the bug is a literal missed field assignment.

### Current code (`internal/mcp/tools.go:818-821`)

```go
docs, err := a.queries.RecentDocuments(ctx, sqlc.RecentDocumentsParams{
    WorkspaceHash: ws,
    Limit:         int32(limit),
})
```

### Fixed code

```go
docs, err := a.queries.RecentDocuments(ctx, sqlc.RecentDocumentsParams{
    WorkspaceHash: ws,
    Limit:         int32(limit),
    Collections:   []string{"memory", "session-summary"},
})
```

Identical to `internal/server/handlers/wakeup.go:88-92`.

## Why not extract a shared helper?

Tempting: define `var wakeUpCollections = []string{"memory", "session-summary"}` in a shared package (e.g. `internal/storage` or a new `internal/wakeup`) and reference it from both handlers.

Rejected for this change:
- Adds a new package or pollutes an existing one for **2 call sites**.
- The list is small, stable, and self-documenting at the call site.
- The structural fix (a shared service layer for wake-up) is a separate change that would also extract `WorkspaceDocStats` + `WorkspaceChunkCount` + `ListCollectionsWithLastUpdated` aggregation. Doing it piecemeal in this PR risks scope creep.

**Mitigation for repeat regressions**: the new unit test asserts the exact `Collections` slice passed by the MCP handler. Any future caller that drops the filter will turn the test red.

## Test approach

`internal/mcp/tools_test.go` currently uses inline struct mocks (per project convention — no gomock). The existing `TestMemoryWakeUp_RejectsWorkspaceAll` proves the test harness is in place.

New test `TestMemoryWakeUp_PassesMemoryAndSessionSummaryCollections`:

1. Build a mock querier struct with a `recordedParams` field.
2. Mock `RecentDocuments` captures `arg` into `recordedParams` and returns an empty result.
3. Mock the other three queries (`WorkspaceDocStats`, `WorkspaceChunkCount`, `ListCollectionsWithLastUpdated`) with zero-value valid returns.
4. Invoke the MCP `memory_wake_up` tool handler via the existing test scaffolding.
5. Assert `recordedParams.Collections` deep-equals `[]string{"memory", "session-summary"}`.

The test runs in `-short` mode (no DB) and adds < 1 ms to CI.

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| Mock doesn't match the real `Queries` interface signature | low | Existing tests use the same pattern; reuse them as a template |
| Hardcoded list drifts between HTTP and MCP again | low | New unit test fails immediately if MCP drops the filter; consider follow-up to extract shared constant |
| MCP `Adapter` doesn't expose a way to inject a mock querier | unknown | Check `internal/mcp/adapter.go` before writing the test; if not possible, fall back to a higher-level integration-style test using an in-memory PG schema |

The third risk is the only one that could expand scope. If injection isn't possible, the fallback is: add a small interface to `adapter.go` mirroring the WakeUpQuerier pattern from `handlers/wakeup.go:15-20` — itself a tiny refactor that improves testability without changing behaviour.

## Rollback

`git revert <fix-commit>` — restores nil `Collections`, recreates the bug. No DB or schema state to roll back.
