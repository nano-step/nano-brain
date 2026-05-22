# Feature Intake

<!-- generated-by: harness-init v0.1.0 -->
<!-- project: nano-brain -->

Every implementation prompt enters the intake gate before code changes.
The human does not need to classify risk. The harness does.

## Intake Flow

```text
User prompt
    |
    v
Create GitHub issue (skeleton)        ← gh issue create --repo nano-step/nano-brain
    |                                    (skip only for pure questions / read-only)
    |
    v
Classify input type
    |
    v
Restate as work item
    |
    v
Find affected product docs and stories
    |
    v
Run risk checklist
    |
    v
Choose lane: tiny, normal, or high-risk
    |
    v
Update issue with lane + change-type labels
```

## Step 0 — Create GitHub Issue

**Before classifying anything**, create a tracking issue:

```bash
gh issue create \
  --repo nano-step/nano-brain \
  --title "<concise restatement of intent>" \
  --body "$(cat <<'EOF'
## Intent
<verbatim user request, or paraphrased with stated assumptions>

## Lane
TBD

## Change Type
TBD

## Proposal
TBD

## Acceptance Criteria
TBD

## Progress
- [ ] Feature Intake
- [ ] Proposal + design
- [ ] Deep-design (Metis + Oracle)
- [ ] Specs + Story packet
- [ ] Implementation
- [ ] User-flow test
- [ ] Review Gate
- [ ] PR opened + bot review
- [ ] Merged + archived
EOF
)"
```

Record the returned issue number (`#N`). This is the **harness tracking ID**
for the entire flow. Update it at every milestone (see HARNESS.md
§ GitHub Issue Tracking).

**Skip issue creation only for:**
- Pure questions / explanations (no deliverable expected)
- Read-only exploration
- Live setup tasks where user is the orchestrator
- Tasks that revise the harness itself (those go via `HARNESS_BACKLOG.md`)

When unsure: **create the issue**. Closing is cheap.


## Input Types

Use the input type to decide where the work should land before choosing the risk
lane.

| Type | Use when | Typical artifact |
| --- | --- | --- |
| New spec | Turning a user-provided project spec into harness-ready docs | Product docs, candidate epics, decisions |
| Spec slice | Implementing selected behavior from an accepted spec | Story packet |
| Change request | Changing, fixing, or refining accepted behavior | Story packet or direct patch |
| New initiative | Adding a larger product area that needs multiple stories | Initiative notes plus story packets |
| Maintenance request | Changing technical, operational, or dependency behavior | Story packet, validation report, or decision |
| Harness improvement | Improving how humans and agents collaborate | Direct docs update or `docs/HARNESS_BACKLOG.md` |

## Lanes

### Tiny

Low-risk docs, copy, config, or narrow single-file edits.

Steps:
1. Patch directly.
2. Run `validate:quick` — `npm run build && vitest run`.
3. Update affected docs if changed.
4. Log friction to `HARNESS_BACKLOG.md` if found.
5. Close issue #N with single comment containing the diff + validate output.

No proposal required. PR Bot Review still applies if pushing remotely.

---

### Normal

Story-sized behavior with bounded blast radius (1-3 risk flags).

Steps:
1. **Propose**
   ```bash
   openspec new change "<kebab-name>"
   ```
   Write `proposal.md` and `design.md`. Include `Tracking: #N` at top of `proposal.md`.
      → `gh issue comment <N> --body "Proposal: <location>"`

2. **Deep-design gap analysis** *(if available)*
   ```
   /deep-design
   ```
   - If gaps found → revise artifacts → re-run deep-design.
   - Proceed only on clean pass.
      → `gh issue comment <N> --body "Deep-design synthesis: <gaps + resolutions>"`

3. **Generate specs + story packet**
   ```bash
   openspec instructions specs --change "<name>"
   openspec validate "<name>" --strict
   ```
   Create `docs/stories/<name>.md` from `docs/templates/story.md`.
      Set `github_issue: #N` in story frontmatter.
   Update `docs/TEST_MATRIX.md`.
      → `gh issue comment <N> --body "Acceptance criteria: <paste from spec>"`

4. **Implement**
   ```
   /opsx-apply
   ```
   Keep `npm run build && vitest run` green on every commit.
      → Issue update: tick off tasks in Progress checklist as completed.
   → For multi-day work, post status comment every ~3 substantive commits.

5. **Validate**
   Run `validate:quick` + `test:integration`. Paste output in story Evidence.

5b. **User-flow test** (skip if change type = infra/refactor/docs)
   Run at least 1 test through the user's entry point matching the changed
   surface (see HARNESS.md § User-Flow Testing). Paste command + output in
   story Evidence section.
      → `gh issue comment <N> --body "User-flow test PASS: <command + output>"`

5c. **Review Gate** (skip if change type = infra/refactor/docs)
   Spawn a fresh review agent to verify each acceptance criterion against
   evidence. Reviewer ≠ implementer. Paste Review Verdict in story Evidence.
   Proceed only on PASS.
      → `gh issue comment <N> --body "Review Gate: PASS — <verdict table>"`

5d. **PR + Bot Review Loop**
   Push branch, open PR with `Closes #N` in body. Address bot review comments (fix or
   reasoned reply). Re-run validate + user-flow + Review Gate if implementation
   changes. Loop until bot approves (max 3 push cycles → escalate to human).

6. **Close**
   ```bash
   openspec archive "<name>"
   ```
      Merge PR → issue auto-closes via `Closes #N`. Verify it closed.
   Update story status and `docs/TEST_MATRIX.md` with evidence + Review Verdict.

---

### High-Risk

Touches auth, authorization, data-model, audit-security, external-providers, public-api-contracts, search-quality, embedding-vector-provider, or multi-domain scope (4+ risk flags, or any hard gate).

Steps:
1. **Propose** — same as Normal, plus fill design.md in full detail.
      → Issue: link proposal + apply `lane:high-risk` label.

2. **Deep-design gap analysis** — mandatory; do not skip.
   - All blocking gaps must be resolved before proceeding.
   - Record architecture decisions in `docs/decisions/`.
      → Issue: paste full Metis + Oracle synthesis as a comment.

3. **Human confirmation** — present synthesis to human; get explicit go-ahead
   before writing any spec.
      → Issue: comment "Human approved: <date> — proceeding to specs".

4. **Generate specs + story folder**
   ```bash
   openspec instructions specs --change "<name>"
   openspec validate "<name>" --strict
   ```
   Create story folder from `docs/templates/high-risk-story/`.
   Fill `overview.md`, `design.md`, `execplan.md`, `validation.md`.

5. **Implement** — same as Normal.

6. **Validate**
   Run `validate:quick` + `test:integration` + `test:e2e`. Paste output in
   story Evidence.

6b. **User-flow test + evidence artifacts**
   Run user-flow tests covering **primary path + at least 1 error/edge path**.
   For web changes: capture screenshots to `docs/evidence/<name>/`.
   For bot/chat: paste simulator output showing each user step.
   Paste all command outputs in story Evidence section.

6c. **Review Gate (full)**
   Run full review-work skill (5 parallel sub-agents). All must pass.
   Reviewer ≠ implementer. Paste Review Verdict + per-criterion evidence
   table in story Evidence section. Proceed only on PASS.
      → Issue: paste full review-work verdict + per-criterion evidence.

6d. **PR + Bot Review Loop**
   Push branch, open PR with `Closes #N` in body. Address every bot review comment
   substantively. Re-run validate + user-flow + Review Gate on each substantive
   push. Loop until bot approves (max 3 cycles → escalate to human).

7. **Close**
   ```bash
   openspec archive "<name>"
   ```
      Merge PR → issue auto-closes via `Closes #N`. Verify it closed.
   Record decision in `docs/decisions/` if architecture changed.
   Update `docs/TEST_MATRIX.md` with evidence + Review Verdict.

## Risk Checklist

Mark one flag for each item that applies:

| Risk flag | Applies when the work touches |
| --- | --- |
| Auth | Applies when the work touches login, logout, sessions, JWT, password, refresh token |
| Authorization | Applies when the work touches roles, permissions, tenant or company scope |
| Data model | Applies when the work touches schema, migrations, uniqueness, deletion, retention |
| Audit/security | Applies when the work touches audit logs, privacy, sensitive data, access logs |
| External systems | Applies when the work touches email, payments, cloud services, provider SDKs, queues, webhooks |
| Public contracts | Applies when the work touches API shape, response envelope, client-visible behavior |
| Cross-platform | Applies when the work touches desktop/mobile/browser split, native shell behavior, deep links |
| Existing behavior | Applies when the work touches already implemented or test-covered behavior changes |
| Weak proof | Applies when the work touches unclear or missing tests around the affected area |
| Multi-domain | Applies when the work touches more than one product domain changes at once |

## Classification

```text
0-1 flags:
  tiny or normal, based on code impact

2-3 flags:
  normal with stronger validation

4+ flags:
  high-risk

Any hard gate:
  high-risk unless the human explicitly narrows scope
```

Hard gates: auth, authorization, data-model, audit-security, external-providers, public-api-contracts, search-quality, embedding-vector-provider.

## Output

At the end of intake, the agent must state lane + change type + planned gates:

```text
Issue: nano-step/nano-brain#42
Lane: normal
Change type: user-feature
Reason: touches API contract and existing behavior (2 flags).
Proposal: <link>
Validation: validate:quick + test:integration
User-flow test: matching changed surface
Review Gate: single Oracle review
PR Bot Review: required (max 3 push cycles)
```

For infrastructure/migrations (no user surface):
```text
Issue: nano-step/nano-brain#42
Lane: normal
Change type: infrastructure
Reason: data model touched (1 flag), no user-visible behavior.
Proposal: <link>
Validation: validate:quick + test:integration
User-flow test: not applicable — change type exempt
Review Gate: self-verify
PR Bot Review: required (max 3 push cycles)
```

For tiny lane:
```text
Issue: nano-step/nano-brain#42
Lane: tiny
Change type: docs
Reason: single-file change, 0 risk flags.
Action: patch directly, run validate:quick.
No proposal, no Review Gate, no user-flow test required.
PR Bot Review: still required if pushing to remote.
```
