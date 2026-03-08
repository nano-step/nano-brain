## MODIFIED Requirements

### Requirement: memory_write triggers consolidation when enabled
The `memory_write` MCP tool handler SHALL trigger background consolidation when consolidation is enabled in config. The tool response SHALL return immediately without waiting for consolidation to complete.

#### Scenario: memory_write with consolidation enabled
- **WHEN** `memory_write` is called with `{"content": "Project uses Redis"}`
- **AND** consolidation is enabled in config
- **THEN** the document is inserted immediately
- **AND** a consolidation job is enqueued
- **AND** the tool returns success with the new document ID
- **AND** consolidation runs asynchronously in background

#### Scenario: memory_write with consolidation disabled
- **WHEN** `memory_write` is called with `{"content": "Project uses Redis"}`
- **AND** consolidation is disabled or not configured
- **THEN** the document is inserted immediately
- **AND** no consolidation job is enqueued
- **AND** the tool returns success with the new document ID

#### Scenario: memory_write response includes consolidation status
- **WHEN** `memory_write` is called with consolidation enabled
- **THEN** the response includes `consolidation: "pending"` field
- **AND** the response includes the document ID for tracking

### Requirement: harvest command extracts facts when enabled
The `harvest` CLI command SHALL extract facts from session transcripts when extraction is enabled in config.

#### Scenario: harvest with extraction enabled
- **WHEN** `harvest` command is executed
- **AND** extraction is enabled in config
- **AND** new sessions are found
- **THEN** session markdown is generated and indexed
- **AND** facts are extracted from each session transcript
- **AND** extracted facts are stored as separate documents

#### Scenario: harvest with extraction disabled
- **WHEN** `harvest` command is executed
- **AND** extraction is disabled or not configured
- **THEN** session markdown is generated and indexed
- **AND** no fact extraction occurs

#### Scenario: harvest reports extraction statistics
- **WHEN** `harvest` command completes with extraction enabled
- **THEN** the output includes count of sessions processed
- **AND** the output includes count of facts extracted
- **AND** the output includes count of duplicate facts skipped

## ADDED Requirements

### Requirement: memory_consolidation_status tool
The MCP server SHALL provide a `memory_consolidation_status` tool that returns the current state of the consolidation queue and recent consolidation history.

#### Scenario: Query consolidation status
- **WHEN** `memory_consolidation_status` is called
- **THEN** the response includes: pending job count, processing job count, recent completions (last 10), recent failures (last 10)

#### Scenario: Empty consolidation queue
- **WHEN** `memory_consolidation_status` is called
- **AND** no consolidation jobs are pending or processing
- **THEN** the response shows `pending: 0, processing: 0`
- **AND** recent history is still returned if available
