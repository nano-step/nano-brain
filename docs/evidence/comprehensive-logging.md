# Evidence: comprehensive-logging (#144)

OpenSpec change: `openspec/changes/comprehensive-logging/`
Branch: `feat/comprehensive-logging`
Issue: #144

## Summary

Added structured logging across CLI commands, HTTP handlers, and a new
request-scoped middleware. Introduced `-v`/`--verbose` flag (0=info,
1=debug, 2+=trace) and TTY-aware `zerolog.ConsoleWriter` for human-readable
stdout while preserving JSON when piped. Honors `NO_COLOR`.

## Scope of changes

- `internal/health/logger.go` — `trace` level + TTY detection (ConsoleWriter)
- `cmd/nano-brain/main.go` — `cliLog` package logger, `-v`/`--verbose` flags,
  `applyVerbose`, `initCLILog`
- `internal/server/middleware.go` — `requestLoggingMiddleware` with
  `X-Request-ID` propagation, per-request `zerolog.Logger` stored under
  `c.Set("logger", ...)`, start/completion log emissions
- `internal/server/handlers/context.go` — `LoggerFromCtx(c, fallback)` helper
- `internal/server/handlers/*.go` — INFO logs on success returns for
  workspace, document write, reindex, collection add/remove/rename, embed,
  query, bm25, search
- `cmd/nano-brain/{commands,collection,ops,config_cmd,doctor,migrate,daemon,bench}.go`
  — lifecycle start/completion logs; error paths log additionally to user
  `fmt.Fprintln(os.Stderr, ...)` (additive, never replacement)

## Validation

```
$ CGO_ENABLED=0 go build ./...
$ go vet ./...
$ go test -race -short ./...
ok  	github.com/nano-brain/nano-brain/cmd/nano-brain
ok  	github.com/nano-brain/nano-brain/internal/bench
ok  	github.com/nano-brain/nano-brain/internal/chunk
ok  	github.com/nano-brain/nano-brain/internal/config
ok  	github.com/nano-brain/nano-brain/internal/embed
ok  	github.com/nano-brain/nano-brain/internal/harvest
ok  	github.com/nano-brain/nano-brain/internal/health
ok  	github.com/nano-brain/nano-brain/internal/mcp
ok  	github.com/nano-brain/nano-brain/internal/migrate
ok  	github.com/nano-brain/nano-brain/internal/search
ok  	github.com/nano-brain/nano-brain/internal/server
ok  	github.com/nano-brain/nano-brain/internal/server/handlers
ok  	github.com/nano-brain/nano-brain/internal/storage
ok  	github.com/nano-brain/nano-brain/internal/telemetry
ok  	github.com/nano-brain/nano-brain/internal/watcher
```

All packages pass `go build`, `go vet`, and `go test -race -short`.

## Sample log shapes

HTTP request (JSON, piped):

```json
{"level":"debug","request_id":"a1b2c3d4","method":"POST","path":"/api/v1/write","time":"...","message":"request started"}
{"level":"info","workspace":"...","document_id":"...","chunk_count":3,"time":"...","message":"document written"}
{"level":"info","request_id":"a1b2c3d4","method":"POST","path":"/api/v1/write","status":201,"latency_ms":42,"time":"...","message":"request completed"}
```

CLI lifecycle (JSON, piped):

```json
{"level":"info","cmd":"init","time":"...","message":"cli command started"}
{"level":"info","cmd":"init","workspace_hash":"...","time":"...","message":"cli command completed"}
```

When stdout is a TTY, `zerolog.ConsoleWriter` renders the same events as
human-readable colored lines. When `NO_COLOR=1` is set, color is disabled.

## Non-breaking guarantees

- All existing `fmt.Printf`/`fmt.Fprintln(os.Stderr, ...)` calls retained —
  logging is purely additive.
- `cliLog` defaults to `zerolog.Nop()` and is only replaced on successful
  config load + logger construction; CLI flow never panics on a missing
  config.
- No new Go dependencies introduced.
- No changes to `internal/storage`, `internal/embed`, `internal/search`.
- No test assertions on log output (per spec).
