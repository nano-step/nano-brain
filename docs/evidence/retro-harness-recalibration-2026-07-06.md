# Retro: Harness Recalibration — 2026-07-06

Trigger: manual retro requested by Tâm ("kiểm tra và thiết lập lại harness rule").
Scope: full audit of every `[HARNESS-OVERRIDE]` evidence file, plus provenance
cleanup (the harness was authored for OpenCode; the project now runs on
Claude Code with GSD as the executor).

## Metrics

- **40 gate-check overrides across 23 evidence files** (17 issues + 6 stories/epics).
- Override frequency by gate:

| Gate | Times overridden | Root cause |
|---|---|---|
| 3.3 integration tests | 11 | Pre-existing master failures (issues #325–#327 et al.) |
| 3.4 golangci-lint | 10 | Pre-existing master lint debt |
| 1.1 previous PR merged | 8 | Orthogonal open PR (#321) / pre-approved parallel tracks — zero file overlap in all 8 |
| 3.10 self-review evidence | 4 | Docs-only / test-only changes forced through code gates |
| 3.12 smoke:e2e | 4 | No runtime surface (docs, test-only, log-level change) |
| 3.13 stale smoke artifacts | 2 | Evidence from a different feature |
| 3.11 commit count | 1 | GitHub PR view counted base commits after master advanced |

- Gates **never** overridden (healthy): 1.2–1.7, 3.1, 3.2, 3.5, 3.6–3.9, 3.14, all 4.x/5.x.

## Pattern Analysis

1. **Pre-existing master debt blocks every PR (21/40).** Gates 3.3/3.4 were
   absolute ("everything passes") instead of differential ("this PR makes
   nothing worse"). Every PR paid an override tax for debt it didn't create.
2. **Serialization ignores orthogonality (8/40).** Gate 1.1 blocked on ANY
   open PR; every override documented zero file overlap.
3. **Checker ignored the change-type table (4+/40).** HARNESS.md already said
   docs/test-only skip smoke:e2e and review evidence; the script never
   implemented it.
4. **Wrong commit-count source (1/40).** GitHub PR view includes base commits;
   `git rev-list merge-base..HEAD` is the truth.

## Root Cause

The gate script encoded ideal-world absolutes, not measurable differential
conditions. Once master accumulated debt, "FAIL → override with prose" became
the de-facto process — the override mechanism (R7) absorbed what the PASS
conditions should have expressed. Additionally, the harness docs still
described the OpenCode autonomous plugin loop that no longer drives anything.

## Harness Rule Updates (approved by Tâm, 2026-07-06)

- **R90** — gate 1.1 blocks only on file overlap between the open PR and the
  current branch (`gh pr view --json files` ∩ `git diff --name-only
  origin/master...HEAD`).
- **R91** — gates 3.3/3.4 are differential: 3.3 compares failing packages
  against `docs/harness-baseline.txt` (shrink-only); 3.4 uses
  `golangci-lint run --new-from-rev=<merge-base>`.
- **R92** — change-type detected measurably from the diff; docs-only skips
  3.10 + 3.12, test-only skips 3.12.
- Gate 3.11 counts commits via `git rev-list --count <merge-base>..HEAD`.
- State: `docs/harness-state.json` retired; git history + `.planning/STATE.md`
  are the source of truth; process debt → GitHub issues labeled `harness-debt`.
- Enforcement on Claude Code: PreToolUse hook
  (`.claude/hooks/harness-pre-merge-hook.sh`) blocks `gh pr create` while fast
  pre-merge gates FAIL (`HARNESS_FAST=1` skips 3.1–3.4 in the hook; the
  validation ladder still requires them). `[HARNESS-OVERRIDE]` (R7) bypasses.
- Provenance: `/harness-on` loop documented as OpenCode-legacy;
  `/harness-gsd` + `harness-check` skill are the Claude Code entry points;
  stale `b-main` references replaced with `master`.

## Applied Changes

- `scripts/harness-check.sh` — gates 1.1 (R90), 3.3/3.4 (R91), 3.10/3.12
  (R92), 3.11 (git commit count), `HARNESS_FAST` fast mode.
- `docs/harness-baseline.txt` — seeded from origin/master (d64ab8e), 8 packages.
- `docs/HARNESS_GATES.md` — gate table updated, R90/R91/R92 defined,
  state-file line replaced, `b-main` → `master`.
- `docs/HARNESS.md` — "Entry Points (Claude Code)" section replaces the
  OpenCode `/harness-on` workflow section.
- `docs/HARNESS_RUNNER_CONTRACT.md` — reframed as legacy interface spec for
  `--json` output.
- `docs/GLOSSARY.md` — skill path corrected to `.claude/skills/…`.
- `docs/harness-state.json` — removed.
- `.claude/commands/harness-gsd.md` — committed (was untracked).
- `.claude/hooks/harness-pre-merge-hook.sh` + `.claude/settings.json` — new
  enforcement hook (verified live: blocked a `gh pr create` string in-session).
- `.gitignore` — exclude GSD-managed hooks from accidental `git add -A`.
