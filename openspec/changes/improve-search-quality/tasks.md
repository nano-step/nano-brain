## 1. Result Deduplication

- [x] 1.1 Create `internal/search/dedup.go` with deduplication logic
- [x] 1.2 Implement content hashing (SHA-256) for duplicate detection
- [x] 1.3 Add path normalization (case-insensitive, slash normalization, `.agent/` vs `.agents/`)
- [x] 1.4 Integrate deduplication into `HybridSearch` after RRF merge (in `service.go`)
- [ ] 1.5 Add `deduplicated_from` field to search response (deferred — low-value API surface change)

## 2. Context-Aware Snippets

- [x] 2.1 Implement query-relevant snippet extraction (centering window around first lexical match)
- [x] 2.2 Handle vector-only results with graceful head-truncation fallback
- [x] 2.3 Preserve UTF-8 boundaries and add ellipsis indicators for truncated windows
- [x] 2.4 Handle empty/short content and zero-length queries correctly
- [x] 2.5 Update `handlers/search.go` to use `Snippet` field with `ExtractRelevantSnippet`

## 3. Code-Aware Ranking

- [x] 3.1 Add post-RRF boost for results where query tokens appear in file path (1.2x)
- [x] 3.2 Add post-RRF boost for results where query tokens appear in title/symbol name (1.3x)
- [x] 3.3 Add boost for code file extensions (`.go`, `.js`, `.ts`, `.py` → 1.1x) over docs (`.md` → 0.9x)
- [x] 3.4 Integrate boosts into `HybridSearch` after RRF merge and before recency boost

## 4. Testing and Benchmarking

- [x] 4.1 Write unit tests for deduplication logic (`dedup_test.go`)
- [x] 4.2 Write unit tests for snippet generation (`snippet_test.go`) and ranking (`ranking_test.go`)
- [ ] 4.3 Run integration tests with real zengamingx workspace
- [ ] 4.4 Benchmark search performance (ensure P50 < 10ms)
- [ ] 4.5 Build meaningful benchmark with file-path ground truth (not generic keywords)
