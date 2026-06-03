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
│                     │  build + tests must stay green
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

## Two-Output Model (MANDATORY)

Every task produces ONE or BOTH of these outputs. Both are first-class.

1. **Product delta** — app code, tests, API shape, data model, or product docs.
2. **Harness delta** — changes to HARNESS.md, gates, templates, validation
   expectations, backlog items, ADRs (`docs/decisions/`), GLOSSARY, CONTEXT_RULES,
   TRACE_SPEC, or scripts that make the next task easier.

A task that produces ONLY a Harness Delta (no product change) is legitimate work
— record it the same way.

**Decision protocol when friction is discovered mid-task:**
- Fix is < 30 min AND in current PR scope → fix inline as Harness Delta (combined PR)
- Else → add entry to `docs/HARNESS_BACKLOG.md`

The story template's "Harness Delta" section is REQUIRED filled-in
(may be "None" but must be present).

See `docs/GLOSSARY.md` for closed-set definitions of all terms used in this doc.
See `docs/CONTEXT_RULES.md` for what to read per phase × lane (token budget).
See `docs/TRACE_SPEC.md` for evidence file formats per tier.
See `docs/decisions/README.md` for when to write ADRs.

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

## Recommended Workflow: `/harness-on`

For autonomous feature development, use the `/harness-on` slash command in OpenCode. It drives the agent through all gates automatically, injecting fix instructions on failure and stopping when all gates PASS.

```
/harness-on          # Start the loop
/harness-off         # Cancel at any time
```

Manual invocation (`./scripts/harness-check.sh <gate> --json`) remains available for ad-hoc checks and CI scripts.

## Validation Ladder

Run the layers appropriate to the lane. Never claim a layer passes without
running it and seeing exit code 0.

```text
validate:quick   (always — every lane)
  go build ./... && go test -race -short ./...

self-review:response-shape   (user-feature change type only)
  For each new REST endpoint and MCP tool added or modified:
  1. Read the response struct definition.
  2. Read the mapping loop that populates it.
  3. Verify every declared field is explicitly assigned (no zero-value gaps).
  4. If a field is populated from a secondary source (e.g. JSONB metadata),
     verify the unmarshal path exists and is tested.
  This check runs BEFORE push, takes < 2 minutes, and catches
  "struct has fields but loop doesn't fill them" bugs that tests won't catch.

self-review:staged-files   (every lane, before every commit)
  Run `git status` and read the staged file list before committing.
  Confirm no .opencode/ metadata, package-lock.json, or empty doc scaffolds.
  Never run `git add -A` without this step.

test:integration   (normal + high-risk)
  go test -race -tags=integration ./...

smoke:e2e   (normal + high-risk, for user-feature and bug-fix change types)
  Build binary → start real server → curl endpoints → verify responses.
  This is NOT a Go test file. It is a real usage test:
  1. go build -o ./bin/nano-brain ./cmd/nano-brain/
  2. Start server with real PG (NANO_BRAIN_DATABASE_URL, NANO_BRAIN_SERVER_PORT=3199)
  3. Wait for GET /health → {"ready":true}
  4. Exercise changed endpoints via curl (POST/GET with real payloads)
  5. Verify HTTP status codes and response JSON structure
  6. Kill server
  Agent performs these steps manually (no script required), pastes evidence.

smoke:ui   (any PR touching web/src, internal/server/handlers, internal/server/webui,
            internal/server/routes.go, or web/package.json — issue #285)
  Verify embedded UI assets serve correctly via HTTP. Catches the class of
  bugs from #275 (missing JS asset), #277 (workspaces shape), #278/#279
  (stats shape), #281 (documents endpoint missing).

  ```bash
  ./scripts/smoke-ui.sh > docs/evidence/<change-slug>/smoke-ui-output.log 2>&1
  ```

  The script:
  1. Builds dev binary at /tmp/nano-brain-smoke/nano-brain
  2. Starts it on port 3199 with --serve-only --unsafe-no-auth
  3. Waits for /health → ready: true
  4. Fetches /ui/ and asserts DOCTYPE + <script> tag present
  5. Parses asset URLs from HTML
  6. For each /ui/assets/*.js: asserts HTTP 200 + body NOT starting with
     <!DOCTYPE html> + size > 1024 bytes
  7. For each /ui/assets/*.css: asserts HTTP 200 + size > 100 bytes
  8. Tears down server
  Final line of log MUST contain "smoke:ui PASS" for the pre-merge gate.

test:release   (before deploy)
  ./nano-brain status
```

**Lane → required layers:**

| Lane | validate:quick | test:integration | smoke:e2e | smoke:ui |
|------|:-:|:-:|:-:|:-:|
| tiny | ✓ | — | — | ✓ (if web-touching) |
| normal | ✓ | ✓ | ✓ (if user-feature or bug-fix) | ✓ (if web-touching) |
| high-risk | ✓ | ✓ | ✓ | ✓ (if web-touching) |

A PR is "web-touching" if its diff includes any of: `web/src/**`,
`web/package.json`, `internal/server/handlers/**`, `internal/server/webui/**`,
or `internal/server/routes.go`. The pre-merge gate enforces this via check 3.8.

Agents must not claim a layer passes until it has been run and output verified.

## Change Types

The validation ladder is necessary but not sufficient. The **change type**
determines whether user-flow testing and review gate apply.

| Change type | smoke:e2e required? | smoke:ui required? | Review gate? | Example |
|-------------|:-:|:-:|:-:|---|
| **user-feature** (new behavior, new surface) | ✅ build+start+curl | ✅ if web-touching | ✅ | new endpoint, new UI page |
| **bug-fix** (user-visible defect) | ✅ build+start+curl | ✅ if web-touching | ✅ | nil panic, broken response |
| **infrastructure** (migrations, config, deploy) | ❌ validate:quick sufficient | ✅ if web-touching | ⚠️ self-verify | DB migration, env var change |
| **refactor** (same I/O) | ❌ existing tests pass | ✅ if web-touching | ⚠️ self-verify | extract helper, rename internal symbol |
| **docs** (markdown / comments only) | ❌ | ❌ | ❌ | README, ADR write-up |
| **dependency-bump** | ❌ validate:quick | ✅ if web/package.json changed | ⚠️ self-verify | upgrade library version |

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

## User-Flow Testing (smoke:e2e)

After validation ladder passes, run at least one test that exercises the
changed behavior through the **user's actual entry point**. This means
**real usage**: build the binary, start the server, call the API, verify
the response.

### How to run smoke:e2e

```bash
# 1. Build
go build -o ./bin/nano-brain ./cmd/nano-brain/

# 2. Start server (background, non-default port to avoid conflicts)
NANO_BRAIN_DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable" \
NANO_BRAIN_SERVER_PORT=3199 \
NANO_BRAIN_EMBEDDING_PROVIDER="" \
./bin/nano-brain &
SERVER_PID=$!

# 3. Wait for health
for i in $(seq 1 15); do curl -sf http://localhost:3199/health >/dev/null && break; sleep 1; done

# 4. Exercise endpoints (example: init workspace + write + search)
curl -sf -X POST http://localhost:3199/api/v1/init -H 'Content-Type: application/json' \
  -d '{"root_path":"/tmp/e2e-test"}'
# ... exercise changed endpoints, verify HTTP status + JSON structure ...

# 5. Kill server
kill $SERVER_PID; wait $SERVER_PID 2>/dev/null
```

### What to verify per endpoint type

| Changed surface | Verify | Example |
|---|---|---|
| New REST endpoint | HTTP status + response JSON shape | POST /api/v1/query → 200 + `{results:[...]}` |
| Bug fix (REST) | Previously broken request now works | POST /api/v1/write without embedding → 201 (was panic) |
| Backend-only (no user surface) | Existing tests sufficient | `go test` covers it |
| LLM / external service | Health check + basic call | GET /health + POST /api/v1/embed |

### Evidence format

Paste the curl commands and responses in the story Evidence section or PR description.
Agent MUST NOT claim smoke:e2e passes without showing the actual curl output.

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

**R31: Agent-triaged comment verdicts (do NOT trust Gemini's severity labels).**

Every Gemini PR comment MUST be triaged by the agent. Gemini lacks context
(codebase patterns, story intent, prior decisions); the agent has that context
and is the source of truth. Triage is recorded in the self-review evidence file
at `docs/evidence/self-review-<story-slug>.md` under section
`## Gemini Verification Triage`.

**Triage table format (exact columns required):**

```markdown
## Gemini Verification Triage

| Comment ref       | Agent verdict       | Reasoning                          | Action                |
| ----------------- | ------------------- | ---------------------------------- | --------------------- |
| PR#42 line 15     | VALID:critical      | actual nil panic risk              | fixed in commit abc12 |
| PR#42 line 23     | FALSE_POSITIVE      | intentional per ADR-003            | acknowledged in reply |
| PR#42 line 31     | DEFER               | out of scope; tracked in #99       | added to backlog      |
```

**Verdict vocabulary (closed set — use these literal strings only):**

| Verdict           | Meaning                                                        | Blocks merge? |
| ----------------- | -------------------------------------------------------------- | :-----------: |
| `VALID:critical`  | Agent confirms; correctness/security/data loss                 | YES           |
| `VALID:high`      | Agent confirms; significant bug or design issue                | YES           |
| `VALID:medium`    | Agent confirms; fix optional but must acknowledge in PR reply  | no            |
| `VALID:low`       | Agent confirms; cosmetic/nit; acknowledge in PR reply          | no            |
| `FALSE_POSITIVE`  | Agent has context Gemini lacks; explain in Reasoning column    | no            |
| `DEFER`           | Valid but out of current PR scope; link to backlog/follow-up   | no            |
| `ACKNOWLEDGED`    | Informational comment, no action needed                        | no            |

**Resolution rules:**

- Every `VALID:critical` and `VALID:high` row MUST contain `fixed in commit <sha>`
  in the Action column, OR the PR must have a `[HARNESS-OVERRIDE]` comment (see R7).
- The number of triage table rows MUST equal the number of Gemini bot comments on
  the PR (no comment left untriaged).
- **Loop limit:** max 3 push cycles per PR. After 3, escalate to human review.

**Enforced by:** gate 3.6 (PRE-MERGE) — parses triage table, counts Gemini comments
vs triage rows, blocks merge on unresolved `VALID:critical`/`VALID:high` without
fix commit or override.

The PR review loop is not optional. It is the final correctness gate before
the change becomes part of the trunk.

## Harness Gate Enforcement

All development transitions are governed by the gate specification in
[`docs/HARNESS_GATES.md`](HARNESS_GATES.md). Six gates form the lifecycle:

```
① PRE-WORK → ② IN-PROGRESS → ③ PRE-MERGE → ④ POST-MERGE → ⑤ NEXT-READY → ⑥ RETRO-GATE
```

**Core enforcement rules:**

1. **R1: 1 feature = 1 PR = 1 GitHub issue.** No bundling multiple features.
   - PR body MUST contain exactly ONE `Closes #<N>` reference.
   - If a PR closes 2+ issues, split into separate PRs.
   - **Enforced by:** gate 3.8 (PRE-MERGE) — counts `Closes #` occurrences in PR body. FAIL if count ≠ 1.
2. **R3: All gates must PASS** before proceeding to the next phase.
3. **R4: FAIL = BLOCK.** Agent must fix failures before continuing.
4. **R5: Agent MUST NOT start the next feature** until ⑤ NEXT-READY passes.

Run gates via: `./scripts/harness-check.sh <phase> [options]`

The `harness-check` skill (`.opencode/skills/harness-check/`) provides
agent-side enforcement and is invoked automatically at transition points.

### Retro Gate (⑥)

After every epic completes, a mandatory retrospective analyzes failure patterns
and proposes harness rule improvements:

- **Mandatory trigger:** Last story of epic merges.
- **Emergency trigger:** 3+ consecutive stories fail review gate mid-epic.
- **Flag trigger:** Any PR with review cycle count > 2.

Retro output is saved to `docs/evidence/retro-epic-{N}.md`. Any proposed
harness rule changes **require user approval** before being applied.

See `docs/HARNESS_GATES.md` for the full gate specification, check details,
and retro output template.

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
7. **R7: Force-pushing to bypass PR bot review.** PR bot must approve OR the user
   must post an explicit override comment.
   - **Override mechanism:** User account posts PR comment containing exact literal
     string `[HARNESS-OVERRIDE]: <reason>` where `<reason>` is ≥ 20 characters.
   - The override applies ONLY to the PR head commit at time of posting.
     Any subsequent push invalidates the override.
   - **Enforced by:** gate 3.6 (PRE-MERGE) — if `[HARNESS-OVERRIDE]: ...` matched in
     PR comments and length ≥ 20, gate 3.6 returns PASS regardless of unresolved
     Gemini findings.
8. **Dismissing PR comments without action or reasoned reply.** Every
   substantive comment requires a fix or a documented disagreement.
9. **Starting work without a GitHub issue.** Every new user request (except
   pure conversational queries) must have a GitHub issue created BEFORE
   classification. Working without an issue ID = invisible work.
10. **Stale issue.** If implementation progresses but the issue isn't updated
    at the milestones in § GitHub Issue Tracking, the change is in violation.
11. **Starting next feature with gates failing.** All gates (① – ⑤) of the
    current feature must PASS before starting the next. No exceptions.
12. **Skipping retro after epic.** The retro gate (⑥) is mandatory after
    every epic. Skipping it prevents process improvement.
13. **Modifying harness rules without user approval.** Retro-proposed rule
    changes must be approved by the user before being applied to HARNESS.md
    or HARNESS_GATES.md.
14. **`_ = err` on constructor calls in `main.go` or any startup path.**
    Use `log.Warn` + skip the nil value, or `log.Fatal` if the component is
    critical. The `_` discard is only permitted in deferred cleanup
    (e.g. `defer f.Close()`). Concrete pattern for optional components:
    ```go
    goE, err := symbol.NewGoExtractor()
    if err != nil {
        logger.Warn().Err(err).Msg("go extractor init failed, skipping")
    }
    // Pass only non-nil values to registry
    ```

## HUMAN-ONLY Rules (not enforced by harness-check.sh)

These rules apply but cannot be automated by gate checks. They are caught at PR
review time by a human or by another agent acting as reviewer — NOT by
`scripts/harness-check.sh`. Violations are real violations; they just don't
trip an automatic gate FAIL.

| Rule | Statement | Why human-only |
|---|---|---|
| **R14** | No `_ = err` on constructor calls in `main.go` or startup paths. | Requires Go AST parsing; not in shell-script scope. Caught by `golangci-lint` errcheck rule + code review. |
| **R26** | Reviewer ≠ implementer for the Review Gate. | No system tracks "who is acting as agent N now"; relies on agent honesty + PR author/reviewer separation. |
| **R30** | Never force-push to bypass bot review. | `git push --force` is detectable but legitimate rebases look similar; false positives would block legit work. Caught by PR audit. |
| **R84** | High-risk stories need explicit human approval before writing specs. | "Human approval" is free-text intent in issue comments; not reliably parseable. Caught at gate 1 user review. |
| **R87** | Update `docs/decisions/` when architecture changes (high-risk). | "Architecture changed" requires domain judgment, not automation. Caught by reviewer at gate 3.5. |

These rules are reviewer responsibility, not script responsibility. The
distinction is made explicit so agents do not assume harness-check.sh covers
everything.

## Manual npm Publish Runbook

Fallback for when the `npm-publish` job in `.github/workflows/release.yml` fails
(`E404`, `ENEEDAUTH`, or token-permission errors) and an unblocked release is
needed before CI is fixed. Use this only when:

- The auto-tag + release pipeline has already produced a valid git tag and
  GitHub Release (binaries are uploaded) — verify via
  `gh release view v<YYYY>.<M>.<DDNN> --repo nano-step/nano-brain`.
- Only the npm registry step failed; binaries on GitHub are intact.
- The fix to CI (token rotation, OIDC migration, etc.) is being tracked
  in a separate GitHub issue and won't land in the next few minutes.

### Prerequisites

1. **npm login as a maintainer** of `@nano-step/nano-brain`. Verify:
   ```bash
   npm whoami                                    # must be nano-step001 or nhonh
   npm view @nano-step/nano-brain maintainers    # confirm whoami is in the list
   ```
2. **Clean master worktree.** Stash any WIP first — the steps below mutate
   `package.json` in-place and you do NOT want those changes mixed with a
   feature branch.
   ```bash
   git checkout master
   git pull --ff-only origin master
   git status --short                            # must be empty
   ```
3. **Target tag exists.** Pick the latest tag the CI tried to publish:
   ```bash
   git tag --list "v$(date -u +%Y.%-m).*" --sort=-v:refname | head -1
   ```

### Steps

```bash
# 1. Bump package.json in-place (do NOT commit — workflow expects 0.0.0-dev on master)
TAG=v2026.5.3004              # use the actual failed tag
VERSION="${TAG#v}"
npm version --no-git-tag-version "$VERSION"

# 2. Dry-run to confirm tarball contents (4 files: README, npm/run.js, npm/postinstall.js, package.json)
npm publish --tag latest --access public --dry-run

# 3. Publish scoped package
npm publish --tag latest --access public

# 4. Publish unscoped alias — rewrite package.json transiently
node -e "const p=require('./package.json'); p.name='nano-brain'; delete p.publishConfig; require('fs').writeFileSync('package.json',JSON.stringify(p,null,2)+'\n')"
npm publish --tag latest

# 5. Restore package.json to its committed state
git checkout package.json

# 6. Verify both packages on the registry
npm view @nano-step/nano-brain version dist-tags
npm view nano-brain version dist-tags
```

### Evidence to capture

Manual publish is an unusual operation — leave an audit trail:

1. **Comment on the failing CI run's GitHub issue** (or the release issue if
   one exists) with: tag published, npm versions of both packages,
   `npm whoami` output (which maintainer published), and a link to the CI
   run that failed.
2. **Open or update a follow-up issue** to actually fix CI (token rotation,
   OIDC migration). Label `change-type:infrastructure`, lane based on root
   cause. Manual publish is not a fix — it's a workaround.

### Forbidden during manual publish

- **Do NOT commit the bumped `package.json`.** `package.json.version` stays
  at `0.0.0-dev` on master — auto-tag rewrites it in-place per tag.
- **Do NOT publish to a different scope or rename the package.** Use the
  exact names `@nano-step/nano-brain` and `nano-brain` (unscoped alias).
- **Do NOT publish without `--access public` for the scoped package on
  the first publish of a version.** npm defaults scoped packages to private.
- **Do NOT publish from a feature/PR branch.** Master HEAD only — the
  published artifact must match what the failed CI run would have shipped.
- **Do NOT skip step 5 (restore `package.json`).** Leaving the bumped
  version unstaged risks an accidental commit on the next unrelated change.

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

