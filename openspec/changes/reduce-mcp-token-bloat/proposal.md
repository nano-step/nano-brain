## Why

nano-brain has multiple resource consumption issues discovered during analysis:

1. **MCP tool responses are unbounded** — `memory_get` can return 774KB (~3M tokens) for a single session file. `code_impact` and `code_context` return unbounded dependency trees. Every tool response entering the AI's context window should have a hard size cap.

2. **Critical embedding spin loop** — 296 documents with empty bodies (hash `e3b0c442`) are stuck in the pending embedding queue. The embedding cycle fetches them, produces 0 chunks, but never marks them as embedded. This causes 2.9 million retry iterations per 8-hour period, generating 214MB of logs per day and wasting CPU.

3. **Log system has no rotation or size limits** — 427MB of logs accumulated with no cleanup. Logging is synchronous (`appendFileSync`) and has no level filtering — every `insertEmbeddingLocal` call generates a log line (26,820/day just for that tag).

4. **Harvester re-harvest loop** — Sessions with missing output files get re-harvested every 2 minutes (4,325 re-harvest triggers across 173 cycles in one day), reading full session JSON + message files each time.

5. **npm package bloat** — 75% of the published package (1.2MB of 1.7MB) is dev-only files (test/, openspec/, site/, ai/).

## What Changes

### MCP Response Limits
- Add hard response size limits to all MCP tool handlers that return unbounded text
- `memory_get`: Default `maxLines=200` cap
- `memory_multi_get`: Reduce default `maxBytes` from 50,000 to 30,000
- `code_impact`: Truncate by depth (max 3) and total entries (max 50)
- `code_context`: Cap callers/callees (20 each) and flows (10)
- `memory_focus`: Cap dependency/dependent lists (30 each)
- `memory_symbols` / `memory_impact`: Cap result lists (50 each)
- `code_detect_changes`: Cap flows (20, matching existing file/symbol caps)

### Embedding Bug Fix
- Skip empty-body documents in `embedPendingCodebase()` — mark them as "not embeddable" instead of retrying forever
- Add the empty-hash check before entering the embedding loop

### Log System Improvements
- Add log rotation (delete logs older than N days)
- Add log level support (error, warn, info, debug) with configurable threshold
- Add max log file size with rotation

### Harvester Fix
- Track re-harvest failures and stop retrying after N attempts
- Don't re-read full session files just for output path validation

### npm Package
- Add `"files"` whitelist to `package.json`

## Capabilities

### New Capabilities
- `mcp-response-limits`: Hard size limits on all MCP tool responses to prevent unbounded token consumption
- `log-management`: Log rotation, log levels, and size limits

### Modified Capabilities
- `mcp-server`: Tool handlers SHALL enforce response size limits; `memory_get` SHALL have a default line cap

## Impact

- `src/server.ts` — All tool handler response formatting
- `src/codebase.ts` — Empty-body skip in `embedPendingCodebase()`
- `src/logger.ts` — Log rotation, levels, max size
- `src/harvester.ts` — Re-harvest retry limit
- `package.json` — Add `"files"` field
- No breaking changes to tool input schemas — all limits have sensible defaults and can be overridden by callers
