## ADDED Requirements

### Requirement: Process error handlers
The server SHALL register `uncaughtException` and `unhandledRejection` handlers at startup before any other initialization.

`uncaughtException` SHALL log the error with stack trace and call `process.exit(1)`.

`unhandledRejection` SHALL log the error and continue. If 3 or more unhandled rejections occur within 60 seconds, the process SHALL exit with code 1.

#### Scenario: Uncaught exception triggers clean exit
- **WHEN** an uncaught exception occurs during server operation
- **THEN** the error is logged with full stack trace AND the process exits with code 1

#### Scenario: Single unhandled rejection continues
- **WHEN** a single unhandled promise rejection occurs
- **THEN** the error is logged AND the process continues running

#### Scenario: Repeated rejections trigger crash
- **WHEN** 3 unhandled promise rejections occur within 60 seconds
- **THEN** the process logs "rejection threshold exceeded" AND exits with code 1

### Requirement: EADDRINUSE guard replaces singleton guard
The server SHALL NOT use PID-file-based singleton detection. The `setupSingletonGuard()` function and all PID file logic SHALL be removed.

If `httpServer.listen()` fails with EADDRINUSE, the server SHALL log "nano-brain already running on port {port}" and exit with code 0.

#### Scenario: Port available
- **WHEN** the server starts and port 3100 is available
- **THEN** the server binds successfully and begins accepting connections

#### Scenario: Port already in use
- **WHEN** the server starts and port 3100 is already bound by another nano-brain instance
- **THEN** the server logs "nano-brain already running on port 3100" AND exits with code 0

#### Scenario: CLI commands do not kill server
- **WHEN** a CLI command like `nano-brain status` is executed while the server is running
- **THEN** the running server process is NOT affected (no SIGTERM, no PID file overwrite)

### Requirement: Phased startup
The server SHALL start in two phases:
- Phase 1 (synchronous, <500ms): Create HTTP server, bind port, register all routes, serve `/health` with `{ status: "ok", ready: false }`
- Phase 2 (asynchronous): Load embedding provider, reranker, start file watcher, index codebase. Update `/health` to `{ ready: true }` when complete.

MCP tools called during Phase 2 SHALL return `{ isError: true, text: "Server warming up, try again in a few seconds" }`.

#### Scenario: HTTP available before models load
- **WHEN** the server starts
- **THEN** the HTTP server accepts connections within 500ms AND `/health` returns `{ status: "ok", ready: false }`

#### Scenario: Tools during warmup
- **WHEN** a client calls an MCP tool before Phase 2 completes
- **THEN** the tool returns `isError: true` with message "Server warming up, try again in a few seconds"

#### Scenario: Full readiness
- **WHEN** Phase 2 completes (embedding provider + watcher loaded)
- **THEN** `/health` returns `{ status: "ok", ready: true }` AND all MCP tools function normally

### Requirement: Graceful shutdown
On SIGTERM, the server SHALL:
1. Stop accepting new connections
2. Wait up to 5 seconds for in-flight requests to complete
3. Close all SSE/StreamableHTTP sessions
4. Close the SQLite database
5. Exit with code 0

#### Scenario: Clean shutdown with in-flight requests
- **WHEN** SIGTERM is received while requests are being processed
- **THEN** the server waits up to 5 seconds for completion AND then exits cleanly
