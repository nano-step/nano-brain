## Context

Tracking: #405

nano-brain indexes code via tree-sitter symbol extraction and builds a call graph (contains/imports/calls edges). Search uses hybrid BM25 + vector + RRF fusion with recency decay. The `internal/codesummarize/` package exists with manual-trigger LLM summarization (batching, budget, retry, provider) but is not wired to the watcher pipeline.

Research against Greptile, Mem0, Letta, Aider, and Bloop identified that nano-brain lacks:
1. LLM-generated behavioral summaries (Greptile's core differentiator — 12% better retrieval)
2. Entity-based search boosting (Mem0's multi-signal retrieval)
3. Importance-weighted ranking (Aider's PageRank on def/ref graph)

Current state: agents asking "when/how does X work?" get raw code chunks. Goal state: agents get natural language summaries with trigger context.

## Goals / Non-Goals

**Goals:**
- Agents can answer "when/how does X work?" by querying memory alone
- Auto-trigger summarization on file change (no manual API calls)
- Search relevance improves measurably (nDCG improvement gated per phase)
- Zero impact on watcher throughput (async processing only)
- Feature-flagged rollout with kill switch
- Budget-controlled LLM usage (daily caps, content-hash dedup)

**Non-Goals:**
- CFG/dataflow analysis (cut — scope creep, no clear use case for "when/how" queries)
- Real-time summarization (eventual consistency acceptable: 10-60s per file)
- Multi-LLM routing (single configured provider per instance)
- Custom per-language prompts (one universal prompt template)
- Interactive summary editing by users

## Decisions

### D1: 4-Phase Incremental Delivery

| Phase | Scope | Depends On |
|-------|-------|-----------|
| 1 | Auto-trigger summarization (NO flow context) | Nothing |
| 2 | Entity linking + post-RRF boost | Phase 1 (entities extracted from symbol chunks) |
| 3 | Flow-enriched summaries (caller context) | Phase 1 + graph edges |
| 4 | PageRank importance scoring | Graph edges |

Each phase gated on benchmark improvement. No phase ships without measurable quality gain.

**Rationale**: Metis analysis showed Phase 1+2 coupling is risky (flow context has unsolved fan-out). De-risk by proving basic summaries work first, then add complexity. Entity linking (Phase 2) is independent and easy to measure.

### D2: Async Worker Pool (Phase 1)

Watcher hook after `extractAndUpsertSymbols()` enqueues symbols to `summarization_queue` table. Background workers (2 goroutines) poll every 10s, batch 30 symbols per LLM call.

**Rationale**: LLM calls take 2-10s. Blocking watcher drops throughput 100x. Existing `embedQueue` pattern proves async works. Workers respect daily budget cap — when exhausted, pause until daily reset (not queue forever).

**Budget exhaustion behavior**: Workers pause until next daily reset. In-flight batches complete but no new dequeues. Queue items stay pending — processed next day. No unbounded growth because content-hash dedup prevents re-enqueue of unchanged symbols.

### D3: Summary Storage — Column in symbol_documents

```sql
ALTER TABLE symbol_documents ADD COLUMN summary TEXT;
ALTER TABLE symbol_documents ADD COLUMN summary_hash TEXT; -- content hash of source at summary time
```

**Rationale**: Simpler than separate table. symbol_documents already scopes per-symbol per-workspace. summary_hash enables staleness detection (if symbol content changed since last summary).

### D4: Search Injection — Pre-RRF (append to chunk content)

Summaries appended to chunk content before embedding. This means summaries improve BOTH BM25 (keyword match on summary text) and vector search (embedding captures summary semantics) without any query pipeline changes.

**Rationale**: Zero query changes for Phase 1. Summaries participate in existing RRF naturally. If tuning needed later, can add separate tsvector column.

### D5: Entity Linking — Post-RRF Reranker (Phase 2)

After RRF fusion produces ranked results, boost score for chunks matching query entities:
```
score += entity_match_count * entity_boost_factor (default 0.3)
```

Entity store: separate `chunk_entities` table (normalized many-to-many).

**Rationale**: Entity matching is binary (present/absent), not graduated. Adding as 3rd RRF signal would require tuning k parameter and could destabilize existing BM25/Vector balance. Post-retrieval boost is isolated, feature-flaggable, and follows existing recency-decay pattern.

**Entity definition**: Function names, type names, constant names. NOT variables (too noisy, too many false matches). Case-insensitive matching (normalize to lowercase).

### D6: Flow Context — Capped + Graph-Hash Invalidated (Phase 3)

- **Fan-out cap**: Top-10 callers by call-edge frequency. Max depth = 1 hop.
- **Token limit**: Flow context section capped at 1000 tokens per symbol prompt.
- **Invalidation**: Dual-hash approach:
  - `summary_hash` = hash of symbol's own source content (detect code changes)
  - `graph_context_hash` = hash of caller/callee list (detect topology changes)
  - Re-summarize when either hash differs from stored value
- **Cascade cap**: When a symbol changes, mark max 20 caller-summaries stale (sorted by PageRank importance if available, else by recency). Prevents unbounded cascade.
- **Queue overflow**: If summarization queue exceeds 1000 items, drop lowest-priority items (symbols with fewest references). Log dropped items for monitoring.

**Rationale**: Metis identified fan-out as HIGH risk. Without caps, a utility function with 500+ callers would explode prompt size and cascade invalidations. Fixed caps make behavior predictable.

### D7: PageRank — Pre-computed, Threshold-Triggered (Phase 4)

```sql
ALTER TABLE symbol_documents ADD COLUMN importance_score FLOAT NOT NULL DEFAULT 0.0;
```

Compute: iterative PageRank on graph_edges (damping=0.85, max_iter=100, tolerance=1e-6).
Trigger: after 100 edge updates OR daily cron (whichever comes first).
Cold start: if no edges exist, importance_score = 0.5 (neutral — no boost or penalty).
Search boost: `score *= (1 + importance_score * pagerank_weight)` where weight default = 0.2.

### D8: Benchmark Suite (Pre-requisite)

Before Phase 1 rollout, create:
- 50 query/expected-result pairs covering: symbol lookup, behavioral questions, entity-centric queries
- Automated nDCG@5 and Recall@10 measurement
- Regression gate: if nDCG drops >5% after enhancement, block deployment

Storage: `search_benchmarks` table or JSON fixture file checked into repo.

## Conflict Resolution Log

| Topic | Metis | Oracle | Resolution | Confidence |
|-------|-------|--------|------------|------------|
| MVA scope | Phase 1 only | Phase 1+2 | Phase 1 standalone. Phase 3 (flow) gated on success. | MED |
| Phase ordering | 1→3→2→4 (risk-first) | 1+2→3→4+5 (dependency) | 1→2(entity)→3(flow)→4(PR). Entities before flow because independent + measurable. | MED |
| Latency claim | "20min for 10k symbols" | "10-60s" | Both correct at different scales. Per-file = seconds. Full workspace = minutes-hours. Document clearly. | HIGH |
| CFG inclusion | Cut entirely | Hybrid Go SSA + tree-sitter | CUT. No use case for "when/how" queries. Defer indefinitely. | HIGH |
| Entity definition | Undefined | Not addressed | Function + type + constant names. NOT variables. Case-insensitive. | HIGH (autonomous) |
| Schema | Not addressed | Two options | Column in symbol_documents (simpler for Phase 1). | HIGH (autonomous) |
| Budget exhaustion | Unspecified | "Budget system handles" | Workers pause until daily reset. In-flight completes. Queue stays pending. | HIGH (autonomous) |

## Risks / Trade-offs

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| LLM cost explosion (large repos) | Medium | High | Daily budget cap. Content-hash dedup. Priority queue by importance. |
| Flow context fan-out (Phase 3) | High | Medium | Cap top-10 callers. 1000-token limit. Cascade cap = 20 symbols. |
| Search quality regression | Low | High | Benchmark suite. Feature flags. 2-week dual-path. Kill switch via config reload. |
| Prompt injection from code comments | Low | Medium | Output format validation (reject non-structured). Don't include raw comments in prompt. |
| Stale summaries after code change | Medium | Low | Dual-hash invalidation. Re-summarize on next watcher scan. Acceptable eventual consistency. |
| Queue overflow on initial indexing | Medium | Low | Queue cap = 1000. Drop low-priority. Priority by reference count. |
| LLM provider downtime | Low | Low | Workers retry 3x with backoff. Pause on persistent failure. Watcher unaffected (async). |

### Trade-offs Accepted

- **Eventual consistency**: Summaries appear 10-60s after file change (per-file). Full workspace reindex = minutes-hours. Acceptable for enhancement feature.
- **Approximate flow context**: Top-10 callers, not all callers. Misses rare call paths. Acceptable — covers 90% of behavioral questions.
- **No per-language prompts**: Universal prompt may produce lower quality for some languages. Acceptable for v1 — can specialize later.
- **Entity linking is coarse**: Function/type/constant names only. Misses semantic relationships. Acceptable — covers most agent queries.
