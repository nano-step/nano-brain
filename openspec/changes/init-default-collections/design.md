## Context

`initWorkspace` (workspace.go:42-76) already upserts three DB rows in a transaction: one `workspaces` row plus two `collections` rows for `memory` and `sessions`. Both collection paths are hardcoded to `~/.nano-brain/memory/` and `~/.nano-brain/sessions/` — **not** the user-supplied root path.

At server startup (main.go:220-237) the watcher seeding loop calls `os.Stat` on each collection path and skips any that don't exist. Because `~/.nano-brain/memory/` and `~/.nano-brain/sessions/` may not exist yet, the watcher starts with zero watched directories and never indexes anything.

The `code` collection fix is a fourth `UpsertCollection` call in the same transaction using `absPath` (the workspace root). The path always exists — it was just validated by `filepath.Abs` immediately before.

## Goals / Non-Goals

**Goals:**
- After `POST /api/v1/init`, at least one collection (named `code`) exists pointing to `absPath`.
- The watcher picks it up on the next server start via the existing seeding loop.
- The change is atomic: if any upsert fails, the whole transaction rolls back.
- Idempotent: calling init a second time re-runs upserts, leaving state consistent.

**Non-Goals:**
- Live watcher registration from `InitWorkspace` at request time. (`AddCollection` handler does this; `InitWorkspace` does not hold a `*watcher.Watcher` reference and adding one would require a handler signature change. The seeding loop on startup is sufficient.)
- Indexing files immediately during the HTTP response.
- Any change to `initResponse` JSON shape.
- Creating `~/.nano-brain/memory/` or `~/.nano-brain/sessions/` on disk.

## Decisions

### 1. Name the collection `code`

Alternatives: `workspace`, `project`, `root`, `src`.

`code` is the most descriptive for the use case (source code files) and shortest. It matches the mental model in docs and AGENTS.md snippets.

### 2. Glob pattern `**/*`

Same as `memory` and `sessions`. Fine for v1 — the watcher already filters by `maxFileSize` and will skip binary files by attempting to read them. A tighter glob (e.g., `**/*.{go,md,ts}`) is a follow-up with its own tradeoffs.

### 3. update_mode `auto`

Consistent with the other two collections. No behavior change.

### 4. Placement: fourth `UpsertCollection` in `initWorkspace`, not in the HTTP handler

Keeps all three collection upserts in one place and in the same transaction. Adding logic to the handler body would bypass the test helper (`initWorkspace`) and require duplicating transaction handling.

### 5. No `watcher.Watch` call from `InitWorkspace`

`InitWorkspace` currently accepts `WorkspaceQuerier` and `*sql.DB` — no `*watcher.Watcher`. Adding it would change the handler constructor signature used in `routes.go` and tests. The startup seeding loop (main.go:220-237) handles watcher registration on next boot. Post-init users typically start the server once; hot-registration is a nice-to-have, not a correctness requirement.

**Risk**: If the server is already running when `init` is called, the watcher won't start watching `absPath` until the next restart. The user must restart the server to trigger indexing.

**Mitigation**: Document in CLI output and README. This matches existing behavior for `collection add` (which does do live registration) — so the pattern is already known to users.

## Risks / Trade-offs

- **Large root paths**: A root path pointing to a filesystem root (`/`) or home directory would cause the watcher to attempt indexing everything. Risk is pre-existing for `AddCollection` too. Mitigation: existing `maxFileSize` guard in watcher + future improvement to warn on large directories.
- **`UpsertCollection` is idempotent**: Calling `init` twice on the same path results in the same three collections. No orphan rows. Correct.
- **Test mock impact**: `WorkspaceQuerier` interface already includes `UpsertCollection`. The mock in `workspace_test.go` must expect a fourth call. This is a test-only breaking change — not a public API change.

## Migration Plan

No DB migration needed — the `collections` table already exists (migration 0005). The change only adds a new row per workspace on next `init` call. Existing workspaces are unaffected until they re-run `init` or manually call `collection add code <path>`.

## Open Questions

- Should `init` also live-register the `code` collection into the watcher (requires adding `*watcher.Watcher` to `InitWorkspace`)? Deferred — post-init restart is acceptable for v1 and avoids handler signature churn.
- Should the glob pattern be configurable in the `init` request? Deferred — `**/*` is the sensible default; users can `collection add` a custom collection afterward.
