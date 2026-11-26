# Proposal — codex-session-harvesting

Issue: https://github.com/nano-step/nano-brain/issues/739
Lane: normal · Change type: user-feature

## Why

Codex CLI is already a supported MCP client (`nano-brain init` writes
`mcp_servers` to `~/.codex/config.toml`), so Codex agents can call all
`memory_*` tools today. But Codex session transcripts are never ingested —
every new Codex session starts with amnesia of what previous sessions
decided, built, or debugged.

Codex stores sessions as append-only JSONL rollout files:

```
$CODEX_HOME/sessions/YYYY/MM/DD/rollout-<ts>-<uuid>.jsonl[.zst]
$CODEX_HOME/archived_sessions/rollout-*.jsonl[.zst]
$CODEX_HOME/history.jsonl          # prompts only, not sessions
```

Each rollout line is `{timestamp, type, payload}` with `type` in
`{session_meta, response_item, event_msg, turn_context, world_state,
token_usage_record}`. `response_item.payload` carries the conversation:
`message` (roles `developer`/`user`/`assistant`/`agent_message`),
`reasoning` (encrypted_content — opaque), `function_call`,
`function_call_output`, `custom_tool_call`, `custom_tool_call_output`,
`web_search_call`. `session_meta.payload` carries `id` and `cwd`.

This change adds a `CodexHarvester` that ingests these rollouts into the
sessions pipeline, giving Codex users the same session-continuity memory
Claude Code / OpenCode / Pi users already have.

## What Changes

- New `CodexHarvester` (`internal/harvest/codex.go`) implementing the
  `Harvester` interface, modeled on `PiHarvester`/`ClaudeCodeHarvester`:
  scans `<codex_home>/sessions/` recursively plus
  `<codex_home>/archived_sessions/`, renders each rollout to markdown,
  presence-dedups on `summary://codex/<session-id>`, persists raw
  fallback or hands to the `SessionSummarizer`.
- New opt-in config `harvester.codex.{enabled, codex_home}` with
  `CODEX_HOME` env override, auto-detected at `~/.codex` (or
  `$CODEX_HOME`) when unset. **Disabled by default**, matching Pi/Claude
  Code.
- Workspace attribution: `session_meta.payload.cwd` is matched against
  registered workspaces — exact `filepath.Clean` match first, then
  longest-ancestor match (Codex is often run in a subdirectory of the
  registered workspace), then `WorkspaceHash(cwd)` registration check.
  Sessions that cannot be attributed are counted skipped, not ingested.
- New `summarize.SourceCodex` source (`"codex"`): `summary://codex/<id>`
  source paths, `StripClaude` rendering pipeline (Codex renders in the
  same `## role` + `Tool:`/`Result:` markdown convention), tag dispatch
  in `sourceFromTags`/`extractBackfillSource`/`SourceFromPath`.
- Startup wiring in `cmd/nano-brain/codex_init.go` + `main.go`;
  `detectCodexHome` helper alongside existing detectors.
- `.zst`-compressed rollouts are **skipped v1** (counted as skipped with
  a debug log) — decompression is a follow-up; uncompressed rollouts are
  the common case while sessions are recent.

## Non-Goals

- `.zst` decompression (needs `klauspost/compress/zstd`; small dep,
  deferred to keep this diff focused).
- `history.jsonl` ingestion (prompt log only — no assistant content).
- Codex IDE/VS Code extension sessions (different store).
- CCS-managed Codex profiles (`~/.ccs/codex-instances/*`) — covered
  generically once `codex_home` points at them; not special-cased.
- MCP client config changes — Codex MCP setup already exists (#739 docs
  half is already covered by SETUP_AGENT.md).
- Re-summarizing grown sessions — presence-dedup is the inherited
  shared limitation (see pi-session-harvesting design.md).

## Risks

- **Format drift**: Codex rollout schema evolves between CLI releases.
  Parser is tolerant: unknown `type`/`payload.type` values are skipped
  per-line, malformed lines skipped (not fatal), matching
  `ClaudeCodeHarvester.parseJSONLFile` semantics.
- **Encrypted reasoning**: `reasoning` payloads carry
  `encrypted_content` — not rendered (opaque anyway).
- **Large tool outputs**: `function_call_output`/`custom_tool_call_output`
  can embed whole files; chunker + StripClaude already bound this the
  same way they bound Claude Code transcripts.
