## 1. Shared helpers (foundation, no behavior change yet)

- [ ] 1.1 Create `internal/search/snippet.go` with `TruncateSnippet(s string, maxChars int) string` — rune-aware truncation; copy current logic from `internal/server/handlers/search.go:49-61`
- [ ] 1.2 Create `internal/search/snippet_test.go` — table-driven tests covering: empty string, short string, exact-boundary, multi-byte unicode (`é`, `世`, emoji), zero/negative maxChars
- [ ] 1.3 Replace inline `truncateSnippet` in `internal/server/handlers/search.go` to call `search.TruncateSnippet` (proves the extract is byte-identical)
- [ ] 1.4 Run `go test -race -short ./internal/search/... ./internal/server/handlers/...` — all green

## 2. Cursor encoding (foundation, no consumer yet)

- [ ] 2.1 Create `internal/search/cursor.go` with `QueryHash(query string) string`, `EncodeCursor(offset int, queryHash string) string`, `DecodeCursor(token string) (offset int, queryHash string, err error)` — design per `design.md` §D2
- [ ] 2.2 Create `internal/search/cursor_test.go` — covers: roundtrip encode/decode, invalid base64, invalid JSON payload, negative offset rejection, query hash determinism, query hash matches across calls with same query
- [ ] 2.3 Run `go test -race -short ./internal/search/...` — all green

## 3. Stable result ordering (foundation, no consumer yet)

- [ ] 3.1 Edit `internal/search/rrf.go` sort comparator — when RRF scores are equal, break by `id ASC` (lexicographic string compare on UUID). Add inline `// for stable cursor pagination` comment.
- [ ] 3.2 Edit `internal/search/recency.go` sort comparator — same tiebreaker.
- [ ] 3.3 Add unit tests `internal/search/rrf_test.go` and `internal/search/recency_test.go` scenarios for tied-score deterministic order (per `specs/search-pipeline/spec.md`)
- [ ] 3.4 Run `go test -race -short ./internal/search/...` — all green; verify no existing tests fail (only equal-score cases should be affected)

## 4. MCP tool: input schema additions (parameter parsing only, no response shape change yet)

- [ ] 4.1 In `internal/mcp/tools.go` registerMemoryQuery — add `include_content` (bool, default false) and `cursor` (string, optional) to the input schema JSON
- [ ] 4.2 In `internal/mcp/tools.go` registerMemorySearch — add `include_content` and `cursor` to input schema
- [ ] 4.3 In `internal/mcp/tools.go` registerMemoryVSearch — add `include_content` and `cursor` to input schema
- [ ] 4.4 Parse new parameters in each handler (decode cursor → offset+queryHash; verify queryHash matches current query → return `cursor query mismatch` error on mismatch; invalid base64 → return `invalid cursor` error)
- [ ] 4.5 Build: `go build ./...` — green

## 5. MCP tool: response shape changes (the actual behavior change)

- [ ] 5.1 Define shared `mcpSearchResponse` type (or per-tool inline) with fields: `results []searchResultItem`, `total int`, `query_ms int`, `next_cursor *string` — see `design.md` for exact shape
- [ ] 5.2 Define `searchResultItem` matching the spec — always includes `snippet`, includes `content` only when `include_content=true` (Go pattern: omitempty on `content` field, set to full content only when flag is true)
- [ ] 5.3 In each of 3 handlers, replace the current raw `search.Result` marshal with the new shape:
  - Snippet = `search.TruncateSnippet(result.Content, 500)` (or use the existing `Snippet` field if already populated and ≤500)
  - Content = `result.Content` only when `include_content=true`, else field is absent
  - `total` = `len(boostedResults)` before pagination slice
  - `query_ms` = elapsed ms from handler entry
- [ ] 5.4 Implement pagination logic: `HybridSearch(..., maxResults = offset + page_size + 1)`, slice `[offset : offset+page_size]`, set `next_cursor` if `len(boostedResults) > offset+page_size`
- [ ] 5.5 Encode `next_cursor` via `search.EncodeCursor(offset+page_size, queryHash)`
- [ ] 5.6 Build: `go build ./...` — green

## 6. Tool description updates

- [ ] 6.1 Update tool description text in `tools.go` for `memory_query`, `memory_search`, `memory_vsearch` to mention: snippet-by-default, `include_content` opt-in, `cursor` for pagination, recommend `memory_get` for full content
- [ ] 6.2 Verify via `curl -X POST .../mcp -d '{"method":"tools/list"}'` that the new schemas + descriptions surface

## 7. Integration tests (response shape contracts)

- [ ] 7.1 Create `internal/mcp/tools_pagination_integration_test.go` with `//go:build integration` tag
- [ ] 7.2 Test: default response excludes `content` field — insert 3 docs, call `memory_search`, assert no result has `content` key in JSON
- [ ] 7.3 Test: `include_content=true` includes `content` field — same setup, assert every result has `content` matching the stored chunk
- [ ] 7.4 Test: snippet length ≤ 500 chars — insert one large doc, assert `snippet` length
- [ ] 7.5 Test: snippet respects UTF-8 boundary — insert chunk with multi-byte char near position 500, assert no half-character in snippet
- [ ] 7.6 Test: payload size budget — insert 10 typical chunks, call with `max_results=10`, assert response JSON size ≤ 20 KB
- [ ] 7.7 Test: pagination roundtrip — insert 12 docs, call with `max_results=5`, get `next_cursor`, call again, get next 5, call again, get final 2 + no `next_cursor`
- [ ] 7.8 Test: cursor query mismatch — query A returns cursor C1, pass C1 with query B, assert error contains "cursor query mismatch"
- [ ] 7.9 Test: invalid cursor — pass `"not-base64!@#"` as cursor, assert error contains "invalid cursor"
- [ ] 7.10 Test: empty result set — query with zero matches, assert `results: []`, `total: 0`, `query_ms ≥ 0`, no `next_cursor`
- [ ] 7.11 Run `go test -race -tags=integration ./internal/mcp/...` — all green

## 8. SKILL.md update (agent-facing contract docs)

- [ ] 8.1 Edit `.opencode/skills/nano-brain/SKILL.md` — explicitly document the `snippet` field on search tool returns
- [ ] 8.2 Document the new `include_content` and `cursor` parameters with examples
- [ ] 8.3 Add migration note: "If you previously parsed `content` from search results, switch to `snippet` (already truncated) or call `memory_get` for one full document"
- [ ] 8.4 Add usage pattern: "For 'find more matches' workflows, paginate by passing `cursor` from the previous response's `next_cursor`"

## 9. CHANGELOG + tool description sanity check

- [ ] 9.1 Add CHANGELOG.md entry under next release: announce MCP-only breaking change, `include_content` opt-in, pagination. Note: HTTP API unchanged.
- [ ] 9.2 Manual sanity check: build binary, start server, run `curl` against `memory_search` with and without `include_content`/`cursor` — paste output as evidence in story

## 10. Validation ladder

- [ ] 10.1 `go build ./...` — exit 0
- [ ] 10.2 `go test -race -short ./...` — all pass (or document pre-existing failures unrelated to this change)
- [ ] 10.3 `go test -race -tags=integration ./internal/mcp/... ./internal/search/...` — all pass
- [ ] 10.4 `openspec validate mcp-search-snippet-pagination --strict --no-interactive` — pass
- [ ] 10.5 Smoke test: start binary on port 4199, `curl` 3 scenarios (default, include_content, paginated), capture output to `docs/evidence/mcp-search-snippet-pagination/smoke.md`

## 11. Review Gate (high-risk lane — full review-work)

- [ ] 11.1 Reviewer ≠ implementer — spawn fresh review-work session
- [ ] 11.2 Review against every scenario in `specs/mcp-server/spec.md` and `specs/search-pipeline/spec.md`
- [ ] 11.3 Paste verdict table in `docs/evidence/mcp-search-snippet-pagination/review-verdict.md`
- [ ] 11.4 Proceed to PR only on PASS

## 12. PR + Bot Review Loop

- [ ] 12.1 Push branch `feat/358-mcp-search-response-pagination`
- [ ] 12.2 Open PR with `Closes #358` in body, link OpenSpec change folder, paste validation output
- [ ] 12.3 Address Gemini bot review comments (max 3 push cycles, escalate to human after)
- [ ] 12.4 Merge → verify issue #358 auto-closes
- [ ] 12.5 `openspec archive mcp-search-snippet-pagination` on master
