# Trace Specification

A **trace** is a structured record of what an agent did during a task. Traces
feed Retro Gate ⑥ metric collection and serve as institutional memory for
future agents.

This spec defines THREE tiers. Lane determines which tier is required.

## Tier 1 — Minimal (required for `tiny` lane)

A single markdown block at the end of the task, posted as a comment on the
GitHub issue (if any) or appended to the relevant evidence file.

```markdown
## Trace (tier 1)
- **Actions:** 1–4 bullet points of what was done
- **Files changed:** list of file paths (or "none")
- **Duration:** approximate wall time
- **Friction:** anything that slowed the task, or "none"
```

**Pass condition (gate 2.4 / 3.10 — see W3 enforcement):**
- Has the four sections above
- "Friction" is filled in (may be "none")

## Tier 2 — Standard (required for `normal` lane)

A self-review evidence file at `docs/evidence/self-review-<slug>.md`.

```markdown
# Self-Review: <story slug>

## Actions Taken
- (5–15 bullet points; concrete steps, not narrative)

## Files Changed
- <path> — <one-line reason>
- ...

## Findings Summary
- Critical: <count>
- Major: <count>
- Minor: <count>

## Critical
| Finding | Status | Reasoning |
| --- | --- | --- |
| ... | FIXED / DEFERRED | ... |

## Major
(same table)

## Minor
(same table)

## Gemini Verification Triage
| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| ... | VALID:critical / VALID:high / VALID:medium / VALID:low / FALSE_POSITIVE / DEFER / ACKNOWLEDGED | ... | fixed in commit <sha> / acknowledged / ... |

## Resolution Status
- All critical: FIXED | DEFERRED (link)
- All major: FIXED | DEFERRED (link)
- Open items: <list> | none

## Friction
- (anything that slowed the task; goes to HARNESS_BACKLOG.md if out of scope)
```

**Pass condition (gate 2.4):**
- All sections above are present
- Every Critical row has Status ∈ {FIXED, DEFERRED}; DEFERRED rows link to a follow-up issue
- Gemini Verification Triage row count = number of Gemini bot comments on the PR
- All VALID:critical / VALID:high rows have `fixed in commit <sha>` in Action OR PR has `[HARNESS-OVERRIDE]`

## Tier 3 — Detailed (required for `high-risk` lane)

Tier 2 + the following additional sections in the same evidence file:

```markdown
## Architectural Reasoning
- (decisions made during implementation that affect ≥ 2 modules)
- (link to ADR if a durable decision was made)

## Alternatives Considered
- (alternatives evaluated and why rejected — feeds future ADRs)

## Security & Data Integrity Notes
- (auth boundaries crossed, data validation added, rate limits, audit logs)

## Performance Considerations
- (queries added/changed, index usage, expected QPS impact)

## Error & Edge Paths Tested
- (list of error/edge paths exercised in smoke:e2e — high-risk MUST cover ≥ 1)
```

**Pass condition (gate 2.4 + gate 3.5):**
- All Tier 2 conditions
- All 4 additional sections present and non-empty
- "Error & Edge Paths Tested" lists ≥ 1 path

## Trace Quality Scoring (used by Retro Gate ⑥)

Retro analysis classifies each story's trace into a quality level:

| Score | Criteria                                                                                              |
| ----- | ----------------------------------------------------------------------------------------------------- |
| **1** | Tier required but missing sections; vague reasoning; deferred findings without justification         |
| **2** | All required sections present; concrete reasoning; deferred findings have rationale + link            |
| **3** | All required sections; alternatives discussed; explicit security/performance notes; rich friction log |

**Lane threshold (for merge):**

| Lane      | Minimum trace quality score |
| --------- | --------------------------- |
| tiny      | 1                           |
| normal    | 2                           |
| high-risk | 3                           |

Scoring is performed by the reviewer (gate 3.5) or by the retro analysis (gate
6.4). A trace below the lane threshold fails the gate.

## Friction Capture Protocol

Friction = anything that slowed or confused the agent. Examples:
- A rule was ambiguous and required re-reading
- A test failed for unclear reasons
- A tool returned unexpected output
- A file was hard to navigate or named misleadingly

**Capture rule:**
- If fix is < 30 min AND in current PR scope → fix as harness delta in this PR
- Else → add to `docs/HARNESS_BACKLOG.md` with: title, discovered-while,
  current pain, suggested improvement, status: `proposed`

**The harness grows from friction.** A retro with no friction captured for a
high-risk story is suspicious — escalate to human review.
