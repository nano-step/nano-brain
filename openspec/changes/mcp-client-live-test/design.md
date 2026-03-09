## Context

The project already has integration tests that exercise MCP tool handlers directly, but there is no test that validates the Streamable HTTP client/transport against a live MCP server. This change adds a live, opt-in test to cover the real wire path.

## Goals / Non-Goals

**Goals:**

- Add a vitest file that uses the MCP SDK Streamable HTTP transport to call key tools against a running server.
- Ensure the test suite skips gracefully when the server is not available.

**Non-Goals:**

- No changes to MCP server behavior or tool implementations.
- No mocking or simulated transports.

## Decisions

- Use `@modelcontextprotocol/sdk` `Client` and `StreamableHTTPClientTransport` to match production usage.
- Validate responses by checking returned text payloads for expected markers.

## Risks / Trade-offs

- Live tests can fail if the server is not running; mitigate by skipping when unreachable.
