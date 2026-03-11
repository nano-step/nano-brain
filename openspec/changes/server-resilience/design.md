## Context

nano-brain runs as a shared HTTP daemon (`nano-brain serve`) on the host, serving 20+ MCP clients from Docker containers via SSE and StreamableHTTP on port 3100. The current `startServer()` function in `server.ts` (2095 lines) is monolithic — it bundles the HTTP server, file watcher, embedding cycle, session harvester, codebase indexer, singleton guard, and Ollama reconnect timer in one process sharing a single SQLite database.

Current failure modes:
1. No `uncaughtException`/`unhandledRejection` handlers → silent crash
2. `setupSingletonGuard()` kills running server when CLI commands spawn a new instance
3. `child.unref()` detached spawn with no monitoring → stays dead after crash
4. SSE sessions stored in-memory `Map` → lost on restart, clients permanently disconnected

## Goals / Non-Goals

**Goals:**
- Server survives transient errors without crashing
- Server auto-restarts within 2 seconds after fatal crash
- Clients auto-reconnect after server restart with zero manual intervention
- CLI commands never accidentally kill the running server
- Cross-platform service installation (macOS + Linux)

**Non-Goals:**
- Splitting into separate server/worker processes (SQLite single-writer constraint)
- Worker threads for tree-sitter/embedding (defer unless blocking is measured)
- Windows support for service installer
- Changing the storage layer from SQLite
- Client-side SDK changes (we don't control OpenCode/Amp)

## Decisions

### D1: Error handling strategy
**Choice**: `uncaughtException` → log + exit(1), `unhandledRejection` → log + continue with threshold (3 in 60s → crash)

**Why**: uncaughtException leaves Node in undefined state — must exit. unhandledRejection is usually recoverable (network timeouts, Ollama down), but repeated rejections signal corruption. Threshold balances resilience vs safety.

**Alternative considered**: Crash on every unhandledRejection — too aggressive, would crash on every Ollama timeout.

### D2: Replace singleton guard with EADDRINUSE
**Choice**: Delete `setupSingletonGuard()` entirely. If `httpServer.listen()` gets EADDRINUSE, log "nano-brain already running on port {port}" and exit(0).

**Why**: The singleton guard is the #1 crash cause. EADDRINUSE is a natural, race-free guard — the OS guarantees only one process can bind a port. launchd/systemd prevents duplicate launches.

**Alternative considered**: Keep guard but make it graceful (SIGTERM + wait). Too complex, same result as EADDRINUSE.

### D3: Phased startup for <500ms HTTP ready
**Choice**: Split `startServer()` into two phases:
- Phase 1 (sync, <500ms): Create HTTP server, bind port, register routes, serve `/health` with `ready: false`
- Phase 2 (async, background): Load embedding provider, reranker, start file watcher, index codebase

MCP tools called during Phase 2 return: `{ isError: true, text: "Server warming up, try again in a few seconds" }`

**Why**: Clients retry within 3 seconds. If HTTP is up, the retry succeeds and creates a new session. Models loading async means the server is "connectable" immediately even if not fully functional.

### D4: SQLite-backed EventStore for StreamableHTTP
**Choice**: Implement MCP SDK's `EventStore` interface using a new `mcp_events` table in the existing SQLite database. Only for `/mcp` endpoint (StreamableHTTP) — SSE `/sse` doesn't support EventStore per the SDK.

Schema:
```sql
CREATE TABLE mcp_events (
  event_id TEXT PRIMARY KEY,
  stream_id TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_mcp_events_stream ON mcp_events(stream_id, event_id);
```

Configure `retryInterval: 3000` on StreamableHTTP transport so clients wait 3s before reconnecting.

Cleanup: delete events older than 5 minutes on a 60-second timer + on startup.

**Why**: MCP messages are infrequent (tool calls, not streaming). SQLite with WAL handles this easily. In-memory EventStore defeats the purpose — it's lost on crash. SQLite survives restart.

**Alternative considered**: In-memory with periodic flush — defeats crash recovery purpose. Redis — adds external dependency.

### D5: CLI → HTTP proxy
**Choice**: CLI commands (`status`, `query`, `search`, `focus`, `write`) detect running server via `GET http://localhost:{port}/health`. If reachable, proxy the command via new REST API routes. If not reachable, print "Server not running. Start with: nano-brain serve" (or launchctl/systemctl command if installed).

New HTTP API routes on the server:
- `GET /api/status` — returns status JSON
- `POST /api/query` — proxies query command
- `POST /api/search` — proxies search command

**Why**: Eliminates the "CLI spawns new server instance" footgun. CLI becomes a thin HTTP client.

### D6: Platform-specific service installer
**Choice**: `nano-brain serve install` detects platform and generates:
- macOS: `~/Library/LaunchAgents/com.nano-brain.server.plist` with `KeepAlive: true`
- Linux: `~/.config/systemd/user/nano-brain.service` with `Restart=always`

`nano-brain serve uninstall` removes the service file and stops the service.

**Why**: Native service managers are more reliable than pm2 (no Node dependency to supervise Node). User-level services don't require root.

## Risks / Trade-offs

- **[SQLite EventStore write latency]** → Mitigated by WAL mode + infrequent MCP messages. Batch writes if needed.
- **[Warming up errors during Phase 2]** → Acceptable. Clients retry automatically. Tools return clear error message.
- **[EADDRINUSE race on fast restart]** → Mitigated by SO_REUSEADDR on the HTTP server + 100ms delay before bind.
- **[CLI HTTP proxy adds latency]** → Negligible for interactive CLI commands (~5ms local HTTP).
- **[EventStore only works for /mcp, not /sse]** → SSE is legacy. Document recommendation to switch clients to /mcp.
- **[5-minute event TTL may be too short]** → Configurable via config.yaml. Default 5min covers restart scenarios.
