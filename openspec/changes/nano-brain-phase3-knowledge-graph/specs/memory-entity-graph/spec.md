## ADDED Requirements

### Requirement: Entity extraction from memories
The system SHALL extract entities from memory content using LLM when `memory_write` is called with consolidation enabled. Entity types SHALL include: tool, service, person, concept, decision, file, library. Each extracted entity SHALL have a name, type, and optional description.

#### Scenario: Extract entities from technical memory
- **WHEN** user writes memory "We decided to use Redis for caching because it supports pub/sub"
- **THEN** system extracts entities: {name: "Redis", type: "tool"}, {name: "caching", type: "concept"}, {name: "pub/sub", type: "concept"}

#### Scenario: Extract entities with relationships
- **WHEN** user writes memory "The AuthService depends on Redis for session storage"
- **THEN** system extracts entities: {name: "AuthService", type: "service"}, {name: "Redis", type: "tool"}, {name: "session storage", type: "concept"}
- **AND** system extracts relationship: {source: "AuthService", target: "Redis", type: "depends_on"}

#### Scenario: No entities found
- **WHEN** user writes memory with no extractable entities (e.g., "Fixed a typo")
- **THEN** system stores memory without creating any entities

### Requirement: Relationship extraction from memories
The system SHALL extract relationships between entities from memory content. Relationship types SHALL include: uses, depends_on, decided_by, related_to, replaces, configured_with. Each relationship SHALL have a source entity, target entity, and relationship type.

#### Scenario: Extract dependency relationship
- **WHEN** memory contains "PaymentService uses Stripe API"
- **THEN** system creates relationship {source: "PaymentService", target: "Stripe API", type: "uses"}

#### Scenario: Extract decision relationship
- **WHEN** memory contains "Team decided to replace MySQL with PostgreSQL"
- **THEN** system creates entities for MySQL and PostgreSQL
- **AND** system creates relationship {source: "PostgreSQL", target: "MySQL", type: "replaces"}

### Requirement: Entity storage in SQLite graph
The system SHALL store extracted entities in a `memory_entities` table with columns: id, name, type, description, project_hash, first_learned_at, last_confirmed_at. The system SHALL store relationships in a `memory_edges` table with columns: id, source_id, target_id, edge_type, project_hash, created_at.

#### Scenario: Store new entity
- **WHEN** entity "Redis" of type "tool" is extracted
- **AND** no existing entity with same normalized name and type exists
- **THEN** system inserts new row in memory_entities with first_learned_at set to current timestamp

#### Scenario: Update existing entity
- **WHEN** entity "Redis" of type "tool" is extracted
- **AND** existing entity with same normalized name and type exists
- **THEN** system updates last_confirmed_at to current timestamp
- **AND** system does NOT create duplicate entity

### Requirement: Entity deduplication by normalized name and type
The system SHALL deduplicate entities by case-insensitive normalized name combined with entity type. "Redis", "redis", and "REDIS" with type "tool" SHALL resolve to the same entity. "Redis" with type "tool" and "Redis" with type "person" SHALL be different entities.

#### Scenario: Case-insensitive deduplication
- **WHEN** memory mentions "REDIS" and existing entity "redis" of type "tool" exists
- **THEN** system links to existing entity instead of creating new one

#### Scenario: Type-aware deduplication
- **WHEN** memory mentions "Redis" as a person name
- **AND** existing entity "Redis" of type "tool" exists
- **THEN** system creates new entity "Redis" of type "person"

### Requirement: Graph traversal via BFS
The system SHALL support breadth-first search traversal of the entity graph with configurable depth limit. Default depth limit SHALL be 3. Maximum depth limit SHALL be 10.

#### Scenario: Traverse entity relationships
- **WHEN** user queries graph starting from entity "AuthService" with depth 2
- **THEN** system returns AuthService and all entities within 2 relationship hops
- **AND** results are ordered by distance from starting entity

#### Scenario: Depth limit enforcement
- **WHEN** user queries graph with depth 15
- **THEN** system caps depth to maximum of 10
- **AND** returns results up to depth 10

#### Scenario: No relationships found
- **WHEN** user queries graph for entity with no relationships
- **THEN** system returns only the queried entity with empty relationships array
