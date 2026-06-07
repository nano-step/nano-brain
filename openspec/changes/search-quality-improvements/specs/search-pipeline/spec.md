## ADDED Requirements

### Requirement: Query preprocessing translates non-English queries to English before search

The search pipeline SHALL detect non-English queries and translate them to English via an LLM call before executing BM25 and vector search. This ensures content indexed in English is retrievable regardless of query language.

#### Scenario: Vietnamese query returns relevant English content

Given query preprocessing is enabled in config
And a document containing "symbols are indexed when files change via fsnotify" exists
When user queries "khi nào thì symbol được index?"
Then the preprocessor translates to "when does symbol indexing trigger?"
And search returns the relevant document

#### Scenario: English query passes through unchanged

Given query preprocessing is enabled
When user queries "when does symbol indexing trigger?"
Then the preprocessor returns the query unchanged
And search proceeds normally

#### Scenario: Preprocessing disabled falls back to direct search

Given query preprocessing is disabled in config
When user queries in any language
Then the query passes directly to BM25 and vector search without modification

#### Scenario: LLM timeout falls back to original query

Given query preprocessing is enabled
And the LLM provider does not respond within max_latency_ms
Then the original query is used for search
And no error is surfaced to the caller

### Requirement: Query preprocessor expands queries with related terms

The preprocessor SHALL generate 2-3 related search terms alongside translation, improving recall for conceptual queries.

#### Scenario: Conceptual query gets expanded

Given query preprocessing is enabled
When user queries "how does the file watcher work?"
Then expansions include terms like "fsnotify", "debounce", "scanCollection"
And both original and expanded terms are used in search

### Requirement: Query preprocessor detects intent (keyword/conceptual/temporal)

The preprocessor SHALL classify each query into one of three intents to enable downstream optimizations (RRF weight adjustment, HyDE gating, temporal filter extraction).

#### Scenario: Keyword intent detected

Given query preprocessing is enabled
When user queries "memory_query handler"
Then intent is classified as "keyword"

#### Scenario: Conceptual intent detected

Given query preprocessing is enabled
When user queries "how does session harvesting work?"
Then intent is classified as "conceptual"

#### Scenario: Temporal intent detected

Given query preprocessing is enabled
When user queries "what did we decide last week about search?"
Then intent is classified as "temporal"
And a time filter is extracted (created_after = 7 days ago)

## MODIFIED Requirements

### Requirement: BM25 search language is configurable per workspace

The BM25 full-text search SHALL use a configurable PostgreSQL text search configuration instead of hardcoded `'english'`. Default remains `'english'` for backward compatibility.

#### Scenario: Default language is English

Given no bm25_language is configured
When a search query executes
Then `to_tsvector('english', ...)` and `websearch_to_tsquery('english', ...)` are used

#### Scenario: Simple language configured for multilingual

Given `search.bm25_language` is set to `"simple"`
When a search query executes
Then `to_tsvector('simple', ...)` and `websearch_to_tsquery('simple', ...)` are used
And language-agnostic tokenization is applied

### Requirement: Chunk overlap default increased to 600 bytes

The default chunk overlap SHALL be 600 bytes (previously 200) to preserve cross-chunk semantic context. Configurable via `watcher.chunk_overlap`.

#### Scenario: New chunks use 600-byte overlap

Given default config (no override)
When a new file is chunked by the watcher
Then adjacent chunks overlap by 600 bytes

#### Scenario: Custom overlap configured

Given `watcher.chunk_overlap` is set to 400
When a new file is chunked
Then adjacent chunks overlap by 400 bytes

### Requirement: MCP search tools default to document-level grouping

The MCP tools `memory_query`, `memory_search`, and `memory_vsearch` SHALL default to `group_by=document` when the parameter is not specified, returning at most one result per document.

#### Scenario: No group_by specified returns one result per document

Given a workspace with multiple chunks from the same document
When `memory_query` is called without `group_by` parameter
Then results are grouped by document (best chunk per document)

#### Scenario: Explicit group_by=none disables dedup

Given a workspace with multiple chunks from the same document
When `memory_query` is called with `group_by=""`
Then all matching chunks are returned individually
