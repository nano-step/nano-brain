---
name: nano-brain HTTP API
description: Direct HTTP integration patterns for nano-brain — for scripts, dashboards, custom tooling, and tests that bypass the CLI/MCP layer.
---

# nano-brain HTTP API

Use direct HTTP when:
- Writing tests that need deterministic request/response control
- Building a custom dashboard or monitoring integration
- Embedding nano-brain in a non-Node/Go runtime
- Debugging — `curl` + `jq` is the fastest path to inspecting daemon state

## Connection

Default: `http://localhost:3100`. Container agents auto-set `NANO_BRAIN_HOST=host.docker.internal` so calls from inside containers reach the host daemon.

```bash
BASE="http://localhost:3100"   # or http://host.docker.internal:3100 inside a container

# Liveness probe
curl -sf "$BASE/health" >/dev/null && echo "ready"

# Detailed status
curl -s "$BASE/api/status" | jq
```

## Workspace handle

Every data endpoint needs a workspace hash. Get one via init (idempotent — same `root_path` always yields the same hash via SHA-256 of the canonical absolute path):

```bash
WS=$(curl -s -X POST "$BASE/api/v1/init" \
  -H 'Content-Type: application/json' \
  -d "{\"root_path\":\"$PWD\"}" | jq -r .workspace_hash)
echo "$WS"
```

Workspace hashes are stable across daemon restarts and machines (deterministic). Two different machines that init the same path produce different hashes (the path differs) — workspace is path-scoped, not content-scoped.

## Search — pick the right mode

| Endpoint | Engine | Strengths | Latency |
|---|---|---|---|
| `/api/v1/search` | PostgreSQL `tsvector` (BM25) | Exact terms, code identifiers, error messages | ~10ms |
| `/api/v1/vsearch` | pgvector cosine | Semantic concepts, paraphrases | ~50ms (needs embed) |
| `/api/v1/query` | Hybrid (BM25 + vector + RRF) | "Best of both" — recommended default | ~80ms |

### BM25 — keyword/identifier search

```bash
curl -s -X POST "$BASE/api/v1/search" \
  -H 'Content-Type: application/json' \
  -d "{
    \"workspace\": \"$WS\",
    \"query\": \"ECONNREFUSED redis\",
    \"max_results\": 10,
    \"tags\": [\"bug\"]
  }" | jq '.results[] | {title, score, snippet}'
```

`tags` is OPTIONAL on all three search endpoints (`search`, `vsearch`, `query`) — multi-tag filter is conjunctive (chunk must have ALL listed tags). Per `internal/server/handlers/{bm25,search,query}.go`.

### Vector — fuzzy semantic search

```bash
curl -s -X POST "$BASE/api/v1/vsearch" \
  -H 'Content-Type: application/json' \
  -d "{
    \"workspace\": \"$WS\",
    \"query\": \"rate limiting strategy\",
    \"max_results\": 10
  }" | jq '.results'
```

The server embeds the query first (one ollama call ≈ 30-60ms), then runs pgvector cosine ANN.

### Hybrid — best for most questions

```bash
curl -s -X POST "$BASE/api/v1/query" \
  -H 'Content-Type: application/json' \
  -d "{
    \"workspace\": \"$WS\",
    \"query\": \"how did we handle session invalidation\",
    \"max_results\": 10
  }" | jq '.results[] | {title, score, snippet, source_path}'
```

Hybrid uses **Reciprocal Rank Fusion** to merge BM25 + vector rankings. Single result list with combined scores. Falls back to BM25-only if vector store empty.

## Document write

```bash
curl -s -X POST "$BASE/api/v1/write" \
  -H 'Content-Type: application/json' \
  -d "{
    \"workspace\": \"$WS\",
    \"content\": \"## Decision: bla\\n- Why: bla\",
    \"tags\": [\"decision\", \"queue\"],
    \"collection\": \"memory\",
    \"title\": \"Use Redis Streams\",
    \"source_path\": \"memory://decision/redis-streams\"
  }" | jq
```

Returns `{id, hash, collection, workspace_hash, chunk_count, warning?}`.

- `content_hash` deduplication: re-POST same content → 200 OK with `chunk_count: 0` (existing doc unchanged).
- `source_path` is the upsert key. Same `source_path` with different `content` → in-place update, old chunks deleted, new chunks enqueued for embedding.
- Without `source_path`, falls back to content-hash dedup only.
- `chunk_count` returned is the number of chunks queued for embedding. They embed async via the queue (`status` endpoint shows `queue_pending`).

## Status endpoint — what to monitor

```bash
curl -s "$BASE/api/status" | jq
```

Returns (per `internal/server/handlers/health.go:111-122`):

```json
{
  "pg_status": "healthy",
  "migration_version": 13,
  "embedding_queue_depth": 0,        // in-memory queue depth
  "active_provider": "ollama",
  "workspace_count": 1,
  "queue_depth": 0,
  "queue_capacity": 10000,
  "queue_status": "idle",             // "idle" | "busy" | "backpressure"
  "queue_pending": 42,                // DB-pending chunks awaiting embed (all workspaces)
  "harvester_status": {
    "poll_interval_seconds": 120,
    "opencode": {
      "enabled": true,
      "mode": "db_root",              // db_root | db_path | session_dir | disabled
      "db_root": "/Users/x/.ai-sandbox/opencode-dbs",
      "db_count": 1
    },
    "claudecode": {
      "enabled": false,
      "session_dir": ""
    }
  }
}
```

Watch `queue_pending` over time — if it grows without bound and `queue_status == "backpressure"`, embedding throughput is below ingestion rate. Check ollama health via `npx nano-brain doctor`.

## Trigger operations

```bash
# Force-run harvesters
curl -s -X POST "$BASE/api/harvest" | jq

# Reindex a workspace from disk
curl -s -X POST "$BASE/api/v1/reindex" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"root\":\"$PWD\"}" | jq

# Reload config without daemon restart
curl -s -X POST "$BASE/api/reload-config" | jq
```

## Workspace deletion

Two endpoints, different semantics:

```bash
# Reset: wipe all docs+chunks+embeddings but keep workspace registration
curl -s -X POST "$BASE/api/v1/reset-workspace" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\"}" | jq
# Returns: {deleted_documents, workspace}

# Remove: fully delete workspace + all its data (cascades graph_edges too)
curl -s -X DELETE "$BASE/api/v1/workspaces/$WS" | jq
# Returns: {workspace, deleted_docs, workspace_removed: true}
```

## Document operations

```bash
# Fetch one document by source_path or #<uuid>
curl -s -X POST "$BASE/api/v1/get" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"path\":\"memory://decision/redis-streams\"}" | jq

# Batch fetch — returns {results, not_found}
curl -s -X POST "$BASE/api/v1/multi-get" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"paths\":[\"...\"],\"ids\":[\"...\"]}" | jq

# List documents with optional filters
curl -G "$BASE/api/v1/documents" \
  --data-urlencode "workspace=$WS" \
  --data-urlencode "collection=memory" \
  --data-urlencode "tags=decision,architecture" \
  --data-urlencode "text=stripe" | jq

# Delete a single document (cascades chunks + embeddings)
curl -s -X DELETE "$BASE/api/v1/documents/<doc-uuid>" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\"}" | jq
# Returns: {deleted_id}
```

## Graph endpoints

```bash
# Workspace overview — top-N most-connected nodes (NEW #287)
curl -s -X POST "$BASE/api/v1/graph/overview" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"mode\":\"code\",\"limit\":50}" | jq

# 1-hop neighbors
curl -s -X POST "$BASE/api/v1/graph/query" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"node\":\"/abs/path.go::FnName\",\"direction\":\"out\",\"edge_type\":\"calls\"}" | jq

# BFS neighborhood with depth + frontier markers (used by /ui/graph)
curl -s -X POST "$BASE/api/v1/graph/neighborhood" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"focus\":\"FnName\",\"depth\":2,\"node_kind\":\"symbol\"}" | jq

# Reverse impact (what breaks if FnName changes); max_depth clamped 1-3
curl -s -X POST "$BASE/api/v1/graph/impact" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"node\":\"FnName\",\"max_depth\":2}" | jq

# Forward call chain with cycle detection; max_depth clamped 1-10
curl -s -X POST "$BASE/api/v1/graph/trace" \
  -H 'Content-Type: application/json' \
  -d "{\"workspace\":\"$WS\",\"node\":\"FnName\",\"max_depth\":5}" | jq
```

## Error responses

All endpoints return JSON for both success and error:

| HTTP | Meaning | Common cause |
|---|---|---|
| 200 | OK | — |
| 400 | Bad request | Missing `workspace` or `query` field; invalid `source_path` URI |
| 404 | Not found | Workspace hash unregistered |
| 422 | Unprocessable | Schema validation (e.g. `max_depth > 3`) |
| 500 | Server error | PG down, ollama unreachable; check `/health` |

Error body: `{"message":"<reason>"}`.

## Streaming + MCP

The daemon also exposes:

- `POST /mcp` — Streamable HTTP MCP (preferred for agents)
- `GET /sse`, `POST /sse` — SSE transport (legacy MCP clients)

Both expose the same 13 `memory_*` tools as MCP. JSON-RPC 2.0 protocol.

For OpenCode / Claude Code agents, add to MCP config:
```json
{
  "mcpServers": {
    "nano-brain": {
      "url": "http://host.docker.internal:3100/mcp"
    }
  }
}
```

## Performance budget

Rough p99 latencies on local hardware (Postgres 17 + pgvector 0.8.2 + ollama nomic-embed-text):

| Operation | Latency |
|---|---|
| `GET /health` | <5ms |
| `GET /api/status` | <20ms |
| `POST /api/v1/search` (BM25, 10 results) | 10-30ms |
| `POST /api/v1/vsearch` (1 ollama call + ANN) | 40-100ms |
| `POST /api/v1/query` (hybrid) | 60-150ms |
| `POST /api/v1/write` (small doc, sync) | 20-50ms (+ async embedding) |
| `POST /api/v1/reindex` | seconds to minutes — scales with file count |
| `POST /api/harvest` | seconds — scales with new sessions since last tick |

If `vsearch` or `query` is slow, ollama is the bottleneck — check `embedding.url` reachability and model load.

## Auth (VPS / remote deployment)

By default the daemon binds to `localhost` only and skips auth. To bind non-loopback:

1. **Enable HTTP auth** via `server.auth.enabled: true` in config + add users/tokens, OR
2. **Acknowledge the risk** with `--unsafe-no-auth` CLI flag (e.g. inside a private network or VPN).

The daemon refuses to start in any other configuration (see `bindsafety.go`). Auth supports Basic (user+bcrypt-hashed password) and Bearer (static token). See `@references/config-reference.md` for full setup.

CSRF middleware blocks state-changing requests from browsers without the `X-Requested-With: nano-brain-ui` header — non-browser clients (curl, scripts) are unaffected. See `internal/server/middleware/csrf.go`.
