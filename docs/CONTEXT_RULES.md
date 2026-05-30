# Context Retrieval Rules

Agents working in this harness MUST read only the files listed below for each
phase × lane combination. Reading more is **token waste**. Reading less risks
missing required rules.

This doc exists because the harness has ~89 rules across 6+ files. An agent
fixing a typo does not need to read FEATURE_INTAKE.md; an agent shipping an
auth feature MUST read it.

## Phase 1: Intake (classify the work)

Goal: Determine lane, change type, and whether a GitHub issue is needed.

| Lane being considered | Files to READ                                                  | Files to SKIP                 |
| --------------------- | -------------------------------------------------------------- | ----------------------------- |
| all                   | `docs/FEATURE_INTAKE.md`, `docs/GLOSSARY.md` (terms), `AGENTS.md` (Lanes section only) | Full HARNESS.md, all templates |

## Phase 2: Planning (proposal + design)

Goal: Produce OpenSpec proposal, design, specs, story packet.

| Lane      | Files to READ (in addition to Phase 1)                                                                                                                                | Files to SKIP                                          |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------ |
| tiny      | `AGENTS.md` (full), target file's directory `README.md` if exists                                                                                                     | OpenSpec docs, HARNESS_GATES.md, templates             |
| normal    | tiny + `docs/HARNESS.md` (Validation Ladder, Change Types, Review Gate sections), `docs/templates/story.md`, related `docs/decisions/*.md` (only ones touching subject) | HARNESS_GATES.md detail, unrelated decisions           |
| high-risk | normal + `docs/HARNESS_GATES.md` (full), `openspec/changes/<active>/*` (current proposal/design), ALL `docs/decisions/*.md`, `docs/templates/adr.md`                    | Unrelated epics in `docs/stories/`                     |

## Phase 3: Implementation

Goal: Write code/tests to satisfy acceptance criteria.

| Lane      | Files to READ                                                                                                                                  | Files to SKIP                                |
| --------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| tiny      | `AGENTS.md` (File Writing Rules section), target file + adjacent files in same package                                                         | All process docs                             |
| normal    | tiny + relevant product docs (only the contract being touched), the active OpenSpec spec for the capability                                    | Other capabilities' specs                    |
| high-risk | normal + `openspec/changes/<active>/design.md` + `tasks.md`, latest ADR if architecture-affecting                                              | Retro files of past epics                    |

## Phase 4: Validation

Goal: Run the validation ladder and produce evidence.

| Lane      | Files to READ                                                                                                                              | Files to SKIP                                |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------- |
| all       | `docs/HARNESS.md` § Validation Ladder + § Change Types, story file's "Acceptance Criteria" + "Validation" sections                         | HARNESS_GATES.md gate-by-gate detail         |
| high-risk | + `docs/evidence/retro-epic-*.md` for the most recent 2 epics (look for recurring failure patterns relevant to current change)             | —                                            |

## Phase 5: Review / PR

Goal: Triage Gemini comments, produce self-review evidence, manage PR loop.

| Lane      | Files to READ                                                                                                                                                              |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| all       | `docs/HARNESS.md` § PR + Bot Review Loop (R31 vocabulary), § Forbidden Practices #7 (R7 override), `docs/GLOSSARY.md` (Verdict section), existing `docs/evidence/self-review-*.md` for format examples |

## Phase 6: Retro

Goal: Compute metrics, identify patterns, propose harness changes.

| Lane | Files to READ                                                                                                       |
| ---- | ------------------------------------------------------------------------------------------------------------------- |
| all  | `docs/HARNESS_GATES.md` § Gate ⑥, prior `docs/evidence/retro-epic-*.md` (last 2 epics), `docs/HARNESS_BACKLOG.md` |

## Token Budgets

Soft caps on input context per task. If approaching the cap, summarize prior
context and drop low-value files first.

| Lane      | Soft budget (input tokens) |
| --------- | -------------------------- |
| tiny      | 20,000                     |
| normal    | 50,000                     |
| high-risk | 100,000                    |

If budget is exceeded mid-task: pause, summarize what's been read, drop the
files least relevant to the current decision, then continue.

## Anti-Patterns

These behaviors waste tokens and are forbidden:

1. **Reading the full HARNESS.md for every task.** Use the table above to scope.
2. **Reading all of `docs/decisions/` "just in case".** Only read decisions that
   touch the same subsystem as the current change.
3. **Re-reading files within the same task.** Files don't change mid-task; cache
   what you read.
4. **Reading both HARNESS.md and HARNESS_GATES.md for a `tiny` change.** Tiny
   changes don't go through normal/high-risk gates.
5. **Reading retro files of unrelated epics.** Only read retros relevant to the
   current change's subsystem or failure pattern.
