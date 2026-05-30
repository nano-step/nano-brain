# Self-Review: US-9.1 Event Bus Foundation

**Reviewer:** Oracle (gate 2.4)
**Implementer:** Sisyphus-Junior
**Date:** 2026-05-30
**Commit:** (pending — pre-PR review)

## Verdict: PASS

All 11 acceptance criteria verified independently. No critical or blocking issues found. Implementation is clean, race-free under `-race`, and faithfully implements the locked API from `design.md` and the backpressure contract from `spec.md`.

## Per-Criterion Table

| AC | Description | Status | Evidence |
|----|-------------|--------|----------|
| 1 | Zero internal deps | **PASS** | `grep -E '"github.com/nano-brain/nano-brain/internal/' internal/eventbus/*.go` returns empty (exit 1). Test files also clean. |
| 2 | Public API matches design.md | **PASS** | Event struct fields match (Type, Workspace, Payload json.RawMessage, TS time.Time). Publisher interface, Bus struct, New(), Publish(), Subscribe(), Close() all match signatures. Two exported test-support vars (LagTickerInterval, Now) not in design — see Medium #1. |
| 3a | Publish without subscribers non-blocking | **PASS** | `TestPublishWithoutSubscribersIsNonBlocking`: 1000 publishes, asserts <100ms. Implementation uses `select { case b.incoming <- e: default: }`. |
| 3b | Workspace filter at READ site | **PASS** | `matchesWorkspace` called in `fanoutEvent` (dispatch site, not Publish). Matrix verified: global→all, sub("")→all, sub("X")→X+global, sub("X")→!Y. Test confirms with 3 subscribers × 3 event types. |
| 3c | Unsubscribe correct | **PASS** | `sync.Once` makes closer idempotent. Checks map existence before closing channel to prevent double-close with `Close()`. Test verifies stop-receiving + idempotent call. |
| 3d | Drop-newest backpressure | **PASS** | `select { case s.ch <- e: default: s.dropped.Add(1) }` — newest event dropped, old events preserved. Test fills 69 events into 64-capacity buffer, verifies dropped≥1, reads back first 64. |
| 3e | Lag event emitted within 5s | **PASS** | `runLagTicker` at 5s interval. `emitLagEvents` does atomic swap-to-zero on dropped counter, constructs valid JSON payload `{"dropped":N,"since_ts":"<RFC3339>"}`, non-blocking send. Test overrides interval to 50ms, verifies lag event with dropped=3, counter reset. |
| 3f | Graceful shutdown | **PASS** | `Close()` via `closeOnce.Do`: cancels ctx (stops goroutines), drains incoming, closes all subscriber channels under Lock. Test verifies: all 5 channels closed within 500ms, Publish-after-Close no panic, Close-after-Close no panic, closers-after-Close no panic. |
| 3g | Concurrency stress | **PASS** | `TestConcurrentPublishersRaceFree`: 100 publishers × 100 events + 10 subscribers. `go test -race` passes with 0 data races. (Amended from 1000×1000 per task decision.) |
| 4 | `go build ./...` | **PASS** | Exit code 0. Reviewer ran independently. |
| 5 | `go vet ./internal/eventbus/...` | **PASS** | Exit code 0, clean. Reviewer ran independently. |

## Critical Findings (blocking)

None.

## Medium Findings (should fix but not blocking)

1. **Exported test-support vars `LagTickerInterval` and `Now`** are not in the locked API from `design.md`. They're pragmatic testability hooks (LagTickerInterval is copied into Bus at construction; Now is a time source). Low risk since they're documented as test overrides, but they expand the public API surface. Consider making them unexported and using functional options or a test-only build tag in a future story.

2. **Stress test scale**: AC3g specifies "1000×1000" but test uses 100×100 per task amendment. The amendment is reasonable (100×100 = 10K publishes under `-race` is sufficient for race detection), but the story doc AC text should be updated to match. Currently ambiguous.

## Minor Findings (nits)

1. `itoa()` helper in test could use `strconv.Itoa` — trivial, doesn't affect correctness.
2. `bus_test.go` uses white-box package (`package eventbus` not `eventbus_test`) — intentional for `testSubscriberDropped` access. Acceptable.
3. Subscribe-after-Close is not tested — subscriber would never receive events and channel would never close. Not a bug (caller error), but could merit a doc comment on `Subscribe()`.

## Verification Commands Run by Reviewer

```
$ grep -E '"github.com/nano-brain/nano-brain/internal/' internal/eventbus/*.go | grep -v _test.go → empty
$ grep -E '"github.com/nano-brain/nano-brain/internal/' internal/eventbus/*.go → empty  
$ go build ./... → exit 0
$ go vet ./internal/eventbus/... → exit 0
$ go test -race -v ./internal/eventbus/... → all 7 tests PASS, 0 data races
```

VERIFIED
