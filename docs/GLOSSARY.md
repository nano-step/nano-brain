# Glossary

Closed-set vocabulary for the nano-brain harness. When a term appears in any
harness doc (`HARNESS.md`, `HARNESS_GATES.md`, `FEATURE_INTAKE.md`, ADRs, story
templates) it carries the meaning defined here — no other meaning is permitted.

If you need a meaning not in this glossary, add the term here first, then use it.

## Process Terms

**Agent** — An AI assistant operating within the harness (e.g. Sisyphus, Oracle,
Metis, Momus, explore, librarian).

**Harness** — The set of rules, gates, templates, and scripts that govern how
agents propose, implement, review, and ship changes in this repo.

**Lane** — Risk classification of a piece of work. Closed set: `tiny`, `normal`,
`high-risk`. Determined by FEATURE_INTAKE.md risk checklist.

**Hard gate** — A flag that auto-promotes work to `high-risk` regardless of
risk-flag count. Closed set: `auth`, `authorization`, `data-model`,
`audit-security`, `external-providers`, `public-api-contracts`,
`search-quality`, `embedding-vector-provider`.

**Risk flag** — One of 10 categories in FEATURE_INTAKE.md that, when set,
contributes to lane classification.

**Change type** — Closed set: `user-feature`, `bug-fix`, `infrastructure`,
`refactor`, `docs`, `dependency-bump`. Combined with lane to determine which
validation layers apply.

## Gate Terms

**Gate** — A blocking check enforced by `scripts/harness-check.sh` at a
development transition point. Closed set: ① PRE-WORK, ② IN-PROGRESS,
③ PRE-MERGE, ④ POST-MERGE, ⑤ NEXT-READY, ⑥ RETRO.

**Check** — A single PASS/FAIL/SKIP assertion within a gate, identified by
`<gate>.<num>` (e.g. `3.6`, `1.3`).

**PASS / FAIL / SKIP** — Closed set of check outcomes. `PASS` = condition met.
`FAIL` = condition not met, BLOCK proceeding. `SKIP` = check not applicable in
this context (must document why).

**Verdict** (Gemini triage) — Agent's assessment of a Gemini PR comment.
Closed set: `VALID:critical`, `VALID:high`, `VALID:medium`, `VALID:low`,
`FALSE_POSITIVE`, `DEFER`, `ACKNOWLEDGED`. See HARNESS.md § PR + Bot Review Loop.

**Override** — Explicit user-posted PR comment matching `[HARNESS-OVERRIDE]: <reason>`
(reason ≥ 20 chars). Bypasses gate 3.6 Gemini block. Invalidated by any
subsequent push.

## Artifact Terms

**Story** — A unit of work scoped for a single PR. Lives in `docs/stories/` as
part of an epic file, or as an OpenSpec change in `openspec/changes/<name>/`.

**Epic** — A collection of related stories. Each epic gets one
`docs/stories/epic-<N>-<name>.md` file and one retro at completion.

**OpenSpec change** — A proposal-and-design package in `openspec/changes/<name>/`
with files `proposal.md`, `design.md`, `specs/`, `tasks.md`. Required for
`normal` and `high-risk` lanes.

**ADR** (Architecture Decision Record) — A durable record of an architectural
decision. Lives in `docs/decisions/NNNN-<title>.md`. See docs/decisions/README.md.

**Spec** — A behavior contract under `openspec/specs/<capability>/` describing
acceptance criteria for a capability. The living source of truth post-merge.

**Evidence** — Concrete artifact (file, command output, screenshot) that
demonstrates a gate check or acceptance criterion is met. Lives in `docs/evidence/`.

**Self-review evidence file** — Markdown at `docs/evidence/self-review-<slug>.md`
containing findings tables and Gemini triage. Required by gate 2.4.

**Retro evidence file** — Markdown at `docs/evidence/retro-epic-<N>.md` with
sections `## Metrics`, `## Patterns`, `## Root Cause`, `## Proposed Changes`.
Required by gate 6.4 and 6.5.

## Output Terms

**Product delta** — Changes to app code, tests, product docs, or user-visible
behavior. One of two valid task outputs.

**Harness delta** — Changes to HARNESS.md, gates, templates, scripts, or
process docs. One of two valid task outputs. A task producing ONLY harness
delta is legitimate work.

**Friction** — Anything that slows or confuses an agent during work. Captured
in `docs/HARNESS_BACKLOG.md` when out of current scope; otherwise fixed inline
as harness delta.

## Validation Terms

**Validation ladder** — Closed set of validation layers, applied per lane.
Layers: `validate:quick`, `self-review:response-shape`, `self-review:staged-files`,
`test:integration`, `smoke:e2e`, `test:release`. See HARNESS.md § Validation Ladder.

**smoke:e2e** — Real-usage test: build binary → start server → curl endpoints →
verify HTTP status + JSON structure. NOT a Go test file. Evidence is the curl
command + response pasted in story or `docs/evidence/`.

**Review Gate** — Independent verification by a fresh agent (not the implementer)
that every acceptance criterion has evidence. Output is a verdict: `PASS` or
`FAIL`. Required before `openspec archive`.

## File Path Conventions

- `docs/HARNESS.md` — process spec (this is the master doc)
- `docs/HARNESS_GATES.md` — gate check specification
- `docs/FEATURE_INTAKE.md` — risk classification
- `docs/GLOSSARY.md` — this file
- `docs/decisions/NNNN-<title>.md` — ADRs
- `docs/evidence/<kind>-<slug>.md` — evidence files
- `docs/stories/epic-<N>-<name>.md` — epic + story packets
- `docs/templates/<artifact>.md` — reusable templates
- `openspec/changes/<name>/` — active OpenSpec proposal
- `openspec/specs/<capability>/` — accepted behavior contracts
- `openspec/archive/<name>/` — archived OpenSpec changes
- `scripts/harness-check.sh` — gate enforcement script
- `.opencode/skills/harness-check/SKILL.md` — agent-side enforcement skill
