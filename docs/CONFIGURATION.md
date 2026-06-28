# Configuration Reference

All configuration options for nano-brain. Config file: `~/.nano-brain/config.yml`

---

## Table of Contents

- [Config File Location](#config-file-location)
- [Environment Variables](#environment-variables)
- [server](#server)
- [database](#database)
- [embedding](#embedding)
- [harvester](#harvester)
- [intervals](#intervals)
- [watcher](#watcher)
- [search](#search)
- [storage](#storage)
- [telemetry](#telemetry)
- [logging](#logging)
- [summarization](#summarization)
- [code_summarization](#code_summarization)
- [intelligence](#intelligence)
- [bench](#bench)
- [flow](#flow)
- [Validation Rules](#validation-rules)
- [Example Config](#example-config)

---

## Config File Location

nano-brain loads config from (highest priority first):

1. `--config` CLI flag
2. `NANO_BRAIN_CONFIG` environment variable
3. `~/.nano-brain/config.yml` (default)

If no config file exists, defaults are used. All options can be overridden via environment variables.

---

## Environment Variables

### Standard prefix: `NANO_BRAIN_`

Format: `NANO_BRAIN_<SECTION>_<KEY>` → maps to `<section>.<key>`

```bash
NANO_BRAIN_SERVER_PORT=3200        # server.port
NANO_BRAIN_DATABASE_URL=...       # database.url (alias)
NANO_BRAIN_LOGGING_LEVEL=debug    # logging.level
```

### Special env vars (non-prefixed)

| Env Var | Maps To | Description |
|---------|---------|-------------|
| `DATABASE_URL` | `database.url` | PostgreSQL connection string |
| `VOYAGE_API_KEY` | `embedding.voyage_api_key` | Voyage AI API key |
| `OPENCODE_STORAGE_DIR` | `harvester.opencode.session_dir` | OpenCode session directory |
| `OPENCODE_DB_PATH` | `harvester.opencode.db_path` | OpenCode single DB path |
| `OPENCODE_DB_ROOT` | `harvester.opencode.db_root` | OpenCode DB root directory |
| `NANO_BRAIN_EMBED_MAX_CHARS` | `embedding.max_chars` | Max chars per embedding call |
| `NANO_BRAIN_SUMMARIZE_API_KEY` | `summarization.api_key` | Summarization API key |
| `NANO_BRAIN_CODE_SUMMARIZE_API_KEY` | `code_summarization.api_key` | Code summarization API key |
| `NANO_BRAIN_AUTH_ENABLED` | `server.auth.enabled` | Enable auth |
| `NANO_BRAIN_AUTH_REALM` | `server.auth.realm` | Auth realm |
| `NANO_BRAIN_AUTH_TOKENS` | `server.auth.tokens` | Auth tokens (comma-separated) |

---

## server

Server binding and authentication.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server.host` | string | `localhost` | Bind address. Use `0.0.0.0` for remote access |
| `server.port` | int | `3100` | HTTP port (1-65535) |
| `server.serve_only` | bool | `false` | Disable all background workers (embed queue, watcher, harvester). Use for proxy containers sharing a DB with a primary instance |

### server.auth

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server.auth.enabled` | bool | `false` | Enable authentication |
| `server.auth.realm` | string | `nano-brain` | HTTP Basic Auth realm |
| `server.auth.tokens` | []string | `[]` | Bearer tokens for API auth |
| `server.auth.users` | []object | `[]` | Basic Auth users (username + bcrypt hash) |
| `server.auth.users[].username` | string | — | Username |
| `server.auth.users[].password_hash` | string | — | bcrypt hash of password |
| `server.auth.bypass_paths` | []string | `["/health"]` | Paths that skip auth |

---

## database

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `database.url` | string | `postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev` | PostgreSQL connection string |

---

## embedding

Embedding provider configuration. Supports Ollama (local) and Voyage AI (cloud).

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `embedding.provider` | string | `ollama` | Provider: `ollama` or `voyage` |
| `embedding.url` | string | `http://localhost:11434` | Provider URL (Ollama endpoint) |
| `embedding.model` | string | `nomic-embed-text` | Model name |
| `embedding.dimension` | int | `0` | Embedding dimension (0 = auto-detect) |
| `embedding.concurrency` | int | `3` | Parallel embedding workers |
| `embedding.max_chars` | int | `3000` | Max chars per embedding call. Keep ≤3000 for nomic-embed-text (2048 token window) |
| `embedding.voyage_api_key` | string | `""` | Voyage AI API key (required when provider=voyage) |

---

## harvester

Session harvesting from AI coding tools.

### harvester.opencode

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `harvester.opencode.session_dir` | string | `""` | OpenCode JSON sessions directory (legacy) |
| `harvester.opencode.db_path` | string | `""` | Single OpenCode SQLite DB path (legacy) |
| `harvester.opencode.db_root` | string | `""` | OpenCode DB root — scans for per-project SQLite DBs (preferred) |

Source resolution priority at daemon startup (first non-empty wins):
1. `db_root` — scan directory for per-project SQLite DBs (new layout)
2. `db_path` — single global SQLite DB (legacy)
3. `session_dir` — filesystem JSON sessions (legacy)

### harvester.claudecode

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `harvester.claudecode.enabled` | bool | `false` | Enable Claude Code session harvesting |
| `harvester.claudecode.session_dir` | string | `""` | Claude Code JSONL sessions directory |

### harvester.git

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `harvester.git.enabled` | bool | `false` | Enable Git history harvesting |

---

## intervals

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `intervals.session_poll` | int | `120` | Session poll interval in seconds |

---

## watcher

File system watcher for auto-reindexing.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `watcher.debounce_ms` | int | `2000` | Debounce delay in milliseconds |
| `watcher.reindex_interval` | int | `300` | Full reindex interval in seconds |
| `watcher.chunk_overlap` | int | `600` | Character overlap between chunks |
| `watcher.exclude_patterns` | []string | `[]` | Global glob patterns to exclude |
| `watcher.allowed_extensions` | []string | `[]` | Global allowed file extensions (empty = all) |

### Per-workspace overrides

Override filters per workspace directory:

```yaml
watcher:
  workspaces:
    /path/to/project:
      exclude_patterns:
        - "node_modules/**"
        - "*.min.js"
      allowed_extensions:
        - ".go"
        - ".ts"
```

---

## search

Hybrid search pipeline (BM25 + vector + RRF).

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.rrf_k` | float | `60` | RRF constant (higher = more weight to lower-ranked results) |
| `search.recency_weight` | float | `0.3` | Recency decay weight (0-1). 0 = no recency boost, 1 = only recency |
| `search.recency_half_life_days` | int | `180` | Days for recency score to decay by half |
| `search.limit` | int | `20` | Max results per query |
| `search.bm25_language` | string | `english` | BM25 language for stemming |

### search.pagerank

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.pagerank_enabled` | bool | `false` | Enable PageRank boosting |
| `search.pagerank_weight` | float | `0.2` | PageRank weight in final score |
| `search.pagerank_edge_threshold` | int | `100` | Min edges for PageRank computation |

### search.entity_boost

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.entity_boost_enabled` | bool | `false` | Boost entity (function, class) matches |
| `search.entity_boost_factor` | float | `0.3` | Boost factor for entity matches |

### search.query_preprocessing

LLM-based query preprocessing (expand abbreviations, fix typos, add context).

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.query_preprocessing.enabled` | bool | `false` | Enable LLM query preprocessing |
| `search.query_preprocessing.provider_url` | string | `""` | LLM provider URL |
| `search.query_preprocessing.api_key` | string | `""` | LLM API key |
| `search.query_preprocessing.model` | string | `""` | LLM model name |
| `search.query_preprocessing.max_latency_ms` | int | `500` | Max latency before fallback to raw query |

### search.hyde

Hypothetical Document Embedding — generates a hypothetical answer, embeds it, and uses that for vector search.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.hyde.enabled` | bool | `false` | Enable HyDE |
| `search.hyde.provider_url` | string | `""` | LLM provider URL |
| `search.hyde.api_key` | string | `""` | LLM API key |
| `search.hyde.model` | string | `""` | LLM model name |
| `search.hyde.max_latency_ms` | int | `500` | Max latency before fallback |
| `search.hyde.context_hints` | map | `{}` | Domain-specific context hints for HyDE generation |

### search.reranking

Cross-encoder reranking for higher precision.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `search.reranking.enabled` | bool | `false` | Enable reranking |
| `search.reranking.provider` | string | `""` | Reranking provider |
| `search.reranking.provider_url` | string | `""` | Provider URL |
| `search.reranking.api_key` | string | `""` | API key |
| `search.reranking.model` | string | `""` | Reranking model |
| `search.reranking.top_k` | int | `0` | Top K results to rerank (0 = all) |
| `search.reranking.max_latency_ms` | int | `500` | Max latency before fallback |
| `search.reranking.min_score` | float | `0` | Minimum reranking score threshold |

---

## storage

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `storage.max_file_size` | int | `314572800` | Max file size in bytes (300 MB) |
| `storage.max_size` | int | `10737418240` | Max total storage in bytes (10 GB) |

---

## telemetry

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `telemetry.retention_days` | int | `90` | Days to retain telemetry data |

---

## logging

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `logging.level` | string | `info` | Log level: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic` |
| `logging.file` | string | `""` | Log file path (empty = stderr) |

---

## summarization

LLM-based session summarization. Produces structured summaries of AI coding sessions.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `summarization.enabled` | bool | `false` | Enable summarization |
| `summarization.provider_url` | string | `""` | LLM provider URL (OpenAI-compatible) |
| `summarization.api_key` | string | `""` | LLM API key |
| `summarization.model` | string | `nano-brain` | LLM model name |
| `summarization.max_tokens` | int | `8000` | Max output tokens per summary |
| `summarization.concurrency` | int | `3` | Parallel summarization workers |
| `summarization.requests_per_second` | float | `0` | Rate limit (0 = unlimited) |
| `summarization.write_to_disk` | bool | `true` | Write summaries to disk (Obsidian-compatible) |
| `summarization.output_dir` | string | `~/.nano-brain/summaries` | Disk output directory |

---

## code_summarization

Batched LLM code symbol summarization. Generates descriptions for functions, types, and interfaces.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `code_summarization.enabled` | bool | `false` | Enable code summarization |
| `code_summarization.provider_url` | string | `""` | LLM provider URL |
| `code_summarization.api_key` | string | `""` | LLM API key |
| `code_summarization.model` | string | `""` | LLM model name (required when enabled) |
| `code_summarization.fallback_model` | string | `""` | Fallback model if primary fails |
| `code_summarization.batch_size` | int | `30` | Symbols per batch |
| `code_summarization.max_batch_tokens` | int | `100000` | Max tokens per batch |
| `code_summarization.max_output_tokens` | int | `8000` | Max output tokens per symbol |
| `code_summarization.max_symbol_lines` | int | `500` | Skip symbols longer than this |
| `code_summarization.concurrency` | int | `2` | Parallel workers |
| `code_summarization.workers` | int | `0` | Worker count (0 = auto) |
| `code_summarization.max_requests_per_day` | int | `0` | Daily rate limit (0 = unlimited) |
| `code_summarization.max_summaries_per_cycle` | int | `300` | Max summaries per poll cycle |
| `code_summarization.poll_interval_seconds` | int | `60` | Poll interval for new symbols |
| `code_summarization.max_retries` | int | `3` | Max retries on failure |
| `code_summarization.retry_backoff_seconds` | int | `1` | Backoff between retries |
| `code_summarization.request_timeout` | int | `600` | Request timeout in seconds |

---

## intelligence

Memory consolidation and LLM categorization.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `intelligence.enabled` | bool | `false` | Enable intelligence features |
| `intelligence.provider_url` | string | `""` | LLM provider URL |
| `intelligence.api_key` | string | `""` | LLM API key |
| `intelligence.model` | string | `claude-sonnet-4-5` | LLM model name |
| `intelligence.max_tokens` | int | `8000` | Max output tokens |
| `intelligence.concurrency` | int | `3` | Parallel workers |
| `intelligence.consolidation_age` | int | `7` | Days before memories are consolidated |

---

## bench

Benchmark configuration for capability testing.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `bench.query_generation` | string | `content` | Query generation mode: `llm` or `content` |
| `bench.provider_url` | string | `""` | LLM provider URL (for llm mode) |
| `bench.api_key` | string | `""` | LLM API key |
| `bench.model` | string | `""` | LLM model name |
| `bench.max_tokens` | int | `0` | Max output tokens |

---

## flow

Execution-flow visualization (HTTP route → handler → downstream calls).

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `flow.enabled` | bool | `false` | Enable flow indexing and extraction |
| `flow.max_depth` | int | `10` | Max call-chain depth (1-10) |
| `flow.max_fanout` | int | `8` | Max fanout per node |
| `flow.summary_enabled` | bool | `false` | Enable LLM flow summaries (requires `summarization.enabled`) |
| `flow.summary_timeout` | int | `600` | Summary timeout in seconds (0 = 10min default) |

---

## Validation Rules

nano-brain validates config on startup. Invalid config → startup fails with error.

| Rule | Error |
|------|-------|
| `server.port` not in 1-65535 | `server.port must be between 1 and 65535` |
| `embedding.concurrency` < 1 | `embedding.concurrency must be >= 1` |
| `search.rrf_k` < 1 | `search.rrf_k must be >= 1` |
| `search.recency_weight` not in 0-1 | `search.recency_weight must be between 0 and 1` |
| `search.recency_half_life_days` < 1 | `search.recency_half_life_days must be >= 1` |
| `search.limit` < 1 | `search.limit must be >= 1` |
| `storage.max_file_size` < 1 | `storage.max_file_size must be >= 1` |
| `storage.max_size` < 1 | `storage.max_size must be >= 1` |
| `storage.max_file_size` > `storage.max_size` | `storage.max_file_size must not exceed storage.max_size` |
| `intervals.session_poll` < 1 | `intervals.session_poll must be >= 1` |
| `watcher.debounce_ms` < 1 | `watcher.debounce_ms must be >= 1` |
| `watcher.reindex_interval` < 1 | `watcher.reindex_interval must be >= 1` |
| `telemetry.retention_days` < 1 | `telemetry.retention_days must be >= 1` |
| `logging.level` invalid | `logging.level "X" is not valid` |
| `auth.enabled` but no users/tokens | `auth enabled but no users or tokens configured` |
| `summarization.enabled` but no provider_url | `summarization.provider_url is required when enabled` |
| `summarization.enabled` but concurrency < 1 | `summarization.concurrency must be >= 1 when enabled` |
| `code_summarization.enabled` but no provider_url | `code_summarization.provider_url is required when enabled` |
| `code_summarization.enabled` but no model | `code_summarization.model is required when enabled` |
| `flow.enabled` but max_depth not in 1-10 | `flow.max_depth must be between 1 and 10 when enabled` |
| `flow.enabled` but max_fanout ≤ 0 | `flow.max_fanout must be greater than 0 when enabled` |
| `flow.summary_enabled` but summarization disabled | `flow.summary_enabled requires summarization.enabled to be true` |

---

## Example Config

```yaml
# ~/.nano-brain/config.yml

server:
  host: localhost
  port: 3100
  serve_only: false
  auth:
    enabled: false
    realm: nano-brain
    tokens: []
    users: []
    bypass_paths:
      - /health

database:
  url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev

embedding:
  provider: ollama
  url: http://localhost:11434
  model: nomic-embed-text
  dimension: 0
  concurrency: 3
  max_chars: 3000

harvester:
  opencode:
    session_dir: ""
    db_path: ""
    db_root: ""
  claudecode:
    enabled: false
    session_dir: ""
  git:
    enabled: false

intervals:
  session_poll: 120

watcher:
  debounce_ms: 2000
  reindex_interval: 300
  chunk_overlap: 600
  exclude_patterns: []
  allowed_extensions: []

search:
  rrf_k: 60
  recency_weight: 0.3
  recency_half_life_days: 180
  limit: 20
  bm25_language: english
  pagerank_enabled: false
  pagerank_weight: 0.2
  pagerank_edge_threshold: 100
  entity_boost_enabled: false
  entity_boost_factor: 0.3
  query_preprocessing:
    enabled: false
  hyde:
    enabled: false
  reranking:
    enabled: false

storage:
  max_file_size: 314572800    # 300 MB
  max_size: 10737418240       # 10 GB

telemetry:
  retention_days: 90

logging:
  level: info

summarization:
  enabled: false

code_summarization:
  enabled: false

intelligence:
  enabled: false

bench:
  query_generation: content

flow:
  enabled: false
  max_depth: 10
  max_fanout: 8
  summary_enabled: false
```

### Production config (VPS with auth)

```yaml
server:
  host: 0.0.0.0
  port: 3100
  auth:
    enabled: true
    tokens:
      - "nbt_your_token_here"
    bypass_paths:
      - /health

database:
  url: postgres://nanobrain:secret@db-host:5432/nanobrain_prod

embedding:
  provider: voyage
  voyage_api_key: "voyage_key_here"
  model: voyage-3
  concurrency: 5

summarization:
  enabled: true
  provider_url: https://api.openai.com/v1
  api_key: "sk-..."
  model: gpt-4o-mini
  concurrency: 5

code_summarization:
  enabled: true
  provider_url: https://api.openai.com/v1
  api_key: "sk-..."
  model: gpt-4o-mini
  batch_size: 50
  workers: 4

flow:
  enabled: true
  summary_enabled: true
```
