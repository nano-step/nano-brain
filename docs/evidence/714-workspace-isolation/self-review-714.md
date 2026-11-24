# Self-review — issue #714 (post-review-fix pass)

Scope re-checked after applying reviewer findings: `internal/watcher/watcher.go`
(tombstone guard, workspaceGone check, nil guards), `internal/watcher/collections.go`,
`internal/server/handlers/{collection,reindex,workspace,workspace_remove,reset_workspace,routes}.go`,
`cmd/nano-brain/main.go`, `migrations/00033`, and all new/updated tests.

## What I re-verified myself

- Tombstone guard: `w.collections[key]` write now only happens when the key
  still exists; `updated` is a copy of the live entry so `detectedFrameworks`
  can't resurrect a deleted collection or clobber a re-registered one.
- `workspaceGone` loop runs under `w.mu` after the deletes, so it sees the
  post-delete state; `UnwatchWorkspace` still clears unconditionally (correct —
  the workspace is gone entirely).
- Nil-receiver guards return before touching `w.mu`; typed-nil interface from
  `routes.go` now safely no-ops.
- `RemoveCollection` logical-name skip uses the request `name` (same value used
  for `GetCollectionByName`/`DeleteCollection`), so guard and delete are
  consistent.
- `triggerForceWipe` skip is inside the per-collection loop before any DB work;
  `watcherTriggered`/`ReextractSymbolsForWorkspace` unaffected for real
  collections.
- Test message assertions use `fmt.Sprint(he.Message)` — `he.Message` is
  `interface{}` in echo; `strings.Contains` on the sprinted value matches both
  string and `*echo.HTTPError`-wrapped messages.
- `docs/openapi.json` + `internal/server/handlers/openapi.json` regenerated via
  `make generate-openapi`; drift test passes.
- Privacy: no real workspace names/paths/hashes added; test fixtures use
  `ws-tx-rollback`, `ws-reset-rollback`, `/tmp/*` only.

## Residual risks (accepted, documented in apply.md)

- In-flight `scanCollection` snapshot can upsert orphan `documents` rows for a
  workspace deleted mid-scan (bounded; graph layer fails closed via 00033).
- `aliasIndexes` remains path-keyed across unwatch (pre-existing; brief kept it).
- `RemoveCollection` has no logical-name guard on the DELETE itself (product
  decision deferred — deleting the `memory` row is permitted by design scope).
