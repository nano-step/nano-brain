## ADDED Requirements

### Requirement: Backfill fixtures match unified summaries

The integration backfill fixture SHALL create summary documents in the
collection selected by the production backfill query.

#### Scenario: Unified summary fixture is selected

- **WHEN** a fixture inserts a `summary://` document
- **THEN** the backfill query returns that document

### Requirement: Cleanup test preserves its historic schema scenario

The orphan-workspace cleanup test SHALL keep its pre-foreign-key schema and
SHALL insert fixture documents without columns unavailable in that schema.

#### Scenario: Orphan documents can be cleaned before FK migration

- **WHEN** the cleanup fixture migrates through version 10
- **THEN** it inserts registered and orphan documents and verifies only orphan
  rows are removed

### Requirement: Live benchmark requires explicit opt-in

The live nano-brain benchmark SHALL NOT run under `integration` alone.

#### Scenario: Normal integration test run has no live-server dependency

- **WHEN** integration tests run without the `benchmark` tag
- **THEN** the live benchmark test is excluded

### Requirement: MCP wake-up fixture follows its session filter

The integration suite SHALL model the `memory_wake_up` tool with the canonical
`sessions` collection so expected recent documents match production.

#### Scenario: Wake-up returns memory and session documents

- **WHEN** the wake-up integration test inserts a session document
- **THEN** it uses the `sessions` collection selected by the MCP tool
