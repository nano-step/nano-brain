---
name: deep-design
description: "MANDATORY for any non-trivial feature planning. Multi-agent pipeline: Metis (scope/risk) + Oracle (architecture) in parallel → cross-critique → confidence-scored synthesis → early Momus sanity check → OpenSpec proposal → full Momus review. MUST USE when: user describes a feature that touches multiple services or modules, user wants to plan/design/architect before coding, user says 'plan this', 'design this', 'think this through', 'spec this out', 'deep design', 'let me get this right', 'before we start coding', 'rethink how X works', 'migrate from X to Y', 'add multi-tenancy/billing/auth/permissions/real-time/encryption/caching', user describes any system that needs architectural decisions, user mentions wanting a proposal or spec, user wants to avoid mistakes on a complex feature. Also triggers on: 'plan out the migration', 'design a system for', 'I want to add [complex feature]... need to think through', 'this is a big change', 'let me get the design reviewed', 'before anyone writes code'. When in doubt about whether to use this skill, USE IT — it's better to over-plan than to start coding a complex feature without a reviewed design."
compatibility: "OpenCode"
metadata:
  version: "2.0.0"
---

# Deep Design Pipeline

## Overview

A 7-phase orchestration pipeline that turns a user's feature idea into a reviewed, implementation-ready OpenSpec proposal. Inspired by the LLM Council pattern: agents analyze independently, then cross-critique each other's outputs, conflicts are resolved using confidence scoring (not hard rules), and a sanity-check gate catches design flaws before expensive spec-writing begins.

**Pipeline at a glance:**

```
User describes feature
        │
        ▼
┌───────────────────────────────┐
│  Phase 1: PARALLEL ANALYSIS   │
│                               │
│  ┌─────────┐   ┌──────────┐  │
│  │  Metis  │   │  Oracle  │  │
│  │ (scope, │   │ (arch,   │  │
│  │  risks, │   │  design, │  │
│  │ phases) │   │  MVA)    │  │
│  └────┬────┘   └────┬─────┘  │
│       └──────┬───────┘        │
└──────────────┼────────────────┘
               ▼
┌──────────────────────────────────┐
│  Phase 1.5: CROSS-CRITIQUE       │
│                                  │
│  Metis reads Oracle's output →   │
│  critiques architecture          │
│  Oracle reads Metis's output →   │
│  rebuts scope concerns           │
│  (parallel, anonymized roles)    │
└──────────────┬───────────────────┘
               ▼
┌──────────────────────────────┐
│  Phase 2: SYNTHESIS          │
│  Confidence-scored conflict  │
│  resolution, design brief    │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  Phase 2.5: MOMUS SANITY     │
│  Lightweight early review    │
│  of design brief — catch     │
│  fatal flaws before spec     │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  Phase 3: OPENSPEC PROPOSAL  │
│  proposal.md, design.md,     │
│  specs/, tasks.md            │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  Phase 4: MOMUS REVIEW       │
│  Full expert critique of     │
│  the proposal artifacts      │
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│  Phase 5: DELIVER            │
│  Present proposal + review   │
│  to user for approval        │
└──────────────────────────────┘
```

**Cost note:** This pipeline fires Metis + Oracle twice (analysis + cross-critique) and Momus twice (sanity check + full review) — all claude-opus-4-6. It is designed for non-trivial features where getting the design right is worth the cost. The cross-critique round surfaces blind spots that single-pass analysis misses. The early Momus gate saves the cost of writing invalid OpenSpec artifacts.

---

## Input

The user provides a feature description. This can range from a single sentence ("add real-time collaboration") to a detailed brief. The skill works with whatever level of detail is given — Metis and Oracle will independently identify what's missing.

If the user's description is too vague to act on (e.g., "make it better"), ask ONE clarifying question before starting the pipeline. Otherwise, start immediately.

## Graceful Degradation Policy

If **one** of the two agents in Phase 1 or Phase 1.5 fails to respond:
- **Continue** with the available result — do NOT halt the pipeline.
- **Note the gap clearly** in the Design Brief: "⚠️ [Agent] analysis unavailable — [section] decisions have lower confidence."
- **Flag affected conflict resolutions** as "single-source" with lower confidence scores.
- If **both** agents fail → halt and report the error to the user.

## Phase 1: Parallel Analysis (Metis + Oracle)

Fire both agents simultaneously using `run_in_background=true`. They analyze the same feature description independently — this diversity of perspective is the point.

### Metis Prompt

```
task(
  subagent_type="metis",
  run_in_background=true,
  load_skills=["nano-brain"],
  description="Deep design: Metis scope/risk analysis for [feature]",
  prompt="""
CONTEXT: The user wants to build: [paste user's feature description verbatim].
The codebase is at [project root]. Use `npx nano-brain query` to check for
past decisions or context related to this feature area. If the codebase is not
accessible at the given path, work with the described tech stack — do not stall
on codebase discovery.

ROLE: You are a pre-planning consultant. Your job is to find what the user
hasn't thought of yet — the hidden complexities, the scope risks, the things
that will blow up at 2am if nobody plans for them.

ANALYZE AND RETURN:

1. HIDDEN COMPLEXITIES
   - What non-obvious technical challenges does this feature involve?
   - What edge cases will bite during implementation?
   - What existing systems will this interact with unexpectedly?

2. SCOPE RISKS
   - Where is scope likely to creep?
   - What looks simple but is actually hard?
   - What dependencies exist that could block progress?

3. PHASING ADVICE
   - What's the natural decomposition into phases?
   - What should be built first to de-risk the rest?
   - What can be deferred to a v2 without compromising v1?

4. MISSING REQUIREMENTS
   - What has the user not mentioned that they'll definitely need?
   - What error handling, edge cases, or failure modes are unaddressed?

5. QUESTIONS FOR THE USER
   - What critical decisions need user input before design can proceed?
   - List max 3-5 questions, ranked by impact.

FORMAT: Use clear headers and bullet points. Be specific to THIS feature
in THIS codebase — no generic advice.
"""
)
```

### Oracle Prompt

```
task(
  subagent_type="oracle",
  run_in_background=true,
  load_skills=["nano-brain"],
  description="Deep design: Oracle architecture validation for [feature]",
  prompt="""
CONTEXT: The user wants to build: [paste user's feature description verbatim].
The codebase is at [project root]. Use `npx nano-brain query` and `npx nano-brain context`
to understand existing architecture before making recommendations. If the
codebase is not accessible at the given path, work with the described tech
stack — do not stall on codebase discovery.

ROLE: You are a senior architect validating the design of this feature.
Your job is to ensure the architecture is sound, identify the minimum viable
approach, and answer the hard design questions before code is written.

ANALYZE AND RETURN:

1. ARCHITECTURE VALIDATION
   - Does this feature fit cleanly into the existing architecture?
   - What architectural patterns should it follow (based on what exists)?
   - Where are the integration points with existing systems?
   - Are there architectural anti-patterns to avoid?

2. DESIGN QUESTIONS & ANSWERS
   - What are the key design decisions for this feature?
   - For each decision, recommend an approach with reasoning.
   - Note where multiple valid approaches exist and which you'd pick.

3. MINIMUM VIABLE APPROACH (MVA)
   - What is the smallest version of this feature that delivers value?
   - What can be cut from v1 without losing the core value proposition?
   - What technical shortcuts are acceptable vs. which create tech debt?

4. SECURITY & PERFORMANCE CONSIDERATIONS
   - Any security implications? (auth, data exposure, injection risks)
   - Any performance concerns? (N+1 queries, memory, concurrency)
   - Any data integrity risks?

5. TECHNOLOGY RECOMMENDATIONS
   - Should this use existing libraries/tools or introduce new ones?
   - Are there proven patterns in the codebase to reuse?

FORMAT: Use clear headers and bullet points. Be specific to THIS feature
in THIS codebase. Recommend concrete approaches, not abstract principles.
"""
)
```

**After firing both:** Proceed to Phase 1.5 when results arrive. Apply Graceful Degradation Policy if one fails. End your response and wait for `<system-reminder>` notifications.

## Phase 1.5: Cross-Critique Round

**Why this exists:** Inspired by LLM Council's Stage 2 peer review. Each agent reads the other's output and challenges it — this surfaces blind spots that independent analysis misses. An architect may miss scope explosion; a risk analyst may over-weight concerns the architecture already handles.

Fire both critique tasks simultaneously using `run_in_background=true`, passing the other agent's output as input.

### Metis Critiques Oracle

```
task(
  subagent_type="metis",
  run_in_background=true,
  load_skills=["nano-brain"],
  description="Deep design: Metis cross-critique of architecture proposal for [feature]",
  prompt="""
CONTEXT: We are planning: [paste user's feature description verbatim].

Another analyst produced the following architecture proposal:

--- ARCHITECTURE PROPOSAL ---
[paste Oracle's full Phase 1 output here]
--- END ---

ROLE: You are a scope and risk analyst reviewing this architecture proposal.
Your job is to find risks, hidden complexities, and scope concerns that the
architect may have missed or underweighted.

EVALUATE AND RETURN:

1. RISK BLIND SPOTS
   - What scope risks did the architect underestimate or ignore?
   - What edge cases does the architecture not handle?
   - What can go wrong at 2am that this design doesn't protect against?

2. PHASING CONCERNS
   - Is the proposed MVA truly minimal, or is it still too large?
   - Are the phases ordered correctly to de-risk the most uncertain parts first?
   - What should be cut from v1 that the architect included?

3. AGREEMENTS
   - What parts of the architecture proposal do you agree with? (be specific)
   - Where does it align with your own risk analysis?

4. CONFIDENCE RATINGS
   For each major concern you raise, rate your confidence:
   - HIGH: I'm certain this is a real problem
   - MEDIUM: Likely a problem, worth addressing
   - LOW: Possible concern, flag for user to decide

FORMAT: Use clear headers. Be specific and direct — no generic advice.
Only critique things you genuinely disagree with. Don't manufacture concerns.
"""
)
```

### Oracle Critiques Metis

```
task(
  subagent_type="oracle",
  run_in_background=true,
  load_skills=["nano-brain"],
  description="Deep design: Oracle cross-critique of scope analysis for [feature]",
  prompt="""
CONTEXT: We are planning: [paste user's feature description verbatim].

A risk analyst produced the following scope and risk analysis:

--- SCOPE & RISK ANALYSIS ---
[paste Metis's full Phase 1 output here]
--- END ---

ROLE: You are a senior architect reviewing this scope and risk analysis.
Your job is to validate which concerns are architecturally sound, which are
overstated, and what the analysis missed from an architecture perspective.

EVALUATE AND RETURN:

1. OVERBLOWN CONCERNS
   - Which risks are overstated given the architecture you'd recommend?
   - Which "hidden complexities" are actually well-solved problems with known patterns?
   - Where is the analysis too conservative in a way that would over-engineer the solution?

2. MISSED ARCHITECTURAL RISKS
   - What risks did the scope analysis miss that are real architectural concerns?
   - What integration points or system interactions were not considered?

3. AGREEMENTS
   - Which scope risks are genuinely serious from an architecture standpoint?
   - Where does the analysis align with your own architectural assessment?

4. CONFIDENCE RATINGS
   For each point you raise, rate your confidence:
   - HIGH: Architecturally certain
   - MEDIUM: Likely correct, depends on implementation approach
   - LOW: Possible, needs user input to resolve

FORMAT: Use clear headers. Be specific — reference the analyst's exact claims
when agreeing or disagreeing. Don't invent concerns not related to the original analysis.
"""
)
```

**After both critiques complete:** Proceed to Phase 2. Apply Graceful Degradation Policy if one fails. End your response and wait for `<system-reminder>` notifications.

## Phase 2: Synthesis

This phase is performed by you (the orchestrator), not a subagent. You now have 4 inputs:
- **Metis Phase 1**: Original scope/risk analysis
- **Oracle Phase 1**: Original architecture proposal
- **Metis Phase 1.5**: Critique of Oracle's architecture
- **Oracle Phase 1.5**: Critique of Metis's scope analysis

### How to Synthesize

#### Step 1: Build the Confidence Matrix

For each key decision or claim, score confidence using inputs from all 4 analyses:

| Topic | Metis says | Oracle says | Cross-critique result | Confidence | Resolution |
|-------|-----------|-------------|----------------------|------------|------------|
| [Decision] | [stance] | [stance] | Metis: HIGH concern / Oracle: LOW concern | HIGH/MED/LOW | [approach] |

**Confidence scoring rules:**
- **HIGH confidence** (Settled Decision): Both agents agree in Phase 1, AND cross-critiques don't contradict
- **MEDIUM confidence** (Lean one way): Agents disagree but one cross-critique validates the other's view, OR both agents agree but one cross-critique raised a HIGH-rated concern
- **LOW confidence** (Escalate to user): Agents disagree AND cross-critiques don't resolve it, OR both cross-critiques rated the same concern as HIGH

#### Step 2: Resolve by Confidence Level

- **HIGH confidence** → Add to "Settled Decisions" section, note consensus
- **MEDIUM confidence** → Make a call, note the lean and why, document dissent
- **LOW confidence** → Add to "Open Questions for User" — do NOT decide unilaterally
- **Both cross-critiques raised same concern at HIGH** → Treat as critical risk regardless of original positions

**Domain tiebreakers (only when confidence is equal):**
- Architecture/technology implementation → lean Oracle
- Scope explosion and timeline risks → lean Metis
- Security concerns → take the MORE conservative view
- Never silently pick a side — always document the conflict and your reasoning

#### Step 3: Produce the Design Brief

```markdown
## Design Brief: [Feature Name]

### Settled Decisions (HIGH confidence — both agents agree)
- [Decision]: [Approach] | Basis: [what both agreed on]
- ...

### Architecture Approach
[Oracle's recommended architecture, validated by Metis cross-critique — note any HIGH-confidence concerns Metis raised that Oracle's architecture needs to address]

### Implementation Phases
- **Phase 1 (MVP):** [Oracle's MVA scope, filtered through Metis's de-risk ordering]
- **Phase 2:** [next increment]
- **Phase 3 (if applicable):** [deferred items]

### Conflict Resolution Log
| Topic | Metis | Oracle | Cross-critique result | Confidence | Decision |
|-------|-------|--------|----------------------|------------|----------|
| [topic] | [view] | [view] | [who conceded what] | MED | [resolution + reasoning] |

### Key Risks & Mitigations
- [Risk] (source: Metis, confirmed by Oracle cross-critique) → [Mitigation]
- [Risk] (source: Metis, disputed by Oracle — MEDIUM confidence) → [Mitigation + caveat]

### Open Questions for User (LOW confidence — need your input)
1. [Question] — both agents conflict and cross-critiques didn't resolve it
2. ...

### Security & Performance Notes
[From Oracle's analysis, with any concerns Metis cross-critique raised]
```

**Present the Design Brief to the user.** Ask them to answer open questions and confirm before proceeding to Phase 2.5. Do NOT proceed without user confirmation.

If the user has answers/feedback, incorporate them and update the Conflict Resolution Log.

## Phase 2.5: Momus Sanity Check

**Why this exists:** Catch fatal design flaws *before* writing full OpenSpec artifacts. A lightweight Momus review on the Design Brief costs one agent call but can prevent wasted effort writing specs around a fundamentally broken approach.

This is NOT a full review — it's a fast sanity check on the design brief only. Full artifact review comes in Phase 4.

### Momus Sanity Prompt

```
task(
  subagent_type="momus",
  load_skills=["nano-brain"],
  description="Deep design: Momus sanity check on design brief for [feature]",
  prompt="""
CONTEXT: We are about to write OpenSpec artifacts for a new feature.
Before we invest in writing proposal.md, design.md, specs/, and tasks.md,
we need a fast sanity check on the design brief.

DESIGN BRIEF:
[paste full Design Brief from Phase 2 here]

ROLE: You are a design reviewer doing a FAST sanity check — not a full review.
Focus only on fatal issues that would make the entire approach invalid.
Do NOT nitpick details that belong in the spec review.

SANITY CHECK CRITERIA (fatal issues only):

1. FUNDAMENTAL FLAWS
   - Is the core approach architecturally unsound?
   - Is there a simpler solution that makes the entire brief unnecessary?
   - Does the approach contradict itself in a way that can't be resolved?

2. MISSING CRITICAL DECISIONS
   - Are there decisions marked HIGH confidence that you believe should be LOW?
   - Are there unresolved questions that will block spec writing?
   - Is any section too vague to turn into implementable specs?

3. SCOPE ASSESSMENT
   - Is the MVP scope still too large for a v1?
   - Are there scope items that will definitely explode during implementation?

OUTPUT FORMAT (keep it brief):

## Sanity Check Result: [PASS / PASS WITH NOTES / BLOCK]

### PASS: Proceed to OpenSpec.
### PASS WITH NOTES: Proceed, but address these first:
- [Note 1]
### BLOCK: Do NOT proceed until these are resolved:
- [Fatal issue 1]: [Why it blocks] → [What needs to change]

Keep this review under 300 words. Be ruthless about scope — only flag genuine blockers.
"""
)
```

**After Momus sanity check:**
- **PASS**: Proceed directly to Phase 3.
- **PASS WITH NOTES**: Incorporate notes into the Design Brief, then proceed to Phase 3.
- **BLOCK**: Present the blocking issues to the user, revise the Design Brief, then re-run Phase 2.5 before proceeding. Do NOT write OpenSpec artifacts with a blocked design.

## Phase 3: OpenSpec Proposal

With the user-approved, Momus-cleared Design Brief in hand, create a formal OpenSpec change. Use the existing OpenSpec workflow — do not reinvent it.

### Steps

1. **Create the change:**
   ```bash
   openspec new change "<feature-name-kebab-case>"
   ```

2. **Get artifact status and instructions:**
   ```bash
   openspec status --change "<name>" --json
   ```

3. **Create artifacts in dependency order.** For each artifact:
   ```bash
   openspec instructions <artifact-id> --change "<name>" --json
   ```
   Use the template from instructions, but fill it with content from the Design Brief.

4. **Key artifact mapping from Design Brief:**

   | Design Brief Section | OpenSpec Artifact |
   |---------------------|-------------------|
   | Settled Decisions + Architecture Approach | `design.md` |
   | Implementation Phases | `tasks.md` (with checkboxes) |
   | The "why" + scope | `proposal.md` |
   | Security/Performance + Requirements | `specs/<capability>/spec.md` |
   | Key Risks & Mitigations | `design.md` (Risks section) |

5. **Validate the proposal:**
   ```bash
   openspec validate "<name>" --strict --no-interactive
   ```
   Fix any validation errors before proceeding.

### Writing Guidelines

- **proposal.md**: Focus on WHY this feature exists and WHAT changes. Pull from the user's original description + Metis's scope analysis.
- **design.md**: Capture Oracle's architecture recommendations, the settled decisions, and risk mitigations. This is the technical "how."
- **tasks.md**: Break implementation phases into concrete checkboxed tasks. Use Metis's phasing advice for ordering. Each task should be independently completable.
- **specs/**: Write formal requirements with scenarios (WHEN/THEN format). Cover the happy path, error cases, and edge cases that Metis identified.

After all artifacts are created and validated, proceed to Phase 4.

## Phase 4: Momus Review

Fire Momus to stress-test the proposal. Momus is an expert reviewer — it will find gaps, ambiguities, and missing scenarios that slipped through.

### Momus Prompt

```
task(
  subagent_type="momus",
  load_skills=["nano-brain"],
  description="Deep design: Momus review of [feature] proposal",
  prompt="""
CONTEXT: We've completed a multi-agent planning pipeline for a new feature.
The OpenSpec proposal is at: openspec/changes/<name>/

ARTIFACTS TO REVIEW:
- proposal.md: [paste path]
- design.md: [paste path]
- tasks.md: [paste path]
- specs/: [paste path(s)]

Read ALL artifacts before reviewing.

ROLE: You are an expert reviewer evaluating this proposal for implementation
readiness. Your job is to find what's missing, what's ambiguous, and what
will cause problems during implementation.

REVIEW CRITERIA:

1. COMPLETENESS
   - Are all requirements covered by specs with scenarios?
   - Are error cases and edge cases addressed?
   - Are there implicit assumptions that should be explicit?
   - Is anything mentioned in the proposal but missing from tasks?

2. CLARITY
   - Can a developer pick up tasks.md and start working without asking questions?
   - Are design decisions in design.md clear and well-reasoned?
   - Are spec scenarios specific enough to be testable?

3. VERIFIABILITY
   - Can each requirement be verified (tested/checked)?
   - Are acceptance criteria clear for each task?
   - Are there measurable success criteria?

4. RISKS & GAPS
   - What failure modes are unaddressed?
   - What happens if a dependency is unavailable?
   - Are there race conditions, data consistency issues, or security gaps?
   - Is the task ordering correct? Are there hidden dependencies between tasks?

5. SCOPE ASSESSMENT
   - Is the scope appropriate for the stated goals?
   - Is anything over-engineered for v1?
   - Is anything under-specified that will cause scope creep?

OUTPUT FORMAT:

## Review Summary
[1-2 sentence overall assessment: ready / needs work / major concerns]

## Critical Issues (must fix before implementation)
- [Issue]: [Why it matters] → [Suggested fix]

## Recommendations (should fix)
- [Issue]: [Why it matters] → [Suggested fix]

## Minor Notes (nice to have)
- [Observation]

## Verdict
[APPROVED / APPROVED WITH CONDITIONS / NEEDS REVISION]
[If needs revision, list the specific items that must be addressed]
"""
)
```

**After Momus completes:**

- If **APPROVED**: Proceed to Phase 5.
- If **APPROVED WITH CONDITIONS**: Apply the fixes to the OpenSpec artifacts, then proceed to Phase 5. No need to re-run Momus unless the fixes are substantial.
- If **NEEDS REVISION**: Apply the critical fixes, re-validate with `openspec validate`, and present the updated proposal to the user. Ask if they want to re-run Momus or proceed.

## Phase 5: Deliver to User

Present the final result to the user. This is the handoff — they decide what happens next.

### What to Present

```markdown
## Deep Design Complete: [Feature Name]

### Pipeline Summary
- ✅ Metis: Identified [N] hidden complexities, [N] scope risks
- ✅ Oracle: Validated architecture, recommended [approach]
- ✅ Cross-critique: Metis challenged [N] arch decisions / Oracle validated [N] scope concerns
- ✅ Synthesis: [N] HIGH-confidence settled decisions, [N] conflicts resolved, [N] questions escalated to user
- ✅ Momus sanity: [PASS / PASS WITH NOTES]
- ✅ OpenSpec: Created proposal at `openspec/changes/<name>/`
- ✅ Momus full review: [APPROVED / APPROVED WITH CONDITIONS]

### Proposal Location
`openspec/changes/<name>/`
- `proposal.md` — Why and what
- `design.md` — Architecture and decisions (includes Conflict Resolution Log)
- `tasks.md` — Implementation checklist ([N] tasks across [N] phases)
- `specs/` — Formal requirements with scenarios

### Conflict Resolution Highlights
[Top 2-3 conflicts that were resolved and how — so user can override if needed]

### Momus Review Highlights
[Top 2-3 findings from full Momus review, if any]

### Next Steps
- **To implement:** `/opsx-apply <name>` or ask me to start implementation
- **To modify:** Edit any artifact directly or ask me to adjust
- **To explore further:** `/opsx-explore <name>` to think through specific aspects
```

---

## Guardrails

- **Never skip a phase.** Every feature gets the full pipeline.
- **Never proceed past Phase 2 without user confirmation.** The Design Brief is a checkpoint.
- **Never proceed to Phase 3 with a BLOCK from Momus sanity check.** Fix the design first.
- **Never auto-implement.** This skill produces a reviewed proposal — implementation is a separate decision.
- **Always validate OpenSpec artifacts** before running Momus full review. Don't waste an expensive review on invalid specs.
- **Always use `load_skills=["nano-brain"]`** when firing Metis, Oracle, and Momus so they can access project memory and code intelligence.
- **Always collect all background results** before synthesizing. Never synthesize with partial data.
- **Apply Graceful Degradation Policy** — never halt due to a single agent failure; continue with reduced confidence, document the gap.
- **Never silently resolve LOW-confidence conflicts** — always escalate to user. The Conflict Resolution Log must be transparent so the user can override any decision.
