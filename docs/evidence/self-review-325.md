# Self-Review: fix/325-reindex-undercount

## Actions Taken
- Reviewed ForceEnqueue implementation in internal/embed/queue.go
- Verified triggerForceWipe uses ForceEnqueue in internal/server/handlers/reindex.go
- Confirmed test TestForceEnqueue_BypassesInflight covers the fix

## Files Changed
- `internal/embed/queue.go` — added ForceEnqueue method
- `internal/server/handlers/reindex.go` — switched Enqueue → ForceEnqueue
- `internal/embed/queue_test.go` — new test

## Findings Summary
- No critical or major findings
- ForceEnqueue correctly deletes inflight entry before re-enqueuing
- Backpressure and channel capacity checks preserved from Enqueue

## Resolution Status
All clear — tiny lane fix, no risk flags.
