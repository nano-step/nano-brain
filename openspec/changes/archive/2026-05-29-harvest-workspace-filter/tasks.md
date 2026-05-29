## 1. Modify listSessions to accept workspace filter

- [ ] 1.1 Change `listSessions` signature to accept `registeredPaths []string` parameter
- [ ] 1.2 Add `WHERE p.worktree IN (...)` clause using SQLite placeholder expansion when registeredPaths is non-empty
- [ ] 1.3 When registeredPaths is empty, return nil (no sessions) — do not run unfiltered query

## 2. Modify HarvestAll to build workspace cache and filter

- [ ] 2.1 At start of `HarvestAll`, call `sqlc.New(h.pgDB).ListWorkspaces(ctx)` to get registered workspaces
- [ ] 2.2 Build `map[string]string` (path → hash) from workspace list; also extract `[]string` of paths for the filter
- [ ] 2.3 If workspace list is empty, log WARN and return early (0, 0, 0)
- [ ] 2.4 Pass registered paths to `listSessions(ctx, sqdb, registeredPaths)`
- [ ] 2.5 Remove `UpsertWorkspace` call (lines 146-153)
- [ ] 2.6 Replace `storage.WorkspaceHash()` calls with cache lookup from the pre-built map
- [ ] 2.7 Remove the `worktree == ""` fallback branch (lines 134-136) — orphaned sessions excluded by query

## 3. Update unit tests

- [ ] 3.1 Update `listSessions` test to pass registeredPaths and verify filtering
- [ ] 3.2 Add test case: empty registeredPaths → 0 sessions returned
- [ ] 3.3 Add test case: sessions with unregistered worktree are excluded

## 4. Update integration test

- [ ] 4.1 Register a workspace in PG before harvest, verify only matching sessions harvested
- [ ] 4.2 Verify sessions from unregistered worktrees are not persisted

## 5. Validation

- [ ] 5.1 `go build ./...` passes
- [ ] 5.2 `go test -race -short ./...` passes
- [ ] 5.3 `go test -race -tags=integration ./internal/harvest/...` passes (if PG available)
- [ ] 5.4 `lsp_diagnostics` clean on changed files
