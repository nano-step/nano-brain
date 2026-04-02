# RRI-T Phase 1: PREPARE — nano-brain-full

| Field | Value |
|-------|-------|
| Feature | nano-brain (all 30 MCP tools + core modules) |
| Date | 2026-03-29 |
| Version | latest (from source) |
| Owner | tamlh |
| Dimensions | All 7: UI/UX, API, Performance, Security, Data Integrity, Infrastructure, Edge Cases |
| Personas | All 5: End User, Business Analyst, QA Destroyer, DevOps Tester, Security Auditor |

## System Under Test

### 30 MCP Tools

| # | Tool | Category | Description |
|---|------|----------|-------------|
| 1 | `memory_search` | Search | BM25 full-text keyword search |
| 2 | `memory_vsearch` | Search | Semantic vector search |
| 3 | `memory_query` | Search | Hybrid search (BM25+vector+RRF+rerank) |
| 4 | `memory_expand` | Search | Expand compact result to full content |
| 5 | `memory_get` | Retrieval | Get document by path or docid |
| 6 | `memory_multi_get` | Retrieval | Batch get by glob or comma list |
| 7 | `memory_write` | Write | Write content to daily log |
| 8 | `memory_tags` | Metadata | List tags with doc counts |
| 9 | `memory_status` | Metadata | Index health, collection info |
| 10 | `memory_update` | Management | Trigger immediate reindex |
| 11 | `memory_index_codebase` | Code Intel | Index codebase files |
| 12 | `memory_focus` | Code Intel | Dependency graph context for file |
| 13 | `memory_graph_stats` | Code Intel | File dependency graph stats |
| 14 | `memory_symbols` | Code Intel | Cross-repo symbol query |
| 15 | `memory_impact` | Code Intel | Cross-repo symbol impact |
| 16 | `code_context` | Code Intel | 360° symbol view |
| 17 | `code_impact` | Code Intel | Change impact analysis |
| 18 | `code_detect_changes` | Code Intel | Git diff change detection |
| 19 | `memory_consolidate` | Intelligence | Trigger consolidation cycle |
| 20 | `memory_consolidation_status` | Intelligence | Consolidation queue status |
| 21 | `memory_importance` | Intelligence | Document importance scores |
| 22 | `memory_learning_status` | Intelligence | Learning system status |
| 23 | `memory_suggestions` | Intelligence | Proactive suggestions |
| 24 | `memory_graph_query` | Knowledge Graph | Traverse knowledge graph |
| 25 | `memory_related` | Knowledge Graph | Find related memories |
| 26 | `memory_timeline` | Knowledge Graph | Chronological memory timeline |
| 27 | `memory_connections` | Knowledge Graph | Document connections |
| 28 | `memory_traverse` | Knowledge Graph | N-hop graph traversal |
| 29 | `memory_connect` | Knowledge Graph | Create memory connection |
| 30 | `memory_tags` | Metadata | Tag listing |

### Core Modules Under Test

| Module | File | Key Functions |
|--------|------|---------------|
| Store | `store.ts` | createStore, indexDocument, openDatabase, bulkDeactivateExcept, cleanOrphanedEmbeddings, getIndexHealth |
| Search | `search.ts` | hybridSearch, searchFTS, searchVec, rrfFuse, computeDecayScore, parseSearchConfig |
| Symbol Graph | `symbol-graph.ts` | getContext, getImpact, detectChanges |
| Codebase | `codebase.ts` | indexCodebase, embedPendingCodebase, indexSymbolGraph |
| Harvester | `harvester.ts` | harvestSessions, parseSession, saveHarvestState |
| Embeddings | `embeddings.ts` | createEmbeddingProvider, detectOllamaUrl |
| Consolidation | `consolidation.ts` | ConsolidationAgent |
| Watcher | `watcher.ts` | FileWatcher, incremental indexing |
| Bandits | `bandits.ts` | ThompsonSampler |
| Reranker | `reranker.ts` | VoyageAI reranking |
| Cache | `cache.ts` | ResultCache |
| Vector Store | `vector-store.ts` | createVectorStore (Qdrant, sqlite-vec) |
| Connection Graph | `connection-graph.ts` | traverse, getRelatedDocuments |
| Logger | `logger.ts` | setStdioMode |
| Server | `server.ts` | sequentialFileAppend, SSE transport |

### Transport Modes

1. **stdio** — Claude Desktop, direct pipe
2. **SSE** — Server-Sent Events (HTTP)
3. **Streamable HTTP** — Modern HTTP transport

### Existing Test Baseline

- 67 test files, 1448 passing, 9 skipped
- Previous RRI-T Round 3: 40 test cases (all passing)
- 18 source fixes already applied (P0-P2)

## Test Environment

- Runtime: Node.js + better-sqlite3 (WAL mode)
- Test Framework: Vitest
- Vector Store: sqlite-vec (for tests), Qdrant (for prod)
- Embedding: Mock/local (for tests)
- OS: macOS Darwin 25.1.0

## Scope

This RRI-T covers **all 30 MCP tools** and **15 core modules** across all 7 dimensions. Focus areas:
1. Multi-container concurrent access (opencode1+opencode2 writing simultaneously)
2. Claude Desktop stdio transport integrity
3. Search pipeline correctness (BM25 + vector + RRF + rerank)
4. Code intelligence accuracy (Tree-sitter, symbol graphs)
5. Knowledge graph traversal correctness
6. Memory consolidation & learning system stability
7. Session harvesting reliability
