# Epic 5: MCP Integration — User Stories

**Epic summary:** AI agents connect to nano-brain via MCP-over-HTTP. Both SSE and Streamable HTTP transports are mounted on the Echo server. All 9 tools are registered on both transports. Workspace is required on every tool call. Concurrent calls are race-free.

**Depends on:** Epic 4 (search endpoints, write endpoint, status endpoint must be live)

**Primary package:** `internal/mcp/`

---

#### Story 5.1: Mount MCP SSE and Streamable HTTP Transports

**Description:** As a platform engineer, I want both MCP transports registered on the existing Echo server so that MCP clients can connect without running a separate process.

Using `github.com/modelcontextprotocol/go-sdk` v1.5+, create `internal/mcp/sse.go` and `internal/mcp/streamable.go`. Each file creates the respective `mcp.SSEHandler` / `mcp.StreamableHTTPHandler`, which both implement `http.Handler`. Routes are mounted in `internal/server/routes.go`: `GET /sse`, `POST /messages` (SSE), `GET /mcp`, `POST /mcp` (Streamable HTTP). No tools are registered in this story; the transports must accept connections and respond to the MCP initialization handshake.

**Covers:** FR-66, FR-67

**Applies:** AR-4 (MCP Go SDK v1.5+), AR-3 (Echo v4)

**Complexity:** M

**Acceptance Criteria:**

- Given the server starts successfully, when a client sends `GET /sse`, then the server responds with `Content-Type: text/event-stream` and completes the MCP initialization handshake.
- Given the server starts successfully, when a client sends `GET /mcp`, then the server responds with a valid Streamable HTTP session response.
- Given an MCP client connects on `/mcp`, when no tool calls are made, then `POST /mcp` succeeds without errors.
- Given the server is running, when both `/sse` and `/mcp` are hit concurrently by separate clients, then both connections succeed independently.
- Given a test makes an HTTP GET to `/sse` with `Accept: text/event-stream`, when the connection is established, then the response stream includes the MCP `initialize` response event before any timeout.

---

#### Story 5.2: Register All 9 MCP Tools on Both Transports

**Description:** As an AI agent developer, I want all 9 MCP tools available on both SSE and Streamable HTTP so that my agent can use the same toolset regardless of which transport it prefers.

In `internal/mcp/adapter.go`, register the following tools on both handlers: `memory_query`, `memory_search`, `memory_vsearch`, `memory_get`, `memory_write`, `memory_tags`, `memory_status`, `memory_update`, `memory_wake_up`. Each tool delegates to the corresponding service-layer call already implemented in Epics 2, 3, and 4. Tool input schemas mirror the corresponding HTTP endpoint parameters.

**Covers:** FR-71

**Applies:** AR-4 (MCP Go SDK v1.5+), AR-6 (constructor injection — services wired at startup)

**Complexity:** M

**Acceptance Criteria:**

- Given an MCP client calls `tools/list` on the SSE transport, then the response includes all 9 tool names.
- Given an MCP client calls `tools/list` on the Streamable HTTP transport, then the response includes the same 9 tool names.
- Given a valid `memory_query` call on the SSE transport with a workspace and query string, when the call completes, then the response contains a `results` array matching the hybrid search schema.
- Given a valid `memory_search` call on the Streamable HTTP transport, then the response matches the BM25 search schema.
- Given a valid `memory_status` call on either transport, then the response includes `queue_depth` and the active provider name.
- Given a valid `memory_update` call, then the server triggers an async reindex and responds with a 200-equivalent MCP success.

---

#### Story 5.3: Enforce Workspace Parameter on All Tool Calls

**Description:** As a workspace operator, I want every MCP tool call to require an explicit workspace parameter so that no query ever leaks data from another workspace.

In the MCP adapter, each tool handler checks that `workspace` is present before dispatching to the service layer. If the parameter is absent, the tool returns an MCP error response with HTTP status 400 and a descriptive message. Passing `workspace: "all"` is valid for read tools and triggers cross-workspace queries (mirrors FR-11 and FR-21 behavior). The workspace-to-hash resolution uses the same path as the HTTP middleware.

**Covers:** FR-72

**Applies:** AR-4

**NFRs enforced:** NFR-2 (workspace isolation on every tool)

**Complexity:** S

**Acceptance Criteria:**

- Given an MCP client calls `memory_query` without a `workspace` field, then the server returns an MCP error with code 400 and message describing the missing parameter.
- Given an MCP client calls `memory_search` without a `workspace` field, then the same 400 error is returned.
- Given an MCP client calls `memory_write` without a `workspace` field, then the same 400 error is returned.
- Given an MCP client calls `memory_query` with `workspace: "all"`, then the response merges results from all workspaces via RRF (same behavior as `scope=all` on the HTTP endpoint).
- Given workspace A and workspace B both contain documents, when `memory_query` is called with workspace A's hash, then no documents from workspace B appear in the results.
- Given the same test run as above with workspace B's hash, then workspace A documents do not appear.

---

#### Story 5.4: Concurrent Tool Calls Are Race-Free

**Description:** As a platform engineer, I want the MCP server to handle multiple simultaneous tool calls without data races or corruption so that agents running in parallel don't interfere with each other.

Each tool handler executes in its own goroutine as provided by the MCP SDK. All state access goes through the pgxpool connection pool, which is goroutine-safe. No in-process mutable state is shared between tool invocations. The CI gate runs `go test -race` against the MCP adapter.

**Covers:** FR-73

**Applies:** AR-4

**NFRs enforced:** NFR-1 (concurrent tool calls must be race-free)

**Complexity:** M

**Acceptance Criteria:**

- Given 10 concurrent MCP clients each calling `memory_write` with unique content, when all calls complete, then all 10 documents exist in the database with no duplicates and no constraint violations.
- Given 10 concurrent MCP clients each calling `memory_query` with the same workspace, then all 10 receive a valid response and no call hangs or panics.
- Given `go test -race ./internal/mcp/...` runs the MCP adapter tests, then no data races are reported.
- Given the MCP adapter integration test fires 20 concurrent tool calls across `memory_query`, `memory_search`, and `memory_write`, then all calls return without error.
- Given the server's pgxpool is at max capacity, when concurrent MCP calls exceed pool capacity, then excess calls wait (not panic) and complete once a connection is available.

---

#### Story 5.5: Streamable HTTP 30-Second SSE Heartbeats

**Description:** As an AI agent running behind a proxy or load balancer, I want the Streamable HTTP transport to send periodic heartbeat events so that long-running tool calls don't time out at the network layer.

The `mcp.StreamableHTTPHandler` is configured with a 30-second heartbeat interval. When a Streamable HTTP session is established, the server emits a keep-alive SSE comment (`:\n\n`) every 30 seconds with no active tool call. This is transparent to the MCP client and requires no client-side change.

**Covers:** FR-74

**Applies:** AR-4

**Complexity:** S

**Acceptance Criteria:**

- Given a Streamable HTTP MCP session is established and idle, when 30 seconds elapse, then the SSE stream contains at least one heartbeat event or keep-alive comment.
- Given the heartbeat interval is 30 seconds, when a test waits 35 seconds with an open session, then the connection remains open and the client has received at least one heartbeat.
- Given the server is under load serving other tool calls, then heartbeats on idle sessions are not blocked or delayed by more than 5 seconds beyond the 30-second target.
- Given the SSE transport (`/sse`), then no heartbeat requirement applies (heartbeats are Streamable HTTP only per FR-74).

---

#### Story 5.6: memory_get with Document Path, #docid, and Line Range

**Description:** As an AI agent, I want `memory_get` to retrieve a specific document by file path or internal ID, with an optional line range, so that I can fetch exactly the section I need without retrieving an entire large document.

`memory_get` accepts either a file path string or a `#docid` prefixed string as its first parameter. An optional `{start_line, end_line}` object narrows the response to that line range, using the chunk position data stored in Epics 2 and 3 (FR-52). The response includes the document content (or the specified range), plus metadata: title, tags, collection, workspace hash, created_at, updated_at.

`memory_write` accepts an optional `supersedes` field containing a document path or ID. When present, the relationship is recorded in the database. No search-time demotion occurs in v2.0; the data is captured for Tier 2.

**Covers:** FR-75, FR-72b

**Applies:** AR-4, AR-8 (sqlc for document fetch query)

**Complexity:** M

**Acceptance Criteria:**

- Given a document was ingested and assigned an internal ID, when `memory_get` is called with `#<docid>`, then the full document content and metadata are returned.
- Given a document was ingested from a file at `/some/path/doc.md`, when `memory_get` is called with that path string, then the document is returned.
- Given `memory_get` is called with a path and `{start_line: 10, end_line: 20}`, then only the content spanning lines 10 to 20 is returned in the response.
- Given `memory_get` is called with a path and no line range, then the full document content is returned.
- Given `memory_get` is called with a nonexistent path, then the tool returns an MCP error with a descriptive not-found message.
- Given `memory_write` is called with `supersedes: "#<docid>"`, when the write completes, then the database contains a row linking the new document ID to the superseded document ID.
- Given `memory_write` is called without a `supersedes` field, then no supersede relationship is recorded and the write completes normally.
- Given `memory_write` is called with `supersedes: "path/to/old-doc.md"`, then the relationship is resolved by path and recorded.
