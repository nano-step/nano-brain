# Deferred items — Phase 18 (issue #588)

## Pre-existing, out-of-scope integration test failure

`TestMemoryWakeUp_OnlyReturnsMemoryAndSessionSummaryDocs`
(`internal/mcp/tools_wakeup_integration_test.go:102`) fails against
`nanobrain_test`/:3199-style isolated schema, unrelated to this phase's
`hypothetical` param work:

```
recent_memories len = 2, want 3; got: [{Title:memory-doc-2} {Title:memory-doc-1}]
```

Evidence it's out of scope:
- `git diff 0f643ed HEAD -- internal/mcp/tools_wakeup_integration_test.go` is empty — this
  phase never touched the file.
- Fails identically in isolation (`-run TestMemoryWakeUp_OnlyReturnsMemoryAndSessionSummaryDocs`),
  ruling out cross-test pollution from the two new HyDE/hypothetical unit tests.
- Concerns `memory_wake_up`, not `memory_query`/`HybridSearch`/HyDE.

Not fixed here per the Scope Boundary rule (only auto-fix issues directly caused by
this task's changes). Needs separate triage — possibly a `memory_wake_up` recency/
limit regression from an unrelated prior change.
