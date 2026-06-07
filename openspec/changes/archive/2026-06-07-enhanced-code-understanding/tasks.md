## Tasks

Tracking: #405

### Phase 0: Benchmark Suite (prerequisite)

- [ ] 0.1 Create `testdata/search_benchmarks.json` with 50+ query/expected-result pairs across 4 categories (symbol lookup, behavioral, entity-centric, conceptual)
- [ ] 0.2 Implement benchmark runner in `internal/search/benchmark_test.go` — reads fixtures, runs queries, computes nDCG@5, Recall@10, MRR
- [ ] 0.3 Record baseline metrics with current search (no enhancements) — store as reference
- [ ] 0.4 Add `make benchmark` target or test tag for easy execution

### Phase 1: Auto-trigger Summarization

- [ ] 1.1 Migration: `ALTER TABLE symbol_documents ADD COLUMN summary TEXT, ADD COLUMN summary_hash TEXT`
- [ ] 1.2 Implement watcher hook: after `extractAndUpsertSymbols()` in `processFile()`, call `summarizationQueue.EnqueueSymbols(ctx, symbols)` (non-blocking)
- [ ] 1.3 Implement `summarization_queue` logic — can reuse existing `code_summarization_failures` table or add `status` column to track pending/processing/done
- [ ] 1.4 Implement background worker pool in `internal/codesummarize/worker.go`: 2 goroutines, poll 10s, batch 30 symbols, call existing `Service.SummarizeBatch()`
- [ ] 1.5 Content-hash dedup: before enqueue, check if `symbol_documents.summary_hash` matches current symbol content hash — skip if unchanged
- [ ] 1.6 Budget exhaustion: when daily cap reached, workers pause (no dequeue). Resume on daily reset.
- [ ] 1.7 Rate limit handling: on HTTP 429, pause workers 60s with exponential backoff (60s, 120s, 240s, max 900s)
- [ ] 1.8 After summary stored, update chunk content to include summary text → re-enqueue chunk for embedding
- [ ] 1.9 Wire workers into server startup: `go csSvc.StartWorkerPool(ctx, cfg.CodeSummarization.Workers)` guarded by `cfg.CodeSummarization.Enabled`
- [ ] 1.10 Config hot-reload: start/stop workers on `enabled` toggle without restart
- [ ] 1.11 Integration test: index a single file with 1 symbol (controlled queue size) → verify summary appears within 60s → verify `memory_query` returns summary in results
- [ ] 1.12 Run benchmark suite → verify no regression (nDCG@5 must not drop >5%)

### Phase 2: Entity Linking

- [ ] 2.1 Migration: `CREATE TABLE chunk_entities` with schema from spec (chunk_id, entity_name, entity_type, workspace_hash, PK, indexes)
- [ ] 2.2 Implement entity extraction in `internal/chunker/` or `internal/symbol/`: for each symbol chunk, extract referenced function/type/constant names from AST
- [ ] 2.3 sqlc queries: `InsertChunkEntities`, `GetEntitiesForChunks`, `GetChunksByEntityName`
- [ ] 2.4 Wire extraction into watcher pipeline: after chunk upsert, extract and insert entities
- [ ] 2.5 Implement query entity extraction: tokenize query, remove stopwords, lookup each token against `chunk_entities.entity_name` for workspace (case-insensitive), use matches as boost set
- [ ] 2.6 Implement post-RRF entity boost in `internal/search/service.go`: after RRF fusion, boost matching chunks by `entity_boost_factor` (configurable, default 0.3)
- [ ] 2.7 Config: add `search.entity_boosting_enabled` and `search.entity_boost_factor` to config struct
- [ ] 2.8 Feature flag: skip boost step entirely when disabled
- [ ] 2.9 Integration test: index repo → query with entity name → verify entity-matching chunks rank higher
- [ ] 2.10 Run benchmark suite → verify nDCG@5 improvement ≥3%

### Phase 3: Flow-Enriched Summaries

- [ ] 3.1 Migration: `ALTER TABLE symbol_documents ADD COLUMN graph_context_hash TEXT`
- [ ] 3.2 Implement `GetCallerContext(ctx, symbolName, workspaceHash, limit=10)` in `internal/graph/service.go` — bulk query callers/callees with frequency count
- [ ] 3.3 Modify `internal/codesummarize/prompt.go`: add "TRIGGERED BY" and "CALLS" sections when graph context available. Cap flow section at 1000 tokens.
- [ ] 3.4 Implement dual-hash invalidation: compute `graph_context_hash` (SHA-256 of sorted caller+callee names), compare with stored, re-summarize on mismatch
- [ ] 3.5 Implement cascade cap: when symbol changes, mark max 20 caller-summaries stale (sort by importance_score DESC, then created_at DESC)
- [ ] 3.6 Implement queue overflow protection: if queue > 1000 items, drop lowest-priority (fewest graph references). Log dropped items.
- [ ] 3.7 Integration test: index repo with graph edges → verify summary includes caller context → verify behavioral query returns flow-aware answer
- [ ] 3.8 Run benchmark suite → verify behavioral query category improves ≥5%

### Phase 4: PageRank Importance

- [ ] 4.1 Migration: `ALTER TABLE symbol_documents ADD COLUMN importance_score FLOAT NOT NULL DEFAULT 0.0`
- [ ] 4.2 Implement PageRank in `internal/graph/pagerank.go`: iterative algorithm (damping=0.85, max_iter=100, tol=1e-6) operating on graph_edges
- [ ] 4.3 Implement trigger: track edge update count per workspace. When count ≥ 100 since last compute, trigger recomputation.
- [ ] 4.4 Implement daily cron fallback: if no threshold trigger in 24h, recompute anyway
- [ ] 4.5 Cold start: if workspace has 0 edges, set all importance_score = 0.5
- [ ] 4.6 Implement search boost in `internal/search/service.go`: after entity boost (if any), apply `score *= (1 + importance_score * pagerank_weight)`
- [ ] 4.7 Config: add `search.pagerank_enabled` and `search.pagerank_weight` (default 0.2)
- [ ] 4.8 HTTP endpoint: `POST /api/v1/graph/pagerank/compute` for manual trigger
- [ ] 4.9 Integration test: index repo → compute PageRank → verify highly-referenced symbols rank higher
- [ ] 4.10 Run benchmark suite → verify overall nDCG@5 improvement

### Phase 5: Rollout & Validation

- [ ] 5.1 Enable feature flags one at a time in staging config
- [ ] 5.2 Run full benchmark suite with all enhancements enabled simultaneously
- [ ] 5.3 2-week dual-path observation (log both baseline and enhanced results)
- [ ] 5.4 If quality gate passes after 2 weeks: remove dual-path code, commit to enhanced path
- [ ] 5.5 Update user-facing docs: explain new search capabilities
- [ ] 5.6 User-flow test: demonstrate agent answering "when/how" questions from memory alone
- [ ] 5.7 Archive OpenSpec change
