---
phase: "08"
plan: "01"
subsystem: harvest
tags: [harvest, session, refactor, migration, adapter-pattern]
dependency_graph:
  requires: []
  provides: [SessionSource-interface, NormalizedSession-model, Engine-harvester, sessions-collection-unification, session-summary-migration]
  affects: [internal/harvest, internal/summarize, internal/server/handlers, internal/links, internal/mcp]
tech_stack:
  added: []
  patterns: [adapter-interface, compile-time-assertions, idempotent-sql-migration]
key_files:
  created:
    - internal/harvest/source.go
    - internal/harvest/model.go
    - internal/harvest/opencode_source.go
    - internal/harvest/claude_source.go
    - internal/harvest/engine.go
    - internal/harvest/migration.go
  modified:
    - internal/harvest/claudecode.go
    - internal/summarize/persist.go
    - internal/server/handlers/wakeup.go
    - internal/links/extract.go
    - internal/mcp/tools.go
    - cmd/nano-brain/main.go
decisions:
  - "Engine.writeRawFallback writes to collection 'sessions' (not 'session-summary')"
  - "buildSourcePath now has explicit cases for 'claude' and 'opencode'; unknown sources get 'summary://<source>/<id>' rather than wrong opencode default"
  - "Existing ClaudeCodeHarvester and OpenCodeSQLiteHarvester left in place; Engine is additive (engine flag path deferred to phase 8 follow-up)"
  - "MigrateSessionSummaryToSessions runs at startup before Runner; non-fatal"
  - "links/extract.go guard updated to accept 'sessions' alongside legacy 'session-summary' (Rule 2: link extraction would silently no-op after rename)"
  - "mcp/tools.go memory_wake_up updated to 'sessions' collection for consistency"
metrics:
  duration: "~25 minutes"
  completed: "2026-06-29"
  tasks_completed: 4
  tasks_total: 4
  files_created: 6
  files_modified: 6
status: complete
---

# Phase 8 Plan 01: Pluggable Refactor + Unified Collection Summary

**One-liner:** Introduced `SessionSource` adapter interface + `NormalizedSession` model; refactored OpenCode/Claude into thin adapters behind a generic `Engine`; unified all session docs into the `sessions` collection with an idempotent SQL migration from `session-summary`.

## Tasks Completed

| # | Task | Commit | Files |
|---|------|--------|-------|
| 1 | Define SessionSource interface + normalized model | c22fd87 | source.go, model.go |
| 2 | Implement adapters (OpenCodeSource, ClaudeSource) + Engine | 6de7c9b | opencode_source.go, claude_source.go, engine.go, claudecode.go |
| 3 | Unify collection + migrate session-summary docs | 03fcddb | persist.go, wakeup.go, migration.go, extract.go, tools.go |
| 4 | Wire migration at startup in main.go | e8f1d21 | main.go |

## What Was Built

### Task 1 — Interface + Model
- `internal/harvest/source.go`: `Location` struct + `SessionSource` interface (`Name()`, `Discover()`, `Read()`)
- `internal/harvest/model.go`: `NormalizedMessage`, `NormalizedSession`, `IsActive()` (mirrors existing `isActiveSession`), `RenderMarkdown()` shared helper

### Task 2 — Adapters + Engine
- `internal/harvest/opencode_source.go`: `OpenCodeSource` delegating to `ScanOpenCodeDBRoot` + `listSessions`/`listMessages` helpers; compile-time assertion `var _ SessionSource = (*OpenCodeSource)(nil)`
- `internal/harvest/claude_source.go`: `ClaudeSource` delegating to `parseJSONLFile`; populates `Branch` and `Cwd` from per-record `gitBranch`/`cwd` JSONL fields; compile-time assertion
- `internal/harvest/engine.go`: `Engine` implementing `Harvester` (compile-time assertion `var _ Harvester = (*Engine)(nil)`); drives discover→read→dedup→skip-active→render→summarize/persist→raw-fallback pipeline; writes to `sessions` collection
- `internal/harvest/claudecode.go`: added `GitBranch`, `Cwd`, `IsSidechain` fields to `claudeCodeMessage` struct

### Task 3 — Collection Unification
- `internal/summarize/persist.go`: `Collection: "session-summary"` → `"sessions"` in `UpsertDocumentBySourcePath` and link extractor call; `buildSourcePath` adds explicit `case SourceOpenCode` and generic default `"summary://<source>/<id>"`
- `internal/server/handlers/wakeup.go`: `RecentDocuments` collections `["memory","session-summary"]` → `["memory","sessions"]`
- `internal/harvest/migration.go`: `MigrateSessionSummaryToSessions` — single `UPDATE documents SET collection='sessions' WHERE collection='session-summary'`; idempotent; logs count
- `internal/links/extract.go`: guard updated to allow `"sessions"` alongside `"memory"`/`"session-summary"` — link extraction would have silently no-oped for all renamed session summaries
- `internal/mcp/tools.go`: `memory_wake_up` `RecentDocuments` collections updated to `"sessions"`

### Task 4 — Startup Wiring
- `cmd/nano-brain/main.go`: `harvest.MigrateSessionSummaryToSessions` called after DB open, before Runner construction; non-fatal (logs warn on error, continues)

## Verification

```
CGO_ENABLED=0 go build ./...  → PASS (no output)
go test -race -short ./...    → 28 packages PASS, 0 FAIL
```

All packages pass including `internal/harvest`, `internal/summarize`, `internal/server/handlers`, `internal/links`, `internal/mcp`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] links/extract.go guard updated for 'sessions' collection**
- **Found during:** Task 3
- **Issue:** `links/extract.go:101` had `if doc.Collection != "memory" && doc.Collection != "session-summary"` — after renaming the collection, link extraction would silently return `nil` for all session summaries. The plan did not include this file.
- **Fix:** Added `&& doc.Collection != "sessions"` to the guard so link extraction continues working for summarized sessions.
- **Files modified:** `internal/links/extract.go`
- **Commit:** 03fcddb

**2. [Rule 2 - Missing critical functionality] mcp/tools.go memory_wake_up updated**
- **Found during:** Task 3 (scanning all `session-summary` references)
- **Issue:** `internal/mcp/tools.go:1545` had `Collections: []string{"memory", "session-summary"}` in the MCP `memory_wake_up` tool — would return 0 session docs after migration.
- **Fix:** Updated to `["memory", "sessions"]` alongside wakeup.go.
- **Files modified:** `internal/mcp/tools.go`
- **Commit:** 03fcddb

## Post-Review Fixes (commit 6df45ac)

Independent review APPROVED architecture, migration, and harvest/wake_up paths,
but flagged non-harvest paths still referencing the now-unwritten `session-summary`
collection. These were direct consequences of the Task 3 rename and are fixed:

**3. [Rule 1 - Bug] Cleanup/backfill/dedup queries targeted the empty `session-summary` collection**
- **Found during:** Post-execution independent review
- **Issue:** Post-unification, both raw and summarized session docs live in collection `sessions` (distinguished by source_path scheme). These queries still filtered `collection = 'session-summary'` and silently matched nothing:
  - `documents.sql:CountStaleRawOpenCodeDocs` / `DeleteStaleRawOpenCodeDocs` — summary EXISTS subquery filtered `d_summary.collection = 'session-summary'`
  - `documents.sql:ListSummaryDocumentsForBackfill` — filtered `collection = 'session-summary'`
  - `summarize.go:89` — dedup built `summaryPath := "session-summary://" + ...`, which never matched the harvester/persister scheme `summary://<source>/<id>`, so dedup always missed
- **Fix:**
  - Stale-raw queries: summary EXISTS subquery now checks `collection = 'sessions'`; raw vs summary distinguished by source_path (`opencode://session/%` vs `summary://%`)
  - Backfill query: now `collection = 'sessions' AND source_path LIKE 'summary://%'`
  - `summarize.go`: `summaryPath` now `"summary://" + sourceFromTags(tags) + "/" + sessionID` (matches `buildSourcePath`; `sourceFromTags` returns `claude`/`opencode`)
  - Regenerated `documents.sql.go` via `sqlc generate` (query consts only; left version header at v1.30.0 to avoid unrelated churn from the local v1.31.1 binary)
  - Updated `summarize_test.go` fixtures to the `summary://` scheme
  - `cmd_cleanup_stale_raw.go`: help text updated to say collection `sessions`
- **Files modified:** `internal/storage/queries/documents.sql`, `internal/storage/sqlc/documents.sql.go`, `internal/server/handlers/summarize.go`, `internal/server/handlers/summarize_test.go`, `cmd/nano-brain/cmd_cleanup_stale_raw.go`
- **Commit:** 6df45ac
- **Verification:** `sqlc generate` clean, `CGO_ENABLED=0 go build ./...` clean, `go test -race -short ./...` all pass.

## Known Stubs

None — all data paths are wired. The `Engine` is built and tested but not yet wired to `Runner` in `main.go` (engine flag path intentionally deferred per plan, to be done in a Phase 8 follow-up once confidence is established).

## Threat Flags

None — no new network endpoints, auth paths, file access patterns, or schema changes introduced. The SQL migration is a `WHERE collection = 'session-summary'` UPDATE on an existing table with no new trust boundary.

## Self-Check: PASSED

All 6 created files found on disk. All 4 task commits + post-review fix (6df45ac) verified in git log.
Build: `CGO_ENABLED=0 go build ./...` — clean.
Tests: `go test -race -short ./...` — all packages PASS, 0 FAIL.
