## ADDED Requirements

### Requirement: Related memory suggestions on write
The system SHALL automatically surface related memories when `memory_write` adds new content. The system SHALL use vector similarity search to find related memories. The system SHALL return up to `maxSuggestions` related memories (configurable, default 3).

#### Scenario: Surface related memories on write
- **WHEN** user writes memory about "Redis configuration for caching"
- **AND** existing memories mention Redis or caching
- **THEN** system returns up to 3 related memories as "see also" suggestions
- **AND** suggestions are ordered by relevance score

#### Scenario: No related memories found
- **WHEN** user writes memory about a completely new topic
- **AND** no existing memories have vector similarity above threshold
- **THEN** system returns empty suggestions array

#### Scenario: Proactive surfacing disabled
- **WHEN** config has `proactive.enabled: false`
- **THEN** system does NOT run vector search after write
- **AND** system returns no suggestions

### Requirement: Related memory suggestions on change detection
The system SHALL automatically surface related memories when `code_detect_changes` finds changed symbols. For each changed symbol, the system SHALL search for memories mentioning that symbol or related concepts.

#### Scenario: Surface memories for changed code
- **WHEN** `code_detect_changes` detects changes to `AuthService.ts`
- **AND** existing memories mention AuthService or authentication
- **THEN** system returns related memories alongside change detection results

#### Scenario: Multiple changed files
- **WHEN** `code_detect_changes` detects changes to 5 files
- **THEN** system aggregates related memories across all changed files
- **AND** deduplicates suggestions that appear for multiple files

### Requirement: Configurable suggestion limits
The system SHALL respect `proactive.maxSuggestions` configuration for limiting returned suggestions. Default value SHALL be 3. Maximum value SHALL be 10.

#### Scenario: Custom suggestion limit
- **WHEN** config has `proactive.maxSuggestions: 5`
- **AND** 10 related memories exist
- **THEN** system returns only top 5 most relevant memories

#### Scenario: Limit exceeds maximum
- **WHEN** config has `proactive.maxSuggestions: 20`
- **THEN** system caps to maximum of 10 suggestions

### Requirement: Lightweight vector-based matching
The system SHALL use vector similarity search for finding related memories. The system SHALL NOT use LLM calls for proactive surfacing. Search latency SHALL be under 100ms for typical collections (< 10,000 documents).

#### Scenario: Fast suggestion retrieval
- **WHEN** collection has 5,000 documents
- **AND** user writes new memory
- **THEN** related memory suggestions are returned within 100ms

#### Scenario: Vector search only
- **WHEN** proactive surfacing runs
- **THEN** system uses only vector similarity (no LLM API calls)
- **AND** no additional tokens are consumed
