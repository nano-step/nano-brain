# harvest package

Session harvesting — ingests OpenCode and Claude Code AI sessions into nano-brain storage.

## Files

| File | Role |
|------|------|
| `runner.go` | `Runner` — owns the tick loop, serializes harvest cycles via mutex, fans out to all registered `Harvester` impls |
| `harvest.go` | Shared types: `Harvester` interface, `ChunkEnqueuer`, `SessionSummarizer`, `SummaryMeta` |
| `opencode_sqlite.go` | `OpenCodeSQLiteHarvester` — opens OpenCode's SQLite DB read-only, renders sessions to markdown, upserts into PG; filters by registered workspace paths before ingesting. Also exports `ScanOpenCodeDBRoot` (multi-DB discovery helper). |
| `opencode.go` | Legacy OpenCode JSON session format reader |
| `claudecode.go` | `ClaudeCodeHarvester` — scans a directory for Claude Code JSONL files, renders each session to markdown, upserts into PG |
| `automemory.go` | `AutoMemoryExtractor` — regex-scans harvested session content for `DECISION:` / `LESSON:` markers and heading patterns; writes extracted items as discrete memory documents |

## Key Pattern

```
External DB / files
  └─ Harvester.HarvestAll()
       ├─ render session → markdown string
       ├─ SHA-256 content hash → skip if unchanged
       ├─ ChunkEnqueuer.Enqueue() → PG upsert
       └─ SessionSummarizer.SummarizeAndPersist() (optional, async)
```

All harvesters are registered with `Runner.AddHarvester()`. `Runner.Run()` fires immediately then ticks at the configured interval. `RunOnce()` is the serialized entry point — used by both the ticker and `POST /api/harvest`.

## Workspace Filtering

`OpenCodeSQLiteHarvester` queries registered nano-brain workspaces from PG, then filters SQLite sessions by matching the session's `project.worktree` against those workspace root paths. Worktree is normalized via `filepath.Clean` before lookup so trailing-slash variants still match (preserving empty-string semantics — Gemini review on PR #200 caught that `filepath.Clean("")` returns `"."`). Sessions from unregistered workspaces are counted as skipped, not ingested.

## Multi-DB Discovery (db_root mode)

OpenCode harvesters are selected at startup in priority order:

| Priority | Mode | Config key | Env override |
|----------|------|-----------|--------------|
| 1 | `db_root` | `harvester.opencode.db_root` | `OPENCODE_DB_ROOT` |
| 2 | `db_path` | `harvester.opencode.db_path` | `OPENCODE_DB_PATH` |
| 3 | `session_dir` | `harvester.opencode.session_dir` | `OPENCODE_STORAGE_DIR` |
| — | disabled | (none set) | — |

### db_root mode

`ScanOpenCodeDBRoot(ctx, root, registered, logger)` globs `<root>/*/opencode.db`, opens each read-only, reads `project.worktree`, normalizes via `filepath.Clean`, and matches against the registered workspace map (path → hash). One `OpenCodeSQLiteHarvester` is instantiated per match. Zero matches falls through to the next priority.

Auto-detected at `~/.ai-sandbox/opencode-dbs` on linux/darwin when not set.

Discovery is startup-only; new per-project DBs require daemon restart (live rescan deferred — see `docs/HARNESS_BACKLOG.md`).

## SQLite Access

Uses `modernc.org/sqlite` (pure Go, `CGO_ENABLED=0`). Opens external DBs with `?mode=ro` to avoid write-lock conflicts. `openSQLite()` accepts an injected `*sql.DB` for tests; falls back to opening `dbPath` otherwise.

## Summarizer Integration

`SessionSummarizer` is optional. When set via `Runner.WithSummarizer()`, it's propagated to every harvester that implements `summarizerSettable`. Harvesters call `SummarizeAndPersist` after a successful upsert. On error or nil summarizer, they fall back to raw markdown storage.

## AutoMemory

`AutoMemoryExtractor` runs post-harvest on session markdown. Regex patterns target:
- Heading-level: `## Decisions`, `## Lessons Learned` (and variants)
- Inline: `DECISION: ...` and `LESSON: ...` lines

Each extracted item is written as a separate document with `collection=auto-memory`.

## Testing

- Unit tests use `NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB)` with in-memory SQLite fixtures — no file I/O.
- Integration tests (`//go:build integration`) call `testutil.SetupTestDB` for a real PG schema.
- `claudecode_test.go` mocks `ChunkEnqueuer` with an inline struct.
