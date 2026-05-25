## Context

nano-brain's three search tools (`memory_query`, `memory_search`, `memory_vsearch`) call `formatSearchResults()` which renders each result as a markdown block: title, path, score, lines, optional symbols/cluster/flows, and a full ~700-char snippet. A typical 10-result query produces ~7000 chars of snippet text alone — most of which the calling agent never reads.

Headroom's CCR (Compress-Cache-Retrieve) pattern demonstrates that returning compact summaries and offering on-demand expansion can save 60-80% tokens without losing information access. We adapt this pattern to nano-brain's search pipeline as an opt-in feature.

**Current state:**
- `formatSearchResults()` at server.ts:158 — always renders full snippets
- `SearchResult` interface (types.ts:1-19) — includes `snippet: string` (~700 chars), `docid`, `title`, `path`, `score`, `symbols`, `clusterLabel`, `flowCount`
- `memory_get` tool exists for full document retrieval by path/docid, but requires knowing the docid upfront and returns the entire document — not the specific snippet that matched
- No caching layer between search and result delivery

**Constraints:**
- TypeScript, Node.js (ESM), better-sqlite3, sqlite-vec
- Zero external runtime dependencies
- Performance-sensitive MCP server (stdio transport)
- Must not break existing CLI `--json` output format
- MCP stdio = single client per server process (no concurrent access concerns)
- Must not break existing MCP consumers (backward compatibility)

## Goals / Non-Goals

**Goals:**
- Reduce token consumption of search results by 60-80% when agents opt into compact mode
- Preserve full data access via on-demand expansion (`memory_expand`)
- Cache full search results server-side so expansion doesn't re-run the search
- Maintain CLI parity with a `--compact` flag
- Zero breaking changes — verbose remains the default everywhere

**Non-Goals:**
- LLM-generated summaries (too slow, adds dependency) — we use deterministic truncation
- Adaptive snippet sizing based on expansion history (future work)
- Compression of non-search tool outputs (memory_status, memory_get, etc.)
- Changing the `SearchResult` interface or search pipeline internals
- Breaking the existing `--json` CLI output format
- Expansion tracking / analytics (deferred to v2 — YAGNI until we have a concrete question to answer)

## Decisions

### D1: Compact format — deterministic first-line truncation

**Choice:** Compact results show `{index}. [{score}] {title} ({docid}) — {path}:{startLine} | {first_line_of_snippet_truncated_to_80_chars}`

**Alternatives considered:**
- LLM-generated summaries: Too slow (100ms+ per result), adds inference dependency, non-deterministic
- Keyword extraction: Moderate complexity, still needs NLP, marginal improvement over first-line
- First N words: Loses structural context (first line of code/prose is more meaningful than first N words)

**Rationale:** First-line truncation is zero-cost, deterministic, and preserves the most informative part of the snippet (function signatures, headings, first sentences). The docid is included so agents can call `memory_expand` directly.

### D2: Result cache — in-memory Map with TTL-only eviction

**Choice:** `Map<string, { results: SearchResult[], query: string, startLines: number[], endLines: number[], docids: string[], expires: number }>`. Default TTL: 15 minutes. No entry limit — TTL-only eviction with lazy cleanup on access. Cache also stores `startLine`, `endLine`, and `docid` per result for bounded fallback on cache miss.

**Alternatives considered:**
- SQLite table: Durable but unnecessary — cache only needs to survive within a session, not across restarts. Adds write overhead on every search.
- LRU eviction with entry cap: Over-engineered for single-client stdio. With ~1 search/min, we'll never accumulate enough entries to matter within 15 min TTL.
- WeakRef-based cache: Too aggressive eviction, no TTL control.
- No cache (re-run search on expand): Expensive, non-deterministic (results may change between search and expand).

**Rationale:** MCP stdio = single client per server process. Access pattern is sequential (search → expand → search → expand). TTL-only is the simplest correct solution. 15-min TTL covers realistic workflows where an agent does other work before expanding. Lazy cleanup (check expiry on `get()`, periodically sweep on `set()`) keeps memory bounded without complexity.

### D3: Cache key design — incrementing counter

**Choice:** Incrementing counter per server lifetime: `search_1`, `search_2`, `search_3`, etc. The cache key is returned to the client as `cacheKey` in the compact response header.

**Alternatives considered:**
- SHA-256 of serialized params: Overkill for a single-client in-memory cache. No collision risk with one client. Adds crypto dependency for no benefit.
- Raw JSON string as key: Works but verbose in output. Agent has to pass back a long string.

**Rationale:** Single-client stdio means zero collision risk. Short keys are easier for agents to parse and pass back. Counter resets on server restart, but so does the cache — no inconsistency.

**Per-tool parameter sets** (for deterministic duplicate detection, if needed later):
- `memory_query`: `{ query, limit, collection, minScore, workspace, tags, since, until }`
- `memory_search`: `{ query, limit, collection, workspace }` (FTS exact match)
- `memory_vsearch`: `{ query, limit, collection, workspace }` (semantic search)

### D4: `memory_expand` tool — expand by cacheKey + index/indices or docid

**Choice:** New MCP tool with parameters:
- `cacheKey` (string, required): The cache key from the compact search response
- `index` (number, optional): 1-based result index to expand (single result)
- `indices` (number[], optional): Array of 1-based indices to expand (multiple results)
- `docid` (string, optional): Document ID to expand (fallback if cache expired)

If cache hit: return full snippet(s) from cached results. If cache miss and docid provided: fall back to `store.findDocument()` + `store.getDocumentBody(docid, startLine, maxLines)` using the cached `startLine`/`endLine` for bounded retrieval. If cache fully expired (no line info): return first 2000 chars of document body with warning. If neither cache nor docid: return error with suggestion to re-run search.

**Alternatives considered:**
- Expand all at once: Defeats the purpose — agents should expand selectively
- Expand by docid only: Loses the specific snippet context (docid maps to full document, not the matched chunk)
- Index-only (no indices array): Forces agents to make N separate calls to expand N results — inefficient

**Rationale:** Index-based expansion is the common case. `indices` array supports efficient multi-expand without N round-trips. Docid fallback with stored line ranges provides bounded retrieval even on cache miss — avoids returning unbounded 50KB documents.

### D5: Default behavior — verbose everywhere, compact is opt-in

**Choice:** Both MCP tools and CLI commands default to verbose (current behavior). Compact mode is opt-in via `compact: true` parameter (MCP) or `--compact` flag (CLI).

**Alternatives considered:**
- Compact as default for MCP: Saves tokens automatically but breaks existing agents that expect verbose output. Silent behavioral change is worse than wasting tokens.

**Rationale:** Backward compatibility is paramount. Existing agents and AGENTS.md instructions assume verbose output. We can update AGENTS.md to recommend `compact: true` and flip the default in a future major version once agents are adapted. This is a non-breaking additive change.

### D6: Expansion tracking — deferred to v2

**Choice:** No expansion tracking in v1. The `expansion_log` table and related logic are deferred until we have a concrete analytics question to answer.

**Alternatives considered:**
- Lightweight SQLite table logging every expand: ~0.1ms overhead per call, but no concrete plan to use the data. Classic YAGNI.

**Rationale:** "Learn optimal snippet sizing" is a hypothesis, not a requirement. We should collect data when we have a specific question (e.g., "what % of results get expanded?"). Adding tracking later is trivial — it's a single table + one INSERT per expand call. Not worth the code/test surface area in v1.

## Risks / Trade-offs

**[Cache memory growth]** → Mitigation: 15-min TTL with lazy cleanup. Single-client stdio means ~1 search/min realistic max. Worst case ~1MB. No entry cap needed.

**[Cache miss on expand]** → Mitigation: Cache stores `startLine`/`endLine`/`docid` per result. On miss, fallback uses `store.getDocumentBody(docid, startLine, maxLines)` for bounded retrieval. If line info also lost, returns first 2000 chars with warning.

**[Adoption friction]** → Mitigation: Compact is opt-in (not default). AGENTS.md can be updated to recommend `compact: true` for token-sensitive workflows. Zero risk of breaking existing consumers.

**[Agents not understanding compact format]** → Mitigation: The compact response includes a 1-line instruction: "Use memory_expand(cacheKey, index) for full content." Agents that don't understand simply don't pass `compact: true`.

**[Special characters in compact line]** → Mitigation: If title contains `|` or `—`, the compact format could be ambiguous. Implementation should escape or replace these characters in the title portion.
