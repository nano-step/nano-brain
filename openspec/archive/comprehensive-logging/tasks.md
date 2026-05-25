# Tasks: Comprehensive Logging

## Phase 1 — Logger + verbose flag foundation

- [x] **1.1** Add `"trace"` case to `parseLogLevel` in `internal/health/logger.go`.
- [x] **1.2** Add TTY-aware ConsoleWriter in `NewLogger`: check `os.Stdout.Stat().Mode() & os.ModeCharDevice != 0`; if TTY, use `zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339, NoColor: os.Getenv("NO_COLOR") != ""}` instead of bare `os.Stdout` in the MultiWriter.
- [x] **1.3** Add verbose flag in `cmd/nano-brain/main.go`: `flag.IntVar(&verbose, "v", 0, "...")` + `flag.IntVar(&verbose, "verbose", 0, "")`. Apply after config load, before `NewLogger`.
- [x] **1.4** Initialize `cliLog` package-level logger in `main()` for commands that never enter `startServer`. Pattern: right after verbose → level mapping, call `NewLogger(cfg.Logging)` and store in `var cliLog zerolog.Logger`. Commands call `cliLog.Info()` etc.

  Validation: `CGO_ENABLED=0 go build ./... && go test -race -short ./...`

## Phase 2 — HTTP request middleware

- [x] **2.1** Add `requestLoggingMiddleware(logger zerolog.Logger) echo.MiddlewareFunc` to `internal/server/middleware.go` (see design.md for full implementation including `generateShortID`, request_id propagation, start/completion logs).
- [x] **2.2** Add `logger zerolog.Logger` field to `Server` struct in `internal/server/server.go`; update `NewServer` to accept it.
- [x] **2.3** Wire logger through: find where `NewServer` is called in `cmd/nano-brain/main.go`; pass `logger`.
- [x] **2.4** Register middleware in `registerMiddleware(s *Server)` as FIRST middleware: `s.echo.Use(requestLoggingMiddleware(s.logger))`.
- [x] **2.5** Add `LoggerFromCtx(c echo.Context, fallback zerolog.Logger) zerolog.Logger` helper (either in middleware.go or new `internal/server/handlers/context.go`).

  Validation: `CGO_ENABLED=0 go build ./... && go test -race -short ./...`

## Phase 3 — Handler success-path logs

For each file, add ONE `logger.Info()` at the success return. Use `LoggerFromCtx(c, h.logger)` to get the per-request logger.

- [x] **3.1** `handlers/workspace.go` — `InitWorkspace` success: log `workspace_hash`, `root_path`.
- [x] **3.2** `handlers/document.go` — write success: log `document_id`, `workspace`; delete success: log `document_id`.
- [x] **3.3** `handlers/reindex.go` — queued (202): log `workspace`.
- [x] **3.4** `handlers/collection.go` — add: log `name`, `workspace`; remove: log `name`; rename: log `from`, `to`.
- [x] **3.5** `handlers/embed.go` — trigger success: log `workspace`.
- [x] **3.6** `handlers/query.go` — complete: log `workspace`, `results` count, `latency_ms`.
- [x] **3.7** `handlers/bm25.go` — complete: log `workspace`, `results` count.
- [x] **3.8** `handlers/search.go` — complete: log `workspace`, `results` count.

  Validation after all 3.x: `CGO_ENABLED=0 go build ./... && go test -race -short ./internal/server/...`

## Phase 4 — CLI command lifecycle logs

For each file, add INFO at start + completion. Keep existing `fmt.Printf`/`fmt.Fprintln` for user output. Use `cliLog` (package-level logger from Phase 1).

- [x] **4.1** `cmd/nano-brain/commands.go` — `init --root`, `write`, `query`, `search`, `vsearch`, `harvest`: start + completion logs.
- [x] **4.2** `cmd/nano-brain/collection.go` — `add`, `remove`, `list`, `rename`: start + completion logs.
- [x] **4.3** `cmd/nano-brain/ops.go` — `docker start/stop/status`, `logs`, `status` command: start + completion.
- [x] **4.4** `cmd/nano-brain/config_cmd.go` — `config show`, `config check`: debug log (not info, these are read-only).
- [x] **4.5** `cmd/nano-brain/doctor.go` — start + completion at debug level.
- [x] **4.6** `cmd/nano-brain/migrate.go` — start + completion logs.
- [x] **4.7** `cmd/nano-brain/daemon.go` — PID file write/remove, daemon fork: debug logs.
- [x] **4.8** `cmd/nano-brain/bench.go` — generate/run/compare/stress: start + completion logs.

  Validation after all 4.x: `CGO_ENABLED=0 go build ./... && go vet ./... && go test -race -short ./...`

## Phase 5 — Full validation

- [x] **5.1** `CGO_ENABLED=0 go build ./...`
- [x] **5.2** `go vet ./...`
- [x] **5.3** `go test -race -short ./...` — all packages pass
- [x] **5.4** Grep check: `grep -rn "fmt.Fprintf(os.Stderr" cmd/nano-brain/*.go | wc -l` — verify count decreased (error paths replaced with dual print+log).

## Phase 6 — Evidence + mark tasks

- [x] **6.1** Write `docs/evidence/comprehensive-logging.md` with sample log output after running a few commands (or document unit test coverage if live server not available).
- [x] **6.2** Mark all `[ ]` → `[x]`.

## Phase 7 — PR (orchestrator)

- [x] **7.1** Push branch, open PR linking issue #144.
