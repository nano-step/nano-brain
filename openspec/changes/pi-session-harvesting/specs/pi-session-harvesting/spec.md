## ADDED Requirements

### Requirement: Pi harvester is disabled by default
The Pi session harvester SHALL be disabled unless explicitly enabled via `harvester.pi.enabled: true` (or the equivalent environment override), consistent with the Claude Code harvester's opt-in default.

#### Scenario: Default config leaves Pi harvesting off
- **WHEN** the daemon starts with no `harvester.pi` configuration present
- **THEN** no `PiHarvester` is instantiated and no Pi session directories are scanned

#### Scenario: Operator opts in
- **WHEN** `harvester.pi.enabled: true` is set and `harvester.pi.session_root` resolves to an existing directory
- **THEN** the daemon instantiates one `PiHarvester` per registered workspace that has a matching Pi session directory

### Requirement: Pi sessions are matched to registered workspaces by directory encoding
For each registered workspace, the system SHALL encode the workspace's root path using Pi's directory-naming scheme (every `/` replaced with `-`, then wrapped in a leading and trailing `--`) and instantiate a harvester only when a directory of that exact name exists under the configured Pi session root.

#### Scenario: Matching workspace and Pi directory
- **WHEN** a workspace is registered at `/Users/alice/project` and `~/.pi/agent/sessions/--Users-alice-project--/` exists
- **THEN** a `PiHarvester` is created for that workspace, scoped to that directory

#### Scenario: No matching Pi directory
- **WHEN** a workspace is registered but no correspondingly-encoded directory exists under the Pi session root
- **THEN** no harvester is created for that workspace, and no error is raised (this is a benign, expected case)

### Requirement: Pi session files are parsed and rendered to markdown
The harvester SHALL parse each `*.jsonl` file in a matched Pi session directory as a sequence of JSON objects, extract the session `id` and `cwd` from the first (`type: "session"`) line, and render every subsequent `type: "message"` line into a single markdown document per session. Rendering SHALL dispatch on the nested `message.role` field (`user`, `assistant`, and `toolResult` are all rendered — `toolResult` is not a sub-part of `assistant` content and MUST NOT be dropped) and, within each message, on each `message.content[]` block's `type`: `text` blocks render as their `text` value; `toolCall` blocks render as a compact tool-invocation line; `thinking` blocks render as a labeled thinking-trace section; `image` blocks render as a placeholder line. Non-message event lines (`type: "model_change"`, `type: "thinking_level_change"`) SHALL be ignored for rendering purposes.

#### Scenario: Well-formed session file
- **WHEN** a Pi session file contains a header line followed by a mix of `message` (with `user`, `assistant`, and `toolResult` roles and `text`/`toolCall`/`thinking`/`image` content blocks), `model_change`, and `thinking_level_change` lines
- **THEN** the rendered markdown includes the content of every `message` line — across all three roles and all four content-block types — in file order, and excludes `model_change`/`thinking_level_change` lines entirely

#### Scenario: Malformed line
- **WHEN** one line in a session file fails to parse as JSON
- **THEN** that single line is skipped (matching `ClaudeCodeHarvester`'s existing per-line-skip behavior), the rest of the same session file continues to be parsed and rendered, and the session is not discarded

#### Scenario: Truncated last line
- **WHEN** a session file's final line is a partial write (process terminated mid-write, no trailing newline, incomplete JSON)
- **THEN** that final line is treated as a malformed line (skipped, not fatal to the session) and the session is re-harvested on a later tick once the write completes and its content hash changes

### Requirement: Pi sessions are deduplicated by presence, matching Claude Code's existing behavior
The harvester SHALL skip re-summarizing a session once a document already exists at its canonical `source_path`, using the same presence-based dedup mechanism `ClaudeCodeHarvester` already uses — not a content-hash comparison against the new rendering. This is an explicit, inherited limitation shared with Claude Code, not a Pi-specific gap: a session file that grows after its first harvest is **not** re-ingested by either harvester today. Fixing that (re-summarizing on real content growth) is a pre-existing cross-harvester limitation and out of scope for this change.

#### Scenario: Unchanged session on a later harvest tick
- **WHEN** a Pi session file has already been harvested (a document exists at its `summary://pi/<session-id>` path with a non-empty content hash)
- **THEN** the harvester skips re-summarizing it without inspecting the file's current content (counted as skipped, not harvested)

#### Scenario: Session file grows (new messages appended)
- **WHEN** a previously-harvested Pi session file gains new `type: "message"` lines
- **THEN** the harvester still skips it on presence alone (matching `ClaudeCodeHarvester`'s existing behavior) — the new messages are NOT picked up until this shared limitation is addressed in a separate change

### Requirement: Pi documents use a canonical source path
Each harvested Pi session SHALL be stored with `source_path` of the form `summary://pi/<session-id>`, where `<session-id>` is the `id` field from the session's header line, matching the existing `summary://<source>/<session-id>` convention used by Claude Code and OpenCode.

#### Scenario: Source path format
- **WHEN** a Pi session with header `id: "019fdb51-2ce0-7085-a1d1-471be7c19602"` is harvested
- **THEN** its document is stored with `source_path = "summary://pi/019fdb51-2ce0-7085-a1d1-471be7c19602"`

### Requirement: Pi content is stripped using Claude Code's strip function before storage
The summarization pipeline's strip-function dispatch (`internal/summarize/pipeline.go`) SHALL route content whose `SessionMetadata.Source` is `SourcePi` through `StripClaude`, not the `OpenCode` strip path, since Pi's rendered markdown follows Claude Code's rendering conventions (see the "Pi session files are parsed and rendered to markdown" requirement).

#### Scenario: Pi session summarized via fresh harvest
- **WHEN** `Pipeline.Summarize` is called with `meta.Source` set to `SourcePi`
- **THEN** the content is stripped using `StripClaude`, not `StripOpenCode`

### Requirement: Manually triggered re-summarization correctly classifies Pi documents
The manual/backfill re-summarization path (`TriggerSummarize`'s `sourceFromTags` classification) SHALL recognize a Pi document's tag and classify it as `"pi"`, so its dedup lookup uses the same canonical `source_path` the fresh-harvest path produces, rather than defaulting to `"opencode"` and creating a duplicate document.

#### Scenario: Re-summarizing a Pi-tagged document
- **WHEN** `TriggerSummarize` processes a document tagged with the Pi session tag (mirroring how a Claude Code document is tagged `"claude_code"`)
- **THEN** `sourceFromTags` returns `"pi"`, the dedup lookup checks `summary://pi/<session-id>`, and no duplicate document is created for a session that was already harvested
