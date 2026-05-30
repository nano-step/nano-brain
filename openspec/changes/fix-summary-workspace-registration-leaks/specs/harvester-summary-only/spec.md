# harvester-summary-only Delta — Fix Workspace-Registration Leaks

## MODIFIED Requirements

### Requirement: Summary-first harvest when summarizer is active

When `cfg.Summarization.Enabled = true` and the summarizer client is available, both the OpenCode SQLite harvester (`internal/harvest/opencode_sqlite.go`) and the Claude Code harvester (`internal/harvest/claudecode.go`) SHALL persist ONLY the LLM-generated summary document for each session. They SHALL NOT also persist the raw session transcript. The summary document SHALL be stored at `source_path = "summary://<source>/<session_id>"` in the `session-summary` collection. **Both harvesters SHALL verify that the target `workspace_hash` is registered in the `workspaces` table before persisting any document. Sessions targeting an unregistered workspace SHALL be skipped with a WARN log.**

#### Scenario: OpenCode session indexed as summary only

- **WHEN** `HarvestAll()` processes a previously-unseen OpenCode session with a non-empty `worktree` that maps to a registered workspace_hash and summarizer is active
- **THEN** `SummarizeAndPersist` is called with the rendered raw markdown
- **AND** a document with `source_path = "summary://opencode/<session_id>"` exists in `documents` table with `collection = 'session-summary'`
- **AND** NO document with `source_path = "opencode://session/<session_id>"` exists for that session
- **AND** chunks generated from the summary text are enqueued for embedding

#### Scenario: Claude Code session indexed as summary only

- **WHEN** the Claude Code harvester is started with a `session_dir` whose computed `WorkspaceHash` is registered in the `workspaces` table, and the harvester processes a previously-unseen session, and summarizer is active
- **THEN** a document with `source_path = "summary://claudecode/<session_filename>"` exists in `documents` table with `collection = 'session-summary'`
- **AND** NO document with `source_path = "claudecode://session/<session_filename>"` exists for that session

#### Scenario: OpenCode session with empty worktree is skipped

- **WHEN** `HarvestAll()` processes an OpenCode session whose `worktree` field is empty
- **THEN** the harvester logs at WARN: `opencode harvest: skipping orphan session (no worktree)` with the session_id
- **AND** NO document of any kind is created for that session
- **AND** the harvester continues processing other sessions

#### Scenario: OpenCode session with unregistered worktree is skipped

- **WHEN** `HarvestAll()` processes an OpenCode session whose `worktree` maps to a `workspace_hash` that is NOT present in the `workspaces` table
- **THEN** the harvester logs at WARN: `opencode harvest: skipping session for unregistered workspace` with session_id, worktree, and computed workspace_hash
- **AND** NO document of any kind is created for that session
- **AND** the harvester does NOT fall back to `WorkspaceHash(dbPath)` (legacy fallback is removed)

#### Scenario: Claude Code harvester refuses to start for unregistered session_dir

- **WHEN** server bootstrap reads `cfg.Harvester.ClaudeCode.Enabled = true` and `cfg.Harvester.ClaudeCode.SessionDir = /path/X`
- **AND** the computed `WorkspaceHash("/path/X")` is NOT present in the `workspaces` table
- **THEN** the server logs at WARN: `claude code session_dir is not a registered workspace; harvester disabled. Run nano-brain init --root=<path> to register.` with the computed hash
- **AND** the Claude Code harvester is NOT added to the harvest runner
- **AND** NO documents are created under that hash

### Requirement: Skip-check uses summary source path

The harvest skip-check SHALL identify already-processed sessions by looking up the summary `source_path` (`summary://<source>/<id>`) when summarizer is active, instead of the raw `<source>://session/<id>` path. Sessions whose summary already exists with matching content hash SHALL be skipped without LLM calls and without DB writes. **Skip-check lookups SHALL use the registered workspace_hash; sessions targeting unregistered workspaces are skipped before reaching this check.**

#### Scenario: Re-harvest of already-summarized session is idempotent

- **GIVEN** a session has been harvested and summary persisted at `summary://opencode/ses_abc123` with content_hash = `H` under a registered workspace_hash
- **WHEN** the harvester encounters the same session again with unchanged content
- **THEN** the skip-check returns true
- **AND** NO LLM call is made
- **AND** NO database write occurs
