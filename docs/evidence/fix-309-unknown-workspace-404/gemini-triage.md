# Gemini Review Triage — PR #310

## Cycle 1 — both findings accepted

### Finding 1 HIGH — Redundant DB query in middleware chain
**Verdict: ACCEPTED**

Production wires both middlewares with s.db. Without coordination, GetWorkspaceByHash runs TWICE per write request — once in workspaceMiddleware, once in workspaceRegisteredMiddleware.

**Fix:** Set `c.Set("workspace_validated", true)` after the first successful lookup. The second middleware checks this flag and skips its own lookup when set. ~3 LOC change, preserves behavior, eliminates the duplicate roundtrip.

### Finding 2 HIGH — Test masking the production code path
**Verdict: ACCEPTED**

The pre-existing `chainWorkspaceMiddlewares` test passed `nil` to workspaceMiddleware, masking that in production the FIRST middleware (with db) now returns 404 before workspaceRegisteredMiddleware (which returns 400/`workspace_not_registered`) is reached.

**Implicit fix (no test code change needed):** The Finding 1 fix removes the masking — production code path now passes through cleanly because workspaceRegisteredMiddleware short-circuits via the flag. The test still asserts the 400/`workspace_not_registered` semantics correctly because it passes nil (skipping first validation), exercising the second middleware in isolation.

## Cycle 1 verification

- go build ./... PASS
- go test -race -short ./internal/server/... PASS
- go test -race -short ./... PASS
- go vet clean

Both findings real, both accepted, ~5 LOC additional. Cycle 1 complete.
