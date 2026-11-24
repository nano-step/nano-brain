# Brief — #714 workspace isolation

Issue: https://github.com/nano-step/nano-brain/issues/714
Branch: `fix/714-workspace-isolation` (worktree `.opencode/worktrees/fix-714-workspace-isolation`)
Change type: bug-fix (critical). Lane: high-risk (data-model + public-api-contract adjacent).

## Root causes (verified)

1. `initWorkspace` registers `memory`/`sessions` collections pointing at shared `~/.nano-brain/{memory,sessions}` and `InitWorkspace` watches all three.
2. Watcher state is keyed by absolute path only (`collections`, `dirty`, `warmed`, `startupAdmitted`, `fileCache`), so two workspaces sharing a path overwrite each other.
3. `DELETE /api/v1/workspaces/:hash` never calls the watcher; deleted workspaces can be resurrected because `ensure_workspace_short_id` INSERTs into `workspaces`/`workspace_ids` on any graph upsert.
4. `RemoveWorkspace` runs warn-only cleanup inside a tx; a failed statement aborts the tx and `DeleteWorkspace` then fails.

## Design (locked)

- Treat `memory`/`sessions` as logical DB-backed collections: never watch, never walk, never trigger fsnotify scans for them. Reserve the names for user collections.
- Key watcher state by `(workspaceHash, dirPath)`; keep fsnotify dir watches path-keyed with safe pruning.
- Add `Watcher.UnwatchWorkspace(hash)` and `Watcher.UnwatchCollection(workspaceHash, dirPath)`; wire `RemoveWorkspace` (and `RemoveCollection`) to them.
- Migration `00033`: `ensure_workspace_short_id` raises instead of auto-inserting.

## Locked design decisions

1. **Logical collections.** `memory` and `sessions` are DB-backed logical collections, not filesystem roots. Add `watcher.IsLogicalCollection(name string) bool` (new small file `internal/watcher/collections.go`, <50 lines) returning true for `"memory"`/`"sessions"`. They must never be watched, walked, or fsnotify-scanned. Reserve both names: `AddCollection` rejects them (400), `RenameCollectionHandler` rejects them as `new_name` AND rejects renaming *from* a logical name (otherwise its shared `~/.nano-brain/*` path becomes a disk-backed collection again).
2. **Watcher state keyed by (workspace, path).** Introduce `type collectionKey struct{ workspaceHash, dirPath string }` (unexported, watcher.go or collections.go) plus `func (col watchedCollection) key() collectionKey`. Re-key: `collections`, `dirty`, `warmed`, `startupAdmitted` to `map[collectionKey]…`. Keep `watchedDirs` path-keyed (physical fsnotify watches are shared). Re-key `fileCache` to `map[string]map[string]fileState` — outer key = file path, inner key = workspaceHash — so `delete(w.fileCache, event.Name)` on Remove/Rename still clears every workspace at once, while per-workspace cache entries stay isolated.
3. **fsnotify ownership.** `WatchWithFilter` stores under the composite key; when `w.fsw != nil`, skip `fsw.Add` if `watchedDirs[absPath]` is already set (another collection/workspace already watches that inode), but ALWAYS set `dirty[key]` + hot-register so the new collection still scans. `Run` dedupes root `fsw.Add` the same way.
4. **Unwatch API.** Replace path-only `Unwatch(dirPath)` with `UnwatchCollection(workspaceHash, dirPath string) error` and add `UnwatchWorkspace(workspaceHash string) error`. Both delete the composite-key state (collections/dirty/warmed/startupAdmitted), drop that workspace's `fileCache` entries (all of them for `UnwatchWorkspace`; only paths under the collection root for `UnwatchCollection`), clear `contextualReextract[hash]`/`rateLimiters[hash]`, then prune `watchedDirs`/fsnotify watches that no remaining collection still needs (keep a watched dir if it equals, is under, or is an ancestor of any remaining `col.dirPath`). `RemoveCollection` calls `UnwatchCollection(col.WorkspaceHash, col.Path)`; `RemoveWorkspace` calls `UnwatchWorkspace(hash)` after the DB delete commits.
5. **`ensure_workspace_short_id` stops resurrecting.** New migration `migrations/00033_*.sql`: `CREATE OR REPLACE FUNCTION ensure_workspace_short_id` that SELECTs `short_id` and `RAISE EXCEPTION` when the workspace is not registered (no INSERT into `workspaces`/`workspace_ids`). `Down` restores the 00030 auto-insert body verbatim. `resolve_workspace_short_id` unchanged.
6. **RemoveWorkspace tx bug.** Inside the tx branch, the warn-only `DeleteCodeSummarization{Usage,Failures}ByWorkspace` calls must rollback + 500 on error (a failed statement aborts the tx; continuing guarantees `DeleteWorkspace` fails). Apply the same fix in `reset_workspace.go`'s tx branch. Non-tx (`db == nil`) paths may keep warn-only.
7. **Init/startup skip logical collections.** `InitWorkspace` only attaches the watcher for `code` (drop memory/sessions from the watch list — the DB rows in `initWorkspace` stay for compatibility). `cmd/nano-brain/main.go` startup loop `continue`s on `watcher.IsLogicalCollection(col.Name)`. `triggerIncremental` in reindex.go `continue`s on logical collections before `walkCollectionFiles`.

## In scope (files)

- `internal/watcher/watcher.go` — all re-keying + unwatch/prune + cleanupDeletedDocument loop fix (it currently `return`s after the first matching collection and on `ErrNoRows`; iterate ALL matching collections, `continue` on `ErrNoRows`/errors).
- `internal/watcher/collections.go` — NEW, `IsLogicalCollection` + `collectionKey` (+ `key()` helper).
- `internal/watcher/watcher_test.go` — update every `w.collections[…]`/`w.dirty[…]`/`w.fileCache[…]`/`w.startupAdmitted[…]` access to the new keys; extend `mockQuerier` if needed for per-workspace behavior.
- `internal/watcher/watcher_integration_test.go` and any other test touching `fileCache`/`collections` internals.
- NEW `internal/watcher/workspace_isolation_714_integration_test.go` (`//go:build integration`).
- `internal/server/handlers/workspace.go`, `collection.go`, `reindex.go`, `workspace_remove.go`, `reset_workspace.go`, `routes.go` (+ their `_test.go`).
- `cmd/nano-brain/main.go`.
- `migrations/00033_*.sql`.
- Fix any integration/unit test that relied on `ensure_workspace_short_id` auto-creating a workspace (register the workspace first, or assert the new error where that is the point).

## Out of scope

- No sqlc query changes (no `sqlc generate` needed). Do NOT edit `internal/storage/sqlc/`.
- No changes to `resolve_workspace_short_id`, graph schema, or collection row deletion in `RemoveWorkspace` (orphan `collections` rows are inert once unwatched; deleting them is a separate concern).
- `aliasIndexes` stays keyed by `dirPath` (same path ⇒ same import aliases; sharing is correct).
- Renaming/removing logical collections' DB rows beyond the Add/Rename guards above.
- PR #628's broader logical-collection refactor — implement only what #714 needs.

## Acceptance criteria

- Two workspaces CAN register collections on the same path; watcher keeps both, events dirty both, `fileCache`/`warmed`/`startupAdmitted` do not collide, `UnwatchWorkspace(A)` leaves B fully watched.
- `memory`/`sessions` never reach `WatchWithFilter` from init/startup/reindex; users cannot Add/Rename a collection to those names.
- `DELETE /api/v1/workspaces/:hash` removes watcher state; a subsequent scan/`processAll`/stale graph upsert cannot recreate the `workspaces` row (`ensure_workspace_short_id` raises).
- A failing summarization-cleanup statement inside the remove/reset tx rolls back cleanly (500, no aborted-tx cascade).
- Counterexample test must FAIL on the pre-fix code and PASS after (state that in apply.md).

## Commands

```bash
GOFLAGS=-buildvcs=false go build ./...
GOFLAGS=-buildvcs=false go test -race -short ./...
GOFLAGS=-buildvcs=false go test -race -tags=integration ./internal/watcher ./internal/server/handlers ./internal/storage
# full integration if DB is up: go test -race -tags=integration ./...
```

Integration tests need Postgres on `localhost:5432` (`nanobrain_test` via `testutil.SetupTestDB`). If Docker/Postgres is down, still deliver unit-test evidence and mark integration as blocked-with-reason in apply.md.

## Privacy

No real workspace names, paths, or hashes in committed files, tests, or evidence. Use `t.TempDir()`, synthetic hashes (`"ws-a"`, `"ws-b"`), generic collection names.

## Deliverable

Implement everything above, run the commands, then write `docs/evidence/714-workspace-isolation/apply.md` with: what changed per file, test output (real, pasted), the counterexample result, and any deviations/blockers.
