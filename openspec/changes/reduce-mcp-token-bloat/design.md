## Context

nano-brain has multiple resource consumption issues discovered during analysis:

**MCP Response Sizes (unbounded):**
- `memory_get`: up to 774KB (session transcripts)
- `code_impact`: 20-50KB for central symbols
- `code_context`: 5-15KB, grows with callers/callees
- `memory_focus`: grows with dependency count
- `memory_symbols` / `memory_impact`: grows with match count
- `code_detect_changes`: flows unbounded
- `memory_multi_get`: 50KB default cap
- `memory_search/query/vsearch`: 700 chars/result — already well-bounded

**Embedding Spin Loop (critical bug):**
- 296 empty-body documents (hash `e3b0c442`) stuck in pending queue
- `embedPendingCodebase()` fetches them, produces 0 chunks, never marks them done
- 2.9 million retry iterations per 8 hours, generating 214MB/day of logs
- Adaptive backoff never triggers because batch count > 0

**Log System:**
- 427MB accumulated logs, no rotation
- Synchronous `appendFileSync` per line
- No log levels — debug-level messages always written
- `[store] insertEmbeddingLocal` alone generates 26,820 lines/day

**Harvester Re-harvest Loop:**
- Sessions with missing output files re-harvested every 2 minutes
- 4,325 re-harvest triggers across 173 cycles in one day
- Full session JSON + message files re-read each time

**npm Package:** 269 files (1.7MB) published, only ~40 files (423KB) needed at runtime.

## Goals / Non-Goals

**Goals:**
- Every MCP tool response has a hard size cap with sensible defaults
- Empty-body documents never enter the embedding queue
- Log files are rotated and size-limited
- Harvester stops retrying permanently-failing sessions
- npm package contains only runtime files

**Non-Goals:**
- Changing search result quality or ranking
- Modifying the underlying data storage or chunking strategy
- Adding response compression or streaming
- Rewriting the harvester architecture (just fix the retry loop)
- Adding structured/JSON logging (keep current text format)

## Decisions

### 1. MCP Response Limits

| Tool | Current | New Default | Rationale |
|---|---|---|---|
| `memory_get` | unlimited | `maxLines=200` | 200 lines ≈ 8KB. Caller can pass higher. |
| `memory_multi_get` | 50,000 bytes | 30,000 bytes | 30KB still generous for batch retrieval |
| `code_impact` | unlimited | max 50 entries, max depth 3 | Deeper deps rarely actionable |
| `code_context` | unlimited | 20 callers + 20 callees + 10 flows | Top entries by confidence most useful |
| `memory_focus` | unlimited | 30 deps + 30 dependents | Top 30 suffices for context |
| `memory_symbols` | unlimited | 50 results | Matches typical expectations |
| `memory_impact` | unlimited | 50 results | Same reasoning |
| `code_detect_changes` | flows unbounded | 20 flows | Match existing file/symbol caps |

Targeting ~5-15KB max per tool response (~1.2K-3.7K tokens).

Truncation format: `... and N more` suffix — clear signal to AI that data was truncated.

Limits applied at response formatting layer in `server.ts`, not in store/search layer. Keeps data layer clean, allows callers to override.

### 2. Empty-Body Embedding Fix

In `embedPendingCodebase()`, after chunking a batch:
- If a document produces 0 chunks, insert a sentinel row in `content_vectors` (seq=-1) to mark it as "processed but not embeddable"
- This prevents it from appearing in `getHashesNeedingEmbedding()` results
- The sentinel row uses a special seq value that search queries already filter out (they join on seq >= 0)

**Alternative considered:** Adding a `skip_embedding` column to documents table. Rejected — requires schema migration and touches more code. Sentinel row is simpler and self-contained.

### 3. Log Rotation and Levels

Add to `logger.ts`:
- **Log levels:** `error`, `warn`, `info`, `debug` with configurable threshold via `config.logging.level` (default: `info`)
- **Rotation:** On each write, check if current log file exceeds 50MB. If so, rotate to `.1` suffix and delete files older than 7 days.
- **Demote noisy logs:** `insertEmbeddingLocal` → `debug` level. `searchFTS` → `debug`. `createStore` → `info`.

**Why not a logging library:** Zero external dependencies is a project constraint. The current approach is fine with rotation added.

### 4. Harvester Re-harvest Limit

Add a `maxRetries` counter (default: 3) per session in the harvest state file. After 3 failed re-harvest attempts (output file still missing), mark the session as permanently skipped.

State file format change:
```json
{
  "ses_abc.json": { "mtime": 1234567890, "retries": 0 },
  "ses_def.json": { "mtime": 1234567890, "retries": 3, "skipped": true }
}
```

### 5. npm `files` Whitelist

```json
"files": [
  "src/",
  "!src/eval/",
  "!src/bench.ts",
  "bin/",
  ".opencode/",
  "SKILL.md",
  "AGENTS.md",
  "AGENTS_SNIPPET.md",
  "opencode-mcp.json"
]
```

## Risks / Trade-offs

- **[Truncation hides useful data]** → All limits overridable via tool parameters. Truncation indicators show how much was cut.
- **[Sentinel row hack for empty bodies]** → Simple but unconventional. If content_vectors schema changes, sentinel logic must be updated. Documented in code.
- **[Log level change hides debug info]** → Set `logging.level: debug` in config to restore full verbosity.
- **[Harvester skip is permanent]** → After 3 retries, session won't be re-harvested even if output path issue is fixed. Manual `memory_update` can force re-harvest.
- **[npm files whitelist misses needed files]** → Verify with `npm pack --dry-run` before publishing.
