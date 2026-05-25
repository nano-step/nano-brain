## Context

nano-brain runs as a single Docker container serving 6 simultaneous OpenCode containers. It indexes ~9500 documents across multiple workspaces (zengamingx, Tools, sessions). Current hybrid search (FTS5 + sqlite-vec + RRF fusion) has three compounding quality issues:

1. **Workspace leakage**: Qdrant fetches `limit * 3` without workspace filter, then post-filters — often returning fewer than `limit` relevant results, filled with cross-workspace noise. FTS SQL uses `IN (?, 'global')` which leaks all `project_hash = 'global'` docs into every query.
2. **Size bias**: Large markdown files (memory notes, long session summaries) accumulate more BM25 keyword hits and dominate RRF scores regardless of relevance.
3. **Temporal blindness**: Sessions from 2025 and 2026 on the same topic score identically. Superseded docs are demoted by only `0.3×` — still competitive.

## Goals / Non-Goals

**Goals:**
- Top-5 results come from the correct workspace in ≥90% of queries
- Recent sessions/memory notes rank above older ones on same-topic queries
- Explicitly superseded documents are effectively invisible (score < 0.05×)
- Agents can see `createdAt` in search results to reason about recency themselves
- No new external dependencies or services

**Non-Goals:**
- Automatic LLM-based contradiction detection (Artistry Approach 1/3 — deferred)
- VoyageAI reranker integration (separate concern)
- Cross-workspace search mode changes (scope=all still works as before)
- Changing the FTS5 schema or re-indexing existing text content

## Decisions

### D1: Push workspace filter into Qdrant payload (not post-filter)

**Decision**: Store `project_hash` as a Qdrant point payload field on every upsert. Pass a Qdrant filter `{ key: "project_hash", match: { value: projectHash } }` in every `search()` call.

**Rationale**: Post-filtering after `limit * 3` fetch wastes Qdrant capacity and consistently under-delivers results. Server-side filter is exact and scales with collection size. Migration: one-time backfill of `project_hash` payload from SQLite → Qdrant (batched, non-blocking).

**Alternative considered**: Keep post-filter but increase multiplier to `limit * 10`. Rejected — O(n) waste grows with index size.

### D2: Log-based length normalization after RRF

**Decision**: After RRF fusion, apply `score *= 1 / (1 + Math.log2(Math.max(1, charLength / 2000)))` where `2000` chars is the normalization anchor (roughly one A4 page of prose).

**Rationale**: BM25 is not length-normalized by default in SQLite FTS5 (no BM25F). A 20,000-char memory file accumulates ~10× more keyword hits than a 2,000-char session. Log scaling is gentler than linear — a 10× longer doc gets ~70% score reduction, not 90%.

**Alternative considered**: Re-index with BM25F parameters. Rejected — requires FTS schema rebuild and loses existing indexed content.

### D3: Time-decay recency boost (collections: sessions, memory only)

**Decision**: After length normalization, apply recency boost only to `collection IN ('sessions', 'memory')`:
```
recencyBoost = 1 / (1 + daysSince / halfLifeDays)
finalScore = score * (1 + recencyWeight * recencyBoost)
```
Default config: `recency_weight: 0.3`, `recency_half_life_days: 180`.

**Rationale**: Codebase files are updated in-place and should not decay. Session and memory docs represent evolving knowledge where recency matters. Half-life of 180 days means a 1-year-old doc retains ~37% recency bonus — gradual, not cliff-edge.

**Alternative considered**: Global recency across all collections. Rejected — source code from 2023 is still fully valid; decaying it would hide relevant implementations.

### D4: Supersede demotion 0.3 → 0.05

**Decision**: Reduce `supersede_demotion` multiplier from `0.3` to `0.05`. Fix bug in `memory.ts` where `supersedeDocument(targetDoc.id, 0)` passes `0` as new doc ID — should pass the inserting document's ID.

**Rationale**: A superseded doc scoring at `0.3×` of original is still often above threshold. At `0.05×` it effectively disappears from top-10 results. The bug means superseding chains are broken — new doc cannot be linked back from old doc.

### D5: Bayesian domain_type + last_reinforced_at (schema only, computation deferred)

**Decision**: Add `domain_type TEXT DEFAULT 'general'` and `last_reinforced_at TEXT` columns to the `documents` table via migration. Do NOT implement decay computation in this change — columns only.

**Rationale**: Artistry's Approach 2 (Bayesian decay by domain volatility) is the right long-term direction. Adding schema columns now avoids a future migration on a large table. Actual computation is a separate change once the reinforcement pass is built.

## Risks / Trade-offs

- **Qdrant backfill duration**: ~9500 points, batched at 100/batch ≈ 2-3 minutes. Server stays responsive during migration (read path unaffected).
- **Recency bias on old but valid sessions**: A session from 2025 documenting a stable architecture decision gets penalized. Mitigated by moderate `recency_weight: 0.3` — old docs still appear, just ranked lower.
- **`global` project_hash removal**: Some docs may be intentionally global (system-wide memory notes). Mitigation: introduce `scope: 'workspace' | 'global'` column in future; for now treat all docs as workspace-scoped.
- **Length normalization anchor**: 2000 chars is a reasonable anchor but may need tuning per deployment. Expose as `length_norm_anchor` config option.

## Migration Plan

1. Run SQLite migration: add `domain_type`, `last_reinforced_at` columns (safe, additive)
2. Deploy new server code
3. Server startup: detect missing `project_hash` payload in Qdrant → run backfill job async
4. Backfill reads `(id, project_hash)` from SQLite `documents` table, updates Qdrant points in batches of 100
5. Rollback: revert server code; Qdrant payload filter is additive — old code ignores payload

## Open Questions

- Should `scope=all` queries bypass workspace filter entirely, or still apply recency/length normalization? (Assumption: bypass workspace filter, keep scoring improvements)
- What is the correct `project_hash` for memory notes stored in `~/.nano-brain/memory/`? (Assumption: use `'global'` for memory, apply recency but not workspace filter)
