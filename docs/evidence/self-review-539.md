# Self-Review — Issue #539 (agent-ergonomics)

Date: 2026-07-06
Branch: feat/539-agent-ergonomics
Story: 539 (agent field report — 4 findings)
Lane: high-risk · Change type: user-feature / bug-fix

## Actions Taken

- Confirmed all four root causes directly in code (bare `calls`-edge targets in every
  extractor; `#<uuid>` → document-only lookup leaking `sql:`; symbols persisted as
  signature-only with no end line; no query-embed cache).
- F1: added `ResolveSymbolByName` query; `memory_trace` now resolves bare targets to
  qualified `file::symbol` nodes at query time, filters externals (default), flags
  ambiguity, and traverses via the `::` path (kills name-suffix phantom cycles). Added
  `include_external` param.
- F2: `resolveDocumentByAnyID` helper (document→chunk→parent-document fallback), clean
  non-SQL errors on all memory_get miss paths; also accept a `file::Symbol` graph node.
- F3: `EndLine` added to `symbol.Symbol` + all 5 extractors; watcher persists
  `line`/`end_line`; `memory_symbols` emits them; `memory_get` returns the full body
  span for a symbol path / graph node (falls back to signature when no span/parent).
- F4: bounded thread-safe query-embedding LRU cache in front of the two query-embed
  call sites only; document/chunk embeds untouched.
- Applied all fixes from the independent reviewer (2 MEDIUM + 2 LOW).

## Files Changed

- internal/mcp/tools.go — memory_get (F2 + `file::Symbol` + F3 body/signature fix),
  memory_trace (F1 resolution/filter/ambiguity + panic guard), memory_symbols (F3 lines).
- internal/storage/queries/documents.sql + sqlc/documents.sql.go — `ResolveSymbolByName`.
- internal/symbol/{symbol,go_extractor,javascript_extractor,typescript_extractor,python_extractor,ruby_extractor}.go — `EndLine`.
- internal/watcher/watcher.go — persist `line`/`end_line` symbol metadata.
- internal/search/service.go + internal/embed/cache.go — F4 query-embed cache.
- Tests: internal/mcp/agent_ergonomics_539_integration_test.go (10),
  internal/embed/cache_test.go, internal/search/service_test.go.

## Findings Summary

Independent reviewer (`oh-my-claudecode:code-reviewer`, R88): verdict PASS.
- MEDIUM: F3 symbol path dumped the whole file when line metadata absent — FIXED.
- MEDIUM: F1 negative-slice panic on symbol source_path with no `?` — FIXED (guard).
- LOW: F2 masked non-ErrNoRows DB errors — FIXED (early return).
- LOW: F4 could cache an empty embedding — FIXED (len==0 skip).
- LOW: AC-1 memory_get not re-feedable — CLOSED (added `file::Symbol` support).
No critical/major findings remain.

## Resolution Status

All findings RESOLVED. No unresolved critical/major items.

Test evidence (full detail in `docs/evidence/539/review-539.md`):
- `go test -race -short ./...` → exit 0.
- `go test -race -tags=integration ./internal/mcp/...` (real Postgres, nanobrain_test)
  → 10 #539 tests + existing memory_get tests PASS.
- `CGO_ENABLED=0 go build ./...` + `go vet` clean.

smoke:e2e: live `serve --unsafe-no-auth` boot was blocked by the auto-mode safety
classifier and not worked around; user-flow coverage is provided by the integration
tests driving the real MCP adapter against real Postgres (the agent's actual entry
point). See review-539.md § smoke:e2e note.

## Gemini Verification Triage

Gemini posted 1 review (COMMENTED) with 4 inline findings on PR #540.

| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| PR#540 tools.go:1285 (symbol body slice) | VALID:high | Real regression I introduced: metadata span (e.g. lines 2–4) stayed set when the parent doc failed to load, so the 1-line signature got sliced to `""` for any symbol starting past line 1. | fixed — reset span to 0 on parent-miss when span came from metadata; strengthened `TestMemoryGet_SymbolPathFallsBackToSignatureWhenParentMissing` (line 3–5) |
| PR#540 tools.go:1161 (chunk error masked) | VALID:medium | A real DB error on `GetChunkByID` was masked as "no document or chunk found". | fixed — surface non-`ErrNoRows` chunk errors |
| PR#540 service.go:127 (nil embedder/cache guard) | FALSE_POSITIVE | `embedQueryCached` is only reached after the existing `if s.embedder == nil { return nil }` guards at both call sites (service.go:352, 859); `queryEmbedCache` is set by the sole constructor `NewSearchService` — no struct-literal path exists. Adding an internal nil guard is defensive code for an unreachable state. | acknowledged in reply |
| PR#540 cache.go:80 (zero-value-safe QueryCache) | FALSE_POSITIVE | `QueryCache` is only ever created via `NewQueryCache`; no zero-value/struct-literal construction path exists in the codebase. Lazy-init would guard an impossible state. | acknowledged in reply |

Both VALID findings fixed; both FALSE_POSITIVE findings have documented reasoning. No `VALID:critical`/`VALID:high` left unresolved.
