## ADDED Requirements

### Requirement: Confidence classification on edges
Every edge in the graph SHALL have a `confidence` field with one of three values: `EXTRACTED`, `INFERRED`, or `AMBIGUOUS`.

#### Scenario: AST-extracted edges
- **WHEN** an edge is created from direct AST extraction (import statement, direct function call)
- **THEN** the edge's `confidence` SHALL be `EXTRACTED`

#### Scenario: Inferred edges
- **WHEN** an edge is created from heuristic inference (call-graph second pass, co-occurrence)
- **THEN** the edge's `confidence` SHALL be `INFERRED`

#### Scenario: Ambiguous edges
- **WHEN** an edge cannot be confidently classified
- **THEN** the edge's `confidence` SHALL be `AMBIGUOUS`

### Requirement: Default confidence for existing edges
All existing edges SHALL default to `EXTRACTED` confidence, as they originate from AST extraction.

#### Scenario: Migration backfill
- **WHEN** the `confidence` column is added to the edges table
- **THEN** all existing rows SHALL have `confidence = 'EXTRACTED'`

### Requirement: Confidence in MCP responses
All MCP tools that return edges SHALL include the `confidence` field in their response.

#### Scenario: memory_graph returns confidence
- **WHEN** an agent calls `memory_graph` for a symbol
- **THEN** each returned edge SHALL include `confidence: "EXTRACTED|INFERRED|AMBIGUOUS"`

#### Scenario: memory_impact returns confidence
- **WHEN** an agent calls `memory_impact` for a symbol
- **THEN** each returned edge SHALL include `confidence: "EXTRACTED|INFERRED|AMBIGUOUS"`

### Requirement: Confidence filtering
Agents SHALL be able to filter graph queries by confidence level.

#### Scenario: Filter by confidence
- **WHEN** an agent queries edges with `confidence = "EXTRACTED"`
- **THEN** only edges with that confidence level SHALL be returned
