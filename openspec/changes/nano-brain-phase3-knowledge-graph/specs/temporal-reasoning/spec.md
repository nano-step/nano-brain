## ADDED Requirements

### Requirement: Track first-learned timestamp
The system SHALL record `first_learned_at` timestamp when an entity or fact is first extracted from a memory. This timestamp SHALL NOT be updated on subsequent mentions of the same entity/fact.

#### Scenario: Record first learning
- **WHEN** entity "Redis" is extracted for the first time
- **THEN** system sets first_learned_at to current timestamp

#### Scenario: Preserve first-learned on re-mention
- **WHEN** entity "Redis" is mentioned in a new memory
- **AND** entity already exists with first_learned_at = "2024-01-15"
- **THEN** system keeps first_learned_at as "2024-01-15"
- **AND** system updates last_confirmed_at to current timestamp

### Requirement: Track last-confirmed timestamp
The system SHALL record `last_confirmed_at` timestamp when an entity or fact is mentioned in a memory. This timestamp SHALL be updated each time the entity/fact is re-confirmed.

#### Scenario: Update confirmation timestamp
- **WHEN** existing entity "Redis" is mentioned in new memory
- **THEN** system updates last_confirmed_at to current timestamp

#### Scenario: Initial confirmation equals first-learned
- **WHEN** new entity is extracted
- **THEN** both first_learned_at and last_confirmed_at are set to current timestamp

### Requirement: Contradiction detection
The system SHALL detect when new memory content contradicts existing knowledge. Contradiction detection SHALL integrate with Phase 2 consolidation process. Detected contradictions SHALL be flagged with `contradicted_at` timestamp and reference to contradicting memory.

#### Scenario: Detect direct contradiction
- **WHEN** existing memory states "We use Redis for caching"
- **AND** new memory states "We switched from Redis to Memcached for caching"
- **THEN** system flags contradiction on the Redis-caching relationship
- **AND** system records contradicted_at timestamp
- **AND** system references the new memory as contradiction source

#### Scenario: No contradiction for additions
- **WHEN** existing memory states "We use Redis for caching"
- **AND** new memory states "We also use Redis for session storage"
- **THEN** system does NOT flag contradiction (this is additional information)

#### Scenario: Contradiction detection requires consolidation
- **WHEN** Phase 2 consolidation is disabled
- **THEN** contradiction detection is skipped
- **AND** system logs warning about missing prerequisite

### Requirement: Timeline view of knowledge evolution
The system SHALL provide chronological timeline of how knowledge about a topic evolved. Timeline entries SHALL include: timestamp, memory summary, change type (new, updated, contradicted).

#### Scenario: Generate topic timeline
- **WHEN** user requests timeline for topic "Redis"
- **THEN** system returns chronological list of memories mentioning Redis
- **AND** each entry includes timestamp and change indicator

#### Scenario: Timeline shows contradictions
- **WHEN** timeline includes a contradicted fact
- **THEN** entry shows "contradicted" change type
- **AND** entry references the contradicting memory

#### Scenario: Empty timeline for unknown topic
- **WHEN** user requests timeline for topic with no memories
- **THEN** system returns empty timeline array

### Requirement: Contradiction confidence scoring
The system SHALL include confidence score (0.0-1.0) with contradiction flags. High confidence (>0.8) indicates clear contradiction. Low confidence (<0.5) indicates possible but uncertain contradiction.

#### Scenario: High confidence contradiction
- **WHEN** memories directly contradict (e.g., "use X" vs "don't use X")
- **THEN** contradiction confidence is > 0.8

#### Scenario: Low confidence contradiction
- **WHEN** memories partially conflict (e.g., different versions mentioned)
- **THEN** contradiction confidence is < 0.5
- **AND** system still flags for user review
