## ADDED Requirements

### Requirement: Debugging search mode
The system SHALL support a `mode=debugging` parameter on `memory_search` and `memory_query` MCP tools that runs parallel searches across code, sessions, and config collections.

#### Scenario: Debugging mode returns merged results with source labels
- **WHEN** agent calls `memory_search(query="stripe payment wrong tax", mode="debugging")`
- **THEN** the system runs 3 parallel searches: code, session (with debug terms appended), and config
- **THEN** results from all 3 searches are merged using existing RRF fusion
- **THEN** each result includes a `source` field: `"code"`, `"session"`, or `"config"`

#### Scenario: Debugging mode preserves existing behavior when omitted
- **WHEN** agent calls `memory_search(query="stripe payment wrong tax")` without `mode`
- **THEN** behavior is identical to current implementation (no parallel search, no source labels)

#### Scenario: Debugging mode handles partial failures gracefully
- **WHEN** agent calls `memory_search(query="...", mode="debugging")`
- **AND** one of the 3 parallel searches fails or times out (2s timeout)
- **THEN** results from the successful searches are returned
- **THEN** failed search sources are omitted from results (not included as empty)

#### Scenario: Debugging mode with max_results distributes across sources
- **WHEN** agent calls `memory_search(query="...", mode="debugging", max_results=10)`
- **THEN** the system allocates results roughly evenly across code, session, and config (e.g., 4+3+3)
- **THEN** total results do not exceed `max_results`

### Requirement: Source label on search results
Each search result SHALL include a `source` field indicating which collection it came from.

#### Scenario: Code results labeled as source=code
- **WHEN** a result comes from the code collection
- **THEN** the result includes `"source": "code"`

#### Scenario: Session results labeled as source=session
- **WHEN** a result comes from the session or session-summary collection
- **THEN** the result includes `"source": "session"`

#### Scenario: Config results labeled as source=config
- **WHEN** a result comes from the config or memory collection
- **THEN** the result includes `"source": "config"`
