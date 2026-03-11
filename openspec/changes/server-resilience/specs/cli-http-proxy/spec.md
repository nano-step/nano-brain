## ADDED Requirements

### Requirement: CLI detects running server
Before executing a command, the CLI SHALL check if a server is running by sending `GET http://localhost:{port}/health` with a 1-second timeout.

If the server is reachable, the CLI SHALL proxy the command via HTTP API.
If the server is not reachable, the CLI SHALL execute the command locally (current behavior).

#### Scenario: Server running — proxy via HTTP
- **WHEN** `nano-brain status` is executed AND the server is running on port 3100
- **THEN** the CLI sends `GET http://localhost:3100/api/status` AND displays the response

#### Scenario: Server not running — local execution
- **WHEN** `nano-brain status` is executed AND no server is running
- **THEN** the CLI executes the status command locally against the SQLite database

### Requirement: HTTP API endpoints
The server SHALL expose REST API endpoints for CLI proxy:

- `GET /api/status` — returns server status (same as `nano-brain status` output)
- `POST /api/query` — executes a memory query, body: `{ query, tags?, scope?, limit? }`
- `POST /api/search` — executes a search, body: `{ query, limit? }`

All API endpoints SHALL return JSON responses.

#### Scenario: Status endpoint
- **WHEN** `GET /api/status` is called
- **THEN** the server returns JSON with index health, model status, workspace info, and uptime

#### Scenario: Query endpoint
- **WHEN** `POST /api/query` is called with `{ "query": "auth patterns" }`
- **THEN** the server executes the query and returns results in JSON format

### Requirement: CLI never spawns server
CLI commands (status, query, search, write, focus, etc.) SHALL NOT spawn a new server process. Only the explicit `nano-brain serve` and `nano-brain mcp` commands SHALL start a server.

#### Scenario: Status command does not start server
- **WHEN** `nano-brain status` is executed and no server is running
- **THEN** the command executes locally without spawning any background process
