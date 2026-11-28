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
| `NANO_BRAIN_TOKEN` | — | CLI bearer token; overrides the first configured `server.auth.tokens` entry |
| `NANO_BRAIN_ALLOW_INSECURE_REMOTE_TOKEN` | — | Set to `1` to permit CLI credentials over remote HTTP; unsafe without a trusted network |

---

## server

Server binding and authentication.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server.host` | string | `localhost` | Bind address. Use `0.0.0.0` for remote access |
| `server.port` | int | `3100` | HTTP port (1-65535) |
| `server.allowed_hosts` | []string | built-in set | `Host` headers accepted on **all routes** (REST API, `/health`, `/mcp`, `/sse`) — a DNS-rebinding guard; any other `Host` gets `403`. Enforced for connections that arrive on a loopback address, and for **every** connection while `server.auth.enabled` is `false` (e.g. the Docker deployment, where the published port lands on the container address); skipped only for non-loopback connections with auth enabled. Separately, state-changing requests (`POST`/`PUT`/`PATCH`/`DELETE`) on any route that carry a foreign `Origin` (or `Referer`) are rejected with `403`; requests with neither header (CLI, curl, MCP clients) are unaffected. Built-in set: `localhost`, `127.0.0.1`, `::1`, `host.docker.internal`. A non-empty list **replaces** the built-in set. Needed when a reverse proxy or tunnel on the same host forwards the public hostname. Env: `NANO_BRAIN_SERVER_ALLOWED_HOSTS` (comma-separated). Requires a restart |
| `server.serve_only` | bool | `false` | Disable all background workers (embed queue, watcher, harvester). Use for proxy containers sharing a DB with a primary instance |

### Request limits

Fixed, not configurable:

- **Body size:** 4 MiB on every route except `/mcp` and `/sse`, which accept 32 MiB. A larger body is answered with `413`, whether or not it declares a `Content-Length`.
- **`POST /api/v1/multi-get`:** at most 200 `paths` and `ids` in total; more is answered with `400`.
- **Timeouts:** 10 s to send the request headers, 120 s for an idle keep-alive connection. There is no read or write deadline on the body, because `/sse`, `/api/v1/events` and long MCP calls stream.
- **`POST /api/harvest`, `POST /api/reload-config`:** a browser request must come from the daemon's own origin (same host and port); another local port is answered with `403`. Requests without `Origin` and `Sec-Fetch-Site` (CLI, curl) are unaffected.

### server.auth

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `server.auth.enabled` | bool | `false` | Enable authentication |
| `server.auth.realm` | string | `nano-brain` | HTTP Basic Auth realm. Letters, digits, spaces, `.`, `_` and `-` only |
| `server.auth.tokens` | []string | `[]` | Bearer tokens for API auth. Empty tokens are rejected |
| `server.auth.users` | []object | `[]` | Basic Auth users (username + bcrypt hash). When auth is enabled, startup fails on an invalid entry |
| `server.auth.users[].username` | string | — | Username. Must be non-empty, unique, without leading or trailing whitespace, and must not contain `:` or control or invisible characters (newline, zero-width space, BOM) |
| `server.auth.users[].password_hash` | string | — | bcrypt hash of password. Must be a complete bcrypt hash, every user must use the same bcrypt cost, and the cost must not exceed 14; generate one with `nano-brain auth hash <password>` |
| `server.auth.bypass_paths` | []string | `["/health", "/api/openapi.json"]` | Paths that skip auth. On a non-loopback bind, startup prints a warning for every entry other than `/health` and `/api/openapi.json` |

`GET /api/v1/config` and `nano-brain config show` redact credentials embedded in URLs: the `user:password@` part and the values of the `api_key`, `key`, `token`, `access_token`, `secret` and `password` query parameters are shown as `<redacted>`.

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

> **Privacy:** nano-brain applies *egress-only credential redaction* — every
> request sent to an embedding, summarization, reranking, or LLM provider is
> scrubbed by `internal/redact` (API keys, tokens, `.env` secrets, PEM blocks,
> JWTs → `[REDACTED:<kind>]`). Content stored in the local database is **not**
> altered, so local search and retrieval stay lossless.

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
| `harvester.claudecode.session_dir` | string | `""` | Claude Code projects parent directory |
| `harvester.claudecode.session_dirs` | list | `[]` | Additional Claude Code projects directories (`~` and globs like `~/.ccs/instances/*/projects` supported); `session_dir` is treated as one more entry |

Claude entries are **projects parent directories**, not leaf session directories. See [CCS_HARVESTING.md](CCS_HARVESTING.md) for the user-facing CCS setup/UX flow and real-transcript E2E evidence. When `enabled: true` and both `session_dir` and `session_dirs` are empty, nano-brain auto-detects platform Claude locations plus CCS instance roots at startup. CCS detection checks `CCS_DIR`, `$CCS_HOME/.ccs`, and `~/.ccs` as a union of possible historical roots and uses each root's `instances/*/projects` glob. At runtime the standard platform Claude patterns and default `~/.ccs` instance glob are retained even before their directories exist, so a first install after daemon startup can still be discovered. A per-invocation CCS `--config-dir` cannot be inferred; add its projects glob explicitly. Relative `CCS_DIR`/`CCS_HOME` values are not guessed against the daemon cwd — use absolute paths or explicit `session_dirs`.

A non-empty `session_dir` or `session_dirs` is a complete explicit source list and disables auto-detection. For a custom CCS root, include every standard location you still want:

```yaml
harvester:
  claudecode:
    enabled: true
    session_dir: ""
    session_dirs:
      - ~/.claude/projects
      - ~/.ccs/instances/*/projects
      - /path/to/custom-ccs/instances/*/projects
```

Configured patterns are re-expanded on every harvest cycle, so new CCS profiles, first sessions, and newly registered workspaces do not require a daemon restart. Editing the YAML values themselves still requires restart. Scanning multiple CCS profiles merges those profiles' sessions into the same workspace memory; use a restrictive explicit list if that is not desired.

Existing config files are never rewritten implicitly. To opt in to auto-detected Claude/CCS harvesting, set `enabled: true` and leave both directory fields empty, then restart nano-brain. If harvesting was already enabled with empty directory fields before CCS support, upgrading widens discovery from the platform Claude directory to the documented union of Claude/CCS roots; pin an explicit `session_dirs` list if you need a narrower privacy boundary. Harvested session documents already written before disabling or narrowing the config are not deleted automatically.

Claude's encoded project directory names are lossy. nano-brain therefore uses them only for discovery and, for modern transcripts containing `cwd`, skips a transcript when that `cwd` does not match the registered workspace being harvested. Older transcripts without `cwd` keep the legacy encoded-directory attribution.

### harvester.pi

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `harvester.pi.enabled` | bool | `false` | Enable Pi CLI agent session harvesting |
| `harvester.pi.session_dir` | string | `""` | Pi JSONL sessions directory (e.g. `$HOME/.pi/agent/sessions`) — set as an absolute path; nano-brain does not expand `~` for this key |

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

### Built-in safety filters

These always apply. There is no opt-out: `exclude_patterns`, `allowed_extensions`, `.gitignore` and `.nano-brainignore` (including `!` negations) cannot turn them off.

**Symlinks.** A symlinked file is read only if its real path (all symlinks resolved) lies inside the real path of the collection root *and* the file it resolves to would itself be indexed:

- A symlink that leaves the root is not indexed. It is logged at `warn` the first time it is seen in a process and at `debug` afterwards. To index shared files that live elsewhere, register their directory as its own collection.
- A symlink to a file on the credential deny list below is not indexed, whatever the link is called (`docs/setup.md -> ../.env`).
- A symlink to a file excluded by the built-in directory/lockfile excludes, the global `.nano-brainignore`, a `.gitignore` or `.nano-brainignore` in the root or in any directory between the root and the target, `exclude_patterns` or `allowed_extensions` is not indexed either (`readme2.md -> tok.txt` with `tok.txt` gitignored; `alias.md -> sub/priv.txt` with `priv.txt` listed in `sub/.gitignore`). A target whose spelling differs from its on-disk directory entries — different casing, or a different Unicode normalisation form (NFC vs NFD) — is refused even when it would resolve to the same file; a legitimate link whose spelling does not match the disk entry is skipped rather than indexed.
- A symlink to another indexable file inside the root is indexed as usual.
- A broken symlink is skipped. Symlinked directories are not followed.
- On a case-insensitive filesystem a root registered in a different casing than on disk still contains its own files: containment is decided by the directory itself, not by how the path is spelled.

A document indexed by an earlier version for a symlink in one of the skipped cases is removed on the next scan of its collection.

**Non-regular files.** Named pipes (FIFOs), sockets and device files considered for indexing are skipped at `debug` level, including symlinks to them. Non-regular `.gitignore` and `.nano-brainignore` files — in the root or any subdirectory — are not parsed, and neither is the global `~/.nano-brain/.nano-brainignore` if it is not a regular file. The same 4 MiB bound applies to every ignore file, so one that resolves to a regular-but-unbounded pseudo file (a committed symlink to `/proc/self/pagemap` on Linux) is treated as absent instead of exhausting memory; a global ignore file over 4 MiB is likewise skipped, with a warning logged at startup. Framework detection and JS import alias discovery (`go.mod`, `package.json`, `Gemfile`, `tsconfig.json`, `jsconfig.json`) read a filename only when it resolves to a regular file of at most 4 MiB, so a FIFO cannot block the scan and a symlink to a device cannot exhaust memory. Incremental reindex applies its 10 MiB size limit to the symlink's target and bounds the hash read itself. Two residual cases remain: the check-then-open window (a file swapped for a special file between the type check and the read, which requires a local attacker racing the scan) and hard links to files outside the root.

**Credential files.** Not indexed, matched on the file's base name, case-insensitively, at any depth:

| Denied | Still indexed |
|--------|---------------|
| `.env`, `.env.*`, `*.env` (`prod.env`), `.envrc` | names marked `example`, `sample`, `template` or `dist`: `.env.example`, `.env.production.sample`, `example.env`, `app.example.env` |
| `.env-*`, `.env_*` (`.env-production`, `.env_local`) | names ending in `.`, `-` or `_` plus `example`, `sample`, `template` or `dist`: `.env-production.example`, `.env-example`, `.env_sample` |
| any name starting with `id_rsa`, `id_dsa`, `id_ecdsa`, `id_ed25519` (`id_rsa`, `id_rsa.bak`, `id_rsa_github`, `id_ed25519_sk`, `id_rsa.txt`) | the same names ending in `.pub` (`id_ed25519.pub`) or in a source-code or Markdown extension (`id_rsa_test.go`, `id_dsa.md`); the full list is below |
| `*.pem`, `*.key`, `*.p12`, `*.pfx`, `*.jks`, `*.keystore`, `*.ppk`, `*.kdbx`, `*.ovpn`, `*.pem.bak`, `*.key.bak` | |
| `.npmrc`, `.netrc`, `.pgpass`, `.htpasswd`, `htpasswd`, `.git-credentials`, `.pypirc`, `.dockercfg`, `.vault-token`, `.my.cnf`, `.s3cfg`, `.boto`, `kubeconfig`, `kubeconfig.yaml`, `kubeconfig.yml` | `kubeconfig.md`, … (exact names only) |
| `credentials`, `credentials.json`, `credentials.yml`, `credentials.yaml`, `secrets.yml`, `secrets.yaml`, `secrets.json` | `credentials.md`, `secrets.md`, … (exact names only) |
| `client_secret*.json`, `service-account*.json`, `service_account*.json` | `client_secret.go`, `service-account.md` (only `.json`) |
| `*.tfstate`, `*.tfstate.backup`, `*.tfvars`, `*.tfvars.json` | `terraform.tfvars.example` |

SSH key names: a name starting with `id_rsa`, `id_dsa`, `id_ecdsa` or `id_ed25519` is denied unless it ends in `.pub` or its final extension is one of `.go`, `.py`, `.js`, `.ts`, `.tsx`, `.jsx`, `.rb`, `.rs`, `.java`, `.kt`, `.c`, `.h`, `.cpp`, `.cs`, `.php`, `.swift`, `.sh`, `.html`, `.css`, `.md`. Text and data extensions (`.txt`, `.json`, `.yaml`, `.yml`, `.toml`) are deliberately not on that list: `id_rsa.txt` is a plausible key backup.

The match is by name only, so it also excludes files that are not secrets: public certificates and CA bundles (`*.pem`), Keynote presentations (`*.key`), an `.npmrc` holding only registry settings, and Terraform variable files without secrets (`*.tfvars`). Rename such a file if it must be searchable.

A document indexed for a denied file by an earlier version is removed on the next scan of its collection.

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
| `auth.enabled` and a user with an empty or whitespace-only username | `server.auth.users[N].username must not be empty` |
| `auth.enabled` and a username containing `:` | `server.auth.users[N].username must not contain ':'` |
| `auth.enabled` and a repeated username | `server.auth.users[N].username "X" is a duplicate` |
| `auth.enabled` and a username with leading or trailing whitespace | `server.auth.users[N].username must not start or end with whitespace` |
| `auth.enabled` and a username containing a control or format character (newline, NUL, zero-width space, BOM) | `server.auth.users[N].username must not contain control or invisible characters` |
| `auth.enabled` and an incomplete or unparsable `password_hash` | `server.auth.users[N].password_hash is not a valid bcrypt hash` |
| `auth.enabled` and users hashed at different bcrypt costs | `server.auth.users[N].password_hash has bcrypt cost X but an earlier user has cost Y` |
| `auth.enabled` and a `password_hash` with a bcrypt cost above 14 | `server.auth.users[N].password_hash has bcrypt cost X; costs above 14 make every login attempt, including unauthenticated ones, very slow` |
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
    # session_dirs:
    #   - ~/.claude/projects
    #   - ~/.ccs/instances/*/projects
  pi:
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
