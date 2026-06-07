## ADDED Requirements

### Requirement: Entity extraction at index time
The system SHALL implement the following behavior:
WHEN a chunk of type `symbol` is indexed THEN entities (function names, type names, constant names) referenced within that chunk are extracted and stored in `chunk_entities` table. Variable names are excluded.

#### Scenario: Entities extracted from symbol chunk
- **GIVEN** file src/watcher/watcher.go indexed with symbol chunk for "processFile"
- **WHEN** chunk contains references to "extractAndUpsertSymbols", "chunkContent", "Symbol"
- **THEN** chunk_entities rows created with entity names normalized to lowercase
- **AND** entity_type set to "function" or "type" based on AST node kind

### Requirement: Post-RRF entity boost
The system SHALL implement the following behavior:
WHEN hybrid search completes RRF fusion AND `entity_boosting_enabled = true` THEN results are boosted by entity match count: `score += match_count * entity_boost_factor`. Query entity extraction SHALL use lookup-based matching: tokenize query, check each token against existing `chunk_entities.entity_name` values for the workspace (case-insensitive), and use matches as boost set. Stopwords (the, how, does, work, what, when, is, are, a, an) SHALL be excluded from lookup. When query contains no identifiable entities after lookup, boost step is skipped.

#### Scenario: Entity boost improves ranking
- **GIVEN** chunk A contains entities ["processfile", "symbol", "watcher"]
- **AND** chunk B contains entities ["config", "logger"]
- **WHEN** user queries "how does ProcessFile extract symbols?"
- **THEN** query tokens: ["how", "does", "processfile", "extract", "symbols"]
- **AND** stopwords removed: ["processfile", "extract", "symbols"]
- **AND** lookup against chunk_entities: "processfile" matches, "extract" no match, "symbols" no match
- **AND** boost set: ["processfile"]
- **AND** chunk A gets boost: score += 1 * 0.3
- **AND** chunk B gets no boost

#### Scenario: No entities found in query
- **GIVEN** query "how does authentication work?"
- **WHEN** tokens ["authentication", "work"] looked up against chunk_entities
- **AND** neither exists in chunk_entities for this workspace
- **THEN** entity boost step is skipped
- **AND** results identical to non-entity-linked search

### Requirement: Cascade deletion
The system SHALL implement the following behavior:
WHEN a chunk is deleted (file removed or re-indexed) THEN its entries in `chunk_entities` are cascade-deleted via FK ON DELETE CASCADE.

#### Scenario: File removal cleans entities
- **GIVEN** file src/old.go removed from workspace
- **WHEN** watcher deletes chunks for that file
- **THEN** all chunk_entities rows for those chunks auto-deleted
- **AND** no orphan entity records remain

### Requirement: Feature flag
The system SHALL implement the following behavior:
WHEN `entity_boosting_enabled = false` THEN post-RRF entity boost step is skipped entirely (zero overhead).

#### Scenario: Disabled entity linking
- **GIVEN** entity_boosting_enabled = false
- **WHEN** search query executes
- **THEN** RRF fusion runs normally
- **AND** entity boost step skipped
- **AND** results identical to pre-entity-linking behavior
