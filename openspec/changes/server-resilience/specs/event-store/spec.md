## ADDED Requirements

### Requirement: SQLite-backed EventStore
The server SHALL implement the MCP SDK `EventStore` interface using a `mcp_events` table in the existing SQLite database.

Table schema:
```sql
CREATE TABLE mcp_events (
  event_id TEXT PRIMARY KEY,
  stream_id TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_mcp_events_stream ON mcp_events(stream_id, event_id);
```

The EventStore SHALL be passed to `StreamableHTTPServerTransport` constructor.

#### Scenario: Store and replay events
- **WHEN** a client connects via `/mcp` AND the server sends MCP messages
- **THEN** each message is stored in `mcp_events` with a unique event_id

#### Scenario: Client reconnects with Last-Event-ID
- **WHEN** a client reconnects to `/mcp` with `Last-Event-ID` header
- **THEN** the server replays all events after that ID in order via `replayEventsAfter()`

### Requirement: Event cleanup
The server SHALL delete events older than 5 minutes. Cleanup SHALL run on a 60-second timer and on server startup.

The TTL SHALL be configurable via `config.yaml` under `server.eventTtlSeconds` (default: 300).

#### Scenario: Old events cleaned up
- **WHEN** the cleanup timer fires
- **THEN** all events with `created_at` older than 5 minutes are deleted from `mcp_events`

#### Scenario: Startup cleanup
- **WHEN** the server starts
- **THEN** stale events from previous runs are deleted before accepting connections

### Requirement: StreamableHTTP retryInterval
The server SHALL configure `retryInterval: 3000` on the StreamableHTTP transport so clients wait 3 seconds before reconnecting after disconnect.

#### Scenario: Client receives retry interval
- **WHEN** a client connects via `/mcp`
- **THEN** the SSE priming event includes `retry: 3000`
