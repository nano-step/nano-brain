# PRE-WORK gate — Issue #326

**Date**: 2026-06-02
**Issue**: #326 (internal/server/handlers integration test failures)
**Lane**: tiny (2 test files, ~15 line edits)
**Change-type**: bug-fix
**Branch**: `fix/326-handlers-integration`

## Failures

1. **`events_integration_test.go:96`** — `expected state=started, got map[payload:map[state:started] ts:... type:reindex workspace:...]`
   - Root cause: `readEvent` returns the full SSE event object `{type, workspace, payload, ts}` where `payload` is the inner JSON.RawMessage. Test directly accessed `payload["state"]` but should access `payload["payload"].(map[string]any)["state"]`.

2. **`workspace_integration_test.go:127`** — `workspace with hash X not found in list`
   - Root cause: Two separate bugs in one test:
     - Test decoded response as `[]map[...]` but handler returns wrapped `{"workspaces": [...]}` (per `openspec/specs/workspaces-api-contract`).
     - Test looked up `item["workspace_hash"]` but handler returns `item["hash"]` (per `workspaceItem` struct JSON tag).

## Lane (tiny)

- 2 test files changed
- ~15 line edits total (test assertions only)
- 0 production code changes
- Risk flags: 0

## Skip justifications

- OpenSpec proposal: SKIP (tiny lane)
- smoke:e2e: SKIP — test-only file changes, no runtime surface
- Review gate: ⚠️ self-verify only (per HARNESS.md for bug-fix in test files)

## After-fix verification

```
$ go test -race -tags=integration -short ./...
ok  internal/embed
ok  internal/harvest
ok  internal/search
ok  internal/server/handlers   ← FIXED
ok  internal/storage
... (all packages PASS)
```

**Full integration suite is now GREEN.** This PR closes the last remaining gate 3.3 failure on master. Going forward, every PR can have a clean gate 3.3 PASS.
