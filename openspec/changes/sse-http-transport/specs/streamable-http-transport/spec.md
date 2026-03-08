## ADDED Requirements

### Requirement: Streamable HTTP transport endpoint

The system SHALL expose a Streamable HTTP endpoint at `/mcp` that supports the newer MCP Streamable HTTP protocol.

#### Scenario: Client initiates Streamable HTTP session
- **WHEN** client sends GET request to `/mcp` with appropriate headers
- **THEN** server establishes SSE stream for server-to-client messages

#### Scenario: Client sends message via Streamable HTTP
- **WHEN** client sends POST request to `/mcp` with JSON-RPC message
- **THEN** server processes message and responds appropriately

#### Scenario: Client closes Streamable HTTP session
- **WHEN** client sends DELETE request to `/mcp`
- **THEN** server closes the session and cleans up resources

### Requirement: Streamable HTTP session management

The system SHALL manage Streamable HTTP sessions independently from SSE sessions, using session IDs from request headers.

#### Scenario: Session ID in request header
- **WHEN** client includes `mcp-session-id` header in POST/DELETE request
- **THEN** server routes request to the corresponding session

#### Scenario: Missing session ID for new connection
- **WHEN** client sends GET to `/mcp` without session ID
- **THEN** server creates new session and returns session ID in response headers

### Requirement: Coexistence with SSE transport

The system SHALL support both SSE (`/sse`, `/messages`) and Streamable HTTP (`/mcp`) endpoints simultaneously on the same HTTP server.

#### Scenario: Both transports active
- **WHEN** server is started with `--http` flag
- **THEN** both `/sse` and `/mcp` endpoints are available and functional

#### Scenario: Independent client sessions
- **WHEN** one client uses SSE transport and another uses Streamable HTTP
- **THEN** both clients operate independently without interference
