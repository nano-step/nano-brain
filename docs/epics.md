---
stepsCompleted: [1, 2, 3]
inputDocuments:
  - docs/prds/prd-nano-brain-greenfield-2026-05-23/prd.md
  - docs/architecture.md
---

# nano-brain-greenfield - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for nano-brain-greenfield, decomposing the requirements from the PRD and Architecture into implementable stories.

## Requirements Inventory

### Functional Requirements

- FR-1: The harvester reads OpenCode session files from `harvester.opencode.session_dir`, discovers all sessions, sorts messages chronologically, and converts each to a markdown document.
- FR-2: The harvester reads Claude Code session files from `harvester.claude_code.session_dir` when `harvester.claude_code.enabled = true`.
- FR-3: Each session document is identified by SHA-256 hash of rendered content. Unchanged sessions are skipped. Dedup state persists across restarts.
- FR-4: Harvested sessions are inserted into the `sessions` collection and immediately available for search after the next embedding cycle.
- FR-5: Harvest poll interval configurable via `intervals.session_poll` (default 120s).
- FR-6: The harvester does not acquire any global lock. Concurrent harvesters on the same PostgreSQL produce no duplicates or corruption.
- FR-7: `POST /api/query` (and `memory_query` MCP tool) executes the full 7-stage hybrid pipeline. Required: `query`. Optional: `workspace`, `tags`, `limit`, `scope`.
- FR-8: `POST /api/search` (and `memory_search` MCP tool) executes BM25 only.
- FR-9: `POST /api/vsearch` (and `memory_vsearch` MCP tool) executes vector search only. Returns empty result set (not error) if no embeddings exist.
- FR-10: All three search endpoints return consistent schema: `{results: [{id, title, snippet, score, tags, collection, workspace_hash, created_at, updated_at}], total, query_ms}`.
- FR-11: Search results scoped to workspace identified by `workspace` parameter. `scope=all` merges all workspaces via RRF.
- FR-12: Workspace isolation invariant: no query scoped to workspace A returns document from workspace B. Enforced at database query layer, not post-filter.
- FR-13: Search requests complete within 3 seconds for up to 100,000 documents per workspace.
- FR-14: RRF fusion parameter k configurable via `search.rrf_k` (default 60).
- FR-15: Recency boost weight and half-life configurable via `search.recency_weight` (0.3) and `search.recency_half_life_days` (180).
- FR-16: Every write operation stores workspace hash as non-nullable partition key.
- FR-17: Every read operation filters by workspace hash in WHERE clause, not post-processing.
- FR-18: Creating workspace requires only `nano-brain init --root`. System derives hash and provisions metadata.
- FR-19: `GET /api/v1/workspaces` returns all registered workspaces with hashes, document counts, and last-updated timestamps.
- FR-20: `nano-brain collection {add|remove|list|rename}` manages collections within current workspace scope only.
- FR-21: Cross-workspace queries (`scope=all`) supported on read-only operations only.
- FR-22: File watcher monitors all collection directories. On change event, sets dirty flag for that collection.
- FR-23: Dirty collections reindexed after debounce interval (default 2000ms). Multiple rapid changes trigger single reindex.
- FR-24: Reindex is hash-based: unchanged files skipped. Only changed files re-chunked and re-embedded.
- FR-25: Periodic full-scan poll (configurable `watcher.reindex_interval`, default 300s) catches changes missed by event watcher.
- FR-26: File watcher runs as goroutine, does not block HTTP API.
- FR-27: `nano-brain collection add/remove/list/rename` manages collections. Watcher monitors new directory immediately without restart.
- FR-28: Storage limits enforced: `storage.max_file_size` (300KB), `storage.max_size` (10GB per workspace).
- FR-29: Embedding provider configured via `embedding.provider` and `embedding.url`. Ollama needs running instance. VoyageAI needs `VOYAGE_API_KEY`.
- FR-30: Embedding runs asynchronously in background goroutine. HTTP API and search remain available while embedding pending.
- FR-31: Embedding goroutine uses adaptive backoff on errors: 60s start, 1.5× multiplier, 300s max.
- FR-32: Embedding concurrency configurable via `embedding.concurrency` (default 3).
- FR-33: `POST /api/embed` triggers immediate embedding pass for up to 50 pending chunks. Returns `{embedded: N, remaining: M}`.
- FR-34: `nano-brain embed [--force]` triggers embedding via CLI. `--force` re-embeds all chunks.
- FR-35: `GET /api/status` includes embedding queue depth and active provider name.
- FR-36: Chunks failing embedding after 3 attempts marked `embed_failed`, excluded from vector search, available for BM25.
- FR-36b: Embedding queue has bounded in-memory capacity of 10,000 chunk IDs (configurable).
- FR-36c: Queue depth exceeds rejection threshold (50,000) → HTTP 503 + `Retry-After: 5` header. Chunks remain in PG.
- FR-36d: `GET /api/status` includes queue_depth, queue_capacity, embed_rate_per_sec, estimated_drain_seconds, queue_status.
- FR-36e: Structured log warnings at 60% capacity (warn) and 90% capacity (error).
- FR-37: `nano-brain bench generate --scale=N` generates labeled benchmark dataset of N query-answer pairs.
- FR-38: `nano-brain bench run --scale=N` computes P@5, R@10, MRR, insert p50/p95, query p50/p95 latency.
- FR-39: `nano-brain bench compare` flags regressions. Thresholds: P@5 drop >0.10, R@10 drop >0.10, MRR drop >0.05, latency >2×.
- FR-40: Benchmark results saved with `--save <path>`.
- FR-41: Concurrency test: N concurrent writers, verify zero data loss and no constraint violations.
- FR-42: Every mutation runs inside a database transaction. Failed step → entire transaction rollback.
- FR-43: Document and chunk inserts use upsert semantics (ON CONFLICT). Re-running ingestion is idempotent.
- FR-44: On startup, nano-brain runs schema version check via goose. Behind-version schemas auto-migrated.
- FR-45: `GET /health` returns `{status: "ok", ready: true}` only when PG pool healthy and migrations current.
- FR-46: Connection pool performs periodic health checks. Unhealthy connections auto-replaced.
- FR-47: `nano-brain status` reports PG pool health, active connections, pending migration count.
- FR-48: Documents chunked with target 3600 chars (~900 tokens) and 200-char overlap (~15%).
- FR-49: Chunker scores break points: H1 > H2 > H3/code-fence > H4-H6 > hr > blank > list > newline.
- FR-50: Chunker tracks open code fences, never cuts inside code block.
- FR-51: Chunks shorter than 200 chars merged with adjacent chunk.
- FR-52: Each chunk stored with position (sequence number, start line, end line).
- FR-53: Chunker is deterministic: same input → same chunks.
- FR-54: `GET /health` returns health and readiness. No authentication required.
- FR-55: `GET /api/status` returns full index health including PG status, embedding queue, active provider.
- FR-56: `POST /api/query` executes hybrid search.
- FR-57: `POST /api/search` executes BM25 search.
- FR-58: `POST /api/vsearch` executes vector search.
- FR-59: `POST /api/write` inserts new document. Request: `{content, tags?, collection?, workspace?}`.
- FR-60: `GET/POST /api/wake-up` returns compact session briefing.
- FR-61: `POST /api/reindex` triggers async reindex for collection or workspace.
- FR-62: `POST /api/update` triggers async reindex of all collections.
- FR-63: `POST /api/embed` triggers embedding pass.
- FR-64: `GET /api/v1/tags` returns all tags with document counts.
- FR-65: `GET /api/v1/workspaces` returns all registered workspaces.
- FR-66: `GET /sse` and `POST /messages` implement MCP SSE transport.
- FR-67: `GET /mcp` and `POST /mcp` implement MCP Streamable HTTP transport.
- FR-68: All workspace-data endpoints require workspace identifier. Missing → HTTP 400. No default fallback.
- FR-69: API version in every response header as `X-Nano-Brain-Version: <semver>`.
- FR-70: API accepts/returns JSON only. Non-JSON body → HTTP 415.
- FR-71: All 9 MCP tools registered on both SSE and Streamable HTTP transports.
- FR-72: Each MCP tool requires `workspace` parameter. Omitted → HTTP 400. `workspace: "all"` → cross-workspace query.
- FR-73: MCP server handles concurrent tool calls without race conditions.
- FR-74: Streamable HTTP uses 30-second SSE heartbeats.
- FR-75: `memory_get` accepts document path or `#docid` with optional `{start_line, end_line}` range.
- FR-72b: `memory_write` accepts optional `supersedes` parameter. Relationship recorded for Tier 2.
- FR-76: `nano-brain init --root <dir>` registers workspace, creates default collections, outputs AGENTS.md snippet.
- FR-77: `nano-brain query/search/vsearch <text>` invoke search and print formatted results.
- FR-78: `nano-brain write <text> [--tags]` writes document to daily log.
- FR-79: `nano-brain status` prints PG health, embedding queue, workspace count, collection stats.
- FR-80: `nano-brain harvest` triggers immediate harvest cycle.
- FR-81: `nano-brain docker start/stop/status` wraps docker compose commands.
- FR-82: `nano-brain db:migrate` runs pending goose migrations.
- FR-83: All CLI commands honor `NANO_BRAIN_HOST` and `NANO_BRAIN_PORT` env vars.
- FR-84: All CLI commands support `--json` output.
- FR-85: `nano-brain db:migrate --from-v1 <path>` reads v1 SQLite documents/chunks/tags and inserts into v2 PG. Embeddings regenerated separately.
- FR-86: Migration is idempotent via content-addressed upsert.
- FR-87: Migration reports progress: `Migrated N / M documents`.
- FR-88: Migration logs unparseable records and continues (no abort on single corrupt record).
- FR-89: Config from `~/.nano-brain/config.yml`. Overridable via `--config` or `NANO_BRAIN_CONFIG_PATH`.
- FR-90: Missing config file → generate from built-in defaults on first run.
- FR-91: All configurable values listed in PRD §4.13.
- FR-92: Environment variables override config file values.
- FR-93: Config errors → descriptive error + non-zero exit at startup.
- FR-93b: `POST /api/reload-config` hot-reloads safe settings. Returns reloaded/unchanged/requires_restart lists.
- FR-94: Logs written to `~/.nano-brain/logs/nano-brain-YYYY-MM-DD.log`. Rotation: 50MB max, 5 files.
- FR-95: Log levels: error, warn, info (default), debug. Configurable.
- FR-96: `nano-brain logs [-f] [-n N]` displays recent log output.
- FR-97: Search telemetry stored in database. 90-day retention (configurable).
- FR-98: No log data or telemetry transmitted externally.
- FR-99: Docker Compose: two services — nano-brain + PostgreSQL 17 with pgvector 0.8.2.
- FR-100: `docker compose up` → ready within 30 seconds.
- FR-101: Postgres data persisted to named Docker volume. Restart preserves data.
- FR-102: Container auto-migrates on startup.
- FR-103: Container health check calls `GET /health` every 30s. Unhealthy after 3 consecutive failures.
- FR-104: Server detects Docker environment and adjusts PG host to internal service name.
- FR-105: CLI `docker start/stop` wraps docker compose commands.

### NonFunctional Requirements

- NFR-1 (Concurrency Safety §8.1): All shared mutable state in PostgreSQL, not application memory. pgxpool is goroutine-safe. CI runs `go test -race`. Every transaction uses context.WithTimeout. Background goroutines communicate only via database.
- NFR-2 (Workspace Isolation §8.2): Binary, non-negotiable invariant. Every query includes WHERE workspace_hash. No default workspace fallback. Missing workspace → HTTP 400. Cross-workspace queries explicitly list authorized hashes.
- NFR-3 (Search Quality §8.3): v2 hybrid search must meet/exceed v1 quality metrics (P@5≥0.835, R@10≥0.970, MRR≥1.000). Benchmarking suite is the validation instrument.
- NFR-4 (Data Integrity §8.4): All inserts are atomic transactions. All inserts are idempotent via content-addressed upsert. Migrations forward-only. Database is a derivable cache.
- NFR-5 (Privacy §8.5): Ollama default (local, no outbound). VoyageAI opt-in. No telemetry transmitted externally.

### Additional Requirements

From Architecture document:

- AR-1: Go 1.23+ with `CGO_ENABLED=0` static binary. No starter template — `go mod init` from scratch.
- AR-2: PostgreSQL 17 + pgvector 0.8.2 (pinned `pgvector/pgvector:0.8.2-pg17`). HNSW vector index.
- AR-3: Echo v4 HTTP framework with centralized `HTTPErrorHandler`.
- AR-4: Official MCP Go SDK (`modelcontextprotocol/go-sdk` v1.5+) — SSE + Streamable HTTP as `http.Handler`.
- AR-5: errgroup + context for 3 background goroutines (watcher, harvester, embedder).
- AR-6: Manual constructor injection, consumer-side interfaces (Go idioms).
- AR-7: Multi-stage Dockerfile: golang:1.23 build → distroless runtime (~15MB).
- AR-8: sqlc for type-safe SQL generation. All DB access via sqlc-generated code.
- AR-9: goose v3 for SQL schema migrations. Migrations in `/migrations/` directory.
- AR-10: zerolog for structured JSON logging.
- AR-11: koanf v2 for YAML + env var config loading.
- AR-12: fsnotify v1 for OS-level file change notifications.
- AR-13: Hybrid migration (D12): migrate user data only from v1 SQLite → v2 PG using `modernc.org/sqlite` (pure Go, no CGO). Regenerate code index, embeddings, knowledge graph via v2 pipeline.
- AR-14: User-defined collections (D13): retain v1 collection system (Obsidian vault, custom folders). `collection add/remove/list/rename`.
- AR-15: Dev-in-container (D14): all dev/test inside container. PG+Ollama on host via `host.docker.internal`.
- AR-16: 14 `/internal/` packages with inward dependency flow. No circular dependencies.
- AR-17: Naming conventions: DB snake_case, Go PascalCase/camelCase, API snake_case.
- AR-18: `golangci-lint` + `go test -race` as CI gates.

### UX Design Requirements

No UX Design document exists. This is a backend/CLI project with no graphical user interface.

### FR Coverage Map

| FR | Epic | Brief Description |
|---|---|---|
| FR-42 | Epic 1 | Transactional mutations |
| FR-43 | Epic 1 | Upsert semantics (idempotent) |
| FR-44 | Epic 1 | Schema migration on startup (goose) |
| FR-45 | Epic 1 | Health endpoint (PG pool + migrations) |
| FR-46 | Epic 1 | Connection pool health checks |
| FR-47 | Epic 1 | CLI status: PG health, connections, migrations |
| FR-54 | Epic 1 | GET /health (no auth) |
| FR-55 | Epic 1 | GET /api/status (full index health) |
| FR-89 | Epic 1 | Config from ~/.nano-brain/config.yml |
| FR-90 | Epic 1 | Default config generation on first run |
| FR-91 | Epic 1 | All configurable values from PRD §4.13 |
| FR-92 | Epic 1 | Env vars override config file |
| FR-93 | Epic 1 | Config errors → descriptive error + exit |
| FR-94 | Epic 1 | Log file rotation (50MB, 5 files) |
| FR-95 | Epic 1 | Log levels: error/warn/info/debug |
| FR-99 | Epic 1 | Docker Compose: nano-brain + PG 17 + pgvector |
| FR-100 | Epic 1 | docker compose up → ready ≤30s |
| FR-101 | Epic 1 | PG data on named Docker volume |
| FR-102 | Epic 1 | Container auto-migrates on startup |
| FR-103 | Epic 1 | Container health check every 30s |
| FR-104 | Epic 1 | Docker env detection for PG host |
| FR-105 | Epic 1 | CLI docker start/stop wraps compose |
| FR-16 | Epic 2 | Write ops store workspace_hash (non-nullable) |
| FR-17 | Epic 2 | Read ops filter by workspace_hash in WHERE |
| FR-18 | Epic 2 | nano-brain init --root registers workspace |
| FR-19 | Epic 2 | GET /api/v1/workspaces lists workspaces |
| FR-22 | Epic 2 | File watcher monitors collection dirs |
| FR-23 | Epic 2 | Debounced reindex (2000ms) |
| FR-24 | Epic 2 | Hash-based reindex (skip unchanged) |
| FR-25 | Epic 2 | Periodic full-scan poll (300s) |
| FR-26 | Epic 2 | File watcher as non-blocking goroutine |
| FR-27 | Epic 2 | Collection add/remove/list/rename + live watch |
| FR-28 | Epic 2 | Storage limits (300KB file, 10GB workspace) |
| FR-48 | Epic 2 | Chunker: 3600 chars target, 200 overlap |
| FR-49 | Epic 2 | Chunker: scored break points (H1>H2>...) |
| FR-50 | Epic 2 | Chunker: no cut inside code blocks |
| FR-51 | Epic 2 | Chunker: merge short chunks (<200 chars) |
| FR-52 | Epic 2 | Chunk position tracking (seq, start/end line) |
| FR-53 | Epic 2 | Chunker deterministic |
| FR-59 | Epic 2 | POST /api/write inserts document |
| FR-68 | Epic 2 | Workspace required on data endpoints → 400 |
| FR-69 | Epic 2 | X-Nano-Brain-Version header |
| FR-70 | Epic 2 | JSON only (415 on non-JSON) |
| FR-76 | Epic 2 | CLI init --root |
| FR-77 | Epic 2 | CLI query/search/vsearch (stub, full in Epic 4) |
| FR-78 | Epic 2 | CLI write --tags |
| FR-83 | Epic 2 | CLI honors NANO_BRAIN_HOST/PORT |
| FR-84 | Epic 2 | CLI --json output |
| FR-9 | Epic 3 | POST /api/vsearch (vector only) |
| FR-29 | Epic 3 | Embedding provider config (Ollama/VoyageAI) |
| FR-30 | Epic 3 | Async embedding goroutine |
| FR-31 | Epic 3 | Adaptive backoff (60s start, 1.5×, 300s max) |
| FR-32 | Epic 3 | Embedding concurrency config (default 3) |
| FR-33 | Epic 3 | POST /api/embed (immediate pass, 50 chunks) |
| FR-34 | Epic 3 | CLI embed [--force] |
| FR-35 | Epic 3 | GET /api/status includes embed queue |
| FR-36 | Epic 3 | embed_failed after 3 attempts |
| FR-36b | Epic 3 | Bounded queue (10K capacity) |
| FR-36c | Epic 3 | Queue overflow → HTTP 503 + Retry-After |
| FR-36d | Epic 3 | Status: queue_depth, capacity, rate, ETA |
| FR-36e | Epic 3 | Structured log warnings at 60%/90% |
| FR-58 | Epic 3 | POST /api/vsearch endpoint |
| FR-63 | Epic 3 | POST /api/embed endpoint |
| FR-7 | Epic 4 | POST /api/query (7-stage hybrid pipeline) |
| FR-8 | Epic 4 | POST /api/search (BM25 only) |
| FR-10 | Epic 4 | Consistent result schema across endpoints |
| FR-11 | Epic 4 | Workspace-scoped search + scope=all via RRF |
| FR-12 | Epic 4 | Workspace isolation invariant (DB-level) |
| FR-13 | Epic 4 | Search ≤3s for 100K docs |
| FR-14 | Epic 4 | RRF k configurable (default 60) |
| FR-15 | Epic 4 | Recency boost weight + half-life config |
| FR-56 | Epic 4 | POST /api/query endpoint |
| FR-57 | Epic 4 | POST /api/search endpoint |
| FR-60 | Epic 4 | GET/POST /api/wake-up (session briefing) |
| FR-66 | Epic 5 | MCP SSE transport (GET /sse, POST /messages) |
| FR-67 | Epic 5 | MCP Streamable HTTP (GET /mcp, POST /mcp) |
| FR-71 | Epic 5 | 9 MCP tools on both transports |
| FR-72 | Epic 5 | MCP tools require workspace → 400 |
| FR-73 | Epic 5 | Concurrent MCP tool calls (no races) |
| FR-74 | Epic 5 | Streamable HTTP 30s heartbeats |
| FR-75 | Epic 5 | memory_get: path or #docid with line range |
| FR-72b | Epic 5 | memory_write: optional supersedes param |
| FR-1 | Epic 6 | OpenCode session harvester |
| FR-2 | Epic 6 | Claude Code session harvester |
| FR-3 | Epic 6 | SHA-256 dedup, persists across restarts |
| FR-4 | Epic 6 | Sessions → sessions collection, searchable |
| FR-5 | Epic 6 | Harvest poll interval config (120s) |
| FR-6 | Epic 6 | No global lock, concurrent-safe |
| FR-80 | Epic 6 | CLI harvest (immediate cycle) |
| FR-37 | Epic 7 | bench generate --scale=N |
| FR-38 | Epic 7 | bench run: P@5, R@10, MRR, latency |
| FR-39 | Epic 7 | bench compare: regression thresholds |
| FR-40 | Epic 7 | bench results --save |
| FR-41 | Epic 7 | Concurrency test: N writers, zero loss |
| FR-85 | Epic 8 | db:migrate --from-v1 (SQLite → PG) |
| FR-86 | Epic 8 | Migration idempotent (upsert) |
| FR-87 | Epic 8 | Migration progress reporting |
| FR-88 | Epic 8 | Migration logs + continues on corrupt records |
| FR-93b | Epic 8 | POST /api/reload-config (hot reload) |
| FR-96 | Epic 8 | CLI logs [-f] [-n N] |
| FR-97 | Epic 8 | Search telemetry (90-day retention) |
| FR-98 | Epic 8 | No external telemetry |
| FR-20 | Epic 8 | CLI collection add/remove/list/rename |
| FR-21 | Epic 8 | Cross-workspace scope=all (read-only) |
| FR-61 | Epic 8 | POST /api/reindex (async) |
| FR-62 | Epic 8 | POST /api/update (reindex all) |
| FR-64 | Epic 8 | GET /api/v1/tags (with counts) |
| FR-65 | Epic 8 | GET /api/v1/workspaces |
| FR-69 | Epic 8 | X-Nano-Brain-Version header (shared with Epic 2) |
| FR-79 | Epic 8 | CLI status (full) |
| FR-81 | Epic 8 | CLI docker start/stop/status |
| FR-82 | Epic 8 | CLI db:migrate |

**Coverage summary:** 111 FRs → 8 Epics. All FRs mapped. 5 NFRs enforced across all epics. 18 ARs applied as implementation constraints.

## Epic List

### Epic 1: Foundation & Data Layer
The nano-brain server starts, connects to PostgreSQL, runs migrations, responds to health checks, and loads configuration. Developer can `docker compose up` and get a working (empty) instance with health monitoring and structured logging.

**FRs covered:** FR-42, FR-43, FR-44, FR-45, FR-46, FR-47, FR-54, FR-55, FR-89, FR-90, FR-91, FR-92, FR-93, FR-94, FR-95, FR-99, FR-100, FR-101, FR-102, FR-103, FR-104, FR-105
**ARs applied:** AR-1, AR-2, AR-3, AR-5, AR-6, AR-7, AR-8, AR-9, AR-10, AR-11, AR-16, AR-17, AR-18
**NFRs enforced:** NFR-1 (concurrency-safe pool), NFR-4 (transactional mutations), NFR-5 (no external telemetry)

### Epic 2: Ingestion Pipeline
User can register a workspace, ingest documents via API and CLI, chunk them with the markdown-aware chunker, and manage collections. File watcher keeps content current automatically with debounced, hash-based reindexing.

**FRs covered:** FR-16, FR-17, FR-18, FR-19, FR-22, FR-23, FR-24, FR-25, FR-26, FR-27, FR-28, FR-48, FR-49, FR-50, FR-51, FR-52, FR-53, FR-59, FR-68, FR-69, FR-70, FR-76, FR-77, FR-78, FR-83, FR-84
**ARs applied:** AR-5 (watcher goroutine), AR-8 (sqlc), AR-12 (fsnotify), AR-14 (collections), AR-17
**NFRs enforced:** NFR-2 (workspace isolation on every write/read), NFR-4 (idempotent upserts)

### Epic 3: Embedding & Vector Search
Documents get embedded asynchronously via Ollama or VoyageAI. Bounded queue with backpressure. Vector search endpoint returns results. Embedding status visible in API and CLI.

**FRs covered:** FR-9, FR-29, FR-30, FR-31, FR-32, FR-33, FR-34, FR-35, FR-36, FR-36b, FR-36c, FR-36d, FR-36e, FR-58, FR-63
**ARs applied:** AR-2 (pgvector HNSW), AR-5 (embedder goroutine), AR-8 (sqlc)
**NFRs enforced:** NFR-1 (goroutine-safe queue), NFR-5 (Ollama default, VoyageAI opt-in)

### Epic 4: Hybrid Search
Full hybrid search pipeline: BM25 + vector search in parallel, RRF fusion, recency boost. Three search endpoints with consistent schema. Wake-up briefing. Search completes in <3s at scale.

**FRs covered:** FR-7, FR-8, FR-10, FR-11, FR-12, FR-13, FR-14, FR-15, FR-56, FR-57, FR-60
**ARs applied:** AR-8 (sqlc for BM25 queries)
**NFRs enforced:** NFR-2 (workspace isolation in all queries), NFR-3 (search quality ≥ v1)

### Epic 5: MCP Integration
AI agents connect via MCP SSE and Streamable HTTP transports. All 9 tools registered and functional. Concurrent tool calls handled safely. Workspace required on every tool call.

**FRs covered:** FR-66, FR-67, FR-71, FR-72, FR-73, FR-74, FR-75, FR-72b
**ARs applied:** AR-4 (MCP Go SDK v1.5+)
**NFRs enforced:** NFR-1 (concurrent tool calls), NFR-2 (workspace on every tool)

### Epic 6: Session Harvesting
OpenCode and Claude Code sessions auto-harvested on configurable interval. SHA-256 dedup ensures no duplicates. Sessions inserted into collections and searchable after next embedding cycle.

**FRs covered:** FR-1, FR-2, FR-3, FR-4, FR-5, FR-6, FR-80
**ARs applied:** AR-5 (harvester goroutine)
**NFRs enforced:** NFR-1 (concurrent harvesters safe), NFR-4 (dedup via content hash)

### Epic 7: Benchmarking Suite
Generate labeled benchmark datasets, measure search quality (P@5, R@10, MRR) and latency, compare results across runs, detect regressions. Concurrency stress test validates zero data loss.

**FRs covered:** FR-37, FR-38, FR-39, FR-40, FR-41
**NFRs enforced:** NFR-3 (quality validation instrument), NFR-1 (concurrency test)

### Epic 8: V1 Migration & Operations
Existing v1 users migrate data from SQLite to PG. Operations CLI: logs, docker commands, config hot-reload, collection management, tags endpoint, telemetry.

**FRs covered:** FR-85, FR-86, FR-87, FR-88, FR-93b, FR-96, FR-97, FR-98, FR-20, FR-21, FR-61, FR-62, FR-64, FR-65, FR-69, FR-79, FR-81, FR-82
**ARs applied:** AR-13 (modernc.org/sqlite for v1 migration)
**NFRs enforced:** NFR-4 (idempotent migration), NFR-5 (no external telemetry)

### Dependency Flow

```
Epic 1 (Foundation) ──→ Epic 2 (Ingestion) ──→ Epic 3 (Embedding) ──→ Epic 4 (Hybrid Search)
                                                                              │
                                                                              ├──→ Epic 5 (MCP)
                                                                              ├──→ Epic 7 (Benchmarking)
                                                                              │
                                            Epic 2 ──→ Epic 6 (Session Harvesting)
                                            Epic 2 + Epic 3 ──→ Epic 8 (Migration & Ops)
```

Each epic is standalone: Epic 2 delivers ingestion value without requiring Epic 3. Epic 3 delivers vector search without requiring Epic 4. Epic 5 requires Epic 4 for search endpoints. Epic 6 requires Epic 2 for ingestion. Epic 7 requires Epic 4 for search. Epic 8 requires Epic 2 + Epic 3 for migration target.
