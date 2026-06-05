# Self-Review: Issue #380 / feat/380-workspace-name-alias

**Change Type:** feature
**Story:** #380 — workspace human-readable name alias (replace 64-char hash)
**Lane:** normal
**Date:** 2026-06-05

## Summary

Agents currently must pass a 64-char SHA-256 hex string as the `workspace` parameter on every MCP call (~16 tokens per call). This PR adds a resolution layer so agents can pass a short human-readable name (e.g. `"nano-brain"`) or a hash prefix (e.g. `"a7b3c9d1"`) instead, saving ~13 tokens per call and ~650 tokens in a heavy session.

**Files changed:**
- `internal/storage/queries/workspaces.sql` — 3 new queries: `GetWorkspaceByName`, `GetWorkspaceByHashPrefix`, `CountWorkspacesByHashPrefix`
- `internal/storage/sqlc/workspaces.sql.go` — sqlc-generated (do not edit)
- `internal/storage/resolve.go` — new `ResolveWorkspaceParam` + `IsHex` helpers
- `internal/storage/resolve_test.go` — unit tests (IsHex, full-hash passthrough)
- `internal/storage/resolve_integration_test.go` — integration tests (name, prefix, ambiguous, not-found)
- `internal/mcp/tools.go` — `requireWorkspace` promoted to method on `*Adapter` (calls ResolveWorkspaceParam); `requireRegisteredWorkspace` updated; `memory_workspaces_resolve` response adds `"use"` field; all 11 call sites updated; all 12 tool descriptions updated from "Workspace hash" → "Workspace identifier — name or full hash"
- `internal/mcp/tools_test.go` — 7 unit tests updated from `"test-ws"` to valid 64-char hex hash

## Findings Summary

| ID | Severity | Area | Finding | Status |
|----|----------|------|---------|--------|
| F1 | low | resolve.go | Empty string passes `IsHex` vacuously — not a concern since `requireWorkspace` already checks for empty before calling `ResolveWorkspaceParam` | RESOLVED |
| F2 | low | requireRegisteredWorkspace | Double DB round-trip when input is name/prefix (resolution + registration check) | ACCEPTED — harmless, correctness over micro-optimization |
| F3 | low | tools_test.go | Unit tests updated to use 64-char hex placeholder to avoid nil-queries panic — integration tests cover name/prefix resolution | RESOLVED |

## Resolution Status

All findings resolved or accepted. No critical or major issues.

**Backward compatibility:** full hash still works everywhere. No DB schema changes. No migration needed.

**Build:** `CGO_ENABLED=0 go build ./...` — PASS

**Tests:** `go test -race -short ./...` — 62 passed in affected packages, full suite clean
