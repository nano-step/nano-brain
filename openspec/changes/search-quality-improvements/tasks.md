## Tasks

### Phase 1: Quick Wins

- [ ] 1.1 Create `internal/search/preprocess/` package with Preprocessor interface and LLM-based implementation
- [ ] 1.2 Implement query intent detection (keyword/conceptual/temporal) via single LLM call
- [ ] 1.3 Integrate Preprocessor into `HybridSearch()` in `internal/search/service.go`
- [ ] 1.4 Add config fields: `search.query_preprocessing.*` to config struct + defaults
- [ ] 1.5 Write migration 00015: parameterize BM25 tsvector language via `get_tsvector_config()` function
- [ ] 1.6 Update search SQL queries to use configurable language
- [ ] 1.7 Increase default chunk overlap: 200 → 600 bytes, add `watcher.chunk_overlap` config
- [ ] 1.8 Default `group_by=document` in MCP search tools when not specified
- [ ] 1.9 Unit tests for preprocessor (mock LLM, verify JSON parsing + fallback)
- [ ] 1.10 Integration tests for full pipeline with preprocessing enabled
- [ ] 1.11 Update README with query preprocessing config section

### Phase 2: Advanced Retrieval

- [ ] 2.1 Implement HyDE in `internal/search/preprocess/hyde.go` (conditional, gated on intent)
- [ ] 2.2 Research + integrate Go-native reranker (ONNX purego or LLM-based fallback)
- [ ] 2.3 Implement dynamic RRF weights based on query intent
- [ ] 2.4 Integration tests for HyDE + reranking
- [ ] 2.5 Benchmark suite: 50 curated queries, measure precision@10 before/after

### Infrastructure

- [ ] 0.1 Fix harness-check.sh gate 1.2 bug (openspec list "active" false positive) — DONE
- [ ] 0.2 Commit harness-check fix to feature branch
