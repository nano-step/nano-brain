## ADDED Requirements

### Requirement: LLM-driven consolidation decisions
When consolidation is enabled and a new memory is written, the system SHALL use an LLM to compare the new memory against existing similar memories and return a structured decision: ADD (new memory), UPDATE (merge with existing), DELETE (contradicts existing), or NOOP (already known).

#### Scenario: New memory with no similar existing memories
- **WHEN** `memory_write` is called with content "Project uses Redis for caching"
- **AND** consolidation is enabled
- **AND** no existing memories have similarity score above threshold
- **THEN** the LLM returns `{"action": "ADD", "reason": "No similar memories found"}`
- **AND** the new memory is inserted without modification

#### Scenario: New memory updates existing memory
- **WHEN** `memory_write` is called with content "Project switched from MySQL to PostgreSQL"
- **AND** consolidation is enabled
- **AND** an existing memory states "Project uses MySQL for the database"
- **THEN** the LLM returns `{"action": "UPDATE", "reason": "Database technology changed", "mergedContent": "Project uses PostgreSQL for the database (migrated from MySQL)", "targetDocId": 42}`
- **AND** the existing memory is marked as superseded
- **AND** the merged content is inserted as a new memory

#### Scenario: New memory contradicts and deletes existing memory
- **WHEN** `memory_write` is called with content "We removed Redis from the project"
- **AND** consolidation is enabled
- **AND** an existing memory states "Project uses Redis for caching"
- **THEN** the LLM returns `{"action": "DELETE", "reason": "Redis is no longer used", "targetDocId": 43}`
- **AND** the existing memory is marked as inactive (active=0)
- **AND** the new memory is inserted

#### Scenario: New memory is duplicate of existing
- **WHEN** `memory_write` is called with content "We use PostgreSQL"
- **AND** consolidation is enabled
- **AND** an existing memory states "Project uses PostgreSQL for the database"
- **THEN** the LLM returns `{"action": "NOOP", "reason": "Information already captured in existing memory"}`
- **AND** no new memory is inserted

### Requirement: Consolidation runs asynchronously
The consolidation process SHALL run in the background after `memory_write` returns. The MCP tool response SHALL NOT be blocked by LLM processing.

#### Scenario: memory_write returns before consolidation completes
- **WHEN** `memory_write` is called with consolidation enabled
- **THEN** the tool returns success within 100ms
- **AND** consolidation processing begins in background
- **AND** consolidation completes within 30 seconds (depending on LLM latency)

#### Scenario: Consolidation queue persists across restarts
- **WHEN** the MCP server is restarted while consolidation jobs are pending
- **THEN** pending jobs are recovered from the queue table
- **AND** consolidation resumes after restart

### Requirement: Embedding-based candidate selection
Before invoking the LLM, the system SHALL use vector search to find the top N most similar existing memories as consolidation candidates.

#### Scenario: Candidate selection with maxCandidates=5
- **WHEN** consolidation is triggered for a new memory
- **AND** config specifies `maxCandidates: 5`
- **THEN** vector search returns at most 5 similar documents
- **AND** only these candidates are included in the LLM prompt

#### Scenario: No candidates found
- **WHEN** consolidation is triggered for a new memory
- **AND** vector search returns zero results above similarity threshold
- **THEN** the LLM is called with empty candidate list
- **AND** the LLM returns ADD decision

### Requirement: Consolidation configuration
The system SHALL support consolidation configuration with provider, model, enabled flag, and maxCandidates.

#### Scenario: Consolidation disabled by default
- **WHEN** no `consolidation` section exists in config
- **THEN** consolidation is disabled
- **AND** `memory_write` inserts documents without LLM comparison

#### Scenario: Consolidation enabled with Ollama
- **WHEN** config contains `consolidation: { enabled: true, provider: "ollama", model: "llama3.2" }`
- **THEN** consolidation uses Ollama API at configured URL
- **AND** the specified model is used for decisions

#### Scenario: Consolidation enabled with OpenAI-compatible
- **WHEN** config contains `consolidation: { enabled: true, provider: "openai", url: "https://api.openai.com", apiKey: "sk-...", model: "gpt-4o-mini" }`
- **THEN** consolidation uses OpenAI-compatible API
- **AND** the specified model is used for decisions

### Requirement: Graceful degradation on LLM failure
When the LLM provider is unavailable or returns an error, consolidation SHALL fail gracefully without affecting the original memory write.

#### Scenario: LLM provider unreachable
- **WHEN** consolidation is triggered
- **AND** the LLM provider is unreachable (network error, timeout)
- **THEN** the consolidation job is logged as failed
- **AND** the original memory remains inserted
- **AND** no retry is attempted for this job

#### Scenario: LLM returns malformed response
- **WHEN** consolidation is triggered
- **AND** the LLM returns non-JSON or invalid schema
- **THEN** the consolidation job is logged as failed with parse error
- **AND** the original memory remains inserted

### Requirement: Consolidation logging
All consolidation decisions SHALL be logged with full context for debugging and auditing.

#### Scenario: Successful consolidation logged
- **WHEN** consolidation completes with any action (ADD/UPDATE/DELETE/NOOP)
- **THEN** a log entry is created with: timestamp, new memory hash, action, reason, target document (if applicable), LLM model used

#### Scenario: Failed consolidation logged
- **WHEN** consolidation fails due to LLM error
- **THEN** a log entry is created with: timestamp, new memory hash, error message, LLM model used
