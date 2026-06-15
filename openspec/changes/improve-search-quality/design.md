## Context

nano-brain is a persistent memory server for AI agents that indexes code, documentation, and session data. The search pipeline combines BM25 + vector search with RRF fusion. Current issues discovered during real-world testing:

**What already works:**
- Symbol extraction for Go, JS/TS, Python via tree-sitter (`internal/symbol/`, `internal/chunker/symbol.go`)
- Hybrid search (BM25 + vector + RRF + recency + entity boost + PageRank + reranking)
- 4 workspaces indexed (93,951 chunks with embeddings)

**Actual gaps discovered:**
- No deduplication: same file appears multiple times from different search legs
- Snippets use fixed 700-char truncation (`search.go:40: maxSnippetLen = 700`) without code structure awareness
- Search doesn't prioritize code files over documentation when query matches symbol names
- Handler uses `Content` field for snippets instead of `Snippet` field

**Current state:**
- 4 workspaces indexed: nano-brain (2,161 docs), capyhome (24,482), zengamingx (17,764), oh-my-harness-loop (1,218)
- Total: 93,951 chunks with embeddings
- Search latency: P50=3.2ms, P95=5.8ms (post-embedding-fix)

## Goals / Non-Goals

**Goals:**
- Deduplicate search results from files with similar content or paths
- Generate context-aware snippets based on code structure (function boundaries)
- Prioritize code files over documentation when query matches symbol names
- Maintain search performance (P50 < 10ms)

**Non-Goals:**
- Adding new language support (JS/TS/Python extraction already works)
- Changing the embedding model or re-embedding
- Real-time symbol updates (batch processing is sufficient)

## Decisions

### Decision 1: Deduplication via content hashing + path normalization

**Choice:** Hash chunk content and normalize paths before dedup

**Alternatives considered:**
- **Path-only dedup** — Fails when files are copied to different directories
- **Fuzzy matching** — Too expensive for large codebases
- **Manual exclusion rules** — Doesn't scale

**Rationale:** Content hashing is O(n) and catches exact duplicates. Path normalization (lowercase, slash normalization, `.agent/` vs `.agents/`) catches near-duplicates from directory name variations.

### Decision 2: Context-aware snippets using AST metadata

**Choice:** Use existing AST metadata from `chunker.Chunk` (StartLine, EndLine, SymbolName, SymbolKind) to extract meaningful snippets

**Alternatives considered:**
- **Fixed-size windows** — Current approach, loses context
- **Re-parsing AST at query time** — Too expensive
- **LLM-based summarization** — Too slow and expensive

**Rationale:** AST metadata is already stored in chunks. We can extract the full symbol body + surrounding comments instead of truncating at 700 chars.

### Decision 3: Code-aware ranking boost

**Choice:** Add a post-RRF boost factor for results where query tokens appear in file path or symbol name

**Alternatives considered:**
- **Pre-filtering** — Removes too many results
- **Separate code/docs search** — Adds complexity, loses cross-domain results
- **Collection-based filtering** — Doesn't work when code and docs are in same collection

**Rationale:** A lightweight post-RRF boost (1.2x for path match, 1.5x for symbol name match) preserves all results while surfacing code files higher.

## Risks / Trade-offs

**Risk 1: Dedup may remove valid distinct chunks**
- Two different functions in same file could have similar content
- **Mitigation:** Dedup by document_id + content hash, not just content alone

**Risk 2: Snippet extraction may miss context**
- AST boundaries may not include relevant comments or imports
- **Mitigation:** Include 2 lines of context before function start

**Risk 3: Code boost may over-prioritize**
- Query "error handling" should find docs AND code
- **Mitigation:** Boost is additive (1.2x-1.5x), not multiplicative; docs still appear
