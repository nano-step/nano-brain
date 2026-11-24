# Independent Review — issue #714 workspace isolation

Review Verdict: PASS
Reviewer: independent-review-subagents (watcher-core + handlers/migration scopes)

Two independent read-only reviewer subagents reviewed the full diff (watcher
core + handlers/migration/tests). Verdicts: **no blockers**. Findings and
disposition below; all actionable items were fixed and re-verified.

## Reviewer A (watcher core)

| # | Severity | Finding | Disposition |
|---|----------|---------|-------------|
| 1 | MAJOR | `processFile` framework re-detect could write tombstone `watchedCollection{dirPath:""}` if `Unwatch*` ran mid-scan → dirty-on-every-event + `pruneWatchedDirsLocked` no-op | **FIXED** — write guarded by `if updated, ok := w.collections[key]; ok` |
| 2 | MINOR | `UnwatchCollection` dropped workspace-scoped `contextualReextract`/`rateLimiters` belonging to sibling collections | **FIXED** — only cleared when no collections remain for the workspace |
| 3 | MINOR | fileCache cleanup over-deletes for nested same-workspace collections | Accepted — perf nit only; DB content-hash prevents re-indexing |
| 4 | MINOR | Typed-nil `*watcher.Watcher` → non-nil `WorkspaceUnwatcher` interface could panic | **FIXED** — nil-receiver guards on `UnwatchCollection`/`UnwatchWorkspace` |
| 5 | MINOR | `triggerForceWipe` didn't skip logical collections | **FIXED** — same `IsLogicalCollection` skip as incremental |
| 6 | MINOR | In-flight scan can upsert orphan `documents` rows for a just-deleted workspace | Accepted — bounded, pre-existing race shape; graph layer fails closed via migration 00033; orphan docs are inert (hash-scoped) |
| 7–9 | NIT | `aliasIndexes` not pruned (pre-existing, brief keeps path-keyed); `/` root prefix edge (pre-existing pattern); mock `sourcePathHash` ignores workspace (integration test covers) | Accepted / out of scope |

Reviewer A explicitly verified: all `collections`/`dirty`/`warmed`/
`startupAdmitted`/`fileCache` access sites re-keyed; `pruneWatchedDirsLocked`
three-way keep logic correct; no nil-map panics; no cross-workspace leakage;
no lock-order inversion; `handleFSEvent`/`cleanupDeletedDocument` correct;
`WatchWithFilter` bookkeeping consistent; `TriggerRescanByName` safe.

## Reviewer B (handlers / migration / tests)

| # | Severity | Finding | Disposition |
|---|----------|---------|-------------|
| 1 | MAJOR | tx-rollback tests only asserted HTTP 500 + row survival — identical observable outcome on pre-fix warn-and-continue code (aborted tx fails later anyway) | **FIXED** — tests now assert `cleanup code summarization` in `he.Message`, which only the fixed early-rollback path produces |
| 2 | MINOR | Typed-nil interface hazard (same as A#4) | **FIXED** (nil-receiver guards) |
| 3 | MINOR | `RemoveCollection` on a logical collection could unwatch an unrelated same-workspace collection sharing the path | **FIXED** — unwatch skipped for logical names |
| 4 | NIT | `triggerForceWipe` asymmetry (same as A#5) | **FIXED** |
| 5 | NIT | `UnwatchCollection` workspace-state clearing (same as A#2) | **FIXED** |
| 6 | NIT | Swagger comment implied memory/sessions are watched | **FIXED** — comment clarified + openapi.json regenerated |
| 7 | NIT | `reindex` with `root` selecting only logical collections returns 202 with zero counters | Accepted — harmless |

Reviewer B verified: init compat (3 DB rows still created, only `code`
watched); both name guards fire before DB writes; tx branches roll back +
500; `UnwatchWorkspace` runs only after successful delete in both branches;
`main.go` startup skip placed correctly; migration 00033 `Down` is verbatim
00030; no legitimate production path breaks under fail-closed `RAISE`;
`ALTER TABLE … RENAME COLUMN` test trick sound (`search_path` caveat handled);
`TestTwoWorkspacesSamePathStaySeparate` is version-agnostic and fails pre-fix;
privacy clean.

## Post-fix verification

- `go build -buildvcs=false ./...` clean
- `go test -race -short ./...` all ok
- `go test -race -tags=integration -run 'TestTwoWorkspacesSamePathStaySame|TestWorkspaceIsolation714|TestRemoveWorkspaceTxCleanupFailureRollsBack|TestResetWorkspaceTxCleanupFailureRollsBack|TestCollection'` ok
- `TestOpenAPISpec_NoDrift` ok after `make generate-openapi`
