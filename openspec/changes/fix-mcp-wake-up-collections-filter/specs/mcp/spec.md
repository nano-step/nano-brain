# MCP Tool: memory_wake_up

## ADDED Requirements

### Requirement: memory_wake_up MUST filter recent_memories to memory and session-summary collections

The `memory_wake_up` MCP tool MUST invoke the underlying `RecentDocuments` storage query with `Collections = ["memory", "session-summary"]`, matching the behaviour of the HTTP `POST /api/v1/wake-up` endpoint introduced by issue #338.

#### Scenario: Workspace contains memory and code documents

- **GIVEN** a registered workspace with N>0 documents in the `memory` collection AND M>0 documents in the `code` collection
- **WHEN** an MCP client calls `memory_wake_up` with that workspace hash and `limit=10`
- **THEN** the response `recent_memories` array MUST contain only documents whose `collection` is `memory` or `session-summary`
- **AND** documents in the `code` collection MUST NOT appear in `recent_memories`
- **AND** the response shape MUST be identical to the HTTP `POST /api/v1/wake-up` endpoint for the same workspace and limit

#### Scenario: Workspace only has code documents

- **GIVEN** a registered workspace with only `code` collection documents
- **WHEN** an MCP client calls `memory_wake_up` with that workspace hash
- **THEN** `recent_memories` MUST be an empty array (not null)
- **AND** `active_collections` MUST still list the `code` collection with its document count
