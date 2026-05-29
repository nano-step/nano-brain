# nano-brain Roadmap

> Last updated: 2026-05-29

---

## Vision

nano-brain là persistent memory + code intelligence layer cho AI agents.
Goal: agent biết context của project, lịch sử decision, và có thể dự đoán/chuẩn bị trước task tiếp theo.

---

## Pillar 1: Code Intelligence

**What:** Hiểu codebase như một senior engineer.

| Feature | Description | Status |
|---|---|---|
| File indexing | Watch + chunk + embed toàn bộ source files | ✅ |
| Symbol extraction | Functions, types, interfaces, constants | ✅ |
| Knowledge graph | Module → function → dependency relationships | ✅ |
| Impact analytics | Thay đổi X → affects Y, Z (cross-file) | ✅ |
| Call chain tracing | Trace execution path từ entry point | ✅ |

---

## Pillar 2: Session Harvesting

**What:** Thu thập + summarize sessions từ AI tools, scope per workspace.

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
| Auto-memory from sessions | Extract decisions từ harvested sessions | ✅ |
| 9 MCP tools | query, search, vsearch, get, write, tags, status, update, wake_up | ✅ |
| Hybrid search pipeline | BM25 + pgvector HNSW + RRF fusion + recency decay | ✅ |
| Benchmarking suite | generate, run, compare, stress | ✅ |
| Init onboarding wizard | Interactive config setup on first run | ✅ |
| Doctor command | Check prerequisites (PG, pgvector, Ollama, model) | ✅ |
| V1 SQLite migration | Import from V1 format (pure Go, no CGO) | ✅ |
| Config hot-reload | `POST /api/reload-config` | ✅ |
| Search telemetry | Local-only, 90-day retention, non-blocking | ✅ |

---

## Pillar 4: Self-Learning & Prediction

**What:** Học pattern từ user behavior → chuẩn bị context trước.

> ⚠️ Cần discuss thêm với user về scope/approach.

### 4a. Pattern Learning
- Phân tích prompt history từ harvested sessions
- Nhận diện recurring workflows (e.g., "user thường fix bug → run test → commit")
- Build user-specific workflow graph

### 4b. Proactive Context Pre-loading
- Dựa trên current task → dự đoán task tiếp theo
- Pre-fetch relevant code symbols, memory entries, past decisions
- Surface as "you might need next: ..."

### 4c. Self-Lesson Learn
- Sau mỗi session: extract lessons ("what worked", "what failed")
- Store as tagged memory entries
- Surface relevant lessons khi bắt đầu similar task

### 4d. Auto-execution (Stretch)
- Nano-brain tự trigger next step mà không cần user prompt
- Requires: high confidence prediction + user opt-in flag
- Risk: false positives → cần confidence threshold

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

Phase 4 — Hardening (Current)
  ├── #180 — Ollama context length overflow on large chunks
  ├── #181 — UTF-8 null byte in harvested sessions
  ├── #184 — Require explicit --workspace on CLI commands
  ├── #158 — Incremental reindex (only changed files)
  └── #190, #191 — Harvest follow-ups (stale doc cleanup, max_tokens bump)

Phase 5 — CLI Completeness (Next)
  ├── #153 — Code intelligence CLI (context, code-impact, detect-changes)
  ├── #151 — Wake-up command (compact context briefing)
  ├── #152 — get, tags, multi-get commands
  ├── #155 — Workspace remove command
  ├── #156 — Cross-workspace search (--scope=all)
  ├── #157 — Cache management (clear, stats)
  └── #160 — --tags filter for query/search

Phase 6 — Enhanced Code Intelligence
  ├── #174 — Symbol extraction with go-tree-sitter (replace regex)
  └── Cross-language support (TypeScript, Python, Rust)

Phase 7 — Self-Learning (Discuss)
  ├── #154 — Memory consolidation + categorization + Thompson Sampling
  ├── Pattern learning from prompt history
  ├── Proactive context pre-loading
  └── Self-lesson extraction
```

---

## Open Questions

1. **Pillar 4 scope**: Proactive suggestions only, or auto-execution? (Needs design discussion)
2. **Tree-sitter vs regex**: #174 proposes go-tree-sitter for symbol extraction — worth the binary size increase?
3. **Cross-workspace search**: #156 — privacy implications of searching across all workspaces?
4. **Memory consolidation**: #154 — Thompson Sampling for relevance ranking — need benchmarks first?

### Resolved Questions

- ~~LLM for summarization~~ → OpenAI-compatible endpoint via `summarization.provider_url`
- ~~Output dir~~ → `~/.nano-brain/summaries/` (configurable)
- ~~Incremental harvest~~ → On-demand via `POST /api/harvest`, tracks last-harvested per session
- ~~Claude projects/memory~~ → Not harvesting `~/.claude/projects/` — only transcripts
- ~~costs.jsonl~~ → Not indexed — analytics-only, not searchable
