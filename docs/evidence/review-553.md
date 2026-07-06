## Review Verdict: PASS

Reviewer: orchestrator (Opus) — independent of the implementer (a separate Sonnet
executor authored the code; the orchestrator did not write it). The spawned
`code-reviewer` sub-agent (R88) terminated on an account session limit; this cold
review substitutes for it and the Gemini PR bot (gate 3.6) is the identity-bound
external check. Substitution disclosed for honesty.
Date: 2026-07-06
Commit: 1516a24
Issue: nano-step/nano-brain#553 (Phase 2, PR-A)

Change: `memory_impact(direction:"in")` seeds the query frontier with the bare
symbol suffix of qualified `file::symbol` entries so `GetImpactorsByTargets`
matches bare-stored `calls`-edge targets. Query-time only; no schema/migration.

| Acceptance Criterion | Evidence | Status |
|---|---|---|
| AC-A1: impact(file::B, in) returns direct caller A, count exact | `TestMemoryImpact_BareCallsTarget_DirectCallerReturned` PASS (seeds `TargetNode:"B"` bare; asserts count=1, node=`a.go::A`) | ✓ |
| AC-A2: transitive callers resolve at depth (bare seed applied to every depth batch) | `TestMemoryImpact_BareCallsTarget_TransitiveCallersAtDepth` PASS (B@depth1, A@depth2); confirmed in code `frontier = impactFrontierWithBareSuffix(next)` inside the loop | ✓ |
| AC-A3: single-repo exactness + no regression to out / qualified imports-in | `TestMemoryImpact_BareCallsTarget_NoRegressionOutAndQualifiedImports` PASS; `out` branch untouched (guarded by `if direction == "in"`); imports target has no `::` so helper is a no-op for it | ✓ |
| G1: over-inclusion intentional, no ad-hoc per-repo scoping added | code comment on `impactFrontierWithBareSuffix` defers to root-cause C (#542 F2); no scoping logic present | ✓ |
| Termination / no infinite loop | loop bounded by `depth <= maxDepth`; handler `seen` dedups result SourceNodes | ✓ |
| Style: no `_ = err` constructors, surgical diff | pure helper + 2 seed lines; no unrelated changes | ✓ |
| REST parity: `POST /api/v1/graph/impact` (`collectImpact`) had the SAME bug — fixed via shared `symbol.ExpandImpactFrontier` (no duplication) | `TestGraphImpact_BareCallsTarget_{DirectCallerReturned,TransitiveCallersAtDepth}` + `TestGraphImpact_NoRegression_QualifiedImportsTarget` PASS; HTTP smoke returns the caller | ✓ |
| DRY: single implementation shared by MCP + REST | `internal/symbol/impact_frontier.go`; both call sites refactored to it | ✓ |

### Validation evidence (run by orchestrator, not the implementer)

- `CGO_ENABLED=0 go build ./...` → exit 0.
- `go test -race -tags=integration ./internal/mcp/ -run Impact -v` → 4 PASS
  (`TestMemoryImpact_RelativeInputAndOutput`, `..._DirectCallerReturned`,
  `..._TransitiveCallersAtDepth`, `..._NoRegressionOutAndQualifiedImports`), `ok 2.378s`.
- `go test -race -short ./...` (validate:quick) → 31 packages ok, 0 FAIL.
- All against `nanobrain_test` (host.docker.internal:5432) — never dev DB / :3100.

### smoke:e2e — PASS (real HTTP)

Live server on :3199 / nanobrain_test, seeded a bare-target calls edge, hit the
fixed REST endpoint:
`POST /api/v1/graph/impact {"workspace":"smoke553","node":"b.go::B","edge_type":"calls","max_depth":1}`
→ `HTTP/1.1 200` `{"node":"b.go::B","impacted":[{"node":"caller.go::doThing","depth":1,"edge_type":"calls"}]}`.
Full transcript: `docs/evidence/smoke-e2e-impact.md`. The smoke gate is what
surfaced the REST-handler gap (the initial fix was MCP-only) — now closed.

Note: the earlier scope claim "no new HTTP surface" was wrong — `POST
/api/v1/graph/impact` (`internal/server/handlers/impact.go`) is a second surface
with the identical defect; it is now fixed via the shared helper and smoked above.

### Deferred (tracked)

- Multi-repo bare-name over-inclusion on impact-`in` → root-cause C, later phase.
- Minor: a bare name may be re-queried across depth iterations (handler `seen`
  tracks qualified SourceNodes only); results still deduped, bounded by maxDepth
  — non-blocking inefficiency.
