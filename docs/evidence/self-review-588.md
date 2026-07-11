# Self-review — Issue #588 (agent-supplied query expansion / provider-free HyDE)

Branch: `fix/588-search-stage-deadline`. Change-type: user-feature + bug-fix. Lane: high-risk (search-quality gate).

Independent review performed by the orchestrator (Opus) — NOT the implementing agent (a separate Sonnet executor wrote the code), so this satisfies R88 (no self-approval in the authoring context).

## Verdict: PASS

## What was checked (against the diff `origin/master..HEAD`)

- **Correctness:** vector-leg logic is `embedQuery := query; if hypothetical != "" { embedQuery = hypothetical } else if hydeGenerator != nil && hydeEnabled { …Generate… }`. When `hypothetical` is supplied, `s.hydeGenerator.Generate` is never called and the supplied text is embedded. When absent, behavior is byte-identical to master. Verified the embed happens once (`embedQueryCached(gctx, embedQuery)`) before the workspace=="all"/normal branch split, so the override applies to both.
- **BM25 unaffected:** the BM25 leg still uses the raw `query` in every branch (with/without tags, all/normal). Confirmed `hypothetical` only touches the vector leg.
- **Interface extraction:** `HyDEGenerator` interface defined consumer-side (matches repo convention); `*hyde.Generator` satisfies it unchanged; removed the now-unused `hyde` import. Field + `SetHydeGenerator` retyped to the interface.
- **Contract parity:** MCP `memory_query` schema + description, REST `QueryRequest.Hypothetical`, and both `openapi.json` files updated consistently. Response shape unchanged. All `HybridSearch` callers updated (prod: mcp/tools.go, server/handlers/query.go, bench/run.go; tests pass "").
- **Tests:** unit tests in service_test.go assert (A) embedder receives the hypothetical + HyDE not called, and (B) no regression when absent. Real assertions, not trivial.

## Evidence

- `gofmt -l` on all changed Go files: clean.
- `go vet ./internal/search/... ./internal/mcp/... ./internal/server/handlers/...`: clean.
- `CGO_ENABLED=0 go build ./...`: OK.
- `go test -race -short ./internal/search/... ./internal/mcp/... ./internal/config/...`: PASS.
- `go test -race -tags=integration ./internal/search/... ./internal/mcp/...`: only failure is the PRE-EXISTING `TestMemoryWakeUp_OnlyReturnsMemoryAndSessionSummaryDocs` (#580, red on master before this branch — unrelated to this change).
- smoke:e2e: `docs/evidence/smoke-e2e-588-search-stage-deadline.md` — TC-A (hypothetical supplied) 200/non-empty/`query_ms:7`/HyDE provider never contacted; TC-B (no hypothetical) 200/`query_ms:3184`/HyDE provider contacted then degrades. Proves D1/D2/D3.

## Accepted low-severity notes (not blockers)

1. `mode=debugging` (DebugSearch) ignores `hypothetical` — by design; DebugSearch is a separate parallel-leg flow. Documented here rather than expanding scope.
2. Task 4 (lower `config.test.yml` HyDE `max_latency_ms`) was a no-op: `config.test.yml` has no `hyde` block. The 120000 footgun lives only in the uncommitted dev `~/.nano-brain/config.yml`; operators should lower it (noted in PR).
