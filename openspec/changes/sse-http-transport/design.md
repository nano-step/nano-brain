## Context

nano-brain currently only supports stdio transport for MCP communication. The `--http` and `--port` CLI flags are parsed in `handleMcp()` but never passed to `startServer()` — a bug. The existing HTTP handler (server.ts:1271-1304) uses raw JSON-RPC over POST, which doesn't match the MCP SSE protocol that OpenCode expects.

**Current state:**
- `handleMcp()` parses `--http`, `--port`, `--daemon` flags but ignores them when calling `startServer()`
- `startServer()` accepts `httpPort` in `ServerOptions` but the raw HTTP handler doesn't implement SSE
- OpenCode expects `"type": "sse"` transport with `/sse` (GET) and `/messages` (POST) endpoints

**Constraints:**
- Must use Node.js built-in `http` module (no Express)
- Must not break stdio transport (default)
- Must support multiple concurrent SSE clients sharing the same `ServerDeps`
- SDK already installed: `@modelcontextprotocol/sdk@^1.26.0`

## Goals / Non-Goals

**Goals:**
- Wire up `--http` and `--port` flags to actually enable HTTP mode
- Implement proper SSE transport using `SSEServerTransport` from SDK
- Add Streamable HTTP transport using `StreamableHTTPServerTransport` for newer clients
- Support multiple concurrent clients (each gets own transport, shares `ServerDeps`)
- Add `--host` flag for bind address control (default `127.0.0.1`)
- Preserve `/health` endpoint

**Non-Goals:**
- Adding Express or other HTTP frameworks
- Changing tool registrations in `createMcpServer()`
- Modifying `ServerDeps` interface (except optional new fields)
- Authentication/authorization (future work)

## Decisions

### 1. Use SDK's SSEServerTransport with raw http module

**Decision:** Use `SSEServerTransport` from `@modelcontextprotocol/sdk/server/sse.js` with Node's built-in `http.createServer`.

**Rationale:** The SDK's SSEServerTransport works with raw `http.ServerResponse` — no Express required. This matches the existing pattern in server.ts and avoids new dependencies.

**Alternatives considered:**
- Express middleware: Rejected — adds dependency, overkill for simple routing
- Custom SSE implementation: Rejected — SDK already provides tested implementation

### 2. Create new McpServer per client session

**Decision:** Each SSE/HTTP connection creates a new `McpServer` instance via `createMcpServer(deps)`, sharing the same `ServerDeps`.

**Rationale:** MCP servers are stateful per-connection. Multiple clients need isolated server instances but can share the underlying store, providers, and collections.

**Alternatives considered:**
- Single shared McpServer: Rejected — MCP protocol is connection-scoped
- Separate ServerDeps per client: Rejected — wasteful, defeats purpose of shared daemon

### 3. Dual transport support (SSE + Streamable HTTP)

**Decision:** Support both SSE (`/sse`, `/messages`) and Streamable HTTP (`/mcp`) endpoints.

**Rationale:** SSE is current standard (OpenCode uses it). Streamable HTTP is newer MCP spec for future-proofing. Both can coexist on same server.

**Endpoints:**
- `GET /sse` → Establishes SSE stream, returns endpoint for POST
- `POST /messages?sessionId=X` → Receives client messages for SSE session
- `GET /mcp` → Streamable HTTP SSE stream
- `POST /mcp` → Streamable HTTP messages
- `DELETE /mcp` → Streamable HTTP close session
- `GET /health` → Health check (preserved)

### 4. Session management via Map

**Decision:** Track active sessions in a `Map<string, { transport, server }>` keyed by session ID.

**Rationale:** SSE transport generates session IDs. Need to route POST `/messages` to correct transport. Map provides O(1) lookup.

### 5. Host binding via --host flag

**Decision:** Add `--host` flag defaulting to `127.0.0.1`. Use `0.0.0.0` for remote access.

**Rationale:** Security by default (localhost only). Explicit opt-in for network exposure.

## Risks / Trade-offs

**[Risk] Multiple clients increase memory usage** → Mitigation: Each McpServer is lightweight; heavy resources (store, embedder, reranker) are shared via ServerDeps.

**[Risk] Session cleanup on disconnect** → Mitigation: SSE transport emits close events; clean up Map entries on disconnect.

**[Risk] Breaking stdio transport** → Mitigation: HTTP mode is opt-in via `--http` flag; stdio remains default path.

**[Trade-off] No authentication** → Accepted for v1. Document that `--host 0.0.0.0` exposes server to network without auth.
