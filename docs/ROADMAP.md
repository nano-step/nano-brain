# nano-brain Roadmap

> Last updated: 2026-06-22

---

## Vision

nano-brain is a persistent memory and code intelligence layer for AI coding agents.
Goal: agents know the project context, decision history, and can anticipate what's needed next — across sessions, machines, and team members.

---

## Pillar 1: Code Intelligence

**What:** Understand the codebase like a senior engineer.

| Feature | Description | Status |
|---|---|---|
| File indexing | Watch + chunk + embed toàn bộ source files | ✅ |
| Symbol extraction | Functions, types, interfaces, constants | ✅ |
| Knowledge graph | Module → function → dependency relationships | ✅ |
| Impact analytics | Change X → affects Y, Z (cross-file) | ✅ |
| Call chain tracing | Trace execution path from entry point | ✅ |
| Control-flow graphs | CFG extraction with branch-aware edges | ✅ |
| Sequence diagrams | Mermaid sequence diagrams from flow data | ✅ |
| Ruby/Rails support | Rails routes, controller→service→model chains | ✅ |
| Ruby cross-file resolution | Class→file index, resolver, reconcile edges | ✅ |
| Ruby CFG extraction | `if`/`else`, loops, `begin`/`rescue`, method defs | ✅ |

---

## Pillar 2: Session Harvesting

**What:** Collect and summarize sessions from AI tools, scoped per workspace.

| Feature | Description | Status |
|---|---|---|
| OpenCode SQLite harvester | Parse `opencode.db`, extract sessions/messages | ✅ |
| Claude Code JSONL harvester | Parse `ses_*.jsonl` transcripts | ✅ |
| Workspace filtering | Only harvest sessions matching registered workspace paths | ✅ |
| LLM summarization pipeline | Map-reduce chunking, token-bucket rate limiter | ✅ |
| Incremental harvest | Track last-harvested timestamp, dedup by session ID | ✅ |
| Summary persistence | `.md` files + vector DB (`session-summary` collection) | ✅ |
| Embed queue workspace isolation | Queue scan scoped to registered workspaces only | ✅ |

### Architecture

```
opencode.db / ses_*.jsonl
  → filter by workspace path
  → extract messages
  → map-reduce LLM summary (token-bucket rate limited)
  → chunk → embed → index (session-summary collection)
  → .md summary file to output_dir
```

### Config

```yaml
harvester:
  opencode:
    session_dir: ~/.local/share/opencode/storage
  claudecode:
    enabled: false
    session_dir: ~/.claude/transcripts/

summarization:
  enabled: true
  provider_url: "https://ai-proxy.example.com/v1"
  model: "claude-sonnet-4-5"
  max_tokens: 4096
  concurrency: 3
  output_dir: "~/.nano-brain/summaries"
```

---

## Pillar 3: Memory & Developer Experience

**What:** Persistent cross-session memory + ergonomic tooling.

| Feature | Description | Status |
|---|---|---|
| Write memory | `nano-brain write "..."` | ✅ |
| Semantic search | `nano-brain query "..."` | ✅ |
| Tag-based filter | `--tags decision,auth` | ✅ |
| Supersede | Replace stale memory entries | ✅ |
| Auto-memory from sessions | Extract decisions from harvested sessions | ✅ |
| 14 MCP tools | query, search, vsearch, get, write, tags, status, update, wake_up, graph, trace, impact, symbols, flow | ✅ |
| Hybrid search pipeline | BM25 + pgvector HNSW + RRF fusion + recency decay | ✅ |
| BM25 OR fallback | Retry with OR semantics when AND returns 0 results | ✅ |
| Debugging-aware search | Parallel search mode for debugging queries | ✅ |
| Incoming edges symbol fallback | Fallback to symbol name when target_node lookup fails | ✅ |
| Benchmarking suite | generate, run, compare, stress | ✅ |
| Workspace-specific benchmarks | Queries tailored to each project's domain | ✅ |
| Init onboarding wizard | Interactive config setup on first run | ✅ |
| Doctor command | Check prerequisites (PG, pgvector, Ollama, model) | ✅ |
| V1 SQLite migration | Import from V1 format (pure Go, no CGO) | ✅ |
| Config hot-reload | `POST /api/reload-config` | ✅ |
| Search telemetry | Local-only, 90-day retention, non-blocking | ✅ |

---

## Pillar 4: Team & Multi-user

**What:** Shared knowledge base for the whole team — one server, multiple users, role-controlled access.

**Use case:** Deploy one nano-brain server for the entire team. Every developer's AI agent connects to the same PostgreSQL instance — decisions, architecture notes, and code intelligence are instantly shared. New team members get full project context from day one without any per-machine setup.

### Authentication

| Method | Description | Status |
|---|---|---|
| Bearer token auth | Single shared token for all users | ✅ |
| Basic auth | Username/password per user | ✅ |
| TLS termination | HTTPS support (native or reverse proxy) | ❌ |
| Rate limiting | Per-user, per-IP request limits | ❌ |
| CORS configuration | Restrict allowed origins | ❌ |

### Authorization (Role-Based Access Control)

| Role | Description | Status |
|---|---|---|
| Admin | Full read/write + config + workspace management | ❌ |
| Developer | Read/write memory, scoped to assigned workspaces | ❌ |
| Reader | Read-only access (search, get, wake-up, status) | ❌ |

### Role Matrix

| Operation | Admin | Developer | Reader |
|---|---|---|---|
| `memory_query` / `memory_search` / `memory_vsearch` | ✅ | ✅ | ✅ |
| `memory_get` / `memory_wake_up` / `memory_status` | ✅ | ✅ | ✅ |
| `memory_write` / `memory_update` | ✅ | ✅ | ❌ |
| `memory_graph` / `memory_impact` / `memory_trace` | ✅ | ✅ | ✅ |
| Workspace init / delete | ✅ | ❌ | ❌ |
| Config reload / patch | ✅ | ❌ | ❌ |
| Collection create / delete | ✅ | ✅ | ❌ |
| Reindex / harvest | ✅ | ✅ | ❌ |

### Deployment Options

| Option | Description | Status |
|---|---|---|
| Local machine | Ollama + Docker, single user | ✅ |
| VPS / team server | Shared memory across machines | ✅ |
| Build from source | Go binary, no CGO | ✅ |
| Docker Compose | Production-ready container setup | ❌ |
| Kubernetes / Helm | Cloud-native deployment | ❌ |
| Cloud managed | AWS RDS, GCP Cloud SQL, Azure DB | ❌ |

### Config sketch (proposed)

```yaml
server:
  auth:
    enabled: true
    users:
      - username: alice
        password_hash: "$2a$10$..."
        role: admin
      - username: bob
        password_hash: "$2a$10$..."
        role: developer
        workspaces: ["abc123..."]  # scoped to specific workspaces
      - username: reviewer
        password_hash: "$2a$10$..."
        role: reader
    tokens:
      - token: "nbt_admin_..."
        role: admin
      - token: "nbt_dev_..."
        role: developer
        workspaces: ["abc123..."]
      - token: "nbt_readonly_..."
        role: reader
  rate_limit:
    enabled: true
    requests_per_minute: 60
    burst: 10
  cors:
    enabled: true
    allowed_origins:
      - "https://app.example.com"
```

---

## Pillar 5: Self-Learning & Prediction

**What:** Learn patterns from user behavior → prepare context proactively.

> ⚠️ Needs further design discussion on scope and approach.

### 5a. Pattern Learning
- Analyze prompt history from harvested sessions
- Identify recurring workflows (e.g., "user typically fixes bug → runs tests → commits")
- Build user-specific workflow graph

### 5b. Proactive Context Pre-loading
- Based on current task → predict what's needed next
- Pre-fetch relevant code symbols, memory entries, past decisions
- Surface as "you might need next: ..."

### 5c. Self-Lesson Extraction
- After each session: extract lessons ("what worked", "what failed")
- Store as tagged memory entries
- Surface relevant lessons when starting a similar task

### 5d. Auto-execution (Stretch)
- nano-brain autonomously triggers the next step without a user prompt
- Requires: high confidence prediction + explicit user opt-in flag
- Risk: false positives — needs confidence threshold

---

## Implementation Order

```
Phase 1 — Foundation ✅ (shipped 2026-05)
  ├── File indexing, watcher, chunking, embedding
  ├── OpenCode SQLite harvester
  ├── Claude Code JSONL harvester
  └── Workspace registration + isolation

Phase 2 — Code Intelligence ✅ (shipped 2026-05)
  ├── Symbol extraction (regex-based)
  ├── Knowledge graph (module → function → dependency)
  ├── Impact analytics (cross-file change propagation)
  └── Call chain tracing

Phase 3 — Memory & DX ✅ (shipped 2026-05)
  ├── Hybrid search (BM25 + vector + RRF + recency)
  ├── MCP tools (9 tools)
  ├── Session summarization pipeline
  ├── Workspace filtering for harvest + embed
  ├── Init onboarding, doctor, benchmarks
  └── V1 migration, config hot-reload, telemetry

Phase 4 — Hardening (mostly shipped)
  ├── ✅ #180 — Ollama context length overflow on large chunks (PR #208/#209)
  ├── ✅ #181 — UTF-8 null byte in harvested sessions
  ├── ⚠️ #184 — Require explicit --workspace on CLI commands (partial: only reset-embeddings)
  ├── ❌ #158 — Incremental reindex (only changed files) — still full reindex
  ├── ✅ #190 — cleanup-stale-raw command
  └── ✅ #191 — Summarization max_tokens default 4096 → 8000

Phase 5 — CLI Completeness (in progress)
  ├── ✅ #153 — Code intelligence CLI (context, code-impact, detect-changes)
  ├── ⚠️ #151 — Wake-up (REST API + MCP done; CLI command pending)
  ├── ❌ #152 — get, tags, multi-get commands
  ├── ❌ #155 — Workspace remove command (SQL query exists, no CLI/handler)
  ├── ❌ #156 — Cross-workspace search (--scope=all)
  ├── ❌ #157 — Cache management (clear, stats)
  └── ⚠️ #160 — --tags filter (works on write; pending on query/search)

Phase 6 — Enhanced Code Intelligence (in progress)
  ├── ✅ #174 — Symbol extraction with go-tree-sitter (Python extractor shipped)
  ├── ✅ Ruby/Rails flow & sequence diagrams (PRs #467, #469, #471, #473)
  │   ├── Ruby CFG extraction (if/else, loops, begin/rescue, method defs)
  │   ├── Ruby call graph extractor (class/module capture, unresolved edges)
  │   ├── Rails route extraction (resources, get/post/patch/put/delete, namespace)
  │   ├── Ruby class→file index with namespace preference
  │   ├── Cross-file resolver with reconcile edge builder
  │   └── Flows reach 20-34 nodes (entry → handler → func → calls chain)
  └── ⚠️ Cross-language support — Python, Ruby via tree-sitter; TypeScript, Rust pending

Phase 7 — Team & Multi-user (Planned)
  ├── Role-based access control (Admin / Developer / Reader)
  ├── Per-token and per-user role assignment in config
  ├── Read-only enforcement at middleware layer
  └── Audit log (who wrote/deleted what)

Phase 8 — Self-Learning (Discuss)
  ├── #154 — Memory consolidation + categorization + Thompson Sampling
  ├── Pattern learning from prompt history
  ├── Proactive context pre-loading
  └── Self-lesson extraction

Phase 9 — Agent Memory Benchmarking ✅ (shipped 2026-06)
  ├── ✅ Benchmark framework (20 queries, ground truth, 6 tool runners)
  ├── ✅ Competitor comparison (LlamaIndex, Qdrant/Mem0)
  ├── ✅ Fair comparison with same raw source files
  ├── ✅ Workspace-specific queries (gaming-platform, nano-brain, rails-project)
  ├── ✅ BM25 OR fallback for zero-result queries
  ├── ✅ Results: nano-brain P@5=0.749, MRR=0.967
  └── ✅ Known issue: 2 rails-project queries still return 0

Phase 10 — Deployment & Security (Planned)
  ├── Deployment guides
  │   ├── ✅ Local machine (Ollama + Docker, ~5 min)
  │   ├── ✅ VPS / team server (shared memory across machines)
  │   ├── ✅ Build from source
  │   ├── ⚠️ Docker Compose production setup
  │   ├── ❌ Kubernetes / Helm chart
  │   ├── ❌ Cloud provider guides (AWS, GCP, Azure)
  │   └── ❌ CI/CD integration (GitHub Actions, GitLab CI)
  ├── Authentication & authorization
  │   ├── ✅ Bearer token auth (single shared token)
  │   ├── ✅ Basic auth (username/password per user)
  │   ├── ❌ Role-based access control (Admin / Developer / Reader)
  │   ├── ❌ Per-token role assignment (each `nbt_` token carries a role)
  │   ├── ❌ Per-user basic auth role (role assigned per username)
  │   ├── ❌ Workspace-scoped access (Developer role limited to assigned workspaces)
  │   └── ❌ Audit log (who wrote/deleted what, when)
  ├── Security hardening
  │   ├── ❌ Rate limiting (per-user, per-IP)
  │   ├── ❌ Request size limits (prevent abuse)
  │   ├── ❌ CORS configuration (restrict origins)
  │   ├── ❌ TLS termination (HTTPS support)
  │   ├── ❌ Input validation & sanitization
  │   └── ❌ Secrets management (env vars, not config files)
  └── Observability
      ├── ✅ Search telemetry (local-only, 90-day retention)
      ├── ❌ Prometheus metrics endpoint
      ├── ❌ Structured logging with request IDs
      ├── ❌ Health check enhancements (dependency checks)
      └── ❌ Distributed tracing (OpenTelemetry)
```

---

## Open Questions

1. **Pillar 4 scope**: Proactive suggestions only, or auto-execution? (Needs design discussion)
2. **Cross-workspace search**: #156 — privacy implications of searching across all workspaces?
3. **Memory consolidation**: #154 — Thompson Sampling for relevance ranking — need benchmarks first?
4. **Ruby limitations**: No `before_action`/`after_action`, no ActiveRecord dynamic methods, no metaprogramming — worth implementing?
5. **Benchmark accuracy**: How to improve P@5 from 0.749 to 0.9+ — better embedding models, HyDE, or reranking?
6. **Deployment target**: Self-hosted VPS vs cloud-managed (RDS, Cloud SQL) — which to prioritize?
7. **Auth granularity**: Is workspace-scoped access enough, or do we need collection-level permissions?
8. **TLS**: Should nano-brain handle TLS termination, or rely on reverse proxy (nginx, Caddy)?

### Resolved Questions

- ~~LLM for summarization~~ → OpenAI-compatible endpoint via `summarization.provider_url`
- ~~Output dir~~ → `~/.nano-brain/summaries/` (configurable)
- ~~Incremental harvest~~ → On-demand via `POST /api/harvest`, tracks last-harvested per session
- ~~Claude projects/memory~~ → Not harvesting `~/.claude/projects/` — only transcripts
- ~~costs.jsonl~~ → Not indexed — analytics-only, not searchable
- ~~Tree-sitter vs regex~~ → go-tree-sitter for Python, Ruby; regex for Go, JS/TS
- ~~Agent memory benchmark~~ → nano-brain P@5=0.749 vs LlamaIndex 0.55 vs Qdrant 0.27
