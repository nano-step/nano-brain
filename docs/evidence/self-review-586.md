# Self-review — #586 (top-level pub/sub subscribers dropped by integration extractors)

Branch: `fix/586-toplevel-subscriber`. Change-type: bug-fix. Lane: normal (3 extractors + tests).

## Summary

The JS, Python, and Go integration-edge extractors each guarded their call walk
with `source := enclosingFunc(...); if source == "" { return }`, dropping every
call made at module top level (outside any named function). This defeated the
pub/sub producer→consumer stitcher (#546/#578) for the most common real
subscription pattern — a connect-then-subscribe bootstrap registered at module
scope, e.g. `sub.on('connect', () => { sub.subscribe('channelX', ...) })` — so no
`CONSUME channelX` node was ever created and the subscriber was invisible.

Fix: attribute top-level calls to a synthetic `<file>::<module>` source symbol,
then via a deferred `keepTopLevelCoupling` filter keep only the pub/sub coupling
edge kinds (`queue_publish`, `queue_consumer`, `cache_pubsub`) and drop HTTP/cache
ops. `defer` is required because every emitting switch branch returns early, so a
filter placed at the end of the callback would be unreachable.

## Scope decision

Rescue pub/sub only; keep dropping top-level HTTP/cache. This traces directly to
the issue (subscribers), preserves the two pre-existing `*_TopLevelCall_NoEdge`
tests (both HTTP), and matches how `.on()`/`.subscribe()` already behave *inside*
functions. Verified by grep that only these three extractors carry the
`source == ""` guard — no sibling (express/nestjs are TS → JS extractor; Ruby
integration edges use a DSL path) was missed.

## Response shape

N/A — no API/response struct changed. Output remains `([]Edge, error)`; only the
set of emitted edges changed.

## Staged files

`internal/graph/integration_extractor.go` (shared helper + guard),
`internal/graph/js_integration_extractor.go`,
`internal/graph/python_integration_extractor.go`,
plus the three matching `_test.go` files and `docs/evidence/*`. No other files;
no `.opencode/`, no lockfiles.

## Verification

- `CGO_ENABLED=0 go build ./...` OK; `go vet ./internal/graph/` clean.
- `go test -race -short ./...` — 31 packages ok, 0 failures.
- New tests pass; pre-existing `*_TopLevelCall_NoEdge` (top-level HTTP → no edge)
  still pass, confirming no new init noise.
- `keepTopLevelCoupling` uses a three-index slice `edges[:before:before]` so the
  filtered tail is never clobbered mid-loop; the `defer` closes over the per-call
  `before`/`edges` and fires on every early-return branch.

## Notes

- Accepted, pre-existing behavior: `.on('connect', ...)` yields a dangling
  `CONSUME connect` node. It never matches a publisher (harmless), and `.on`
  already does this inside functions today — suppressing lifecycle events would
  be a new denylist the issue didn't request.
- Continues the #546/#542 dogfooding lineage (event-driven coupling under-modeled).
