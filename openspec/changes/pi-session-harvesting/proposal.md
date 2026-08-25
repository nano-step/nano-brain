## Why

nano-brain's session harvester (`internal/harvest/`) currently ingests only two coding-agent sources: Claude Code (`ClaudeCodeHarvester`, JSONL files under `~/.claude/projects/<encoded-cwd>/`) and OpenCode (`OpenCodeSQLiteHarvester`, SQLite DBs). A third coding agent, **Pi** (a CLI agent installed at `~/.pi/agent/`, confirmed present on this machine), writes its own per-project session transcripts to `~/.pi/agent/sessions/<encoded-cwd>/<timestamp>_<uuid>.jsonl` — verified directly on disk (multiple session files for this repo, 120 files / 7,839 lines total across all projects under `~/.pi/agent/sessions/` at review time; the count grows continuously, so exact figures drift). None of that history is currently visible to nano-brain's cross-session memory (`memory_query`, `memory_wake_up`), even though the harvest architecture was explicitly designed to add new sources this way (`Harvester` interface + `Runner.AddHarvester()`; see `internal/harvest/AGENTS.md`).

This proposal adds Pi as a third harvested source, following the exact extension point the architecture already provides — no new abstraction, no framework change.

## What Changes

- New `PiHarvester` (`internal/harvest/pi.go`) implementing the existing `Harvester` interface, modeled directly on `ClaudeCodeHarvester`: scans a session directory for `*.jsonl` files, parses each one, renders it to markdown (handling all three message roles — `user`, `assistant`, `toolResult` — and all content-block types — `text`, `toolCall`, `thinking`, `image` — per the corrected schema in design.md), content-hashes it, and calls the existing `SessionSummarizer`/`ChunkEnqueuer` pipeline.
- New `initPiHarvesters()` in `cmd/nano-brain/pi_init.go`, mirroring `initClaudeCodeHarvesters()`: for each registered workspace, encode its root path using Pi's own directory-naming scheme, check whether `~/.pi/agent/sessions/<encoded>` exists, and instantiate one harvester per match.
- New `PiHarvesterConfig` (`Enabled bool`, `SessionDir string`) under `HarvesterConfig.Pi`, following `ClaudeCodeHarvesterConfig`'s field-naming exactly (`SessionDir`, not a novel `SessionRoot`; no `TicketPatterns` — see design.md's Non-Goals for why that field is deliberately omitted rather than copied a third time as dead config). **Disabled by default** (opt-in), consistent with the existing Claude Code harvester's opt-in default — Pi sessions are private developer transcripts and must not be silently ingested by an upgrade.
- New `SourcePi` constant and a `case SourcePi: return "summary://pi/" + meta.SessionID` branch in `buildSourcePath` (`internal/summarize/persist.go`) — the function already has a generic fallback for unknown sources, so this is a one-line addition for consistency with the explicit-case style the existing comment calls for, not a functional necessity.
- **New `case SourcePi: stripped = StripClaude(sessionContent)` in `internal/summarize/pipeline.go`'s `Source` switch.** Found during adversarial review: without this, a Pi transcript falls into the switch's `default` branch and is run through `StripOpenCode` instead — silently wrong content stripping despite the "pipeline unchanged" framing this proposal originally assumed. See design.md Decision 5.
- **`sourceFromTags` (`internal/server/handlers/summarize.go`) gains a `"pi"` tag → `"pi"` branch.** Found during adversarial review: this second, independent source-classification function (used only on the manual/backfill re-summarization path) defaults any unrecognized tag — including a Pi document's tag — to `"opencode"`, which would corrupt the dedup lookup path and create duplicate documents. See design.md Decision 6.
- Docs: `internal/harvest/AGENTS.md` gains a "Pi" row/section; root `SKILL.md`/`docs/SETUP_AGENT.md` gain the new `harvester.pi.*` config keys where the Claude Code ones are documented today; `internal/config/template.go`'s `fullConfigTemplate` gains a `pi:` block (not `config.example.yml`, which does not exist in this repo — see design.md Decision 8).

### Non-goals (explicitly out of scope for this change)

- **Not harvesting `.pi-subagents/artifacts/`.** A separate, project-local directory (`<repo>/.pi-subagents/artifacts/*.jsonl`) holds records of Pi's own *subagent delegations* (e.g. code-review subagent runs), not the primary coding session. It has a different shape (per-run `meta.json`/`input.md`/`output.md`/`transcript.jsonl` quadruplet, project-relative rather than a global per-project directory) and a different consumer story (delegation audit trail vs. session memory). Harvesting it is a plausible future capability but is not part of this change — flagging it as a follow-up avoids scope creep and an unreviewed second data model in the same PR.
- **Not building a generic "coding agent plugin" abstraction.** The `Harvester` interface and `Runner.AddHarvester()` are already source-agnostic; adding Pi is a second data point (after Claude Code, OpenCode) proving the existing extension point generalizes. No new registration framework, config schema abstraction, or adapter-discovery mechanism is introduced. A fourth agent later follows this same recipe.
- **No live filesystem watching of `~/.pi/agent/sessions/`.** Like Claude Code and OpenCode's `session_dir` mode, discovery/matching happens per harvest tick (`Runner`'s existing interval), not via fsnotify.

## Capabilities

### New Capabilities
- `pi-session-harvesting`: ingest Pi CLI agent session transcripts (`~/.pi/agent/sessions/<encoded-cwd>/*.jsonl`) into nano-brain's document store via the existing harvest pipeline, opt-in and disabled by default.

### Modified Capabilities
(none — this is additive; no existing capability's requirements change)

## Impact

- **New files:** `internal/harvest/pi.go`, `internal/harvest/pi_test.go`, `cmd/nano-brain/pi_init.go`, `cmd/nano-brain/pi_init_test.go`.
- **Modified files:** `internal/config/config.go` (`HarvesterConfig.Pi` field + `PiHarvesterConfig` type), `internal/summarize/persist.go` (`SourcePi` constant + `buildSourcePath` case), `internal/summarize/pipeline.go` (`case SourcePi` in the strip-function switch — see design.md Decision 5), `internal/server/handlers/summarize.go` (`sourceFromTags`'s `"pi"` branch — see design.md Decision 6), `cmd/nano-brain/main.go` (wire `initPiHarvesters` alongside the existing Claude Code / OpenCode wiring), `internal/harvest/AGENTS.md`, `internal/config/template.go` (new `pi:` block in `fullConfigTemplate`), root `SKILL.md` / `docs/SETUP_AGENT.md` (document the new config keys).
- **No schema/migration changes** — reuses the existing `sessions` collection and `documents`/`chunks` tables; `sessions` is a DB-backed logical collection (no filesystem watching implications — see the `fix-install-ux-regressions` change for why that distinction matters; confirmed during review that `isLogicalCollection` gates by collection name, so Pi reusing `Collection:"sessions"` is automatically covered with no extra change needed).
- **No breaking changes.** Disabled by default; existing Claude Code and OpenCode harvesting behavior is unaffected.
