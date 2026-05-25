## ADDED Requirements

### Requirement: SSE transport endpoint

The system SHALL expose an SSE endpoint at `GET /sse` that establishes a Server-Sent Events stream for MCP communication when started with `--http` flag.

#### Scenario: Client connects to SSE endpoint
- **WHEN** client sends GET request to `/sse`
- **THEN** server responds with `Content-Type: text/event-stream` and keeps connection open

#### Scenario: Server sends session endpoint
- **WHEN** SSE connection is established
- **THEN** server sends an event containing the POST endpoint URL with session ID (e.g., `/messages?sessionId=<uuid>`)

### Requirement: Message endpoint for SSE clients

The system SHALL expose a POST endpoint at `/messages` that receives MCP messages from SSE clients.

#### Scenario: Client sends message to correct session
- **WHEN** client POSTs to `/messages?sessionId=<valid-session-id>` with JSON-RPC message
- **THEN** server routes message to the corresponding SSE transport and returns 200

#### Scenario: Client sends message to invalid session
- **WHEN** client POSTs to `/messages?sessionId=<invalid-session-id>`
- **THEN** server returns 404 with error message

### Requirement: Multiple concurrent SSE clients

The system SHALL support multiple concurrent SSE client connections, each with isolated MCP server state but shared underlying resources (store, embedder, reranker).

#### Scenario: Two clients connect simultaneously
- **WHEN** client A and client B both connect to `/sse`
- **THEN** each receives unique session IDs and can communicate independently

#### Scenario: Client disconnect cleanup
- **WHEN** an SSE client disconnects
- **THEN** server removes the session from active sessions map and cleans up resources

### Requirement: CLI flag wiring for HTTP mode

The system SHALL pass `--http` and `--port` flags from `handleMcp()` to `startServer()` to enable HTTP transport mode.

#### Scenario: Start with HTTP flag
- **WHEN** user runs `npx nano-brain mcp --http --port=3100`
- **THEN** server starts HTTP listener on port 3100 instead of stdio

#### Scenario: Default to stdio without HTTP flag
- **WHEN** user runs `npx nano-brain mcp`
- **THEN** server starts with stdio transport (existing behavior)

### Requirement: Host binding configuration

The system SHALL support a `--host` flag to configure the bind address, defaulting to `127.0.0.1`.

#### Scenario: Default localhost binding
- **WHEN** user runs `npx nano-brain mcp --http --port=3100` without `--host`
- **THEN** server binds to `127.0.0.1:3100`

#### Scenario: Explicit host binding
- **WHEN** user runs `npx nano-brain mcp --http --port=3100 --host=0.0.0.0`
- **THEN** server binds to `0.0.0.0:3100` (all interfaces)

### Requirement: Health endpoint preservation

The system SHALL preserve the existing `/health` endpoint for monitoring.

#### Scenario: Health check request
- **WHEN** client sends GET request to `/health`
- **THEN** server returns JSON with `{ status: 'ok', uptime: <seconds> }`
