---
name: nano-brain daemon config reference
description: Full nano-brain config.yml schema, defaults, env-var overrides. Load when configuring the daemon, debugging config issues, or onboarding a new deployment.
---

# nano-brain daemon config reference

The daemon reads `~/.nano-brain/config.yml` by default (override via `--config <path>`). Defaults come from `internal/config/defaults.go`; the full schema is `internal/config/config.go:34-176`.

```yaml
# Connection + auth
server:
  host: localhost           # listen address; binding non-loopback requires server.auth.enabled OR --unsafe-no-auth
  port: 3100
  serve_only: false         # NEW (#282) — true disables embed queue, file watcher, harvester, summarizer.
                            # Use for read-only HTTP/MCP frontends in containers / read replicas.
  auth:
    enabled: false          # if true, require Basic or Bearer auth on all non-/health routes
    realm: "nano-brain"
    bypass_paths: ["/health"]
    # users and tokens are configured via env: NANO_BRAIN_AUTH_USERS, NANO_BRAIN_AUTH_TOKENS

# Storage
database:
  url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev
  # also accepts DATABASE_URL env var (no NANO_BRAIN_ prefix needed)

# Embedding pipeline
embedding:
  provider: ollama          # ollama | voyageai
  url: http://localhost:11434
  model: nomic-embed-text   # 2048-token context window
  concurrency: 3
  max_chars: 3000           # matches chunker DefaultMaxChunkBytes (#300). Lower if provider still 400s on dense CSV.
                            # override via NANO_BRAIN_EMBED_MAX_CHARS
  voyage_api_key: ""        # if provider=voyageai; or VOYAGE_API_KEY env

# Session harvesters (read AI coding agent sessions into the corpus)
harvester:
  opencode:
    db_root: ""             # ~/.ai-sandbox/opencode-dbs (multi-DB; preferred)
    db_path: ""             # ~/.local/share/opencode/opencode.db (single SQLite)
    session_dir: ""         # legacy JSON
  claudecode:
    enabled: false
    session_dir: ""         # ~/.claude/sessions
intervals:
  session_poll: 120         # harvester tick interval (seconds)

# File watcher
watcher:
  debounce_ms: 2000
  reindex_interval: 300     # full re-scan interval (seconds)
  exclude_patterns: []      # global glob excludes
  allowed_extensions: []    # if non-empty, ONLY these are watched

# Search ranking
search:
  rrf_k: 60                 # RRF smoothing constant; higher = flatter ranking
  recency_weight: 0.3       # blend recency vs relevance (0..1)
  recency_half_life_days: 180
  limit: 20                 # default max results per query

# Storage budgets
storage:
  max_file_size: 314572800  # 300 MB; bigger files are skipped at chunk time
  max_size: 10737418240     # 10 GB total; harvester back-off at threshold

# Summarization (optional, off by default — LLM credits required)
summarization:
  enabled: false
  provider_url: ""          # OpenAI-compatible endpoint
  api_key: ""               # or NANO_BRAIN_SUMMARIZE_API_KEY env
  model: "nano-brain"       # logical model name (provider-specific)
  max_tokens: 8000          # bumped from 4096 — issue #191
  concurrency: 3
  requests_per_second: 0    # 0 = unlimited; set for rate-limited providers
  write_to_disk: null       # *bool — default true; set false to skip Obsidian-vault export (#258)
  output_dir: "~/.nano-brain/summaries"  # tilde-expanded

# Operational
telemetry:
  retention_days: 90
logging:
  level: info               # debug | info | warn | error
  file: ""                  # stdout if empty
```

## Env-var override gotcha

koanf maps env vars by **converting the first underscore after the `NANO_BRAIN_` prefix to a dot, and leaving the rest as-is.** This means:

| Env var | Maps to |
|---|---|
| `NANO_BRAIN_SERVER_PORT` | `server.port` (works) |
| `NANO_BRAIN_EMBEDDING_MODEL` | `embedding.model` (works) |
| `NANO_BRAIN_HARVESTER_OPENCODE_SESSION_DIR` | `harvester.opencode_session_dir` ❌ — DOES NOT WORK |

For deeply nested keys use the special non-prefixed env vars or set them in YAML:
- `DATABASE_URL` → `database.url`
- `VOYAGE_API_KEY` → `embedding.voyage_api_key`
- `OPENCODE_STORAGE_DIR` → `harvester.opencode.session_dir`
- `NANO_BRAIN_SUMMARIZE_API_KEY` → `summarization.api_key`
- `NANO_BRAIN_AUTH_USERS` / `NANO_BRAIN_AUTH_TOKENS` → `server.auth.users` / `.tokens`

## Validation rules (enforced at startup)

The daemon refuses to start if config violates:

| Rule | Source |
|---|---|
| `server.port` ∈ [1, 65535] | config.go:308 |
| `embedding.concurrency` ≥ 1 | config.go:313 |
| `search.rrf_k` ≥ 1 | config.go:318 |
| `search.recency_weight` ∈ [0, 1] | config.go:321 |
| `search.recency_half_life_days` ≥ 1 | config.go:324 |
| `search.limit` ≥ 1 | config.go:327 |
| `storage.max_file_size` ≤ `storage.max_size`, both ≥ 1 | config.go:332-340 |
| `intervals.session_poll` ≥ 1 | config.go:343 |
| `watcher.debounce_ms` ≥ 1 | config.go:348 |
| `watcher.reindex_interval` ≥ 1 | config.go:351 |
| `telemetry.retention_days` ≥ 1 | config.go:355 |
| `logging.level` is valid zerolog level (or empty) | config.go:359 |
| If `server.auth.enabled`: must have ≥1 user OR ≥1 token | config.go:366 |
| If `summarization.enabled`: `provider_url` required, `concurrency` ≥ 1 | config.go:373-379 |

## Hot reload

Change config and apply without restart:

```
curl -X POST http://localhost:3100/api/reload-config
```

Returns `{reloaded: [...], unchanged: [...], requires_restart: [...]}`. Most fields hot-reload; `database.url`, `server.host`, `server.port` require restart.

## Multi-DB OpenCode harvest

If you use `ai-sandbox-wrapper` (per-project SQLite at `~/.ai-sandbox/opencode-dbs/<slug>-<8hex>/opencode.db`), set:

```yaml
harvester:
  opencode:
    db_root: ~/.ai-sandbox/opencode-dbs
```

Verify via `/api/status`:

```
curl -s http://localhost:3100/api/status | jq '.harvester_status.opencode'
# {"enabled":true, "mode":"db_root", "db_root":"…", "db_count":N, ...}
```

`mode` values: `db_root` (multi-DB), `db_path` (single SQLite), `session_dir` (legacy JSON), `disabled`.

## Auth setup (for VPS / remote deployment)

When binding non-loopback (`server.host` ≠ localhost), the daemon REFUSES to start unless one of:
- `server.auth.enabled: true` with at least one user or token configured, OR
- `--unsafe-no-auth` CLI flag (explicit acknowledgement of the risk)

This is the "bind-safety" guard. See `cmd/nano-brain/bindsafety.go`.

Configure auth via env:

```
NANO_BRAIN_AUTH_USERS=admin:$(htpasswd -nB admin | cut -d: -f2)
NANO_BRAIN_AUTH_TOKENS=secret-bearer-token-1,secret-bearer-token-2
```

Or YAML (less preferred — secret in plaintext config):

```yaml
server:
  auth:
    enabled: true
    users:
      - username: admin
        password_hash: $2y$10$...
    tokens:
      - "secret-bearer-token-1"
```
