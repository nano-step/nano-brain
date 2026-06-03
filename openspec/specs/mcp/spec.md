# mcp Specification

## Purpose
TBD - created by archiving change fix-mcp-wake-up-collections-filter. Update Purpose after archive.
## Requirements
### Requirement: memory_wake_up MUST filter recent_memories to memory and session-summary collections

The `memory_wake_up` MCP tool MUST invoke the underlying `RecentDocuments` storage query with `Collections = ["memory", "session-summary"]`, matching the behaviour of the HTTP `POST /api/v1/wake-up` endpoint introduced by issue #338.

#### Scenario: Workspace contains memory and code documents

- **GIVEN** a registered workspace with N>0 documents in the `memory` collection AND M>0 documents in the `code` collection
- **WHEN** an MCP client calls `memory_wake_up` with that workspace hash and `limit=10`
- **THEN** the response `recent_memories` array MUST contain only documents whose `collection` is `memory` or `session-summary`
- **AND** documents in the `code` collection MUST NOT appear in `recent_memories`
- **AND** the response shape MUST be identical to the HTTP `POST /api/v1/wake-up` endpoint for the same workspace and limit

#### Scenario: Workspace only has code documents

- **GIVEN** a registered workspace with only `code` collection documents
- **WHEN** an MCP client calls `memory_wake_up` with that workspace hash
- **THEN** `recent_memories` MUST be an empty array (not null)
- **AND** `active_collections` MUST still list the `code` collection with its document count

### Requirement: memory_query, memory_search, memory_vsearch MUST accept four optional time-range filter parameters

The MCP tools `memory_query`, `memory_search`, and `memory_vsearch` SHALL each accept four optional input-schema parameters: `created_after`, `created_before`, `updated_after`, `updated_before`. Each parameter SHALL accept either an RFC3339 timestamp string (e.g. `"2026-05-04T12:00:00Z"`) or a relative duration string (Go-style `"720h"` or humanish `"30d"`, `"1w"`, `"2mo"`, `"1y"`). When supplied, the filter SHALL restrict results to chunks whose parent document's `created_at` / `updated_at` falls within the bounds (AND semantics across multiple filters). When omitted, the tool's behavior SHALL be identical to the pre-change behavior.

#### Scenario: All filters omitted — behavior unchanged

- **GIVEN** a workspace with 100 documents written over the past year
- **WHEN** an MCP client calls `memory_query` with `query="..."` and no time-range parameters
- **THEN** the response SHALL be byte-identical to the pre-change `memory_query` response for the same input
- **AND** the generated SQL plan (verified via EXPLAIN ANALYZE) SHALL be identical to master pre-change on the same fixture

#### Scenario: updated_after with relative duration

- **GIVEN** a workspace with documents at `updated_at` = 5 days ago, 20 days ago, 60 days ago
- **WHEN** an MCP client calls `memory_search` with `query="..."` and `updated_after="30d"`
- **THEN** only the 5-day-old and 20-day-old documents' chunks SHALL appear in results
- **AND** the 60-day-old document's chunks SHALL NOT appear

#### Scenario: created_after with RFC3339 timestamp

- **GIVEN** a workspace with documents created on 2026-04-01, 2026-05-01, 2026-06-01
- **WHEN** an MCP client calls `memory_vsearch` with `query="..."` and `created_after="2026-05-15T00:00:00Z"`
- **THEN** only the 2026-06-01 document's chunks SHALL appear in results

#### Scenario: All four filters combined (AND semantics)

- **GIVEN** a workspace with mixed `created_at` / `updated_at` values
- **WHEN** an MCP client calls `memory_query` with all four filters specifying a 1-week window on both `created_at` and `updated_at`
- **THEN** only documents satisfying ALL four bounds SHALL appear in results
- **AND** documents satisfying 3 of 4 bounds SHALL NOT appear

#### Scenario: Invalid duration string rejected before DB call

- **WHEN** an MCP client calls `memory_query` with `updated_after="banana"`
- **THEN** the tool SHALL return an MCP error response naming the offending parameter (`updated_after`) and the rejected value (`banana`)
- **AND** NO database query SHALL be executed
- **AND** the error message SHALL document the accepted formats (RFC3339 / Go duration / humanish relative)

#### Scenario: Date-only string rejected (timezone required)

- **WHEN** an MCP client calls `memory_search` with `updated_after="2026-05-04"`
- **THEN** the tool SHALL return an MCP error response indicating the parameter must include a timezone
- **AND** the error message SHALL suggest the corrected form `2026-05-04T00:00:00Z`

#### Scenario: Pagination cursor invalidated when filters change between calls

- **GIVEN** a paginated `memory_search` call with `updated_after="30d"` returned a `next_cursor` token
- **WHEN** a follow-up call passes the same `next_cursor` but changes `updated_after` to `"7d"`
- **THEN** the tool SHALL return a "cursor invalidated, restart pagination" error (consistent with #359 behavior on query-text change)

#### Scenario: Pagination cursor remains valid when filters unchanged

- **GIVEN** a paginated `memory_query` with `updated_after="30d"` returned `next_cursor`
- **WHEN** a follow-up call passes the same `next_cursor` AND the same `updated_after="30d"`
- **THEN** the tool SHALL return the next page of the same filtered result set
- **AND** no result SHALL appear in both pages
- **AND** no result in the filtered set SHALL be skipped between pages

#### Scenario: Negative or zero relative duration rejected

- **WHEN** an MCP client calls `memory_query` with `updated_after="-30d"` or `updated_after="0d"` or `updated_after="-720h"`
- **THEN** the tool SHALL return an MCP error response naming the offending parameter and explaining that relative durations must be positive
- **AND** NO database query SHALL be executed
- **AND** the error SHALL NOT silently produce a future cutoff

#### Scenario: Inverted time range returns empty result set

- **GIVEN** a workspace with documents at various timestamps
- **WHEN** an MCP client calls `memory_search` with `updated_after="2026-06-01T00:00:00Z"` AND `updated_before="2026-05-01T00:00:00Z"` (inverted)
- **THEN** the tool SHALL return an empty results array (not an error)
- **AND** the response shape SHALL be the same as a valid filter that matches zero documents
- **AND** `total` (if present) SHALL be `0`

#### Scenario: Tag change between paginated calls invalidates cursor

- **GIVEN** a paginated `memory_search` call with `tags=["bug"]` returned a `next_cursor`
- **WHEN** a follow-up call passes the same `next_cursor` but changes `tags` to `["feature"]`
- **THEN** the tool SHALL return a "cursor invalidated" error (this scenario captures the fix to the pre-existing bug where only query text invalidated cursors; the extended `QueryHash` now invalidates on any filter change including tags, scope, collections, and time-range)

