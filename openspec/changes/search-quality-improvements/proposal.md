## Why

Tracking: #407

nano-brain's natural language search returns scattered/irrelevant results for behavioral and conceptual queries. Root causes identified through research:

1. **Zero query preprocessing** — queries pass straight to BM25/vector with no normalization, translation, or expansion
2. **BM25 hardcoded to English** — `to_tsvector('english', ...)` tokenizes non-English queries incorrectly
3. **Insufficient chunk overlap** — 200 bytes (6.7%) loses cross-chunk semantic context
4. **No reranking** — RRF fusion output not refined by relevance model
5. **No query intent detection** — keyword vs conceptual vs temporal queries treated identically

Expected improvement: +25-40% retrieval quality across search tools.

## What

### Phase 1: Query Preprocessing (Quick Wins)

1. **LLM Query Preprocessor** — Before search, optionally call LLM to:
   - Translate non-English queries to English
   - Expand query with synonyms/related terms
   - Detect temporal intent and extract time filters
   - Gate: only activate when `search.query_preprocessing.enabled: true` in config

2. **BM25 Language Configuration** — Make tsvector language configurable:
   - Per-workspace config option: `search.bm25_language` (default: `'english'`)
   - Option to use `'simple'` for language-agnostic tokenization
   - Requires migration to update trigger function

3. **Chunk Overlap Increase** — Change defaults:
   - `DefaultOverlap`: 200 → 600 bytes
   - Make configurable via `watcher.chunk_overlap` config key

4. **Document-Level Dedup Default** — MCP search tools default to `group_by=document` when not specified

### Phase 2: Advanced Retrieval

5. **HyDE (Hypothetical Document Embedding)** — Conditional:
   - Classify query as factual/conceptual
   - For conceptual queries: generate hypothetical answer via LLM, embed that instead
   - Gate on config: `search.hyde.enabled: true`
   - Latency budget: +150-300ms (only when triggered)

6. **Cross-Encoder Reranking** — After RRF fusion:
   - Rerank top-K results using cross-encoder model
   - Go-native via ONNX purego or external reranker API
   - Gate on config: `search.reranking.enabled: true`
   - Latency budget: +50-100ms

7. **Dynamic RRF Weights** — Based on query intent:
   - Keyword query → boost BM25 weight
   - Conceptual query → boost vector weight
   - Temporal query → apply time filter + boost recency

## How

### Architecture

```
User Query
    │
    ▼
┌─────────────────────────┐
│ Query Preprocessor (LLM)│  ← NEW (Phase 1)
│ - Language detection     │
│ - Translation            │
│ - Expansion              │
│ - Intent classification  │
└───────────┬─────────────┘
            │ preprocessed query + metadata
            ▼
┌─────────────────────────┐
│ HybridSearch (existing) │
│ - BM25 (configurable)   │  ← MODIFIED (language param)
│ - Vector (unchanged)    │
│ - RRF Fusion            │  ← MODIFIED (dynamic weights, Phase 2)
│ - Recency Decay         │
└───────────┬─────────────┘
            │ top-K candidates
            ▼
┌─────────────────────────┐
│ Reranker (cross-encoder)│  ← NEW (Phase 2)
└───────────┬─────────────┘
            │ reranked results
            ▼
┌─────────────────────────┐
│ Document Dedup + Format │  ← MODIFIED (default group_by)
└───────────┬─────────────┘
            ▼
        Response
```

### New Package

`internal/search/preprocess/` — Query preprocessing:
- `preprocessor.go` — Orchestrator (calls LLM, returns enhanced query)
- `intent.go` — Query intent classification (keyword/conceptual/temporal)
- `translate.go` — Language detection + translation
- `expand.go` — Query expansion (synonyms, related terms)

### Config Additions

```yaml
search:
  bm25_language: "english"          # or "simple" for language-agnostic
  query_preprocessing:
    enabled: false                   # opt-in
    provider_url: ""                 # reuse summarization provider
    api_key: ""                      # or NANO_BRAIN_SEARCH_PREPROCESS_API_KEY
    model: ""                        # reuse summarization model
    max_latency_ms: 500             # timeout for preprocessing
  hyde:
    enabled: false                   # opt-in, Phase 2
    confidence_threshold: 0.65       # only trigger below this
  reranking:
    enabled: false                   # opt-in, Phase 2
    model: ""                        # cross-encoder model path or API
    top_k: 20                        # rerank top-K candidates

watcher:
  chunk_overlap: 600                 # increased from 200
```

## Constraints

- Must remain CGO_ENABLED=0 (no C dependencies)
- Single binary distribution (no Python sidecar)
- All new features opt-in via config (zero breaking changes)
- LLM preprocessing reuses existing provider infrastructure (summarization)
- Latency budget: total query time < 2s even with all features enabled
- Backward compatible: existing behavior unchanged when features disabled

## Risks

| Risk | Mitigation |
|------|-----------|
| LLM preprocessing adds latency | Config timeout + bypass on timeout |
| LLM hallucination in query expansion | Keep original query as fallback, RRF-fuse both |
| Reranker model not available in Go | Fallback: skip reranking, use RRF-only |
| BM25 language change requires migration | Goose migration, backward compatible |
| Chunk overlap increase triggers re-embed | Only new chunks affected; existing unchanged |
