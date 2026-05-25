## ADDED Requirements

### Requirement: memory_graph_query MCP tool
The system SHALL expose `memory_graph_query` tool via MCP interface. The tool SHALL accept parameters: entity (required), maxDepth (optional, default 3), relationshipTypes (optional filter). The tool SHALL return the queried entity, connected entities, and relationships between them.

#### Scenario: Query entity with relationships
- **WHEN** user calls memory_graph_query with entity="Redis"
- **THEN** system returns Redis entity details
- **AND** system returns all entities connected to Redis within default depth
- **AND** system returns relationship edges between entities

#### Scenario: Query with depth limit
- **WHEN** user calls memory_graph_query with entity="AuthService", maxDepth=1
- **THEN** system returns only directly connected entities (depth 1)
- **AND** system does NOT traverse beyond first-degree relationships

#### Scenario: Query with relationship filter
- **WHEN** user calls memory_graph_query with entity="PaymentService", relationshipTypes=["depends_on"]
- **THEN** system returns only entities connected via "depends_on" relationships
- **AND** system excludes other relationship types from results

#### Scenario: Entity not found
- **WHEN** user calls memory_graph_query with non-existent entity
- **THEN** system returns error indicating entity not found
- **AND** system suggests similar entity names if available

### Requirement: memory_related MCP tool
The system SHALL expose `memory_related` tool via MCP interface. The tool SHALL accept parameters: topic (required), collection (optional), limit (optional, default 5). The tool SHALL return memories related to the given topic across specified or all collections.

#### Scenario: Find related memories by topic
- **WHEN** user calls memory_related with topic="authentication"
- **THEN** system returns up to 5 memories related to authentication
- **AND** results are ordered by relevance score

#### Scenario: Find related memories in specific collection
- **WHEN** user calls memory_related with topic="Redis", collection="decisions"
- **THEN** system searches only the "decisions" collection
- **AND** returns memories about Redis from that collection

#### Scenario: Custom result limit
- **WHEN** user calls memory_related with topic="caching", limit=10
- **THEN** system returns up to 10 related memories

#### Scenario: No related memories
- **WHEN** user calls memory_related with topic that has no matches
- **THEN** system returns empty array
- **AND** system does NOT return error

### Requirement: memory_timeline MCP tool
The system SHALL expose `memory_timeline` tool via MCP interface. The tool SHALL accept parameters: topic (required), startDate (optional), endDate (optional). The tool SHALL return chronological list of memories about the topic with change indicators.

#### Scenario: Get full timeline for topic
- **WHEN** user calls memory_timeline with topic="Redis"
- **THEN** system returns all memories mentioning Redis in chronological order
- **AND** each entry includes timestamp, summary, and change type

#### Scenario: Get timeline with date range
- **WHEN** user calls memory_timeline with topic="deployment", startDate="2024-01-01", endDate="2024-06-30"
- **THEN** system returns only memories within the specified date range

#### Scenario: Timeline shows evolution
- **WHEN** topic has multiple memories over time
- **THEN** timeline entries show change types: "new" for first mention, "updated" for confirmations, "contradicted" for conflicts

#### Scenario: Empty timeline
- **WHEN** user calls memory_timeline with topic that has no memories
- **THEN** system returns empty timeline array
- **AND** system does NOT return error

### Requirement: Tool parameter validation
The system SHALL validate all tool parameters before execution. Invalid parameters SHALL return descriptive error messages. Required parameters SHALL be enforced.

#### Scenario: Missing required parameter
- **WHEN** user calls memory_graph_query without entity parameter
- **THEN** system returns error "Required parameter 'entity' is missing"

#### Scenario: Invalid parameter type
- **WHEN** user calls memory_graph_query with maxDepth="three"
- **THEN** system returns error "Parameter 'maxDepth' must be a number"

#### Scenario: Parameter out of range
- **WHEN** user calls memory_graph_query with maxDepth=100
- **THEN** system returns error "Parameter 'maxDepth' must be between 1 and 10"
