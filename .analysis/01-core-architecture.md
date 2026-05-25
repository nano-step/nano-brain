# nano-brain Core Architecture Analysis

> Generated: 2026-04-08 | Version: 2026.7.7

## Table of Contents

- [1. Project Overview](#1-project-overview)
- [2. Entry Points](#2-entry-points)
  - [2.1 CLI](#21-cli-srcindexts--1400-lines)
  - [2.2 MCP Server](#22-mcp-server-srcserverts--4087-lines)
  - [2.3 HTTP REST API](#23-http-rest-api-embedded-in-srcserverts)
- [3. Layered Architecture](#3-layered-architecture)
- [4. Complete Module Inventory](#4-complete-module-inventory)
  - [4.1 Interface Layer](#41-interface-layer)
  - [4.2 Search Pipeline](#42-search-pipeline)
  - [4.3 Memory & Knowledge Graph](#43-memory--knowledge-graph)
  - [4.4 Extraction & Indexing](#44-extraction--indexing)
  - [4.5 Code Intelligence](#45-code-intelligence)
  - [4.6 Storage & Backends](#46-storage--backends)
  - [4.7 Infrastructure](#47-infrastructure)
  - [4.8 Eval Framework](#48-eval-framework-srceval)
  - [4.9 Web UI](#49-web-ui-srcweb)
- [5. Provider/Plugin Architecture](#5-providerplugin-architecture)
  - [5.1 Embedding Providers](#51-embedding-providers)
  - [5.2 Vector Store Providers](#52-vector-store-providers)
  - [5.3 LLM Providers](#53-llm-providers)
  - [5.4 Reranker](#54-reranker)
- [6. Data Flow](#6-data-flow)
  - [6.1 Memory Ingestion Flow](#61-memory-ingestion-flow)
  - [6.2 Search Flow](#62-search-flow)
  - [6.3 Codebase Indexing Flow](#63-codebase-indexing-flow)
- [7. Workspace Management](#7-workspace-management)
  - [7.1 Per-Workspace Isolation](#71-per-workspace-isolation)
  - [7.2 Database Schema (v9)](#72-database-schema-v9)
  - [7.3 Schema Versioning](#73-schema-versioning)
  - [7.4 Multi-Project Support](#74-multi-project-support)
- [8. Background Job System](#8-background-job-system)
  - [8.1 Watcher](#81-watcher)
  - [8.2 FTS Worker Thread](#82-fts-worker-thread)
  - [8.3 Consolidation Worker](#83-consolidation-worker)
- [9. Search Pipeline (Detailed)](#9-search-pipeline-detailed)
  - [9.1 Hybrid Search](#91-hybrid-search)
  - [9.2 Intent Types](#92-intent-types)
- [10. Knowledge Graph System](#10-knowledge-graph-system)
- [11. Learning & Adaptation](#11-learning--adaptation)
- [12. Eval Framework](#12-eval-framework)
- [13. Web UI](#13-web-ui)
- [14. Module Dependency Graph](#14-module-dependency-graph)

## 1. Project Overview

**nano-brain** is a persistent memory and code intelligence system for AI coding agents. It provides hybrid search (BM25 + vector), cross-session recall, symbol analysis, knowledge graphs, and adaptive search optimization.

| Property | Value |
|----------|-------|
| Version | 2026.7.7 |
| Language | TypeScript (ESM-only, strict) |
| Target | ESNext |
| Module | ESNext (bundler resolution) |
| Runtime | Node.js |
| Package manager | npm |
| Entry (CLI) | `bin/cli.js` → `src/index.ts` |
| Entry (MCP) | `src/server.ts` |
| Build output | `dist/` |
| Data directory | `~/.nano-brain/` |
| Config file | `~/.nano-brain/config.yml` |

### Key Dependencies

| Dependency | Purpose |
|------------|----------|
| better-sqlite3 | SQLite database engine |
| sqlite-vec | Vector similarity search extension |
| @qdrant/js-client-rest | Qdrant vector DB client |
| @modelcontextprotocol/sdk | MCP server protocol |
| tree-sitter + grammars | AST parsing (TS, JS, Python) |
| chokidar | File system watching |
| yaml | Config file parsing |
| zod | Schema validation |
| fast-glob | File scanning |
| p-limit | Concurrency control |
| express | HTTP/REST API server |

## 2. Entry Points

### 2.1 CLI (`src/index.ts` u2014 ~1400 lines)

The primary entry point. Parses all CLI commands using raw `process.argv` parsing (no framework). Per-workspace SQLite DB path resolved via SHA-256 hash of the workspace path.

**Commands:**
- `init` u2014 Initialize config and DB
- `mcp` u2014 Start MCP server (stdio/HTTP/SSE)
- `status` u2014 Show workspace status, health checks
- `search` / `vsearch` / `query` u2014 Hybrid, vector, or combined search
- `update` u2014 Ingest markdown files from collections
- `embed` u2014 Generate/refresh embeddings
- `collection` u2014 CRUD for collections
- `harvest` u2014 Ingest OpenCode sessions
- `write` u2014 Store a memory with tags
- `cache` u2014 Manage result cache
- `logs` u2014 View log files
- `bench` u2014 Run benchmarks
- `docker` u2014 Docker compose management
- `qdrant` u2014 Qdrant health/migration
- `learning` u2014 Entity extraction, consolidation
- `categorize-backfill` u2014 Backfill LLM categories
- `reset` u2014 Reset database
- `rm` u2014 Remove documents

### 2.2 MCP Server (`src/server.ts` u2014 ~4087 lines)

Full MCP (Model Context Protocol) server exposing nano-brain as tools/resources/prompts.

**Transports:** stdio, HTTP, SSE, StreamableHTTP

**Pattern:** `ServerDeps` dependency injection u2014 all handlers receive a shared deps object containing store, embedder, LLM provider, config, etc.

**Workspace resolution:** In daemon mode, per-request workspace detection from headers/params.

### 2.3 HTTP REST API (embedded in `src/server.ts`)

Express-based REST endpoints for programmatic access:
- `GET /api/status` u2014 Health/stats
- `POST /api/query` u2014 Hybrid search
- `POST /api/search` u2014 FTS search
- `POST /api/write` u2014 Store memory
- `POST /api/embed` u2014 Trigger embedding
- `GET /api/logs` u2014 View logs
- `GET /metrics` u2014 Prometheus metrics

## 3. Layered Architecture

```
u250cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2510
u2502  LAYER 1: Interface                                              u2502
u2502  CLI (index.ts) | MCP Server (server.ts) | REST API (server.ts)  u2502
u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2524
u2502  LAYER 2: Search Pipeline                                        u2502
u2502  search.ts | intent-classifier.ts | bandits.ts | reranker.ts     u2502
u2502  cache.ts | telemetry.ts | preference-model.ts                   u2502
u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2524
u2502  LAYER 3: Memory & Knowledge Graph                                u2502
u2502  memory-graph.ts | connection-graph.ts | symbol-graph.ts          u2502
u2502  entity-extraction.ts | entity-merger.ts | importance.ts          u2502
u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2524
u2502  LAYER 4: Extraction & Indexing                                   u2502
u2502  extraction.ts | harvester.ts | codebase.ts | consolidation.ts    u2502
u2502  categorizer.ts | llm-categorizer.ts | expansion.ts              u2502
u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2524
u2502  LAYER 5: Code Intelligence                                       u2502
u2502  treesitter.ts | graph.ts | symbols.ts | flow-detection.ts       u2502
u2502  chunker.ts | sequence-analyzer.ts                               u2502
u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2524
u2502  LAYER 6: Storage & Backends                                      u2502
u2502  store.ts | vector-store.ts | providers/sqlite-vec.ts             u2502
u2502  providers/qdrant.ts | event-store.ts | storage.ts               u2502
u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2524
u2502  LAYER 7: Infrastructure                                          u2502
u2502  logger.ts | host.ts | metrics.ts | collections.ts               u2502
u2502  workspace-profile.ts | types.ts | watcher.ts                    u2502
u2514u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2518
```

## 4. Complete Module Inventory

### 4.1 Interface Layer

| File | Lines | Purpose | Key Exports | Dependencies |
|------|-------|---------|-------------|---------------|
| `src/index.ts` | ~1400 | CLI entry point, all commands | `GlobalOptions` | store, search, collections, embeddings, harvester, codebase, watcher, bench, consolidation, extraction, symbols, pruning, entity-merger, importance, llm-provider, llm-categorizer, sequence-analyzer, storage, workspace-profile, logger |
| `src/server.ts` | ~4087 | MCP server + REST API | `startServer()`, `ServerDeps` | All modules (central orchestrator for daemon mode) |

### 4.2 Search Pipeline

| File | Lines | Purpose | Key Exports | Dependencies |
|------|-------|---------|-------------|---------------|
| `src/search.ts` | 758 | Hybrid search (BM25 + vector + RRF fusion) | `hybridSearch()` | store, types, telemetry, bandits, intent-classifier, fts-client, symbol-graph, connection-graph, importance, preference-model, reranker |
| `src/intent-classifier.ts` | 56 | Keyword-based query intent classification | `IntentClassifier`, `IntentType` | types, logger |
| `src/bandits.ts` | 144 | Thompson Sampling for search param optimization | `ThompsonSampler`, `BanditConfig` | logger |
| `src/reranker.ts` | 104 | VoyageAI neural reranking | `Reranker`, `VoyageAIReranker`, `createReranker()` | logger |
| `src/cache.ts` | 59 | In-memory TTL result cache | `ResultCache` | types |
| `src/telemetry.ts` | 105 | Search telemetry, query chain detection, reformulation detection | `generateQueryId()`, `detectReformulation()`, `detectQueryChains()` | (none) |
| `src/preference-model.ts` | 142 | Category weight learning from user expand behavior | `computeCategoryWeights()`, `updatePreferenceWeights()` | types, store, workspace-profile, logger |
| `src/expansion.ts` | 61 | LLM-based query expansion | `QueryExpander`, `createLLMQueryExpander()` | consolidation (LLMProvider), logger |

### 4.3 Memory & Knowledge Graph

| File | Lines | Purpose | Key Exports | Dependencies |
|------|-------|---------|-------------|---------------|
| `src/memory-graph.ts` | 192 | Knowledge graph: entities + edges, BFS traversal | `MemoryGraph` | types, better-sqlite3 |
| `src/connection-graph.ts` | 50 | Document-level connection traversal | `traverse()`, `getRelatedDocuments()` | types |
| `src/symbol-graph.ts` | ~704 | Code symbol call graph, context/impact analysis | `SymbolGraph` | store, types, logger |
| `src/entity-extraction.ts` | 124 | LLM-based entity/relationship extraction from memories | `extractEntitiesFromMemory()`, `parseEntityExtractionResponse()` | consolidation (LLMProvider), logger |
| `src/entity-merger.ts` | ~267 | Deduplication via Levenshtein + prefix matching | `findSimilarEntities()`, `mergeEntities()` | types, logger |
| `src/importance.ts` | 101 | Importance scoring: usage + entity density + recency + connections | `ImportanceScorer` | types, logger |
| `src/pruning.ts` | 83 | Soft/hard delete of contradicted & orphan entities | `runPruningCycle()`, `softDeleteContradictedEntities()` | types, logger |

### 4.4 Extraction & Indexing

| File | Lines | Purpose | Key Exports | Dependencies |
|------|-------|---------|-------------|---------------|
| `src/extraction.ts` | ~234 | Fact extraction from sessions via LLM | `extractFactsFromSession()`, `storeExtractedFact()` | types, consolidation (LLMProvider), store, logger |
| `src/harvester.ts` | 858 | OpenCode session harvesting (SQLite + JSON) | `harvestSessions()` | store, types, logger |
| `src/codebase.ts` | 876 | Codebase indexing: file scan, AST chunking, symbol graph, PageRank, Louvain | `indexCodebase()` | store, treesitter, graph, chunker, symbol-graph, flow-detection, embeddings, logger |
| `src/consolidation.ts` | ~436 | LLM consolidation agent: summarize, connect, supersede | `ConsolidationAgent`, `LLMProvider` interface | store, types, logger |
| `src/consolidation-worker.ts` | 109 | Background polling worker for consolidation jobs | `ConsolidationWorker` | store, consolidation, logger |
| `src/categorizer.ts` | 61 | Keyword-based auto-categorization | `categorize()` | (none) |
| `src/llm-categorizer.ts` | 99 | LLM-based categorization with confidence thresholds | `categorizeMemory()` | consolidation (LLMProvider), types, logger |

### 4.5 Code Intelligence

| File | Lines | Purpose | Key Exports | Dependencies |
|------|-------|---------|-------------|---------------|
| `src/treesitter.ts` | ~861 | Tree-sitter AST parsing, symbol/edge extraction | `extractSymbols()`, `extractEdges()`, `parseToAST()`, `CodeSymbol`, `SymbolEdge` | graph (SupportedLanguage), logger |
| `src/graph.ts` | ~710 | File dependency graph: import resolution for JS/TS/Python/Ruby/Vue | `detectLanguage()`, `buildDependencyGraph()`, `pageRank()`, `louvainClustering()` | better-sqlite3, fs, path |
| `src/symbols.ts` | 556 | Cross-repo symbol extraction: Redis keys, PubSub, MySQL tables, API endpoints, Bull queues | `extractSymbolsFromContent()`, `SymbolType` | logger |
| `src/flow-detection.ts` | ~250 | Call flow tracing from entry points through symbol edges | `detectEntryPoints()`, `traceFlows()`, `storeFlows()` | store, logger, better-sqlite3 |
| `src/chunker.ts` | ~553 | Markdown heading-aware + Tree-sitter AST chunking | `chunkContent()`, `chunkByAST()` | treesitter |
| `src/sequence-analyzer.ts` | ~422 | Query pattern analysis: k-means clustering, transition matrices, next-query prediction | `SequenceAnalyzer` | store, types, logger |

### 4.6 Storage & Backends

| File | Lines | Purpose | Key Exports | Dependencies |
|------|-------|---------|-------------|---------------|
| `src/store.ts` | ~1332+ | Central SQLite storage: 18+ tables, schema v9, FTS5, prepared statements | `createStore()`, `Store` impl, `computeHash()`, `indexDocument()` | types, better-sqlite3, sqlite-vec, metrics, logger |
| `src/vector-store.ts` | 84 | Vector store abstraction + factory | `VectorStore` interface, `createVectorStore()` | providers/sqlite-vec, providers/qdrant, logger |
| `src/providers/sqlite-vec.ts` | ~227 | sqlite-vec vector store implementation | `SqliteVecStore` | vector-store (types), logger, better-sqlite3 |
| `src/providers/qdrant.ts` | ~281 | Qdrant vector store implementation | `QdrantVecStore` | vector-store (types), @qdrant/js-client-rest, host, logger |
| `src/event-store.ts` | 75 | SQLite event store for MCP StreamableHTTP resumability | `SqliteEventStore` | better-sqlite3, @modelcontextprotocol/sdk |
| `src/storage.ts` | ~269 | Disk space management, retention eviction, size eviction | `parseStorageConfig()`, `evictExpiredSessions()`, `evictBySize()` | types, logger, fs |
| `src/fts-worker.ts` | ~238 | Worker thread for FTS/vector queries (read-only SQLite) | Worker message handler | better-sqlite3, sqlite-vec, types |
| `src/fts-client.ts` | 138 | Main-thread RPC wrapper for FTS worker | `initFTSWorker()`, `searchFTSAsync()`, `searchVecWorker()` | worker_threads, types, logger |

### 4.7 Infrastructure

| File | Lines | Purpose | Key Exports | Dependencies |
|------|-------|---------|-------------|---------------|
| `src/types.ts` | ~911 | Core type definitions for all modules | `SearchResult`, `Document`, `Store`, `MemoryChunk`, `Collection`, all config types | (none) |
| `src/logger.ts` | 141 | File + stdout logging with rotation, stdio mode suppression | `log()`, `initLogger()`, `setStdioMode()`, `cliOutput()` | fs, path, os |
| `src/host.ts` | 31 | Docker container detection + localhost-to-host.docker.internal rewrite | `isInsideContainer()`, `resolveHostUrl()` | fs |
| `src/metrics.ts` | 98 | In-memory counters + Prometheus text export | `incrementCounter()`, `getMetricsAsPrometheus()` | (none) |
| `src/collections.ts` | 279 | YAML config loading/saving, collection CRUD | `loadCollectionConfig()`, `saveCollectionConfig()` | yaml, types, logger |
| `src/workspace-profile.ts` | 64 | Per-workspace usage profiling from telemetry | `WorkspaceProfile` | types, logger |
| `src/watcher.ts` | ~869 | Chokidar file watcher + background job orchestration (9+ jobs) | `startWatcher()` | chokidar, store, harvester, codebase, embeddings, consolidation, extraction, importance, pruning, sequence-analyzer, entity-merger, logger |
| `src/embeddings.ts` | 490 | Embedding provider factory + Ollama/OpenAI-compatible implementations | `createEmbeddingProvider()`, `EmbeddingProvider` | types, host, logger |
| `src/llm-provider.ts` | 140 | LLM provider factory: Ollama + GitLab Duo (OpenAI-compatible) | `createLLMProvider()`, `OllamaLLMProvider`, `GitlabDuoLLMProvider` | consolidation (LLMProvider interface), types, logger |
| `src/bench.ts` | ~1133 | Comprehensive benchmark suite: search, embed, cache, store, connections, quality, scale, consolidation, memory | `runBenchmarks()` | store, search, embeddings, collections, connection-graph, consolidation |

### 4.8 Eval Framework (`src/eval/`)

| File | Lines | Purpose | Key Exports |
|------|-------|---------|-------------|
| `src/eval/types.ts` | 73 | Ground truth types, metrics types, eval report structure | `GroundTruth`, `DimensionMetrics`, `EvalReport`, `CalibrationBucket` |
| `src/eval/harness.ts` | u2014 | Eval harness: runs fixtures, computes precision/recall/F1 | `runEval()` |
| `src/eval/calibration.ts` | u2014 | Confidence calibration analysis | Calibration metrics |
| `src/eval/regression.ts` | u2014 | Regression testing across versions | Regression detection |
| `src/eval/report.ts` | u2014 | Report generation (text + JSON) | Report formatting |
| `src/eval/loader.ts` | u2014 | Fixture loading from YAML/JSON | Fixture loading |

### 4.9 Web UI (`src/web/`)

| File | Purpose |
|------|---------|
| `src/web/vite.config.ts` | Vite build config for web UI |
| `src/web/src/api/client.ts` | API client for REST endpoints |
| `src/web/src/store/app.ts` | Frontend state management |
| `src/web/src/lib/graph-adapter.ts` | Graph visualization adapter |
| `src/web/src/lib/colors.ts` | Color utilities |

## 5. Provider/Plugin Architecture

### 5.1 Embedding Providers (`src/embeddings.ts`)

```
EmbeddingProvider (interface)
u251cu2500u2500 embed(text: string) -> { embedding: number[], model: string }
u251cu2500u2500 embedBatch(texts: string[]) -> { embeddings: number[][], model: string }
u2514u2500u2500 getDimensions() -> number

Implementations:
u251cu2500u2500 OllamaEmbeddingProvider     (local Ollama, /api/embeddings)
u2514u2500u2500 OpenAICompatibleEmbeddingProvider (any OpenAI-compatible API)

Factory: createEmbeddingProvider(config) -> EmbeddingProvider
```

**Features:** Rate limiting via p-limit, sub-batching for large inputs, model context detection (auto-detect dimensions from first response), Docker host URL rewriting.

### 5.2 Vector Store Providers (`src/vector-store.ts`)

```
VectorStore (interface)
u251cu2500u2500 search(embedding, options) -> VectorSearchResult[]
u251cu2500u2500 upsert(point) -> void
u251cu2500u2500 batchUpsert(points) -> void
u251cu2500u2500 delete(id) -> void
u251cu2500u2500 deleteByHash(hash) -> void
u251cu2500u2500 health() -> VectorStoreHealth
u2514u2500u2500 close() -> void

Implementations:
u251cu2500u2500 SqliteVecStore (src/providers/sqlite-vec.ts)
u2502   - Uses sqlite-vec extension
u2502   - Cosine distance metric
u2502   - Auto-rebuilds vec table on dimension change
u2502   - Batch upsert via SQLite transactions
u2514u2500u2500 QdrantVecStore (src/providers/qdrant.ts)
    - Uses @qdrant/js-client-rest
    - SHA-256 deterministic UUIDs (collision-safe)
    - Auto-creates collection with payload indexes
    - Retry logic with exponential backoff for socket errors
    - Lazy initialization (ensureCollection on first use)

Factory: createVectorStore(config) -> VectorStore
```

### 5.3 LLM Providers (`src/llm-provider.ts`)

```
LLMProvider (interface, defined in consolidation.ts)
u251cu2500u2500 complete(prompt: string) -> { text: string, tokensUsed: number }
u2514u2500u2500 model: string

Implementations:
u251cu2500u2500 OllamaLLMProvider       (local Ollama, /api/generate, 120s timeout)
u2514u2500u2500 GitlabDuoLLMProvider    (OpenAI-compatible /v1/chat/completions, 60s timeout)

Factory: createLLMProvider(config) -> LLMProvider | null
Default endpoint: https://ai-proxy.thnkandgrow.com
Default model: gitlab/claude-haiku-4-5
```

**Used by:** Consolidation, extraction, entity extraction, LLM categorization, query expansion.

### 5.4 Reranker (`src/reranker.ts`)

```
Reranker (interface)
u2514u2500u2500 rerank(query, documents) -> RerankedResult[]

Implementations:
u2514u2500u2500 VoyageAIReranker (api.voyageai.com/v1/rerank)

Factory: createReranker(apiKey?) -> Reranker | null
```

## 6. Data Flow

### 6.1 Memory Ingestion Flow

```
Markdown files / Sessions / CLI write
        u2502
        u25bc
  Content hashing (SHA-256)
        u2502
        u25bc
  Chunking (heading-aware or AST-based)
        u2502
        u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2510
        u25bc                               u25bc
  store.insertContent()          Embedding generation
  store.insertDocument()         (Ollama/OpenAI-compatible)
  store.insertTags()                    u2502
        u2502                               u25bc
        u2502                     VectorStore.upsert()
        u2502                     (sqlite-vec or Qdrant)
        u25bc
  Auto-categorization
  (keyword u2192 auto:* tags)
  (LLM u2192 llm:* tags, if configured)
        u2502
        u25bc
  Entity extraction (LLM)
  u2192 memory_entities + memory_edges
        u2502
        u25bc
  Consolidation queue
  u2192 LLM detects: supersessions, connections, summaries
```

### 6.2 Search Flow

```
Query input
    u2502
    u25bc
Intent classification (lookup/explanation/architecture/recall)
    u2502
    u25bc
Config overrides applied per intent
    u2502
    u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2510
    u25bc                       u25bc
  FTS5 (BM25)          Vector search
  (main or worker)     (embed query u2192 cosine)
    u2502                       u2502
    u2514u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u252cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2518
               u2502
    Reciprocal Rank Fusion (RRF)
    k = bandit-tuned (30/60/90)
               u2502
    Post-processing boosts:
    u251cu2500 Top-rank bonus (+0.3/0.15)
    u251cu2500 Centrality boost (PageRank u00d7 weight)
    u251cu2500 Usage boost (access_count)
    u251cu2500 Category weight boost (preference-model)
    u251cu2500 Supersede demotion (u00d70.3 if superseded)
    u251cu2500 Importance boost (scorer)
    u2514u2500 Position-aware blending
               u2502
    Neural reranking (VoyageAI, optional)
               u2502
    Connection graph expansion (optional)
               u2502
    Symbol graph enrichment (optional)
               u2502
    Thompson Sampling reward recording
               u2502
    Telemetry logging
               u2502
               u25bc
         SearchResult[]
```

### 6.3 Codebase Indexing Flow

```
Workspace root
    u2502
    u25bc
File scanning (fast-glob, respects .gitignore)
    u2502
    u25bc
Language detection (JS/TS/Python/Ruby/Vue)
    u2502
    u251cu2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2500u2510
    u25bc                          u25bc
Import resolution           Tree-sitter AST parsing
(graph.ts)                  (treesitter.ts)
    u2502                          u2502
    u25bc                          u25bc
File dependency graph       Symbol + edge extraction
    u2502                     (functions, classes, calls)
    u251cu2500u2500 PageRank                   u2502
    u251cu2500u2500 Louvain clustering         u2502
    u2502                          u25bc
    u2502                     SymbolGraph storage
    u2502                          u2502
    u2502                          u25bc
    u2502                     Flow detection
    u2502                     (entry points u2192 trace calls)
    u25bc                          u2502
Results stored in:                u25bc
u2022 code_symbols table      execution_flows table
u2022 symbol_edges table      flow_steps table
u2022 louvain_communities     file centrality scores
```

## 7. Workspace Management

### 7.1 Per-Workspace Isolation

Each workspace gets its own SQLite database:

```
~/.nano-brain/data/<sha256-of-workspace-path>.sqlite
```

The SHA-256 hash of the absolute workspace path produces a deterministic, collision-free DB filename. This ensures complete data isolation between workspaces.

### 7.2 Database Schema (v9)

18+ tables organized into 5 groups:

**Core Storage:**
- `content` u2014 Raw content blobs keyed by SHA-256 hash
- `documents` u2014 Document metadata (path, collection, title, hash, project_hash, centrality, cluster_id, superseded_by, access_count, last_accessed_at, importance_score)
- `document_tags` u2014 Tag associations
- `content_vectors` u2014 Embedding tracking (hash_seq u2192 model, dimensions)
- `vectors_vec` u2014 sqlite-vec virtual table for vector similarity
- `documents_fts` u2014 FTS5 virtual table for full-text search

**Knowledge Graph:**
- `memory_entities` u2014 Named entities (tool, service, person, concept, decision, file, library)
- `memory_edges` u2014 Relationships (uses, depends_on, decided_by, related_to, replaces, configured_with)
- `memory_connections` u2014 Document-level connections (supports, contradicts, extends, supersedes, related, caused_by, refines, implements)

**Code Intelligence:**
- `code_symbols` u2014 Functions, classes, methods with file/line info, exported flag, cluster_id
- `symbol_edges` u2014 Call graph edges (CALLS, IMPORTS, EXTENDS, IMPLEMENTS) with confidence
- `execution_flows` u2014 Detected call flows with entry/terminal symbols
- `flow_steps` u2014 Individual steps in execution flows
- `louvain_communities` u2014 Community detection results

**Telemetry & Learning:**
- `search_telemetry` u2014 Query logs with results, expanded indices, timing
- `bandit_state` u2014 Thompson Sampling parameter state
- `query_clusters` u2014 k-means query clusters for sequence analysis
- `cluster_transitions` u2014 Markov transition probabilities
- `workspace_profiles` u2014 Per-workspace usage profiles

**Operations:**
- `consolidation_jobs` u2014 Pending/completed consolidation jobs
- `harvest_state` u2014 Incremental harvesting cursor
- `llm_cache` u2014 LLM response caching
- `mcp_events` u2014 MCP event store for StreamableHTTP resumability

### 7.3 Schema Versioning

Uses SQLite `user_version` pragma. `TARGET_VERSION = 9`. Incremental migrations run automatically on store open. Includes corruption recovery: detects malformed databases and rebuilds from scratch.

### 7.4 Multi-Project Support

Within a single workspace DB, documents are tagged with `project_hash` for multi-project isolation. Search can filter by project or use `'global'` scope. The `'all'` project hash searches across all projects.

## 8. Background Job System

### 8.1 Watcher (`src/watcher.ts` u2014 ~869 lines)

Chokidar-based file watcher that orchestrates 9+ background jobs with configurable intervals:

| Job | Default Interval | Purpose |
|-----|------------------|---------|
| Reindex | On file change | Re-scan changed files, update content/documents |
| Harvest | 30s | Ingest new OpenCode sessions |
| Embed | 10s | Generate embeddings for unembedded content |
| Learning (entity extraction) | Configurable | Extract entities/relationships from new memories |
| Consolidation | Configurable | LLM-based supersession detection, summarization |
| Importance recalculation | Configurable | Recompute importance scores |
| Sequence analysis | Configurable | k-means clustering + transition matrices |
| Soft pruning | 6h | Soft-delete contradicted/orphan entities |
| Hard pruning | Configurable | Permanently delete old soft-deleted entities |

**Pattern:** Dirty flag tracking u2014 each job has a dirty flag set by file changes, jobs only run when dirty. Jobs run sequentially to avoid DB contention.

### 8.2 FTS Worker Thread (`src/fts-worker.ts` + `src/fts-client.ts`)

Dedicated worker thread for read-only FTS/vector queries to avoid blocking the main event loop during writes:

- Opens a separate **read-only** SQLite connection
- Communicates via `worker_threads` message passing
- UUID-based RPC with 30s timeout per request
- Loads sqlite-vec extension independently
- `busy_timeout = 0` (never blocks on locks)

### 8.3 Consolidation Worker (`src/consolidation-worker.ts`)

Polling worker that processes consolidation jobs from the `consolidation_jobs` table:
- Polls every 5s (configurable)
- Processes one job at a time
- Job state machine: pending u2192 processing u2192 completed/failed

## 9. Search Pipeline (Detailed)

### 9.1 Hybrid Search (`src/search.ts`)

The `hybridSearch()` function is the central search orchestrator:

1. **Intent classification** u2014 `IntentClassifier.classify()` detects query intent (lookup/explanation/architecture/recall) via keyword patterns. Each intent has config overrides (e.g., architecture queries increase limit, recall queries boost recency).

2. **Thompson Sampling** u2014 `ThompsonSampler.selectSearchConfig()` selects search parameters (rrf_k, centrality_weight) via Beta distribution sampling. Optimized parameters:
   - `rrf_k`: 30, 60, or 90
   - `centrality_weight`: 0.0, 0.05, 0.1, or 0.2

3. **Parallel retrieval** u2014 FTS5 (BM25) and vector search run concurrently. FTS can run in worker thread via `searchFTSAsync()` or main thread.

4. **Reciprocal Rank Fusion** u2014 Combines BM25 and vector results: `score = 1/(k + rank_fts) + 1/(k + rank_vec)`

5. **Post-processing cascade:**
   - Top-rank bonus: +0.3 for rank 1, +0.15 for rank 2
   - Centrality boost: `score += centrality * centralityWeight`
   - Usage boost: `score *= (1 + log(1 + accessCount) * 0.05)`
   - Category weight boost: per-category multiplier from preference model
   - Supersede demotion: `score *= 0.3` if document is superseded
   - Importance boost: `score *= (1 + importanceWeight * importanceScore)`
   - Position-aware blending: alternates FTS/vector results for diversity

6. **Neural reranking** u2014 Optional VoyageAI reranker for final re-scoring.

7. **Graph enrichment** u2014 Connection graph traversal and symbol graph context added to results.

8. **Reward recording** u2014 Thompson Sampling records success/failure based on user expand actions (via telemetry).

### 9.2 Intent Types

| Intent | Trigger Keywords | Config Overrides |
|--------|-----------------|------------------|
| lookup | "where is", "find", "show me" | Higher FTS weight |
| explanation | "how does", "explain", "why" | Longer snippets |
| architecture | "architecture", "design", "structure" | Increased limit |
| recall | "remember", "last time", "what did we" | Recency boost |

## 10. Knowledge Graph System

### 10.1 Memory Graph (`src/memory-graph.ts`)

Entity-relationship graph for learned knowledge:

**Entity types:** tool, service, person, concept, decision, file, library

**Edge types:** uses, depends_on, decided_by, related_to, replaces, configured_with

**Operations:**
- `insertEntity()` u2014 Upsert with COLLATE NOCASE dedup
- `insertEdge()` u2014 INSERT OR IGNORE
- `traverse()` u2014 BFS up to depth 10, returns entities + edges + paths
- `findSimilarEntities()` u2014 LIKE-based name search
- `getStats()` u2014 Entity/edge counts by type

**Lifecycle:** Entities track `first_learned_at`, `last_confirmed_at`, `contradicted_at`, `pruned_at`.

### 10.2 Connection Graph (`src/connection-graph.ts`)

Document-level connections with 8 relationship types: supports, contradicts, extends, supersedes, related, caused_by, refines, implements.

**Operations:**
- `traverse()` u2014 BFS from document, optional relationship type filter
- `getRelatedDocuments()` u2014 Direct neighbors only

### 10.3 Symbol Graph (`src/symbol-graph.ts`)

Code-level call graph:
- `insertSymbol()` / `insertEdge()` u2014 From Tree-sitter extraction
- `searchByName()` u2014 Find symbols by name pattern
- `context()` u2014 Get callers + callees of a symbol
- `impact()` u2014 Transitive closure of callers ("what breaks if I change this?")

### 10.4 Entity Lifecycle

```
Extraction (LLM) u2192 memory_entities
       u2502
       u25bc
Merging (Levenshtein dedup) u2192 canonical selection
       u2502
       u25bc
Consolidation (LLM) u2192 detect contradictions, supersessions
       u2502
       u25bc
Pruning: soft-delete contradicted (30d) / orphan (90d)
       u2502
       u25bc
Hard delete (30d after soft-delete)
```

## 11. Learning & Adaptation

### 11.1 Thompson Sampling (`src/bandits.ts`)

Multi-armed bandit for search parameter optimization:
- Beta distribution sampling via Marsaglia-Tsang gamma method
- Tuned parameters: `rrf_k` (30/60/90) and `centrality_weight` (0.0/0.05/0.1/0.2)
- Min 100 observations before exploitation (pure exploration until then)
- State persisted in `bandit_state` table
- Rewards based on user expand actions from telemetry

### 11.2 Preference Model (`src/preference-model.ts`)

Learns per-category weights from user behavior:
- Tracks which categories users expand in search results
- Computes expand rate per category tag (auto:* and llm:*)
- Weight = expandRate / baselineExpandRate, clamped to [0.5, 2.0]
- Cold start protection: requires min 20 queries before activating
- Updates stored in workspace profile

### 11.3 Sequence Analysis (`src/sequence-analyzer.ts`)

Predicts next query from query patterns:
- **k-means clustering** of query embeddings (k-means++ initialization)
- **Markov transition matrix** between query clusters
- **Next-query prediction** based on current cluster u2192 most probable next cluster
- Used for proactive prefetching in daemon mode

### 11.4 Importance Scoring (`src/importance.ts`)

Weighted formula: `score = w_usage * usageNorm + w_entity * entityNorm + w_recency * recencyNorm + w_connections * connectionNorm`

- `usageNorm`: access_count / max_access_count
- `entityNorm`: min(tagCount / 5, 1.0)
- `recencyNorm`: exp(-0.693 * daysSinceAccess / halfLifeDays)
- `connectionNorm`: connectionCount / maxConnections

### 11.5 Fact Extraction (`src/extraction.ts`)

LLM extracts discrete facts from sessions:
- Categories: architecture-decision, technology-choice, coding-pattern, preference, debugging-insight, config-detail
- SHA-256 dedup prevents storing identical facts
- Facts stored as documents with `auto:extracted-fact` tag

### 11.6 Consolidation (`src/consolidation.ts`)

LLM agent that processes memories to:
- Detect supersessions (newer info replaces older)
- Create connections between related memories
- Generate summaries
- Mark contradicted entities

## 12. Eval Framework

Located in `src/eval/`. Measures code intelligence accuracy:

**Ground truth:** YAML/JSON fixtures with expected symbols, edges, and flows.

**Metrics per dimension (symbols, edges, flows):**
- Precision, Recall, F1
- True/False positives, False negatives

**Calibration:** Buckets edge confidence scores and measures predicted vs actual accuracy (ECE-style).

**Regression:** Compares current results against baseline to detect regressions.

**Report:** Text + JSON output with aggregate and per-fixture results.

## 13. Web UI

Vite-based SPA in `src/web/`:
- **API client** (`src/web/src/api/client.ts`) u2014 Wraps REST endpoints
- **State management** (`src/web/src/store/app.ts`) u2014 Frontend state
- **Graph visualization** (`src/web/src/lib/graph-adapter.ts`) u2014 Adapts knowledge graph data for visualization
- **Colors** (`src/web/src/lib/colors.ts`) u2014 Theming utilities

Served by the MCP server's Express instance on the same port.

## 14. Module Dependency Graph

```
index.ts (CLI)
  u251cu2500 store.ts
  u251cu2500 search.ts
  u2502    u251cu2500 store.ts
  u2502    u251cu2500 bandits.ts
  u2502    u251cu2500 intent-classifier.ts
  u2502    u251cu2500 telemetry.ts
  u2502    u251cu2500 fts-client.ts u2192 fts-worker.ts
  u2502    u251cu2500 reranker.ts
  u2502    u251cu2500 connection-graph.ts u2192 types.ts
  u2502    u251cu2500 symbol-graph.ts
  u2502    u251cu2500 importance.ts
  u2502    u2514u2500 preference-model.ts u2192 workspace-profile.ts
  u251cu2500 collections.ts u2192 yaml, types.ts
  u251cu2500 embeddings.ts u2192 host.ts, types.ts
  u251cu2500 harvester.ts u2192 store.ts, types.ts
  u251cu2500 codebase.ts
  u2502    u251cu2500 treesitter.ts u2192 graph.ts
  u2502    u251cu2500 graph.ts (import resolution, PageRank, Louvain)
  u2502    u251cu2500 chunker.ts u2192 treesitter.ts
  u2502    u251cu2500 symbol-graph.ts
  u2502    u2514u2500 flow-detection.ts u2192 store.ts
  u251cu2500 watcher.ts (orchestrates all background jobs)
  u251cu2500 extraction.ts u2192 consolidation.ts (LLMProvider)
  u251cu2500 consolidation.ts u2192 store.ts
  u251cu2500 consolidation-worker.ts u2192 consolidation.ts
  u251cu2500 llm-provider.ts u2192 consolidation.ts (LLMProvider interface)
  u251cu2500 llm-categorizer.ts u2192 consolidation.ts (LLMProvider interface)
  u251cu2500 entity-extraction.ts u2192 consolidation.ts (LLMProvider interface)
  u251cu2500 entity-merger.ts u2192 types.ts
  u251cu2500 importance.ts u2192 types.ts
  u251cu2500 pruning.ts u2192 types.ts
  u251cu2500 sequence-analyzer.ts u2192 store.ts, types.ts
  u251cu2500 storage.ts u2192 types.ts, fs
  u251cu2500 workspace-profile.ts u2192 types.ts
  u251cu2500 categorizer.ts (standalone)
  u251cu2500 expansion.ts u2192 consolidation.ts (LLMProvider)
  u251cu2500 symbols.ts (standalone regex extractors)
  u2514u2500 bench.ts u2192 store.ts, search.ts, embeddings.ts, collections.ts, connection-graph.ts

server.ts (MCP + REST)
  u251cu2500 All modules above
  u251cu2500 event-store.ts u2192 better-sqlite3, @modelcontextprotocol/sdk
  u251cu2500 memory-graph.ts u2192 better-sqlite3, types.ts
  u2514u2500 vector-store.ts
       u251cu2500 providers/sqlite-vec.ts u2192 better-sqlite3
       u2514u2500 providers/qdrant.ts u2192 @qdrant/js-client-rest, host.ts

Shared across all:
  u251cu2500 types.ts (core interfaces, no dependencies)
  u251cu2500 logger.ts (file + stdout logging)
  u2514u2500 metrics.ts (in-memory counters)
```

### Key Architectural Patterns

1. **Central Store interface** u2014 `types.ts` defines `Store` with 100+ methods. `store.ts` implements it. All modules depend on Store, never on raw SQLite.

2. **Provider/Factory pattern** u2014 All external integrations (embeddings, vector DB, LLM, reranker) use interfaces with factory functions. Providers are selected at runtime from config.

3. **LLMProvider as shared interface** u2014 Defined in `consolidation.ts`, implemented in `llm-provider.ts`, consumed by 5+ modules. Single interface, multiple implementations.

4. **Worker thread isolation** u2014 FTS queries run in a dedicated read-only worker thread to prevent main-thread blocking during writes.

5. **Per-workspace DB isolation** u2014 SHA-256 hash of workspace path u2192 separate SQLite file. No cross-workspace data leakage.

6. **Background job orchestration** u2014 `watcher.ts` coordinates 9+ jobs with dirty flags, sequential execution, and configurable intervals.

7. **Adaptive search** u2014 Thompson Sampling + preference model + importance scoring create a feedback loop that improves search quality over time.

8. **Schema versioning** u2014 SQLite `user_version` pragma with incremental migrations. Corruption recovery rebuilds from scratch.
