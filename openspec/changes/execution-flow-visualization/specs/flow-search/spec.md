## ADDED Requirements

### Requirement: Materialize a searchable summary per endpoint
The system SHALL materialize one searchable document per detected HTTP endpoint via a workspace-level pass that runs after indexing settles, written through the normal chunk and embedding pipeline so it is discoverable by existing search tools.

#### Scenario: Endpoint flow is searchable
- **WHEN** the workspace contains `POST /api/topup` and indexing has settled
- **THEN** a document titled `"POST /api/topup flow"` tagged `flow` exists and is returned by `memory_query("topup flow")`

#### Scenario: Summary content
- **WHEN** a flow-summary document is generated
- **THEN** its body contains the entry, the ordered in-process chain, the externals, and node roles, and does NOT embed the Mermaid diagram (rendered on demand)

### Requirement: Keep flow summaries fresh
The materialization pass SHALL upsert summaries for current endpoints and SHALL delete summaries for endpoints that no longer exist in the graph.

#### Scenario: Route removed
- **WHEN** a route is deleted from source and the workspace is re-indexed
- **THEN** the corresponding flow-summary document is deleted

#### Scenario: Handler chain changed
- **WHEN** a handler's downstream call chain changes and the workspace is re-indexed
- **THEN** the flow-summary document for that endpoint reflects the new chain

### Requirement: Workspace-level materialization
Flow materialization SHALL operate per workspace using the full edge set, not per file, because flows span multiple files.

#### Scenario: Cross-file flow
- **WHEN** the route registration and the handler implementation live in different files
- **THEN** the materialized flow still links the route to the handler's call chain

### Requirement: Isolate flow documents in a dedicated collection
Flow-summary documents SHALL be written to a dedicated `flows` collection so they do not skew or silently flood ordinary search results.

#### Scenario: Default search not flooded
- **WHEN** a workspace has many endpoints and a user runs an ordinary `memory_query`
- **THEN** flow-summary documents are discoverable but do not dominate/skew results because they are isolated in the `flows` collection

### Requirement: Single-flight materialization
Materialization for a given workspace SHALL NOT run concurrently with itself; overlapping triggers SHALL be coalesced.

#### Scenario: Rapid successive edits
- **WHEN** multiple settle ticks fire while a materialization pass is in progress
- **THEN** the in-progress pass completes without racing on flow-doc upsert/delete, and at most one additional pass is scheduled afterward

### Requirement: Gated by configuration
Flow extraction and materialization SHALL be controlled by a config flag and SHALL be fully inert when disabled.

#### Scenario: Disabled
- **WHEN** flow indexing is disabled in config
- **THEN** no `http`/`middleware` edges are produced and no flow documents are created or deleted
