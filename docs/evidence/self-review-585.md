# Self-review — #585 (flow fanout cap drops integration/reconcile edges)

Branch: `fix/585-flow-fanout-continue`. Change-type: bug-fix. Lane: tiny.

## Summary
`internal/flow/builder.go`: the calls fanout cap used `break` in two out-edge scan loops (the main loop and the reconcile-target inner loop). Integration and reconcile edges are handled by their own branches earlier in the loop (they are exempt from the calls fanout cap), but `break` exited the whole scan the moment the (maxFanout+1)th **calls** edge was seen — silently dropping any integration/reconcile edge positioned AFTER it in the slice. Changed both `break`→`continue`: the cap now skips only excess calls edges and keeps scanning, so exempt edges still emit.

## Response shape
N/A — pure function (`BuildFlow`), no API/response struct changed.

## Staged files
`internal/flow/builder.go` (2-line logic change + comments), `internal/flow/builder_test.go` (new regression test), `docs/evidence/*`. No other files.

## Verification
- `CGO_ENABLED=0 go build ./...` OK.
- `go test -race -count=1 ./internal/flow/` — all pass, incl. new `TestFanoutCapKeepsIntegrationEdges` (a node with 3 calls edges + a trailing integration edge, maxFanout=2 → asserts calls capped at 2 AND the integration node survives). This test fails on the old `break` (integration edge dropped) and passes on `continue` — it directly guards the regression.
- Existing `TestFanoutCap` still passes (the calls cap itself is unchanged).

## Notes
- Termination unaffected: `continue` iterates a finite slice; once fanout is maxed, further calls edges are skipped while integration/reconcile still emit.
- This is the flow-side of the #546/#542 dogfooding lineage (event-driven edges under-modeled). Pure-function fix; the unit test is the definitive e2e for `BuildFlow`.
