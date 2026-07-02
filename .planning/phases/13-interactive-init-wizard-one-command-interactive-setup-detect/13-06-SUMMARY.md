---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
plan: 06
subsystem: cli
tags: [go, cli, wizard, daemon, build-tags, tdd]

# Dependency graph
requires:
  - phase: 13-02
    provides: doctor.RunAll and doctor.Check (PostgreSQL status inspection)
provides:
  - stepServe wizard server-start step (D-14) gating daemon launch on doctor's PostgreSQL check
  - launchServeDaemonFn / serverHealthyFn test seams for future wizard steps to reuse the pattern
  - init_serve_unix.go / init_serve_windows.go build-tag pair isolating runServeDaemonFn from Windows compilation
affects: [13-07 (orchestrator wiring), interactive-init-wizard]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "!windows / windows build-tag file pair for wrapping a platform-only daemon launcher behind a tag-free seam var"

key-files:
  created:
    - cmd/nano-brain/init_serve.go
    - cmd/nano-brain/init_serve_unix.go
    - cmd/nano-brain/init_serve_windows.go
    - cmd/nano-brain/init_serve_test.go
  modified: []

key-decisions:
  - "RED and GREEN committed together in one feat commit (not split test/feat) because repo pre-commit harness-check.sh blocks commits while tests are red — matches the Phase 10-01 precedent"
  - "serverHealthyFn used for both the already-running precheck and the post-launch health wait, avoiding a second seam var — a single probe naturally answers both questions"
  - "stepServe still accepts a *bufio.Scanner parameter per the plan's declared signature, even though the accept/decline prompt is driven by promptStartServer via the promptReader/promptWriter seams (matching client.go's recoverFromConnectionRefused pattern) rather than the scanner — kept for interface consistency with other step functions and possible future use"

patterns-established:
  - "Platform-only symbol access from a tag-free seam file: declare `var launchXFn = platformX` in the tag-free file, define `platformX` once each in `_unix.go` (!windows) and `_windows.go` (windows) — only the unix file references the daemon-package symbol"

requirements-completed: [D-14]

coverage:
  - id: D1
    description: "stepServe aborts (no daemon launch) when doctor reports a PostgreSQL FAIL"
    requirement: "D-14"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_serve_test.go#TestStepServe_AbortsOnPostgreSQLFail"
        status: pass
    human_judgment: false
  - id: D2
    description: "stepServe skips with an already-running note when a healthy server is already reachable"
    requirement: "D-14"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_serve_test.go#TestStepServe_AlreadyRunningSkipsLaunch"
        status: pass
    human_judgment: false
  - id: D3
    description: "stepServe launches the daemon via launchServeDaemonFn and waits for health on healthy prerequisites + TTY + accept"
    requirement: "D-14"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_serve_test.go#TestStepServe_AcceptAndStart"
        status: pass
    human_judgment: false
  - id: D4
    description: "stepServe skips without launching on user decline or non-TTY"
    requirement: "D-14"
    verification:
      - kind: unit
        ref: "cmd/nano-brain/init_serve_test.go#TestStepServe_DeclineSkipsLaunch"
        status: pass
      - kind: unit
        ref: "cmd/nano-brain/init_serve_test.go#TestStepServe_NonTTYSkipsLaunch"
        status: pass
    human_judgment: false
  - id: D5
    description: "GOOS=windows go build ./cmd/nano-brain/ introduces zero new init_serve* errors (pre-existing daemon.go undefined-symbol gap unchanged)"
    requirement: "D-14"
    verification:
      - kind: other
        ref: "GOOS=windows go build ./cmd/nano-brain/ 2>&1 | grep -c init_serve (result: 0)"
        status: pass
    human_judgment: false

duration: ~30min
completed: 2026-07-02
status: complete
---

# Phase 13 Plan 06: Wizard server-start step (D-14) Summary

**stepServe wizard step gates daemon launch on doctor's PostgreSQL check, skips when already running or declined, and degrades to a manual-instruction stub on Windows via a `!windows`/`windows` build-tag file pair.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-07-02T14:55:00Z
- **Completed:** 2026-07-02T15:13:07Z
- **Tasks:** 2
- **Files modified:** 4 (all new)

## Accomplishments
- `stepServe` implements the full D-14 decision tree: abort on PostgreSQL FAIL, skip on already-healthy, skip on decline/non-TTY, launch+wait-for-health on accept
- `init_serve_unix.go` / `init_serve_windows.go` build-tag pair keeps the wizard step file compiling under both native and `GOOS=windows`, isolating the only reference to `runServeDaemonFn` to the `!windows` file (matching RESEARCH Pattern 4 / Pitfall 4)
- `TestStepServe` (5 subtests) covers all four outcome branches via injected `launchServeDaemonFn` / `serverHealthyFn` / `isTTYFn` / `promptReader` seams — no real daemon spawned, no real DB or HTTP contacted

## Task Commits

Both tasks were committed together as a single `feat` commit because the repo's pre-commit `harness-check.sh` hook blocks any commit while the build/test suite is red (the RED test task, in isolation, fails to compile since the seams it references don't exist until Task 2). This matches the established precedent from Phase 10-01 (see STATE.md decision log).

1. **Task 1 (RED) + Task 2 (GREEN): init_serve.go + unix/windows launch files + init_serve_test.go** - `eb92bf6` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE.md, per orchestrator-owned metadata commit — worktree agents do not update STATE.md/ROADMAP.md directly)

## Files Created/Modified
- `cmd/nano-brain/init_serve.go` - Build-tag-free `serveOutcome` enum + `stepServe` decision logic + `launchServeDaemonFn`/`serverHealthyFn` seams
- `cmd/nano-brain/init_serve_unix.go` (`//go:build !windows`) - `platformLaunchServeDaemon` delegates to the existing `runServeDaemonFn` seam
- `cmd/nano-brain/init_serve_windows.go` (`//go:build windows`) - `platformLaunchServeDaemon` prints the manual `nano-brain serve` instruction, references no daemon symbols
- `cmd/nano-brain/init_serve_test.go` - `TestStepServe` (5 subtests) + `withServeHooks` helper mirroring `withRecoveryHooks`

## Decisions Made
- RED+GREEN committed together (see key-decisions above) — repo-specific pre-commit constraint, not a plan deviation
- `serverHealthyFn` reused for both the already-running precheck and the post-launch wait rather than adding a second seam, since both are "is the server answering healthy right now" questions
- Kept the `scanner *bufio.Scanner` parameter on `stepServe` per the plan's declared signature even though the actual accept/decline read goes through `promptStartServer(promptReader, promptWriter)` (the existing seam pattern from `client.go`'s `recoverFromConnectionRefused`), for signature consistency with other step functions in this package

## Deviations from Plan

None — plan executed exactly as written. The RED/GREEN commit-grouping is a mechanical consequence of the repo's pre-commit hook (documented as a decision, not a deviation from task content).

## Issues Encountered
- Initial test-hook wiring bug (self-caught in Task 1/2 development, not a plan deviation): the first draft of `init_serve_test.go` only fed the "Y\n"/"n\n" answer into the `scanner` parameter passed to `stepServe`, but `stepServe`'s prompt path reads from the package-level `promptReader`/`promptWriter` seams (matching `client.go`'s existing pattern), not from `scanner`. Fixed by having `withServeHooks` also override `promptReader`/`promptWriter`, matching `withRecoveryHooks` in `commands_test.go`. Caught immediately by the failing `TestStepServe_AcceptAndStart` subtest before any commit was made — no broken state was ever committed.
- A `cd`-based Bash call during initial verification silently drifted the shell's working directory to the main repository instead of the worktree (per the known #3097 issue class). Caught before any file operations were affected (only a `go test` was run there, immediately followed by cwd verification) by comparing `git rev-parse --show-toplevel` against a persisted sentinel; all subsequent commands used `(cd "$WT_ROOT" && ...)` subshells anchored to the worktree root instead of a bare `cd`.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- `stepServe` is ready for Plan 07 to wire into the orchestrator's step sequence, consuming its `serveOutcome` return value
- `launchServeDaemonFn` / `serverHealthyFn` seams are available for Plan 07's tests to reuse if the orchestrator needs to assert on server-start behavior end-to-end
- Windows build status unchanged (still pre-existing 5 undefined-symbol errors from `daemon.go`/`main.go`, none newly introduced by this plan) — a full Windows daemon implementation remains explicitly out of scope per RESEARCH Open Question 1

---
*Phase: 13-interactive-init-wizard-one-command-interactive-setup-detect*
*Completed: 2026-07-02*

## Self-Check: PASSED

- FOUND: cmd/nano-brain/init_serve.go
- FOUND: cmd/nano-brain/init_serve_unix.go
- FOUND: cmd/nano-brain/init_serve_windows.go
- FOUND: cmd/nano-brain/init_serve_test.go
- FOUND: .planning/phases/13-interactive-init-wizard-one-command-interactive-setup-detect/13-06-SUMMARY.md
- FOUND commit: eb92bf6 (feat)
- FOUND commit: aaf2198 (docs)
