---
name: nano-brain
description: Persistent memory + code intelligence for AI coding agents. Hybrid search (BM25 + vector + RRF + recency), cross-session recall, symbol analysis, impact checks, knowledge graph, OpenCode/Claude Code session harvesting. Use this skill when you need to recall prior decisions across sessions, search across multiple codebases, trace symbol callers/callees, analyze code impact (what breaks if X changes), persist long-term context, or query the knowledge graph. Triggers — "remember what we did about X", "search across sessions", "find references to X", "what breaks if I change Y", "save this decision", "wake-up briefing".
compatibility: OpenCode, Claude Code, any MCP-aware agent
metadata:
  author: nano-step
  version: 3.1.1
  repo: nano-step/nano-brain
---


# nano-brain — Persistent Memory + Code Intelligence

A read-mostly knowledge graph + vector DB. Three call surfaces: HTTP, CLI (`npx nano-brain ...`), MCP tools. All operations require a `workspace_hash` that maps to one project root path.

## TL;DR

```bash
# Bootstrap once per shell:
eval "$(npx nano-brain workspaces current --export)"

# Then query:
npx nano-brain query "how did we solve auth caching"
```

All CLI commands read `$NANO_BRAIN_WORKSPACE` so you do not need to pass `--workspace=...` after bootstrap. For HTTP/MCP direct calls, see Phase 3.

---

## Phase 1 — DISCOVER (bootstrap)

Run these steps once per shell session. They are idempotent — safe to re-run.

### 1.1 Verify server is reachable

**From a container** (most agents):

```bash
curl -fsS http://host.docker.internal:3100/api/status | jq -r .pg_status
```

**From the host directly:**

```bash
curl -fsS http://localhost:3100/api/status | jq -r .pg_status
```

**Success criterion:** prints `healthy`.

**Failure recovery:** If `curl` fails, the server is not running. Recovery in order:
1. On host: `npx nano-brain docker start`
2. Ask the user to start the server. **Do not** try to start the server yourself inside a container — see "nano-brain Server Rule" in the project AGENTS.md.

### 1.2 Resolve the current workspace hash

**Recommended — CLI bootstrap one-liner:**

```bash
eval "$(npx nano-brain workspaces current --export)"
```

This sets `NANO_BRAIN_WORKSPACE` for the rest of the shell. All subsequent `npx nano-brain ...` commands pick it up implicitly.

**HTTP fallback** (for non-CLI agents):

```bash
HASH=$(curl -fsS -X POST http://host.docker.internal:3100/api/v1/workspaces/resolve \
  -H 'Content-Type: application/json' \
  -d "{\"path\":\"$PWD\"}" | jq -r .workspace_hash)
export NANO_BRAIN_WORKSPACE="$HASH"
```

The `/api/v1/workspaces/resolve` endpoint is **read-only**: it computes the deterministic SHA-256 hash from your absolute path and reports `registered: true|false`. It does NOT create the workspace.

### 1.3 Ensure the workspace is registered

```bash
npx nano-brain workspaces current --check 2>/dev/null \
  || npx nano-brain init --root="$PWD"
```

`workspaces current --check` exits **2** if the workspace is not yet registered. `init` is idempotent — safe to run multiple times.

**Success criterion:** `$NANO_BRAIN_WORKSPACE` is set AND `npx nano-brain workspaces current --check` exits 0.

---

## Phase 2 — SELECT (decision tree)

Map the user's intent to the right call:

| User intent | Operation | CLI | HTTP endpoint |
|---|---|---|---|
| "What did we decide about X?" | hybrid recall | `query "X"` | `POST /api/v1/query` |
| "Find exact term/identifier" | BM25 only | `search "X"` | `POST /api/v1/search` |
| "Semantic concept (no exact words)" | vector only | `vsearch "X"` | `POST /api/v1/vsearch` |
| "Across all my projects" | cross-workspace | `query --scope=all "X"` | `POST /api/v1/query` body `{"workspace":"all", ...}` |
| "Filter by tag (e.g. decisions)" | tagged search | `query --tags=decision "X"` | body adds `"tags":["decision"]` |
| "Save a decision/summary" | persist | `write` | `POST /api/v1/write` |
| "What does my workspace contain?" | briefing | `wake-up` | `GET /api/v1/wake-up?workspace=$NANO_BRAIN_WORKSPACE` |
| "Find function/type definition" | symbol lookup | `get` + code-intelligence | `GET /api/v1/symbols?workspace=...&query=...` |
| "Who calls FunctionName?" | graph traversal | (HTTP/MCP) | `POST /api/v1/graph/query` |
| "What breaks if I change Y?" | impact analysis | (HTTP/MCP) | `POST /api/v1/graph/impact` |
| "Trace call chain from entry" | call trace | (HTTP/MCP) | `POST /api/v1/graph/trace` |
| "List all tags with counts" | tag inventory | `tags` | `GET /api/v1/tags?workspace=...` |

**Default for ambiguous "find X":** use `query` (hybrid). It combines BM25 + vector + RRF + recency decay and gives the best results.

---

## Phase 3 — EXECUTE

Every example below assumes Phase 1 has set `$NANO_BRAIN_WORKSPACE`.

### 3.1 Hybrid query — `POST /api/v1/query`

```bash
# CLI:
npx nano-brain query "redis caching for inv compression" --json

# HTTP:
curl -fsS -X POST http://host.docker.internal:3100/api/v1/query \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$NANO_BRAIN_WORKSPACE\",\"query\":\"redis caching\",\"max_results\":10}"
```

Response: `{"results":[{"source_path","excerpt","score","tags",...}, ...]}`.

| Error | Meaning | Recovery |
|---|---|---|
| 400 `workspace_required` | Missing `workspace` in body | Re-run Phase 1.2 |
| 400 `workspace_not_registered` | Hash valid but not in DB | Re-run Phase 1.3 (`init`) |
| 500 | Server error | Read `nano-brain logs -n 50`, retry once |

### 3.2 BM25 keyword search — `POST /api/v1/search`

Use when the query is an exact identifier / error message / function name.

```bash
npx nano-brain search "EmbedQueue.PendingCount"
```

### 3.3 Vector semantic search — `POST /api/v1/vsearch`

Use when the user describes a concept without exact terms.

```bash
npx nano-brain vsearch "ways to keep the embedding queue from overflowing"
```

### 3.4 Cross-workspace search

```bash
# CLI:
npx nano-brain query --scope=all "rate limiter pattern"

# HTTP:
curl -fsS -X POST http://host.docker.internal:3100/api/v1/query \
  -H 'Content-Type: application/json' \
  -d '{"workspace":"all","query":"rate limiter pattern"}'
```

The literal string `"all"` is allowed for read-only endpoints (query/search/vsearch). It is **rejected** by write endpoints (`/api/v1/write`, `/api/v1/update`) — those require a specific registered hash.

### 3.5 Persist a decision — `POST /api/v1/write`

```bash
# CLI:
npx nano-brain write \
  --title="Decision: use RRF k=60 for hybrid search" \
  --tags=decision,search \
  --content='## Context\n...\n## Decision\n...'

# HTTP:
curl -fsS -X POST http://host.docker.internal:3100/api/v1/write \
  -H 'Content-Type: application/json' \
  -d "$(jq -n --arg ws "$NANO_BRAIN_WORKSPACE" \
        '{workspace:$ws, source_path:"notes/decision-rrf.md", content:"# Decision\nUse k=60.", tags:["decision"]}')"
```

### 3.6 Workspace briefing — `wake-up`

Use at start of a new task to summarize what's in the workspace.

```bash
npx nano-brain wake-up
```

Returns `{summary, recent_memories}` — `summary` is a natural-language string with doc/collection counts and last-activity timestamp; `recent_memories` is an array of `{id, title, snippet, tags, date}` for the most recently written documents.

### 3.7 Code intelligence — graph / trace / impact / symbols

For deep code analysis, see [`references/code-intelligence.md`](references/code-intelligence.md). Three patterns:

- **Symbol lookup:** `GET /api/v1/symbols?workspace=...&query=...&kind=...&limit=...` — find a function/type definition. Returns `{count, symbols: [{name, kind, language, signature, source_path}]}`.
- **Impact analysis:** `POST /api/v1/graph/impact` body `{workspace, node, max_depth, edge_type}` — who depends on this symbol, transitively.
- **Call trace:** `POST /api/v1/graph/trace` body `{workspace, node, max_depth}` — what does this entry function call.

### 3.8 Tags — `GET /api/v1/tags`

```bash
npx nano-brain tags
```

Returns `[{tag, count}, ...]` sorted by count desc.

---

## Phase 4 — RECOVER

Error catalog with deterministic recovery actions. Never retry more than 2 times. Never silently fall back without telling the user.

| Symptom | Most likely cause | Recovery |
|---|---|---|
| `curl: (7) connection refused` | Server not running | Phase 1.1 recovery |
| HTTP 400 `workspace_required` | Missing `workspace` field in body | Re-run Phase 1.2; verify `$NANO_BRAIN_WORKSPACE` is set |
| HTTP 400 `workspace_not_registered` | Path valid but never `init`'d | `npx nano-brain init --root="$PWD"` |
| HTTP 400 `path is required` | `/resolve` called with empty `path` | Pass an absolute path; default to `$PWD` |
| HTTP 400 `invalid path` | Path could not be normalized | Check path exists and is absolute |
| HTTP 503 | Server restarting / overload | Sleep 5s, retry **once** |
| HTTP 500 | Server bug | `nano-brain logs -n 100`; report stack to user |
| `workspace_all_not_supported` on write | Wrote with `workspace:"all"` | Use a specific registered hash |
| Empty `results: []` | Topic not in memory | Try broader query OR `search` (exact) OR fall back to grep/ast-grep over the codebase |

**Last resort fallback:** If nano-brain is unreachable for >2 retries, switch to native tools (grep / ast-grep / Read) and tell the user the limitation.

---

## CLI flags reference

| Flag | On | Effect |
|---|---|---|
| `--workspace=<hash>` | query, search, vsearch, write, get, tags, wake-up, multi-get | Override `$NANO_BRAIN_WORKSPACE` |
| `--scope=all` | query, search, vsearch | Cross-workspace |
| `--tags=t1,t2` | query, search, vsearch | Filter to docs with any of these tags |
| `--json` | most commands | Print raw JSON response |
| `--path=<p>` | workspaces current | Override `$PWD` for resolution |
| `--export` | workspaces current | Print `export NANO_BRAIN_WORKSPACE=<hash>` |
| `--check` | workspaces current | Exit 2 if workspace not registered |
| `--root=<p>` | init | Path to register; required |
| `--force` | init, workspaces remove | Skip confirmation |

---

## Anti-patterns (do not do)

- **Do not** hardcode a workspace hash. Always resolve from `$PWD` via Phase 1.
- **Do not** call `init` on every query. It is idempotent but wasteful; `resolve` is the read-only check.
- **Do not** swallow connection errors. If nano-brain is down, tell the user.
- **Do not** call write endpoints with `workspace:"all"`. They will reject with `workspace_all_not_supported`.
- **Do not** use `/api/query` or `/api/write` (no `/v1`). Those paths return 404. Always use `/api/v1/*`.
- **Do not** retry indefinitely. Cap retries at 2 per call.

---

## When to use nano-brain vs native tools

| Goal | Tool |
|---|---|
| "Have we discussed X before?" | nano-brain (cross-session memory) |
| "Where is X defined in this repo right now?" | grep / ast-grep / Read |
| "What did we decide about X back in March?" | nano-brain (recall) |
| "Show me line 142 of foo.go" | Read |
| "What's the AST shape of this function?" | ast-grep |
| "Has this error appeared in any project before?" | nano-brain `--scope=all` |
| "Who imports this module across my repos?" | nano-brain `graph/impact` (cross-repo) |

**Order:** recall from memory first (cheap, gives context). Then use native tools for precise current-state inspection.

---

## Reference docs (load on demand)

- [`references/http-api.md`](references/http-api.md) — full HTTP schema, all endpoints, performance budgets
- [`references/cli-cheatsheet.md`](references/cli-cheatsheet.md) — every CLI command + flag combo
- [`references/code-intelligence.md`](references/code-intelligence.md) — graph / trace / impact / symbols deep-dive
- [`references/config-reference.md`](references/config-reference.md) — `config.yml` fields
- Source contracts: `internal/server/routes.go` (HTTP) + `internal/mcp/tools.go::RegisterTools` (MCP) in the `nano-step/nano-brain` repo
