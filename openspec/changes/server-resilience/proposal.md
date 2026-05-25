## Why

nano-brain runs as a shared daemon on the host machine serving 20+ MCP clients from Docker containers. It crashes silently (no error handlers), the singleton guard kills it when CLI commands run, there's no auto-restart, and clients can't reconnect after a restart because SSE sessions are lost and the MCP SDK retry window is too short (~3s). This makes the entire AI toolchain fragile — one crash takes down code intelligence for all agents.

## What Changes

- **Add process error handlers** — `uncaughtException` logs and exits cleanly (let supervisor restart), `unhandledRejection` logs and continues with crash-after-threshold (3 in 60s)
- **Phased startup** — HTTP server binds port in <500ms (Phase 1), models/watcher load async (Phase 2). Tools return "warming up" if called before ready.
- **SQLite-backed EventStore** — implement MCP SDK's `EventStore` interface for StreamableHTTP resumability. Clients auto-reconnect with `Last-Event-ID` and replay missed events. Configure `retryInterval: 3000`.
- **HTTP API endpoints for CLI proxy** — new REST routes (`/api/status`, `/api/query`, etc.) so CLI commands proxy to running server instead of spawning new instances
- **Remove singleton guard** — replace PID-file-based guard with EADDRINUSE detection. CLI commands use HTTP proxy, never start a new server process.
- **Service installer** — `nano-brain serve install` generates platform-specific service config (launchd on macOS, systemd on Linux) for auto-restart

## Capabilities

### New Capabilities
- `server-resilience`: Process error handling, phased startup, EADDRINUSE guard, graceful shutdown with in-flight request tracking
- `event-store`: SQLite-backed MCP EventStore for StreamableHTTP message resumability after client disconnection
- `cli-http-proxy`: CLI commands detect running server and proxy via HTTP API instead of spawning new instances
- `service-installer`: Platform-specific service file generation for auto-restart (launchd/systemd)

### Modified Capabilities
- `mcp-server`: StreamableHTTP transport gains EventStore + retryInterval configuration

## Impact

- **Files modified**: `src/server.ts` (error handlers, phased startup, EventStore, HTTP API routes, remove singleton guard), `src/index.ts` (CLI proxy logic, service installer commands)
- **New files**: `src/event-store.ts` (SQLite EventStore implementation)
- **Dependencies**: No new dependencies — uses existing `@modelcontextprotocol/sdk`, `better-sqlite3`
- **APIs**: New HTTP routes `/api/status`, `/api/query`, `/api/search`; StreamableHTTP `/mcp` gains resumability
- **Backward compatible**: SSE `/sse` endpoint unchanged; stdio transport unchanged; new features are additive
