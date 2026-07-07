# Self-Review — Issue #577 (#546 increment 1: Redis pub/sub flow linking)

Change-type: user-feature · Lane: normal · Branch: `feat/redis-pubsub-flow`
Author: kokorolx.

## Actions Taken

Deep-design found the event-driven infra mostly exists (subscribers extracted,
`flow.Stitch` links producers↔consumers by topic; no migration needed — reuse
`edge_type=integration` + `metadata.kind`). Fixed the two code-only gaps that
blocked the Redis pub/sub case:

- **Slice A** — `internal/graph/js_integration_extractor.go` (`handleRedisCall`
  `cache_pubsub` branch): a subscribe-like method (`jsConsumeMethods`) now emits
  a `CONSUME <topic>` consumer entry (`SourceNode="CONSUME <topic>"`,
  `TargetNode=handler`, `metadata.kind="queue_consumer"`) — matching the generic
  bus shape — instead of the old `subscribe:<topic>` leaf that
  `ListConsumerEntryNodesByWorkspace` never matched. The publish side is
  unchanged (still a topic-carrying publisher edge).
- **Slice C** — `internal/mcp/tools.go` (`memory_flow`): always include the
  current workspace in the stitch set, so an in-workspace publisher→consumer
  link surfaces without the caller passing `stitch_workspaces`. Stitch is a
  no-op when no consumer topic matches.

## Files Changed

- `internal/graph/js_integration_extractor.go` — Redis subscribe → consumer entry.
- `internal/mcp/tools.go` — auto-stitch current workspace in memory_flow.
- `internal/graph/js_integration_extractor_test.go` — 2 tests updated: they had
  codified the old `subscribe:topic`/`cache_pubsub` behavior; now assert the
  `CONSUME`/`queue_consumer` contract (legitimate contract change per #546).
- `internal/mcp/flow_pubsub_577_integration_test.go` — new: memory_flow
  auto-links publisher→consumer entry via a `cross_service` edge, no
  `stitch_workspaces` arg.

## Findings Summary

- No schema migration — the issue's literal `publishes_to`/`subscribes_to` edge
  types would need a migration + re-index (high-risk lane); the existing
  `integration` + `metadata.kind` + `flow.Stitch` path already carries the
  semantics. Bull `queue.process` extraction is a deferred follow-up.
- **Red-green proven**: Slice A via the updated extractor unit tests (real JS
  source → `CONSUME` entry); Slice C via the integration test (consumer entry +
  `cross_service` edge absent without the auto-stitch).
- No regression: publish extraction unchanged; explicit `stitch_workspaces` still
  works (ws added on top); stitch is a no-op with no topic match.
- **R88 MEDIUM (unscoped stitch → pollution + dangling/self edges) — addressed**
  before PR: (1) `filterPublishEdges` now excludes consumer entries (`queue_consumer`
  / `CONSUME `/`ON ` source) so a consumer can't stitch to itself; (2)
  `flowReachablePublishers` scopes stitching to publishers present in the built
  flow (by id or bare `::`-suffix), so unrelated workspace pub/sub isn't imported
  on every call; (3) `appendStitchedToFlow` registers BOTH endpoints so no edge
  dangles. The integration test now asserts no self-edge and no dangling endpoint.

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean; `go test -race -short ./...` all ok (graph incl. updated
  extractor tests).
- Integration (nanobrain_test): auto-stitch e2e test PASS.
- smoke:e2e: `docs/evidence/smoke-e2e-redis-pubsub-flow.md` (MCP-over-HTTP:
  publisher auto-linked to consumer entry by topic). Dev DB never touched.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
