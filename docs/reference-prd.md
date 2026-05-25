# nano-brain — Product Requirements Document

**Version:** 2026.8.19
**Status:** Living document
**Owner:** nano-step / kokorolx
**Last updated:** 2026-05-22

---

## Table of contents

1. [Product summary](#1-product-summary)
2. [Problem & target users](#2-problem--target-users)
3. [Goals & non-goals](#3-goals--non-goals)
4. [Differentiators](#4-differentiators)
5. [Architecture overview](#5-architecture-overview)
6. [Feature inventory](#6-feature-inventory)
   - 6.1 Data ingestion
   - 6.2 Storage & reliability
   - 6.3 Search pipeline
   - 6.4 Code intelligence
   - 6.5 Knowledge graph
   - 6.6 Self-learning & adaptation
   - 6.7 Background jobs
   - 6.8 Provider abstractions
   - 6.9 Chunking strategy
   - 6.10 CLI surface
   - 6.11 MCP tool surface
   - 6.12 HTTP API surface
   - 6.13 Deployment modes
   - 6.14 Web UI & dashboards
   - 6.15 AI agent integration
   - 6.16 Benchmarking suite
   - 6.17 Logging & telemetry
   - 6.18 Configuration system
7. [Configuration reference](#7-configuration-reference)
8. [Operational requirements](#8-operational-requirements)
9. [Open issues & known limitations](#9-open-issues--known-limitations)
10. [Roadmap signals](#10-roadmap-signals)
11. [Glossary](#11-glossary)

---

## 1. Product summary

nano-brain is a **persistent memory and code intelligence server for AI coding agents**. It runs as a local daemon (Docker container, launchd service, or stdio MCP process) and gives any MCP-capable agent two things their host model cannot do alone:

1. **Cross-session recall** — automatically harvested AI sessions, curated notes, and codebase symbols, indexed with a 6-signal hybrid search pipeline (BM25 + vector + RRF + PageRank + supersede demotion + neural reranking).
2. **Code intelligence** — Tree-sitter symbol graph, call-flow detection, file dependency graph with PageRank centrality and Louvain clustering, and cross-repo infrastructure symbol tracking (Redis keys, MySQL tables, API endpoints, Bull queues).

The product is exposed through three integration surfaces — **CLI** (29 commands), **MCP** (24 tools across stdio / HTTP / SSE transports), and a **REST + streaming HTTP API** (29 endpoints) — backed by a self-tuning learning loop (Thompson Sampling bandits, preference learning, importance scoring, query-sequence pattern detection).

It is **privacy-first by default** (100% local with Ollama + sqlite-vec) and **production-ready when scaled** (VoyageAI + Qdrant, per-workspace SQLite isolation, automatic corruption recovery).

---

## 2. Problem & target users

### Problem

Modern AI coding agents (Claude Code, OpenCode, Cursor, etc.) have three structural weaknesses:

1. **No cross-session memory.** Every new chat re-derives context from scratch. Past decisions, debugging insights, and architectural choices are lost.
2. **Context-window blindness.** Codebases routinely exceed any model's context. Agents grep blindly, miss patterns, and contradict prior decisions.
3. **No code intelligence.** Agents cannot answer "what calls this?", "what does this break?", or "where is this Redis key written?" without expensive whole-codebase scans.

### Target users

- **Primary:** Developers running AI coding agents (OpenCode, Claude Code) who work in long-lived multi-month projects and want their assistant to "remember."
- **Secondary:** Teams building custom AI agents who need a drop-in memory + code-intelligence backend with MCP support.
- **Tertiary:** Researchers benchmarking retrieval pipelines who need a reproducible harness with quality (P@5, R@10, MRR) and latency metrics.

### Personas

| Persona | Pain | nano-brain win |
|---|---|---|
| Solo dev with a 6-month side project | "I keep re-explaining my auth model to Claude" | `memory_query` recalls the decision + the file + the date |
| Team running OpenCode in CI | "Sessions are siloed; nobody benefits from yesterday's debugging" | Session harvester ingests all sessions, hybrid search makes them queryable |
| Refactor lead | "I need to know what calls `getUser()` across 12 services" | `code_impact` + `memory_symbols` answer in <100ms |
| MCP integrator | "I want a self-hosted memory backend with a stable API" | 24 MCP tools, REST API, Docker compose, OSS (MIT) |

---

## 3. Goals & non-goals

### Goals (in scope)

- **G1.** Persistent, queryable memory across AI agent sessions.
- **G2.** Hybrid retrieval that beats BM25 alone and beats vector-only on code-heavy corpora (validated by `bench` suite).
- **G3.** Code intelligence (symbol graph, call graph, dependency graph, flow detection) for TypeScript / JavaScript / Python.
- **G4.** Self-learning loop that improves retrieval quality over time without manual tuning.
- **G5.** Privacy-first deployment: 100% local option (Ollama + sqlite-vec), no required cloud calls.
- **G6.** Multi-workspace isolation with cross-workspace queries (`scope=all`).
- **G7.** First-class MCP support: stdio, HTTP, SSE transports.
- **G8.** Operational reliability: automatic corruption recovery, retention enforcement, daemon lifecycle.

### Non-goals (out of scope)

- **NG1.** Hosting a multi-tenant SaaS. nano-brain is single-user / single-team self-hosted.
- **NG2.** Replacing the AI agent. nano-brain is a backend; the agent does the reasoning.
- **NG3.** A general-purpose RAG framework. The pipeline is opinionated for code + AI-session corpora.
- **NG4.** Full-IDE integration (LSP, refactor codemods). Code intelligence is read-only.
- **NG5.** Universal language support. Tree-sitter coverage is TS / JS / Python only.
- **NG6.** Cluster / horizontal scaling. Single-node SQLite is the storage primitive.

---

## 4. Differentiators

| Capability | nano-brain | Mem0 | Zep / Graphiti | Letta | Claude native |
|---|---|---|---|---|---|
| 6-signal hybrid search | ✓ | ✗ | ✗ | ✗ | ✗ |
| Code intelligence (AST + call graph) | ✓ | ✗ | ✗ | ✗ | ✗ |
| Cross-repo infra symbol tracking | ✓ | ✗ | ✗ | ✗ | ✗ |
| Session auto-harvesting | ✓ | ✗ | ✗ | ✗ | partial |
| Neural reranking | ✓ | ✗ | ✗ | ✗ | ✗ |
| Self-tuning (Thompson Sampling) | ✓ | ✗ | ✗ | ✗ | ✗ |
| 100% local option | ✓ | ✗ | partial | ✓ | ✓ |
| MCP-native (stdio / HTTP / SSE) | ✓ | ✗ | ✗ | ✗ | partial |
| Per-workspace isolation | ✓ | ✗ | partial | partial | ✗ |
| 22 MCP tools / 29 HTTP endpoints | ✓ | 4–9 tools | 9–10 | 7 | 0 |

The combination — **code intelligence + AI-session harvesting + self-tuning hybrid retrieval + MCP** — is unique in the agent-memory category.

---

## 5. Architecture overview

```
                        ┌──────────────────────────┐
                        │  AI agent (OpenCode etc) │
                        └──────────┬───────────────┘
                                   │ MCP (stdio / HTTP / SSE)
                                   ▼
            ┌──────────────────────────────────────────┐
            │  nano-brain server                       │
            │  ┌────────────┐  ┌────────────────────┐ │
            │  │ MCP server │  │ HTTP / SSE server  │ │
            │  │ (24 tools) │  │ (29 endpoints)     │ │
            │  └─────┬──────┘  └─────────┬──────────┘ │
            │        └──────────┬────────┘            │
            │                   ▼                     │
            │  ┌─────────────────────────────────┐    │
            │  │  Search pipeline                │    │
            │  │  BM25 + vector + RRF + PageRank │    │
            │  │  + supersede demote + rerank    │    │
            │  └────────┬───────┬────────┬───────┘    │
            │           ▼       ▼        ▼            │
            │   ┌──────┐  ┌────────┐  ┌──────────┐    │
            │   │ FTS5 │  │ vec DB │  │ Symbol   │    │
            │   │      │  │ Qdrant │  │ graph    │    │
            │   │      │  │ /sqlite│  │          │    │
            │   └──────┘  └────────┘  └──────────┘    │
            │           SQLite (per-workspace)        │
            │                                         │
            │  ┌──────────────────────────────────┐   │
            │  │ Background jobs                  │   │
            │  │ harvest │ index │ embed │ learn  │   │
            │  │ consolidate │ prune │ importance │   │
            │  └──────────────────────────────────┘   │
            └──────────────────────────────────────────┘
                       │              │
                       ▼              ▼
              ┌────────────┐  ┌─────────────────┐
              │ Embedding  │  │ Reranker        │
              │ (VoyageAI/ │  │ (VoyageAI       │
              │  Ollama)   │  │  rerank-2.5)    │
              └────────────┘  └─────────────────┘
```

**Data flow (write path):**
`source files / AI sessions` → harvester / file watcher → chunker (900 tokens, 15% overlap) → SQLite + FTS5 → async embedder → vector store → entity extractor → knowledge graph → categorizer (`auto:*` + `llm:*` tags).

**Data flow (read path):**
agent query → MCP / HTTP → query expansion (optional) → parallel BM25 + vector + symbol search → RRF fusion (k=60) → PageRank boost (0.1×) → supersede demote (0.05×) → usage / length / recency / category boosts → top-K candidates → neural reranker → position-aware blending (75/25, 60/40, 40/60) → results.

**Isolation:** Each workspace gets a dedicated SQLite file `~/.nano-brain/data/{name}-{12-char-hash}.sqlite`. Cross-workspace queries (`scope=all`) open multiple stores and RRF-fuse the results.

---

## 6. Feature inventory

### 6.1 Data ingestion

Three parallel ingestion pipelines run in the background, all governed by `~/.nano-brain/config.yml`.

#### 6.1.1 Session harvesting

| Source | Adapter | Default path | Format |
|---|---|---|---|
| OpenCode | `OpenCodeAdapter` | `~/.local/share/opencode/storage/` | JSON sessions + per-message files |
| Claude Code | `ClaudeCodeAdapter` | configured in `config.yml` | JSON sessions |

- Polls every **120 s** (`config.intervals.sessionPoll`).
- For each session: load metadata (`id`, `slug`, `title`, `projectID`, `directory`, `time_created`), load messages from `message/<sessionId>/msg_*.json`, sort chronologically.
- Convert to markdown via `sessionToMarkdown()` + `messagesToMarkdown()` with front-matter.
- Hash-based dedup (SHA-256 over rendered markdown). State persisted to `~/.nano-brain/harvest-state.json`.
- Write to `~/.nano-brain/sessions/<sessionId>.md` and insert into `documents` (collection=`sessions`).
- Optional fact extraction (LLM): up to **10 000** facts per database, stored in `entities` + `relationships`.

#### 6.1.2 File watching (collections)

- `chokidar` watches all configured collections (default: `memory`, `sessions`, plus user-added).
- Debounce **2000 ms**; reindex poll every **300 s**.
- On dirty flag, scans collection and calls `indexDocument()` (hash-based skip).
- Storage retention enforced in same loop: evict expired sessions (90-day default), evict by size if disk budget (10 GB default) exceeded.

#### 6.1.3 Codebase indexing

- Glob scan with `BUILTIN_EXCLUDE_PATTERNS` (`node_modules`, `.git`, `dist`, `build`, …).
- Limits: max file size **300 KB**, max corpus size **2 GB**.
- For each file: SHA-256 hash → skip if unchanged → Tree-sitter parse → emit symbols (function / class / method / variable / interface / type / enum / property) → resolve call edges + heritage edges → parse imports/exports → write to `code_symbols`, `symbol_edges`, `file_edges`.
- After scan: PageRank centrality (damping 0.85, 100 iterations), Louvain clustering (min 20 nodes), call-flow detection (entry → leaf, max depth 10, max branching 4, min steps 2, max 75 flows).
- Async embedding with adaptive backoff (60 s start, ×1.5 on failure, max 300 s).

### 6.2 Storage & reliability

#### 6.2.1 SQLite schema (24 tables, 5 groups)

**Core documents (3):** `documents`, `content` (SHA-256 keyed), `chunks` (virtual via triggers).
**Search indexes (2):** `documents_fts` (FTS5, porter unicode61 stemming), `content_vectors` (sqlite-vec).
**Code intelligence (5):** `code_symbols`, `symbol_edges`, `file_edges`, `execution_flows`, `flow_steps`.
**Knowledge graph (2):** `entities`, `relationships`.
**Learning & intelligence (9):** `search_telemetry`, `bandit_stats`, `config_versions`, `consolidations`, `importance_scores`, `workspace_profiles`, `global_learning`, `llm_cache`, `token_usage`.
**Query pattern detection (2):** `query_chain_membership`, `query_clusters`.

Triggers auto-maintain FTS5 index on insert / delete / hash-update.

#### 6.2.2 Per-workspace isolation

```
{dirName}-{first12CharsOfSha256(workspacePath)}.sqlite
```

Example: `nano-brain-a1b2c3d4e5f6.sqlite`.
The `project_hash` column further partitions data within each DB. Cross-workspace queries open multiple stores.

> **Known issue (2026-05-22):** `POST /api/query` and `POST /api/search` ignore the client's CWD and use the server-startup `currentProjectHash`. CLI does not send a workspace identifier. Tracked for OpenSpec proposal.

#### 6.2.3 Content-addressed storage

Every chunk: `hash = SHA-256(content)`, stored once in `content`, referenced by `documents.hash`. Identical content across documents shares a single row. `chunks` records `(hash, seq, pos, startLine, endLine)`.

#### 6.2.4 Corruption detection & recovery

- `PRAGMA quick_check` runs in `createStore()` before any DB ops (50–500 ms).
- On corruption: rename file to `index.db.corrupted.{ISO-timestamp}` (last 5 retained), delete `-wal` / `-shm`, reapply schema with pragmas (`journal_mode=WAL`, `foreign_keys=ON`, `busy_timeout=15s`, `synchronous=NORMAL`), run migrations, verify, emit metric `database_corruption_detected`.
- launchd (macOS) auto-restarts on fatal exit (10 s throttle).
- Recovery is safe because the DB is a derivable cache: sessions re-harvest, codebase re-indexes, vectors regenerate.

### 6.3 Search pipeline

Hybrid retrieval over BM25 + vector + symbol-name match, fused and reranked.

| Stage | Detail | Default |
|---|---|---|
| 1. Query expansion | 2–3 variants, weight 1.0× each, original 2.0× | provider stub, off |
| 2. BM25 (FTS5) | porter stemmer, 5 s timeout | — |
| 3. Vector search | sqlite-vec or Qdrant, cosine, 5 s timeout | — |
| 4. Symbol search | exact-name match against `code_symbols` | — |
| 5. RRF fusion | `Σ weight / (k + rank + 1)` | k = 60 |
| 6. Centrality boost | `× (1 + w · centrality)` | w = 0.1 |
| 7. Supersede demotion | `× supersede_demotion` if replaced | 0.05 |
| 8. Usage boost | `log₂(1+access) · 1/(1 + days/30)` | weight 0.15 |
| 9. Length normalization | `× 1/(1 + log₂(chars/anchor))` | anchor 2000 chars |
| 10. Recency boost | exponential half-life | weight 0.3, half-life 180 d |
| 11. Category weights | `auto:*` / `llm:*` tag multipliers | 0.5×–2.0× from preference learning |
| 12. Importance boost | document importance score | optional |
| 13. Top-K selection | candidate pool for reranker | top_k = 15 |
| 14. Neural reranking | VoyageAI `rerank-2.5-lite`, cached | — |
| 15. Position-aware blend | top 3: 75/25, ranks 4–10: 60/40, ranks 11+: 40/60 | RRF / rerank |
| 16. Output | `limit=10`, snippets ≤ 700 chars | — |

Snippets are enriched with symbol info, cluster labels, and flow counts when applicable.

### 6.4 Code intelligence

#### Symbol extraction
- Languages: TypeScript, JavaScript, Python (Tree-sitter).
- Kinds: function, class, method, interface, type, enum, variable, property.
- Per-symbol: file path, start/end line, exported boolean.

#### Call graph (`symbol_edges`)
- Edge types: `CALLS` (caller → callee), `IMPORTS`, `EXPORTS`, `INHERITS`.
- Confidence per edge (0–1, default 1.0 for AST-detected).

#### File dependency graph (`file_edges`)
- Imports / exports between files; transitive deps derivable.
- PageRank centrality (damping 0.85, 100 iters) → `documents.centrality`.

#### Louvain clustering
- Activates at ≥ 20 nodes, modularity-maximizing community detection → `clusters` + `code_symbols.cluster_id`.

#### Call-flow detection
- Entry points = exported functions with no callers.
- BFS to leaves: max depth 10, max branching 4, min steps 2, max 75 flows / project.
- Flow types: `intra_community` / `cross_community`.

#### Cross-repo infrastructure symbols
Tracked in `code_symbols` with type tags:
- `redis_key` (read / write operations)
- `pubsub_channel` (publish / subscribe)
- `mysql_table` (column-level, read / write)
- `api_endpoint` (Express / FastAPI; method + path)
- `http_call` (outbound calls)
- `bull_queue` (produce / consume)
- GraphQL types, queries, mutations

Queryable via `memory_symbols` + `memory_impact` MCP tools, or `nano-brain symbols` / `impact` CLI.

### 6.5 Knowledge graph

LLM-extracted entities and relationships, surfaced as a queryable graph.

| Aspect | Detail |
|---|---|
| Entity types (7) | `person`, `tool`, `concept`, `api`, `decision`, `pattern`, `library` |
| Relationship types (6) | `uses`, `depends_on`, `decided_by`, `related_to`, `replaces`, `configured_with` |
| Document-level relationships (8) | `supports`, `contradicts`, `extends`, `supersedes`, `related`, `caused_by`, `refines`, `implements` |
| Storage | `entities`, `relationships` tables, per-workspace |
| Extraction | LLM-driven via configured provider; runs every 30 min, expedited to 5 s when ≥ 10 docs pending |
| Pruning | Soft delete contradicted (TTL 30 d) and orphan (TTL 90 d) entities; hard delete after 30 d retention; 6 h cycle |
| Traversal | BFS via SQL joins, exposed through `memory_graph_query`, `memory_related`, `memory_timeline`, `memory_connections` |

### 6.6 Self-learning & adaptation

Six learning systems run continuously without manual tuning.

#### Thompson Sampling bandits (`bandits.ts`)
- Tunes `rrf_k`, `centrality_weight`, `top_k`.
- 3–5 variants per parameter; reward = result expansion event.
- Beta(successes, failures) sampling; dampening factor 0.1.
- Default off (`learning.enabled = true` enables); update every 10 min.

#### Preference learning (`preference-model.ts`)
- Tracks expand rates per `auto:*` / `llm:*` category per workspace.
- Multiplier range 0.5×–2.0×; baseline expand rate 10 %.
- Cold start: first 20 queries use neutral weights.
- Updates every 10 min via learning cycle.

#### Importance scoring (`importance.ts`)
- `importance = 0.4·usage + 0.2·entity_density + 0.2·recency + 0.2·connections`.
- 30-day half-life on usage decay.
- Default off (opt-in via config); rescores every 30 min.

#### Intent classification (`intent-classifier.ts`)
- 4 intents: `lookup`, `explanation`, `architecture`, `recall`.
- Keyword-triggered; per-intent overrides on `centrality_weight` / `rrf_k`.
- Default off.

#### Query expansion (`expansion.ts`)
- Pipeline ready; **no active provider** as of 2026.8.19.
- Generates 2–3 variants when enabled.

#### Consolidation (`jobs/consolidation.ts`)
- LLM summarizes related memories every 1 h.
- Batch ≤ 20 memories, requires ≥ 2, confidence ≥ 0.6.
- Stores in `consolidations` table linking source memories.

#### Query sequence detection (`sequence-analyzer.ts`)
- Groups queries within 5-min windows; clusters via embeddings (50 clusters); learns Markov transitions.
- Predictions activate at 50+ queries; max 5 suggestions, confidence ≥ 0.3.
- Rebuilds every 30 min.

#### Categorization (auto + LLM)
- **Keyword (`auto:*`):** synchronous, 7 categories (decision, pattern, tool, architecture, debugging, workflow, context).
- **LLM (`llm:*`):** async, fire-and-forget; confidence ≥ 0.6, content < 2 000 chars.

### 6.7 Background jobs

All jobs run inside a single `startWatcher()` instance. Intervals are configurable.

| # | Job | Default interval | Trigger |
|---|---|---|---|
| 1 | File reindex | 300 s | Dirty flag |
| 2 | Session harvest | 120 s | Timer |
| 3 | Embedding | 60 s (adaptive ≤ 300 s) | Pending docs |
| 4 | Learning cycle (bandits + prefs) | 600 s | Timer |
| 5 | Consolidation | 3600 s | Timer |
| 6 | Importance scoring | 1800 s | Timer |
| 7 | Sequence analysis | 1800 s | Timer |
| 8 | Entity extraction | 1800 s (5 s expedited if ≥ 10 pending) | Timer |
| 9 | Pruning soft-delete | 21 600 s (6 h) | Timer |
| 10 | Pruning hard-delete | 604 800 s (7 d, hardcoded) | Timer |

Maintenance mode (`POST /api/maintenance/prepare` / `/resume`) pauses watcher and checkpoints WAL for backups.

### 6.8 Provider abstractions

#### Embedding providers

| Provider | Config | Dim | Max chars | Notes |
|---|---|---|---|---|
| **VoyageAI** | `provider: openai`, `url: https://api.voyageai.com` | 1024 | 8000 | `voyage-code-3` recommended; needs `VOYAGE_API_KEY` |
| **Ollama** | `provider: ollama`, `url: http://localhost:11434` | auto | auto | Local, free, no key |
| **OpenAI-compatible** | `provider: openai`, custom `url` | model-dep. | model-dep. | Azure, LM Studio, etc. |

Default concurrency: **3** (override via `NANO_BRAIN_EMBEDDING_CONCURRENCY`).

#### Reranker
- **VoyageAI `rerank-2.5-lite`** — only supported reranker. Falls back to RRF-only blend if unavailable.

#### Vector stores
- **Qdrant** (production) — bundled in `docker compose`, ports 6333 (gRPC) / 6334 (HTTP).
- **sqlite-vec** (embedded) — default if Qdrant absent. Migration tool: `nano-brain qdrant migrate`.

#### LLM providers (consolidation, extraction, categorization)
- OpenAI-compatible (default `gpt-4o-mini`).
- Ollama (set `provider: ollama`).

### 6.9 Chunking strategy

Heading-aware markdown chunking (`chunker.ts`).

- Target size **3600 chars** (~900 tokens).
- Overlap **200 chars** (~15 %).
- Min chunk **200 chars**.
- Search window **800 chars** for best break point.

Break-point scoring:
| Break | Score |
|---|---|
| `# H1` | 100 |
| `## H2` | 90 |
| `### H3` / code fence | 80 |
| H4–H6 | 70 |
| Horizontal rule | 60 |
| Blank line | 20 |
| List item | 5 |
| Newline | 1 |

Code fences are tracked: cuts inside fences prefer the nearest fence boundary. Each chunk content-addressed via SHA-256.

### 6.10 CLI surface

29 commands, dispatched in `src/cli/index.ts`. Container-aware: when running inside Docker, search/write/reindex commands proxy to `http://host.docker.internal:3100` instead of opening local SQLite.

**Global options:** `--db=<path>`, `--config=<path>`, `--help`, `--version`. Env overrides: `NANO_BRAIN_DB_PATH`, `NANO_BRAIN_HOST`, `NANO_BRAIN_PORT`, `NANO_BRAIN_LOG`, `NANO_BRAIN_EMBEDDING_CONCURRENCY`, `OPENCODE_STORAGE_DIR`.

| Group | Commands |
|---|---|
| **Setup** | `init [--root --force --all]`, `setup` |
| **Server** | `mcp [--http --port --host --daemon \| stop]` |
| **Search** | `search`, `vsearch`, `query` (each: `-n -c --json --files --compact --scope=all --tags --since --until`; `query` adds `--min-score`) |
| **Memory** | `write [--tags --supersedes]`, `get`, `tags` |
| **Index** | `update`, `reindex [--root]`, `embed [--force]` |
| **Collections** | `collection {add\|remove\|list\|rename}` |
| **Cleanup** | `reset [--databases --sessions --memory --logs --vectors --confirm --dry-run]`, `rm <ws> [--list --dry-run]`, `harvest` |
| **Docker** | `docker {start\|stop\|restart [svc]\|status}` |
| **Qdrant** | `qdrant {up\|down\|status\|migrate\|verify\|activate\|cleanup\|recreate}` |
| **Cache** | `cache {clear [--all --type]\|stats}` |
| **Logs** | `logs [-f -n --date --clear \| path]` |
| **Code intel** | `context <sym>`, `code-impact <sym> [--direction --max-depth --min-confidence --file]`, `detect-changes [--scope]`, `focus <file>`, `graph-stats`, `symbols [--type --pattern --repo --operation]`, `impact --type --pattern` |
| **Status** | `status [--all]`, `wake-up [--workspace --json]` |
| **Intelligence** | `consolidate`, `categorize-backfill [--batch-size --rate-limit --dry-run --workspace]`, `learning rollback [version]` |
| **Bench** | `bench {generate\|run\|compare} [--scale --json --save --compare --force]` |
| **Maintenance** | `db:clean [--list-only --dry-run --confirm]` |

Default command (no args) → `mcp` in stdio mode.

### 6.11 MCP tool surface

24 tools, registered in `src/mcp/`. All accept optional `workspace` parameter (hash, path, or `"all"`); daemon mode requires it for write operations.

#### Search & retrieval (5)

| Tool | Purpose |
|---|---|
| `memory_search` | BM25 only; fast exact keyword. |
| `memory_vsearch` | Vector only; semantic similarity (FTS fallback if embedder missing). |
| `memory_query` | Full hybrid pipeline (6 ranking signals + reranker). |
| `memory_expand` | Expand a compact result via cache key + index. |
| `memory_get` | Retrieve doc by path or `#docid` (line range supported). |

#### Memory management (9)

| Tool | Purpose |
|---|---|
| `memory_write` | Append to daily log; auto-tag, async LLM categorization, proactive related-memory surfacing. |
| `memory_tags` | List all tags with counts. |
| `memory_status` | Index health, embedder/vector status, codebase stats, learning stats. |
| `memory_update` | Reindex all collections immediately. |
| `memory_wake_up` | Compact session-start briefing (~200–500 tokens, JSON or text). |
| `memory_consolidate` | Trigger manual consolidation cycle. |
| `memory_consolidation_status` | Queue stats + recent activity. |
| `memory_importance` | Importance scores (placeholder; feature behind opt-in flag). |
| `memory_learning_status` | Telemetry, bandit variants, preference weights, prediction accuracy. |
| `memory_suggestions` | Predicted next queries (≥ 50 queries gate) or workspace insights if no context. |

#### Code intelligence (5)

| Tool | Purpose |
|---|---|
| `memory_focus` | File centrality + cluster + dependents/dependencies (max 30 each). |
| `memory_symbols` | Cross-repo infra symbol query (Redis / MySQL / API / queue / etc.). |
| `memory_impact` | Cross-repo impact (readers vs writers, producers vs consumers). |
| `code_context` | 360° symbol view: callers, callees, cluster, flows, infra. |
| `code_impact` | BFS upstream / downstream with depth + confidence filters. |

#### Knowledge graph (5)

| Tool | Purpose |
|---|---|
| `memory_graph_stats` | File-deps stats, top-centrality files, cycles. |
| `memory_graph_query` | Entity-graph BFS with optional relationshipTypes filter. |
| `memory_related` | Topic-related memories with entity context. |
| `memory_timeline` | Chronological view of memories for a topic (active vs superseded). |
| `memory_connections` | Document → document relationships, filterable by type and direction. |

#### Indexing (1)

| Tool | Purpose |
|---|---|
| `memory_index_codebase` | Async background re-index (Tree-sitter, call graph, PageRank, clusters, flows, infra). |

### 6.12 HTTP API surface

29 endpoints (`src/http/routes.ts`). Mix of `/api/*` (legacy) and `/api/v1/*` (versioned). Workspace handling: `?workspace=<hash|path|all>` query param on read endpoints; write endpoints always use `currentProjectHash`.

#### Health & status (3)
- `GET /health` — `{status, ready, version, uptime, sessions, [corruption_recovered]}`.
- `GET /api/status` — full index health + workspace identity.
- `GET /api/vector-health` — `{provider, ok, vectorCount}` (5 s timeout).

#### Search (4)
- `POST /api/query` — hybrid (`{query, tags, scope, limit}`), 6 s timeout.
- `POST /api/search` — BM25 (`{query, limit}`), 8 s deadline / 5 s FTS timeout.
- `POST /api/vsearch` — vector (`{query, limit, workspace}`), 8 s timeout.
- `GET /api/v1/search` — same as POST search but query-string.

#### Memory operations (6)
- `POST /api/write` — `{content, tags, workspace}`.
- `GET /api/wake-up` / `POST /api/wake-up` — session briefing.
- `POST /api/reindex` — `{root}`, async.
- `POST /api/update`, `POST /api/v1/update` — reindex all collections, async.
- `POST /api/embed` — embed up to 50 pending chunks, async.

#### Maintenance (2)
- `POST /api/maintenance/prepare` — enter maintenance (5 min timeout, watcher off, WAL checkpoint).
- `POST /api/maintenance/resume` — exit maintenance.

#### Graphs & analysis (8)
- `GET /api/v1/status` — summary + workspace list.
- `GET /api/v1/workspaces` — full workspace listing with hashes + counts.
- `GET /api/v1/graph/entities` — knowledge graph nodes + edges + stats.
- `GET /api/v1/graph/stats` — file-deps stats + cycles.
- `GET /api/v1/graph/symbols` — symbol call graph (truncates at limit, prioritizes connected/exported/high-degree).
- `GET /api/v1/graph/flows` — detected + documented flows.
- `GET /api/v1/code/dependencies` — file deps for visualization.
- `GET /api/v1/graph/connections` — doc-to-doc relationship graph.

#### Telemetry & metadata (4)
- `GET /api/v1/telemetry` — query count, bandit stats, preference weights, expand rate.
- `GET /api/v1/graph/infrastructure` — grouped infra symbols (Redis / MySQL / API / queues).
- `GET /api/v1/tags` — tags with counts.
- `GET /api/v1/connections` — connections for a specific docId, by direction.

#### MCP transports (3)
- `GET /sse`, `POST /messages` — Server-Sent Events MCP transport.
- `GET /POST /mcp` — Streamable HTTP MCP transport with session IDs (30 s heartbeat).

#### Web (1)
- `GET /web` / `/web/**` — static assets from `dist/web/` (404 if not built).

### 6.13 Deployment modes

| Mode | Command | Use case |
|---|---|---|
| **Docker compose** (recommended) | `npx nano-brain docker start` | One-command full stack: nano-brain (port 3100) + Qdrant (6333/6334), 2 GB memory limit, restart `unless-stopped`, 30 s health check. |
| **Standalone HTTP daemon** | `npx nano-brain mcp --http --daemon --port=8282` | Single-process, PID at `~/.nano-brain/server.pid`. |
| **launchd (macOS)** | `launchctl load ~/Library/LaunchAgents/com.tamlh.nano-brain.plist` | OS-managed, auto-restart on exit (10 s throttle), survives corruption recovery. |
| **stdio MCP** | `npx nano-brain mcp` (default) | Direct integration with local agents (OpenCode, Claude Code). |
| **MCP remote (HTTP/SSE)** | client connects to `host.docker.internal:3100/mcp` or `/sse` | Container deployments, remote agents. |

Container detection: `/proc/1/cgroup` for Docker, `KUBERNETES_SERVICE_HOST` for K8s, `LAUNCHD_PROCESS_PID` for launchd. Adjusts host resolution (`host.docker.internal` vs `localhost`) and stdio handling.

### 6.14 Web UI & dashboards

Static SPA served from `dist/web/` at `/web`. API-first design: dashboards consume `/api/*` and `/api/v1/*` endpoints.

Available views (when web bundle present):
- **Symbol graph viewer** — interactive dependency graph with Louvain clusters, node cap + truncation banner, focus mode.
- **Knowledge graph** — entity nodes + relationships, BFS traversal, type filter.
- **Symbol call graph** — function-level callers / callees with confidence edges.
- **Memory timeline** — temporal view by topic, supersession status.
- **Query dashboard** — telemetry, expand rates, learning variants.
- **Embedding progress** — pending queue, provider latency, coverage.

The `/agents-viewer/` static asset directory hosts agent-readable HTML reports.

### 6.15 AI agent integration

Two integration artifacts auto-loaded by MCP-aware agents:

#### `SKILL.md` (skill registration)
- Routing rules: when to use BM25 vs vector vs hybrid.
- Trigger phrases: "search memory", "what did we", "who calls this", etc.
- Slash commands: `/nano-brain-init`, `/nano-brain-status`, `/nano-brain-reindex`.
- Full CLI + MCP tool reference.

#### `AGENTS_SNIPPET.md` (project-level managed block)
- Injected into project's `AGENTS.md` via `<!-- OPENCODE-MEMORY:START -->` block.
- HTTP API quick reference, session start / end patterns.
- Container setup notes for SQLite isolation.
- Installed via `npx nano-brain init --root=<project>`.

### 6.16 Benchmarking suite

Reproducible quality + latency harness in `benchmarks/`.

**Scales:** 100 (smoke), 500, 1 000, 5 000, 10 000, 100 000.

**Quality metrics:**
- **P@5** (precision at 5) — regression threshold 0.10 drop.
- **R@10** (recall at 10) — regression threshold 0.10 drop.
- **MRR** (mean reciprocal rank) — regression threshold 0.05 drop.

**Latency:** insert + query, p50 / p95 — regression threshold 2× increase.

**Baseline (v2026.8.3, scale-100, Ollama local):**
| Mode | P@5 | R@10 | MRR | Insert p50 / p95 | Query p50 / p95 |
|---|---|---|---|---|---|
| FTS | 0.975 | 0.985 | 1.000 | 32 / 59 ms | 1 / 3 ms |
| Vector | 0.875 | 0.925 | 1.000 | 32 / 59 ms | 29 / 50 ms |
| Hybrid | 0.835 | 0.970 | 1.000 | 32 / 59 ms | 34 / 69 ms |

**Commands:** `bench generate --scale=N`, `bench run --scale=N`, `bench compare new.json baseline.json [--save --force]`.

**Combination tests:** write → reindex → query, supersede → query, harvest → reindex → search.

### 6.17 Logging & telemetry

#### Logging
- Levels: `error`, `warn`, `info` (default), `debug`.
- File: `~/.nano-brain/logs/nano-brain-YYYY-MM-DD.log`.
- Rotation: max 50 MB / file, max age 2 days, check every 60 s, keep 5 files.
- stdio mode (auto-enabled for MCP stdio transport): suppresses console output to protect JSON-RPC.

#### Telemetry
- All searches logged to `search_telemetry`: query, results, expand events, mode, latency.
- Retention 90 days (`telemetry.retention_days`).
- Powers Thompson Sampling, preference learning, sequence analysis.
- Inspect via `nano-brain status` or `GET /api/v1/telemetry`.

### 6.18 Configuration system

YAML at `~/.nano-brain/config.yml`, auto-generated from `config.default.yml`. See [§7](#7-configuration-reference) for the field reference.

---

## 7. Configuration reference

### 7.1 Top-level sections

| Section | Key fields | Default |
|---|---|---|
| `logging` | `enabled`, `level`, `file`, `maxSize`, `maxFiles` | enabled, `info`, 10 MB, 5 files |
| `collections` | `{name: {path, pattern, update, excludeFolders}}` | `memory`, `sessions` (both `**/*.md`) |
| `vector` | `provider`, `url`, `collection` | `qdrant`, `http://qdrant:6333`, `nano-brain` |
| `embedding` | `provider`, `url`, `model`, `apiKey`, `dimensions`, `maxChars` | Ollama `nomic-embed-text` or VoyageAI `voyage-code-3` |
| `reranker` | `model`, `apiKey`, `provider` | `rerank-2.5-lite` |
| `codebase` | `enabled`, `languages`, `exclude`, `maxFileSize` | true, [ts/js/py], standard, 1 MB |
| `watcher` | `debounce`, `reindexInterval`, `chokidarIntervalMs` | 300 ms, 300 s, 5000 ms |
| `intervals` | `sessionPoll`, `healthCheck` | 120 s, 60 s |
| `storage` | `maxSize`, `retention.{sessions,logs}` | 10 GB, 90 d, 30 d |
| `telemetry` | `enabled`, `retention_days` | true, 90 |
| `learning` | `enabled`, `update_interval_ms` | true, 600 000 |
| `consolidation` | `enabled`, `interval_ms`, `model`, `endpoint`, `apiKey`, `provider`, `max_memories_per_cycle`, `min_memories_threshold`, `confidence_threshold` | false, 3 600 000, `gpt-4o-mini`, OpenAI, 20, 2, 0.6 |
| `extraction` | `enabled`, `model`, `endpoint`, `apiKey`, `maxFactsPerSession` | false, `gpt-4o-mini`, OpenAI, 20 |
| `categorization` | `llm_enabled`, `confidence_threshold`, `max_content_length` | true, 0.6, 2000 |
| `preferences` | `enabled`, `min_queries`, `weight_min`, `weight_max`, `baseline_expand_rate` | true, 20, 0.5, 2.0, 0.1 |
| `pruning` | `enabled`, `interval_ms`, `contradicted_ttl_days`, `orphan_ttl_days`, `batch_size`, `hard_delete_after_days` | true, 21 600 000, 30, 90, 100, 30 |
| `intents` | `enabled`, per-intent overrides | false |
| `proactive` | `enabled`, `chain_timeout_ms`, `min_queries_for_prediction`, `max_suggestions`, `confidence_threshold`, `cluster_count`, `analysis_interval_ms` | true, 300 000, 50, 5, 0.3, 50, 1 800 000 |
| `importance` | `enabled`, `weight`, `decay_half_life_days`, `formula_weights` | false, 0.1, 30, `{usage:0.4, entity_density:0.2, recency:0.2, connections:0.2}` |
| `workspaces` | `isolation`, `defaultScope` | true, `current` |
| `harvester` | `opencode.{enabled,sessionDir}`, `claudeCode.enabled` | true / auto, false |

### 7.2 Search config defaults (selected)

| Key | Default |
|---|---|
| `search.rrf_k` | 60 |
| `search.top_k` | 15 |
| `search.centrality_weight` | 0.1 |
| `search.supersede_demotion` | 0.05 |
| `search.usage_boost_weight` | 0.15 |
| `search.length_norm_anchor` | 2000 |
| `search.recency_weight` | 0.3 |
| `search.recency_half_life_days` | 180 |
| `search.expansion.enabled` / `.weight` | false / 1.0 |
| `search.blending.top3` | rrf 0.75 / rerank 0.25 |
| `search.blending.mid` | rrf 0.60 / rerank 0.40 |
| `search.blending.tail` | rrf 0.40 / rerank 0.60 |

### 7.3 Environment variables

| Var | Purpose |
|---|---|
| `NANO_BRAIN_DB_PATH` | Override default DB path |
| `NANO_BRAIN_HOST` | HTTP host (default: `localhost` / `host.docker.internal` in container) |
| `NANO_BRAIN_PORT` | HTTP port (default: 3100) |
| `NANO_BRAIN_LOG` | Force file logging |
| `NANO_BRAIN_EMBEDDING_CONCURRENCY` | Parallel Ollama requests (default 3) |
| `NANO_BRAIN_APP` | Path to nano-brain source (compose) |
| `NANO_BRAIN_HOME` | Data dir (default `~/.nano-brain`) |
| `NANO_BRAIN_WORKSPACE` | Workspace to mount in container |
| `OPENCODE_STORAGE_DIR` | Override OpenCode session dir |
| `OPENCODE_DATA_DIR` | OpenCode data dir (read-only mount) |
| `NODE_OPTIONS` | Node memory limit (default `--max-old-space-size=1536`) |
| `VOYAGE_API_KEY` | VoyageAI key (referenced from config) |
| `CONSOLIDATION_API_KEY` | Override consolidation LLM key |

---

## 8. Operational requirements

### 8.1 Resource budget (single workspace)

| Resource | Typical | Limit |
|---|---|---|
| Node heap | ~ 500 MB | 1.5 GB (`NODE_OPTIONS=--max-old-space-size=1536`) |
| Container memory | ~ 800 MB | 2 GB (compose) |
| SQLite DB size | ~ 100 MB / 10 K docs | `storage.maxSize` (10 GB default) |
| Qdrant disk | ~ 50 MB / 10 K vectors | depends on Qdrant config |
| Embedding queue | drains at ~ 3 / s (Ollama local) | adaptive backoff |

### 8.2 Reliability

- **Corruption recovery:** automatic, < 1 s in normal cases. Backup retained as `.corrupted.{timestamp}`.
- **Auto-restart:** launchd (10 s throttle on macOS); `restart: unless-stopped` in compose.
- **Health endpoint:** `GET /health` returns `{ready: bool}`; container health check every 30 s.
- **Maintenance mode:** `POST /api/maintenance/prepare` for safe backups (5 min timeout).

### 8.3 Privacy

- Default Docker stack: 100% local (Ollama + Qdrant). No outbound calls.
- Cloud providers (VoyageAI, OpenAI) opt-in via config.
- No telemetry phoning home. All logs and telemetry stay on disk.

### 8.4 Multi-workspace behaviour

- Each workspace gets isolated SQLite at `~/.nano-brain/data/{name}-{hash}.sqlite`.
- `scope=all` (CLI) / `workspace=all` (MCP/HTTP read endpoints) opens multiple stores and RRF-fuses results.
- Write operations always target `currentProjectHash` (never cross-write).

### 8.5 Compatibility

- Node.js ≥ 22 (uses `node:22-slim` in compose).
- Tested on Linux + macOS. Windows not officially supported.
- Tree-sitter languages: TypeScript, JavaScript, Python.
- MCP SDK: `@modelcontextprotocol/sdk` (stdio + HTTP/SSE transports).

---

## 9. Open issues & known limitations

### 9.1 Workspace routing in `/api/query` and `/api/search` (active bug, 2026-05-22)

**Symptom:** `npx nano-brain query "x"` from inside a Docker container returns results from the server-startup workspace, regardless of client CWD.

**Root cause:**
- CLI (`src/cli/commands/search.ts`) sends only `{query, limit, tags, scope}` — no workspace identifier.
- Server (`src/http/routes.ts:176`) hardcodes `effectiveProjectHash = scope === 'all' ? 'all' : currentProjectHash`. Other endpoints accept `?workspace=<hash>`, these two don't.

**User-visible:** misinterpreted as "creates a new database"; actually a routing bug.

**Tracked for OpenSpec proposal:** fix CLI to send hashed CWD; extend `/api/query` + `/api/search` to honor `workspace` from body.

### 9.2 Query expansion has no active provider

The expansion pipeline is wired (RRF accepts variants, weights are configured at 1.0×) but no provider is registered. `expansion.enabled = false` by default.

### 9.3 `memory_importance` returns placeholder

The MCP tool currently returns `"Importance scoring not yet active..."` even though the scoring infrastructure (`importance.ts`, `importance_scores` table, background job) exists. Activation gated on importance config opt-in.

### 9.4 No incremental call-graph updates

Codebase reindex rebuilds the symbol graph from scratch. For very large repos (> 100 K LoC) this is the dominant cost. Hash-based file skip avoids re-parsing unchanged files but graph computation (PageRank, Louvain, flows) is global.

### 9.5 Tree-sitter language coverage

Only TypeScript, JavaScript, Python. Go, Rust, Java, C#, Ruby, PHP not supported. Cross-repo symbols (Redis / MySQL / API) are detected via pattern matching on parsed code, so they only work in supported languages.

### 9.6 Single-node only

SQLite is the storage primitive. No clustering, no read replicas. Workspaces scale by adding more SQLite files, not by sharding.

### 9.7 Web UI build is optional

`/web` returns 404 if `dist/web/` not built. Some deployments ship API-only.

---

## 10. Roadmap signals

These are derived from `openspec/changes/` (53 archived, plus active ones) and `docs/HARNESS_BACKLOG.md`. **Not a commitment** — read as direction signals.

- **Workspace routing fix** (§9.1) — CLI sends workspace, server endpoints honor body override.
- **Query expansion provider** — connect a real LLM expander (`expansion.ts` interface ready).
- **Importance scoring activation** — flip `importance.enabled` from opt-in to default once stable.
- **Knowledge-graph UI upgrade** — referenced in archived OpenSpec.
- **More languages** — Go and Rust are the most-requested Tree-sitter additions.
- **Cross-repo symbol enrichment** — GraphQL, Kafka, Postgres listen/notify.
- **Multi-tenant deployment story** — not a goal yet (§3 NG1) but pressure exists.

---

## 11. Glossary

| Term | Meaning |
|---|---|
| **BM25** | Probabilistic ranking function used by FTS5. Term-frequency × inverse-document-frequency with length normalization. |
| **RRF** | Reciprocal Rank Fusion: `Σ weight / (k + rank + 1)`. Combines result sets from heterogeneous searchers. |
| **PageRank** | Iterative centrality measure on a directed graph (damping 0.85, 100 iterations). |
| **Louvain** | Modularity-maximizing community-detection algorithm. |
| **Supersede** | Document marked as replaced by another via `--supersedes=<path|#docid>`. Demoted 0.05× in ranking. |
| **Centrality boost** | Multiplier applied to ranking score using PageRank centrality. |
| **Position-aware blend** | Final score blend of RRF rank vs reranker score, weighted by result position. |
| **Thompson Sampling** | Multi-armed bandit using Beta-distribution posterior sampling for exploration / exploitation. |
| **Workspace hash** | First 12 chars of SHA-256(workspaceRoot). Used in DB filename and `project_hash` column. |
| **Project hash** | Same as workspace hash. Partition key inside a multi-workspace SQLite file. |
| **Collection** | Logical grouping of documents (`memory`, `sessions`, plus user-added). |
| **Chunk** | Heading-aware slice of a document (target 900 tokens, 15% overlap). |
| **Content-addressed** | Storage where each blob is keyed by its SHA-256 hash. Enables dedup. |
| **MCP** | Model Context Protocol. JSON-RPC over stdio / HTTP / SSE for AI-tool exposure. |
| **Harvester** | Background job converting AI-agent JSON sessions into searchable markdown. |
| **Hard gate** | Risk-classification flag in the engineering harness (auth, data-model, etc.) that forces high-risk lane. |

---

*End of document. Source last verified at git HEAD on 2026-05-22.*
