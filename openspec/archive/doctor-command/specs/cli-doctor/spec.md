# Spec: `doctor` Command

## ADDED Requirements

### Requirement: Doctor CLI subcommand

The system MUST provide a `nano-brain doctor [--json]` command that checks all prerequisites and reports their status.

#### Scenario: All prerequisites are met

- Given PostgreSQL with pgvector is running and accessible
- And Ollama is running with the configured embedding model
- And config file exists and is valid
- And all migrations are applied
- When user runs `nano-brain doctor`
- Then each check shows "OK" with detail info
- And exit code is 0

#### Scenario: PostgreSQL is not reachable

- Given PostgreSQL is not running
- When user runs `nano-brain doctor`
- Then PostgreSQL check shows "FAIL" with hint "Is PostgreSQL running? Try: docker compose up -d"
- And remaining checks still execute
- And exit code is 1

#### Scenario: JSON output mode

- Given any environment state
- When user runs `nano-brain doctor --json`
- Then output is valid JSON with `checks` array and `all_passed` boolean
- And each check has `name`, `status`, `detail` fields

### Requirement: Six prerequisite checks

The doctor command MUST run these checks in order:

1. **Config file** — `config.Load()` succeeds
2. **PostgreSQL** — `pgx.Connect` + `Ping` succeeds
3. **pgvector extension** — `SELECT extversion FROM pg_extension WHERE extname = 'vector'` returns a row
4. **Migrations** — All goose migrations applied
5. **Ollama** — `GET {embedding.url}/api/tags` returns HTTP 200 (skip if provider is voyage)
6. **Embedding model** — Configured model found in Ollama response (or `VOYAGE_API_KEY` set if voyage)

#### Scenario: Voyage AI provider configured

- Given config has `embedding.provider: voyage`
- When user runs `nano-brain doctor`
- Then Ollama check is skipped
- And embedding model check validates `VOYAGE_API_KEY` env var is non-empty

#### Scenario: Per-check timeout

- Given a check hangs (e.g., unreachable host)
- When 3 seconds elapse for that check
- Then the check is marked as FAIL with timeout hint
- And the next check proceeds
