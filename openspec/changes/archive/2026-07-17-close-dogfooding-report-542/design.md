## Context

Issue #542 collects ten observations from one agent-facing dogfooding exercise.
The shipped work was intentionally split into narrow issues and pull requests.
The final tracker state must distinguish a delivered fix from a partial
mitigation, a separately owned follow-up, and a deliberate product boundary.

## Goals / Non-Goals

**Goals:**

- Publish one auditable finding-to-disposition table for #542.
- Transfer F2's remaining extraction-time work to #609 without losing the
  limits of the #575 query-time mitigation.
- Make the F10 by-design decision discoverable at the tracker.

**Non-Goals:**

- Implement or redefine the high-risk call-resolution work in #609.
- Change search ranking, graph extraction, MCP behavior, or persistence.
- Reopen delivered fixes to consolidate their implementation history.

## Decisions

### Use a repository evidence record plus GitHub links

The closure record lives under `docs/evidence/` and links each finding to its
issue and merged pull request. GitHub is the operational tracker, while the
repository record remains reviewable with the OpenSpec change.

### Treat F2 as handed off, not complete

#575 mitigates one trace-time ambiguity case by proximity. It does not prove a
callee's source-level identity and it does not fix extraction, flow, or impact.
The record therefore marks F2 as handed off to #609 rather than falsely calling
it fixed.

### Record F10 as by-design

Embedding similarity cannot encode every product-specific distinction without
an explicit vocabulary and ranking policy. Adding that policy is product work,
not a corrective change to this dogfooding report, so F10 is closed as
won't-fix/by-design.

## Risks / Trade-offs

- [Stale links or an incomplete finding table] → Validate every issue/PR URL
  before review and include all ten F1–F10 rows.
- [Tracker closure hides unfinished graph quality work] → Link #609 from the
  F2 row and final GitHub comment, and state its high-risk scope explicitly.
- [A documentation record looks like a runtime promise] → State that no
  runtime behavior changes in this change.
