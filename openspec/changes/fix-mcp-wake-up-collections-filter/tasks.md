# Tasks — Fix MCP wake_up Collections Filter (#356)

## Pre-implementation
- [x] Confirm bug via DB query + HTTP vs MCP path comparison (workspace `7f44...` showed HTTP returns 10 summaries, MCP returns `[]`).
- [x] Confirm fix #338 / PR #340 only touched `internal/server/handlers/wakeup.go` + `internal/storage/queries/wakeup.sql`, missed `internal/mcp/tools.go`.
- [ ] Read `internal/mcp/adapter.go` to confirm querier injection works for the new test (per design.md risk).

## Implementation
- [ ] Edit `internal/mcp/tools.go:818-821` — add `Collections: []string{"memory", "session-summary"}` to the `RecentDocumentsParams` struct literal in `registerMemoryWakeUp`.

## Tests
- [ ] Add `TestMemoryWakeUp_PassesMemoryAndSessionSummaryCollections` to `internal/mcp/tools_test.go` (mock asserts `RecentDocumentsParams.Collections == ["memory", "session-summary"]`).
- [ ] Verify `TestMemoryWakeUp_RejectsWorkspaceAll` still passes.
- [ ] Verify `go test -race -short ./internal/mcp/...` passes.

## Validation ladder
- [ ] `go build ./... && go test -race -short ./...` exits 0.
- [ ] Manual repro: start daemon, call `memory_wake_up` MCP tool, assert `recent_memories` is non-empty on the nano-brain workspace.

## Docs
- [ ] `CHANGELOG.md` `[Unreleased] ### Fixed` entry referencing #356.
- [ ] No README change (no API/CLI surface change).

## Archive (after merge)
- [ ] `openspec archive fix-mcp-wake-up-collections-filter` — moves to `openspec/changes/archive/2026-06-03-fix-mcp-wake-up-collections-filter/`.
