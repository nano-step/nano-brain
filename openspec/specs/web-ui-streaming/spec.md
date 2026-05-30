# web-ui-streaming Specification

## Purpose
TBD - created by archiving change web-ui. Update Purpose after archive.
## Requirements
### Requirement: SSE event bus
The server SHALL expose `GET /api/v1/events` as a Server-Sent Events stream that multiplexes runtime events from internal producers (embed queue, watcher, reindex, harvest, summarize) to subscribed browser clients.

#### Scenario: Client subscribes and receives events
- **WHEN** a browser opens an `EventSource` on `/api/v1/events?workspace=<hash>`
- **THEN** the server responds with HTTP 200, headers `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`
- **AND** SHALL send a `data:` SSE message within 100 ms (initial `{"type":"hello","server_version":"..."}` event) so the client can verify the stream is alive
- **AND** SHALL continue sending events as producers publish them

#### Scenario: Events are filtered by workspace
- **WHEN** a client subscribes with `?workspace=abc123`
- **AND** an event is published with `Workspace: "def456"`
- **THEN** the event SHALL NOT be sent to that client

#### Scenario: Events without a workspace are global
- **WHEN** a producer publishes an event with empty `Workspace` (e.g., server log)
- **THEN** the event SHALL be sent to all subscribed clients

#### Scenario: Stream closes on client disconnect
- **WHEN** the browser closes the EventSource (page unload, network error)
- **THEN** the server SHALL detect `c.Request().Context().Done()` and stop the per-request goroutine within 1 second
- **AND** the subscriber's channel SHALL be removed from the bus

### Requirement: Event types
The event bus SHALL emit at minimum the following event types:

| `type` field | When published | Payload shape |
|---|---|---|
| `hello` | On subscribe | `{server_version, workspace, ts}` |
| `embed_queue` | When the embed queue depth changes (debounced 500 ms) | `{pending, embedded, failed, ts}` |
| `reindex` | At start, progress, and completion of a reindex run | `{state: "started"\|"progress"\|"completed"\|"failed", scanned, embedded, deleted, skipped, error?, ts}` |
| `harvest` | At start, progress, completion | `{state, sessions_seen, sessions_summarized, error?, ts}` |
| `watcher` | When a watched file event triggers re-chunk | `{path, action: "added"\|"modified"\|"deleted", ts}` (rate-limited to 10/sec per workspace) |
| `lag` | When the per-subscriber buffer overflows | `{dropped, since_ts}` |

#### Scenario: Reindex progress is reported
- **WHEN** a client triggers `POST /api/v1/reindex`
- **AND** is subscribed to `/api/v1/events?workspace=<hash>`
- **THEN** within 200 ms it receives a `reindex` event with `state=started`
- **AND** receives at least one `state=progress` event during the run
- **AND** receives exactly one terminal event with `state ∈ {"completed","failed"}`

#### Scenario: Lag event is emitted under backpressure
- **WHEN** a slow client cannot drain events fast enough
- **AND** its bounded buffer (64 events) is full at the moment a new event arrives
- **THEN** the server SHALL drop the **newest** event via a non-blocking select-send (race-free; Go channels cannot atomically drain-and-send, so drop-oldest is not implementable)
- **AND** SHALL increment the subscriber's `dropped` counter
- **AND** within at most 5 seconds a periodic ticker SHALL emit a `lag` event to that subscriber containing `{dropped: N, since_ts: <last_lag_ts>}` and reset the counter
- **SO THAT** the UI can re-query authoritative state via REST instead of trusting cached counters

### Requirement: Connection limits
The server SHALL bound the number of concurrent SSE subscribers per remote IP to prevent goroutine exhaustion.

#### Scenario: Per-IP cap enforced
- **WHEN** a single client IP has 8 active SSE connections
- **AND** attempts to open a 9th
- **THEN** the server returns `429 Too Many Requests`

#### Scenario: Idle subscriber is reaped
- **WHEN** a subscriber has not had any event flushed in 5 minutes
- **THEN** the server SHALL close the connection
- **AND** the client EventSource SHALL auto-reconnect (built-in browser behavior)

