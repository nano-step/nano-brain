---
name: nano-brain
description: Persistent memory + code intelligence for AI coding agents. Hybrid search (BM25 + vector + LLM rerank), cross-session recall, symbol analysis, impact checks, OpenCode/Claude Code session harvesting. Use this skill when you need to recall prior decisions, search across sessions/codebase, trace symbol callers/callees, or persist long-term context.
compatibility: OpenCode, Claude Code, any MCP-aware agent
metadata:
  author: nano-step
  version: 3.0.0
  repo: nano-step/nano-brain
---

# nano-brain

Persistent memory + code intelligence service for AI coding agents. This skill teaches an agent **when** to use nano-brain and **how** to call its three API layers correctly.

## Install this skill

This skill lives at `skills/nano-brain/` in the [nano-brain repo](https://github.com/nano-step/nano-brain). Drop it into your agent's skills root:

```bash
# OpenCode — user-level (any project)
mkdir -p ~/.config/opencode/skills
cp -r /path/to/nano-brain/skills/nano-brain ~/.config/opencode/skills/

# OpenCode — project-level (current project only)
mkdir -p ./.opencode/skills
cp -r /path/to/nano-brain/skills/nano-brain ./.opencode/skills/

# Claude Code
mkdir -p ~/.claude/skills
cp -r /path/to/nano-brain/skills/nano-brain ~/.claude/skills/
```

Or one-liner (no clone needed) — fetch the skill directly from GitHub:

```bash
curl -sL https://github.com/nano-step/nano-brain/archive/refs/heads/master.tar.gz \
  | tar -xz --strip-components=2 -C ~/.config/opencode/skills nano-brain-master/skills/nano-brain
```

The skill auto-loads on next agent session. To update, repeat the copy (`--delete` if using rsync) — files are idempotent.

---

## How to talk to nano-brain

| Layer | When | How |
|---|---|---|
| **MCP tools** (`memory_*`) | Preferred. Streamable HTTP at `/mcp`. | Agent auto-discovers; no shell needed |
| **CLI** (`npx @nano-step/nano-brain ...`) | When MCP not configured, or for one-off shell scripts | Wraps the HTTP API |
| **HTTP API** directly | Scripts, integration tests, dashboards | `POST /api/v1/*` |

All three target the same daemon at `http://localhost:3100` (default; container agents auto-set `NANO_BRAIN_HOST=host.docker.internal`).

> **CLI invocation**: always use the scoped package `@nano-step/nano-brain` for clarity and to avoid name collisions:
> ```bash
> npx @nano-step/nano-brain <subcommand> ...
> ```
> An unscoped alias `nano-brain` is also published (e.g. `npx nano-brain ...`), but the scoped form is canonical and stable.

## Quick decision matrix

| You need to... | Use |
|---|---|
| Recall past work on a topic | `memory_query` / `query "topic"` |
| Find an exact string (error msg, fn name) | `memory_search` / `search "term"` |
| Explore a fuzzy concept | `memory_vsearch` / `vsearch "concept"` |
| Save a decision for future sessions | `memory_write` / `write "..." --tags=decision` |
| Check daemon health, queue depth, migration | `memory_status` / `status` |
| Catch up at session start | `memory_wake_up` / `wake-up` |
| Trace symbol callers/callees | `memory_graph` (CLI: `context <name>`) |
| Risk-check before refactor | `memory_impact` (CLI: `code-impact <name>`) |
| Map git diff → affected symbols | CLI: `detect-changes --scope=staged` |
| List all tags / collections / symbols | `memory_tags` / `memory_symbols` |
| Reindex after major code changes | CLI: `reindex` (in workspace dir) |

## Endpoint reference

All `POST /api/v1/*` endpoints require a `workspace` field in the JSON body OR an `X-Workspace` header. The workspace hash is returned by `POST /api/v1/init`.

### Health + status

| Method | Path | Body | Returns |
|---|---|---|---|
| `GET` | `/health` | — | `{status, ready}` — for liveness/readiness probes |
| `GET` | `/api/status` | — | `{pg_status, migration_version, queue_*, workspace_count, harvester_status}` — operator dashboard data |

### Workspace lifecycle

| Method | Path | Body | Returns |
|---|---|---|---|
| `POST` | `/api/v1/init` | `{"root_path":"/abs/path"}` | `{workspace_hash, root_path, agents_snippet}` — registers a workspace, deterministic SHA-256 hash of normalized path |
| `GET` | `/api/v1/workspaces` | — | `[{workspace_hash, root_path, document_count, last_indexed_at}]` |
| `POST` | `/api/v1/reset-workspace` | `{"workspace":"<hash>"}` | `{deleted_documents, workspace}` — wipes all docs+chunks+embeddings for the workspace |

### Search (3 modes — choose by use case)

| Method | Path | Body | Notes |
|---|---|---|---|
| `POST` | `/api/v1/search` | `{"workspace":"<hash>","query":"...","max_results":10,"tags":["bug"]}` | **BM25 full-text** — exact keywords/symbols, fastest, free-text English |
| `POST` | `/api/v1/vsearch` | `{"workspace":"<hash>","query":"...","max_results":10}` | **Vector semantic** — fuzzy concepts, embedding cosine, slower |
| `POST` | `/api/v1/query` | `{"workspace":"<hash>","query":"...","max_results":10}` | **Hybrid** — BM25 + vector + RRF fusion. Default for most questions. |

Response shape (all three):
```json
{"results": [{"id","title","snippet","score","tags","collection","source_path","created_at"}], "took_ms": 42}
```

### Document I/O

| Method | Path | Body | Notes |
|---|---|---|---|
| `POST` | `/api/v1/write` | `{"workspace":"<hash>","content":"...","tags":["..."],"collection":"memory","title":"...","source_path":"memory://...","metadata":{}}` | Upsert by content-hash; chunks + enqueues for embedding async |
| `POST` | `/api/v1/wake-up` | `{"workspace":"<hash>","limit":10}` | Returns the N most-recent updated documents — for session start briefing |
| `POST` | `/api/v1/reindex` | `{"workspace":"<hash>","root":"/abs/path"}` | Reindex workspace from disk; respects `.gitignore` |
| `POST` | `/api/v1/embed` | `{"workspace":"<hash>","chunk_id":"<uuid>"}` | Force-embed a specific chunk (debugging) |
| `POST` | `/api/v1/summarize` | `{"workspace":"<hash>","source":"opencode://session/<id>","limit":1,"force":false}` | Run LLM summarizer on a session |

### Collections + tags + symbols

| Method | Path | Body | Notes |
|---|---|---|---|
| `GET` | `/api/v1/collections?workspace=<hash>` | — | List collections with doc counts |
| `POST` | `/api/v1/collections` | `{"workspace":"<hash>","name":"my-col"}` | Create |
| `PUT` | `/api/v1/collections/:name` | `{"workspace":"<hash>","new_name":"..."}` | Rename |
| `DELETE` | `/api/v1/collections/:name` | `{"workspace":"<hash>"}` | Remove (and all its docs) |
| `GET` | `/api/v1/tags?workspace=<hash>` | — | All tags with usage counts |
| `GET` | `/api/v1/symbols?workspace=<hash>&type=function&name=foo` | — | Symbol search by type+name pattern |

### Code intelligence (graph)

Powered by Tree-sitter AST parsing. **Requires** a prior `reindex` to populate the symbol graph.

| Method | Path | Body | Returns |
|---|---|---|---|
| `POST` | `/api/v1/graph/query` | `{"workspace":"<hash>","node":"funcName","direction":"out","edge_type":"calls"}` | Symbol's neighbors (callers/callees) |
| `POST` | `/api/v1/graph/impact` | `{"workspace":"<hash>","node":"funcName","edge_type":"calls","max_depth":2}` | BFS upstream/downstream — risk = LOW/MEDIUM/HIGH/CRITICAL |
| `POST` | `/api/v1/graph/trace` | `{"workspace":"<hash>","node":"funcName","max_depth":3}` | Outgoing-edge walk for flow detection |

`direction`: `in` (callers) or `out` (callees). `edge_type`: `calls`, `imports`, `references`, etc.

### Harvest + ops

| Method | Path | Body | Notes |
|---|---|---|---|
| `POST` | `/api/harvest` | — | Force-run all configured harvesters (OpenCode SQLite, Claude Code JSONL). Returns `{harvested, skipped, errors}`. |
| `POST` | `/api/reload-config` | — | Reload `~/.nano-brain/config.yml` without restart |

### MCP (Streamable HTTP)

| Method | Path | Notes |
|---|---|---|
| `POST` | `/mcp` | JSON-RPC 2.0 MCP protocol |
| `GET`/`POST` | `/sse` | SSE transport (legacy) |

13 tools exposed: `memory_query`, `memory_search`, `memory_vsearch`, `memory_get`, `memory_write`, `memory_tags`, `memory_status`, `memory_update`, `memory_wake_up`, `memory_graph`, `memory_trace`, `memory_impact`, `memory_symbols`.

## CLI cheatsheet

Every CLI subcommand is a thin client over the HTTP API. Connection defaults to `localhost:3100`; container agents auto-route to host.

| Command | Endpoint |
|---|---|
| `npx @nano-step/nano-brain query "..." --workspace <h>` | `POST /api/v1/query` |
| `npx @nano-step/nano-brain search "..." --workspace <h>` | `POST /api/v1/search` |
| `npx @nano-step/nano-brain vsearch "..." --workspace <h>` | `POST /api/v1/vsearch` |
| `npx @nano-step/nano-brain write "..." --workspace <h> --tags=foo,bar` | `POST /api/v1/write` |
| `npx @nano-step/nano-brain status` | `GET /api/status` |
| `npx @nano-step/nano-brain workspaces` | `GET /api/v1/workspaces` |
| `npx @nano-step/nano-brain wake-up --workspace <h>` | `POST /api/v1/wake-up` |
| `npx @nano-step/nano-brain reindex` (in workspace dir) | `POST /api/v1/reindex` |
| `npx @nano-step/nano-brain context <symbol>` | `POST /api/v1/graph/query` |
| `npx @nano-step/nano-brain code-impact <symbol> --direction=upstream` | `POST /api/v1/graph/impact` |
| `npx @nano-step/nano-brain detect-changes --scope=staged` | (computes git diff client-side, calls graph endpoints) |
| `npx @nano-step/nano-brain harvest` | `POST /api/harvest` |
| `npx @nano-step/nano-brain cleanup-stale-raw [--dry-run]` | Direct PG — wipes pre-PR-#192 orphan raw session docs |
| `npx @nano-step/nano-brain doctor` | Probes daemon + PG + ollama + model availability |

**Critical CLI gotcha**: `reindex` IGNORES the `--root` flag silently. Always run with `workdir`:
```bash
# ✅ Correct
bash(command="npx @nano-step/nano-brain reindex", workdir="/path/to/workspace")

# ❌ Wrong (silently reindexes CWD instead of /path)
npx @nano-step/nano-brain reindex --root=/path/to/workspace
```

## Recipes — best-practice flows

### Recipe 1: Session start (wake-up + recall)

When you start a coding session in a project, run wake-up + a targeted query before exploring. This costs ~500 tokens but prevents you from redoing work or re-discovering the same lessons.

```bash
# 1. Register workspace once per project (idempotent)
WS=$(npx @nano-step/nano-brain init --root="$PWD" --json | jq -r .workspace_hash)

# 2. Catch up — N most-recent updates across all collections
npx @nano-step/nano-brain wake-up --workspace="$WS" --limit=8

# 3. Targeted: anything you've learned about today's task?
npx @nano-step/nano-brain query "<current task topic>" --workspace="$WS" --compact
```

For agents inside OpenCode: use MCP `memory_wake_up` then `memory_query`.

### Recipe 2: Recall before grep

Native grep finds exact strings in CURRENT files. nano-brain finds them in past sessions, prior commits, and documentation. The two are complementary — always recall first (cheap, ~200 tokens) then grep if you need exact code locations.

```bash
# Did we ever debug this before?
npx @nano-step/nano-brain search "ECONNREFUSED redis" --workspace="$WS"

# Concept-level — fuzzy match
npx @nano-step/nano-brain vsearch "rate limiting strategy" --workspace="$WS"

# Complex question — hybrid (default)
npx @nano-step/nano-brain query "how did we handle session invalidation" --workspace="$WS"
```

### Recipe 3: Pre-refactor impact check

Before touching a symbol that's referenced widely, check impact. This catches "I only need to fix this one place" tunnel-vision.

```bash
# What calls DatabaseClient?
npx @nano-step/nano-brain code-impact DatabaseClient --workspace="$WS" --direction=upstream

# What does processOrder depend on?
npx @nano-step/nano-brain code-impact processOrder --workspace="$WS" --direction=downstream --max-depth=3

# Map current git changes to symbols
npx @nano-step/nano-brain detect-changes --scope=staged --workspace="$WS"
```

Risk levels in response: `LOW` (≤3 callers), `MEDIUM` (4-15), `HIGH` (16-50), `CRITICAL` (>50). Treat HIGH/CRITICAL as "needs PR review from someone else."

### Recipe 4: Persist a decision

End every meaningful session with a write. The schema doesn't matter — content is freeform markdown. What matters is consistent tags so future you can filter.

```bash
npx @nano-step/nano-brain write "## Decision: Use Redis Streams over Bull
- **Why:** ordered replay needed for retry
- **Trade-off:** lose Bull's UI; built our own metrics dashboard instead
- **Files:** internal/queue/*.go, see PR #142" \
  --workspace="$WS" \
  --tags=decision,architecture,queue
```

Conventional tag patterns: `decision`, `lesson`, `architecture`, `bug`, `gotcha`, `summary`, `<area>` (e.g. `auth`, `queue`).

### Recipe 5: Triage many results (compact mode)

For broad queries that return 10+ results, fetch compact first (1-line summaries, ~70% fewer tokens), pick the relevant ones, expand only those.

```bash
# Stage 1: scan
npx @nano-step/nano-brain query "auth middleware" --workspace="$WS" --compact

# Stage 2: expand specific result by ID
npx @nano-step/nano-brain get "<doc-id>" --workspace="$WS"
```

### Recipe 6: Cross-workspace knowledge transfer

Same `memory_query` but with `scope=all`. Searches every registered workspace, useful for "have I solved this in another project?"

```bash
# Aggregate across all workspaces
npx @nano-step/nano-brain query "stripe webhook signature verification" --scope=all
```

(Equivalent MCP: `memory_query` with `workspace` omitted or `scope=all` arg.)

### Recipe 7: Filter by collection or tag

Both search modes accept `--collection` and `--tags`. Combine when you know the slice you want.

```bash
# Only past session summaries about bugs
npx @nano-step/nano-brain query "..." --workspace="$WS" --collection=session-summary --tags=bug

# Only your curated notes
npx @nano-step/nano-brain query "..." --workspace="$WS" --collection=memory
```

Collections in a typical setup:
- `codebase` — source-file chunks
- `session-summary` — LLM-summarized AI sessions (OpenCode, Claude Code)
- `sessions` — raw session content (pre-PR-#192 only; run `cleanup-stale-raw` to purge if you've migrated)
- `memory` — agent-written notes via `write`
- `auto-memory` — auto-extracted `DECISION:` / `LESSON:` lines

### Recipe 8: Multi-DB OpenCode harvest (advanced)

If you use the `ai-sandbox-wrapper` which splits OpenCode sessions into per-project SQLite databases at `~/.ai-sandbox/opencode-dbs/<slug>-<8hex>/opencode.db`, configure nano-brain to discover them:

```yaml
# ~/.nano-brain/config.yml
harvester:
  opencode:
    db_root: ~/.ai-sandbox/opencode-dbs   # auto-detected on macOS/Linux
```

The daemon scans the directory, opens each DB read-only, reads `project.worktree`, and only harvests sessions whose worktree matches a registered nano-brain workspace. Verify via `GET /api/status`:

```bash
curl -s http://localhost:3100/api/status | jq '.harvester_status.opencode'
# {"enabled":true, "mode":"db_root", "db_root":"/Users/.../opencode-dbs", "db_count":1, ...}
```

`mode` values: `db_root` (new multi-DB), `db_path` (single SQLite), `session_dir` (legacy JSON), `disabled`.

## Common errors + fixes

| Symptom | Diagnosis | Fix |
|---|---|---|
| `cannot connect to nano-brain server` | Daemon not running | `npx @nano-step/nano-brain serve -d` (background) |
| `workspace not found` | Workspace not registered | `npx @nano-step/nano-brain init --root=.` first |
| `ollama: input length exceeds context length` (pre-2026.5.30.1) | Char-budget too high for token limit | Upgrade to ≥2026.5.30.1; configure `embedding.max_chars` if still hits |
| Search returns 0 results despite content | Embedding queue still working | `npx @nano-step/nano-brain status` → check `queue_pending` and `embedding_queue_depth` |
| Chunk stuck in `embed_failed` retry loop (pre-fix) | Deterministic ollama error | Upgrade ≥2026.5.30.1; new `embed_permanently_failed` state breaks the loop |
| `bin/sh: gh: not found` in workflow | gh CLI not installed in container | Use HTTPS+token URL: `git push https://kokorolx:$TOKEN@github.com/...` |

## Daemon config — key fields

```yaml
# ~/.nano-brain/config.yml — see ${PROJECT}/README.md for the canonical example
database:
  url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev

embedding:
  provider: ollama
  url: http://localhost:11434
  model: nomic-embed-text   # 2048-token context
  concurrency: 3
  max_chars: 3000           # safety budget per embed call; lower if ollama still 400s on dense CSV
                            # override via NANO_BRAIN_EMBED_MAX_CHARS

harvester:
  opencode:
    db_root: ""             # ~/.ai-sandbox/opencode-dbs (multi-DB)
    db_path: ""             # ~/.local/share/opencode/opencode.db (single SQLite)
    session_dir: ""         # legacy JSON
  claudecode:
    enabled: false
    session_dir: ""         # ~/.claude/sessions

intervals:
  session_poll: 120         # harvest tick interval (seconds)

summarization:
  enabled: false            # LLM-generated session summaries
  provider_url: ""
  api_key: ""               # or NANO_BRAIN_SUMMARIZE_API_KEY env
  model: "claude-sonnet-4-5"
  max_tokens: 8000          # bumped from 4096 — issue #191
```

Reload without restart: `curl -X POST http://localhost:3100/api/reload-config`.

## Memory vs native tools

| Question | Tool |
|---|---|
| "What did we decide about X?" | nano-brain (memory) |
| "Where exactly is `processPayment` defined?" | grep / Glob (native) |
| "What calls `processPayment`?" | nano-brain (code intel) → `memory_graph` |
| "What's the AST structure of this function?" | ast-grep (native) |
| "Have I solved this error before across all my projects?" | nano-brain (cross-workspace) |
| "Show me line 142 of foo.go" | Read tool (native) |

**They are complementary. Always recall first (cheap), then use native tools for precise current-state.**

## Further reading

- **Code intelligence deep-dive**: `references/code-intelligence.md`
- **Full HTTP API contract**: nano-brain repo `internal/server/routes.go` + per-handler request/response structs in `internal/server/handlers/`
- **MCP tool schemas**: `internal/mcp/tools.go` `RegisterTools`
- **Migration history**: `nano-step/nano-brain` `CHANGELOG.md` + `openspec/changes/archive/`
