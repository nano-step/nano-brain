## 1. CLI Flag Wiring (Bug Fix)

- [ ] 1.1 Add `--host` flag parsing in `handleMcp()` (src/index.ts)
- [ ] 1.2 Pass `httpPort`, `httpHost` to `startServer()` in `handleMcp()`
- [ ] 1.3 Add `httpHost` to `ServerOptions` interface (src/server.ts)

## 2. SSE Transport Implementation

- [ ] 2.1 Import `SSEServerTransport` from `@modelcontextprotocol/sdk/server/sse.js`
- [ ] 2.2 Create session management Map for tracking active SSE connections
- [ ] 2.3 Implement `GET /sse` endpoint that creates SSEServerTransport and new McpServer
- [ ] 2.4 Implement `POST /messages` endpoint that routes to correct session transport
- [ ] 2.5 Add session cleanup on SSE disconnect

## 3. Streamable HTTP Transport Implementation

- [ ] 3.1 Import `StreamableHTTPServerTransport` from `@modelcontextprotocol/sdk/server/streamableHttp.js`
- [ ] 3.2 Create session management for Streamable HTTP connections
- [ ] 3.3 Implement `GET /mcp` endpoint for Streamable HTTP SSE stream
- [ ] 3.4 Implement `POST /mcp` endpoint for Streamable HTTP messages
- [ ] 3.5 Implement `DELETE /mcp` endpoint for session close

## 4. HTTP Server Refactor

- [ ] 4.1 Replace raw JSON-RPC handler with proper routing for all endpoints
- [ ] 4.2 Preserve `/health` endpoint functionality
- [ ] 4.3 Add host binding support (default 127.0.0.1)
- [ ] 4.4 Update console output to show bound host:port

## 5. Testing & Verification

- [ ] 5.1 Test `npx nano-brain mcp --http --port=3100` starts SSE server
- [ ] 5.2 Verify stdio transport still works as default
- [ ] 5.3 Test `/health` endpoint returns expected JSON
