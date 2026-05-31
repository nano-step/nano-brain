# Self-Review Gate 2.4 — Story 9.4 REST Endpoints

**Reviewer:** Oracle
**Date:** 2026-05-30
**Branch:** story-9.4-rest-endpoints

## Verdict: FAIL

**Blocking reason:** AC1 violated — `ResolveLink` endpoint has zero httptest coverage. AC9 (resolve: ID lookup, title lookup, ambiguous detection) is implemented correctly in code but has no test to verify behavior.

---

## Per-AC Table

| AC | Description | Status | Evidence |
|----|-------------|--------|----------|
| AC1 | All 7 endpoint groups httptest covered | ❌ FAIL | `links_test.go` has Backlinks tests only. Zero tests for `ResolveLink`. |
| AC2 | CSRF 7-step matrix | ✅ PASS | `csrf_test.go`: 13 cases covering all 7 rules + loopback + port mismatch + PUT/DELETE. |
| AC3 | Security headers | ✅ PASS | `security_headers_test.go`: all 4 headers asserted. Not mounted in `routes.go` (deferred to 9.5a). |
| AC4 | Config patch allowlist | ✅ PASS | `config_test.go`: `TestPatchConfig_SecretRejected` (400), `TestPatchConfig_ValidPatchPersists` (200+reload), `TestPatchConfig_UnpatchableField` (422). |
| AC5 | Doctor JSON matches CLI | ✅ PASS | Both `cmd/nano-brain/doctor.go:27` and `handlers/doctor.go:22` call `doctor.RunAll()`. |
| AC6 | Stats endpoint | ✅ PASS | `stats_test.go`: mock querier with populated data, verifies `collections` and `recent_docs` shape. |
| AC7 | Graph neighborhood dual-mode | ⚠️ PARTIAL | Symbol default ✅, invalid node_kind 422 ✅, invalid depth 422 ✅, unknown focus empty ✅. BUT: no test for `node_kind=doc` enrichment path (mock returns nil for `ListDocumentsByIDs`). No test verifying `edge_types` forced to `["references"]` in doc mode. |
| AC8 | Backlinks workspace-scoped | ✅ PASS | `stats.sql:49-50`: `WHERE ge.workspace_hash = $1`. Cross-workspace isolation enforced in SQL. |
| AC9 | Resolve endpoint | ⚠️ PARTIAL | Handler code correct: UUID → `ResolveID`, else → `ResolveTitle` (multiple = ambiguous). No test. |
| AC10 | `go build` + `go test -race` pass | ✅ PASS | Build: zero errors. Tests: all packages pass (cached, race detector enabled). |
| AC11 | No regression | ✅ PASS | All project tests pass. CLI doctor refactor uses same `doctor.RunAll()`. |

---

## Additional Skeptical Checks

| # | Check | Result | Detail |
|---|-------|--------|--------|
| S-a | CSRF mount location | ✅ OK | CSRF on `write` subgroup (POST /write, /embed, /reindex, /update, /summarize, /graph/neighborhood). NOT on `/health`, `/api/status`, `/sse`, `/mcp`, GET endpoints. Note: pre-existing collection POST/PUT/DELETE are on `data` not `write` — not CSRF-protected, but pre-existing, not introduced by this PR. |
| S-2 | YAML writeback preservation | ✅ OK | `yaml.v3` Node manipulation preserves comments + key order. Minor: replaced value nodes may lose same-line trailing comments. Acceptable limitation. |
| S-3 | Secret allowlist completeness | ✅ OK | All secrets covered: `database.url`, `embedding.voyage_api_key`, `summarization.api_key`. No other secret-like fields in `Config` struct. |
| S-4 | BFS 500-node cap | ✅ OK | `graph_neighborhood.go:96`: `if len(visited) >= maxNeighborhoodNodes` checked per-node before expansion. Mid-traversal enforcement confirmed. |
| S-5 | ListDocumentsByIDs query | ✅ OK | `stats.sql:40-43`: `SELECT id, title, collection, updated_at, tags FROM documents WHERE workspace_hash = $1 AND id = ANY($2::uuid[])`. |
| S-6 | Snippet extraction | ✅ OK | `extractSnippet` in `links.go:106-145`: case-insensitive search for `[[target]]` or plain target, ±100 char radius. Falls back to first 200 chars if target not found. |
| S-7 | CLI doctor no regression | ✅ OK | `cmd/nano-brain/doctor.go` uses `doctor.RunAll()`, same human-readable output with `padRight` formatting, same JSON format. |
| S-8 | Doctor checks completeness | ✅ OK | 5 checks preserved: Config, PostgreSQL, pgvector, Embedding provider, Embedding model. Migrations was never a doctor check (separate `db:migrate` command). |
| S-9 | Config reload race | ℹ️ NOTE | `reload()` called synchronously after file write. Concurrent readers see stale config briefly. Go pointer swap is atomic — no data corruption. Acceptable eventual consistency. |
| S-10 | CSRF constructor boundAddr | ✅ OK | `routes.go:55`: `boundAddr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)`. Uses configured host:port, not hardcoded. |
| S-11 | Route mounting order | ✅ OK | New routes appended to existing groups. No displacement of existing routes. No middleware changes to pre-existing endpoints. |
| S-12 | Test pollution | ✅ OK | Tests use `httptest` (fresh per test), mock queriers, `t.TempDir()`. No global state mutation. |

---

## Findings

### Critical (blocks merge)

1. **ResolveLink has zero test coverage.** AC1 requires "All 7 endpoint groups httptest covered." AC9 requires verification of ID lookup, title lookup, and ambiguous detection. The handler code is correct but untested. **Fix:** Add `TestResolveLink_ByID`, `TestResolveLink_ByTitle`, `TestResolveLink_Ambiguous`, `TestResolveLink_NotFound` in `links_test.go` with a mock `links.Resolver` interface or test double.

### Medium

1. **Doc-mode graph neighborhood untested.** `node_kind=doc` enrichment path and `edge_types` forced to `["references"]` have no test coverage. The mock `ListDocumentsByIDs` returns nil. **Fix:** Add `TestGraphNeighborhood_DocModeEnrichment` with mock returning doc rows, assert title/collection/updated_at populated. Add `TestGraphNeighborhood_DocModeEdgeFilter` asserting only `references` edges returned.

### Minor

1. **`internal/health/doctor/` has no test files.** Tests exist in `cmd/nano-brain/doctor_test.go` and `handlers/doctor_test.go` but the package itself has `[no test files]`. Individual check functions (`CheckConfig`, `CheckPostgreSQL`, etc.) could benefit from unit tests but this is low priority since they're integration-dependent.
2. **Pre-existing: collection mutation endpoints not CSRF-protected.** `POST /api/v1/collections`, `PUT /api/v1/collections/:name`, `DELETE /api/v1/collections/:name` are on the `data` group, not the CSRF-protected `write` group. This predates Story 9.4 and is not a regression, but should be addressed as a follow-up.

---

## Build + Test Output

```
$ go build ./...
(no output — success)

$ go test -race -short ./...
ok   github.com/nano-brain/nano-brain/cmd/nano-brain         (cached)
ok   github.com/nano-brain/nano-brain/internal/config         (cached)
ok   github.com/nano-brain/nano-brain/internal/server/handlers (cached)
ok   github.com/nano-brain/nano-brain/internal/server/middleware (cached)
... (all 22 testable packages pass)
```

---

## Required Before Re-Review

1. Add httptest coverage for `ResolveLink` endpoint (critical — blocks AC1 and AC9)
2. Add httptest for doc-mode `GraphNeighborhood` enrichment (medium — strengthens AC7)

Once these are addressed, re-run this gate.

---

## Re-Review After Fix (2026-05-30)

**Reviewer:** Oracle
**Trigger:** Sisyphus-Junior added 6 missing tests to address FAIL verdict.

### Verdict: PASS

Both blocking defects resolved. All original ACs now fully satisfied.

### 6 New Tests Checklist

| # | Test | Status | Evidence |
|---|------|--------|----------|
| 1 | `TestResolveLink_ByID` | ✅ PASS | Passes UUID query → handler parses as UUID → calls `ResolveID` → mock returns `true` → asserts `match="id"` and correct UUID in response. Not tautological: exercises UUID-parse branch, `ResolveID` call path, and response shape. |
| 2 | `TestResolveLink_ByTitle` | ✅ PASS | Passes non-UUID string `"My Document"` → handler takes title branch → calls `ResolveTitle` → mock returns 1 ID → asserts `match="title"` and correct UUID. Not tautological: exercises the else-branch (non-UUID parse), title resolution, and response mapping. |
| 3 | `TestResolveLink_Ambiguous` | ✅ PASS | Mock returns 2 UUIDs for title query → asserts `len(results)==2` and both have `match="title"`. Verifies the handler passes through multiple results (ambiguous detection). |
| 4 | `TestResolveLink_NotFound` | ✅ PASS | Passes valid UUID but mock returns `idExists=false` → asserts empty `results` array (len 0). Verifies the handler's `if exists` guard and empty-result normalization (`nil → []`). |
| 5 | `TestGraphNeighborhood_DocModeEnrichment` | ✅ PASS | Uses `docModeNeighborhoodQuerier` mock with real UUID focus, `node_kind=doc`, mock returns 2 doc rows with title/collection/updated_at/tags → asserts both nodes have all enrichment fields populated. Not tautological: the handler must (a) detect doc mode, (b) parse node IDs as UUIDs, (c) call `ListDocumentsByIDs`, (d) map rows back onto nodes. |
| 6 | `TestGraphNeighborhood_DocModeEdgeFilter` | ✅ PASS | Request body specifies `edge_types:["calls","contains"]` but `node_kind=doc` → handler forces `edgeTypes = ["references"]`. Mock captures `GetOutgoingEdgesParams` → asserts `Column3 == "references"`. Also asserts response edges all have `edge_type=references`. Not tautological: directly verifies the defense-in-depth override at line 82-84 of `graph_neighborhood.go`. |

### Interface Refactor Verified

- `ResolveLink` handler signature changed from `*links.Resolver` to `LinkQueryResolver` interface (lines 147-150 of `links.go`).
- Interface: `ResolveID(ctx, workspace, uuid.UUID) (bool, error)` + `ResolveTitle(ctx, workspace, title string) ([]uuid.UUID, error)`.
- `*links.Resolver` in `internal/links/resolve.go` has both methods (lines 48, 57) — structural satisfaction confirmed.
- `routes.go:91` passes `s.concreteLinkRes` (`*links.Resolver`) to `handlers.ResolveLink()` — compiles without change.

### Build + Test Output

```
$ go build ./...
(exit 0 — clean)

$ go test -race -v ./internal/server/handlers/... -run 'TestResolveLink|TestGraphNeighborhood_DocMode'
=== RUN   TestGraphNeighborhood_DocModeEnrichment
--- PASS: TestGraphNeighborhood_DocModeEnrichment (0.00s)
=== RUN   TestGraphNeighborhood_DocModeEdgeFilter
--- PASS: TestGraphNeighborhood_DocModeEdgeFilter (0.00s)
=== RUN   TestResolveLink_ByID
--- PASS: TestResolveLink_ByID (0.00s)
=== RUN   TestResolveLink_ByTitle
--- PASS: TestResolveLink_ByTitle (0.00s)
=== RUN   TestResolveLink_Ambiguous
--- PASS: TestResolveLink_Ambiguous (0.00s)
=== RUN   TestResolveLink_NotFound
--- PASS: TestResolveLink_NotFound (0.00s)
PASS

$ go test -race -short ./...
All 22 testable packages pass (cached).
```

### No New Issues Found

No regressions, no new findings beyond minor items already documented in original review.

VERIFIED
