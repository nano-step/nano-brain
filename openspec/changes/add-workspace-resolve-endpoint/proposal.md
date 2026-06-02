## Why

Every nano-brain operation (HTTP, CLI, MCP) requires a `workspace_hash`, but agents have no clean way to discover the hash for the current project. Today the only options are:

1. **Hardcode** the hash — brittle, breaks across machines and containers
2. **Call `POST /api/v1/init` every session** — works (UPSERT is idempotent) but conflates "register" with "look up" and is wasteful
3. **Call `GET /api/v1/workspaces` and filter client-side** — works but requires listing every workspace

An audit of the agent-facing docs (`AGENTS.md` OPENCODE-MEMORY block, `skills/nano-brain/AGENTS_SNIPPET.md`) also found:

- Wrong endpoint path (`/api/query` instead of `/api/v1/query`) — agents get 404 following the snippet
- Missing `workspace` field in every `curl` example — agents get 400 `workspace_required`
- No first-time-user bootstrap flow documented — agents have to grep code to figure out the init step

A QA round (RRI-T) flagged the same gap as a recurring UX problem.

## What Changes

This change adds a small, **read-only** workspace resolution surface and rewrites the agent-facing skill to use a clear phase-based onboarding flow.

### Backend
- **`internal/server/handlers/workspace_resolve.go`** (new) — `POST /api/v1/workspaces/resolve` handler. Pure read-only. Computes hash via existing `storage.WorkspaceHash` (SHA-256 of absolute path), checks DB for registration, returns `{workspace_hash, root_path, name, registered}`. Never auto-registers.
- **`internal/server/routes.go`** — register route in the public group (alongside `/init`, `/workspaces`). NOT through `workspaceMiddleware` (input is a path, not a hash).
- **`internal/storage/queries/workspaces.sql`** — add `GetWorkspaceByHash` query if not already present (re-uses existing UPSERT, only needs lookup-by-PK).
- **`internal/server/handlers/workspace_resolve_test.go`** (new) — table-driven unit tests (registered/unregistered/empty path/relative path/abs path).
- **`internal/server/handlers/workspace_resolve_integration_test.go`** (new) — `//go:build integration` real-PG test against `testutil.SetupTestDB`.

### CLI
- **`cmd/nano-brain/workspaces.go`** — add `current` subcommand to the existing `runWorkspacesCmd` dispatcher. Auto-detects CWD via `os.Getwd()` (or `--path=<p>`), calls `/api/v1/workspaces/resolve`, prints result.
  - Default: print hash to stdout
  - `--export`: print `export NANO_BRAIN_WORKSPACE=<hash>` for shell `eval`
  - `--json`: print full response
  - `--check`: exit 2 if `registered=false`
- **`cmd/nano-brain/workspaces_test.go`** — add tests using `httptest.NewServer` (same pattern as `runWorkspacesListWithIO`).

### MCP
- **`internal/mcp/tools.go`** — register `memory_workspaces_resolve` tool. Args: `{path: string}`. Same response shape as HTTP. Validates path is non-empty.
- **`internal/mcp/tools_test.go`** — add unit test for new tool.

### Documentation (skill rewrite — Anthropic 2026 progressive disclosure)
- **`skills/nano-brain/SKILL.md`** — rewrite as 4-phase structure:
  - Phase 1 DISCOVER — verify server up, resolve workspace, register if needed
  - Phase 2 SELECT — user intent → operation decision tree
  - Phase 3 EXECUTE — each operation with error handling block
  - Phase 4 RECOVER — fallback patterns + retry limits
- **`skills/nano-brain/AGENTS_SNIPPET.md`** — rewrite (~40 lines, used by `npx nano-brain init` to inject into project AGENTS.md). New bootstrap one-liner: `eval "$(npx nano-brain workspaces current --export)"`.
- **`AGENTS.md`** (project root) — sync OPENCODE-MEMORY block from new snippet; fix wrong endpoint paths + add `workspace` field to all examples.
- **`.opencode/skills/nano-brain/*`** and **`~/.config/opencode/skills/nano-brain/*`** — mirror from canonical `skills/nano-brain/`.
- **`README.md`** — add new endpoint + CLI command to the tables.

## Capabilities

### New Capabilities
- **`workspace-resolve-endpoint`** — defines the contract for `POST /api/v1/workspaces/resolve`, the matching CLI subcommand `nano-brain workspaces current`, and the MCP tool `memory_workspaces_resolve`. All three share the same request (`{path}`) and response (`{workspace_hash, root_path, name, registered}`) shape.

### Modified Capabilities
None — `/api/v1/init` and `GET /api/v1/workspaces` keep current behavior.

## Impact

- **Code**: 4 new files (handler + 2 tests + sqlc query addition), ~3 edits (routes.go, workspaces.go CLI, tools.go MCP), plus 5 docs files.
- **Behavior additive**: existing endpoints unchanged. New endpoint is purely additive. No client breakage.
- **Risk**: Low — read-only, no middleware/auth touched, no schema migration.
- **Performance**: Single PK lookup (`SELECT * FROM workspaces WHERE hash=$1`). <5ms typical.
- **Security**: Returns workspace metadata (path, name, doc count). Same surface as `GET /api/v1/workspaces` which already exposes this. No new leak.

## Out of Scope (separate issues)

- Implicit `NANO_BRAIN_WORKSPACE` env-var reading in `workspaceMiddleware` (Phương án C — high-risk lane, touches middleware + auth surface). Will be its own proposal.
- `nano-brain skill sync` command to regenerate `AGENTS.md` OPENCODE-MEMORY block from `AGENTS_SNIPPET.md` (manual sync this PR).
- Fix for #234 CLI `--config` precedence bug (separate issue already open).
- `.nano-brain` per-project marker file pattern.
- Symlink resolution (`filepath.EvalSymlinks`) — server uses `filepath.Abs` only, consistent with existing `init` handler. Different behavior between host/container would be a footgun.
