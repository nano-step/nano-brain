## MODIFIED Requirements

### Requirement: StreamableHTTP transport with resumability
The StreamableHTTP `/mcp` endpoint SHALL be configured with:
- `eventStore`: SQLite-backed EventStore instance
- `retryInterval: 3000`: Clients wait 3 seconds before reconnecting
- `sessionIdGenerator: () => randomUUID()`: Session tracking enabled

The SSE `/sse` endpoint SHALL remain unchanged (no EventStore support per MCP SDK limitation).

#### Scenario: StreamableHTTP client reconnects after server restart
- **WHEN** the server restarts AND a StreamableHTTP client reconnects within 5 minutes
- **THEN** the client creates a new session (old session is lost) AND tools are available immediately (or return "warming up" during Phase 2)

#### Scenario: SSE client reconnects after server restart
- **WHEN** the server restarts AND an SSE client reconnects
- **THEN** the client creates a new SSE session (no event replay) AND tools are available
