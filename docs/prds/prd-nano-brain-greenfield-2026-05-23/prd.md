---
title: "nano-brain v2 — Greenfield Rewrite PRD"
status: approved
created: 2026-05-23
updated: 2026-05-23
resolved: 2026-05-23
---

# nano-brain v2 — Product Requirements Document

**Version:** 0.2.0-draft
**Status:** Approved — ready for architecture and epic derivation. All open questions resolved 2026-05-23.
**North Star:** Reliability / Correctness
**Source documents:** `docs/briefs/brief-nano-brain-greenfield-2026-05-22/brief.md`, `docs/research/technical-nano-brain-v2-stack-selection-research-2026-05-22.md`, `docs/reference-prd.md`

---

## 0. Document Purpose

This PRD defines what nano-brain v2 must do, not how it does it. Architecture decisions (Go packages, SQL schemas, goroutine patterns) belong in the architecture document. This document is the decision gate before architecture and epic derivation begin.

Tier 1 features described here are the MVP v2.0 release scope. Tier 2 features are listed in §6.2 with rationale for deferral. Every assumption is tagged `[ASSUMPTION: ...]` inline and indexed in §13.

---

## 1. Vision

nano-brain v2 is the memory layer that AI coding agents can actually trust. Not "usually works," but provably correct: workspace isolation holds as a hard invariant, concurrent writes from multiple containers never corrupt data, and search results always come from the workspace they're supposed to come from.

v1 proved the concept. v2 makes the infrastructure worthy of it. Reliable agent memory compounds — an agent with a trustworthy memory of past decisions, code patterns, and project context performs meaningfully better on long-lived projects than one starting cold each session.

The correctness-first constraint shapes every decision. PostgreSQL over SQLite because MVCC eliminates the corruption class. HTTP over stdio because stateless transport removes lifecycle bugs. Benchmarking as a first-class shipped feature because correctness is testable, not assumed.

---

## 2. Target User

### 2.1 Primary Persona

**The solo developer running AI coding agents on long-lived projects.**

- Works in a single codebase across weeks or months.
- Uses OpenCode, Claude Code, or a similar MCP-capable agent daily.
- Wants the agent to accumulate context across sessions instead of starting cold.
- Comfortable running Docker Compose locally or on a personal dev server.
- Has no interest in cloud dependencies or managed services.
- Accepts that nano-brain is a developer tool, not an end-user product.

**Secondary persona:** A small team running parallel AI agent workflows. Multiple containers, each running an agent on its own task, sharing a single nano-brain instance. This is the scenario that broke v1 most visibly. v2 is explicitly designed for it.

### 2.2 Jobs to Be Done

| Job | Current pain | v2 outcome |
|---|---|---|
| Recall a past decision without re-reading the codebase | Agent starts cold; re-derives context from scratch | `memory_query` returns the decision, file reference, and date in one call |
| Run multiple agent containers in parallel without data corruption | SQLite corrupts under concurrent writes | PostgreSQL MVCC prevents corruption by architecture, not by luck |
| Ensure agent queries only see data from the current project | Workspace routing bug returns data from wrong workspace | Workspace isolation is a tested invariant, not a configuration assumption |
| Index a workspace once and keep it current | Manual reindex required after every session | File watcher triggers incremental reindex automatically |
| Start nano-brain on a new machine | Complex setup: SQLite, Qdrant, Node.js, npm | `docker compose up` — one command, working instance |
| Compare search quality after a config change | No baseline to compare against | `bench run` + `bench compare` give P@5, R@10, MRR deltas |

### 2.3 Non-Users

These users are out of scope. Design decisions should not accommodate them at the cost of primary persona needs.

- **Hosted/managed service users.** nano-brain is self-hosted by design. Multi-tenant SaaS packaging is deferred.
- **Consumer-facing product builders.** multi-tenant memory at scale requires a different architecture.
- **GUI-first users.** nano-brain is API-first and CLI-first. A web UI may ship in Tier 2, but the API always takes precedence.
- **Mobile/cross-platform client developers.** Out of scope entirely.

### 2.4 Key User Journeys

**UJ-1: First-time setup**
User runs `docker compose up` in the nano-brain directory. nano-brain and PostgreSQL start. User runs `nano-brain init --root /path/to/project`. Collections are created, file watcher starts, session harvesting begins. User confirms via `nano-brain status`.

**UJ-2: Agent session start**
Agent calls `memory_wake_up`. Receives a compact briefing (recent decisions, active files, relevant past context) in under 2 seconds. Uses this as session preamble without reading raw logs.

**UJ-3: Parallel container writes**
Three agent containers each write memory entries simultaneously. All writes succeed. No deadlocks. No corruption. Each container's subsequent reads return only its own workspace data.

**UJ-4: Hybrid search**
Agent calls `memory_query` with a natural-language question. System runs BM25 + vector search in parallel, fuses with RRF, applies recency boost, returns top-10 results with snippets. Agent receives results ranked by relevance.

**UJ-5: Search quality validation**
Developer runs `nano-brain bench generate --scale=500` to generate a labeled dataset, then `nano-brain bench run --scale=500` to measure P@5, R@10, MRR and latency. Compares against saved v1 baseline with `nano-brain bench compare`. Confirms v2 meets or exceeds v1 quality before release.

**UJ-6: Migration from v1**
Existing v1 user runs `nano-brain db:migrate`. Tool reads v1 data, transforms to v2 schema, imports into PostgreSQL. User confirms stored memories are queryable in v2 before removing v1.

**UJ-7: Embedding provider switch**
User changes embedding provider from Ollama to VoyageAI in config. Runs `nano-brain reindex` to regenerate all embeddings. System re-embeds all documents using the new provider without data loss.

---

## 3. Glossary

| Term | Definition |
|---|---|
| **Workspace** | A project directory that nano-brain tracks. Each workspace is isolated: queries scoped to workspace A never return data from workspace B. Identified by a hash of the directory path. |
| **Workspace hash** | A stable identifier derived from the workspace root path. Used as a partition key in all database operations. |
| **Collection** | A named logical grouping of documents within a workspace (e.g., `memory`, `sessions`, user-defined). |
| **Chunk** | A heading-aware slice of a document produced by the chunker. Target size ~900 tokens, ~15% overlap with adjacent chunks. Each chunk is content-addressed by SHA-256. |
| **Content-addressed storage** | Storage where each blob is keyed by SHA-256(content). Identical content is stored once and referenced by hash. Enables deduplication across documents. |
| **Embedding** | A fixed-length floating-point vector that captures the semantic meaning of a chunk. Generated by an embedding provider (Ollama or VoyageAI). Stored in PostgreSQL via pgvector. |
| **Embedding provider** | A service that converts text to embedding vectors. v2 supports Ollama (local) and VoyageAI (cloud). |
| **BM25** | Probabilistic term-frequency ranking function used for full-text search. Implemented via PostgreSQL `tsvector`/`tsquery`. |
| **Vector search** | Nearest-neighbor search over embedding vectors using cosine similarity. Implemented via pgvector. |
| **RRF** | Reciprocal Rank Fusion. Merges ranked result lists from heterogeneous search signals: `score = Σ weight / (k + rank + 1)`, default k=60. |
| **Hybrid search** | The combination of BM25 + vector search fused via RRF. The MVP search pipeline. |
| **Recency boost** | A score multiplier that favors recently written or updated documents. Applied after RRF fusion. |
| **Supersede** | A document marked as replaced by a newer document. v2 tracks supersede relationships but does not apply supersede demotion in the MVP search pipeline (deferred to Tier 2). |
| **Harvester** | Background job that reads AI agent session files (OpenCode, Claude Code JSON format) and converts them to searchable markdown documents ingested into nano-brain. |
| **File watcher** | Background process that monitors collection directories for file changes using fsnotify (Go). Triggers incremental reindex on detected changes. |
| **Session briefing** | A compact summary of recent memory and context, generated on-demand for an agent at session start. Returned by `memory_wake_up`. |
| **MCP** | Model Context Protocol. JSON-RPC protocol for AI agent tool exposure. v2 uses MCP-over-HTTP (SSE and Streamable HTTP). MCP stdio is removed. |
| **MCP-over-HTTP** | MCP transport using HTTP rather than stdio. Supports Server-Sent Events (SSE) for streaming and Streamable HTTP with session IDs. |
| **pgvector** | PostgreSQL extension that adds vector storage and nearest-neighbor search operators. Replaces Qdrant and sqlite-vec from v1. |
| **Benchmarking suite** | Developer-facing tool for measuring search quality (P@5, R@10, MRR) and concurrency correctness. Ships as part of the v2.0 release. |
| **Workspace isolation invariant** | The guarantee that no read or write operation on workspace A can access, modify, or return data from workspace B. Binary: either it holds for all cases, or v2 is not ready. |
| **MVCC** | Multi-Version Concurrency Control. PostgreSQL's concurrency model. Provides transaction isolation without the file-locking issues that caused SQLite corruption in v1. |
| **Migration** | The process of moving v1 data (SQLite) into v2 (PostgreSQL). A working migration path is a v2.0 release gate. |
| **goose** | SQL migration tool for Go. Manages the PostgreSQL schema version history. |
| **Docker Compose** | Container orchestration tool. The v2 deployment model: `docker compose up` starts nano-brain and PostgreSQL together. |

---

## 4. Features

### 4.1 Session Harvesting

**Behavior:** nano-brain automatically reads AI agent session files from the host filesystem and ingests them as searchable documents without any manual triggering. Agents do not need to call a write API to store session context — harvesting is a background push from the filesystem.

Session sources are OpenCode (JSON session files in the OpenCode storage directory) and Claude Code (JSON session files at a configured path). [ASSUMPTION: A-1] The harvester converts each session to markdown using a consistent format (front-matter with session metadata, messages as markdown), deduplicates by content hash so re-harvesting the same session is idempotent, and writes the result to the `sessions` collection.

Harvesting runs on a configurable poll interval [ASSUMPTION: A-2] and does not block the HTTP API or search operations.

**Functional Requirements:**

- **FR-1** The harvester reads OpenCode session files from the path configured in `harvester.opencode.session_dir`. It discovers all sessions, sorts messages chronologically, and converts each to a markdown document. Applies to UJ-1, UJ-2.
- **FR-2** The harvester reads Claude Code session files from the path configured in `harvester.claude_code.session_dir` when `harvester.claude_code.enabled = true`.
- **FR-3** Each session document is identified by a SHA-256 hash of its rendered content. If the hash is unchanged since the last harvest run, the session is skipped. The dedup state is persisted across restarts.
- **FR-4** Harvested sessions are inserted into the `sessions` collection and are immediately available for search after the next embedding cycle.
- **FR-5** The harvest poll interval is configurable via `intervals.session_poll` (default 120 s). Applies to UJ-3.
- **FR-6** The harvester does not acquire any global lock. Concurrent writes from multiple harvester instances on the same PostgreSQL instance must not produce duplicates or data corruption. Applies to UJ-3.

**Testable consequences:** A session written to the OpenCode session directory appears in search results within `intervals.session_poll + embedding_cycle_interval` seconds. Re-running harvest on an unchanged session directory produces no new documents. Two harvesters running simultaneously on the same PostgreSQL instance produce the same document count as one.

**Out of scope for FR-1 through FR-6:** LLM fact extraction from sessions, entity/relationship building from harvested sessions (Tier 2).

---

### 4.2 Hybrid Search

**Behavior:** Agents can retrieve relevant memory using three query modes: BM25 keyword search, vector semantic search, and hybrid (BM25 + vector + RRF). The hybrid pipeline is the primary recommended path. All three modes return ranked results with snippet previews.

The v2 MVP search pipeline has 7 stages:
1. Query input (text, workspace scope, optional tag filters)
2. BM25 search via PostgreSQL `tsvector`/`tsquery`
3. Vector similarity search via pgvector cosine distance
4. RRF fusion (k=60) merging results from stages 2 and 3
5. Recency boost applied to fused scores
6. Top-K selection (default limit=10)
7. Output with snippet preview (max 700 chars per result)

[ASSUMPTION: A-3] Stages that exist in v1 but are not in the v2 MVP pipeline — query expansion, centrality boost, supersede demotion, usage boost, length normalization, category weights, importance boost, neural reranking, position-aware blending — are deferred to Tier 2.

**Functional Requirements:**

- **FR-7** `POST /api/query` (and `memory_query` MCP tool) executes the full 7-stage hybrid pipeline. Required parameters: `query` (string). Optional: `workspace` (hash or path), `tags` (array), `limit` (integer, default 10), `scope` ("current" or "all"). Applies to UJ-4.
- **FR-8** `POST /api/search` (and `memory_search` MCP tool) executes BM25 only. Same parameter shape as FR-7 minus `scope=all`.
- **FR-9** `POST /api/vsearch` (and `memory_vsearch` MCP tool) executes vector search only. Requires at least one embedding to exist in the target workspace; returns an empty result set (not an error) if none exist yet.
- **FR-10** All three search endpoints return results in a consistent schema: `{results: [{id, title, snippet, score, tags, collection, workspace_hash, created_at, updated_at}], total, query_ms}`.
- **FR-11** Search results are always scoped to the workspace identified by the `workspace` parameter. When no workspace is specified, the workspace defaults to the one associated with the request's API key or connection context. [ASSUMPTION: A-4] When `scope=all`, results from all workspaces are merged via RRF. Applies to UJ-3, UJ-4.
- **FR-12** The workspace isolation invariant holds: no query scoped to workspace A returns a document belonging to workspace B. This is enforced at the database query layer, not by post-filter. Applies to UJ-3.
- **FR-13** Search requests complete within 3 seconds for up to 100,000 documents per workspace. [ASSUMPTION: A-5] See §10 for latency budgets.
- **FR-14** RRF fusion parameter k is configurable via `search.rrf_k` (default 60).
- **FR-15** Recency boost weight and half-life are configurable via `search.recency_weight` (default 0.3) and `search.recency_half_life_days` (default 180).

**Testable consequences:** A `memory_query` call returns results only from the workspace in the request. BM25 results match exact keyword queries. Vector results return semantically similar documents even with no shared keywords. Hybrid results rank the union of BM25 and vector candidates. Scores are consistent: running the same query twice returns the same ranking. Applies to UJ-4, UJ-5.

---

### 4.3 Per-Workspace Isolation

**Behavior:** Every document, chunk, embedding, and metadata record is associated with exactly one workspace. Queries, writes, and reindex operations are always scoped to a workspace. The isolation invariant — that workspace A never sees workspace B's data — is enforced at the storage layer, not at the application filter layer.

[ASSUMPTION: A-6] The workspace is identified by a hash derived from the workspace root directory path. The hash is stable: the same directory path always produces the same workspace hash.

**Functional Requirements:**

- **FR-16** Every write operation (document insert, chunk insert, embedding insert) stores the workspace hash as a non-nullable partition key. Applies to UJ-3.
- **FR-17** Every read operation (search, get, tags, status) filters by workspace hash before returning results. The filter is applied in the database query (WHERE clause), not in application-layer post-processing.
- **FR-18** Creating a new workspace requires no user action beyond specifying a root directory path in `nano-brain init --root`. The system derives the workspace hash and provisions the necessary metadata.
- **FR-19** `GET /api/v1/workspaces` returns a list of all registered workspaces with their hashes, document counts, and last-updated timestamps.
- **FR-20** `nano-brain collection {add|remove|list|rename}` manages collections within the current workspace scope only.
- **FR-21** Cross-workspace queries (`scope=all`) are supported on read-only operations only. Write operations always target a single workspace. Applies to UJ-2, UJ-4.

**Testable consequences:** Inserting document D into workspace A, then querying workspace B, never returns D. Inserting the same content into two workspaces produces two separate documents. The workspace list endpoint returns the correct document count for each workspace independently. Applies to UJ-3.

---

### 4.4 File Watcher and Collection Scanning

**Behavior:** nano-brain monitors configured collection directories and automatically reindexes changed files without manual intervention. The file watcher uses fsnotify (Go). Changes are debounced to avoid redundant reindex cycles triggered by rapid sequential writes.

[ASSUMPTION: A-7] The default collections are `memory` (user-written notes, `**/*.md` glob) and `sessions` (harvested session files). Users can add custom collections via CLI.

**Functional Requirements:**

- **FR-22** The file watcher monitors all directories registered as collections for a workspace. On detecting a file change event (create, modify, delete), it sets a dirty flag for that collection.
- **FR-23** Dirty collections are reindexed after a debounce interval (default 2000 ms). Multiple rapid changes to the same collection trigger a single reindex, not N reindexes.
- **FR-24** Reindex is hash-based: files whose SHA-256 hash has not changed since the last index are skipped. Only changed files are re-chunked and re-embedded.
- **FR-25** A periodic full-scan poll (configurable via `watcher.reindex_interval`, default 300 s) catches any changes missed by the event-based watcher (e.g., files modified while the watcher was not running).
- **FR-26** File watcher runs as a goroutine and does not block the HTTP API. Applies to UJ-1.
- **FR-27** `nano-brain collection add --path <dir> --name <name>` adds a new collection to the current workspace. The watcher begins monitoring the new directory immediately without restart. `collection remove` stops watching the directory. `collection list` shows all active collections. `collection rename` changes the collection name without data loss.
- **FR-28** Storage limits are enforced during scan: documents exceeding `storage.max_file_size` (default 300 KB) are skipped. Total workspace storage is capped at `storage.max_size` (default 10 GB). Applies to UJ-1.

**Testable consequences:** Creating a new markdown file in a watched collection causes it to appear in search results within `debounce_ms + embedding_cycle_interval` seconds. Modifying a file re-chunks and re-embeds only that file. Deleting a file removes it from search results. Adding a collection and writing to it without restart produces searchable documents.

---

### 4.5 Embedding Providers

**Behavior:** Embeddings convert text chunks into semantic vectors for vector search. v2 supports two embedding providers: Ollama (local, no API key required) and VoyageAI (cloud, requires API key). The active provider is selected via configuration. [ASSUMPTION: A-8] All documents in a workspace must use embeddings from the same provider; switching providers requires a full reindex.

[ASSUMPTION: A-9] The default provider is Ollama with `nomic-embed-text`, which runs entirely locally with no outbound network calls. VoyageAI with `voyage-code-3` is the recommended cloud option.

**Functional Requirements:**

- **FR-29** The embedding provider is configured via `embedding.provider` (`ollama` or `openai`) and `embedding.url`. The Ollama provider requires only a running Ollama instance. The VoyageAI provider requires `VOYAGE_API_KEY`.
- **FR-30** Embedding runs asynchronously in a background goroutine. New chunks are queued for embedding; the queue drains at the rate the provider allows. HTTP API and search remain available while embedding is pending.
- **FR-31** The embedding goroutine uses adaptive backoff on provider errors: start at 60 s, multiply by 1.5 on each consecutive failure, max 300 s. On recovery, return to the base interval.
- **FR-32** Embedding concurrency (number of parallel requests to the provider) is configurable via `embedding.concurrency` (default 3).
- **FR-33** `POST /api/embed` triggers an immediate embedding pass for up to 50 pending chunks. Returns `{embedded: N, remaining: M}`.
- **FR-34** `nano-brain embed [--force]` triggers the same embedding pass via CLI. `--force` re-embeds all chunks, not just pending ones, enabling a full reindex after a provider switch. Applies to UJ-7.
- **FR-35** `GET /api/status` includes embedding queue depth (pending count) and the active provider name. Applies to UJ-7.
- **FR-36** Chunks that fail embedding after 3 attempts are marked as `embed_failed` and excluded from vector search until re-embedded. They remain available for BM25 search.
- **FR-36b** The embedding queue has a bounded in-memory capacity of 10,000 pending chunk IDs (configurable via `embedding.queue_capacity`). Memory footprint: ~1 MB at capacity.
- **FR-36c** When the queue depth exceeds the rejection threshold (default 50,000, configurable via `embedding.queue_max_depth`), new enqueue requests are rejected with HTTP 503 and a `Retry-After: 5` header. Chunks remain in PostgreSQL for later re-enqueuing; no data is lost.
- **FR-36d** `GET /api/status` includes embedding queue fields: `queue_depth`, `queue_capacity`, `embed_rate_per_sec`, `estimated_drain_seconds`, and `queue_status` (one of: `nominal`, `busy`, `backpressure`, `rejecting`). This is the primary mechanism for users and CLI to monitor queue health.
- **FR-36e** Structured log warnings are emitted when queue depth exceeds 60% capacity (warn) and 90% capacity (error). Log entries include queue depth, drain rate, and estimated drain time.

**Testable consequences:** After configuring Ollama and calling `POST /api/embed`, documents with pending embeddings get embeddings and become available for vector search. Switching provider, running `embed --force`, and querying returns consistent results under the new provider. When the embedding provider is unreachable, BM25 search still works and the API does not crash.

---

### 4.6 Benchmarking Suite

**Behavior:** The benchmarking suite is a first-class shipped tool, not an afterthought. It enables a developer to measure search quality (P@5, R@10, MRR) and concurrency correctness on a local instance, compare against a saved baseline, and confirm that v2 meets or exceeds v1 quality before release. The suite ships as part of v2.0 because one of the release gates — "search quality >= v1" — cannot be verified without it.

[ASSUMPTION: A-10] The benchmark dataset is generated from existing workspace data, not from a fixed corpus, so results are representative of each deployment's actual content.

**Functional Requirements:**

- **FR-37** `nano-brain bench generate --scale=N` generates a labeled benchmark dataset of N query-answer pairs from the current workspace. Scales: 100, 500, 1000. Stores the dataset in the workspace data directory.
- **FR-38** `nano-brain bench run --scale=N` runs the search pipeline against the benchmark dataset and computes P@5 (precision at 5), R@10 (recall at 10), MRR (mean reciprocal rank), insert p50/p95, and query p50/p95 latency.
- **FR-39** `nano-brain bench compare <new.json> <baseline.json>` compares two benchmark result files and flags regressions. Regression thresholds: P@5 drop > 0.10, R@10 drop > 0.10, MRR drop > 0.05, latency increase > 2×. Applies to UJ-5.
- **FR-40** Benchmark results can be saved to a file with `--save <path>` for use as a future baseline.
- **FR-41** The benchmarking suite also includes a concurrency test: N concurrent write goroutines write to the same workspace simultaneously, and the suite verifies zero data loss and no PostgreSQL constraint violations after completion. This tests the "zero corruption under concurrent access" release gate. Applies to UJ-3.

**Testable consequences:** Running `bench generate` on a non-empty workspace produces a JSON dataset file. Running `bench run` on that dataset produces a results file with non-null P@5, R@10, MRR values. `bench compare` correctly identifies a regression when results are artificially degraded. The concurrency test passes with N=10 concurrent writers. Applies to UJ-5.

---

### 4.7 Corruption Detection and Recovery

**Behavior:** PostgreSQL MVCC eliminates the SQLite corruption class entirely. However, v2 still needs protection against other failure modes: half-applied migrations, connection pool exhaustion, and incomplete writes that leave the database in an inconsistent state.

Recovery is built around the principle that the nano-brain database is a derivable cache: sessions can be re-harvested, files can be re-indexed, embeddings can be regenerated. If something goes wrong, the recovery path is to drop inconsistent data and re-derive from source, not to attempt surgical repair.

**Functional Requirements:**

- **FR-42** Every mutation (document insert, chunk insert, embedding insert, metadata update) runs inside a database transaction. If any step fails, the entire transaction rolls back. The system never leaves partial writes visible to readers.
- **FR-43** Document and chunk inserts use `ON CONFLICT DO NOTHING` or `ON CONFLICT DO UPDATE` semantics (upsert). Re-running the same ingestion job is idempotent.
- **FR-44** On startup, nano-brain runs a schema version check via goose. If the database schema version is behind the binary's embedded migrations, migrations are applied automatically before serving requests.
- **FR-45** `GET /health` returns `{"status": "ok", "ready": true}` only when the PostgreSQL connection pool is healthy and migrations are current. Returns `{"status": "degraded", "ready": false, "reason": "..."}` otherwise.
- **FR-46** The connection pool (pgxpool) performs periodic health checks on idle connections. Unhealthy connections are removed and replaced automatically without service restart.
- **FR-47** `nano-brain status` reports the PostgreSQL connection pool health, active connections, and any pending migration count.

**Testable consequences:** Writing 1000 documents with an injected failure on document 500 leaves no partial documents in the database. The health endpoint returns `ready: false` before migrations complete. Restarting the service after an unclean shutdown applies any pending migrations without data loss. The service recovers automatically after a brief PostgreSQL restart without manual intervention. Applies to UJ-1, UJ-3.

---

### 4.8 Chunking Strategy

**Behavior:** Documents are split into chunks before embedding and indexing. Chunking is heading-aware: it respects markdown structure and prefers to break at structural boundaries rather than in the middle of a sentence or code block.

[ASSUMPTION: A-11] The chunking algorithm from v1 is carried forward to v2 unchanged, since it produced good baseline search quality. The target size, overlap, and break-point scoring are preserved.

**Functional Requirements:**

- **FR-48** Documents are chunked with a target size of 3600 characters (~900 tokens) and 200-character overlap (~15%).
- **FR-49** The chunker scores candidate break points and selects the highest-scoring break within a 800-character search window before the target boundary. Break-point priority: H1 heading > H2 > H3/code-fence > H4-H6 > horizontal rule > blank line > list item > newline.
- **FR-50** The chunker tracks open code fences and never cuts inside a code block. Cuts near a code block boundary prefer the fence delimiter.
- **FR-51** Chunks shorter than 200 characters are merged with the adjacent chunk rather than stored as standalone entries.
- **FR-52** Each chunk is stored with its position within the document (sequence number, start line, end line) to enable line-range retrieval via `memory_get`.
- **FR-53** The chunker is deterministic: given the same document content, it always produces the same chunks in the same order. This ensures content-addressed deduplication works correctly.

**Testable consequences:** A 10,000-character document produces multiple chunks, none shorter than 200 characters. No chunk contains a dangling code fence opening. Running the chunker twice on the same input produces identical output. Retrieving a specific line range via `memory_get` returns the chunk that contains those lines.

---

### 4.9 HTTP API

**Behavior:** The HTTP API is the canonical interface for nano-brain. The CLI and MCP tools are wrappers over it. All state changes and queries go through HTTP. This makes the system debuggable (curl, any HTTP client), container-friendly (no filesystem side-channels), and transport-agnostic.

[ASSUMPTION: A-12] The API listens on `localhost:3100` by default. The host and port are configurable via environment variables.

The API uses a versioned path scheme: `/api/v1/*` for new endpoints. Legacy `/api/*` paths from v1 are retained for backward compatibility during the transition period. [ASSUMPTION: A-13]

**Functional Requirements:**

- **FR-54** `GET /health` returns server health and readiness. Response: `{status, ready, version, uptime_s, workspace_count}`. Requires no authentication.
- **FR-55** `GET /api/status` returns full index health: PostgreSQL connection status, embedding queue depth, active provider, workspace list summary, and migration version.
- **FR-56** `POST /api/query` executes hybrid search (FR-7). Request: `{query, workspace?, tags?, limit?, scope?}`. Response: FR-10 schema.
- **FR-57** `POST /api/search` executes BM25 search (FR-8). Same request/response shape as FR-56.
- **FR-58** `POST /api/vsearch` executes vector search (FR-9). Same request/response shape as FR-56.
- **FR-59** `POST /api/write` inserts a new document into the specified collection. Request: `{content, tags?, collection?, workspace?}`. Response: `{id, hash, collection, workspace_hash}`.
- **FR-60** `GET /api/wake-up` and `POST /api/wake-up` return a compact session briefing for the current workspace. GET uses query-string parameters; POST uses request body. Response: `{summary, recent_memories: [{id, snippet, tags, date}], active_collections: [...]}`. Applies to UJ-2.
- **FR-61** `POST /api/reindex` triggers async reindex for a collection or workspace. Request: `{root?}`. Response: `{job_id, status: "queued"}`.
- **FR-62** `POST /api/update` triggers async reindex of all collections in the current workspace. Same response shape as FR-61.
- **FR-63** `POST /api/embed` triggers an embedding pass (FR-33).
- **FR-64** `GET /api/v1/tags` returns all tags in the current workspace with document counts.
- **FR-65** `GET /api/v1/workspaces` returns all registered workspaces (FR-19).
- **FR-66** `GET /sse` and `POST /messages` implement the MCP Server-Sent Events transport (§4.11).
- **FR-67** `GET /mcp` and `POST /mcp` implement the MCP Streamable HTTP transport (§4.11).
- **FR-68** All API endpoints that read or write workspace data require a workspace identifier in the request (body for POST, query string for GET). Requests without a workspace identifier return HTTP 400 with error `{"error": "workspace_required", "message": "A workspace identifier is required. Pass workspace in request body (POST) or query string (GET). Use 'all' for cross-workspace queries."}`. There is no default workspace fallback.
- **FR-69** The API version is included in every response header as `X-Nano-Brain-Version: <semver>`.
- **FR-70** The API accepts and returns JSON only. Content-Type validation: requests with a non-JSON body to JSON endpoints return HTTP 415.

**Testable consequences:** All listed endpoints return the correct HTTP status codes (200 for success, 400 for bad input, 404 for missing resources, 500 for internal errors). The health endpoint returns 200 with `ready: true` when the service is healthy. A `POST /api/write` followed immediately by `POST /api/search` (BM25) returns the newly written document. Applies to UJ-1 through UJ-4.

---

### 4.10 MCP-over-HTTP

**Behavior:** MCP-compatible agents connect to nano-brain via HTTP rather than stdio. Two transports are supported: Server-Sent Events (SSE) and Streamable HTTP. Both expose the same 9 MCP tools. MCP stdio is explicitly not supported in v2.

[ASSUMPTION: A-14] The official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) is used for the MCP layer. Both SSE and Streamable HTTP transports are supported by the SDK.

**MCP Tools (Tier 1):**

| Tool | Description |
|---|---|
| `memory_search` | BM25 keyword search. Wraps FR-8. |
| `memory_vsearch` | Vector semantic search. Wraps FR-9. |
| `memory_query` | Full hybrid pipeline. Wraps FR-7. |
| `memory_get` | Retrieve a document by path or ID, with optional line range. |
| `memory_write` | Write a new memory entry to the daily log collection. Wraps FR-59. |
| `memory_tags` | List all tags in the current workspace with counts. Wraps FR-64. |
| `memory_status` | Return index health, embedding status, and workspace summary. Wraps FR-55. |
| `memory_update` | Trigger async reindex of all collections. Wraps FR-62. |
| `memory_wake_up` | Return a compact session briefing. Wraps FR-60. Applies to UJ-2. |

**Functional Requirements:**

- **FR-71** All 9 MCP tools listed above are registered on both the SSE transport (`/sse`, `/messages`) and the Streamable HTTP transport (`/mcp`). Applies to UJ-2, UJ-4.
- **FR-72** Each MCP tool call includes a `workspace` parameter. If omitted, the server returns HTTP 400 (missing required parameter). Passing `workspace: "all"` triggers a cross-workspace query (FR-21). All 9 MCP tools support `scope=all` for read operations.
- **FR-73** The MCP server handles multiple concurrent tool calls without race conditions. Each tool call is processed in its own goroutine using the shared connection pool.
- **FR-74** The Streamable HTTP transport uses 30-second SSE heartbeats to keep connections alive through proxies and load balancers.
- **FR-75** `memory_get` accepts a document path or `#docid` identifier and an optional `{start_line, end_line}` range. Returns the document content (or the specified line range) plus metadata.
- **FR-72b** `memory_write` accepts an optional `supersedes` parameter (document path or ID). When provided, the relationship is recorded in the database for future use by Tier 2 supersede demotion logic. No demotion is applied at search time in v2.0; the data is captured now so Tier 2 has it.

**Testable consequences:** An MCP client connecting via SSE can call `memory_query` and receive results. An MCP client connecting via Streamable HTTP receives the same results. Concurrent calls from multiple agents to `memory_write` on the same workspace complete without errors or duplicates. Applies to UJ-2, UJ-3, UJ-4.

---

### 4.11 CLI

**Behavior:** The CLI is a convenience wrapper over the HTTP API. Every CLI command sends an HTTP request to the running nano-brain server and formats the response for terminal output. [ASSUMPTION: A-15] The CLI is distributed as a single Go binary. It does not embed a database or run a server itself.

**CLI Commands (Tier 1):**

| Group | Commands |
|---|---|
| **Setup** | `init [--root --force]`, `setup` |
| **Search** | `query`, `search`, `vsearch` |
| **Memory** | `write [--tags --supersedes]`, `get`, `tags` |
| **Indexing** | `update`, `reindex`, `embed [--force]` |
| **Status** | `status` |
| **Harvesting** | `harvest` |
| **Collections** | `collection {add\|remove\|list\|rename}` |
| **Docker** | `docker {start\|stop\|status}` |
| **Benchmarking** | `bench {generate\|run\|compare}` |
| **Logs** | `logs [-f -n]` |
| **Migration** | `db:migrate` |

**Functional Requirements:**

- **FR-76** `nano-brain init --root <dir>` registers a workspace for the given directory, creates the default collections (`memory`, `sessions`), and outputs an AGENTS.md snippet for the project. Applies to UJ-1.
- **FR-77** `nano-brain query <text>`, `nano-brain search <text>`, and `nano-brain vsearch <text>` invoke hybrid, BM25, and vector search respectively and print formatted results.
- **FR-78** `nano-brain write <text> [--tags tag1,tag2]` writes a new document to the daily log and prints the assigned document ID.
- **FR-79** `nano-brain status` prints PostgreSQL health, embedding queue depth, workspace count, and collection stats.
- **FR-80** `nano-brain harvest` triggers an immediate harvest cycle without waiting for the poll interval.
- **FR-81** `nano-brain docker start` runs `docker compose up -d` from the nano-brain installation directory. `docker stop` runs `docker compose down`. `docker status` reports running container health. Applies to UJ-1.
- **FR-82** `nano-brain db:migrate` runs pending goose migrations against the configured PostgreSQL instance. Used during the v1-to-v2 data migration path. Applies to UJ-6.
- **FR-83** All CLI commands that communicate with the server honor `NANO_BRAIN_HOST` and `NANO_BRAIN_PORT` environment variables to override the default `localhost:3100`.
- **FR-84** All CLI commands support `--json` output for machine-readable use in scripts and CI.

**Testable consequences:** `nano-brain status` when the server is running prints non-error output. `nano-brain status` when the server is not running prints a clear error message with the expected host:port. `nano-brain query "test"` returns the same results as `POST /api/query` with the same query string. Applies to UJ-1, UJ-4, UJ-5.

---

### 4.12 Data Migration

**Behavior:** Existing v1 users have a working migration path from v1 (SQLite) to v2 (PostgreSQL). They can run the migration, verify their memories are queryable, and only then remove v1. The migration is a one-way operation.

[ASSUMPTION: A-16] The migration tool reads v1's SQLite schema directly. It does not require the v1 server to be running.

**Functional Requirements:**

- **FR-85** `nano-brain db:migrate --from-v1 <v1-db-path>` reads documents, chunks, and tags from a v1 SQLite database and inserts them into the active v2 PostgreSQL instance. Embeddings are not migrated; they are regenerated after migration via `nano-brain embed`.
- **FR-86** Migration is idempotent: running it twice on the same v1 database produces no duplicates (content-addressed upsert, same as FR-43).
- **FR-87** Migration reports progress as it runs: `Migrated N / M documents`. On completion, prints `Migration complete. Run 'nano-brain embed' to regenerate embeddings.`
- **FR-88** If migration encounters a document it cannot parse (corrupt v1 record), it logs the document ID and continues. It does not abort the migration for a single corrupt record.

**Testable consequences:** After migration, `nano-brain status` shows the correct document count matching the v1 database. BM25 search returns migrated documents before `embed` is run. Vector search returns migrated documents after `embed` is run. Applying migration twice produces the same document count as applying it once. Applies to UJ-6.

---

### 4.13 Configuration System

**Behavior:** nano-brain reads configuration from a YAML file at startup. All configurable values have defaults that produce a working system with no user modification required. [ASSUMPTION: A-17] Environment variables override YAML config values for deployment-specific settings (host, port, API keys).

**Functional Requirements:**

- **FR-89** nano-brain reads configuration from `~/.nano-brain/config.yml` by default. The path is overridable via `--config=<path>` or `NANO_BRAIN_CONFIG_PATH`.
- **FR-90** If the config file does not exist, nano-brain generates it from built-in defaults on first run.
- **FR-91** The following values are configurable: `embedding.provider`, `embedding.url`, `embedding.model`, `embedding.concurrency`, `harvester.opencode.session_dir`, `harvester.claude_code.enabled`, `harvester.claude_code.session_dir`, `intervals.session_poll`, `watcher.debounce_ms`, `watcher.reindex_interval`, `search.rrf_k`, `search.recency_weight`, `search.recency_half_life_days`, `search.limit`, `storage.max_file_size`, `storage.max_size`, `logging.level`, `logging.file`.
- **FR-92** `NANO_BRAIN_HOST`, `NANO_BRAIN_PORT`, `NANO_BRAIN_LOG`, `NANO_BRAIN_EMBEDDING_CONCURRENCY`, `VOYAGE_API_KEY`, and `OPENCODE_STORAGE_DIR` environment variables override the corresponding config file values.
- **FR-93** Configuration errors (invalid YAML, unknown keys, values outside valid ranges) print a descriptive error and exit with a non-zero status code at startup rather than silently using defaults.
- **FR-93b** `POST /api/reload-config` triggers an explicit re-read of the YAML config file. Reloadable settings: collection globs, embedding provider, embedding concurrency, log level, search parameters. Non-reloadable settings (require restart): database connection string, listen host/port, workspace root paths. Returns `{"reloaded": ["embedding.concurrency", ...], "unchanged": [...], "requires_restart": [...]}`. The current running session retains existing config until this endpoint is called or the server restarts.

**Testable consequences:** Starting the server with no config file produces a config.yml with default values. Changing `embedding.concurrency` in the config file and restarting changes the concurrency. Setting `NANO_BRAIN_PORT=8080` makes the server listen on 8080. An invalid YAML config causes a non-zero exit. Applies to UJ-1, UJ-7.

---

### 4.14 Logging and Telemetry

**Behavior:** nano-brain logs operational events to a rotating log file. Telemetry (search query logs) is stored in the database for internal use. Neither log data nor telemetry is sent to any external service. All data stays on disk.

**Functional Requirements:**

- **FR-94** Logs are written to `~/.nano-brain/logs/nano-brain-YYYY-MM-DD.log`. Log rotation: max 50 MB per file, max 5 files retained.
- **FR-95** Log levels: `error`, `warn`, `info` (default), `debug`. Level is configurable via `logging.level` and `NANO_BRAIN_LOG`.
- **FR-96** `nano-brain logs [-f] [-n N]` displays recent log output. `-f` follows the log in real time. `-n` limits output to the last N lines.
- **FR-97** Search telemetry (query text, result count, latency, collection, workspace hash) is stored in the database. Retention period is 90 days by default, configurable via `telemetry.retention_days`.
- **FR-98** No log data or telemetry is transmitted outside the local system. There are no external analytics endpoints.

**Testable consequences:** After running several queries, `nano-brain logs` shows the corresponding log entries. Log files rotate when they exceed the size limit. No outbound network connections are made by the server in an offline environment. Applies to UJ-1.

---

### 4.15 Docker Compose Deployment

**Behavior:** The canonical deployment is `docker compose up`. A single command starts nano-brain and its PostgreSQL dependency together. No other infrastructure is required. [ASSUMPTION: A-18] The Docker Compose file is part of the nano-brain repository and is the primary deployment artifact.

**Functional Requirements:**

- **FR-99** The Docker Compose file declares two services: `nano-brain` (the Go application) and `postgres` (PostgreSQL 17 with pgvector 0.8.2, image `pgvector/pgvector:0.8.2-pg17`). [ASSUMPTION: A-19]
- **FR-100** `docker compose up` starts both services and nano-brain is ready to serve requests within 30 seconds on standard hardware. [ASSUMPTION: A-20]
- **FR-101** The `postgres` service persists data to a named Docker volume. Restarting the stack with `docker compose down && docker compose up` does not lose data.
- **FR-102** The nano-brain container runs migrations automatically on startup (FR-44). No manual migration step is required for a fresh deployment.
- **FR-103** The container health check calls `GET /health` every 30 seconds. Docker reports the container as unhealthy if health fails 3 consecutive times.
- **FR-104** nano-brain detects when it is running inside a Docker container and adjusts the default PostgreSQL host to the internal service name. [ASSUMPTION: A-21]
- **FR-105** `nano-brain docker start` and `nano-brain docker stop` CLI commands are convenience wrappers around `docker compose up -d` and `docker compose down`. Applies to UJ-1, FR-81.

**Testable consequences:** On a machine with Docker installed, `docker compose up` from the repository root produces a running nano-brain instance. `GET /health` returns `ready: true` within 30 seconds. `docker compose down && docker compose up` restores the previous state with all previously stored documents intact. Applies to UJ-1.

---

## 5. Non-Goals (Explicit)

These are deliberately excluded from v2. They are not deferred — they are out of scope at the design level for v2.

- **GUI-first experience.** nano-brain is API-first. A web UI may ship in Tier 2, but every user-facing capability will always be available via HTTP API first. The UI is never the only path to a feature.
- **MCP stdio transport.** Removed. Stdio introduced lifecycle and buffering bugs in v1. v2 uses HTTP transports only. Existing agents that support MCP HTTP/SSE are compatible.
- **Multi-tenant SaaS.** nano-brain is single-team self-hosted. Multi-tenant packaging is deferred indefinitely.
- **Mobile or cross-platform clients.** Out of scope entirely.
- **Cloud-hosted vector databases.** No Pinecone, Weaviate Cloud, or similar external vector services. The vector store runs inside PostgreSQL via pgvector.
- **Real-time collaboration / multi-user editing.** Concurrent writes from multiple agents are handled at the PostgreSQL level (MVCC). This is an infrastructure correctness property, not a product collaboration feature.

---

## 6. MVP Scope

### 6.1 In Scope (Tier 1 — v2.0)

All features in §4 are in scope for v2.0:

- Session Harvesting (§4.1)
- Hybrid Search — BM25 + vector + RRF (§4.2)
- Per-Workspace Isolation (§4.3)
- File Watcher and Collection Scanning (§4.4)
- Embedding Providers — Ollama and VoyageAI (§4.5)
- Benchmarking Suite (§4.6)
- Corruption Detection and Recovery (§4.7)
- Chunking Strategy (§4.8)
- HTTP API — all endpoints listed in §4.9 (§4.9)
- MCP-over-HTTP — SSE and Streamable HTTP, 9 tools (§4.10)
- CLI — all commands listed in §4.11 (§4.11)
- Data Migration from v1 (§4.12)
- Configuration System (§4.13)
- Logging and Telemetry (§4.14)
- Docker Compose Deployment (§4.15)

### 6.2 Out of Scope (Tier 2 — v2.1 and later)

These features are explicitly deferred. They will not be built for v2.0 and should not be designed for in the v2.0 architecture, except to ensure the module boundaries that would enable them later are clean.

| Feature | Rationale for deferral |
|---|---|
| Code intelligence (Tree-sitter, symbol extraction, call graph, PageRank, Louvain) | Requires a correct, stable Tier 1 foundation. Cannot be validated without the benchmarking suite shipping first. |
| Knowledge graph (LLM entity/relationship extraction) | Depends on stable ingestion pipeline. LLM integration adds external dependency complexity. |
| Self-learning (Thompson Sampling, preference learning, intent classification) | Requires search telemetry accumulation over time. Cannot be meaningfully tuned on a fresh system. |
| Consolidation (LLM-driven summarization of old memory) | Requires stable storage layer and significant memory accumulation. |
| Neural reranking (VoyageAI rerank-2.5-lite) | Adds cloud dependency and latency. RRF-only baseline must be validated first. |
| Query expansion | No provider implementation exists. Infrastructure deferred. |
| Supersede demotion in search pipeline | Simple to add later; not required for MVP correctness. |
| Web UI and dashboards | Not required for agent workflows. API-first design means UI adds no capability. |
| Cross-repo symbol tracking | Depends on Tree-sitter code intelligence (Tier 2 dependency). |
| `scope=all` cross-workspace queries in MCP tools | Read-path complexity with low immediate demand. HTTP API supports it; MCP tools can be extended. |
| MCP tools beyond the 9 listed | memory_expand, memory_consolidate, code_context, code_impact, memory_symbols, memory_impact, memory_focus, all graph tools, memory_learning_status, memory_suggestions, memory_index_codebase |
| CLI commands beyond those listed | context, code-impact, detect-changes, focus, graph-stats, symbols, impact, consolidate, categorize-backfill, learning, qdrant *, cache *, reset (destructive), rm |
| HTTP endpoints beyond those listed | /api/maintenance/*, /api/v1/graph/*, /api/v1/telemetry, /api/v1/code/*, /web, /api/vector-health |
| launchd / systemd service integration | Docker Compose is sufficient for v2.0. OS-level service management deferred. |

---

## 7. Success Metrics

### 7.1 Primary Metrics (Release Gates)

All seven must pass for v2.0 to ship.

| Gate | Metric | Target | Test method |
|---|---|---|---|
| G1 — Zero corruption | Data integrity after N concurrent writers | 0 documents lost or corrupted in bench concurrency test with N=10 | `bench run` concurrency test (FR-41) |
| G2 — Workspace isolation | Isolation invariant | 0 cross-workspace data leaks in 1000 cross-workspace query permutations | Automated isolation test against all workspace pairs |
| G3 — Search quality >= v1 | P@5, R@10, MRR vs v1 baseline | P@5 >= 0.835, R@10 >= 0.970, MRR >= 1.000 at scale-100 [ASSUMPTION: A-22] | `bench compare new.json v1-baseline.json` (FR-39) |
| G4 — MVP features working | End-to-end harvest → store → search | Full UJ-1 through UJ-4 pass without manual intervention | Integration test suite running all UJs |
| G5 — Drop-in replacement | Migration fidelity | 100% of v1 documents queryable after migration (BM25) | `db:migrate` followed by document count verification (FR-85 through FR-88) |
| G6 — 1-command setup | Setup steps to working state | `docker compose up` + `nano-brain init` only; no additional configuration required [ASSUMPTION: A-20] | Fresh machine test (UJ-1) |
| G7 — Documentation complete | Docs published | README, HTTP API reference, and migration guide all present and accurate at release | Manual review |

### 7.2 Secondary Metrics

These are tracked but do not block release.

- **Search latency p50 < 200 ms** for hybrid queries on 10,000 documents. See §10.
- **Embedding queue drain rate**: >= 3 chunks/s with Ollama local.
- **Session harvest lag**: new sessions appear in search within 5 minutes of being written.
- **Startup time**: server ready to serve requests within 5 seconds on standard hardware.

### 7.3 Counter-Metrics

These indicate the product is going wrong. Investigate if they appear.

- **False isolation**: any query returning data from a different workspace than requested.
- **Silent data loss**: document count decreasing without explicit delete operations.
- **Embed flood**: embedding queue growing unboundedly without draining.
- **Migration regression**: v1 documents not found in v2 after migration.

---

## 8. Cross-Cutting NFRs

### 8.1 Concurrency Safety

- All shared mutable state is stored in PostgreSQL, not in application memory. Application-level shared mutable state is minimized by design.
- The PostgreSQL connection pool (pgxpool) is goroutine-safe. No application-level mutex is needed around pool access.
- CI runs `go test -race` on every push to catch data races before they reach production.
- Every database transaction uses `context.WithTimeout` to prevent indefinitely-held locks.
- Background goroutines (file watcher, harvester, embedder) communicate with the HTTP handler goroutines only via the database, not via in-memory shared state.

### 8.2 Workspace Isolation Invariant

The workspace isolation invariant is binary and non-negotiable: no read or write operation on workspace A can access data from workspace B. This is a release gate (G2), not a best-effort goal.

- Every database query includes a `WHERE workspace_hash = $1` clause.
- The workspace hash is set at the query routing layer before any database call.
- There is no "default workspace" that queries fall back to if the workspace is unspecified — missing workspace returns HTTP 400 with a descriptive error message. This applies to both HTTP API and MCP tools.
- Cross-workspace `scope=all` queries explicitly list all workspace hashes they are authorized to read. They do not use `WHERE 1=1`.

### 8.3 Search Quality Baseline

- v2 hybrid search must meet or exceed v1 quality metrics at release (G3).
- The benchmarking suite (§4.6) is the instrument for validating this. It is not an optional add-on.
- The v2 MVP pipeline (BM25 + vector + RRF + recency boost) is a deliberate simplification of v1's 16-stage pipeline. If the simplified pipeline does not meet quality baselines, additional stages from v1 are candidates for inclusion before v2.0 ships. [ASSUMPTION: A-3]

### 8.4 Data Integrity

- All document and chunk inserts are atomic transactions (FR-42).
- All inserts are idempotent via content-addressed upsert (FR-43).
- Migrations are forward-only and applied automatically (FR-44).
- The database is a derivable cache: if data is lost, re-harvest and re-index restore it from source.
- No data is deleted without explicit user instruction. Retention limits evict old sessions by default (configurable; see FR-28), but core memory documents are not evicted automatically.

### 8.5 Privacy

- Default configuration uses Ollama for embeddings (local, no outbound calls).
- Cloud embedding (VoyageAI) is opt-in via explicit configuration.
- No telemetry or log data is transmitted outside the local system (FR-98).
- The server does not make outbound network calls except to the configured embedding provider.

---

## 9. API Contracts / Public Surface

### 9.1 HTTP REST Endpoints (Tier 1 Canonical)

| Method | Path | Description | FR |
|---|---|---|---|
| GET | /health | Server health and readiness | FR-54 |
| GET | /api/status | Full index health | FR-55 |
| POST | /api/query | Hybrid search | FR-56 |
| POST | /api/search | BM25 search | FR-57 |
| POST | /api/vsearch | Vector search | FR-58 |
| POST | /api/write | Write document | FR-59 |
| GET | /api/wake-up | Session briefing (GET) | FR-60 |
| POST | /api/wake-up | Session briefing (POST) | FR-60 |
| POST | /api/reindex | Async reindex collection | FR-61 |
| POST | /api/update | Async reindex all | FR-62 |
| POST | /api/embed | Embed pending chunks | FR-63 |
| GET | /api/v1/tags | Tags with counts | FR-64 |
| GET | /api/v1/workspaces | Workspace list | FR-65 |
| POST | /api/reload-config | Reload YAML config (hot) | FR-93b |
| GET | /sse | MCP SSE transport | FR-66 |
| POST | /messages | MCP SSE messages | FR-66 |
| GET | /mcp | MCP Streamable HTTP | FR-67 |
| POST | /mcp | MCP Streamable HTTP | FR-67 |

### 9.2 Versioning Policy

- `/api/v1/*` paths are the stable, versioned surface. Breaking changes require a `/api/v2/*` path, not modification of existing v1 behavior.
- `/api/*` (unversioned) paths are retained for backward compatibility with v1 clients during the migration period. They are aliases to v1 behavior.
- `/health` is version-agnostic and will not break across versions.
- The `X-Nano-Brain-Version` response header carries the server version on every response (FR-69).

### 9.3 MCP Tool Surface (Tier 1)

| Tool | Transport | Wraps |
|---|---|---|
| memory_search | SSE, Streamable HTTP | POST /api/search |
| memory_vsearch | SSE, Streamable HTTP | POST /api/vsearch |
| memory_query | SSE, Streamable HTTP | POST /api/query |
| memory_get | SSE, Streamable HTTP | Document fetch by ID |
| memory_write | SSE, Streamable HTTP | POST /api/write |
| memory_tags | SSE, Streamable HTTP | GET /api/v1/tags |
| memory_status | SSE, Streamable HTTP | GET /api/status |
| memory_update | SSE, Streamable HTTP | POST /api/update |
| memory_wake_up | SSE, Streamable HTTP | GET /api/wake-up |

MCP tool parameters mirror their HTTP equivalents. All tools accept an optional `workspace` parameter.

---

## 10. Performance Budgets

| Operation | p50 target | p95 target | Condition |
|---|---|---|---|
| Hybrid search (`memory_query`) | < 200 ms | < 500 ms | 10,000 documents, 1 concurrent query |
| BM25 search (`memory_search`) | < 50 ms | < 200 ms | 10,000 documents, 1 concurrent query |
| Vector search (`memory_vsearch`) | < 150 ms | < 400 ms | 10,000 documents, 1 concurrent query |
| Document write (`POST /api/write`) | < 100 ms | < 300 ms | Excludes async embedding |
| Session briefing (`memory_wake_up`) | < 2 s | < 5 s | 100 recent documents |
| Embedding drain (Ollama local) | >= 3 chunks/s | — | Single goroutine, `nomic-embed-text` |
| Server startup to ready | < 5 s | < 10 s | Cold start, empty database |
| Migration throughput | >= 500 documents/s | — | v1 SQLite to PostgreSQL |

[ASSUMPTION: A-5] These budgets are based on typical laptop/dev-server hardware (4+ cores, SSD, 8+ GB RAM). They will be validated against the benchmarking suite (§4.6) before release.

---

## 11. Constraints and Guardrails

### 11.1 Stack Constraints (Locked)

- **Language:** Go 1.23+. No other languages in the server binary.
- **Database:** PostgreSQL 17 with pgvector 0.8.2+. Pinned Docker image: `pgvector/pgvector:0.8.2-pg17`. No other database or vector store.
- **MCP transport:** HTTP only (SSE + Streamable HTTP). MCP stdio is not supported.
- **Deployment:** Docker Compose for the canonical deployment. Single-node only.
- **Embedding providers:** Ollama and VoyageAI only for v2.0. [ASSUMPTION: A-8]

### 11.2 Operational Constraints

- The server is single-node. No clustering or horizontal scaling.
- Maximum supported vectors per workspace: 500,000. [ASSUMPTION: A-5] Beyond this, pgvector index performance degrades and architecture review is needed.
- Maximum document size: 300 KB per file (configurable, FR-28).
- Maximum workspace storage: 10 GB per workspace (configurable, FR-28).
- No Windows support for the v2.0 release. Linux and macOS are the supported platforms. [ASSUMPTION: A-23]

### 11.3 API Stability Guardrails

- The 9 MCP tools listed in §4.10 are stable for the v2.0 release. Their input/output schemas are frozen.
- The 17 HTTP endpoints listed in §9.1 are stable for the v2.0 release.
- The CLI command names and flag signatures for Tier 1 commands (§4.11) are stable.
- Any breaking change requires a version bump and a migration path document.

---

## 12. Resolved Decisions (formerly Open Questions)

All questions resolved 2026-05-23 during PRD review.

| ID | Question | Decision | FR/Section |
|---|---|---|---|
| OQ-1 | Does `scope=all` need to be in the 9 MCP tools for MVP? | **Yes.** All 9 MCP tools support `scope=all` for read operations. Thin wrapper over HTTP; minimal additional effort. | FR-72 |
| OQ-2 | Should `memory_write` support `--supersedes`? | **Yes.** Capture the relationship at write time. Demotion logic deferred to Tier 2, but the data field ships in MVP. | FR-72b |
| OQ-3 | PostgreSQL version minimum? | **PostgreSQL 17.** Pinned image: `pgvector/pgvector:0.8.2-pg17`. PG 17 is GA stable (18+ months), pgvector 0.8.2 fully compatible, gains in JSON perf (+25-40%), vacuum memory (20× reduction), WAL throughput (2×). | §11.1, FR-99 |
| OQ-4 | Workspace identity in HTTP requests? | **Body (POST), query string (GET). Missing workspace → HTTP 400 error. `"all"` → cross-workspace query.** No default workspace fallback. | FR-68, §8.2 |
| OQ-5 | v1 benchmark baseline for v2 quality validation? | **Adopt v1 baselines as-is** (P@5=0.835, R@10=0.970, MRR=1.000 at scale-100, Ollama local). | §7.1 G3 |
| OQ-6 | Config reload: `nano-brain init` vs server auto-migrate vs hot-reload? | **Server auto-migrates on startup (FR-44). Explicit `POST /api/reload-config` for hot-reloading safe settings.** No fsnotify magic. Config changes require either the reload endpoint or a restart. | FR-93b, §4.13 |
| OQ-7 | Embedding queue depth cap and overflow behavior? | **In-memory queue: 10K chunk IDs. Reject threshold: 50K (HTTP 503 + Retry-After). Notification: `GET /api/status` includes queue depth + ETA. Structured log warnings at 60%/90% capacity.** No SSE for MVP; polling via status endpoint is sufficient. | FR-36b–FR-36e, FR-55 |

---

## 13. Assumptions Index

| ID | Assumption | Section | Risk if wrong |
|---|---|---|---|
| A-1 | OpenCode and Claude Code continue to write session files in the JSON format documented in v1. If either changes their format, the harvester adapter must be updated. | §4.1 | Medium — harvest breaks for that adapter; other features unaffected |
| A-2 | A 120 s poll interval is acceptable for session harvesting latency. Sub-minute freshness is not required. | §4.1 | Low — configurable |
| A-3 | The 7-stage MVP pipeline (BM25 + vector + RRF + recency boost) meets the v1 search quality baseline. If it does not, additional stages (supersede demotion, length normalization, etc.) will be included before v2.0 ships. | §4.2, §8.3 | High — if wrong, Tier 2 stages become Tier 1 scope, expanding v2.0 work |
| A-4 | Workspace identity is passed per-request (body for POST, query string for GET). There is no session-based workspace state in the server. Requests without a workspace return HTTP 400. Passing `"all"` triggers cross-workspace queries on read operations. | §4.9, §4.10, §8.2 | Low — resolved as a firm design decision (OQ-4) |
| A-5 | Performance budgets (§10) are achievable on a 4-core dev machine with SSD and 8 GB RAM for a corpus of up to 100,000 documents. pgvector performance above 500,000 vectors per workspace has not been validated. | §4.2, §10, §11.2 | Medium — if wrong, index or query strategy changes may be needed |
| A-6 | SHA-256 of the workspace root directory path is a stable, collision-resistant workspace identifier. Renaming a workspace root directory changes its hash and creates a new workspace. | §4.3 | Low — predictable behavior; renaming is a rare operation |
| A-7 | The default collections `memory` and `sessions` with a `**/*.md` glob pattern are the right defaults for most users. | §4.4 | Low — configurable |
| A-8 | Ollama and VoyageAI are the only embedding providers needed for v2.0. Other OpenAI-compatible endpoints (Azure, LM Studio) are out of scope until v2.1. | §4.5, §11.1 | Low — the provider abstraction can be extended without breaking changes |
| A-9 | Ollama with `nomic-embed-text` as the default embedding model provides sufficient embedding quality for the v1 baseline. | §4.5 | Medium — if quality is insufficient, default model needs changing before release |
| A-10 | Generating benchmark datasets from existing workspace data (not a fixed golden corpus) is sufficient for validating search quality. | §4.6 | Medium — results are deployment-specific, not universal; may not catch all regression patterns |
| A-11 | The v1 chunking parameters (3600 char target, 200 char overlap, break-point scoring) produce good results for markdown-heavy AI session content. | §4.8 | Low — configurable; can be tuned without schema changes |
| A-12 | `localhost:3100` is the right default port. It does not conflict with other commonly used local services. | §4.9 | Low — configurable |
| A-13 | v1 `/api/*` path compatibility is needed for the migration period. Clients using v1 HTTP paths will not need code changes immediately after migration. | §4.9, §9.2 | Low — if no v1 HTTP clients exist outside the CLI, these aliases can be dropped |
| A-14 | The official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`) is stable enough for production use as of Go 1.23. | §4.10 | Medium — if the SDK has breaking changes before release, the MCP layer needs updating |
| A-15 | The CLI as a pure HTTP wrapper (no embedded DB) is acceptable. Users running the CLI against a remote server need network access to the server. | §4.11 | Low — design intent; remote CLI was a v1 use case |
| A-16 | v1's SQLite schema is stable enough that a migration reader can be written without needing the v1 codebase to run. | §4.12 | Medium — if v1 schema has undocumented variations, migration tool needs branching logic |
| A-17 | YAML is the right config format. Users are comfortable with YAML for local daemon configuration. | §4.13 | Low — format preference only |
| A-18 | A Docker Compose file in the repository is the right primary deployment artifact. Users have Docker installed. | §4.15 | Low — alternative: provide a bare-metal install guide as well |
| A-19 | A two-service Docker Compose (nano-brain + postgres) is sufficient. No PgBouncer, no Qdrant, no Redis at v2.0. | §4.15 | Low — if connection exhaustion is a problem under high load, PgBouncer can be added to Compose |
| A-20 | The "1-command setup" release gate means `docker compose up` followed by `nano-brain init`. Users are expected to install the CLI separately. | §7.1 G6 | Low — alternative is to include CLI install in the setup command |
| A-21 | Container-to-database hostname resolution works via Docker Compose service name (`postgres`). No manual DNS configuration needed. | §4.15 | Low — standard Docker Compose behavior |
| A-22 | v1 benchmark baselines (P@5=0.835, R@10=0.970, MRR=1.000 at scale-100, Ollama local) are the correct targets for v2 quality validation. These numbers come from v1's benchmarking suite output (reference PRD §6.16). | §7.1 G3, OQ-5 | Medium — if v1 baselines used a different test methodology, comparison is not apples-to-apples |
| A-23 | Linux and macOS coverage is sufficient for v2.0. Windows users can use WSL2. | §11.2 | Low — Windows is a lower-priority use case given the Docker Compose deployment model |
