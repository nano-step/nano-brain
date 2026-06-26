---
github_issue: nano-step/nano-brain#489
openspec_change: openspec/changes/improve-rails-capability-score
lane: high-risk
change_type: user-feature
risk_flags:
  - search-quality
  - existing-behavior
  - weak-proof
  - public-api-contract
hard_gates:
  - search-quality
branch: feat/489-rails-capability-score
---

# US-489 Improve Rails Capability Benchmark Score

## Status

in-progress — proposal validated, implementation underway.

## GitHub Issue

`nano-step/nano-brain#489`

## Lane

**high-risk** — touches search/code-intelligence quality and observable REST/MCP graph behavior.

## OpenSpec Change

`openspec/changes/improve-rails-capability-score/`

## Product Contract

nano-brain should answer realistic Rails code-intelligence questions without requiring users to know internal file-qualified graph node ids. Rails trace, impact, flow, and symbol tools must handle common inputs such as `BillingWorker#perform`, `Story#create_print_orders`, job/service class names, and Ruby status constants.

## Acceptance Criteria

1. `memory_trace` / HTTP trace can continue traversal from realistic Rails bare nodes when matching graph edges exist.
2. `memory_impact` / HTTP impact can find callers for bare Rails class and `Class#method` targets when matching graph edges exist.
3. Rails flow supports job/service class entries when no HTTP route edge matches.
4. Ruby constant assignments such as `STATUS_ORDER_PAID` are indexed as `const` symbols.
5. Rails capability benchmark improves from the score-only baseline, with `trace` and `impact` both greater than `0.0` or documented blockers explaining missing graph data.
6. Runtime benchmark artifacts and private workspace identifiers are not committed.

## Design Notes

- Commands: OpenSpec apply change `improve-rails-capability-score`.
- APIs: existing REST/MCP graph tools; no new endpoint planned.
- Queries: symbol-aware graph edge lookup may require sqlc regeneration if SQL changes.
- Domain rules: exact graph node ids remain valid; reconciliation is fallback/expansion, not replacement.
- Privacy: committed benchmark files use placeholders only.

## Validation

| Layer | Expected proof |
| --- | --- |
| Unit | Targeted graph/trace/impact/symbol tests for modified packages. |
| Integration | `go test -race -tags=integration ./...` or documented existing blocker. |
| E2E | Build/start server on test DB and exercise changed graph endpoints via HTTP. |
| Platform | `go build ./... && go test -race -short ./...`. |
| Release | Deferred until PR/release flow. |

## Change Type

`user-feature` — improves observable code-intelligence behavior and benchmark instrumentation.

## Testing Checklist

- [ ] User-flow test covers primary changed behavior through HTTP graph endpoints.
- [ ] Error/edge path tested for high-risk lane.
- [ ] Rails capability benchmark score-only evidence captured.
- [ ] All listed tests pass or documented harness override exists for pre-existing blockers.

## Review

- Reviewer agent: independent 5-agent review gate + targeted re-review.
- Reviewer ≠ implementer: yes.
- Verdict: `PASS after fixes`.
- Date: 2026-06-24.
- Commit: pending commit.

| Acceptance Criterion | Evidence | Status |
| --- | --- | --- |
| AC1 trace bare nodes | MCP trace symbol lookup + targeted MCP tests | ✓ |
| AC2 impact bare targets | `TestGraphImpactQueriesMatchSymbolPart` + SQL fallback | ✓ |
| AC3 flow job/service entry | `TestNonHTTPJobEntry`, `TestGraphFlow_NonHTTPJobEntry`, MCP `mcpHasFlowEntry` | ✓ |
| AC4 Ruby constants | `TestRubyExtractor_Constants` | ✓ |
| AC5 benchmark improvement | Agent-oriented Rails score-only run: overall 0.795 | ✓ |
| AC6 privacy | `.gitignore`, fixture sanitization, changed-file privacy grep | ✓ |

## PR Bot Review

- PR URL: TBD.
- Bot rounds: 0.
- Outstanding comments: TBD.
- Bot approved: TBD.

## Harness Delta

Added `.gitignore` protection for Rails capability runtime output files to prevent accidental commits of private benchmark artifacts.

## Evidence

- `docs/evidence/improve-rails-capability-score/baseline-score.md`
- `docs/evidence/improve-rails-capability-score/agent-score.md`
- `docs/evidence/improve-rails-capability-score/review-verdict.md`
