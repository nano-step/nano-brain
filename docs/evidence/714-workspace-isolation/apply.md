# Apply — #714 workspace isolation

Issue: https://github.com/nano-step/nano-brain/issues/714
Branch: `fix/714-workspace-isolation` (worktree `.opencode/worktrees/fix-714-workspace-isolation`)
Base: `origin/master` @ `e139102`. All changes uncommitted.
Date: 2026-11-24

## What changed (per file)

### `internal/watcher/collections.go` — NEW
- `collectionKey{workspaceHash, dirPath}` + `watchedCollection.key()` — composite identity for all watcher state.
- `IsLogicalCollection(name)` — true for `"memory"`/`"sessions"`; those names are DB-backed logical collections, never filesystem roots.

### `internal/watcher/watcher.go`
- `collections`, `dirty`, `warmed`, `startupAdmitted` re-keyed `map[string]…` → `map[collectionKey]…`; every lookup site updated (WatchWithFilter, TriggerRescanByName, Run, handleFSEvent, processDirty/processAll, scanCollection, warmFileCacheFromDB, recordStartupAdmission, ShouldSkipPath, cleanup*, aliasIndexFor call sites, etc.).
- `fileCache` re-keyed to `map[filePath]map[workspaceHash]fileState` — outer key path, inner key workspace — via `setFileCacheEntry`; per-workspace entries isolated while whole-path clears stay possible.
- `watchedDirs` stays path-keyed (physical fsnotify watches are shared): `WatchWithFilter` skips `fsw.Add` when the path is already watched but ALWAYS sets `dirty[key]` + nudges `hotRegisterCh` so the new (workspace, path) collection still gets its own scan.
- NEW `UnwatchCollection(workspaceHash, dirPath)` — drops the composite-key state, this workspace's `fileCache` entries under the collection root, `contextualReextract[hash]`/`rateLimiters[hash]`, then prunes unneeded fsnotify watches.
- NEW `UnwatchWorkspace(workspaceHash)` — same, for every collection of the workspace (all `fileCache` entries for that workspace).
- NEW `pruneWatchedDirsLocked` — removes `watchedDirs`/`fsw` watches that equal, sit under, or are an ancestor of no remaining `col.dirPath` (ancestor case preserves nested-collection parent watches).
- `cleanupDeletedDocument` — now iterates ALL collections matching the deleted path (previously returned after the first match) and `continue`s on `sql.ErrNoRows`/errors so one workspace missing the doc no longer skips the others.

### `internal/server/handlers/workspace.go`
- `InitWorkspace` watcher attach reduced to the `code` collection only — `memory`/`sessions` DB rows are still created for compatibility but are never watched (previously all three were watched at shared `~/.nano-brain/*` paths).

### `internal/server/handlers/collection.go`
- `AddCollection` rejects `memory`/`sessions` names (400, "reserved for a logical collection").
- `RenameCollectionHandler` rejects a logical `name` ("logical collections cannot be renamed") AND a logical `new_name` ("reserved"), so a logical collection's shared path can't be re-registered as disk-backed and users can't shadow the reserved names.
- `RemoveCollection` now calls `fw.UnwatchCollection(col.WorkspaceHash, col.Path)` instead of path-only `Unwatch`.

### `internal/server/handlers/workspace_remove.go`
- New `WorkspaceUnwatcher` interface (`UnwatchWorkspace(hash) error`); `RemoveWorkspace(q, db, fw, logger)` signature gains `fw`.
- Tx branch: `DeleteCodeSummarizationUsageByWorkspace`/`DeleteCodeSummarizationFailuresByWorkspace` failures now `tx.Rollback()` + 500 (previously warn-and-continue → guaranteed aborted-tx `DeleteWorkspace` failure). Non-tx (`db == nil`) path keeps warn-only per locked design.
- After successful delete (either branch), `fw.UnwatchWorkspace(hash)` detaches watcher state so a deleted workspace can't be resurrected by a stale scan.

### `internal/server/handlers/reset_workspace.go`
- Same tx fix: summarization-cleanup failures inside the tx now rollback + 500 instead of warn-and-continue into an aborted tx. (Diff is mostly re-indentation of the `db == nil`/`else` blocks plus the two error-handling changes.)

### `internal/server/routes.go`
- `DELETE /api/v1/workspaces/:hash` passes `s.watcher` into `RemoveWorkspace`.

### `internal/server/handlers/reindex.go`
- `triggerIncremental` `continue`s on `watcher.IsLogicalCollection(col.Name)` before `walkCollectionFiles` — logical collections are never walked or fsnotify-scanned.

### `cmd/nano-brain/main.go`
- Startup watch loop `continue`s on `watcher.IsLogicalCollection(col.Name)` — persisted `memory`/`sessions` collection rows from older versions are skipped at boot.

### `migrations/00033_ensure_workspace_short_id_no_resurrect.sql` — NEW
- `Up`: `CREATE OR REPLACE FUNCTION ensure_workspace_short_id` — SELECTs `short_id` from `workspace_ids`, `RAISE EXCEPTION 'workspace % is not registered…'` when absent; no INSERT into `workspaces`/`workspace_ids` (no resurrection on graph upsert).
- `Down`: restores the 00030 auto-insert body verbatim.
- `resolve_workspace_short_id` unchanged. No sqlc changes (`internal/storage/sqlc/` untouched).

### Tests
- `internal/watcher/workspace_isolation_714_integration_test.go` — NEW (`//go:build integration`): `TestWorkspaceIsolation714` — two workspaces on the same `t.TempDir()` path keep independent watcher state; `UnwatchWorkspace(A)` leaves B fully watched; deleted-workspace graph upsert raises.
- `internal/watcher/watcher_test.go` — all `collections`/`dirty`/`fileCache`/`startupAdmitted` accesses re-keyed to `collectionKey`; NEW `TestTwoWorkspacesSamePathStaySeparate` (version-agnostic counterexample, see below).
- `internal/watcher/watcher_integration_test.go`, `containment_test.go`, `reindex_622_integration_test.go` — re-keyed to composite keys / new unwatch APIs.
- `internal/server/handlers/workspace_remove_integration_test.go` — NEW (`//go:build integration`): `TestRemoveWorkspaceTxCleanupFailureRollsBack` + `TestResetWorkspaceTxCleanupFailureRollsBack` — force a summarization-cleanup statement to fail inside the tx (column rename in the test schema) and assert 500 + full rollback (workspace row and documents survive).
- `internal/server/handlers/workspace_remove_test.go` — NEW `TestWorkspaceRemove_CallsUnwatchWorkspace` (mock `WorkspaceUnwatcher` proves detach wiring).
- `internal/server/handlers/collection_test.go` — NEW `TestAddCollection_LogicalNameReserved`, `TestRenameCollection_LogicalNamesReserved`.
- Tests that relied on `ensure_workspace_short_id` auto-creating workspaces now register explicitly:
  - `internal/server/handlers/impact_625_integration_test.go`, `impact_bare_calls_integration_test.go` — `UpsertWorkspace` before `UpsertGraphEdge`.
  - `internal/mcp/impact_625_integration_test.go` — `UpsertWorkspace` for the `"other-"+workspace` foreign-workspace fixture.

## Test evidence (real output)

Environment: Postgres `nanobrain-pg` on localhost:5432, db `nanobrain_test`;
`NANO_BRAIN_TEST_DATABASE_URL=postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test?sslmode=disable`.

### Build

```
$ GOFLAGS=-buildvcs=false go build ./...
(no output — exit 0)
```

### Unit / short tests

```
$ GOFLAGS=-buildvcs=false go test -race -short ./...
ok  	github.com/nano-brain/nano-brain/internal/codesummarize	(cached)
ok  	github.com/nano-brain/nano-brain/internal/config	(cached)
ok  	github.com/nano-brain/nano-brain/internal/embed	(cached)
ok  	github.com/nano-brain/nano-brain/internal/evalbench	(cached)
ok  	github.com/nano-brain/nano-brain/internal/eventbus	(cached)
ok  	github.com/nano-brain/nano-brain/internal/flow	(cached)
ok  	github.com/nano-brain/nano-brain/internal/graph	(cached)
ok  	github.com/nano-brain/nano-brain/internal/harvest	(cached)
ok  	github.com/nano-brain/nano-brain/internal/health	(cached)
ok  	github.com/nano-brain/nano-brain/internal/health/doctor	(cached)
ok  	github.com/nano-brain/nano-brain/internal/intelligence	(cached)
ok  	github.com/nano-brain/nano-brain/internal/links	(cached)
ok  	github.com/nano-brain/nano-brain/internal/mcp	(cached)
ok  	github.com/nano-brain/nano-brain/internal/migrate	(cached)
ok  	github.com/nano-brain/nano-brain/internal/openapigen	2.522s
ok  	github.com/nano-brain/nano-brain/internal/redact	(cached)
ok  	github.com/nano-brain/nano-brain/internal/search	(cached)
ok  	github.com/nano-brain/nano-brain/internal/search/hyde	(cached)
ok  	github.com/nano-brain/nano-brain/internal/search/preprocess	(cached)
ok  	github.com/nano-brain/nano-brain/internal/search/reranking	(cached)
ok  	github.com/nano-brain/nano-brain/internal/server	(cached)
ok  	github.com/nano-brain/nano-brain/internal/server/handlers	(cached)
ok  	github.com/nano-brain/nano-brain/internal/server/middleware	(cached)
ok  	github.com/nano-brain/nano-brain/internal/storage	(cached)
ok  	github.com/nano-brain/nano-brain/internal/summarize	(cached)
ok  	github.com/nano-brain/nano-brain/internal/symbol	(cached)
ok  	github.com/nano-brain/nano-brain/internal/telemetry	(cached)
ok  	github.com/nano-brain/nano-brain/internal/timefilter	(cached)
ok  	github.com/nano-brain/nano-brain/internal/tsparse	(cached)
ok  	github.com/nano-brain/nano-brain/internal/watcher	(cached)
```

Forced re-run of the touched packages (`-count=1`):

```
$ go test -race -short -count=1 ./internal/server/handlers ./internal/watcher
ok  	github.com/nano-brain/nano-brain/internal/server/handlers	2.780s
ok  	github.com/nano-brain/nano-brain/internal/watcher	1.806s
```

### Integration (brief's specified packages)

```
$ GOFLAGS=-buildvcs=false NANO_BRAIN_TEST_DATABASE_URL=… go test -race -tags=integration -count=1 ./internal/watcher ./internal/server/handlers ./internal/storage
ok  	github.com/nano-brain/nano-brain/internal/watcher	13.279s
ok  	github.com/nano-brain/nano-brain/internal/server/handlers	10.943s
ok  	github.com/nano-brain/nano-brain/internal/storage	12.227s
```

Migration 00033 applies cleanly — every `SetupTestDB` run logs
`OK   00033_ensure_workspace_short_id_no_resurrect.sql` and
`goose: successfully migrated database to version: 33`.

### Full integration suite (all packages with `//go:build integration` tests)

```
ok  	github.com/nano-brain/nano-brain/internal/bench	1.377s
ok  	github.com/nano-brain/nano-brain/internal/codesummarize	7.541s
ok  	github.com/nano-brain/nano-brain/internal/embed	2.158s
ok  	github.com/nano-brain/nano-brain/internal/flow	3.351s
ok  	github.com/nano-brain/nano-brain/internal/graph	5.546s
ok  	github.com/nano-brain/nano-brain/internal/harvest	17.056s
ok  	github.com/nano-brain/nano-brain/internal/links	1.930s
ok  	github.com/nano-brain/nano-brain/internal/mcp	25.702s
ok  	github.com/nano-brain/nano-brain/internal/search	4.606s
ok  	github.com/nano-brain/nano-brain/internal/server	5.928s
ok  	github.com/nano-brain/nano-brain/internal/server/handlers	10.977s
ok  	github.com/nano-brain/nano-brain/internal/storage	12.227s
ok  	github.com/nano-brain/nano-brain/internal/storage/sqlc	1.947s
ok  	github.com/nano-brain/nano-brain/internal/summarize	7.133s
ok  	github.com/nano-brain/nano-brain/internal/testutil	2.210s
ok  	github.com/nano-brain/nano-brain/internal/watcher	13.279s
```

(`internal/flow`, `internal/watcher`, `internal/storage` results are from the
same `-race -tags=integration` runs; `internal/mcp` and
`internal/server/handlers` re-verified after the workspace-registration test
fixes.)

## Counterexample

The new regression coverage FAILS on the pre-fix code and PASSES after.

### Watcher state collision (unit-level, version-agnostic test)

`TestTwoWorkspacesSamePathStaySeparate` (added to `watcher_test.go`) uses only
APIs present in both revisions (`WatchWithFilter`, `len(w.collections)`), so it
compiles on `e139102`. Verified in a throwaway `git worktree` at `e139102`:

Pre-fix (`e139102`):

```
=== RUN   TestTwoWorkspacesSamePathStaySeparate
    workspace_isolation_714_counterexample_test.go:22: expected 2 watched collections (one per workspace on the same path), got 1 — watcher state is colliding across workspaces
--- FAIL: TestTwoWorkspacesSamePathStaySeparate (0.00s)
FAIL	github.com/nano-brain/nano-brain/internal/watcher	0.548s
```

Post-fix (this branch):

```
=== RUN   TestTwoWorkspacesSamePathStaySeparate
--- PASS: TestTwoWorkspacesSamePathStaySeparate (0.00s)
PASS
ok  	github.com/nano-brain/nano-brain/internal/watcher	1.705s
```

(The deeper `TestWorkspaceIsolation714` integration test exercises
`UnwatchWorkspace`/composite keys that do not exist pre-fix, so it is a
post-fix-only regression test; the unit counterexample above is the
fail-on-old-code proof for the collision itself.)

### `ensure_workspace_short_id` resurrection (live SQL demo)

The shared `public` schema in `nanobrain_test` is still at migration v31, so it
still carries the OLD auto-insert body — used directly as the pre-fix oracle:

```
-- OLD body (public @ v31): auto-creates a workspace row for any hash
SELECT ensure_workspace_short_id('ce-ghost-714-a');
 old_fn_result
---------------
           475
SELECT hash, name, path FROM public.workspaces WHERE hash='ce-ghost-714-a';
      hash      |      name      | path
----------------+----------------+------
 ce-ghost-714-a | ce-ghost-714-a |

-- NEW body (migration 00033, installed in scratch schema ce714_new)
SET search_path TO ce714_new, public;
SELECT ce714_new.ensure_workspace_short_id('ce-ghost-714-b');
ERROR:  workspace ce-ghost-714-b is not registered; register it before writing graph rows
SELECT hash FROM public.workspaces WHERE hash='ce-ghost-714-b';
 hash
------
(0 rows)
```

Demo rows/schema were cleaned up afterwards (`DELETE` + `DROP SCHEMA
ce714_new CASCADE`; verified 0 leftover `ce-ghost-714-%` rows).

## Deviations / notes

- `RenameCollectionHandler`'s diff includes re-indentation of a pre-existing
  mis-indented block (the `new_name` validation + `UpdateDocumentsCollection` +
  `RenameCollection` calls sat one tab too shallow); semantics unchanged, now
  gofmt-clean alongside the new guards.
- The tx-rollback integration tests force the cleanup failure via
  `ALTER TABLE … RENAME COLUMN` inside the per-test schema rather than
  `DROP TABLE`: `SetupTestDB` sets `search_path TO <schema>, public`, and
  `public.code_summarization_*` exists in the shared dev DB, so an unqualified
  `DELETE` would silently fall through to the public copy after a DROP.
- Orphan `collections` rows for a removed workspace are left in place per the
  brief ("deleting them is a separate concern"); they are inert once
  `UnwatchWorkspace` detaches the watcher state.
- No blockers. All changes uncommitted on `fix/714-workspace-isolation`.

## Post-review fixes (2 independent reviewers, no blockers)

- `watcher.go` `processFile`: framework re-detection no longer writes a
  tombstone `watchedCollection{dirPath:""}` when `Unwatch*` lands mid-scan —
  guarded by `if updated, ok := w.collections[key]; ok` (reviewer MAJOR).
- `watcher.go` `UnwatchCollection`: `contextualReextract`/`rateLimiters`
  (workspace-scoped) are only dropped when the workspace's last collection is
  removed; nil-receiver guards added to both `Unwatch*` methods.
- `collection.go` `RemoveCollection`: skips `UnwatchCollection` for logical
  names so a same-path user collection can't be detached by accident.
- `reindex.go` `triggerForceWipe`: skips logical collections for symmetry with
  `triggerIncremental`.
- `workspace_remove_integration_test.go`: both tx-rollback tests now assert the
  fix-specific `cleanup code summarization` error message, so they fail on the
  pre-fix warn-and-continue code (previously only asserted the shared 500).
- `workspace.go` swagger + regenerated `docs/openapi.json` /
  `internal/server/handlers/openapi.json` (TestOpenAPISpec_NoDrift).

Re-verified: `go build -buildvcs=false ./...`, `go test -race -short ./...`
(all ok), targeted integration run (watcher + handlers) ok.
