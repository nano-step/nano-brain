## 1. BM25 AND→OR Fallback

- [ ] 1.1 Add `bm25SearchWithOR` helper in `internal/search/service.go` — constructs OR-based tsquery
- [ ] 1.2 In `hybridSearchInner`, after BM25 returns 0, retry with OR if query has >2 words
- [ ] 1.3 Add unit tests for OR fallback behavior
- [ ] 1.4 Verify existing BM25 tests still pass (no regression)

## 2. Incoming Edges Symbol Fallback

- [ ] 2.1 Update `GetIncomingEdges` SQL in `internal/storage/queries/graph.sql` to add `split_part` fallback
- [ ] 2.2 Regenerate sqlc code
- [ ] 2.3 Add unit test for incoming edges with symbol-only match
- [ ] 2.4 Verify `memory_graph(direction=in)` returns callers for known functions

## 3. Graph Extraction for Unchanged Files

- [ ] 3.1 Add `forceGraphEdges` flag to `processFile` in `internal/watcher/watcher.go`
- [ ] 3.2 When `forceGraphEdges=true`, skip content-hash check for graph extraction only
- [ ] 3.3 Call `processFile` with `forceGraphEdges=true` in `processAll` for files that haven't had graph edges extracted
- [ ] 3.4 Add check: if file already has graph edges, skip re-extraction
- [ ] 3.5 Verify `memory_graph` returns edges for `HybridSearch`, `BuildFlow`, etc.

## 4. Verification

- [ ] 4.1 Run full test suite: `go build ./... && go test -race -short ./...`
- [ ] 4.2 Run benchmark against host server (3100) — confirm P@5 improvement
- [ ] 4.3 Run code intel benchmark — confirm callers accuracy improvement
- [ ] 4.4 Create PR
