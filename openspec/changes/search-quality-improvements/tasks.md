## Tasks

### Phase 1: Quick Wins

- [x] 1.1 Create `internal/search/preprocess/` package with Preprocessor interface and LLM-based implementation
- [x] 1.2 Implement query intent detection (keyword/conceptual/temporal) via single LLM call
- [x] 1.3 Integrate Preprocessor into `HybridSearch()` in `internal/search/service.go`
- [x] 1.4 Add config fields: `search.query_preprocessing.*` to config struct + defaults
- [x] 1.5 Write migration 00023: parameterize BM25 tsvector language via `get_tsvector_config()` function
- [x] 1.6 Update search SQL queries to use configurable language
- [x] 1.7 Increase default chunk overlap: 200 → 600 bytes, add `watcher.chunk_overlap` config
- [x] 1.8 Default `group_by=document` in MCP search tools when not specified
- [x] 1.9 Unit tests for preprocessor (mock LLM, verify JSON parsing + fallback)
- [x] 1.10 Integration tests for full pipeline with preprocessing enabled
- [x] 1.11 Update README with query preprocessing config section

### Phase 2: Advanced Retrieval (Future PR)

- [ ] 2.1 Implement HyDE in `internal/search/preprocess/hyde.go` (conditional, gated on intent)
- [ ] 2.2 Research + integrate Go-native reranker (ONNX purego or LLM-based fallback)
- [ ] 2.3 Implement dynamic RRF weights based on query intent
- [ ] 2.4 Integration tests for HyDE + reranking
- [ ] 2.5 Benchmark suite: 50 curated queries, measure precision@10 before/after

### Infrastructure

- [x] 0.1 Fix harness-check.sh gate 1.2 bug (openspec list "active" false positive)
- [x] 0.2 Commit harness-check fix to feature branch
- [x] 0.3 Fix pre-existing lint issues (unused field/func, errcheck)
- [x] 0.4 Fix pre-existing integration tests (TestBackfill_DryRun, TestGitLog*)
- [x] 0.5 Re-run smoke-ui.sh (stale evidence log)
