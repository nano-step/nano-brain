## Context

Benchmark reveals nano-brain loses to LlamaIndex on zengamingx (P@5=0.53 vs 0.55). Investigation found 3 root causes — all fixable without changing architecture.

## Decisions

### Fix 1: BM25 AND→OR Fallback

**Problem**: `websearch_to_tsquery` uses AND for unquoted terms. "Explain the order lifecycle" → `'explain' & 'order' & 'lifecycle'` → 0 results if any word missing from index.

**Choice**: Add OR fallback — when BM25 with AND returns 0, retry with OR semantics using `to_tsquery('explain | order | lifecycle')`.

**Rationale**: Minimal change, no false positives (OR just broadens recall), existing BM25 behavior preserved as default.

**Implementation**: Modify `HybridSearch` in `service.go`. After BM25 leg completes, if `total == 0` AND query has >2 words, retry with OR. Add OR-based BM25 helper method.

### Fix 2: Incoming Edges Symbol Fallback

**Problem**: `GetIncomingEdges` does `WHERE target_node = $2`. Call edges store `target_node = "BM25Search"` but queries use `"/path/file.go::BM25Search"`.

**Choice**: Add symbol-only fallback in `GetIncomingEdges` SQL:
```sql
AND (target_node = $2 OR split_part(target_node, '::', 2) = $2)
```

**Rationale**: Matches existing pattern in `GetOutgoingEdgesBySymbol`. Low risk, high impact.

### Fix 3: Graph Extraction for Unchanged Files

**Problem**: `processFile` returns early when content hash matches DB. Graph extraction only runs when file content changes.

**Choice**: Add `extractGraphEdges` flag to `processFile`. When `forceGraphEdges=true`, skip content-hash check for graph extraction only (still skip document re-indexing).

**Rationale**: Graph extraction is cheap (tree-sitter parse only). Running it on unchanged files has negligible performance impact.

## Risks

- **Fix 1**: OR fallback may return less relevant results for some queries. Mitigation: test with benchmark to confirm P@5 improves.
- **Fix 2**: Symbol-only fallback may match wrong symbols (e.g., multiple `init` functions). Mitigation: acceptable for v1 — same risk as `GetOutgoingEdgesBySymbol`.
- **Fix 3**: Running graph extraction on all files at startup may add startup latency. Mitigation: graph extraction is CPU-bound but parallelizable, ~50ms per file.
