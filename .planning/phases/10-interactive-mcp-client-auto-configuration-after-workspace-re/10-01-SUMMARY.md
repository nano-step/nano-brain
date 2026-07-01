---
phase: 10-interactive-mcp-client-auto-configuration-after-workspace-re
plan: 01
subsystem: api
tags: [go, echo, cli, json, workspace, mcp]

# Dependency graph
requires:
  - phase: 09-mcp-workspace-config-binding
    provides: "?workspace=<name-or-hash> URL query param contract on the MCP HTTP endpoint"
provides:
  - "initResponse.Name (json:\"name\") on POST /api/v1/init, populated from ws.Name"
  - "runInitCmd's --root branch decode struct exposes result.Name in scope"
affects: [10-02-config-writer, 10-03-per-client-mcp-config]

# Tech tracking
tech-stack:
  added: []
  patterns: ["Additive-only response DTO extension: new json field, no renamed/removed fields, unknown-field-tolerant decoders on existing consumers"]

key-files:
  created: []
  modified:
    - internal/server/handlers/workspace.go
    - internal/server/handlers/workspace_test.go
    - cmd/nano-brain/commands.go

key-decisions:
  - "Populate initResponse.Name from ws.Name (already returned by UpsertWorkspace's RETURNING clause) rather than adding a new query or recomputing filepath.Base client-side"
  - "RED test and GREEN implementation committed together in one commit per task, not as separate test/feat commits, due to a repo pre-commit gate (harness-check.sh) that blocks any commit while the working tree's tests are red"

patterns-established:
  - "Server-computed identifiers (workspace name) are always round-tripped through the API response, never recomputed client-side in the CLI, to avoid future divergence between client and server logic"

requirements-completed: []

coverage:
  - id: D1
    description: "POST /api/v1/init JSON response includes a name field equal to the workspace's directory basename, alongside the pre-existing workspace_hash/root_path/agents_snippet fields"
    verification:
      - kind: unit
        ref: "internal/server/handlers/workspace_test.go#TestInitWorkspaceHandler"
        status: pass
    human_judgment: false
  - id: D2
    description: "runInitCmd's --root branch decodes the workspace name from the init response into result.Name, available for Plan 02's MCP config writer; --json and non---root behavior unchanged"
    verification:
      - kind: unit
        ref: "go build ./... && go test -race -short ./cmd/nano-brain/..."
        status: pass
    human_judgment: false

duration: 3min
completed: 2026-07-01
status: complete
---

# Phase 10 Plan 01: Workspace name in init response Summary

**Added the workspace `name` field to `POST /api/v1/init`'s JSON response and to the CLI's `--root` decode struct, unblocking Plan 02's `?workspace=<name>` MCP config binding.**

## Performance

- **Duration:** 3 min
- **Started:** 2026-07-01T14:15:53Z
- **Completed:** 2026-07-01T14:19:33Z
- **Tasks:** 2 completed
- **Files modified:** 3

## Accomplishments
- `initResponse` (server-side DTO in `internal/server/handlers/workspace.go`) now carries `Name string` (`json:"name"`), populated from `ws.Name` — no new DB query, since `UpsertWorkspace`'s `RETURNING` clause already selects `name`.
- New test assertion in `TestInitWorkspaceHandler` confirms the response's `name` key equals the registered workspace's directory basename (`test-project` for `/tmp/test-project`), while all three pre-existing fields (`workspace_hash`, `root_path`, `agents_snippet`) remain unchanged.
- `runInitCmd`'s `--root` branch anonymous decode struct (`cmd/nano-brain/commands.go`) gained a matching `Name string` (`json:"name"`) field, so `result.Name` is now in scope in the success branch for Plan 02's prompt-orchestration to consume. No output/print change and no client-side recomputation of the name (per RESEARCH's explicit anti-pattern warning) — the value is sourced purely from the API response.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Name to the init response server-side (RED then GREEN)** - `a27c848` (feat)
2. **Task 2: Decode Name in the CLI --root branch** - `4b1d1fe` (feat)

**Plan metadata:** (pending — see final commit)

_Note: Task 1 combined the RED test and GREEN implementation into a single commit rather than two separate `test`/`feat` commits — see Deviations below._

## Files Created/Modified
- `internal/server/handlers/workspace.go` - Added `Name string` (json:"name") to `initResponse`, populated from `ws.Name` in the `c.JSON(...)` construction
- `internal/server/handlers/workspace_test.go` - Added assertion in `TestInitWorkspaceHandler` that the response's `name` key equals the workspace basename
- `cmd/nano-brain/commands.go` - Added `Name string` (json:"name") to the anonymous decode struct in `runInitCmd`'s `--root` success branch

## Decisions Made
- Populated `Name` from `ws.Name` (already returned via `UpsertWorkspace`'s `RETURNING` clause) instead of adding a new query or round trip — zero added latency/DB cost.
- Did not touch the `--json` early-return path (`commands.go`) — it already prints the raw response verbatim, so it carries the new field for free without any code change.
- Did not recompute the workspace name client-side via `filepath.Base(root)` in the CLI — the plan's threat model and RESEARCH explicitly flag this as an anti-pattern (server-side name is canonical and may diverge from a client-side guess if future server logic adds sanitization/collision-handling/renaming).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Collapsed TDD RED/GREEN into a single commit per task**
- **Found during:** Task 1 (Add Name to the init response server-side)
- **Issue:** The plan's `tdd="true"` flow calls for a RED commit (failing test) followed by a separate GREEN commit (passing implementation). This repo's `.git/hooks/pre-commit` runs `./scripts/harness-check.sh in-progress`, which includes a "Build or tests failed" check that fails (and blocks the commit) whenever the working tree has a red test — exactly the state a RED-only commit requires.
- **Fix:** Wrote the RED test, ran it standalone to confirm the expected failure (`workspace_test.go:130: expected name "test-project", got <nil>`), then immediately implemented GREEN before creating any commit. Task 1 was committed once, as `feat(10-01): add workspace name to init response`, with both the test and implementation staged together. The RED failure was still verified and evidenced (see command output in this session), just not captured as its own git commit.
- **Files modified:** internal/server/handlers/workspace.go, internal/server/handlers/workspace_test.go
- **Verification:** `go test -race -short -run TestInitWorkspaceHandler ./internal/server/handlers/...` failed before the fix (RED), passed after (GREEN); full package suite (`go test -race -short ./internal/server/handlers/... ./cmd/nano-brain/...`) green post-commit.
- **Committed in:** a27c848 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking — environment/tooling constraint, no scope or behavior change)
**Impact on plan:** No functional or scope impact. The plan's `<done>` criteria (response carries `name`, tests pass) are fully met; only the git-commit granularity of the TDD RED/GREEN split was adjusted to satisfy this repo's own pre-commit harness gate.

## Issues Encountered
- An environment-level "Pre-commit review required" gate (separate from this repo's own `harness-check.sh`) intercepted both `git commit` attempts and required running `/simplify` and `/code-review` skills before the commit could proceed, then marking a sentinel file for the current HEAD sha. Both review passes found zero issues (diffs were minimal, additive-only struct-field changes with no call-site breakage — verified via grep that the only consumer of `initResponse`'s JSON, `cmd/nano-brain/commands.go`, decodes into a struct that safely ignores unknown fields). Resolved by following the gate's documented steps for each of the two commits.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Plan 02 can now read `result.Name` in `runInitCmd`'s `--root` branch to build the `?workspace=<name>` MCP binding URL for each client config writer.
- No blockers. Both touched packages (`internal/server/handlers`, `cmd/nano-brain`) build and test green.

---
*Phase: 10-interactive-mcp-client-auto-configuration-after-workspace-re*
*Completed: 2026-07-01*

## Self-Check: PASSED

- FOUND: .planning/phases/10-interactive-mcp-client-auto-configuration-after-workspace-re/10-01-SUMMARY.md
- FOUND: internal/server/handlers/workspace.go
- FOUND: cmd/nano-brain/commands.go
- FOUND: commit a27c848
- FOUND: commit 4b1d1fe
