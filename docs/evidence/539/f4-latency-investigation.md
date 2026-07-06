# Issue #539 Finding 4: memory_query latency investigation

## Symptom

Hybrid `memory_query` (BM25 + vector + RRF + recency) takes 18-58s per call on
an idle server with an empty embed queue, while BM25-only `memory_search` is
~82ms.

## Call path

`registerMemoryQuery` (`internal/mcp/tools.go:303`) calls
`a.searchService.HybridSearch` (`internal/search/service.go:131`), which runs
the BM25 leg and the vector leg concurrently via `errgroup`. The vector leg
(`internal/search/service.go:318-340`) embeds the query text before issuing
`VectorSearch*`:

```go
vec, err := s.embedder.Embed(gctx, embedQuery)
```

`DebugSearch` (`internal/search/service.go:825-829`, used when `mode=debugging`)
has the identical pattern.

## Findings

1. **No query-embedding cache exists.** Confirmed by reading
   `internal/search/service.go` end to end and `internal/embed/*.go` — there
   is no cache type anywhere in the embed package, and every `HybridSearch` /
   `DebugSearch` call unconditionally calls `s.embedder.Embed(gctx, ...)`.
   A repeated identical query (common with agents re-running the same
   `memory_query`, pagination re-fetches with `cursor`, or `mode=debugging`
   fanning the same query out three ways) pays the full embedding-provider
   round-trip every time. This is the gap this task closes.

2. **The embed provider interface** is `embed.Embedder`
   (`internal/embed/embed.go`) — `Embed(ctx, text) ([]float32, error)` +
   `Dimension() int`. `search.Embedder` (`internal/search/service.go:66-69`)
   is a structurally identical, separately-declared interface (avoids an
   `internal/search` -> `internal/embed` compile dependency for the interface
   itself, though `internal/embed` types can still be used as the concrete
   value).

3. **HTTP client is already reused, not created per-request.** Both
   `OllamaEmbedder` (`internal/embed/ollama.go:22-34`) and `VoyageAIEmbedder`
   (`internal/embed/voyageai.go:26-45`) build a single `*http.Client{Timeout:
   60*time.Second}` once in their constructor and store it on the struct;
   `Embed()` reuses `o.httpClient` / `v.httpClient` on every call. No fix
   needed here — item 3 in the task ("if the client is created per-request or
   lacks a timeout, fix that") does not apply.

4. **The same `embedder` instance is shared** between the document-embedding
   queue (`embed.Queue`, constructed in `cmd/nano-brain/main.go:499` and used
   in `internal/embed/queue.go:323`) and the query-time `SearchService`
   (`internal/server/server.go:87`). A cache wrapped around the shared
   `Embedder` at that shared construction point would incorrectly cache
   chunk/document embeddings too. To respect "cache query embeddings only,
   never document/chunk embeddings," the cache is instantiated inside
   `SearchService` itself and applied only at the two query-embed call sites
   (`HybridSearch` and `DebugSearch`). `embed.Queue.processChunk` calls
   `q.embedder.Embed` directly and is entirely unaffected.

5. **HyDE and reranking are separate, already-timeout-bound LLM round-trips**
   (`internal/search/hyde/generator.go`, `internal/search/reranking/`), both
   disabled by default (no default config entries in
   `internal/config/defaults.go`). They are a plausible contributor to
   latency when enabled but are out of scope for this fix (task explicitly
   scopes to the embed cache + HTTP client).

## Fix

Added `embed.QueryCache`, a bounded (512-entry, LRU) thread-safe in-process
cache keyed by query text, instantiated per `SearchService` and consulted
only at the query-embedding call sites in `HybridSearch`/`DebugSearch`. See
`internal/embed/cache.go` and `internal/search/service.go`.

Note on keying: the original task spec suggested keying by
`provider-name + model + query-text`. `SearchService` binds to a single
`Embedder` instance for its entire lifetime (confirmed: no `SetEmbedder`/
hot-swap path exists), so one `SearchService` == one fixed provider+model
pair already. Keying on query text alone is behaviorally equivalent here
without needing to plumb `EmbeddingConfig` (provider/model strings) through
`NewSearchService`'s signature, which would require touching
`internal/server/server.go` and `cmd/nano-brain/bench.go` — outside this
task's file surface.
