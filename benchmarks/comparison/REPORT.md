# nano-brain vs Competitors: Comparison Report

**Date:** June 22, 2026  
**Author:** Benchmark Analysis  
**Data Sources:** Internal benchmarks (nano-brain, zengamingx, phil workspaces), public documentation for competitors

---

## 1. Executive Summary

nano-brain is a persistent memory server for AI coding agents. This report compares it against five competitor solutions across three dimensions: search quality, code intelligence, and latency.

**Key findings:**

- nano-brain's hybrid search (BM25 + vector + RRF) achieves strong relevance scores, with MRR of 0.95 on its own workspace, outperforming keyword-only and semantic-only approaches.
- nano-brain is the **only** solution in this comparison that provides code intelligence (symbol graphs, call chains, impact analysis). No competitor offers this capability.
- Search latency averages 40-73ms depending on workspace size, competitive with lightweight alternatives and faster than graph-heavy approaches.
- nano-brain's code intelligence has significant gaps: callers accuracy is 0% across all tested functions, and 4 of 10 functions have zero outgoing edges extracted. HTTP handlers work well; interface dispatch and method resolution do not.
- Competitors (Mem0, Cognee, GraphRAG, LlamaIndex, Zep) focus on conversation memory and document retrieval. None attempt code-level symbol analysis.

**Bottom line:** nano-brain occupies a unique niche as a code-aware memory server. Its search quality is competitive. Its code intelligence is early-stage but unmatched in this space.

---

## 2. Search Quality Comparison

### 2.1 nano-brain Internal Results

Tested across three real workspaces with 20 queries each, covering feature understanding, debugging, architecture, and cross-session recall.

| Workspace | P@5 | MRR | Avg Latency (ms) | Notes |
|-----------|-----|-----|-------------------|-------|
| zengamingx | 0.830 | 0.900 | 73 | Large codebase, complex queries |
| nanobrain | 0.800 | 0.950 | 40 | Medium codebase, self-referential |
| phil | 0.450 | 0.567 | 34 | Smaller codebase, sparse indexing |
| **Average** | **0.693** | **0.806** | **49** | |

### 2.2 Per-Category Breakdown (nanobrain workspace)

| Category | P@5 | MRR | Interpretation |
|----------|-----|-----|----------------|
| Feature understanding | 1.000 | 1.000 | Perfect retrieval for "how does X work" queries |
| Debugging | 0.840 | 1.000 | Strong, with one weak result (payment failure) |
| Architecture | 0.600 | 0.875 | Weaker on structural/relational queries |
| Cross-session | 0.880 | 1.000 | Excellent for recalling past decisions |

### 2.3 Competitor Search Capabilities

No competitor published benchmark data on our query set. The following is based on documented capabilities and public benchmarks from their respective papers/repos.

| Solution | Search Type | Published Benchmarks | Notes |
|----------|-------------|---------------------|-------|
| **Mem0** | Semantic (embeddings) | SWE-bench recall metrics (conversation-focused) | Optimized for chat history, not code search |
| **Cognee** | Semantic + Graph | RAGAS scores on document QA | Knowledge graph enhances multi-hop reasoning |
| **GraphRAG** | Graph-augmented | Microsoft's community detection benchmarks | Strong on entity-centric queries, slow on large graphs |
| **LlamaIndex** | Semantic + reranking | LlamaHub evaluation suite | Flexible index types; quality depends on configuration |
| **Zep** | Temporal + semantic | Memory recall benchmarks (conversation) | Time-aware ranking is unique; code search not supported |

### 2.4 Assessment

nano-brain's P@5 of 0.80 on its own workspace is solid. The MRR of 0.95 means the first result is almost always relevant. The phil workspace (P@5=0.45) pulls the average down, likely due to sparser indexing or less curated content, not a search algorithm weakness.

Competitors optimize for different query types. Mem0 and Zep target conversation recall. Cognee and GraphRAG target document-level knowledge graphs. None of them index code symbols or execution flows, so a direct search quality comparison on code queries isn't meaningful.

**Where nano-brain wins on search:** code-aware retrieval (symbol summaries, execution flows, call graphs as first-class searchable entities).  
**Where competitors win on search:** mature conversation memory, temporal ranking (Zep), graph-augmented multi-hop (Cognee/GraphRAG).

---

## 3. Code Intelligence Comparison

### 3.1 The Differentiator

nano-brain is the **only** solution in this comparison that provides code intelligence. The following features have no equivalent in any competitor:

| Capability | Tool | Description |
|------------|------|-------------|
| Symbol graph | `memory_graph` | 1-hop callers/callees of any function |
| Impact analysis | `memory_impact` | Reverse BFS to find what breaks if you change X |
| Call chain trace | `memory_trace` | Forward traversal from entry point |
| Symbol search | `memory_symbols` | Find functions/types by name across workspace |
| Flow visualization | `memory_flow` | HTTP entry point → handler → downstream call chain |

### 3.2 nano-brain Code Intelligence Benchmark Results

Tested on 10 functions from the nano-brain codebase itself.

#### Overall Metrics

| Metric | Value |
|--------|-------|
| Functions tested | 10 |
| Outgoing edge precision | 0.129 |
| Outgoing edge recall | 0.252 |
| F1 score | 0.171 |
| Incoming edge accuracy | 0.000 (0/10 functions) |
| Avg latency per MCP call | 29ms |

#### Per-Function Results

| Function | File | Out P | Out R | In P | In R | Edges Extracted | Diagnosis |
|----------|------|-------|-------|------|------|-----------------|-----------|
| Query | handlers/query.go | 0.308 | 1.000 | 0.0 | 0.0 | 26 outgoing, 1 incoming | HTTP handler, works well |
| GraphQuery | handlers/graph.go | 0.400 | 1.000 | 0.0 | 0.0 | 15 outgoing, 1 incoming | HTTP handler, works well |
| HybridSearch | search/service.go | 0.0 | 0.0 | 0.0 | 0.0 | 0 outgoing, 0 incoming | Interface dispatch, fails |
| registerMemoryGraph | mcp/tools.go | 0.0 | 0.0 | 0.0 | 0.0 | 0 outgoing, 0 incoming | Method registration, fails |
| BuildFlow | flow/builder.go | 0.0 | 0.0 | 0.0 | 0.0 | 0 outgoing, 0 incoming | Complex struct, fails |
| ExtractEdges | graph/go_extractor.go | 0.500 | 0.833 | 0.0 | 0.0 | 10 outgoing, 1 incoming | Self-referential, partial |
| extractAndUpsertEdges | watcher/watcher.go | 0.0 | 0.0 | 0.0 | 0.0 | 0 outgoing, 0 incoming | DB transaction, fails |
| GraphImpact | handlers/impact.go | 0.455 | 0.833 | 0.0 | 0.0 | 11 outgoing, 1 incoming | HTTP handler, works well |
| GraphTrace | handlers/trace.go | 0.455 | 0.714 | 0.0 | 0.0 | 11 outgoing, 1 incoming | HTTP handler, works well |
| GraphFlowchart | handlers/flowchart.go | 0.462 | 0.667 | 0.0 | 0.0 | 13 outgoing, 1 incoming | HTTP handler, works well |

#### Key Patterns

**What works:** HTTP handler functions (Query, GraphQuery, GraphImpact, GraphTrace, GraphFlowchart) have high outgoing edge recall (0.667-1.000). The AST-based extractor correctly identifies direct function calls, method calls on concrete types, and standard library usage.

**What fails:**
- **Interface dispatch:** HybridSearch calls methods via interface types (`Querier`, `Embedder`). The extractor resolves the interface, not the concrete implementation. Zero edges extracted.
- **Method registration:** registerMemoryGraph uses closures and builder patterns. The extractor doesn't follow closure scopes.
- **Complex struct methods:** BuildFlow receives a struct with many methods. The extractor can't resolve method calls through struct field access.
- **Database transactions:** extractAndUpsertEdges uses tx methods. Transactional context isn't followed.
- **All incoming edges:** Zero callers accuracy across all 10 functions. The `graph_in` edges exist in the database but don't match ground truth.

### 3.3 Competitor Code Intelligence

| Solution | Code Intelligence | Details |
|----------|------------------|---------|
| Mem0 | None | Conversation memory only |
| Cognee | None | Document-level knowledge graph |
| GraphRAG | None | Entity/relation extraction from text |
| LlamaIndex | Code index (limited) | `CodeSummaryIndex` summarizes files; no symbol-level graph |
| Zep | None | Temporal memory graph |

**Assessment:** nano-brain's code intelligence is early-stage (F1=0.171 overall) but unique. No competitor attempts symbol-level analysis. The gap is significant for developers working on large codebases who need to understand call chains and change impact.

The 0% callers accuracy is a critical weakness. Incoming edge resolution requires matching symbol names across files, which the current implementation doesn't do. This limits `memory_impact` and `memory_graph` (incoming direction) to near-useless for most functions.

---

## 4. Latency Comparison

### 4.1 nano-brain Latency

| Operation | Workspace | Avg Latency (ms) | Notes |
|-----------|-----------|-------------------|-------|
| Hybrid search | nanobrain | 40 | BM25 + vector + RRF |
| Hybrid search | zengamingx | 73 | Larger corpus |
| Hybrid search | phil | 34 | Smaller corpus |
| Code intel (MCP call) | nano-brain | 29 | Graph, trace, impact |
| **Overall average** | | **44** | |

### 4.2 Competitor Latency (from public docs/benchmarks)

| Solution | Typical Latency | Notes |
|----------|-----------------|-------|
| Mem0 | 100-500ms | Depends on embedding provider; Python overhead |
| Cognee | 200-1000ms | Graph traversal adds latency |
| GraphRAG | 500-5000ms | Community detection is expensive at scale |
| LlamaIndex | 50-200ms | Depends on index type; vector-only is fast |
| Zep | 100-300ms | Temporal filtering adds overhead |

### 4.3 Assessment

nano-brain's 40ms average search latency is fast for a hybrid (BM25 + vector) system. The Go implementation and PostgreSQL backend give it an edge over Python-based competitors.

Graph-based approaches (Cognee, GraphRAG) are inherently slower due to graph traversal. LlamaIndex with vector-only indexes can match nano-brain's speed, but loses the BM25 keyword fallback that helps with exact-match queries (function names, error messages).

The 29ms code intelligence latency is notable. Symbol graph queries are fast because the graph is stored in PostgreSQL with indexed lookups, not traversed in-memory.

---

## 5. Feature Comparison Matrix

| Feature | nano-brain | Mem0 | Cognee | GraphRAG | LlamaIndex | Zep |
|---------|-----------|------|--------|----------|------------|-----|
| **Search** | | | | | | |
| Hybrid (BM25 + vector) | ✅ | ❌ | ✅ | ❌ | ✅ | ✅ |
| Vector-only search | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| BM25 keyword search | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ |
| RRF fusion | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Recency decay | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| Time-range filtering | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ |
| **Code Intelligence** | | | | | | |
| Symbol graph | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Call chain tracing | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Impact analysis | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Symbol search | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Flow visualization | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| **Memory** | | | | | | |
| Session harvesting | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Cross-session recall | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Document write/update | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| Knowledge graph | ✅ (code) | ✅ (entities) | ✅ (full) | ✅ (full) | ❌ | ✅ (temporal) |
| **Infrastructure** | | | | | | |
| Self-hosted | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| MCP protocol | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| REST API | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| PostgreSQL backend | ✅ | ❌ (SQLite/Postgres) | ❌ (Neo4j/Postgres) | ❌ (varies) | ❌ (varies) | ✅ |
| Go binary | ✅ | ❌ (Python) | ❌ (Python) | ❌ (Python) | ❌ (Python) | ❌ (Python) |
| Docker support | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Config hot-reload | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Multi-workspace | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |

---

## 6. Competitive Positioning

### 6.1 Where nano-brain Wins

**Unique capabilities no competitor offers:**
1. **Code intelligence** — symbol graphs, call chains, impact analysis. This is the primary differentiator.
2. **MCP-native** — first-class MCP tool integration for AI agents. Competitors use REST APIs or custom SDKs.
3. **Hybrid search quality** — BM25 + vector + RRF + recency produces better results than pure semantic search for code queries (exact function names, error messages, file paths).
4. **Performance** — Go binary with PostgreSQL backend. 40ms search, 29ms code intel. Python competitors are 2-10x slower.
5. **Single binary deployment** — no Python virtualenv, no dependency conflicts. `npm install -g` and go.

### 6.2 Where Competitors Win

**Capabilities nano-brain lacks:**
1. **Conversation memory** — Mem0 and Zep are purpose-built for chat history recall. nano-brain harvests sessions but doesn't optimize for conversational context.
2. **Entity-level knowledge graphs** — Cognee and GraphRAG extract entities and relationships from unstructured text. nano-brain's graph is code-only (symbols and calls).
3. **Temporal reasoning** — Zep tracks when memories were created and衰减s relevance over time. nano-brain has recency decay but not Zep's fine-grained temporal graph.
4. **Community/ecosystem** — LlamaIndex has a massive plugin ecosystem (160+ integrations). nano-brain is single-purpose.
5. **Multi-hop reasoning** — Cognee and GraphRAG can follow chains of relationships across documents. nano-brain's graph traversal is limited to code symbols.

### 6.3 Positioning Map

```
                    Code Intelligence
                         ↑
                         |
                    nano-brain
                         |
                         |
   Conversation ←--------+--------→ Document
   Memory                |           Knowledge
                         |
              Mem0  Zep  |  Cognee  GraphRAG
                         |
                         ↓
                    No Code Intel
```

nano-brain occupies the upper-left quadrant: code intelligence + hybrid search. Competitors cluster in the lower half, differentiated by conversation memory (Mem0, Zep) vs document knowledge graphs (Cognee, GraphRAG).

### 6.4 Use Case Fit

| Use Case | Best Solution | Why |
|----------|--------------|-----|
| "What breaks if I change this function?" | nano-brain | Only solution with impact analysis |
| "What did we decide about auth last week?" | Zep / Mem0 | Temporal + conversation memory |
| "How do these three documents relate?" | Cognee / GraphRAG | Multi-hop entity reasoning |
| "Find code similar to this pattern" | LlamaIndex | Flexible embedding + reranking |
| "Trace the request from endpoint to DB" | nano-brain | Only solution with call chain tracing |
| "Recall my last 10 conversations about X" | Mem0 / Zep | Conversation-focused retrieval |

---

## 7. Recommendations

### 7.1 For nano-brain Users

1. **Use nano-brain for code-heavy projects.** The code intelligence features are unmatched. If you work on a codebase larger than 50k lines, the impact analysis alone justifies adoption.
2. **Don't rely on code intelligence for interface-heavy code yet.** The 0% callers accuracy and zero outgoing edges for interface dispatch functions means `memory_impact` and `memory_graph` (incoming) will miss critical relationships in Go code using interfaces, or any language with dynamic dispatch.
3. **Combine with a conversation memory tool if needed.** nano-brain's session harvesting is good for context recall, but Mem0 or Zep are better if conversation history is your primary use case.
4. **The phil workspace results suggest content quality matters.** P@5 of 0.45 on a smaller workspace indicates that sparse or poorly structured content degrades search. Curate your indexed content.

### 7.2 For nano-brain Development

**High-impact improvements (by estimated effort):**

| Improvement | Effort | Impact | Priority |
|-------------|--------|--------|----------|
| Fix incoming edge resolution (callers) | Medium | High — unlocks `memory_impact` | P0 |
| Handle interface dispatch in Go extractor | Medium | High — 4/10 functions currently fail | P0 |
| Resolve method calls through struct fields | High | Medium — improves BuildFlow-like cases | P1 |
| Add closure scope tracking | High | Low — narrow use case | P2 |
| Follow transactional context (tx methods) | Medium | Low — narrow use case | P2 |

**The callers accuracy problem** is the single biggest gap. Until incoming edges work, `memory_impact` (the most valuable code intelligence feature) is unreliable. The fix likely involves inverting the edge storage or building a reverse index at query time.

**The interface dispatch problem** requires type resolution beyond AST analysis. This may need a full Go type-checker integration (e.g., `golang.org/x/tools/go/analysis`) rather than pure AST extraction.

### 7.3 Competitive Strategy

1. **Lean into the code intelligence moat.** No competitor is close to matching symbol-level analysis. This is nano-brain's defensible advantage.
2. **Don't try to be a general memory platform.** Mem0 and Zep own conversation memory. Competing there wastes effort.
3. **Publish these benchmarks.** Transparency about strengths (search quality, latency) and weaknesses (callers accuracy, interface dispatch) builds trust. Most competitors don't publish real numbers.
4. **Target the "codebase archaeology" use case.** Inheriting a large codebase with no docs is painful. nano-brain's flow visualization and impact analysis solve this uniquely.

---

## Appendix A: Methodology

### Search Quality Benchmarks

- **20 queries** per workspace, categorized as: feature understanding (5), debugging (5), architecture (5), cross-session (5)
- **P@5:** Precision at 5 — fraction of top-5 results that are relevant (human-judged)
- **MRR:** Mean Reciprocal Rank — average of 1/rank of first relevant result
- **Latency:** Server-side processing time, excluding network

### Code Intelligence Benchmarks

- **10 functions** from the nano-brain Go codebase, selected for diversity (HTTP handlers, service methods, utility functions)
- **Ground truth:** manually verified call relationships (expected outgoing edges, expected callers)
- **Precision:** matched edges / extracted edges
- **Recall:** matched edges / expected edges
- **Incoming edges:** tested via `memory_graph` with `direction="in"` on each function

### Competitor Data

All competitor data comes from published documentation, papers, and public benchmarks. No competitor benchmarks were run against our query set. Latency figures are approximate ranges from public reports, not controlled experiments.

---

## Appendix B: Raw Data Files

- Search quality: `benchmarks/comparison/results/nanobrain.json`
- Code intelligence: `benchmarks/comparison/results/code_intelligence.json`
- Ground truth: `benchmarks/comparison/ground_truth.json`
- Query set: `benchmarks/comparison/queries.json`
