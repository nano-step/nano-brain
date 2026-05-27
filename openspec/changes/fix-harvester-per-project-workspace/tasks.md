## 1. Update OpenCode SQLite Harvester

- [x] 1.1 Modify `listSessions` SQL query in `opencode_sqlite.go` to `JOIN project ON session.project_id = project.id` and return `p.worktree` alongside each session row
- [x] 1.2 Update `sqSession` struct to include `worktree string` field
- [x] 1.3 Update `NewOpenCodeSQLiteHarvester` signature: remove `workspace string` parameter; update constructor body and `OpenCodeSQLiteHarvester` struct (remove `workspace` field)
- [x] 1.4 Update `NewOpenCodeSQLiteHarvesterFromDB` (test constructor) the same way
- [x] 1.5 In `HarvestAll()`: add local `wsCache map[string]string` (worktree → wsHash); for each session, compute/lookup `WorkspaceHash(session.worktree)`, call `q.UpsertWorkspace` on cache miss, use per-session `wsHash` for all document/chunk writes
- [x] 1.6 Handle orphaned sessions (LEFT JOIN, NULL worktree): fallback to `WorkspaceHash(h.dbPath)` + emit WARN log

## 2. Update main.go

- [x] 2.1 Remove pre-computation of `wsHash` for the SQLite harvester branch (lines ~321–330 in `main.go`)
- [x] 2.2 Update `harvest.NewOpenCodeSQLiteHarvester(...)` call to new 3-arg signature: `(db, logger, dbPath)`

## 3. Fix Tests

- [x] 3.1 Update `NewOpenCodeSQLiteHarvesterFromDB` call sites in test files to remove workspace parameter
- [x] 3.2 Update test SQLite fixtures: add `project` table with at least one row; update `session` rows to reference `project_id`
- [x] 3.3 Add test scenario: two sessions from different projects → stored under different workspace hashes
- [x] 3.4 Add test scenario: orphaned session (no project row) → fallback workspace hash + warning logged

## 4. Validate

- [x] 4.1 `go build ./...` passes with no errors
- [x] 4.2 `go test -race -short ./...` passes (all harvester tests green)
- [x] 4.3 `go test -race -tags=integration ./...` passes
- [ ] 4.4 Smoke: start server, trigger harvest, verify sessions appear under `WorkspaceHash(worktree)` workspace in `GET /api/v1/workspaces`
