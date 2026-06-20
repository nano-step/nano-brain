## EXISTING Implementation

Cross-workspace stitching is **already implemented** and wired end-to-end. This spec documents the existing behavior for accuracy. No new work is required.

### Requirement: Cross-workspace topic matching
The system SHALL match `publish("topic")` edges in workspace A to `consume("topic")` edges in workspace B, producing `cross_service` flow edges linking publisher to consumer.

#### Scenario: Matching publish/consume across workspaces
- **WHEN** workspace A has an integration edge with `metadata.topic = "trade.created"`
- **AND** workspace B has a consumer entry node `CONSUME trade.created`
- **THEN** `Stitch()` produces a `FlowEdge{Kind: "cross_service", CrossServiceWorkspace: <B-hash>}` from the publisher node to the consumer node

#### Scenario: Self-match is excluded
- **WHEN** both publisher and consumer are in the same workspace
- **THEN** no `cross_service` edge is produced

#### Scenario: Noise topics are skipped
- **WHEN** a publish edge has `metadata.topic = "<var:event_topic>"`
- **THEN** no `cross_service` edge is produced for that edge

#### Scenario: No matching consumer
- **WHEN** a publish edge has no matching consumer in any target workspace
- **THEN** no `cross_service` edge is produced

### Implementation details

- **Signature:** `Stitch(ctx, publishEdges []graph.Edge, targetWorkspaces []string, querier StitchQuerier) []FlowEdge`
- **Location:** `internal/flow/stitch.go` (88 lines)
- **Trigger:** Request-driven via `stitch_workspaces` field in request body (NOT config-gated)
- **Matching:** Exact string match on topic name (case-sensitive, no toggle)
- **Topic source:** `Metadata["topic"]` for publishers; `CONSUME `/`ON ` prefix parsing from source node for consumers
- **Noise filter:** Skips topics starting with `<var:`
- **Workspace hash:** Truncated to first 8 characters in `CrossServiceWorkspace`
- **DB query:** `ListConsumerEntryNodesByWorkspace` in `storage/queries/graph.sql`

### Wiring

- **REST:** `POST /api/v1/graph/flow` — `stitch_workspaces` field in request body triggers stitching (`handlers/flow.go:114-118`)
- **MCP:** `memory_flow` — `stitch_workspaces` argument triggers stitching (`mcp/tools.go:2073-2077`)
- **Mermaid:** `cross_service` edges rendered with distinct styling (`mermaid.go:132-133`)
- **Sequence:** `cross_service` edges rendered in sequence diagrams (`sequence.go:156-160`)

### Tests

- `TestStitchMatchesStringLiteralTopics` — matching topics across workspaces
- `TestStitchSkipsVarPlaceholders` — `<var:` noise filtering
- `TestStitchEmptyOnNoMatch` — no match produces empty result
- `TestStitchEmptyEdgesOrWorkspaces` — nil/empty inputs produce nil
- `TestStitchCrossServiceWorkspaceHashes` — workspace hash truncated to 8 chars

### Known limitations (not in scope for this change)

- No case-sensitivity toggle (always exact match)
- No fuzzy/regex topic matching
- No `isNoiseIntegration()` check (only `<var:` prefix check)
- No config flag to enable/disable stitching (request-driven only)
- `Metadata["event"]` is NOT read (only `Metadata["topic"]`)
