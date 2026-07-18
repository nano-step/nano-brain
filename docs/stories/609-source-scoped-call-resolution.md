---
story_id: US-609
title: Source-Scoped JavaScript and TypeScript Call Resolution
status: planned
lane: high-risk
change_type: bug-fix
risk_flags:
  - search-quality
  - public-api-contract
  - existing-behavior
  - weak-proof
  - multi-domain
hard_gates:
  - search-quality
github_issue: nano-step/nano-brain#609
openspec_change: openspec/changes/source-scoped-call-resolution/
validation:
  unit: docs/evidence/source-scoped-call-resolution/validation.md
  integration: pending
  e2e: pending
  platform: pending
  release: deferred
review:
  verdict: pending
  reviewer: ""
  commit: ""
pr:
  url: ""
  bot_rounds: 0
  overridden: false
---

# US-609 Source-Scoped JavaScript and TypeScript Call Resolution

## Status

planned

## GitHub Issue

`nano-step/nano-brain#609`

## Lane

**high-risk** — search-quality is a hard gate; public graph contracts, existing
behavior, weak proof, and multi-domain graph consumers add risk.

## OpenSpec Change

`openspec/changes/source-scoped-call-resolution/`

The change contains the proposal, design, calls-edge specification delta, and
implementation task sequence. Strict OpenSpec validation is recorded in the
durable validation evidence.

## Product Contract

For contextual JavaScript and TypeScript calls, the graph stores exactly one
source-reachable canonical target only when the source file proves it. Calls
that cannot be proved store target-only `<unresolved>`. Canonical targets never
fall back to a bare-name match, and unresolved targets do not create derived
graph fan-out. Legacy bare targets, including Ruby namespaced targets, retain
their established behavior.

## Relevant Product Docs

- `openspec/specs/multi-language-graph-extractors/spec.md`
- `docs/HARNESS.md` — high-risk validation and review gates
- `docs/evidence/deep-design-source-scoped-call-resolution.md`
- `docs/evidence/source-scoped-call-resolution/prework-ownership.md`

## Acceptance Criteria

1. A same-name collision in another project root never becomes the target of a
   contextual JS/TS call.
2. Supported same-file declarations, direct project-local imports, class
   methods, and enumerated typed receivers resolve to matching declarations.
3. Dynamic, ambiguous, shadowed, unsupported, external, and unavailable calls
   emit `<unresolved>` rather than a guessed canonical identity.
4. Canonical targets remain exact through graph SQL, trace, impact, flow, REST,
   and MCP; unresolved targets cannot fan out through derived work.
5. Exporter write/create/rename/removal re-extracts workspace-local contextual
   files, replaces stale importer edges, and preserves unrelated source edges.
6. The isolated test database/server smoke, benchmark comparison, privacy scan,
   strict OpenSpec validation, and independent review provide high-risk proof.

## Design Notes

- Resolver: lexical source facts plus direct project-local import bindings and
  direct-export catalog; no global symbol lookup or TypeScript compiler.
- APIs: existing graph REST/MCP tools retain their public shapes but gain safe
  exact-target behavior through their graph readers.
- Domain rules: canonical JS/TS targets are exact; `<unresolved>` is diagnostic
  raw-edge-only; legacy bare targets remain compatible.
- Lifecycle: admitted JS/TS events cause workspace-local contextual
  re-extraction, including missing or unparsable event paths.
- Authorization: `$omo:start-work issue 609` is the planning/execution
  go-ahead; the plan is the decision record, not a fabricated extra approval.

## Validation

| Layer | Expected proof |
| --- | --- |
| Unit | Focused contextual resolver, extractor, reader, and legacy compatibility tests. |
| Integration | `go test -race -tags=integration ./...` against `nanobrain_test`. |
| E2E | Bounded `:3199` REST/MCP smoke for canonical collision and exporter removal. |
| Platform | `go build ./... && go test -race -short ./...`. |
| Release | Deferred until implementation, review, and PR delivery. |

## Change Type

`bug-fix` — corrects existing graph extraction and public code-intelligence
behavior. The high-risk search-quality gate requires an isolated smoke and
independent review before delivery.

## Testing Checklist

- [ ] User-flow test covers a source-reachable canonical target through graph
  REST and MCP surfaces.
- [ ] Error/edge path covers an unresolved call and an absent canonical target.
- [ ] Exporter rename/removal proves stale importer edges are replaced.
- [ ] E2E applies because this is a high-risk bug fix.
- [ ] Capability benchmark comparison is captured for affected graph operations.
- [ ] All listed tests pass with raw output saved in durable evidence.

## Review

- Reviewer agent: pending independent high-risk reviewer.
- Reviewer ≠ implementer: required.
- Verdict: `PENDING`.
- Date: pending.
- Commit: pending.

| Acceptance Criterion | Evidence | Status |
| --- | --- | --- |
| Canonical collision isolation | Pending focused and smoke evidence | pending |
| Unresolved safety | Pending reader and smoke evidence | pending |
| Lifecycle replacement | Pending watcher integration evidence | pending |
| High-risk validation | Pending benchmark and review evidence | pending |

## PR Bot Review

- PR URL: pending.
- Bot rounds: 0.
- Outstanding comments: none yet.
- Bot approved: pending.

## Harness Delta

None. This story uses the existing high-risk lane process and records the
planning analysis required before implementation.

## Evidence

- `docs/evidence/deep-design-source-scoped-call-resolution.md`
- `docs/evidence/source-scoped-call-resolution/intake.md`
- `docs/evidence/source-scoped-call-resolution/prework-ownership.md`
- `docs/evidence/source-scoped-call-resolution/validation.md`

Implementation evidence remains pending. No task checkbox or external planning
ledger is changed by this story packet.
