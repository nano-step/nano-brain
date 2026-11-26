# Spec — codex-session-harvesting

## ADDED Requirements

### Requirement: Codex harvester is disabled by default
The Codex session harvester SHALL be disabled unless explicitly enabled via
`harvester.codex.enabled: true`.

#### Scenario: Default config leaves Codex harvesting off
- **WHEN** the daemon starts with no `harvester.codex` configuration present
- **THEN** no `CodexHarvester` is instantiated and no Codex session directories are scanned

#### Scenario: Operator opts in
- **WHEN** `harvester.codex.enabled: true` is set and the resolved Codex home contains a `sessions/` or `archived_sessions/` directory
- **THEN** the daemon instantiates one `CodexHarvester` for the resolved home

### Requirement: Codex home resolution
The harvester SHALL resolve the Codex home directory in this order:
`harvester.codex.codex_home` config value, then `$CODEX_HOME`, then
`~/.codex`. The config value is used as-is (no `~` expansion), matching the
`SessionDir` precedent of existing harvesters.

#### Scenario: Env relocation honored
- **WHEN** `CODEX_HOME=/data/codex` is set and no config value is present
- **THEN** the harvester scans `/data/codex/sessions` and `/data/codex/archived_sessions`

#### Scenario: CCS-style relocation
- **WHEN** an operator sets `harvester.codex.codex_home` to a CCS codex-instance directory
- **THEN** that directory's `sessions/` tree is harvested with no CCS-specific code

### Requirement: Rollout discovery
The harvester SHALL scan `<codex_home>/sessions/` recursively and
`<codex_home>/archived_sessions/` non-recursively for `*.jsonl` files. Files
ending in `.zst` SHALL be skipped. Other file types SHALL be ignored.

#### Scenario: Date-partitioned session tree
- **WHEN** `sessions/2026/11/26/rollout-2026-11-26T10-00-00-<uuid>.jsonl` exists
- **THEN** it is discovered and treated as one session

#### Scenario: Compressed rollout skipped
- **WHEN** only `rollout-*.jsonl.zst` files exist for a session
- **THEN** no document is created for it and it is counted as skipped

### Requirement: Session identity and workspace attribution
Each rollout SHALL be attributed by the `id` and `cwd` fields of its
`session_meta` record (found by scanning from the top of the file, at most
256 lines). Workspace matching SHALL try, in order: exact `filepath.Clean`
match against registered workspace paths, longest-ancestor match (session
cwd inside a registered workspace root), and `WorkspaceHash(cwd)`
registration check. Sessions that cannot be attributed SHALL be counted as
skipped, not ingested.

#### Scenario: Session in registered workspace
- **WHEN** a rollout's `session_meta.payload.cwd` equals a registered workspace root
- **THEN** the session is harvested under that workspace's hash

#### Scenario: Session in workspace subdirectory
- **WHEN** `cwd` is `<registered-workspace>/sub/dir`
- **THEN** the session is harvested under the longest matching ancestor workspace

#### Scenario: Session in unregistered directory
- **WHEN** `cwd` matches no registered workspace by any of the three strategies
- **THEN** the session is counted skipped and no document is created

#### Scenario: No session_meta record
- **WHEN** a rollout file contains no `session_meta` record in its first 256 lines
- **THEN** it is counted as an error and not ingested

### Requirement: Rollout rendering to markdown
The harvester SHALL render each attributed rollout to a single markdown
document. `response_item` records SHALL be rendered by payload type:
`message`/`user` and `message`/`assistant` as `## user`/`## assistant`
sections with their text content blocks; `function_call` and
`custom_tool_call` as `Tool: <name>` blocks with their arguments;
`function_call_output`, `custom_tool_call_output`, and `web_search_call`
as `Result:`/tool blocks paired via `call_id`. `developer` and
`agent_message` roles, `reasoning` records (encrypted), and all non-
`response_item` record types SHALL be excluded. Malformed lines SHALL be
skipped without discarding the session; a truncated final line SHALL be
tolerated.

#### Scenario: Well-formed rollout
- **WHEN** a rollout contains user text, assistant text, a function_call followed by its function_call_output, and event_msg/token_usage records
- **THEN** the rendered markdown contains the user and assistant text and a `Tool:`/`Result:` pair, and contains no event_msg or token_usage content

#### Scenario: Reasoning excluded
- **WHEN** a rollout contains `reasoning` records with `encrypted_content`
- **THEN** no reasoning content appears in the rendered markdown

#### Scenario: Malformed line mid-file
- **WHEN** one line fails JSON parsing but surrounding lines are valid
- **THEN** that line is skipped and the rest of the session still renders

### Requirement: Canonical source path and presence dedup
Each harvested Codex session SHALL be stored with `source_path` of the form
`summary://codex/<session-id>` and SHALL be skipped on later harvest cycles
once a document exists at that path with a non-empty content hash — the same
presence-based dedup `ClaudeCodeHarvester`/`PiHarvester` use. Grown sessions
are not re-ingested (shared pre-existing limitation, out of scope).

#### Scenario: Unchanged session on later tick
- **WHEN** a document already exists at `summary://codex/<session-id>` with non-empty content hash
- **THEN** the session is counted skipped without re-rendering or re-summarizing

#### Scenario: Source path format
- **WHEN** a session with `session_meta.payload.id = "019a…"` is harvested
- **THEN** its document's `source_path` is `summary://codex/019a…`

### Requirement: Codex source plumbed through summarize pipeline
`SourceCodex` SHALL be a `summarize.Source` constant routed through
`StripClaude` in the pipeline strip dispatch (Codex rendering follows Claude
Code markdown conventions). The `"codex"` tag/source SHALL be recognized by
`sourceFromTags` (summarize handler), `extractBackfillSource`
(backfill-summaries CLI), and `SourceFromPath` (storage), matching how
`"pi"`/`"claude"` are handled.

#### Scenario: Summarize dispatch
- **WHEN** the summarizer processes a document tagged `codex`
- **THEN** its content is stripped via `StripClaude` and `SessionMetadata.Source` is `"codex"`

#### Scenario: Source path reverse lookup
- **WHEN** `SourceFromPath("summary://codex/<id>")` is called
- **THEN** it returns `"codex"`

### Requirement: Persistence and summarizer integration
Harvested sessions SHALL persist via `persistRawFallback` when no
summarizer is configured or summarization fails, and SHALL invoke
`SessionSummarizer.SummarizeAndPersist` with `Source: SourceCodex` and
`SessionID`/`Workspace`/`Timestamp` metadata when one is set. `HarvestAll`
SHALL emit a structured log line with `Str("source", "codex")` and
harvested/skipped/error counters.

#### Scenario: No summarizer configured
- **WHEN** `WithSummarizer` was never called
- **THEN** each new session is stored as a raw markdown document in the `sessions` collection with chunks enqueued

#### Scenario: Summarizer configured
- **WHEN** a summarizer is set via `Runner.WithSummarizer`
- **THEN** new sessions are handed to `SummarizeAndPersist` after upsert
