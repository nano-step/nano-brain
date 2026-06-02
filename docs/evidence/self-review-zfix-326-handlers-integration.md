# Self-review — Issue #326 / PR (TBD)

**Date**: 2026-06-02
**Story**: 326 (handlers integration test failures)
**Lane**: tiny | **Change-type**: bug-fix
**Implementing agent**: Sisyphus orchestrator (direct edit, test-only)

## Scope

| File | Change |
|---|---|
| `internal/server/handlers/events_integration_test.go` | Replace 2 blocks of `payload["state"]` assertions with `ev["payload"].(map[string]any)["state"]` pattern (the real event wraps payload inside the SSE event object) |
| `internal/server/handlers/workspace_integration_test.go` | Change decode to `{Workspaces []map[...]}` shape (matches handler contract) + change lookup key from `"workspace_hash"` to `"hash"` (matches struct JSON tag) |

Production code: zero changes.

## Why these fixes are correct

### Fix 1 (events)

Production handler `writeSSEEvent` (`internal/server/handlers/events.go:114`) marshals the entire `eventbus.Event` struct to the SSE `data:` line:

```go
data, _ := json.Marshal(ev)   // ev = {Type, Workspace, Payload (json.RawMessage), TS}
fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data)
```

When the test unmarshals `data:` into `map[string]any`, it gets `{"type":"reindex", "workspace":"...", "payload":{"state":"started"}, "ts":"..."}`. The `payload` value itself is the nested map. The test was reading `payload["state"]` (= the outer event's "state" field, which doesn't exist) instead of `payload["payload"]["state"]`.

The reindex handler `publishReindex()` (`reindex.go:139-159`) marshals `{state, enqueued, embedded, ...}` into `eventbus.Event.Payload`, confirming the nested structure.

### Fix 2 (workspaces)

Production handler `ListWorkspaces` (`workspace.go:218`) returns `listWorkspacesResponse{Workspaces: items}` which serializes to `{"workspaces": [{"hash":"...", "root_path":"...", "name":"...", "doc_count":N, ...}]}`. 

The handler comment explicitly documents this contract:
> Field names match web/src/api/types.ts Workspace interface. Renaming these JSON tags is a breaking API change — see openspec/specs/workspaces-api-contract for the canonical contract.

The test was making two wrong assumptions: (1) bare array shape, (2) field named `workspace_hash`. Both contradicted the canonical contract.

## Validation

| Check | Before | After |
|---|---|---|
| `go test -race -tags=integration ./internal/server/handlers/...` | FAIL (2 tests) | ✅ PASS |
| Full integration suite `go test -tags=integration ./...` | FAIL (handlers) | ✅ ALL PASS |
| `go test -race -short ./...` | PASS | ✅ PASS |
| `go build ./...` | PASS | ✅ PASS |

## Backward compat

Zero. Test-file-only changes. Production behavior unchanged.

## Conclusion

This PR makes the integration test assertions match the actual production contract. After merge, harness gate 3.3 will be FULLY GREEN across all packages on master. No more `[HARNESS-OVERRIDE]` 3.3 needed for future PRs.
