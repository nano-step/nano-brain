---
name: nano-brain
description: Persistent memory + code intelligence for AI coding agents. Hybrid search (BM25 + vector + RRF), cross-session recall, symbol graph analysis, impact checks, OpenCode/Claude Code session harvesting. Use when you need to recall prior decisions, search across sessions/codebase, trace symbol callers/callees, or persist long-term context.
compatibility: OpenCode, Claude Code, any MCP-aware agent
metadata:
  author: nano-step
  version: 3.2.0
  upstream: https://github.com/nano-step/nano-brain
---

# nano-brain

Persistent memory + code-intel daemon. Agents talk to it via **MCP** — streamable HTTP at `/mcp` on the daemon. HTTP is available as a fallback for scripts and integration tests.

This file documents the MCP surface. Deeper references:

- `@references/http-api.md` — full HTTP endpoint reference (for scripts, tests, dashboards)
- `@references/code-intelligence.md` — symbol graph (`memory_graph`, `memory_impact`, `memory_trace`, `memory_symbols`)
- `@references/config-reference.md` — daemon `config.yml` schema + env vars

(load these only when you need them — most agent workflows live in this file.)

## When to call which tool

| You need to... | Tool | Why |
|---|---|---|
| Recall past work on a topic | `memory_query` | Hybrid (BM25 + vector + recency) — best default |
| Find exact string (error msg, fn name) | `memory_search` | BM25 — fastest, no rerank |
| Explore fuzzy concept | `memory_vsearch` | Vector only — semantic match |
| Save a decision/lesson | `memory_write` | Persists for future sessions |
| Catch up at session start | `memory_wake_up` | Recent docs + active collections + stats |
| Fetch one doc by ID/path | `memory_get` | Full content + metadata |
| Check daemon health/queue | `memory_status` | PG + embed queue + harvester |
| Trace symbol callers/callees | `memory_graph` | 1-hop neighbors |
| Risk-check before refactor | `memory_impact` | Reverse-impact BFS, depth 1-3 |
| Walk call chain | `memory_trace` | Forward walk with cycle detect, depth 1-10 |
| Find a symbol by name/kind | `memory_symbols` | Function/type/method/interface search |
| List all tags | `memory_tags` | Tag inventory |
| Force re-embed | `memory_update` | Re-queues workspace chunks |

## MCP tool schemas

Connect: streamable HTTP at `/mcp` on the daemon. Container agents use `http://host.docker.internal:3100/mcp`.

Every tool takes a `workspace` string (the SHA-256 hash returned by `POST /api/v1/init`). Listed required fields are in the `required` array of the InputSchema.

### memory_query — hybrid search (DEFAULT)
```
required: workspace, query
optional: max_results (int, default 10, capped at 100)
          include_content (bool, default false) — opt-in full chunk text
          cursor (string) — pagination token from previous response.next_cursor
returns:  {results: [{id, title, snippet, score, tags, collection, source_path, workspace_hash, document_id, created_at, updated_at}], total, query_ms, next_cursor?}
```
Each result includes a `snippet` (≤500 chars, UTF-8 safe). The full `content` field is OMITTED by default — pass `include_content: true` to include it, or call `memory_get` for one full document. `next_cursor` is present only when more results exist beyond the current page.

### memory_search — BM25 keyword
```
required: workspace, query
optional: max_results (capped 100), tags (array of strings — AND filter)
          include_content (bool, default false), cursor (string)
returns:  same shape as memory_query (snippet-only by default)
```
Note: tags filter is conjunctive (chunk must have ALL listed tags).

### memory_vsearch — vector semantic
```
required: workspace, query
optional: max_results (capped 100), include_content (bool), cursor (string)
returns:  same shape as memory_query (snippet-only by default)
```
Slower than BM25 (embedding round-trip); best for "concept similar to…" queries where exact words don't match.

### Pagination & include_content (since #358 / v2026.6.x)

All three search tools share one response envelope. Paginate with the opaque `cursor` returned in `next_cursor` — pass the SAME `query` text on every page (server validates a hash to prevent cross-query cursor reuse → error `"cursor query mismatch"`).

```
# Browse 30 days of bug fixes incrementally — no payload bombs
memory_search(workspace=ws, query="fix bug", max_results=5)
  → {results: [...5...], total: 47, query_ms: 12, next_cursor: "eyJv..."}
memory_search(workspace=ws, query="fix bug", max_results=5, cursor="eyJv...")
  → next 5 results...
```

Need the FULL text of one specific hit? Call `memory_get(workspace, path="#<id-from-result>")` — that tool is the canonical full-content fetch and supports `start_line`/`end_line` slicing. Pass `include_content: true` to a search tool only when you genuinely need full text for ALL results in one shot (rare; inflates response 5–50×).

### Time-range filters (since #360)

Agents asking time-bounded questions ("what bugs have we fixed in the last 30 days?") can push the filter directly to the database — no client-side `updated_at` filtering, no extra pagination loops.

All three search tools (`memory_query`, `memory_search`, `memory_vsearch`) accept four optional parameters:

| Parameter | Type | Description | Example |
|---|---|---|---|
| `created_after` | string | Restrict to docs created after this point | `"30d"`, `"2026-05-01T00:00:00Z"` |
| `created_before` | string | Restrict to docs created before this point | `"7d"`, `"2026-06-01T00:00:00Z"` |
| `updated_after` | string | Restrict to docs updated after this point | `"1w"`, `"720h"` |
| `updated_before` | string | Restrict to docs updated before this point | `"2mo"`, `"2026-04-01T00:00:00Z"` |

Multiple filters combine with AND semantics. Omitting all four leaves behavior identical to pre-#360.

#### Accepted formats

**Preferred for LLM agents: compute the absolute date client-side and pass RFC3339.** You already know today's date from your system context — do the subtraction yourself (`2026-06-03 minus 30 days = 2026-05-04T00:00:00Z`) and pass the result. This avoids the 30-day-month / 365-day-year approximation in relative durations and produces stable, auditable queries. Relative durations exist primarily for human-typed CLI use (`nano-brain search --updated-after=30d`).

- **RFC3339 absolute** (preferred): `"2026-05-04T12:00:00Z"` — timezone required; future timestamps accepted (explicit agent choice)
- **Go-style duration**: `"720h"`, `"30m"`, `"1h30m"` — subtracted from now
- **Humanish relative**: `"30d"`, `"1w"`, `"2mo"`, `"1y"` — units: `s`, `m`, `h`, `d`, `w`, `mo`, `y` (approximate — see caveat below)

#### Rejected inputs (with error messages)

```
"2026-05-04"      → invalid timestamp or duration "2026-05-04": not a valid RFC3339 timestamp...
                    (fix: "2026-05-04T00:00:00Z")

"-30d"            → invalid timestamp or duration "-30d": duration must be positive, got -720h0m0s
"0d"              → invalid timestamp or duration "0d": duration must be positive, got 0s
"-720h"           → invalid timestamp or duration "-720h": duration must be positive, got -720h0m0s

"banana"          → invalid timestamp or duration "banana": not a valid RFC3339 timestamp, Go duration,
                    or humanish duration (e.g., '30d', '1w')
"30x"             → same (unknown unit)
```

Validation runs before any database call. The error names the offending parameter and the rejected value.

#### Rough-relative caveat

`1mo` = 30 days exactly. `1y` = 365 days exactly. These are approximate — not calendar-aware month/year arithmetic. For queries that require calendar precision ("since the start of April"), pass an RFC3339 timestamp instead.

#### Worked examples

```
# memory_search — find bug-fix discussions from the last 30 days
# Preferred (LLM computes the cutoff):
memory_search(workspace=<hash>, query="bug fix", updated_after="2026-05-04T00:00:00Z")
# Or relative (works but less precise):
memory_search(workspace=<hash>, query="bug fix", updated_after="30d")

# memory_query — hybrid search for design docs created since May 1, 2026
memory_query(workspace=<hash>, query="design", created_after="2026-05-01T00:00:00Z")

# memory_vsearch — semantic similarity, only documents from the last week
memory_vsearch(workspace=<hash>, query="rate limiting strategy", updated_after="2026-05-27T00:00:00Z")
```

#### Pagination interaction

The cursor encodes ALL filter values (query + tags + time-range raw strings). Paginating with different filter values returns `"cursor invalidated, restart pagination"`. To page through a filtered result set, pass the SAME filter values on every subsequent call:

```
memory_search(workspace=<hash>, query="bug fix", updated_after="30d", max_results=5)
  → next_cursor: "eyJv..."
memory_search(workspace=<hash>, query="bug fix", updated_after="30d", max_results=5, cursor="eyJv...")
  → next page, same filtered window
```

### memory_get — fetch one doc
```
required: workspace, path
optional: start_line (1-indexed inclusive), end_line (1-indexed inclusive)
path:     either source_path (e.g. memory://foo/bar) OR #<uuid> form
returns:  {id, title, content, source_path, collection, tags, workspace_hash, supersedes_id?, created_at, updated_at}
```
Source: `internal/mcp/tools.go:417-506`, `internal/server/handlers/get_document.go:21-37`. Use `start_line`/`end_line` for huge docs to avoid loading megabytes.

### memory_write — persist a decision
```
required: workspace (must be registered via /api/v1/init — workspace="all" is REJECTED, issue #238), content (max 5MB)
optional: title, tags (array), collection (default "memory"), source_path, metadata (object), supersedes (#<uuid> or source_path of doc this replaces)
returns:  {id, hash, collection, workspace_hash, chunk_count, warning?}
```
Source: `internal/mcp/tools.go:508-680`. Tags convention: `decision`, `lesson`, `summary`, `bug`, `gotcha`, plus an area tag (`auth`, `queue`, etc.). Same `source_path` upserts (replaces) the existing doc.

### memory_wake_up — session-start briefing
```
required: workspace
optional: limit (default 10, capped 50)
returns:  {summary, recent_memories: [{id, title, snippet, tags, date}], active_collections: [{name, document_count, last_updated}], stats: {total_documents, total_chunks, last_activity}}
```
Source: `internal/mcp/tools.go:791-914`, `internal/server/handlers/wakeup.go:22-52`. Call first thing after registering a workspace; the `summary` field gives the agent a one-paragraph orientation.

### memory_status — daemon health
```
required: (none)
returns:  {pg_status, migration_version, embedding_queue_depth, active_provider, workspace_count, queue_depth, queue_capacity, queue_status, queue_pending, harvester_status: {...}}
```
Source: `internal/mcp/tools.go:731-763`, `internal/server/handlers/health.go:111-122`. Check `queue_pending` if `memory_search` returns nothing — chunks may still be embedding.

### memory_graph — 1-hop symbol neighbors
```
required: workspace, node ("/abs/path.go" OR "/abs/path.go::FunctionName")
optional: direction ("out" | "in" | "both", default "out"), edge_type ("calls" | "imports" | "contains" | empty for all)
returns:  {node, direction, edges: [{source, target, edge_type}]}
```
Source: `internal/mcp/tools.go:916-1007`. Requires prior `reindex` to populate the graph.

### memory_impact — reverse impact BFS
```
required: workspace, node
optional: edge_type, max_depth (1-3, server-clamped, default 1)
returns:  {node, impacted: [{node, depth, edge_type}]}
```
Source: `internal/mcp/tools.go:1087-1157`. Use before refactor — `impacted` is the set of nodes that would break if `node` changes.

### memory_trace — forward call chain
```
required: workspace, node
optional: max_depth (1-10, server-clamped, default 5)
returns:  {entry, chain: [{node, depth, via}]}
```
Source: `internal/mcp/tools.go:1009-1085`. Walks outgoing edges with cycle detection. Use to understand "what does this entry point eventually call?".

### memory_symbols — symbol search
```
required: workspace
optional: query (substring filter), kind ("function" | "method" | "type" | "interface" | "struct" | "const" | "var"), limit (default 50, capped 200)
returns:  {symbols: [{name, kind, language, signature, source_path}], count}
```
Source: `internal/mcp/tools.go:1159-1223`.

### memory_tags — tag inventory
```
required: workspace
returns:  array of tag/collection summaries (see tool schema)
```
Source: `internal/mcp/tools.go:682-729`.

### memory_update — force re-embed
```
required: workspace (must be registered)
returns:  count of chunks re-queued
```
Source: `internal/mcp/tools.go:765-789`. Rare — only useful after switching embedding model or fixing corrupt embeddings.

## Recipes

### R1 — Session start
```
1. memory_wake_up(workspace, limit=8)      // briefing + recent docs
2. memory_query(workspace, query="<task topic>")  // anything we learned about this?
```
Costs ~500 tokens; saves much more by preventing redundant exploration.

### R2 — Recall before grep
nano-brain finds matches in past sessions, prior commits, docs. Grep finds matches in current files. Always recall first (cheap, ~200 tokens) then grep for exact code locations.

| Need | Call |
|---|---|
| "Did we hit this error before?" | `memory_search("ECONNREFUSED redis")` |
| "How did we handle rate limiting?" | `memory_vsearch("rate limiting strategy")` |
| "What did we decide about X?" | `memory_query("X decision")` |

### R3 — Pre-refactor impact check
```
memory_impact(workspace, node="/path/file.go::Symbol", edge_type="calls", max_depth=2)
```
Read the `impacted` array — count > 10 means HIGH risk, treat as "needs review."

### R4 — Persist a decision (end of session)
```
memory_write(
  workspace,
  content="## Decision: …\n- Why: …\n- Trade-off: …\n- Files: …",
  tags=["decision", "architecture", "<area>"],
  collection="memory"
)
```
Tags are how future-you filters. Be consistent: `decision`, `lesson`, `summary`, `bug`, `gotcha`, plus an area.

### R5 — Triage many results
For broad queries with 10+ hits, scan via low max_results first, then expand the relevant ones:
```
1. memory_query(workspace, query="…", max_results=5)  // top hits only
2. memory_get(workspace, path="#<uuid-from-top-hit>")  // full content of the chosen one
```

### R6 — Cross-workspace recall
There is no `workspace="all"` for write paths (rejected for safety per #238). For READ paths, switch workspaces in a loop:
```
for ws in $registered_workspaces:
  memory_query(workspace=ws, query="<topic>")
```
Or use HTTP `GET /api/v1/workspaces` to list hashes, then iterate.

## Common errors

| Symptom | Cause | Fix |
|---|---|---|
| `cannot connect to daemon` | nano-brain not running on host | Operator action — start the daemon on the host (out of scope for agents). Then re-resolve MCP URL via `NANO_BRAIN_MCP_URL`. |
| `workspace_not_found` (HTTP 404) | Workspace hash not in DB (#309 fix) | `POST /api/v1/init {root_path: …}` first |
| `workspace_required` (HTTP 400) | Empty workspace field | Always pass `workspace` arg/body field |
| `memory_search` returns 0 | Embedding queue still working | Check `memory_status.queue_pending`; new docs need embed before vector search hits them. BM25 lands immediately. |
| `chunk truncated before embedding` warns flood log (pre-v2026.6.0202) | Chunker emitted oversize chunks | Upgrade ≥v2026.6.0202 — #297 + #300 align chunker default with embed budget |
| Title-only query returns 0 (pre-v2026.6.0201) | BM25 indexed only chunk content | Upgrade ≥v2026.6.0201 — migration 13 adds title to tsvector (#305) |
| `memory_query` returns score=0 + empty title (pre-v2026.6.0107) | MCP serialization missing JSON tags | Upgrade ≥v2026.6.0107 — #303 added snake_case tags to search.Result |
| New workspace registered but not indexing (pre-v2026.6.0108) | File watcher loaded list once at startup | Upgrade ≥v2026.6.0108 — #308 wires hot-register signal |

## Connection details (rare — most MCP clients handle this for you)

The daemon is expected to be already running on the host. Agents connect via the registered MCP server.

**MCP URL** — set `NANO_BRAIN_MCP_URL` to override the resolved URL:
- Container agents (default): `http://host.docker.internal:3100/mcp`
- Bare-metal / host agents: `http://localhost:3100/mcp`

| Layer | URL | When |
|---|---|---|
| MCP (default) | `http://host.docker.internal:3100/mcp` from container, `http://localhost:3100/mcp` from host | Agent tools |
| HTTP API | Same host, no `/mcp` suffix | See `@references/http-api.md` |

For HTTP / config deep dives: load the matching `@references/` file.

## Troubleshooting

| Symptom | Fix |
|---|---|
| MCP tool calls fail with connection error | Operator action — daemon is not running on the host. Out of scope for agents; surface the error to the user. |
| MCP URL wrong in container | Export `NANO_BRAIN_MCP_URL=http://host.docker.internal:3100/mcp` before starting agent |
| MCP URL wrong on bare-metal | Export `NANO_BRAIN_MCP_URL=http://localhost:3100/mcp` |
| MCP tools return `workspace_not_found` | Call `memory_workspaces_resolve` first to resolve the project path to a registered workspace hash; if `registered: false`, the project has not been initialized. |

