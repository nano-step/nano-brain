# Coding Conventions

> Observed patterns from the nano-brain codebase. Last reviewed: 2026-06-28.

## Language & Build

- **Go 1.23**, `CGO_ENABLED=0` static binary, no external DI framework
- Constructor injection throughout — config, logger, `*pgxpool.Pool` passed at construction
- `sqlc` for type-safe SQL; generated files in `internal/storage/sqlc/` are **DO NOT EDIT**
- Migrations via `goose v3`, embedded in `migrations/FS`

## Package Layout

| Convention | Example |
|---|---|
| One package per domain concern | `internal/search`, `internal/embed`, `internal/harvest` |
| `cmd/nano-brain/` — CLI dispatcher + server entry | `main.go`, `commands.go`, `daemon.go` |
| `internal/server/handlers/` — one file per endpoint group | `query.go`, `health.go`, `graph.go` |
| `internal/testutil/` — shared test helpers | `testdb.go`, `testutil.go` |

## Naming

- **Files:** `snake_case.go`; test files `snake_case_test.go`
- **Packages:** short, lowercase, single-word (`search`, `embed`, `graph`, `symbol`)
- **Exported types:** PascalCase (`HybridSearcher`, `Embedder`, `Harvester`)
- **Unexported functions:** camelCase (`maskPassword`, `expandTildeForConfig`)
- **Interfaces:** small, role-based, named after the capability (`HybridSearcher`, `PoolChecker`, `EmbedQueueInfo`). Defined on the consumer side, not the provider
- **Config structs:** `XxxConfig` with `koanf:"snake_case" json:"snake_case"` tags (both tags always present)

## Error Handling

```go
// Standard pattern — wrap with context, never bare errors.New in callers
return nil, fmt.Errorf("failed to create pool: %w", err)

// Errors are always wrapped; custom error types are not used
// Callers check with errors.Is()
if errors.Is(err, sql.ErrNoRows) { ... }
```

- No custom error types anywhere in the codebase
- No `_ = err` on constructor calls in startup paths — use `log.Warn` (optional) or `log.Fatal` (critical)
- Errors include context: `fmt.Errorf("parse config: %w", err)`

## Logging

- **zerolog** structured JSON logging throughout
- Scope per component: `.With().Str("component", "x").Logger()`
- Per-request logger via middleware, retrieved with `LoggerFromCtx(c, fallback)`
- CLI logs to stderr (not stdout) to avoid polluting JSON output

## Configuration

- **koanf** YAML + env vars; config path precedence: `--config` flag > `NANO_BRAIN_CONFIG` env > `~/.nano-brain/config.yml`
- Dynamic reload via `RWMutex`; hot-reload via `POST /api/reload-config`
- Env var mapping: `NANO_BRAIN_<SECTION>_<FIELD>` → `section.field` (first underscore becomes dot)
- Special non-prefixed env vars: `DATABASE_URL`, `VOYAGE_API_KEY`, `OPENCODE_STORAGE_DIR`, etc.
- Config validation accumulates errors into `[]error`, returns combined message
- Defaults defined in `internal/config/defaults.go`

## Context

- `ctx context.Context` is always the first parameter on all I/O functions
- `errgroup` (`golang.org/x/sync/errgroup`) for goroutine lifecycle management
- Signal handling via `signal.NotifyContext` for graceful shutdown

## HTTP Handlers (Echo v4)

- Constructor function returns `echo.HandlerFunc`; dependencies passed as arguments:
  ```go
  func Query(searcher HybridSearcher, logger zerolog.Logger) echo.HandlerFunc {
      return func(c echo.Context) error { ... }
  }
  ```
- Each handler file defines a small interface for its dependency (keeps it testable)
- Request binding: `c.Bind(&req)` → validate → `c.JSON(http.StatusOK, resp)`
- Errors: `echo.NewHTTPError(statusCode, "message")` — never `c.JSON(4xx, ...)`
- Workspace extracted from context: `c.Get("workspace").(string)`, injected by middleware

## Database

- Connection: `storage.NewPool(ctx, cfg, logger)` → `*pgxpool.Pool` (MaxConns=10, HealthCheckPeriod=30s)
- Queries: `sqlc.New(db)` wraps pool; generated methods on `*sqlc.Queries`
- DSN passwords masked in logs: `maskPassword(dsn)`
- Error redaction: `storage.RedactError(err)` / `storage.RedactString(s)` for CLI output
- Workspace hash: SHA-256 of absolute root path → hex string

## Struct Tag Convention

Config structs always carry both tags:
```go
Field string `koanf:"field_name" json:"field_name"`
```
JSON output uses snake_case exclusively (no PascalCase keys).

## Builder / Fluent Patterns

Used for optional configuration chains:
```go
fw := watcher.New(db, queries, logger, *cfg).
    WithSymbolRegistry(symRegistry).
    WithGraphRegistry(graphRegistry, queries).
    WithFrameworkDetector(frameworkDetector).
    WithDispatcher(dispatcher)
```

## File Organization

- One concern per file; small files preferred
- Package doc comment on the first line of `package.go` files
- `AGENTS.md` in key directories documents package-level conventions for AI agents
- No comments unless explaining non-obvious behavior (AGENTS.md convention)

## Forbidden Practices

- No `_ = err` on constructors in startup paths
- No bare `errors.New` in callers (always wrap with `fmt.Errorf`)
- No `git push origin <branch>` without verifying current branch
- No direct commits to `master`
- No self-review
- No real workspace names, paths, or hashes in committed files
