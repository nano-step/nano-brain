## MODIFIED Requirements

### Requirement: Symbol graph queries include community context

The system SHALL include community ID in symbol metadata when available, enabling agents to understand which natural grouping a symbol belongs to.

#### Scenario: memory_symbols returns community_id
- **WHEN** an agent calls `memory_symbols` for a symbol
- **THEN** the response SHALL include `community_id` (integer or null)

#### Scenario: memory_graph returns community context
- **WHEN** an agent calls `memory_graph` for a symbol
- **THEN** the symbol's `community_id` SHALL be included in the response

### Requirement: Graph traversal respects confidence

The system SHALL support filtering graph traversal by confidence level, allowing agents to focus on high-confidence edges.

#### Scenario: Filter memory_impact by confidence
- **WHEN** an agent calls `memory_impact` with `confidence = "EXTRACTED"`
- **THEN** only edges with `confidence = "EXTRACTED"` SHALL be traversed

#### Scenario: Filter memory_trace by confidence
- **WHEN** an agent calls `memory_trace` with `confidence = "EXTRACTED"`
- **THEN** only edges with `confidence = "EXTRACTED"` SHALL be traversed
