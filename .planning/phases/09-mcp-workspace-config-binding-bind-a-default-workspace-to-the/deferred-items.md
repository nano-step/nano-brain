# Deferred Items — Phase 9

Out-of-scope discoveries logged during plan execution, per the executor's scope
boundary rule (only auto-fix issues directly caused by the current task's
changes).

## Pre-existing test failure: TestMemoryTrace_RelativeInputAndOutput

**Found during:** 09-03 Task 1/verification (`go test -race -tags=integration ./internal/mcp/... ./internal/server/...`)

**Symptom:** `panic: interface conversion: interface {} is nil, not []interface {}` at
`internal/mcp/graph_paths_integration_test.go:218`, in `TestMemoryTrace_RelativeInputAndOutput`.

**Scope check:** This test file was last modified in commit `4311500` ("feat:
execution flow visualization + search quality improvements (#423)"), well
before Phase 9's branch (`feat/mcp-workspace-config-binding`) existed. None of
Phase 9's three plans (09-01, 09-02, 09-03) touch `graph_paths_integration_test.go`
or the `memory_trace`/graph-paths code it exercises — confirmed via
`git diff --stat` across all three plan commit ranges (empty diff for that
file).

**Disposition:** Out of scope for Phase 9. Not fixed. Flagging for a separate
bugfix outside this phase's scope — likely a nil-vs-empty-slice mismatch in a
graph-paths JSON response shape that predates this branch.

**Verification impact:** Does not block Phase 9's own gates — all of Phase
9's specific verification commands
(`TestStreamableHTTP_ConnectionDefaultWorkspace`,
`TestRequireRegisteredWorkspace_UsesConnectionDefault`, `go build ./...`,
`go build -tags=integration ./...`, docs grep gate) pass cleanly in isolation.
The full-package `-tags=integration ./internal/mcp/...` run fails only because
of this unrelated pre-existing test.
