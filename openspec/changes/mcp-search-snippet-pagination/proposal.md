Tracking: #358

## Why

The MCP search tools (`memory_query`, `memory_search`, `memory_vsearch`) currently return the FULL document `content` field on every result. With the default `max_results=10`, a single search response is typically 50–500 KB. This exceeds the host agent's tool-output threshold (~50 KB on OpenCode), forcing the agent to spawn a second subagent just to grep the truncated file — adding 30–60 seconds per memory lookup.

The HTTP layer (`internal/server/handlers/search.go`) solved this years ago by mapping results to `SearchResult` with only a 700-char `Snippet` field. The MCP layer (`internal/mcp/tools.go`) bypassed that pattern and returns raw `search.Result` with both `Snippet` and `Content`. The `Content` field is also undocumented in `.opencode/skills/nano-brain/SKILL.md` for search tools, yet it is the single largest contributor to response bloat.

In addition, neither the HTTP nor the MCP layer supports pagination. An agent asking "what bugs did we fix in the last 30 days" has only two options today: cap at `max_results=10` (and miss most matches) or `max_results=100` (and dump 1–5 MB of irrelevant chunks). There is no way to incrementally browse hits.

## What Changes

- **BREAKING (MCP only):** `memory_query`, `memory_search`, `memory_vsearch` no longer include the full `content` field in search results by default. They return a 500-char `snippet` instead. Agents that need full text MUST either pass `include_content: true` or call `memory_get` with the result's `id`/`source_path`.
- Add **opt-in `include_content` boolean parameter** (default `false`) to all three search tools. When `true`, every result includes the full `content` field alongside `snippet` (preserves the pre-change payload for agents that genuinely need it).
- Add **stateless cursor-based pagination** to all three search tools:
  - New input parameter `cursor` (optional opaque string).
  - New response field `next_cursor` (present only when more results exist beyond the current page).
  - New response field `total` (approximate count of results available in the current fused result set, for "should I page?" decisions).
  - Cursor format: `base64url(JSON{offset, query_hash})`. Server validates the query hash on every page to prevent cross-query cursor reuse.
- Add new response field `query_ms` (server-side latency, useful for operator visibility — already present in HTTP responses).
- **Stable result ordering:** Tied RRF / recency scores now break by `id ASC` (deterministic across paginated pages).
- HTTP API (`/api/v1/search`, `/api/v1/query`, `/api/v1/vsearch`) is **unchanged** — already snippet-only, and changing it is out of scope for this PR.
- `memory_wake_up` is **unchanged** — already returns 200-char `LEFT(content, 200)` snippets and is paginated by `limit` only.
- `memory_get` is **unchanged** — already the canonical full-content fallback (supports `start_line`/`end_line` slicing).
- Update `.opencode/skills/nano-brain/SKILL.md` to (a) explicitly document the `snippet` field, (b) document the new `cursor` / `include_content` parameters, (c) recommend `memory_get` for full content.
- Add response-shape integration tests (none exist today — zero coverage on MCP search response fields).

### Explicitly OUT of scope (separate issues)

- Time-range filters (`created_after` / `created_before`) on search tools — would partially answer the user's "last 30 days" use case but requires SQL + new parameters; tracked as a follow-up.
- LRU query-embedding cache — independent optimization (repeated query text → cached vector); tracked as #359.
- Per-stage latency log instrumentation on `HybridSearch` — observability improvement; tracked separately.
- Changing the HTTP API response shape — already snippet-only; out of scope.
- Refactoring the three search tools to share a response struct — code quality, not a user-facing fix.

## Capabilities

### New Capabilities
- None.

### Modified Capabilities
- `mcp-server`: search tool response contract changes (drop `content` by default → return `snippet`; add `cursor` / `include_content` parameters; add `next_cursor` / `total` / `query_ms` response fields). Existing requirements on workspace scoping, schema, and response validity are preserved.
- `search-pipeline`: result ordering acquires a stable tiebreaker (`id ASC` on tied scores). RRF fusion and recency boost behavior unchanged.

## Impact

- **Affected code:** `internal/mcp/tools.go` (three search tool registrations), `internal/search/cursor.go` (new file, ~60 lines), `internal/search/snippet.go` (new file, extracts the 12-line `truncateSnippet` helper currently inlined in `handlers/search.go`), `internal/search/rrf.go` and `internal/search/recency.go` (stable tiebreaker on tied scores), `.opencode/skills/nano-brain/SKILL.md` (documentation).
- **Unchanged:** `internal/search/service.go` (HybridSearch signature stays; caller passes a larger `maxResults` for deep pages), `internal/search/search.go` (`Result` struct already has both `Snippet` and `Content` fields — we just stop forwarding `Content` in the MCP layer), all SQL queries in `internal/storage/queries/`, HTTP handlers in `internal/server/handlers/`.
- **MCP clients:** Agents parsing `content` from search results will get an empty/missing field. They must adopt either `include_content: true` or `memory_get`. Documentation update + tool description text covers the migration path.
- **Tests:** Add `internal/search/cursor_test.go`, `internal/search/snippet_test.go`, `internal/mcp/tools_pagination_integration_test.go`. No existing tests are deleted; the existing `tools_test.go` continues to cover registration and error paths.
- **Performance:** Default-case response size drops ~90% (50–500 KB → ~5–15 KB). Deep pages (page 2+) re-run the full hybrid search with `LIMIT = offset + page_size + 1` — acceptable because (a) deep pagination is rare, (b) PG handles 300-row limits trivially, (c) embedding cache is tracked separately (#359).
- **Risk gates touched:** `public-api-contract` (MCP tool response shape changes), `existing-behavior` (default response changes), `search-quality` (response visibility changes — hard gate), `multi-domain` (mcp + search packages). Lane: **high-risk**.
