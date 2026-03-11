## 1. Process Error Handlers

- [x] 1.1 Add `process.on('uncaughtException')` handler at top of `startServer()` — log error with stack trace, call `process.exit(1)`
- [x] 1.2 Add `process.on('unhandledRejection')` handler with threshold counter — log error, continue; crash after 3 rejections in 60s
- [x] 1.3 Add unit tests for rejection threshold logic (counter increment, window expiry, crash trigger)

## 2. Phased Startup

- [x] 2.1 Refactor `startServer()` to bind HTTP server synchronously in Phase 1 before any async model loading
- [x] 2.2 Add `ready` flag to server state — `/health` returns `{ ready: false }` during Phase 2, `{ ready: true }` after
- [x] 2.3 Add warmup guard to `createMcpServer()` — all tools return `isError: true` with "warming up" message when `ready === false`
- [x] 2.4 Move embedding provider, reranker, file watcher initialization to async Phase 2 with `Promise.all()`
- [x] 2.5 Test: verify HTTP responds within 500ms of process start, before models are loaded

## 3. SQLite EventStore

- [x] 3.1 Create `src/event-store.ts` with `SqliteEventStore` class implementing MCP SDK `EventStore` interface
- [x] 3.2 Add `mcp_events` table creation to store initialization (with `event_id`, `stream_id`, `message`, `created_at` columns + index)
- [x] 3.3 Implement `storeEvent()` — insert event row, return UUID event_id
- [x] 3.4 Implement `replayEventsAfter()` — query events after given ID for the stream, call send() for each
- [x] 3.5 Add event cleanup timer (60s interval) — delete events older than configurable TTL (default 300s)
- [x] 3.6 Add startup cleanup — delete stale events on server start
- [x] 3.7 Wire EventStore into StreamableHTTPServerTransport constructor with `retryInterval: 3000`
- [x] 3.8 Unit tests for SqliteEventStore: store, replay, cleanup, TTL

## 4. HTTP API Endpoints for CLI Proxy

- [x] 4.1 Add `GET /api/status` route — returns server status JSON (index health, model status, workspace info, uptime)
- [x] 4.2 Add `POST /api/query` route — accepts `{ query, tags?, scope?, limit? }`, returns query results
- [x] 4.3 Add `POST /api/search` route — accepts `{ query, limit? }`, returns search results
- [x] 4.4 Integration test: call each API endpoint and verify response format

## 5. CLI HTTP Proxy

- [x] 5.1 Add `detectRunningServer()` helper in `index.ts` — `GET http://localhost:{port}/health` with 1s timeout
- [x] 5.2 Modify `handleStatus()` to proxy via HTTP when server is detected
- [x] 5.3 Modify `handleQuery()` to proxy via HTTP when server is detected
- [x] 5.4 Modify `handleSearch()` to proxy via HTTP when server is detected
- [x] 5.5 Ensure CLI commands never spawn a server process (remove any implicit server start logic)

## 6. Remove Singleton Guard

- [x] 6.1 Delete `setupSingletonGuard()` function from `server.ts`
- [x] 6.2 Delete `writePidFile()` and `removePidFile()` functions
- [x] 6.3 Remove PID file references from `cleanup()` function
- [x] 6.4 Add EADDRINUSE error handler on `httpServer.listen()` — log "nano-brain already running on port {port}", exit(0)
- [x] 6.5 Test: start two server instances, verify second exits cleanly with EADDRINUSE message

## 7. Service Installer

- [x] 7.1 Add `nano-brain serve install` command — detect platform (macOS/Linux)
- [x] 7.2 Implement macOS launchd plist generation with KeepAlive, RunAtLoad, log paths
- [x] 7.3 Implement Linux systemd user service generation with Restart=always, RestartSec=2
- [x] 7.4 Add `nano-brain serve uninstall` command — stop service and remove config file
- [x] 7.5 Add `--force` flag to overwrite existing service file
- [x] 7.6 Test: verify generated plist/service file content is valid

## 8. Graceful Shutdown

- [x] 8.1 Refactor SIGTERM handler to stop accepting new connections, wait up to 5s for in-flight requests
- [x] 8.2 Close all SSE and StreamableHTTP sessions on shutdown
- [x] 8.3 Ensure SQLite database is closed cleanly on exit
