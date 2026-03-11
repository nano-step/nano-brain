# compact-search-results Specification

## Purpose
TBD - created by archiving change ccr-compressed-search-results. Update Purpose after archive.
## Requirements
### Requirement: Compact search result format
The system SHALL support a compact output format for search results that renders each result as a single line containing: 1-based index, score (3 decimal places), title, docid, path with start line, and the first line of the snippet truncated to 80 characters.

#### Scenario: Compact format renders single-line results
- **WHEN** a search returns 3 results and compact mode is enabled
- **THEN** each result is rendered as one line: `{index}. [{score}] {title} ({docid}) — {path}:{startLine} | {first_line_truncated}`
- **THEN** the total output is 3 lines of results plus a header and footer

#### Scenario: Snippet first line exceeds 80 characters
- **WHEN** a result's snippet first line is 120 characters long and compact mode is enabled
- **THEN** the compact line truncates the snippet portion to 80 characters followed by `…`

#### Scenario: Snippet first line is under 80 characters
- **WHEN** a result's snippet first line is 50 characters long and compact mode is enabled
- **THEN** the compact line includes the full first line without truncation

#### Scenario: Result has symbols in compact mode
- **WHEN** a result has `symbols: ["fetchUser", "validateToken"]` and compact mode is enabled
- **THEN** the compact line includes symbols: `{index}. [{score}] {title} ({docid}) — {path}:{startLine} [fetchUser, validateToken] | {first_line}`

#### Scenario: Empty results in compact mode
- **WHEN** a search returns 0 results and compact mode is enabled
- **THEN** the output is `No results found.` (same as verbose mode)

### Requirement: Compact response header
The system SHALL prepend a header to compact search responses containing the cache key and a usage hint for expansion.

#### Scenario: Compact response includes cache key and hint
- **WHEN** a search returns results in compact mode
- **THEN** the response starts with: `🔑 {cacheKey} | Use memory_expand(cacheKey, index) for full content | verbose:true for all`
- **THEN** the results follow after a blank line

### Requirement: Result cache for search results
The system SHALL cache full `SearchResult[]` arrays in an in-memory Map keyed by an incrementing counter (`search_1`, `search_2`, etc.). Cache entries SHALL expire after a configurable TTL (default 15 minutes). Expired entries SHALL be cleaned up lazily on access. The cache SHALL also store `startLine`, `endLine`, and `docid` per result for bounded fallback on cache miss.

#### Scenario: Search results are cached on compact query
- **WHEN** `memory_query` executes a search with compact mode enabled
- **THEN** the full `SearchResult[]` array is stored in the cache
- **THEN** the cache key is an incrementing counter (e.g., `search_1`, `search_2`)
- **THEN** the cache entry includes `startLine`, `endLine`, and `docid` for each result

#### Scenario: Cache entry expires after TTL
- **WHEN** a cache entry was created 16 minutes ago and the TTL is 15 minutes
- **THEN** the entry is not returned on lookup
- **THEN** the entry is removed from the cache on next access (lazy cleanup)

#### Scenario: Verbose query does not cache
- **WHEN** `memory_query` executes a search with compact mode disabled (verbose)
- **THEN** no cache entry is created

### Requirement: memory_expand MCP tool
The system SHALL provide a `memory_expand` MCP tool that retrieves the full snippet for specific search result(s) from the cache. It SHALL accept `cacheKey` (required), `index` (optional, 1-based number), `indices` (optional, array of 1-based numbers), and `docid` (optional) parameters.

#### Scenario: Expand by cache key and single index
- **WHEN** `memory_expand` is called with a valid `cacheKey` and `index: 3`
- **THEN** the tool returns the full formatted result for the 3rd item in the cached results array
- **THEN** the output includes title, path, score, lines, symbols, cluster, flows, and full snippet

#### Scenario: Expand by cache key and multiple indices
- **WHEN** `memory_expand` is called with a valid `cacheKey` and `indices: [1, 3, 5]`
- **THEN** the tool returns the full formatted results for items 1, 3, and 5
- **THEN** each result is separated by a divider

#### Scenario: Expand with expired cache and docid fallback
- **WHEN** `memory_expand` is called with an expired `cacheKey` and `docid: "abc123"`
- **THEN** the tool retrieves the document via `store.findDocument("abc123")`
- **THEN** the tool returns a bounded section of the document body using the cached `startLine`/`endLine` if available, or the first 2000 characters if line info is unavailable
- **THEN** the response includes a note: `⚠️ Cache expired. Showing document section instead of matched snippet.`

#### Scenario: Expand with expired cache and no docid
- **WHEN** `memory_expand` is called with an expired `cacheKey` and no `docid`
- **THEN** the tool returns an error: `Cache expired. Re-run your search or provide a docid.`

#### Scenario: Expand with invalid index
- **WHEN** `memory_expand` is called with `index: 15` but the cached results only have 10 items
- **THEN** the tool returns an error: `Index 15 out of range. Results have 10 items (1-10).`

#### Scenario: Expand without index, indices, or docid
- **WHEN** `memory_expand` is called with only `cacheKey` and none of `index`, `indices`, or `docid`
- **THEN** the tool returns an error: `Provide index, indices, or docid to expand.`

### Requirement: Compact parameter on existing search tools
The `memory_query`, `memory_search`, and `memory_vsearch` MCP tools SHALL accept an optional `compact` boolean parameter (default: false). When `compact` is true, results are returned in compact format with caching. When `compact` is false or omitted, results are returned in the current verbose format (backward compatible).

#### Scenario: memory_query defaults to verbose
- **WHEN** `memory_query` is called via MCP without a `compact` parameter
- **THEN** results are returned in the current verbose format (full snippets)
- **THEN** no cache entry is created

#### Scenario: memory_query with compact true
- **WHEN** `memory_query` is called with `compact: true`
- **THEN** results are returned in compact format with cache key header
- **THEN** results are cached for memory_expand

#### Scenario: memory_search with compact true
- **WHEN** `memory_search` is called with `compact: true`
- **THEN** results are returned in compact format with cache key header

### Requirement: CLI compact flag
The CLI `query`, `search`, and `vsearch` commands SHALL accept a `--compact` flag. When present, output uses the compact format. CLI defaults to verbose (current behavior).

#### Scenario: CLI query with --compact flag
- **WHEN** user runs `nano-brain query "auth middleware" --compact`
- **THEN** output uses compact single-line format with cache key header

#### Scenario: CLI query without --compact flag
- **WHEN** user runs `nano-brain query "auth middleware"`
- **THEN** output uses the current verbose format (unchanged behavior)

#### Scenario: CLI --compact with --json
- **WHEN** user runs `nano-brain query "auth" --compact --json`
- **THEN** JSON output includes the full `SearchResult[]` array (--json overrides --compact)
- **THEN** no compact formatting is applied

