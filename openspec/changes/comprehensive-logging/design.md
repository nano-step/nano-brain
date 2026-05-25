# Design: Comprehensive Logging

## 1. Verbose Flag

### Flag definition (main.go)

```go
var verbose int
flag.IntVar(&verbose, "v", 0, "verbosity: 0=info, 1=debug, 2=trace")
```

Also accept `--verbose` as synonym by registering `flag.IntVar(&verbose, "verbose", 0, "")`.

Mapping:
- 0 → use `cfg.Logging.Level` (default "info")
- 1 → `zerolog.DebugLevel`
- 2 → `zerolog.TraceLevel`

Apply AFTER config load, BEFORE `NewLogger` call:
```go
if verbose > 0 {
    switch verbose {
    case 1: cfg.Logging.Level = "debug"
    case 2: cfg.Logging.Level = "trace"
    default: cfg.Logging.Level = "trace"
    }
}
```

Also add `"trace"` case to `parseLogLevel` in `internal/health/logger.go` (currently missing — trace is a valid zerolog level).

### TTY-aware ConsoleWriter

Modify `NewLogger` in `internal/health/logger.go`:
- If `os.Stdout` is a TTY (`stdout.Stat().Mode() & os.ModeCharDevice != 0`) → use `zerolog.ConsoleWriter` for the stdout part (human-readable, colored).
- File writer always remains JSON.
- Combined with `io.MultiWriter` as today.

This gives interactive users colored output in the terminal without breaking JSON log files for tooling.

## 2. HTTP Request Middleware

Add to `internal/server/middleware.go`:

```go
func requestLoggingMiddleware(logger zerolog.Logger) echo.MiddlewareFunc {
    return func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()
            reqID := c.Request().Header.Get("X-Request-ID")
            if reqID == "" {
                reqID = generateShortID() // 8-char hex, stdlib crypto/rand
            }
            c.Request().Header.Set("X-Request-ID", reqID)
            c.Response().Header().Set("X-Request-ID", reqID)

            // Store scoped logger in context
            reqLogger := logger.With().
                Str("request_id", reqID).
                Str("method", c.Request().Method).
                Str("path", c.Request().URL.Path).
                Logger()
            c.Set("logger", reqLogger)

            reqLogger.Debug().Msg("request started")

            err := next(c)

            latency := time.Since(start)
            status := c.Response().Status
            evt := reqLogger.Info()
            if err != nil || status >= 500 {
                evt = reqLogger.Error().Err(err)
            }
            evt.Int("status", status).
                Int64("latency_ms", latency.Milliseconds()).
                Msg("request completed")

            return err
        }
    }
}

func generateShortID() string {
    b := make([]byte, 4)
    _, _ = rand.Read(b)
    return hex.EncodeToString(b)
}
```

Register in `registerMiddleware(s *Server)` BEFORE `versionHeaderMiddleware` and workspace middleware:
```go
s.echo.Use(requestLoggingMiddleware(s.logger))   // first
s.echo.Use(versionHeaderMiddleware(s.version))
// workspace middleware registered per-group in routes.go
```

The `Server` struct must have a `logger zerolog.Logger` field — check `internal/server/server.go` for current structure; add it if not present, wire it through `NewServer`.

### Handler logger extraction helper

Add to `middleware.go` or a new `internal/server/handlers/context.go`:
```go
func LoggerFromCtx(c echo.Context, fallback zerolog.Logger) zerolog.Logger {
    if l, ok := c.Get("logger").(zerolog.Logger); ok {
        return l
    }
    return fallback
}
```

## 3. Handler Success-Path Logs

For each mutating handler, add ONE `logger.Info()` on the happy-path return. Use `LoggerFromCtx` to get the per-request logger with request_id attached.

| Handler file | Function | New log |
|---|---|---|
| `handlers/workspace.go` | `InitWorkspace` success | `logger.Info().Str("workspace_hash", ws.Hash).Str("root_path", absPath).Msg("workspace registered")` |
| `handlers/document.go` | write/upsert success | `logger.Info().Str("document_id", id).Str("workspace", ws).Msg("document written")` |
| `handlers/document.go` | delete success | `logger.Info().Str("document_id", id).Msg("document deleted")` |
| `handlers/reindex.go` | reindex queued | `logger.Info().Str("workspace", ws).Msg("reindex queued")` |
| `handlers/collection.go` | add success | `logger.Info().Str("workspace", ws).Str("name", name).Msg("collection added")` |
| `handlers/collection.go` | remove success | `logger.Info().Str("workspace", ws).Str("name", name).Msg("collection removed")` |
| `handlers/collection.go` | rename success | `logger.Info().Str("from", old).Str("to", newName).Msg("collection renamed")` |
| `handlers/embed.go` | embed triggered | `logger.Info().Str("workspace", ws).Msg("embed triggered")` |
| `handlers/query.go` | search complete | `logger.Info().Str("workspace", ws).Int("results", n).Int64("latency_ms", ms).Msg("hybrid search complete")` |
| `handlers/bm25.go` | search complete | `logger.Info().Str("workspace", ws).Int("results", n).Msg("bm25 search complete")` |
| `handlers/search.go` | vsearch complete | `logger.Info().Str("workspace", ws).Int("results", n).Msg("vector search complete")` |

Handlers that already have the logger passed as a closure param can use it directly. Handlers where the logger isn't present: use `LoggerFromCtx(c, zerolog.Nop())` — `zerolog.Nop()` is a zero-cost no-op if context doesn't have a logger (safe fallback).

## 4. CLI Command Logs

CLI commands currently use `fmt.Printf` for results and `fmt.Fprintln(os.Stderr, err)` for errors. We add lifecycle logger calls WITHOUT removing the user-facing printf output — `fmt.Printf` is for the user (results, tables), logger is for the operational record.

The CLI currently has no `logger` in scope in command functions. Options:
- (a) Build a CLI-level logger (always writes to the log file, stdout only if TTY + verbose)
- (b) Use `zerolog.New(logFileWriter)` (file-only, no TTY check needed)

**Decision: Simple CLI logger** — a package-level `var cliLog zerolog.Logger` initialized in `main.go` after `NewLogger` call. Pass it as the server logger or derive from it. Use `cliLog.Info()` / `cliLog.Error()` in command functions.

Implementation: after `logger, err := health.NewLogger(cfg.Logging)` in `startServer`, also set a package-level `var cliLog zerolog.Logger = logger`. For CLI commands that don't call `startServer` (they run before the server starts), create a minimal file-only logger inline in `main()` before dispatching commands.

Log points per command file:
- `init --root`: `cliLog.Info().Str("root_path", root).Msg("registering workspace")` + `cliLog.Info().Str("workspace_hash", hash).Msg("workspace registered")`
- `write`: `cliLog.Info().Str("workspace", ws).Msg("writing document")` + `cliLog.Info().Str("document_id", id).Msg("document written")`
- `query/search/vsearch`: `cliLog.Info().Str("query", q).Msg("executing search")` + `cliLog.Info().Int("results", n).Msg("search complete")`
- `harvest`: `cliLog.Info().Msg("triggering harvest")` + `cliLog.Info().Int("harvested", n).Msg("harvest complete")`
- `collection add/remove/list/rename`: `cliLog.Info().Str("name", name).Msg("collection <action>")`
- `ops.go` (docker, logs, daemon): `cliLog.Info().Str("action", action).Msg("docker command")` etc.
- `config_cmd.go`: `cliLog.Debug().Str("config_path", p).Msg("config show")`
- `doctor.go`: `cliLog.Debug().Msg("running doctor checks")`

Error paths: replace `fmt.Fprintln(os.Stderr, "Error: "+msg)` with BOTH `fmt.Fprintln(os.Stderr, ...)` (keep for user) AND `cliLog.Error().Str("cmd", cmd).Err(err).Msg("command failed")`.

## Files Changed (summary)

| File | Change type | ~LOC added |
|---|---|---|
| `internal/health/logger.go` | Modify (TTY ConsoleWriter, trace level) | +20 |
| `internal/server/middleware.go` | Modify (add requestLoggingMiddleware, generateShortID) | +50 |
| `internal/server/server.go` | Modify (add logger field to Server) | +5 |
| `internal/server/routes.go` | Modify (register middleware, wire logger) | +5 |
| `internal/server/handlers/workspace.go` | Modify (1 info log) | +3 |
| `internal/server/handlers/document.go` | Modify (2 info logs) | +6 |
| `internal/server/handlers/reindex.go` | Modify (1 info log) | +3 |
| `internal/server/handlers/collection.go` | Modify (3 info logs) | +9 |
| `internal/server/handlers/embed.go` | Modify (1 info log) | +3 |
| `internal/server/handlers/query.go` | Modify (1 info log) | +5 |
| `internal/server/handlers/bm25.go` | Modify (1 info log) | +5 |
| `internal/server/handlers/search.go` | Modify (1 info log) | +5 |
| `cmd/nano-brain/main.go` | Modify (verbose flag, cliLog init) | +20 |
| `cmd/nano-brain/commands.go` | Modify (~6 log points) | +18 |
| `cmd/nano-brain/collection.go` | Modify (~6 log points) | +18 |
| `cmd/nano-brain/ops.go` | Modify (~8 log points) | +24 |
| `cmd/nano-brain/config_cmd.go` | Modify (~2 log points) | +6 |
| `cmd/nano-brain/doctor.go` | Modify (~2 log points) | +6 |
| `cmd/nano-brain/migrate.go` | Modify (~3 log points) | +9 |
| `cmd/nano-brain/daemon.go` | Modify (~2 log points) | +6 |
| `cmd/nano-brain/bench.go` | Modify (~2 log points) | +6 |

**Total: ~21 files, ~232 LOC added**

## Key Decisions

### No test changes for log assertions

Adding assertions on log output to existing tests would be brittle (exact message strings, log level checks). The added logs are operational traces, not behavioral contracts. Tests verify behavior; logs are observable side-effects. Existing tests must still pass (no regression), but we do NOT add new tests that assert log lines.

### `zerolog.Nop()` as fallback

When a handler can't extract the per-request logger (e.g. non-HTTP call paths in tests), `zerolog.Nop()` is a zero-allocation no-op. Safe to use everywhere.

### Middleware ordering

1. `requestLoggingMiddleware` (outermost — captures all requests)
2. `versionHeaderMiddleware`
3. `workspaceMiddleware` (per-group — after request ID is set so handlers can log workspace + request_id together)

### CLI logger initialization for command-only paths

Commands like `nano-brain query` never call `startServer`. They need a logger too. Solution: in `main()`, after `config.Load`, always initialize a minimal `cliLog` (file-only, respects verbose flag). `startServer` then uses the same logger (already set). This means `cliLog` is set before command dispatch.

### TTY detection reuse

If proposal #1 (connect-error-ux) has shipped and `isTTY()` exists in `client.go`, reuse it for the ConsoleWriter decision in `logger.go`. If not (independent branch), inline the same `os.Stdout.Stat().Mode() & os.ModeCharDevice` check.
