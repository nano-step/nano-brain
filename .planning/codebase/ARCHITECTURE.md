# ARCHITECTURE.md — nano-brain

**Last updated:** 2026-06-28

---

## Overview

nano-brain is an agent-oriented memory and code intelligence daemon. It provides persistent context, impact analysis, and call-chain tracing for AI agents via the MCP (Model Context Protocol). A single Go binary serves HTTP (REST + MCP) backed by PostgreSQL with pgvector.

**Core thesis:** Agents don't read docs — they call tools. Every capability is exposed as an MCP tool or REST endpoint.

---

## Architectural Patterns

| Pattern | Implementation |
|---------|----------------|
| **Service daemon** | Single binary, PID-file lifecycle, `errgroup` goroutine orchestration |
| **Constructor injection** | No DI framework. `config`, `logger`, `*pgxpool.Pool`, `*sqlc.Queries` passed at construction |
| **Interface-based decoupling** | Small role interfaces (`Embedder`, `Querier`, `Harvester`) defined on consumer side |
| **Pipeline architecture** | File → chunk → embed → search. Each stage is a queue/work item |
| **Registry pattern** | `symbol.Registry`, `graph.Registry` — pluggable language extractors |
| **Event bus** | `eventbus.Bus` — async pub/sub between watcher, embed queue, flow materializer |
| **Code generation** | sqlc generates type-safe Go from SQL queries (no hand-written DB code) |

---

## System Layers

```
┌─────────────────────────────────────────────────────────┐
│  AI Agent (OpenCode, Claude Code, Cursor, etc.)         │
└──────────────┬──────────────────────────────────────────┘
               │ MCP Protocol (streamable HTTP / SSE)
               │ REST API (JSON)
┌──────────────▼──────────────────────────────────────────┐
│  Transport Layer                                        │
│  ├── internal/mcp/     — MCP server (16 tools)          │
│  ├── internal/server/  — Echo v4 HTTP server            │
│  │   ├── handlers/     — 76 handler files (per-endpoint)│
│  │   └── middleware/   — auth, workspace, CSRF           │
│  └── cmd/nano-brain/   — CLI dispatcher + daemon mgmt   │
├─────────────────────────────────────────────────────────┤
│  Business Logic                                         │
│  ├── internal/search/     — Hybrid search (BM25+vector) │
│  ├── internal/graph/      — Code intelligence extractors│
│  ├── internal/harvest/    — Session harvesting           │
│  ├── internal/embed/      — Embedding queue + providers  │
│  ├── internal/flow/       — Execution flow materializer  │
│  ├── internal/summarize/  — LLM session summarization    │
│  ├── internal/codesummarize/ — Batched code summaries    │
│  ├── internal/intelligence/  — Memory consolidation      │
│  ├── internal/symbol/     — Code symbol extraction       │
│  ├── internal/chunker/    — Document chunking strategies │
│  ├── internal/links/      — Document link resolution     │
│  └── internal/watcher/    — File system watcher           │
├─────────────────────────────────────────────────────────┤
│  Infrastructure                                         │
│  ├── internal/config/     — YAML + env config (koanf)   │
│  ├── internal/storage/    — pgxpool, sqlc, goose         │
│  ├── internal/eventbus/   — Async pub/sub                │
│  ├── internal/telemetry/  — Query logging + metrics      │
│  ├── internal/health/     — Health checks, doctor        │
│  ├── internal/timefilter/ — Time range parsing           │
│  └── internal/testutil/   — Test DB helpers              │
├─────────────────────────────────────────────────────────┤
│  PostgreSQL 17 + pgvector 0.8.2                         │
│  ├── Documents, Chunks, Embeddings                      │
│  ├── Graph edges, PageRank scores                        │
│  ├── Symbol-aware chunks, Flowcharts                     │
│  └── Telemetry, Code summarization tracking              │
└─────────────────────────────────────────────────────────┘
```

---

## Data Flow

### 1. Document Ingestion

```
File change (watcher) or API write
  → Watcher detects / handler receives
  → chunker.Dispatcher selects strategy (symbol / heading / fixed)
  → Chunks stored in `chunks` table
  → EmbedQueue.Enqueue(chunk_ids)
  → EmbedQueue worker calls Embedder (Ollama / VoyageAI)
  → Vectors stored in `embeddings` table (pgvector)
  → BM25 search vectors updated
```

### 2. Search / Query

```
Agent calls memory_query (or POST /api/v1/query)
  → SearchService.Query()
  ├── BM25 full-text search (tsvector/tsquery)
  ├── Vector similarity (pgvector HNSW cosine)
  ├── Reciprocal Rank Fusion (RRF)
  ├── Recency decay (exponential half-life)
  ├── PageRank boost (pre-computed)
  └── Optional: HyDE, reranking, query preprocessing
  → Ranked results returned as structured JSON
```

### 3. Code Intelligence

```
Agent calls memory_impact / memory_trace / memory_graph
  → MCP handler → sqlc queries
  ├── graph_edges table (pre-computed by watcher)
  ├── Reverse BFS for impact analysis
  ├── Forward BFS for call chain tracing
  └── One-hop lookup for graph queries
  → Structured results (<50ms target)
```

### 4. Session Harvesting

```
OpenCode / Claude Code session files
  → Harvester polls (configurable interval)
  → Deduplication (content hash)
  → Optional LLM summarization (map-reduce pipeline)
  → Stored as documents + chunks
  → Embed queue processes new chunks
```

### 5. Execution Flow

```
File change → Watcher
  → Graph extractors (Echo, Express, Rails, Gin, etc.)
  → Graph edges stored (http, middleware, calls)
  → Flow materializer builds call chains
  → Optional LLM summarization of flow paths
  → Mermaid / sequence diagrams via memory_flow
```

---

## Key Abstractions

| Abstraction | Description |
|-------------|-------------|
| **Workspace** | SHA-256 hash of absolute root path. Isolates all data. |
| **Collection** | Named path within a workspace. Groups related documents. |
| **Document** | Source file metadata + content + tags. Versioned via `supersedes_id`. |
| **Chunk** | Sub-document unit (symbol-aware, heading-based, or fixed-size). |
| **Embedding** | Vector representation of a chunk (provider + model tracked). |
| **GraphEdge** | Directed edge: `source_node → target_node` with type + metadata. |
| **Symbol** | Code entity (function, type, const) extracted by language-specific extractors. |
| **FlowChart** | Pre-materialized control-flow graph for a function span. |

---

## Entry Points

### Daemon Startup

```
cmd/nano-brain/main.go:main()
  → flag.Parse()
  → startServer(configPath)
    → guardBeforeStart()           // single-instance check
    → config.Load()                // YAML + env
    → storage.NewPool()            // pgxpool
    → storage.RunMigrations()      // goose
    → symbol.NewRegistry()         // language extractors
    → graph.NewRegistry()          // graph extractors
    → watcher.New()                // file watcher
    → embed.NewQueue()             // embedding queue
    → eventbus.New()               // pub/sub
    → server.New()                 // HTTP server
    → errgroup: server.Start() + watcher.Run() + queue.Run() + harvester.Run()
```

### MCP Tool Registration

```
internal/mcp/tools.go:RegisterTools(server, adapter)
  → 16 register*() functions
  → Each: schema + handler closure → server.AddTool()
```

### REST Route Registration

```
internal/server/routes.go:registerRoutes(s)
  → /health, /api/status, /api/version
  → /api/v1/ (workspace-scoped group)
    ├── /init, /workspaces, /collections
    ├── /query, /search, /vsearch
    ├── /write, /embed, /reindex
    ├── /graph/*, /flow/*
    └── /events (SSE)
  → /mcp (streamable HTTP + SSE)
```

---

## Concurrency Model

- **errgroup** manages goroutine lifecycle at startup (server, watcher, embed queue, harvester, code summarizer)
- **embed.Queue** — bounded worker pool with configurable concurrency
- **harvest.Runner** — polls harvesters on interval, deduplicates, optionally summarizes
- **watcher.Watcher** — fsnotify-based, debounced, processes file changes sequentially per collection
- **flow.Materializer** — async trigger from watcher, builds call chains with configurable depth/fanout
- **eventbus.Bus** — fan-out pub/sub for cross-component notifications

---

## Error Handling

- `fmt.Errorf("context: %w", err)` — wrapped errors throughout
- No custom error types; callers use `errors.Is` / `errors.As`
- Constructor failures → `log.Warn` (optional) or `log.Fatal` (critical)
- Nil-safe interfaces: optional dependencies checked for nil before use
- Graceful degradation: failed embedder → skip embeddings, failed summarizer → raw harvest

---

## Testing Strategy

| Layer | Approach |
|-------|----------|
| Unit | `package <name>_test`, inline struct mocks, table-driven with `t.Run` |
| Integration | `//go:build integration`, `testutil.SetupTestDB(t)` creates isolated PG schema |
| Handler | `httptest.NewServer` + `httptest.NewRecorder`, mock interfaces |
| MCP | Tool handler unit tests with mock `*Adapter` |
| Benchmark | `bench/` package with generate/run/compare/stress cycle |

**Quick:** `go build ./... && go test -race -short ./...`
**Full:** `go test -race -tags=integration ./...`
