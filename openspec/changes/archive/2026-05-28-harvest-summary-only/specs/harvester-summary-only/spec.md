# harvester-summary-only Delta Specification

**Tracking:** nano-step/nano-brain#189

## ADDED Requirements

### Requirement: Summary-first harvest when summarizer is active

When `cfg.Summarization.Enabled = true` and the summarizer client is available, both the OpenCode SQLite harvester (`internal/harvest/opencode_sqlite.go`) and the Claude Code harvester (`internal/harvest/claudecode.go`) SHALL persist ONLY the LLM-generated summary document for each session. They SHALL NOT also persist the raw session transcript. The summary document SHALL be stored at `source_path = "summary://<source>/<session_id>"` in the `session-summary` collection.

#### Scenario: OpenCode session indexed as summary only

- **WHEN** `HarvestAll()` processes a previously-unseen OpenCode session and summarizer is active
- **THEN** `SummarizeAndPersist` is called with the rendered raw markdown
- **AND** a document with `source_path = "summary://opencode/<session_id>"` exists in `documents` table with `collection = 'session-summary'`
- **AND** NO document with `source_path = "opencode://session/<session_id>"` exists for that session
- **AND** chunks generated from the summary text are enqueued for embedding

#### Scenario: Claude Code session indexed as summary only

- **WHEN** the Claude Code harvester processes a previously-unseen session and summarizer is active
- **THEN** a document with `source_path = "summary://claudecode/<session_filename>"` exists in `documents` table with `collection = 'session-summary'`
- **AND** NO document with `source_path = "claudecode://session/<session_filename>"` exists for that session

### Requirement: Skip-check uses summary source path

The harvest skip-check SHALL identify already-processed sessions by looking up the summary `source_path` (`summary://<source>/<id>`) when summarizer is active, instead of the raw `<source>://session/<id>` path. Sessions whose summary already exists with matching content hash SHALL be skipped without LLM calls and without DB writes.

#### Scenario: Re-harvest of already-summarized session is idempotent

- **GIVEN** a session has been harvested and summary persisted at `summary://opencode/ses_abc123` with content_hash = `H`
- **WHEN** `HarvestAll()` runs again and the session's raw content still hashes to `H`
- **THEN** the session is skipped (skipped counter incremented)
- **AND** no LLM call is issued for that session
- **AND** no DB write occurs

### Requirement: Graceful fallback to raw when summarizer is absent or fails

When `cfg.Summarization.Enabled = false`, OR the summarizer client failed to initialize, OR the summarizer returns an error for a specific session, the harvester SHALL fall back to persisting the raw transcript so that no session is lost. The fallback raw document SHALL be stored under the same `source_path = "summary://<source>/<id>"` as the summary would have used, so that subsequent harvests with summarizer healthy treat it as already-processed (preventing re-call during outages).

#### Scenario: Summarization disabled in config

- **GIVEN** `cfg.Summarization.Enabled = false`
- **WHEN** `HarvestAll()` processes a session
- **THEN** the raw transcript is persisted under `source_path = "summary://opencode/<session_id>"` in collection `sessions` with `metadata.fallback = true`
- **AND** a WARN log is emitted: `summarizer not configured, falling back to raw`

#### Scenario: Summarizer returns error mid-harvest

- **GIVEN** summarizer is active but the LLM provider returns 5xx
- **WHEN** `SummarizeAndPersist` returns error for a specific session
- **THEN** a WARN log is emitted with `session_id` and `error`
- **AND** the raw transcript is persisted under `source_path = "summary://opencode/<session_id>"` in collection `sessions` with `metadata.fallback = true`
- **AND** harvest continues to the next session (error counter NOT incremented; treated as soft failure)
- **AND** the `summary_fallback` counter for this harvest cycle is incremented

#### Scenario: DB upsert error during summary persistence falls back to raw

- **GIVEN** summarizer succeeded and produced a summary
- **WHEN** `Persister.Save` returns a database error (e.g., connection drop, constraint violation)
- **THEN** a WARN log is emitted with `session_id` and the DB error
- **AND** the harvester attempts a raw fallback upsert under `source_path = "summary://opencode/<session_id>"` with `metadata.fallback = true`
- **AND** if the fallback upsert also fails, the session is skipped and `errors` counter is incremented (no infinite retry)

#### Scenario: Fallback documents are NOT automatically re-summarized when LLM recovers

- **GIVEN** session `S` has a fallback raw doc at `summary://opencode/S` (collection `sessions`, `metadata.fallback = true`)
- **AND** the session's underlying messages have NOT changed since the fallback was written
- **WHEN** the summarizer is now healthy and the next harvest cycle runs
- **THEN** the skip-check finds the existing doc with matching `content_hash` → session is skipped
- **AND** the doc remains at `collection = 'sessions'` with `metadata.fallback = true`
- **AND** no LLM call is made for this session
- **NOTE** Operators can re-summarize via `DELETE FROM documents WHERE source_path = 'summary://opencode/S' AND collection = 'sessions'` then re-harvest; or wait for session content to change (which auto-triggers re-processing via content_hash mismatch). Future work: `nano-brain harvest --resummarize-fallbacks` CLI flag (tracked separately).

### Requirement: Summary document carries correct workspace_hash

The summary persister (`internal/summarize/persist.go`) SHALL receive the workspace hash for each session via the `SummaryMeta` struct, NOT via a globally-hardcoded empty string at `Persister` construction. Both harvesters SHALL pass `WorkspaceHash(session.Worktree)` for OpenCode sessions and `WorkspaceHash(session_dir)` for Claude Code sessions when calling `SummarizeAndPersist`.

#### Scenario: Summary stored under session's project workspace

- **GIVEN** an OpenCode session whose `project.worktree = "/Users/alice/projects/foo"`
- **WHEN** the session is summarized
- **THEN** the summary document's `workspace_hash` column equals `WorkspaceHash("/Users/alice/projects/foo")`
- **AND** the document is NOT stored with `workspace_hash = ''` (empty string)

#### Scenario: Summary document is queryable per-project

- **GIVEN** a summary document persisted under workspace hash `H` (matching a registered workspace)
- **WHEN** `POST /api/query` is called with `workspace = <session worktree path>`
- **THEN** the summary document appears in the result set
- **AND** the document does NOT appear when querying a different workspace

### Requirement: Summarizer initialized before harvester construction

The application bootstrap in `cmd/nano-brain/main.go` SHALL construct the summarizer before constructing the harvester runner. The runner SHALL accept the summarizer as a required constructor argument when summarization is enabled, NOT via a post-construction `WithSummarizer()` builder call. When `cfg.Summarization.Enabled = false`, the runner SHALL accept `nil` and the harvesters SHALL operate in raw-fallback mode.

#### Scenario: Bootstrap with summarization enabled

- **GIVEN** `cfg.Summarization.Enabled = true` with valid provider URL and API key
- **WHEN** the server starts
- **THEN** the summarizer client is constructed first
- **AND** the harvester runner receives the summarizer in its constructor
- **AND** no post-init `WithSummarizer()` call occurs

#### Scenario: Bootstrap with summarization disabled

- **GIVEN** `cfg.Summarization.Enabled = false`
- **WHEN** the server starts
- **THEN** the harvester runner is constructed with summarizer = nil
- **AND** harvesters operate in raw-fallback mode
- **AND** no panic or crash on startup

### Requirement: Summary persister no longer writes intermediate .md files

The summary persister SHALL NOT write `.md` files to `output_dir`. The DB document is the single source of truth for summary content. The `writeFile()` method SHALL be removed from `Persister` and the `Persister` constructor SHALL NOT require an `output_dir` parameter. The `output_dir` YAML config key SHALL be retained as a no-op (silently ignored) to preserve backward compatibility for existing user config files; the field SHALL be removed from the `SummarizationConfig` struct and from the `nano-brain init` interactive prompt.

#### Scenario: Summary persistence does not touch the filesystem

- **WHEN** `SummarizeAndPersist` succeeds for a session
- **THEN** the summary content is stored in the `documents.content` column
- **AND** no `.md` file is created in `~/.nano-brain/summaries/` (or any other directory)
- **AND** `Persister` constructor does not require an `output_dir` parameter

#### Scenario: Existing config with output_dir does not break startup

- **GIVEN** an existing user config file containing `summarization.output_dir: ~/.nano-brain/summaries`
- **WHEN** the server starts
- **THEN** config parsing succeeds (no `unknown field` error)
- **AND** the value is silently ignored
- **AND** no `.md` files are written

### Requirement: Harvest cycle emits structured observability counters

At the end of each harvest cycle, each harvester SHALL emit a single structured INFO log line with the breakdown of session outcomes: `summary_success` (sessions summarized successfully), `summary_fallback` (sessions written as raw fallback because summarizer was nil, returned error, or DB upsert failed), `skipped` (sessions matched by skip-check), `active` (sessions skipped by the active-session age gate), and `errors` (sessions that failed both summary and raw-fallback persistence).

#### Scenario: Mixed-outcome harvest cycle emits structured counters

- **GIVEN** a harvest cycle processes 10 sessions: 7 summarize successfully, 2 trigger summarizer error and fall back, 1 is skipped by content_hash match
- **WHEN** the harvester finishes the cycle
- **THEN** exactly one INFO log line is emitted with structured fields:
  `summary_success=7 summary_fallback=2 skipped=1 active=0 errors=0`
- **AND** the log message is `harvest cycle complete` (or equivalent canonical phrase)
- **AND** the log includes the harvester source identifier (e.g., `source=opencode` or `source=claudecode`)
