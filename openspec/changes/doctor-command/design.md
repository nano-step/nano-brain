# Design: `doctor` Command

## Checks (in order)

1. **Config file** — Does `~/.nano-brain/config.yml` exist? Parseable?
2. **PostgreSQL** — Can connect to `database.url`? Responds to ping?
3. **pgvector extension** — Is `vector` extension installed? (`SELECT extversion FROM pg_extension WHERE extname = 'vector'`)
4. **Database migrations** — Are all migrations applied? (goose status)
5. **Ollama** — Is embedding provider reachable? (`GET {embedding.url}/api/tags`)
6. **Embedding model** — Is the configured model available? (check model list from Ollama response)

## Output Format

```
nano-brain doctor

  Config file ......... /Users/x/.nano-brain/config.yml OK
  PostgreSQL .......... localhost:5432 OK
  pgvector extension .. v0.8.2 OK
  Migrations .......... 7/7 applied OK
  Ollama .............. localhost:11434 OK
  Embedding model ..... nomic-embed-text OK

All checks passed.
```

On failure:
```
  PostgreSQL .......... localhost:5432 FAIL
    → Connection refused. Is PostgreSQL running?
    → Fix: docker compose up -d postgres
```

## Implementation

- Single file `cmd/nano-brain/doctor.go` with `runDoctorCmd()` function
- Uses `internal/config.Load()` for config
- Direct `pgx` ping for PostgreSQL check
- Raw SQL query for pgvector check
- HTTP GET for Ollama check
- No new dependencies — reuse existing `pgx`, `net/http`, `config`
- `--json` flag for machine-readable output
- Exit code 0 if all pass, 1 if any fail

## Constraints

- Must work without a running nano-brain server (CLI-only, no HTTP calls to self)
- Must not modify any state (read-only checks)
- Must complete within 10 seconds (per-check timeout: 3s)
- Must handle partial failures gracefully (continue checking after first failure)
