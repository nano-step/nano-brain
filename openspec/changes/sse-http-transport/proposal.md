## Why

nano-brain currently only runs as a stdio MCP server, meaning it must be spawned as a child process inside the same container as the AI coding agent. When running OpenCode in Docker via ai-sandbox-wrapper, nano-brain (with SQLite, Qdrant vector DB, embedding models, and reranker) consumes ~300MB+ RAM inside the container alongside other MCP servers (Playwright, graphql-tools, etc.), causing severe memory/CPU bloat. Adding SSE/HTTP transport allows nano-brain to run as a persistent daemon on the host or a separate server, with containers connecting remotely — reducing per-container overhead to near zero and enabling shared memory across multiple agents.

## What Changes

- **Wire up existing `--http` and `--port` CLI flags** that are already parsed in `handleMcp()` but never passed to `startServer()` — a bug fix
- **Replace the raw HTTP/JSON-RPC handler** (server.ts lines 1271-1304) with proper `SSEServerTransport` from `@modelcontextprotocol/sdk` so OpenCode can connect via `"type": "sse"`
- **Add Streamable HTTP transport** support using `StreamableHTTPServerTransport` for future-proofing (newer MCP spec)
- **Support multiple concurrent SSE clients** connecting to the same nano-brain instance
- **Add `/health` endpoint** for monitoring (already exists in current raw HTTP handler, preserve it)
- **Add `--host` flag** to control bind address (default `127.0.0.1`, option for `0.0.0.0` for remote access)
- **Update CLI help text** to document the SSE/HTTP transport mode

## Capabilities

### New Capabilities
- `sse-transport`: SSE server transport enabling remote MCP connections from OpenCode, Claude Desktop (via mcp-proxy), and other MCP clients
- `streamable-http-transport`: Streamable HTTP transport (newer MCP spec) for clients that support it

### Modified Capabilities

## Impact

- **Files modified**: `src/server.ts` (transport initialization), `src/index.ts` (CLI flag wiring)
- **Dependencies**: No new dependencies — `@modelcontextprotocol/sdk@^1.26.0` already includes `SSEServerTransport` and `StreamableHTTPServerTransport`
- **APIs**: New HTTP endpoints for SSE (`/sse`, `/messages`) and Streamable HTTP (`/mcp`)
- **Backward compatible**: stdio remains the default transport; `--http` opt-in for SSE mode
- **Deployment**: Enables `npx nano-brain mcp --http --port 3100` on host, with OpenCode connecting via `"type": "sse", "url": "http://host.docker.internal:3100"`
