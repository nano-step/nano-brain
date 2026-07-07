# Self-Review — Issue #581 (#546 increment 2: Bull queue producer/consumer)

Change-type: user-feature · Lane: normal · Branch: `feat/bull-queue-flow`
Author: kokorolx.

## Actions Taken

E1 (Redis pub/sub) was closed by #577/#578. This increment covers E2 from
#546: Bull/BullMQ couples a producer `queue.add("jobName", data)` to a
consumer `queue.process("jobName", handler)` via the job-name string, not a
call — invisible to `memory_graph`/`memory_trace`/`memory_flow`.

- **`internal/graph/js_integration_extractor.go`** — added `isBullQueueReceiver`
  (a "queue" substring naming hint — Bull has no fixed client name to key off
  of, unlike Redis's exact-match `jsRedisReceivers`) and a producer/consumer
  branch in `handleMemberExpression`: `<queueVar>.add("jobName", data)` emits
  a `queue_publish` edge (`Metadata["topic"]=jobName`); `<queueVar>.process("jobName", handler)`
  emits a `"CONSUME <jobName>"` consumer entry — the exact same shape the
  Redis/generic bus paths already produce, so `flow.Stitch` and
  `ListConsumerEntryNodesByWorkspace` require **zero changes**.
- Gated on requiring a literal string/template job-name argument (argCount≥2)
  to avoid misreading an unrelated `Set.add()`/`Map`-like `.process()` call as
  a queue op when the receiver name happens to contain "queue".
- **`internal/mcp/flow_bull_546_integration_test.go`** (new) — proves the #578
  stitching plumbing generalizes to Bull with no `tools.go`/`stitch.go`
  changes: seeds a route → producer → consumer chain, asserts `memory_flow`
  auto-links them via a `cross_service` edge without `stitch_workspaces`.

## Files Changed

- `internal/graph/js_integration_extractor.go` — Bull producer/consumer edges.
- `internal/graph/js_integration_extractor_test.go` — `TestJSIntegrationExtractor_BullQueue`:
  producer edge, consumer edge, non-queue-receiver `.add()` unaffected,
  single-arg `.process()` (no job name) not misread.
- `internal/mcp/flow_bull_546_integration_test.go` — new integration test.

## Findings Summary

- No changes needed outside the extractor — this is the direct payoff of
  #578's generic, topic-based stitching design.
- Out of scope (documented in #581, not this increment): webhook dispatch
  (URL-string coupling across services, different stitch shape) and Bull's
  single-arg `queue.process(handler)` ("process all jobs", no job-name topic
  to stitch on).
- **Red-green proven**: reverted the extractor change only (kept the new
  tests), confirmed `queue.add`/`queue.process` sub-tests FAIL
  (`expected at least one EdgeIntegration edge`); reapplied, confirmed PASS.
  The two guard sub-tests (non-queue receiver, single-arg process) passed in
  both states, proving they don't just trivially pass.
- No regression: `go test -race -short ./...` all green;
  `go test -tags=integration ./internal/graph/... ./internal/mcp/...` green.

## Resolution Status

- In scope resolved. No critical/major issues.
- `go build ./...` clean.
- Integration (nanobrain_test): new Bull stitch test PASS; full graph+mcp
  integration suite PASS (excluding the pre-existing, unrelated #580).

## Gemini Verification Triage

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| js_integration_extractor.go:441 (MEDIUM) — reorder condition so `methodName` check short-circuits before `isBullQueueReceiver`'s string allocation, since this branch runs on every member expression | VALID | Correct: `isBullQueueReceiver` calls `strings.ToLower` unconditionally; checking the cheap method-name equality first avoids that allocation on the vast majority of calls (console.log, array.length, etc.) that never reach this branch. | FIXED — swapped to `(methodName == "add" \|\| methodName == "process") && isBullQueueReceiver(receiverName)`. |
