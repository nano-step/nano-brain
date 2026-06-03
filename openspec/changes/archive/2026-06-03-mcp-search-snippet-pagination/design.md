## Context

### Current state

nano-brain exposes three search MCP tools (`memory_query`, `memory_search`, `memory_vsearch`) backed by the shared `internal/search/Service.HybridSearch` pipeline:

```
query → [BM25 leg | Vector leg]  (parallel, errgroup, fetchLimit = max(maxResults*3, 30))
         ↓
        RRF fusion (k=60, computed in Go)
         ↓
        Recency boost (exponential half-life decay)
         ↓
        Slice [0 : maxResults]
         ↓
        Marshal to JSON via textResult()
```

The `search.Result` struct (`internal/search/search.go:10-22`) carries both a `Snippet` field (already populated, ≤700 chars from the SQL `LEFT(content, ...)` projection or from a Go-side truncation) and a `Content` field (full chunk text, often 5–50 KB).

The HTTP layer (`internal/server/handlers/search.go`) maps `search.Result` → a local `SearchResult` struct that omits `Content` entirely and emits `Snippet` only. The MCP layer marshals raw `search.Result` (or near-equivalent inline `resultRow` structs) — so `Content` ships on every result.

The MCP `tools/call` response envelope (per MCP 2025-06-18 spec) has no built-in pagination cursor. Listing endpoints (`tools/list`, `resources/list`) do, but tool-call results are free-form `{content[], structuredContent?, isError?}`. Any pagination on tool-call results is therefore a **server-defined convention**, not a protocol feature.

### Constraints

- **Single static binary, no Redis.** Server-side cache for paginated snapshots would add lifecycle complexity (eviction, TTL, capacity). Avoid.
- **Stateless server preferred.** Each request must be answerable from request + DB alone.
- **Read-mostly memory workload.** Writes happen seconds–minutes apart for the same workspace; agent pagination happens in seconds. Offset-drift on a mutable index is acceptable.
- **Embedding API call is expensive (200–800 ms).** Re-running hybrid search per page re-embeds the query each time. Acceptable now; LRU cache is tracked separately (#359).
- **HTTP API stays untouched.** Out of scope. Different surface, different consumers, no truncation problem there.

### Stakeholders

- **AI coding agents** consuming nano-brain via MCP (OpenCode, Claude Code, Cursor, others). They are the only direct consumers of the search tool responses.
- **Skill docs** (`.opencode/skills/nano-brain/SKILL.md`) — the agent-facing contract surface.
- **CI / integration tests** — zero response-shape coverage exists today; this change must add it.

## Goals / Non-Goals

**Goals:**

1. Reduce default MCP search response size by **≥85%** (target: ≤15 KB for `max_results=10`) so responses fit in OpenCode's ~50 KB tool-output budget without triggering truncation.
2. Provide a **stateless, opaque cursor** so agents can paginate beyond `max_results` without dumping the entire result set in one shot.
3. Preserve a **migration path** for existing agents that need the full content (`include_content: true` opt-in + `memory_get` recommendation).
4. **Deterministic pagination order** — same query + same cursor on a quiescent index returns the same slice.
5. **No new infrastructure.** No cache, no Redis, no separate service. All work in-process and stateless.

**Non-Goals:**

- Time-range filters (`created_after` / `created_before`). Different feature, separate issue.
- Changing the HTTP API contract. Already correct.
- Query-embedding LRU cache. Tracked as #359; orthogonal value.
- Per-stage latency log instrumentation. Useful but separate.
- Refactoring the three search tools to share a response struct. Code quality, not a user-facing fix.
- `memory_wake_up` changes. Already snippet-only and bounded.
- Adding pagination to non-search tools (`memory_get`, `memory_status`, etc.). Out of scope.

## Decisions

### D1 — Default to snippet-only, opt-in for full content

**Decision:** Drop the `content` field from all three MCP search tool responses by default. Add an `include_content` boolean parameter (default `false`) that re-enables full content per request.

**Why:**
- The HTTP API already does this (`handlers/search.go:35-47`). MCP was the inconsistent outlier.
- SKILL.md never documented `content` as a search-tool return field — only `snippet`.
- Most agent workflows want "what matched and where" first, then full content for the 1–3 hits they care about. The `memory_get` tool already covers the full-content fetch (and supports line slicing).
- Empirically gives the ~90% size reduction needed to fit under the truncation threshold at default settings.

**Alternatives considered:**

| Option | Rejected because |
|---|---|
| Keep `content`, add `compact: true` opt-in | Doesn't fix the default-case bloat that motivates this issue. Most agents won't opt in. |
| Rename `content` → `snippet` (same field, just truncated) | Misleading: the same JSON key sometimes returns 700 chars, sometimes 50 KB depending on call site. Worse than removal. |
| New tool: `memory_search_v2` etc. | Doubles the MCP tool count, splits documentation, doesn't retire the broken default. Pure tech debt. |

**Trade-off:** Breaking change for any agent that consumed the undocumented `content` field. Mitigated by the `include_content` escape hatch and explicit migration text in tool descriptions + SKILL.md.

### D2 — Stateless offset-based cursor with query-hash guard

**Decision:** Encode the cursor as `base64url(JSON{"o": offset, "q": queryHash})` where `queryHash` is the first 16 hex chars of `sha256(query_text)`. Server decodes, verifies the hash matches the current request's query, and re-runs hybrid search with `LIMIT = offset + page_size + 1` to detect "has more".

**Why:**
- Stateless — no server cache, no TTL, no eviction. Survives server restarts. Survives load-balancer rebalances if nano-brain is ever horizontally scaled.
- Query-hash guard prevents a foot-gun: agent passes the cursor from query A to query B and silently gets wrong-looking results.
- Offset-based is correct on a quiescent index. On a mutating index it can drift by a small N (new docs inserted between pages), which is acceptable for memory-search workloads.
- The cursor is fully self-describing; no shared secret or signing scheme needed. Opaque-to-client requirement is satisfied by base64.

**Alternatives considered:**

| Option | Rejected because |
|---|---|
| Score-based cursor (`score < X`) | RRF scores are computed in Go, not directly indexable. Ties between equal-score results force a secondary sort key anyway. Marginal benefit, much more complexity. |
| Server-side snapshot cache keyed by cursor token | Adds an eviction + capacity subsystem to a single-binary server. Wrong shape for nano-brain. |
| Composite cursor `(query_hash, snapshot_ts, offset)` | Snapshot_ts is unused without a server-side snapshot. Premature complexity. |
| Reuse MCP `cursor` / `nextCursor` listing convention verbatim | MCP `tools/call` results have no `nextCursor` field — pagination on tool-call results is a server-defined extension. We borrow the naming convention but not a protocol field. |

**Trade-off:** Each paginated page re-embeds the query (200–800 ms). Acceptable because deep pagination is rare; query-embedding cache (#359) makes the trade-off even better when shipped.

### D3 — Stable tiebreaker on tied scores

**Decision:** When RRF (`internal/search/rrf.go`) or recency boost (`internal/search/recency.go`) produces tied scores, break the tie by `id ASC`. Apply at both stages.

**Why:**
- Without a stable secondary sort, offset-based cursoring can silently reorder results between pages (map-iteration order in Go is randomized; PostgreSQL `LIMIT` without `ORDER BY` is undefined).
- Tied scores already mean "equally relevant" — picking the smaller `id` is arbitrary but stable.
- One-line change in two existing sort comparators.

**Alternative considered:** Sort by `created_at ASC`. Rejected because `created_at` ties also happen on bulk imports and lack the uniqueness guarantee `id` (UUID) carries.

### D4 — Snippet length = 500 chars in MCP (vs 700 in HTTP)

**Decision:** Truncate snippets to 500 chars in the MCP layer, even though HTTP uses 700.

**Why:**
- MCP consumers are token-constrained AI agents; HTTP consumers are humans (curl, browser, dashboard).
- At `max_results=10`, the difference is 2 KB (10 × 200 chars). Small enough to matter for token budgets.
- 500 chars is roughly 80–120 tokens — long enough to disambiguate hits, short enough to scan in bulk.

**Alternative considered:** Make snippet length configurable per-call (`snippet_len` parameter). Rejected to keep the parameter surface small. Easy to add later if a real use case emerges.

### D5 — Re-run hybrid search per page, no caching

**Decision:** Each paginated request re-runs `HybridSearch` from scratch with `maxResults = cursor.offset + page_size + 1` (the +1 detects "more pages exist").

**Why:**
- Correct under index mutation — results always reflect current data.
- No cache lifecycle to manage.
- PG indexed search at LIMIT 300 is sub-100 ms in practice; the bottleneck is the embedding API, which the LRU cache (#359) addresses orthogonally.

**Trade-off:** Linear cost in offset for deep pages. Practical max is page 10 (offset 100, fetchLimit ≈ 300 per leg) — still well within PG capacity. If real workloads ever exceed page 10 routinely, we revisit with a snapshot-cache decision; not today.

### D6 — Approximate `total` field

**Decision:** Include a `total` field in the response equal to the count of results in the **current fused result set** (i.e., `len(boosted)` before slicing), not the count of all possible matches in the index.

**Why:**
- Exact total requires a separate `COUNT(*)` query on both BM25 and vector legs — adds latency without much benefit.
- Approximate total answers the operational question the agent actually has: "did I see everything, or should I page?"
- If `len(boosted) == requested fetchLimit` we can't be sure no more exist — that's exactly what `next_cursor` signals.

**Trade-off:** `total` underestimates on later pages (the result set grows as fetchLimit grows). Documented in the field description.

### D7 — Shared `truncateSnippet` helper extracted to `internal/search/snippet.go`

**Decision:** Move the 12-line `truncateSnippet` function from `internal/server/handlers/search.go:49-61` to a new file `internal/search/snippet.go` and import it from both the HTTP and MCP layers.

**Why:**
- DRY across two callers, both in this codebase.
- Lives next to `search.Result` and the other search-layer helpers.
- Avoids drift if HTTP truncation logic ever changes.

**Trade-off:** One-line import in `handlers/search.go`. Trivial.

## Risks / Trade-offs

| Risk | Severity | Likelihood | Mitigation |
|---|---|---|---|
| Agents parsing the undocumented `content` field break on PR merge | High | Medium | (a) `include_content: true` escape hatch preserves old behavior. (b) Tool descriptions + SKILL.md updated with explicit migration text. (c) `memory_get` already exists for full-content fetch. (d) Document the break in CHANGELOG.md. |
| Offset-drift on rapidly-written workspaces causes duplicate/missing results across pages | Low | Low | Memory-search workloads write minutes apart, paginate seconds apart. Documented as known limitation in the cursor field description. Escalation: revisit with snapshot-cache design if real-world reports surface. |
| Cursor passed from query A to query B silently returns query-B results from query-A's offset | Low | Medium | Query-hash guard in cursor decode. Mismatch → `INVALID_CURSOR` error with clear text. Unit-tested. |
| Stable-tiebreaker change reorders ties for existing search callers | Low | Low | Only affects results with mathematically identical RRF/recency scores. "Equally relevant" results getting a stable order is an improvement, not a regression. |
| Deep pagination (page 10+) becomes slow due to fetchLimit growth | Medium | Low | Caps naturally at `max_results × 3 × current_page_offset_factor`. Practical max ~300 rows per leg. Mitigated long-term by #359 (query-embedding cache). If real workloads exhibit deep pagination, revisit with snapshot cache. |
| New `include_content: true` agents re-introduce the original payload problem | Low | Low | Opt-in is explicit. Tool description warns about size impact. Operator can monitor via `query_ms` and request-size logs (existing telemetry). |
| MCP integration tests are unfamiliar territory in this repo | Medium | Low | `internal/mcp/tools_test.go` already exists with `testutil.SetupTestDB`. Pattern is established. |
| Snippet truncation breaks on multi-byte unicode boundaries | Low | Medium | `truncateSnippet` in handlers already handles this (rune-aware). Reusing it via D7 inherits the correctness. Add explicit unicode-boundary test in `snippet_test.go`. |

## Migration Plan

1. **Ship the change as a single PR.** No multi-phase rollout — the response shape change is the entire fix.
2. **CHANGELOG.md entry** under the next release announces the MCP-only breaking change, the `include_content` opt-in, and the `cursor` parameter. Calls out: "HTTP API unchanged."
3. **SKILL.md update** lands in the same PR — agent docs match the new contract on day one.
4. **Tool descriptions** in `tools.go` updated to mention the new parameters and the snippet-by-default behavior. MCP clients auto-discover via `tools/list`.
5. **No data migration** — no DB schema changes, no goose migration, no embedding rebuild.
6. **Rollback:** Single-PR revert. No persistent state to undo.

## Open Questions

None blocking. The following are tracked or deferred:

- LRU query-embedding cache → tracked as #359, independent of this change.
- Per-stage latency log instrumentation → noted in Metis review, separate enhancement.
- Time-range filters (`created_after` / `created_before`) → separate issue; would partially address the "last 30 days" use case.
- Whether to expose `snippet_len` as a per-call parameter → deferred; ship with fixed 500 chars, revisit if requested.
