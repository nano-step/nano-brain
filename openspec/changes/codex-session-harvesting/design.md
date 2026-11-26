# Design — codex-session-harvesting

## Decision 1: File layout scan (not a single dir)

Codex rollouts live under `$CODEX_HOME/sessions/YYYY/MM/DD/*.jsonl` and
`$CODEX_HOME/archived_sessions/*.jsonl`. Unlike Claude Code's flat
per-project dirs, the date-partitioned tree means we walk recursively.

`CodexHarvester.codexHome` points at the Codex home (`~/.codex` default).
Two roots are scanned: `<home>/sessions` (recursive `filepath.WalkDir`)
and `<home>/archived_sessions` (flat). Each `*.jsonl` file is one session.
`.zst` files are skipped v1 (see Non-Goals).

Files are collected once per `HarvestAll` call into a slice — the
session count is bounded (Codex retention is user-managed, typically
hundreds not millions) and each file is read once anyway.

## Decision 2: Session metadata via bounded first-pass scan

`session_meta` is not guaranteed to be the first line (older rollouts
may lead with other record types). We scan lines until the first
`type=="session_meta"` record or EOF — capped at 256 lines — extracting
`payload.id` (session UUID) and `payload.cwd`. If none found, the file
is counted as an error (unattributable: no workspace to file under).

A second full pass renders the conversation. Two passes keep the
session-id/dedup decision cheap — we can skip the parse+render entirely
for already-harvested sessions.

Filename fallback is NOT used: `rollout-<ts>-<uuid>.jsonl` embeds the
same UUID as `session_meta.payload.id`, so when meta is missing we cannot
trust cwd attribution anyway → skip with error count.

## Decision 3: Workspace attribution

Order per session `cwd`:

1. `filepath.Clean(cwd)` exact match in the registered-workspaces map
   (path → hash) loaded once per `HarvestAll`.
2. Longest-ancestor match: cwd is a descendant of a registered
   workspace root (e.g. Codex launched in `<ws>/sub/dir`). Longest
   prefix wins when nested workspaces exist.
3. `storage.WorkspaceHash(cwd)` + `GetWorkspaceByHash` existence check
   — covers workspaces registered under a path variant our Clean missed.
4. Otherwise: count skipped (unregistered workspace — same semantics as
   OpenCodeSQLiteHarvester).

Empty cwd → skipped.

## Decision 4: Rendering — Claude-style markdown

Rendered doc mirrors `ClaudeCodeHarvester`'s conventions so
`StripClaude` and downstream consumers work unchanged:

```
---
session_id: <uuid>
source: codex
cwd: <cwd>
created_at: <ts>
---

## user (<ts>)
<text>

## assistant (<ts>)
<text>

Tool: <name>
<arguments>
Result: <output>
```

Payload mapping (all under `type=="response_item"`):

| payload.type | Render |
|---|---|
| `message` role `user` | `## user` — `input_text` blocks joined |
| `message` role `assistant` | `## assistant` — `output_text` blocks joined |
| `message` role `developer` | skipped (system prompts) |
| `message` role `agent_message` | skipped (subagent internals) |
| `reasoning` | skipped (`encrypted_content` opaque) |
| `function_call` / `custom_tool_call` | `Tool: <name>` + arguments |
| `function_call_output` / `custom_tool_call_output` | `Result: <output>` — paired to call via `call_id` map collected in the same pass |
| `web_search_call` | `Tool: web_search` + query |
| anything else | skipped |

Tool call/result pairing: outputs may appear after the call in file
order (they do — Codex writes call then output). We buffer rendered
items; when an output record arrives we look up its `call_id` in a map
of pending calls and append `Result:` under that call's `Tool:` block.
Unpaired outputs render standalone `Result:` lines; unpaired calls
render without Result. This is a single pass — no reordering needed
because outputs always follow their calls in rollout order.

Non-`response_item` record types (`event_msg`, `turn_context`,
`world_state`, `token_usage_record`) are skipped for rendering; they
carry telemetry/UI state, not conversation.

Malformed lines: skipped per-line (matching `parseJSONLFile`), truncated
final line tolerated. Session with zero rendered items → skipped.

## Decision 5: Dedup + source path + strip function

- `source_path = "summary://codex/" + sessionID` (sanitized via
  `sanitizeText` before lookup, per sanitize.go convention).
- Dedup is presence-based on `GetDocumentBySourcePath` with non-empty
  `ContentHash` — identical to Pi/ClaudeCode (shared limitation:
  grown sessions are not re-ingested; out of scope).
- `summarize.SourceCodex = "codex"`; `buildSourcePath` case;
  `pipeline.go` Source switch → `StripClaude` (rendering follows Claude
  conventions — `## role` headers + `Tool:`/`Result:` bodies);
  `sourceFromTags` (handlers/summarize.go), `extractBackfillSource`
  (cmd_backfill_summaries.go), `SourceFromPath` (storage/sourcepath.go)
  all gain a `"codex"` branch — the pi-session-harvesting post-review
  found all three secondary sites; we cover them up front.
- Raw fallback uses shared `persistRawFallback` (raw_fallback.go).
- Tags: `["codex", "session", "summary"]` matching the pi/claude tag
  convention.

## Decision 6: Config

```yaml
harvester:
  codex:
    enabled: false
    codex_home: ""        # default: $CODEX_HOME or ~/.codex
```

- `enabled` gates everything (opt-in like pi/claudecode).
- `codex_home` resolution order: config value → `$CODEX_HOME` env →
  `~/.codex`. No `~` expansion for config value (matches
  `ClaudeCodeHarvesterConfig.SessionDir` precedent — document absolute
  path in CONFIGURATION.md).
- Env override `NANO_BRAIN_HARVESTER_CODEX_HOME`? — check how pi/claude
  env overrides are named; match that convention (`CODEX_HOME` itself is
  already Codex's own env var and is honored in the chain above).
- `detectCodexHome()` in detect.go returns the resolved home or "" —
  used to auto-fill config at init like other detectors.

## Decision 7: Startup wiring

`cmd/nano-brain/codex_init.go`:

```go
func initCodexHarvesters(ctx, cfg config.CodexHarvesterConfig,
    db *sql.DB, logger zerolog.Logger) []*harvest.CodexHarvester
```

Single harvester per daemon (Codex home is global, unlike Claude
Code's per-project dirs — no workspace-encoding step). Returns nil when
disabled or `sessions/`+`archived_sessions/` both absent. Workspace
filtering happens inside `HarvestAll` via the registered-workspaces
query, same as OpenCodeSQLiteHarvester.

`main.go`: after pi/claudecode wiring, `for _, h := range
initCodexHarvesters(...) { hr.AddHarvester(h) }` and
`hr.WithSummarizer(...)` propagates automatically via
`summarizerSettable`.

## Testing

- `codex_test.go`: fixture rollout JSONL covering session_meta (first
  and non-first position), user/assistant messages, function_call +
  output pairing, custom_tool_call, web_search_call, developer/
  agent_message/reasoning/event_msg skipping, malformed line, truncated
  final line, missing session_meta, missing cwd, .zst skip, nested
  date dirs, archived_sessions, exact source path, render content
  assertions.
- `codex_init_test.go`: disabled → nil; missing dirs → nil; present →
  one harvester with resolved home.
- Integration (`//go:build integration`): presence-dedup against real
  `nanobrain_test` — mirrors `pi_dedup_integration_test.go`.
- Pipeline test: `SourceCodex` routes through `StripClaude` (mirror
  `TestPipeline_Summarize_PiSourceUsesStripClaude`).
- `sourcepath_test.go` + `extractBackfillSource` test additions.

## Validation ladder

`gofmt` → `go build ./...` → `go test -race -short ./...` →
`go test -race -tags=integration ./internal/harvest/... ./internal/summarize/...
./internal/config/... ./internal/server/handlers/... ./internal/storage/...
./cmd/nano-brain/...` against `nanobrain_test`.
