## ADDED Requirements

### Requirement: Symbol search as third lane in hybrid search
The `hybridSearch()` function SHALL query the `code_symbols` table as a third result set alongside FTS and vector results. Symbol matches SHALL be converted to `SearchResult[]` format and included in RRF fusion.

#### Scenario: Symbol search lane executes when db is available
- **WHEN** `hybridSearch()` is called with `options.db` defined and `code_symbols` table has rows
- **THEN** `SymbolGraph.searchByName(query, projectHash)` is called for each query variant
- **THEN** matching symbols are converted to `SearchResult` objects using the symbol's `file_path` to look up the corresponding document
- **THEN** the symbol result set is included in the `allResultSets` array passed to `rrfFuse()`

#### Scenario: Symbol search lane skipped when db is unavailable
- **WHEN** `hybridSearch()` is called with `options.db` undefined
- **THEN** no symbol search is performed
- **THEN** FTS and vector lanes execute normally (existing behavior preserved)

#### Scenario: Symbol search lane skipped when code_symbols is empty
- **WHEN** `hybridSearch()` is called with `options.db` defined but `code_symbols` has 0 rows for the project
- **THEN** the symbol result set is empty
- **THEN** RRF fusion proceeds with only FTS and vector result sets

#### Scenario: Symbol results deduplicated with FTS/vector results
- **WHEN** a document matches in both the FTS lane and the symbol lane
- **THEN** RRF fusion combines their scores (same document ID accumulates scores from both lanes)
- **THEN** the document appears once in the final results with a higher combined score

#### Scenario: Symbol-only match surfaces in results
- **WHEN** a query matches a symbol name but the document does not match FTS or vector search
- **THEN** the document still appears in results via the symbol lane contribution to RRF

### Requirement: Symbol-to-SearchResult conversion
Symbol matches SHALL be converted to `SearchResult` objects by looking up the document that contains the symbol's file path.

#### Scenario: Symbol maps to indexed document
- **WHEN** a symbol with `file_path = "/src/store.ts"` matches the query
- **THEN** the corresponding document from the `documents` table (matching `path` and `collection`) is returned as a `SearchResult`
- **THEN** the `snippet` field contains the symbol's surrounding code context (lines `start_line` to `end_line`)

#### Scenario: Symbol file not in documents table
- **WHEN** a symbol matches but its `file_path` has no corresponding document in the `documents` table
- **THEN** the symbol match is silently dropped (not included in results)

### Requirement: Symbol lane weighting in RRF
The symbol search lane SHALL use a configurable weight in RRF fusion, defaulting to the same weight as the original query's FTS/vector lanes.

#### Scenario: Default symbol weight matches original query weight
- **WHEN** `hybridSearch()` runs with default `searchConfig`
- **THEN** the symbol result set weight equals 2 (same as original query FTS/vector weight)

#### Scenario: Symbol results from expanded queries use expansion weight
- **WHEN** query expansion produces additional queries
- **THEN** symbol results from expanded queries use weight 1 (same as expanded FTS/vector weight)

### Requirement: Post-fusion pipeline unchanged
All post-RRF-fusion processing (top-rank bonus, centrality boost, supersede demotion, reranking, score filtering, snippet formatting, symbol enrichment) SHALL continue to work identically after adding the symbol lane.

#### Scenario: Reranking works with three-lane results
- **WHEN** `hybridSearch()` runs with `useReranking: true` and all three lanes produce results
- **THEN** the reranker receives the fused top-K candidates
- **THEN** position-aware blending applies the same weights as before

#### Scenario: Centrality boost applies to symbol-surfaced results
- **WHEN** a document is surfaced only via the symbol lane and has `centrality > 0`
- **THEN** its score is boosted by `(1 + centrality_weight * centrality)` like any other result
