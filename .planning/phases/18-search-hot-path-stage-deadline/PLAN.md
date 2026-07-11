# Phase 18 — Plan (agent-supplied query expansion)

Issue #588 · high-risk (search-quality gate) · user-feature+bug-fix. Branch: `fix/588-search-stage-deadline` (already checked out).

## Task 1 — plumb `hypothetical` through HybridSearch
- `internal/search/service.go`: add trailing param `hypothetical string` to `func (s *SearchService) HybridSearch(...)` (:169).
- In the vector leg (~:360-372): replace the HyDE block with:
  `embedQuery := query; if hypothetical != "" { embedQuery = hypothetical } else if s.hydeGenerator != nil && hydeEnabled { … existing Generate … }`.
  BM25 leg untouched (still uses `query`).
- Update all callers to pass `hypothetical` (`""` where none):
  - prod: `internal/mcp/tools.go:430`, `internal/server/handlers/query.go:82`, `internal/bench/run.go:25`
  - tests: `internal/search/isolation_test.go` (×5), `chunk_type_vector_571_test.go:58`, `service_test.go:181`
- Verify: `CGO_ENABLED=0 go build ./...`.

## Task 2 — expose `hypothetical` on memory_query MCP tool
- `internal/mcp/tools.go`: add to `registerMemoryQuery` InputSchema (:329) a `"hypothetical"` string prop; parse via `argString(args, "hypothetical")`; pass to HybridSearch at :430. Not in required list.
- Update the tool Description (:328) with one sentence: agents MAY pass `hypothetical` (a short ideal-answer paragraph they generate with their own model) to improve vector recall on conceptual queries; omit for exact/keyword lookups.

## Task 3 — REST parity (self-review:response-shape)
- `internal/server/handlers/query.go`: add optional `Hypothetical string \`json:"hypothetical"\`` to the request struct; pass to HybridSearch at :82. Keep response shape unchanged.

## Task 4 — fix committed footgun (D5)
- `config.test.yml`: HyDE `max_latency_ms` 120000 → 3000.

## Task 5 — tests (AC-1..AC-3)
- `internal/search/service_test.go`: fake embedder recording embedded text + fake HyDE generator with call counter.
  - Case A: `hypothetical="X"` → embedder saw "X", HyDE counter==0, BM25 saw raw query.
  - Case B: `hypothetical=""`, hyde enabled → HyDE counter==1 (no regression).
- `go test -race -short ./internal/search/... ./internal/mcp/...`.

## Task 6 — smoke:e2e (gate 3.12)
- `nanobrain_test`/:3199 only, PID-scoped kill, NEVER dev DB/:3100. Point `search.hyde.provider_url` at an unreachable host + `hyde.enabled: true` to PROVE the provider is never contacted when `hypothetical` is supplied. Seed docs, `curl -i` `/api/v1/query?workspace=<hash>` with `{"query":"...","hypothetical":"..."}`, assert HTTP 200 + non-empty results + fast (no provider hang). Capture `curl`+`HTTP/` lines into `docs/evidence/smoke-e2e-search-hot-path-stage-deadline.md`.

## Task 7 — roadmap note for sampling
- Add a one-line future item to `.planning/ROADMAP.md`: "MCP sampling for provider-free HyDE (needs non-stateless transport + client support; SDK TODO #148)."

## Gates
validate:quick → integration → smoke:e2e → independent reviewer (R88, NOT executor) → ship PR to master (author kokorolx, no AI footer, Closes #588).
