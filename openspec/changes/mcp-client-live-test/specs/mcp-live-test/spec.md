## ADDED Requirements

### Requirement: Provide live MCP client test
The system SHALL include a vitest file that connects to a running nano-brain MCP server using the Streamable HTTP transport and exercises key MCP tools.

#### Scenario: Server is reachable
- **WHEN** the live MCP test runs and the server is reachable
- **THEN** the test suite executes tool calls and validates text responses

#### Scenario: Server is not reachable
- **WHEN** the live MCP test runs and the server is unreachable
- **THEN** the test suite skips all live MCP tests without failing
