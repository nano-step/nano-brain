# Comprehensive Logging

## Problem

nano-brain has solid logging *infrastructure* (zerolog, multi-writer, lumberjack rotation) but very uneven *coverage*. After running any CLI command, the log file captures nothing useful — no record that the command ran, what it did, or whether it succeeded. On the server side, HTTP requests are invisible (no middleware request logging), and handlers only log errors — so a successful `POST /api/v1/write` leaves no trace.

Specific gaps confirmed by audit:

1. **9 CLI command files have zero logger calls** — they only use `fmt.Printf`. Users debugging via `tail -f ~/.nano-brain/logs/nano-brain.log` see server lifecycle events but nothing from CLI operations.

2. **HTTP request lifecycle is invisible** — no per-request logging (method, path, status, latency, request_id). Impossible to correlate a CLI call with a server-side event.

3. **Handler success paths are unlogged** — 10+ handler files only log errors. A successful `POST /api/v1/init` leaves no server-side record.

4. **Verbose flag missing** — the only way to change log level is editing `~/.nano-brain/config.yml`. No `-v`/`--verbose` flag for interactive debugging.

## Solution

Four targeted additions, shipped in one PR:

1. **`-v`/`--verbose` CLI flag** — overrides `logging.level` from config:
   - default (no flag) → config level (default: info)
   - `-v` → debug
   - `-vv` / `--verbose=2` → trace

2. **HTTP request middleware** — new Echo middleware logging every request start (debug) and completion (info, with method, path, status, latency_ms, request_id).

3. **Handler success-path INFO logs** — add structured INFO log at completion of every mutating handler (write, init workspace, reindex, collection add/remove/rename, embed trigger, harvest).

4. **CLI command lifecycle logs** — add INFO at start and completion of every CLI command that talks to the server. Replace `fmt.Fprintln(os.Stderr, err)` error paths with structured `logger.Error()`.

## Scope

- **In scope**:
  - `internal/health/logger.go` — add ConsoleWriter for TTY + verbose level support
  - `internal/server/middleware.go` — new request-logging middleware
  - `internal/server/routes.go` — register middleware; pass logger
  - `internal/server/handlers/*.go` — add INFO logs on success paths (8 files)
  - `cmd/nano-brain/main.go` — add `-v` / `--verbose` flag; apply before logger init
  - `cmd/nano-brain/*.go` (9 CLI command files) — add lifecycle logs
- **Out of scope**:
  - Distributed tracing (OpenTelemetry)
  - Per-workspace log file isolation
  - Log shipping (Datadog, Loki)
  - Removing all `fmt.Printf` (user-facing table/result output stays as printf; logger is for operational traces only)
  - Test file changes beyond what's needed for handler log assertions

## Risk Classification

- Cross-cutting (touches 21+ files): 1 flag
- New Echo middleware in server chain: 1 flag
- Logger constructor change (verbose level): 1 flag
- CLI flag addition: 1 flag

**Total: 4 flags — upper edge lane:normal.** No DB schema change, no public API contract break, no auth/security code. Middleware is additive, no existing route changes. Treating as lane:normal with careful middleware ordering documented.

## References

- Issue #144
- Logger: `internal/health/logger.go` (NewLogger)
- Middleware: `internal/server/middleware.go` (registerMiddleware)
- CLI flag parsing: `cmd/nano-brain/main.go` lines 25-38 (flag.Parse)
- Audit results: ~127 current log calls; ~140 new log points to add
