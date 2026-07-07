# Self-Review — Issue #547 (README rewrite + LICENSE)

Change-type: **docs** · Lane: tiny · Branch: `docs/547-readme-rewrite`
Author: kokorolx. Per harness change-type table, docs requires neither
smoke:e2e nor an independent review gate; this self-review is the record.

## Actions Taken

- Rewrote `README.md` from the ready-to-merge draft in issue #547, corrected
  against **verified current behavior** (not the draft's assumed post-fix state).
- Dropped the "50ms rule" (stated 3×) and the "42ms" figure; the Performance
  section now scopes fast-path wording to code-intelligence lookups only and
  does not claim the hybrid `memory_query` path is sub-50ms.
- Kept `memory_impact(direction:"in")` as the headline capability after
  empirically confirming it returns real reverse callers.
- Corrected the tool count 16 → 18 (`memory_ticket`, `memory_workspaces_list`),
  regrouped the tool table, added an explicit ordered agent workflow.
- Repaired broken links; created a top-level MIT `LICENSE` (badge + License
  section now resolve). Genericized benchmark labels; trimmed 570 → 240 lines.

## Files Changed

- `README.md` — full rewrite (240 lines, was 570).
- `LICENSE` — new (MIT, copyright nano-step, matches package.json `license: MIT`).

## Findings Summary

- **Verification before claiming.** Ran `memory_impact ExpandImpactFrontier
  direction=in` on the live `nano-brain` workspace → 7 real reverse edges
  (`registerMemoryImpact`, `collectImpact` via `calls`; container files via
  `contains`; depth-2 callers). So #544 N1's "returns empty" is resolved for the
  call/containment case → the impact claims are honest. File-level reverse
  *import* edges remain #378's scope (still open); the README does not
  over-claim there.
- **Deliberate omission.** The draft's "Multi-repo & event-driven code" section
  documents `publishes_to`/`subscribes_to`/`produces`/`consumes` edges that are
  a **proposed** feature in #546 (not shipped) and monorepo-scoped resolution
  that is deferred. Including it would reintroduce the false-claim problem this
  change fixes, so it was omitted. Add once #546 lands.

## Resolution Status

- All in-scope items resolved. No critical/major issues.
- `CGO_ENABLED=0 go build ./...` → exit 0 (docs-only; no code touched).
- Link check: all 10 relative README links resolve (0 broken), incl. `LICENSE`.
- Residual stale-string grep (`50ms`/`42ms`/`16 tools`/`publishes_to`) → none.

## Gemini Verification Triage

_Pending — populate after the Gemini bot reviews the PR._

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| _(none yet)_ | | | |
