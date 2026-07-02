---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
plan: 01
subsystem: infra
tags: [docker, os-exec, postgres, cli, test-seam]

# Dependency graph
requires: []
provides:
  - "runDocker injectable dockerRunner seam (var runDocker dockerRunner = defaultRunDocker) wrapping exec.CommandContext(ctx, \"docker\", args...) with a fixed argv"
  - "dockerStatus(ctx) classification: dockerStatusNotInstalled / dockerStatusDaemonNotRunning / dockerStatusAvailable / dockerStatusUnknownError"
  - "provisionPostgres(ctx) (dbURL string, err error) — runs the fixed nanobrain-pg/pgvector:pg17 container, recovers from name-conflict (docker start) and port-conflict (docker rm stray + retry on :5433)"
affects: [13-05 (DB step wizard consumes dockerStatus + provisionPostgres)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Injectable exec-runner test seam: package-level function-typed var (runDocker) swapped in tests via t.Cleanup-restored save/override, mirroring the existing runServeDaemonFn/isTTYFn hook-var idiom in client.go"
    - "Fixed-literal argv construction for os/exec: container name and image are Go string constants, never string-concatenated or shell-interpolated, closing the command-injection trust boundary named in the plan's threat model"

key-files:
  created:
    - cmd/nano-brain/docker_provision.go
    - cmd/nano-brain/docker_provision_test.go
  modified: []

key-decisions:
  - "Committed RED test + GREEN implementation in a single commit (not split test-then-feat commits) because the repo's pre-commit hook (scripts/harness-check.sh in-progress) runs go build ./... && go test -race -short ./... across the WHOLE repo and blocks any commit where that fails — an uncompilable RED-only commit would be rejected. Verified RED failure locally first (go test showed 10 'undefined: X' compile errors referencing every required symbol), then restored the implementation and committed both files together once green. Same precedent as Phase 999.1-01 and Phase 10-01 per STATE.md decision log."
  - "provisionPostgres treats runDocker returning a non-nil err (docker binary missing) as an unrecoverable error from provisionPostgres itself, wrapped with fmt.Errorf and errors.Is-compatible (%w) — dockerStatus should be called first by the wizard (Plan 05) to detect this case with a clearer status before ever calling provisionPostgres"

requirements-completed: [D-06, D-07]

coverage:
  - id: D1
    description: "dockerStatus classifies docker info results into not-installed / daemon-down / available / unknown-error"
    requirement: "D-06"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/docker_provision_test.go#TestDockerStatus (4 subtests: not-installed, daemon-not-running, available, unknown-error)"
        status: pass
    human_judgment: false
  - id: D2
    description: "provisionPostgres runs the fixed D-06 docker run command and returns the :5432 dbURL on success with zero recovery calls"
    requirement: "D-06"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/docker_provision_test.go#TestProvisionPostgres/success_path_-_single_run,_no_start/rm_calls"
        status: pass
    human_judgment: false
  - id: D3
    description: "name-conflict (exit 125, 'is already in use by container') recovers via docker start and returns the :5432 dbURL"
    requirement: "D-07"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/docker_provision_test.go#TestProvisionPostgres/name-conflict_path_-_docker_start_recovers"
        status: pass
    human_judgment: false
  - id: D4
    description: "port-conflict (exit 125, 'port is already allocated') removes the stray Created container via docker rm, then retries docker run on :5433 and returns the :5433 dbURL"
    requirement: "D-07"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/docker_provision_test.go#TestProvisionPostgres/port-conflict_path_-_rm_stray_then_retry_on_5433"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-07-02
status: complete
---

# Phase 13 Plan 01: Docker Provision Wrapper Summary

**os/exec Docker CLI wrapper (runDocker seam, dockerStatus classification, provisionPostgres with name/port-conflict recovery) behind an injectable test seam — zero real-daemon dependency, fixed argv only**

## Performance

- **Duration:** 25 min
- **Started:** 2026-07-02T14:33:00Z
- **Completed:** 2026-07-02T14:58:19Z
- **Tasks:** 2
- **Files modified:** 2 (both new)

## Accomplishments
- `dockerRunner` type + `runDocker` package-level hook var (test seam), `defaultRunDocker` shelling out via `exec.CommandContext(ctx, "docker", args...)` with a fixed argv (no shell string, no user interpolation)
- `dockerStatus(ctx)` classifies `docker info` results into `dockerStatusNotInstalled` (binary missing, `*exec.Error`), `dockerStatusDaemonNotRunning` (`stderr` contains "Cannot connect to the Docker daemon"), `dockerStatusAvailable` (exit 0), `dockerStatusUnknownError` (other nonzero exit) — matches RESEARCH.md's empirically-verified Docker CLI exit-code/stderr strings
- `provisionPostgres(ctx)` runs the fixed D-06 `docker run -d --name nanobrain-pg --restart unless-stopped -p 5432:5432 -e POSTGRES_USER=nanobrain -e POSTGRES_PASSWORD=nanobrain -e POSTGRES_DB=nanobrain_dev pgvector/pgvector:pg17` command; recovers name-conflict via `docker start nanobrain-pg`; recovers port-conflict via `docker rm nanobrain-pg` (best-effort, ignoring not-found) then retries on `-p 5433:5433`
- `docker_provision_test.go`: `withDockerRunner` fake-seam helper (save/override/`t.Cleanup`-restore, mirrors `commands_test.go`'s `withRecoveryHooks`) plus `TestDockerStatus` (4 cases) and `TestProvisionPostgres` (5 cases: success, name-conflict, port-conflict, unrecovered-exit, binary-missing) — all exercise only the injected fake, no real Docker daemon touched

## Task Commits

Both tasks (RED test + GREEN implementation) were committed together in a single commit, per the plan's explicit instruction and repo pre-commit constraints (see Decisions Made below):

1. **Task 1 + Task 2: docker_provision.go + docker_provision_test.go** - `a01a48f` (feat)

RED evidence was captured locally before the joint commit: with `docker_provision.go` temporarily removed, `go test -race -short ./cmd/nano-brain/ -run 'TestDockerStatus|TestProvisionPostgres'` failed to compile with 10 `undefined: X` errors covering every symbol from the plan's `<artifacts_produced>` contract (`runDocker`, `dockerStatusType`, all four `dockerStatus*` constants, `dockerStatus`, `provisionPostgres`). The implementation was then restored and GREEN was reverified before staging/committing.

**Plan metadata:** (worktree mode — SUMMARY.md commit happens separately per parallel-execution protocol, no STATE.md/ROADMAP.md write in this agent)

## Files Created/Modified
- `cmd/nano-brain/docker_provision.go` - `dockerRunner` type, `runDocker`/`defaultRunDocker`, `dockerStatusType` + 4 constants, `dockerStatus(ctx)`, `provisionPostgres(ctx)`
- `cmd/nano-brain/docker_provision_test.go` - `withDockerRunner` fake-seam test helper, `TestDockerStatus`, `TestProvisionPostgres`

## Decisions Made
- Committed RED+GREEN together in one commit rather than two (test-only, then feat) because `scripts/harness-check.sh in-progress` (wired as the repo's `.git/hooks/pre-commit`) runs `go build ./...` and `go test -race -short ./...` across the entire repository and blocks the commit on failure — an uncompilable test-only commit is not committable here. RED was still verified as real evidence (see Task Commits) before writing the implementation, satisfying the TDD intent without violating the commit gate. This mirrors the precedent already recorded in STATE.md for Phase 999.1-01 and Phase 10-01.
- `provisionPostgres` wraps execution failures (docker binary missing) with `fmt.Errorf("...: %w", err)` so `errors.Is(err, exec.ErrNotFound)` works for callers — verified by an explicit test case.
- No new dependencies added; `git diff go.mod` is empty as required by the plan's `<verification>` section.

## Deviations from Plan

None - plan executed exactly as written. The RED/GREEN commit-ordering note in Task 1's `<action>` explicitly anticipated and pre-authorized the joint-commit approach taken here, so this is not tracked as a deviation.

## Issues Encountered

`go build ./cmd/nano-brain/` alone does not catch missing symbols referenced only from `_test.go` files (Go doesn't compile test files as part of a normal package build) — the RED verification had to use `go vet` and `go test` instead of `go build` to observe the actual compile failure. This is a tooling nuance, not a plan issue; the plan's own `<verify>` block for Task 1 already specifies `go test`, not `go build`, for this reason.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- `dockerStatus(ctx)` and `provisionPostgres(ctx)` are ready for Plan 05 (DB step) to call directly — both are pure functions over the injectable `runDocker` seam, fully unit-testable without a real Docker daemon.
- Full repo test suite (`go test -race -short ./...`) passes with these two new files added; no regressions in any other package.
- No blockers for downstream plans in this phase.

---
*Phase: 13-interactive-init-wizard-one-command-interactive-setup-detect*
*Completed: 2026-07-02*

## Self-Check: PASSED

- FOUND: cmd/nano-brain/docker_provision.go
- FOUND: cmd/nano-brain/docker_provision_test.go
- FOUND: .planning/phases/13-interactive-init-wizard-one-command-interactive-setup-detect/13-01-SUMMARY.md
- FOUND: a01a48f (feat commit)
- FOUND: 259aece (docs commit, pre-self-check)
