# Harness Philosophy

## The two surfaces

> The app is what users touch. The harness is what agents touch.

A project's product code lives in one place. The harness — docs, templates,
validation commands, review rules — lives next to it and is the only thing
AI agents directly modify before they touch the product.

This separation lets the harness evolve independently of the product. When an
agent gets confused, the fix is to update the harness, not to remember the
lesson in a chat thread that vanishes at session end.

## Why classify by lane

Not every change deserves the same scrutiny. A typo fix and a payments
migration require different ceremony.

The harness uses three lanes:

- **tiny**: low blast radius, patch directly. No proposal, no review gate.
- **normal**: bounded scope, follow proposal → implement → review.
- **high-risk**: touches a hard gate (auth, data, contract, external system).
  Full ceremony: design doc, deep-design gap analysis, multi-agent review,
  human confirmation before lock.

Lanes are determined by the harness via a risk checklist — not by the human
asking for the work. Humans don't always know which change is dangerous.

## Why classify by change type

A migration cleanup and a new login flow are both "normal" lane but have
fundamentally different test profiles. The harness adds a second axis:

| Change type | E2E gate? | Review gate? |
|-------------|:-:|:-:|
| user-feature | ✅ | ✅ |
| bug-fix | ✅ | ✅ |
| infrastructure | ❌ (smoke test) | ⚠️ self-verify |
| refactor | ❌ (existing tests) | ⚠️ self-verify |
| docs | ❌ | ❌ |
| dependency-bump | ❌ (smoke test) | ⚠️ self-verify |

Together: **lane × change type = the actual gate**. A high-risk migration
still skips Playwright E2E because there's no user surface to test. A
normal-lane new endpoint still requires a user-flow test because users
will hit it.

## Why proposal-before-code

The cheapest bug to fix is the one you catch before writing code.

The harness routes every non-trivial change through a proposal artifact
(OpenSpec or plain markdown). The proposal forces explicit answers to:

- What problem are we solving? (and what's NOT in scope?)
- What's the design? (data model, API shape, dependencies)
- What are the acceptance criteria? (testable through user's entry point)
- What are the implementation tasks? (each completable in <2 hours)

This is uncomfortable for fast iteration. It is essential for changes that
cross module boundaries, modify data shapes, or touch security.

## Why deep-design gap analysis

Most bugs are surfaced by an architecture that hides them.

Deep-design is a parallel multi-agent review of the proposal + design BEFORE
specs are written. Two distinct agents (Metis = scope/risk, Oracle =
architecture) examine the documents independently, then cross-critique each
other's findings. The output is a confidence-scored list of gaps.

Blocking gaps (auth, data model, API contract, isolation boundary,
multi-domain scope) prevent progress until resolved. Stylistic gaps are
non-blocking.

This is the harness saying "we have catch-the-mistake budget; spend it
on the design, not on production."

## Why a review gate before archive

Self-review is a known anti-pattern. The implementing agent has anchored
to its solution and cannot see what's missing.

The harness mandates a **fresh** review agent for every normal/high-risk
change before archive. The reviewer reads only:

- The git diff
- The proposal and spec
- The evidence section of the story

The reviewer's job is to map each acceptance criterion to a piece of evidence
and produce a verdict (PASS or FAIL with specific unmet criteria).

PASS unlocks archive. FAIL sends the change back to fix-and-retry.

## Why a PR bot loop

The review gate runs locally; the bot review runs in CI on the actual PR.
They catch different things: the local review verifies criteria coverage,
the bot review catches stylistic and library-best-practice issues the
implementing agent missed.

The harness requires the PR bot loop to converge before merge:

- Substantive comments → fix → re-validate → re-test → push
- Stylistic comments → fix inline or reply with reasoning
- Loop capped at 3 push cycles → escalate to human

This is not optional. It is the final correctness gate before trunk.

## Why issue tracking at every milestone

Work that exists only in a chat session is invisible work. Six months later,
nobody can reconstruct *why* a particular line was written.

The harness creates a tracking issue at intent time (before classification)
and updates it at every milestone: intake, proposal, deep-design, specs,
implementation, user-flow test, review, PR, archive.

The result: an external reader looking only at the issue can reconstruct
the meaningful decisions and proofs that led to the merged change.

## Why a backlog

Friction is signal. When an agent says "I had to look this up three times"
or "the rule wasn't clear", that's a gap in the harness, not a personal
failing.

The harness has a `HARNESS_BACKLOG.md` file dedicated to capturing those
gaps. Items don't have to be fixed immediately — they just have to be
captured so the next agent doesn't hit the same friction.

This is the harness saying "we get better by getting honest about where
we fail."

## Why evidence

Claims without evidence are not claims.

Every passing gate (validate, user-flow test, review, PR bot) produces an
artifact: command output with exit code, screenshot of test run,
verdict table with per-criterion evidence. These are pasted into the
story's Evidence section.

If a story is marked done but Evidence is empty, the harness considers it
not done. This rule prevents the most common AI-assisted-development
failure mode: false completion claims.

## What this is not

- **Not a substitute for thinking.** The harness routes work; it doesn't
  make the design for you.
- **Not a substitute for talking to users.** The proposal still requires
  someone to know what the user wants.
- **Not a guarantee.** A bad proposal will produce a bad change even with
  every gate passing. The harness catches process errors; it doesn't catch
  judgment errors.

The point of the harness is to make the *process errors* expensive to make
and the *judgment errors* visible early enough to fix.
