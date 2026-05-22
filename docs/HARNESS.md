# Harness

<!-- generated-by: harness-init v0.1.0 -->
<!-- project: nano-brain -->

The app is what users touch. The harness is what agents touch.

This harness classifies every change by risk lane, requires a proposal-and-review
cycle for non-trivial changes, and enforces a validation + user-flow test +
review gate before any work is archived.

## Mental Model

```text
┌─────────────────────┐
│   Human intent      │
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  GitHub Issue       │  gh issue create --repo nano-step/nano-brain
│  (skeleton)         │  title from user intent, lane TBD, body = raw request
│                     │  → returns #N (tracker for the whole flow)
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Feature Intake     │  classify risk → choose lane
│                     │
│                     │  → update issue: add lane:* + change-type:* labels
└────────┬────────────┘
         │
         ├── tiny ──► patch + validate + close issue #N (single comment with diff)
         │
         ▼  normal / high-risk
┌─────────────────────┐
│  Propose            │  openspec new change "<name>" → proposal.md + design.md + tasks.md
│                     │  → update issue #N: link proposal location
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Deep-Design        │  spawn deep-design agent → find gaps, ambiguities, risks
│  Gap Analysis       │  (Metis + Oracle in parallel → cross-critique → synthesis)
└────────┬────────────┘
         │
         ├── gaps found ──► revise proposal/design ──► re-run deep-design
         │
         ▼  clean pass

┌─────────────────────┐
│  Specs + Story      │  acceptance criteria per behavior slice
│                     │  story in docs/stories/ (link proposal + issue)
│                     │  update docs/TEST_MATRIX.md with expected proof
│                     │  → update issue #N: paste synthesis + acceptance criteria
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Implement          │  work through tasks list
│                     │  npm must stay green
│                     │
│                     │  → update issue #N: tick off tasks as completed
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│  Validate           │  run validation ladder appropriate to lane
└────────┬────────────┘
         │
         ├── fail ────► fix → re-validate (max 2 attempts before consulting Oracle)
         │
         ▼  pass
┌─────────────────────┐
│  User-Flow Test     │  run test through user's entry point matching changed surface
│                     │  Exempt if change type = infra/refactor/docs (see § Change Types)
└────────┬────────────┘
         │
         ├── fail ────► fix → re-test (max 2 attempts)
         │
         ▼  pass
┌─────────────────────┐
│  Review Gate        │  fresh review agent verifies each acceptance criterion
│                     │  Reviewer ≠ implementer. Cite evidence per criterion.
│                     │  → update issue #N: paste Review Verdict + evidence table
└────────┬────────────┘
         │
         ├── FAIL ────► fix → re-review (max 1 re-review before consulting human)
         │
         ▼  PASS
┌─────────────────────┐
│  PR + Bot Review    │  push branch → open PR (gh pr create --body 'Closes #N')
│  Loop               │  automated PR review
│                     │  agent reads PR comments → fix → re-validate → re-test
└────────┬────────────┘
         │
         ├── bot comments ──► triage → fix or justify → push again
         │
         ▼  approved
┌─────────────────────┐
│  Harness Delta      │  merge PR → openspec archive "<name>"
│                     │  update docs/stories/, docs/decisions/, docs/TEST_MATRIX.md
│                     │  capture friction → HARNESS_BACKLOG.md if needed
│                     │  → close issue #N with link to merged PR
└────────┬────────────┘
         │
         ▼
┌─────────────────────┐
│   Next intent       │
└─────────────────────┘
```

Every task has two possible outputs:

1. **Product delta**: app code, tests, API shape, data model, or product docs.
2. **Harness delta**: docs, templates, validation expectations, backlog items, or
   decision records that make the next task easier.

## Source Hierarchy

```text
Human intent / prompt
  └── GitHub Issue tracker (nano-step/nano-brain)
  └── Feature Intake (docs/FEATURE_INTAKE.md)
        └── OpenSpec change proposal (openspec/changes/<name>/)
              ├── proposal.md   — what and why
              ├── design.md     — how (architecture, data model, API shape)
              ├── specs/        — one spec per behavior slice
              └── tasks.md      — implementation checklist
        └── Story packet (docs/stories/<name>.md)
              └── links to OpenSpec change, lists acceptance criteria
        └── docs/TEST_MATRIX.md
              └── maps each story to unit / integration / E2E proof
        └── docs/decisions/
              └── records why contracts or architecture changed
```

Before implementation, product docs and proposal artifacts describe intent.
After implementation, those artifacts plus passing tests are the living contract.

## OpenSpec Integration

OpenSpec is the **proposal and design layer** of this harness. Every normal or
high-risk change must have an OpenSpec change before implementation starts.

### Commands
```bash
openspec new change "<name>"            # scaffold change directory
openspec validate "<name>" --strict     # validate all artifacts
openspec archive "<name>"               # archive after merge
```

## Deep-Design Gap Analysis

After the proposal produces `proposal.md` and `design.md`, run **deep-design**
before locking any spec.

- Spawns Metis (scope/risk) + Oracle (architecture) in parallel
- Cross-critiques their findings
- Produces a confidence-scored synthesis: gaps, ambiguities, hidden risks

### Gate rule

```text
deep-design pass (no blocking gaps)
  → proceed to specs/ + story packet

deep-design finds gaps
  → revise proposal.md or design.md
  → re-run deep-design
  → repeat until clean pass
```

A gap is blocking if it touches: auth, data model, API contract, isolation
boundary, or multi-domain scope. Stylistic gaps are non-blocking.

## Spec Lifecycle

Ongoing work enters the harness as one of these input types:

| Type | What to do |
|---|---|
| New spec | Populate `docs/product/`, create candidate story list, run deep-design on scope |
| Spec slice | Propose → deep-design → specs/ → story → implement |
| Change request | Propose → deep-design (if normal+) → story → implement |
| New initiative | Initiative notes in `docs/stories/` + multiple proposals |
| Maintenance | Story packet only (no proposal required for tiny) |
| Harness improvement | Direct docs update or `HARNESS_BACKLOG.md` |

Do not extend a monolithic spec. Use change proposals + story packets as the
living surface.

## Growth Rule

The harness grows from friction.

When an agent is confused, repeats manual reasoning, needs a new validation
command, discovers a missing rule, or sees a recurring failure pattern, it must
either improve the harness directly or add a proposal to `HARNESS_BACKLOG.md`.

## Validation Ladder

Run the layers appropriate to the lane. Never claim a layer passes without
running it and seeing exit code 0.

```text
validate:quick   (always — every lane)
  npm run build && vitest run

test:integration   (normal + high-risk)
  # N/A

test:e2e   (high-risk or when UI behavior changes)
  # N/A

test:release   (before deploy)
  npx nano-brain status
```

**Lane → required layers:**

| Lane | validate:quick | test:integration | test:e2e |
|------|:-:|:-:|:-:|
| tiny | ✓ | — | — |
| normal | ✓ | ✓ | — |
| high-risk | ✓ | ✓ | ✓ |

Agents must not claim a layer passes until it has been run and output verified.

## Change Types

The validation ladder is necessary but not sufficient. The **change type**
determines whether user-flow testing and review gate apply.

| Change type | E2E required? | Review gate? | Example |
|-------------|:-:|:-:|---|
| **user-feature** (new behavior, new surface) | ✅ | ✅ | new endpoint, new UI page |
| **bug-fix** (user-visible defect) | ✅ | ✅ | "OTP not arriving", broken response |
| **infrastructure** (migrations, config, deploy) | ❌ smoke test sufficient | ⚠️ self-verify | DB migration, env var change |
| **refactor** (same I/O) | ❌ existing tests pass | ⚠️ self-verify | extract helper, rename internal symbol |
| **docs** (markdown / comments only) | ❌ | ❌ | README, ADR write-up |
| **dependency-bump** | ❌ smoke test | ⚠️ self-verify | upgrade library version |

**Combined gate:** Lane × Change Type. Both must pass to proceed.

For change types marked **❌ smoke test** instead of E2E:
- Run a deterministic check that exercises the changed surface (e.g.
  `alembic upgrade head` for migrations, `import <app>` for refactors).
- Paste the output in story Evidence section.
- No user-flow test required — there is no user surface to test.

For change types marked **⚠️ self-verify**:
- Implementing agent runs the validation ladder and pastes output.
- No independent review agent required.
- Still subject to PR bot review (see below).

## User-Flow Testing

After validation ladder passes, run at least one test that exercises the
changed behavior through the **user's actual entry point**. Choose the tool
that matches the changed surface:

| Changed surface | Tool | Command |
|---|---|---|
| Bot / chat handler | Command simulator | `# N/A` |
| Web UI | Playwright / Cypress | `# N/A` |
| REST API | API integration test | `# N/A` |
| Backend-only (no user surface) | Existing integration tests | `# N/A` |
| LLM / external service call | Live smoke script | `npx nano-brain status` |

**Lane × user-flow requirement:**

| Lane | User-flow test required? |
|------|:-:|
| tiny | No (escalate to normal if user-visible behavior changes) |
| normal | Yes — at least 1 test covering the primary changed behavior |
| high-risk | Yes — cover primary + at least 1 error/edge path |

**E2E not applicable**: If change type is infra/refactor/docs/deps, write
"E2E: not applicable — [reason]" in the story Evidence section. The review
gate validates this justification.

**Happy-path-only is insufficient for high-risk**: at minimum cover one
error/edge path (auth fail, rate limit, malformed input, etc.).

## Review Gate

After user-flow tests pass, a **fresh review agent** verifies the implementation.
The reviewer **must not be** the implementing agent.

**What the reviewer checks:**
1. Read `git diff <default-branch>` + the proposal, design, and spec.
2. For each acceptance criterion, find evidence (test output, screenshot,
   command result) that it is satisfied.
3. Produce a verdict: **PASS** (all criteria met with evidence) or **FAIL**
   (list unmet criteria + missing evidence).

**Lane × Change Type → review requirement:**

| Lane | user-feature / bug-fix | infra / refactor / deps | docs |
|------|---|---|---|
| tiny | n/a (escalate if user-visible) | self-verify | none |
| normal | Single Oracle review | self-verify | none |
| high-risk | Full review-work skill (5 parallel sub-agents) | single Oracle | n/a |

**Review output format:**

```text
## Review Verdict: PASS | FAIL

Reviewer: <agent name>
Date: YYYY-MM-DD
Commit: <sha>

| Acceptance Criterion | Evidence | Status |
|---|---|---|
| "Users can upload receipt photo" | test_receipt_upload.py passes (output below) | ✓ |
| "Items appear in inventory" | simulator output shows items listed | ✓ |

Unmet criteria (if FAIL):
- [criterion] — missing [evidence type]
```

**Rule:** `openspec archive "<name>"` is forbidden until Review Verdict = PASS.

## PR + Bot Review Loop

After the local Review Gate passes, push branch and open a PR. The PR triggers your configured automated reviewer.

```text
1. Push branch + open PR
        │
        ▼
2. PR bot posts review comments
        │
        ├── comments substantive ──► agent reads → fix → push
        │                            │
        │                            ▼
        │                   re-run validate + user-flow test
        │                            │
        │                            ▼
        │                   if substantive impl change → re-run Review Gate
        │                            │
        │                            ▼
        │                   wait for bot re-review
        │
        ├── comments stylistic only ─► address inline or reply with reason
        │
        ▼
3. Bot approves → merge → openspec archive "<name>"
```

**Rules for handling PR comments:**

- **Read every comment.** Do not collapse / dismiss without action or reasoned reply.
- **Substantive comment** (correctness, security, missing case): MUST fix.
  After fix, re-run validate + user-flow + Review Gate before pushing.
- **Stylistic comment** (naming, ordering, preference): fix if cheap, or reply
  with reasoning and tag for human review.
- **Disagreement**: do NOT silently dismiss. Reply with rationale; tag human.
- **Loop limit**: max 3 push cycles per PR. After 3, escalate to human review.
- **Never**: force-push to bypass bot, dismiss without reading, or merge
  without bot approval (unless human override documented in PR).

The PR review loop is not optional. It is the final correctness gate before
the change becomes part of the trunk.

## Forbidden Practices

1. **Claiming "tests pass" without output.** Paste the command and its exit code.
   A claim without evidence is not a claim.
2. **Self-review.** The implementing agent must not perform its own Review Gate.
   Use review-work skill or spawn a fresh review agent.
3. **Skipping user-flow tests for "refactors."** If the refactor changes
   observable behavior (response shape, timing, error messages, side effects),
   it needs a user-flow test. Only pure internal refactors (identical I/O)
   qualify as "E2E not applicable."
4. **Happy-path-only E2E for high-risk changes.** High-risk must cover at least
   one error or edge path.
5. **Archiving without review verdict.** openspec archive "<name>" is blocked until
   the story shows Review Verdict = PASS with per-criterion evidence.
6. **Backdating evidence.** Evidence must reference the current implementation
   commit, not a previous passing run.
7. **Force-pushing to bypass PR bot review.** PR bot must approve or be
   overridden by documented human decision.
8. **Dismissing PR comments without action or reasoned reply.** Every
   substantive comment requires a fix or a documented disagreement.
9. **Starting work without a GitHub issue.** Every new user request (except
   pure conversational queries) must have a GitHub issue created BEFORE
   classification. Working without an issue ID = invisible work.
10. **Stale issue.** If implementation progresses but the issue isn't updated
    at the milestones in § GitHub Issue Tracking, the change is in violation.

## GitHub Issue Tracking

Every user request that triggers harness work (not a pure question) gets a
GitHub issue in `nano-step/nano-brain`. **Create early, update at every milestone.**

### When to create

**Create immediately after Intent Gate, BEFORE Feature Intake classification.**

The issue starts as a skeleton with the raw user request. It evolves as the
flow progresses.

**Skip issue creation for:**
- Pure conversational questions ("how does X work?")
- Read-only exploration that doesn't produce a deliverable
- Interactive setup tasks initiated by the user

When unsure: create the issue. Closing is cheap.

### Issue lifecycle

| Phase | Action | Command |
|-------|--------|---------|
| Intent | Create skeleton issue | `gh issue create --repo nano-step/nano-brain --title "<intent>" --body "<raw request + assumptions>"` |
| Intake | Add lane + change-type labels | `gh issue edit <N> --add-label "lane:normal,change-type:user-feature"` |
| Proposal | Comment with location | `gh issue comment <N> --body "Proposal: <location>"` |
| Deep-design | Comment with synthesis | `gh issue comment <N> --body "Deep-design: $verdict"` |
| Specs | Comment with acceptance criteria | `gh issue comment <N> --body "Acceptance: ..."` |
| Implementation | Comment per major task | `gh issue comment <N> --body "Implemented: ..."` |
| User-flow test | Comment with proof | `gh issue comment <N> --body "User-flow PASS: ..."` |
| Review Gate | Comment Review Verdict | `gh issue comment <N> --body "Review: PASS — ..."` |
| PR | Link PR to issue | `gh pr create ... --body "Closes #<N>"` |
| Archive | Close issue | auto-closed by PR merge (via `Closes #N`) |

### Labels

Apply `lane:*` + `change-type:*` (+ optional `status:*`) labels as soon as
classification completes. See `scripts/setup_labels.sh` in this skill or run:

```bash
bash ~/.config/opencode/skills/harness-init/scripts/setup_labels.sh nano-step/nano-brain
```

