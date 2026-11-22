# Correctness issues #622, #625, #632, #608, #599

## Decision

Ship independent fixes, each from `origin/master` in a dedicated worktree and
PR. Do not use the live `nanobrain_dev` database for reproduction. The shared
integration database is `nanobrain_test`; test schemas must be isolated.

## Evidence and order

1. **#632, test isolation (first):** `testutil.testSchema(t.Name())` is
   deterministic and omits package/process identity. Two packages or concurrent
   runs with the same test name use the same schema, race over migrations and
   drop each other's fixtures. There is also a second root cause: with
   `search_path=test_schema,public` and no local `goose_db_version`, goose
   reads the public table at version 30 and silently runs *zero migrations*
   in the test schema. All unqualified test SQL then accesses public tables.
   Give every invocation a unique schema and pre-create its local goose version
   table before migration. Exercise same-name tests and private-row visibility;
   audit remaining global-count assertions. A full integration run then exposed
   tests named for migration 11 that migrated through v30 and used columns
   removed in v30. Their fixture must stop at v11 and insert using its actual
   schema, not current sqlc-generated upserts.
   Independent review: two reviewers verified the changed helpers and focused
   tests. One raised a scope concern: several pre-existing, separate test
   helpers still derive deterministic schema names and drop schemas at setup
   (`cmd/nano-brain/pi_init_test.go`,
   `internal/storage/storage_normalization_integration_test.go`,
   `internal/summarize/persist_test.go`). The adjudicator checked #632's
   observed failing tests and OpenSpec contract: both require isolation of
   `SetupTestDB` invocations, which this PR provides. Verdict MERGE for #691;
   residual cross-process collision in the other helpers needs separate
   tracking. Passing integration tests do not prove simultaneous full-suite
   processes cannot collide in those legacy helpers.
2. **#625, impact contract:** REST `collectImpact` and MCP `memory_impact`
   initialize their result slices as nil; empty results serialize to `null`.
   Distinguish unknown nodes from existing leaf nodes with a workspace-scoped
   existence query against graph endpoints and indexed symbols. Keep REST and
   MCP contracts consistent; test empty, missing, and filtered-out cases.
3. **#622, cold workspace:** the reported server was migration v29; current
   branch is v30. `reindex` scans disk to *predict* work, but persists through
   the asynchronous watcher. Inspect watcher attachment/scan and reproduce
   registration -> reindex -> persisted document in an isolated test server
   before changing response semantics or ingest logic.
   Measured on v30: `TestFreshWorkspaceReindexPersistsDocuments` runs real
   `InitWorkspace`, `Watcher.Run` and `TriggerReindex` against a private schema;
   a fresh source file and at least one chunk were persisted. An additional
   HTTP server test exercised real routing/middleware and `GET /api/v1/stats`
   with three files (`.js`, `.ts`, `.vue`): 3 documents and at least 3 chunks
   appeared within 8 seconds. Those tests did not isolate reindex from the
   initial registration scan, as independent reviewers noted. A new
   `TestTriggerRescanWakesIdleWatcher` waits for an initial marker document
   and an unregistered dirty sentinel that can only be drained after the
   initial scan returns; its snapshot cannot contain the not-yet-created
   file even if the sentinel scan has not returned. It then removes
   fsnotify registration, creates a new file and proves that only the
   explicit rescan wakes the watcher and persists it. Before the fix,
   `TestTriggerRescanByName` failed because marking dirty emitted no wake-up;
   `TriggerRescanByName` now uses the existing buffered registration signal.
   The original v29 environment is still unavailable for exact reproduction.
   Independently confirmed: with the watcher stopped, `reindex` reports
   `embedded:2` and `chunks_enqueued:2` although neither document nor chunk
   exists (red `TestIncrementalReindex_*`). Change incremental responses to
   report `files_pending_ingest` and zero actual enqueue/embed counts.
   Async SSE progress now reports `queued` rather than `completed` before
   watcher ingestion. Keep the historical v29 issue open until its exact
   failure is confirmed fixed with a full-server repro.
   The reported request used `force:true`, but the API field is `force_wipe`;
   empty `root` is valid and means all collections. `/update` only re-extracts
   from already indexed files. `stats.collections` groups persisted documents
   and is empty when there are none; it is not the registered collection list.
4. **#599 and #608, summarization:** `GetUnsummarizedSymbols` has `LIMIT`
   without ordering or failure eligibility; `splitAndSend` records failures,
   while `processAllWorkspaces` visits workspaces in a fixed order, each of
   which can consume a whole cycle. Introduce bounded cross-cycle retry with
   backoff and deterministic fair selection; rotate the workspace start and
   address under-yield only after a fixture reproduces it. Preserve explicit
   retry/reset controls.

## Gates

Each fix: OpenSpec proposal/design/spec/tasks before code, regression test,
`go build ./...`, `go test -race -short ./...`, focused and full integration
against `nanobrain_test`, independent review when available, and PR/CI.
Never claim a fix without observed before/after behavior.

## Summarization review follow-up (#599/#608)

Independent reviews sent PR #693 back for retry-all truncation at the
cycle limit, starvation of graph-stale work, same-hash symbols sharing
failure state, incorrect success counts after partial budget exhaustion,
and a persisted empty graph-context marker that could hide a later edge.

Measured red/green regressions cover more retries than one cycle, fresh
and stale queues competing, two files with the same symbol hash retaining
independent failure rows, source-scoped summary existence, and a calls edge
appearing after an edge-free interval. Migration 00031 changes only the
partial unique index on unresolved failures, from workspace plus hash to
workspace, hash, file and symbol name. Explicit retries select failure IDs;
successful summaries resolve the same identity. The stale selector now
checks for relevant calls edges instead of storing a permanent empty hash.
The legacy hash-keyed migration down is intentionally allowed to fail when
distinct same-hash failures exist rather than deleting either record.
An isolated upgrade test starts from v30 with a real failure, migrates
to v31 and confirms that its ID survives while a second same-hash
failure in another file can be inserted. A provider-backed test retries
one of two same-hash failure IDs and confirms that the other remains
unresolved and selectable.

The provider-429 worker backoff gap predates this change and remains open
for separate investigation. Validation and final independent review of
the migration-bearing revision are required before merging #693.
