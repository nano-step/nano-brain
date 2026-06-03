## Purpose

MCP server providing persistent memory tools (search, status, update, get) for AI coding agents via the Model Context Protocol.
## Requirements
### Requirement: ESM module compliance
All source files in `src/` SHALL use ESM `import` syntax exclusively. No `require()` calls SHALL exist in any TypeScript source file.

#### Scenario: Server starts under Node.js ESM runtime
- **WHEN** the MCP server is started via `node bin/cli.js mcp`
- **THEN** the server starts without `require is not defined` errors
- **THEN** all tool handlers execute without CJS/ESM compatibility errors

#### Scenario: No require() in source files
- **WHEN** running `grep -r "require(" src/` on the source directory
- **THEN** zero matches are returned (excluding comments and string literals)

### Requirement: Dynamic collection config reload
The `memory_update` tool handler SHALL reload the collection configuration file on every invocation, not use the cached startup value.

#### Scenario: Collection added after server start
- **WHEN** a user adds a collection via CLI (`collection add`) while the MCP server is running
- **THEN** calling `memory_update` through MCP indexes documents from the newly added collection
- **THEN** no server restart is required

#### Scenario: Collection removed after server start
- **WHEN** a user removes a collection via CLI while the MCP server is running
- **THEN** calling `memory_update` through MCP no longer indexes documents from the removed collection

### Requirement: All MCP tool handlers return valid responses
Every registered MCP tool SHALL return a valid JSON-RPC response for valid inputs, never an unhandled exception.

#### Scenario: memory_search with valid query
- **WHEN** `memory_search` is called with `{"query": "test"}` via JSON-RPC
- **THEN** a valid response with `content` array is returned

#### Scenario: memory_update with configured collections
- **WHEN** `memory_update` is called via JSON-RPC with collections configured
- **THEN** a valid response with reindex summary is returned, not a runtime error

#### Scenario: memory_status returns health info
- **WHEN** `memory_status` is called via JSON-RPC
- **THEN** a valid response with document count, chunk count, and collection info is returned

### Requirement: Search tools support workspace filtering
The `memory_search`, `memory_vsearch`, and `memory_query` MCP tools SHALL accept an optional `workspace` parameter. When omitted, results are scoped to the current workspace and global documents. When set to `"all"`, results include all workspaces.

#### Scenario: memory_search with default workspace scoping
- **WHEN** `memory_search` is called with `{"query": "test"}` and no `workspace` parameter
- **THEN** results are filtered to `currentProjectHash` and `'global'` documents only

#### Scenario: memory_vsearch with workspace="all"
- **WHEN** `memory_vsearch` is called with `{"query": "test", "workspace": "all"}`
- **THEN** results include documents from all workspaces

#### Scenario: memory_query with specific workspace
- **WHEN** `memory_query` is called with `{"query": "test", "workspace": "abc123def456"}`
- **THEN** results are filtered to `project_hash = 'abc123def456'` and `project_hash = 'global'`

### Requirement: memory_status reports storage usage
The `memory_status` MCP tool SHALL return vector store health, token usage metrics, and per-workspace storage info in addition to the existing index health, collection info, and model status.

#### Scenario: Status with Qdrant vector store
- **WHEN** `memory_status` is called and Qdrant is the active vector provider
- **THEN** the response SHALL include a "Vector Store" section with provider, connectivity status, vector count, and dimensions

#### Scenario: Status with token usage data
- **WHEN** `memory_status` is called and token_usage table has data
- **THEN** the response SHALL include a "Token Usage" section with per-model token counts and request counts

#### Scenario: Status with no vector store or token data
- **WHEN** `memory_status` is called with sqlite-vec (default) and no token usage recorded
- **THEN** the response SHALL include vector store section showing sqlite-vec as built-in, and omit the token usage section

### Requirement: Search tool parameter schema includes workspace

The MCP tool registration for `memory_search`, `memory_vsearch`, and `memory_query` SHALL include `workspace` in their input schema as an optional string parameter with description explaining the scoping behavior. Additionally, all three tools SHALL include a `compact` boolean parameter (optional, defaults to false) with description: "Set to true for compact single-line results with caching. Defaults to verbose." Additionally, all three tools SHALL include the `include_content` and `cursor` parameters per the snippet-only and pagination requirements in this capability.

#### Scenario: Tool schema advertises workspace parameter

- **WHEN** an MCP client lists available tools
- **THEN** `memory_search`, `memory_vsearch`, and `memory_query` each show a `workspace` parameter in their input schema
- **THEN** the parameter description explains: omit for current workspace, `"all"` for cross-workspace search

#### Scenario: Tool schema advertises compact parameter

- **WHEN** an MCP client lists available tools
- **THEN** `memory_search`, `memory_vsearch`, and `memory_query` each show a `compact` boolean parameter in their input schema
- **THEN** the parameter description explains: defaults to false (verbose), set to true for compact single-line results

#### Scenario: Tool schema advertises pagination and content parameters

- **WHEN** an MCP client lists available tools
- **THEN** `memory_search`, `memory_vsearch`, and `memory_query` each show `cursor` (string, optional) and `include_content` (boolean, optional) parameters in their input schema

### Requirement: memory_expand tool registration
The MCP server SHALL register a `memory_expand` tool with description "Expand a compact search result to see full content" and input schema: `cacheKey` (string, required), `index` (number, optional, 1-based result index), `indices` (number[], optional, array of 1-based indices), `docid` (string, optional, document ID fallback).

#### Scenario: memory_expand appears in tool listing
- **WHEN** an MCP client lists available tools
- **THEN** `memory_expand` is listed with its description and input schema
- **THEN** the schema shows `cacheKey` as required and `index`/`indices`/`docid` as optional

### Requirement: Search tools default to snippet-only responses

The `memory_query`, `memory_search`, and `memory_vsearch` MCP tools SHALL omit the `content` field from each search result in the default response. Each result SHALL include a `snippet` field containing at most 500 UTF-8 characters of the matched chunk text. The snippet SHALL preserve full Unicode characters (no truncation in the middle of a multi-byte rune).

#### Scenario: memory_query default response excludes content

- **WHEN** `memory_query` is called with `{"workspace": "<hash>", "query": "fix bug"}` (no `include_content` parameter)
- **THEN** every object in `response.results` SHALL include a `snippet` field of length ≤ 500 characters
- **THEN** no object in `response.results` SHALL include a `content` field (or `content` SHALL be omitted from the JSON, not present as empty string)

#### Scenario: memory_search default response excludes content

- **WHEN** `memory_search` is called with `{"workspace": "<hash>", "query": "test"}` (no `include_content` parameter)
- **THEN** every object in `response.results` SHALL include a `snippet` field of length ≤ 500 characters
- **THEN** no object in `response.results` SHALL include a `content` field

#### Scenario: memory_vsearch default response excludes content

- **WHEN** `memory_vsearch` is called with `{"workspace": "<hash>", "query": "test"}` (no `include_content` parameter)
- **THEN** every object in `response.results` SHALL include a `snippet` field of length ≤ 500 characters
- **THEN** no object in `response.results` SHALL include a `content` field

#### Scenario: Snippet truncation respects UTF-8 boundaries

- **WHEN** a search result's underlying chunk contains a multi-byte Unicode character (e.g., `é`, `世`, emoji) at character position 499–501
- **THEN** the returned `snippet` SHALL NOT contain a half-character (no invalid UTF-8 sequence in the response)
- **THEN** the snippet SHALL end at a valid character boundary at or before position 500

#### Scenario: Default response payload size budget

- **WHEN** any of `memory_query`, `memory_search`, `memory_vsearch` is called with `max_results: 10` and default parameters (no `include_content`)
- **THEN** the serialized JSON response body SHALL be ≤ 20,000 bytes for a result set of 10 results with typical 50–2000 char chunks

### Requirement: Search tools accept opt-in include_content parameter

The `memory_query`, `memory_search`, and `memory_vsearch` MCP tools SHALL accept an optional boolean input parameter `include_content`. When `include_content: true`, the response SHALL include a `content` field on every result containing the full chunk text. When `include_content: false` or omitted, the response SHALL omit the `content` field per the snippet-only requirement above.

#### Scenario: include_content=true preserves full chunk content

- **WHEN** `memory_query` is called with `{"workspace": "<hash>", "query": "test", "include_content": true}`
- **THEN** every object in `response.results` SHALL include both `snippet` (≤500 chars) and `content` (full chunk text) fields
- **THEN** `content` field length SHALL equal the underlying `chunks.content` column value for that result

#### Scenario: include_content=false matches default behavior

- **WHEN** `memory_search` is called with `{"workspace": "<hash>", "query": "test", "include_content": false}`
- **THEN** the response SHALL be byte-identical to the same call with `include_content` omitted

#### Scenario: Tool schema advertises include_content parameter

- **WHEN** an MCP client lists available tools via `tools/list`
- **THEN** `memory_query`, `memory_search`, and `memory_vsearch` SHALL each show an `include_content` boolean parameter in their input schema
- **THEN** the parameter description SHALL explain: "Set to true to include full chunk content alongside the snippet. Defaults to false. Increases response size; prefer memory_get for fetching one full document."

### Requirement: Search tools support cursor-based pagination

The `memory_query`, `memory_search`, and `memory_vsearch` MCP tools SHALL accept an optional string input parameter `cursor` and SHALL emit an optional string response field `next_cursor`. When `next_cursor` is present, the agent MAY pass it as the `cursor` parameter on the next call to retrieve the subsequent page. When the same call with `cursor` is made on a quiescent index, the returned page SHALL contain results that immediately follow the previously returned page (no overlap, no skip).

The cursor SHALL be an opaque base64url-encoded JSON object with shape `{"o": <int offset>, "q": <string query_hash>}` where `query_hash` is the first 16 hex characters of `sha256(query_text)`. Clients MUST NOT parse or modify the cursor; servers MAY change the encoding without notice.

When `cursor` is provided and the embedded `query_hash` does not match `sha256(query_text)[:16]` for the current call's query, the tool SHALL return an error response with message containing "cursor query mismatch" or equivalent.

#### Scenario: First page omits cursor parameter

- **WHEN** `memory_search` is called with `{"workspace": "<hash>", "query": "test", "max_results": 5}` and no `cursor`
- **THEN** the response SHALL contain `results` (length 0–5)
- **THEN** if there exist more than 5 matching results, the response SHALL include a non-empty `next_cursor` string
- **THEN** if 5 or fewer results matched in total, `next_cursor` SHALL be absent from the response (not present as empty string or null)

#### Scenario: Subsequent page uses cursor from previous response

- **GIVEN** a workspace containing 12 documents matching the query "alpha"
- **WHEN** `memory_search` is called with `{"workspace": "<hash>", "query": "alpha", "max_results": 5}` returning results `[r1..r5]` and `next_cursor: "C1"`
- **AND THEN** `memory_search` is called with `{"workspace": "<hash>", "query": "alpha", "max_results": 5, "cursor": "C1"}`
- **THEN** the response SHALL contain `results: [r6..r10]` with no overlap and no skip
- **THEN** the response SHALL include `next_cursor: "C2"` (since 2 more results exist)

#### Scenario: Final page has no next_cursor

- **GIVEN** a workspace containing exactly 12 documents matching the query "alpha"
- **WHEN** the agent paginates through to the third page (results 11–12)
- **THEN** the response SHALL contain `results: [r11, r12]`
- **THEN** `next_cursor` SHALL be absent from the response

#### Scenario: Cursor from different query is rejected

- **WHEN** `memory_search` is called with `{"workspace": "<hash>", "query": "beta", "cursor": "<cursor-encoded-for-query-alpha>"}`
- **THEN** the tool SHALL return an error response
- **THEN** the error message SHALL contain the substring "cursor query mismatch"

#### Scenario: Malformed cursor is rejected

- **WHEN** any search tool is called with `{"workspace": "<hash>", "query": "x", "cursor": "not-valid-base64!@#"}`
- **THEN** the tool SHALL return an error response with message containing "invalid cursor"

#### Scenario: Tool schema advertises cursor parameter

- **WHEN** an MCP client lists available tools
- **THEN** `memory_query`, `memory_search`, and `memory_vsearch` SHALL each show a `cursor` string parameter (optional) in their input schema
- **THEN** the parameter description SHALL explain: "Opaque pagination cursor from the next_cursor field of a previous response. Pass the same query when paginating."

### Requirement: Search responses include total and query_ms metadata

The response envelope of `memory_query`, `memory_search`, and `memory_vsearch` SHALL include a `total` integer field and a `query_ms` integer field at the top level (alongside `results` and optional `next_cursor`).

`total` SHALL be the count of results in the fused result set fetched on the current request (i.e., `len(results_after_fusion_and_recency_boost)` before pagination slicing), not an exact COUNT(*) over the index.

`query_ms` SHALL be the server-measured elapsed milliseconds from the request entering the tool handler to the response being marshaled, rounded to the nearest integer.

#### Scenario: Response includes total field

- **WHEN** any search tool is called and returns ≥ 1 result
- **THEN** the response SHALL include a `total` integer field at the top level
- **THEN** `total` SHALL be ≥ `len(results)` in the current page

#### Scenario: Response includes query_ms field

- **WHEN** any search tool is called and completes successfully
- **THEN** the response SHALL include a `query_ms` integer field at the top level
- **THEN** `query_ms` SHALL be ≥ 0

#### Scenario: Empty result set still includes both fields

- **WHEN** any search tool is called with a query that matches zero documents
- **THEN** the response SHALL include `results: []`, `total: 0`, and `query_ms: <non-negative integer>`
- **THEN** the response SHALL NOT include `next_cursor`

