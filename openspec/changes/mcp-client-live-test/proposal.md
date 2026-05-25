## Why

We need a live MCP client test to validate that the Streamable HTTP transport and core tools work against a running nano-brain server. This closes the gap between unit tests and real MCP connectivity.

## What Changes

- Add a dedicated vitest file that connects to the MCP server over Streamable HTTP and exercises key tools.
- Make the test self-skipping when the server is not reachable so it does not fail the default test run.

## Capabilities

### New Capabilities
- `mcp-live-test`: Live MCP client test coverage against a running server.

### Modified Capabilities

## Impact

- New test file under `test/` that depends on a running MCP server at localhost.
- No runtime code or API changes.
