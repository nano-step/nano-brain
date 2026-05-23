# Epic 3: Embedding & Vector Search — User Stories

**Depends on:** Epic 2 (documents and chunks stored in PostgreSQL)
**Packages:** `internal/embed`, `internal/search` (vsearch portion)
**Key decisions:** D2 (HNSW pgvector), D6 (errgroup), D7 (buffered channel queue), D9 (consumer-side interfaces)

---

## Overview

This epic delivers semantic embedding of document chunks and vector-similarity search. Chunks produced by Epic 2 get queued for embedding, processed asynchronously by Ollama or VoyageAI, stored as pgvector vectors, and made queryable via `POST /api/vsearch`. Backpressure, failure marking, and observable queue health round out the feature.

Stories must be implemented in sequence: provider interface first, then the async queue, then failure handling and backpressure, then the search endpoint, and finally CLI and status observability.

---

## Story 3.1: Embedding Provider Interface and Implementations

**Description:** Define the `Embedder` interface (`internal/embed`) and implement two concrete providers: Ollama and VoyageAI. Both receive a text string and return a `[]float32` embedding vector. Provider selection and URL are driven by config. Integration tests run against a real Ollama instance on `host.docker.internal`.

**Covers:** FR-29, FR-32

**Applies:** AR-5 (embedder goroutine lifecycle), AR-6 (constructor injection), AR-8 (sqlc for later stories)

**Complexity:** M

**Acceptance Criteria:**

- Given `embedding.provider = "ollama"` and a running Ollama instance at `host.docker.internal:11434`, when `Embedder.Embed(ctx, "hello world")` is called, then a non-empty `[]float32` slice is returned with no error.
- Given `embedding.provider = "voyageai"` and `VOYAGE_API_KEY` is set, when `Embedder.Embed(ctx, "hello world")` is called, then a non-empty `[]float32` slice is returned with no error.
- Given `embedding.provider = "voyageai"` and `VOYAGE_API_KEY` is absent, when the server starts, then startup fails with a descriptive config error (FR-93).
- Given `embedding.concurrency = 3`, when the embedder goroutine is started, then at most 3 parallel embed requests are in flight to the provider at any time.
- Given an unknown provider value in config, when the server starts, then it exits non-zero with a message naming the unknown value.

**Unit tests:** mock `Embedder` interface, verify constructor injection accepts any implementation. **Integration tests** (`//go:build integration`): connect real Ollama via `host.docker.internal`, embed a short string, assert vector length matches the model's dimension.

---

## Story 3.2: Async Embedding Queue

**Description:** Implement the bounded in-memory queue (`chan ChunkID`, capacity 10,000) and the background embedder goroutine managed by `errgroup`. When a chunk is stored in PostgreSQL (by Epic 2), its ID is sent to the queue channel. The goroutine drains the channel, embeds each chunk, and writes the resulting vector to the `embeddings` table. On restart the goroutine scans for un-embedded chunks and re-enqueues them — the queue is ephemeral by design (D7).

**Covers:** FR-30, FR-31, FR-32, FR-36b

**Applies:** AR-5 (errgroup), AR-8 (sqlc), AR-2 (pgvector column)

**Complexity:** L

**Acceptance Criteria:**

- Given a chunk is inserted into PostgreSQL, when the embedding goroutine is running, then an embedding row appears in the `embeddings` table within one embedding cycle without any manual trigger.
- Given the embedding goroutine is started after a restart with 50 un-embedded chunks in PostgreSQL, when it starts up, then it scans and enqueues those chunks without waiting for new writes.
- Given the Ollama provider is unreachable, when the embedder goroutine encounters the error, then it backs off starting at 60 s, multiplying by 1.5 on each consecutive failure, capping at 300 s. On recovery it resets to the base interval (FR-31).
- Given `embedding.concurrency = 3`, when the goroutine is draining the queue, then at most 3 concurrent calls to the provider are made.
- Given the server receives `SIGTERM`, when `errgroup.Wait()` is called, then the embedder goroutine finishes its current batch before exiting — it does not abandon partially embedded chunks.
- Given the queue channel has 10,000 items, when one more chunk is produced, then the send is handled gracefully (see Story 3.3 for the 503 path — this story only ensures the channel size is enforced and the atomic counter reflects the PG backlog).

**Unit tests:** mock provider, verify backoff timing, verify channel capacity is respected. **Integration tests:** insert N chunks, wait for drain, assert N embedding rows exist.

---

## Story 3.3: Queue Backpressure and Failure Handling

**Description:** Add the atomic pending counter, the rejection threshold (50,000), and the `embed_failed` failure-marking logic. When total pending (in-channel + PG backlog) exceeds 50,000, new enqueue requests return HTTP 503 with `Retry-After: 5`. Chunks that fail embedding after 3 attempts are marked `embed_failed` — still available for BM25 but excluded from vector search. Structured log warnings fire at 60% and 90% queue capacity.

**Covers:** FR-36, FR-36b, FR-36c, FR-36e

**Applies:** AR-8 (sqlc for `embed_failed` column), AR-10 (zerolog structured warnings)

**Complexity:** M

**Acceptance Criteria:**

- Given a chunk has failed embedding 3 times, when `GET /api/status` is called, then that chunk's ID is not in the pending count and the `embeddings` table has no row for it; the `chunks` table has `embed_status = 'embed_failed'`.
- Given `embed_status = 'embed_failed'`, when `POST /api/search` (BM25) is called with a query matching that chunk, then the chunk appears in results.
- Given `embed_status = 'embed_failed'`, when `POST /api/vsearch` (vector) is called, then the chunk does not appear.
- Given the total pending count (atomic counter) reaches 50,000, when a new chunk is produced and an enqueue is attempted, then the HTTP handler returns `503 Service Unavailable` with a `Retry-After: 5` header; the chunk remains safely in PostgreSQL.
- Given queue depth reaches 60% of capacity (6,000), then a `warn`-level zerolog entry is emitted with `queue_depth`, `drain_rate`, and `estimated_drain_seconds` fields.
- Given queue depth reaches 90% of capacity (9,000), then an `error`-level zerolog entry is emitted with the same fields.
- Given `POST /api/embed` is called while the rejection threshold is active, then it returns 503 — it does not bypass backpressure.

**Unit tests:** inject failing mock provider, drive to 3 failures, assert `embed_failed` flag set. Test atomic counter increments and decrements correctly. Test log emission at threshold percentages.

---

## Story 3.4: Vector Search Endpoint

**Description:** Implement `POST /api/vsearch` and the `memory_vsearch` MCP tool stub (wired in Epic 5). The handler embeds the query string via the `Embedder` interface, runs a cosine-similarity query against the `embeddings` table (pgvector HNSW index, AR-2), and returns results in the standard FR-10 schema. Returns an empty result set — not an error — when no embeddings exist yet.

**Covers:** FR-9, FR-58

**Applies:** AR-2 (HNSW index), AR-8 (sqlc query), AR-3 (Echo handler)

**Complexity:** M

**Acceptance Criteria:**

- Given chunks are embedded and stored, when `POST /api/vsearch` is called with `{"query": "semantic topic", "workspace": "<hash>"}`, then results are returned ranked by cosine similarity, each with `id`, `title`, `snippet`, `score`, `tags`, `collection`, `workspace_hash`, `created_at`, `updated_at`.
- Given no embeddings exist in the workspace, when `POST /api/vsearch` is called, then the response is `{"results": [], "total": 0, "query_ms": <n>}` with HTTP 200 — not an error.
- Given a request with no `workspace` field, when the handler receives it, then it returns HTTP 400 with `{"error": "workspace_required", ...}`.
- Given workspace A has embeddings and workspace B has none, when `POST /api/vsearch` is called with workspace A's hash, then results include only workspace A's documents (isolation invariant, NFR-2).
- Given the HNSW index has been created on the `embeddings` table, when `POST /api/vsearch` is called with a 100,000-embedding workspace, then results return within 3 seconds (NFR-3 threshold — shared with Epic 4 full pipeline, but vector-only must be within budget).
- Given the request body is not valid JSON, when the handler receives it, then it returns HTTP 415.

**Unit tests:** mock `Embedder`, verify SQL query parameters, verify empty-set path. **Integration tests:** insert 10 chunks with real Ollama embeddings, call vsearch, assert top result is the most semantically similar chunk.

---

## Story 3.5: Immediate Embed Trigger and Status Observability

**Description:** Implement `POST /api/embed` (FR-33/FR-63) for an on-demand embedding pass, `nano-brain embed [--force]` CLI command (FR-34), and the embedding queue fields in `GET /api/status` (FR-35, FR-36d). The status endpoint becomes the primary window into queue health for users and monitoring scripts.

**Covers:** FR-33, FR-34, FR-35, FR-36d, FR-63

**Applies:** AR-3 (Echo), AR-8 (sqlc for counting pending/failed chunks)

**Complexity:** S

**Acceptance Criteria:**

- Given pending un-embedded chunks exist, when `POST /api/embed` is called, then up to 50 chunks are embedded synchronously in the request; the response is `{"embedded": N, "remaining": M}` where N <= 50.
- Given all chunks are already embedded, when `POST /api/embed` is called, then the response is `{"embedded": 0, "remaining": 0}`.
- Given `--force` is passed to `nano-brain embed`, when the CLI command runs, then all chunks (including previously embedded ones) are re-enqueued for embedding.
- Given chunks with `embed_status = 'embed_failed'` exist, when `nano-brain embed --force` is called, then those chunks are also re-enqueued and their `embed_status` reset to `pending`.
- Given the embedding queue is running, when `GET /api/status` is called, then the response includes `queue_depth` (current in-channel count), `queue_capacity` (10,000), `embed_rate_per_sec` (rolling 60-second average), `estimated_drain_seconds` (depth / rate, or 0 when idle), and `queue_status` (one of `nominal` / `busy` / `backpressure` / `rejecting`).
- Given `queue_depth` is 0 and no embeddings are pending in PG, then `queue_status = "nominal"`.
- Given `queue_depth > 0` but below the rejection threshold, then `queue_status` is either `"busy"` (< 60% capacity) or `"backpressure"` (>= 60% capacity).
- Given the rejection threshold is active (total pending >= 50,000), then `queue_status = "rejecting"`.

**Unit tests:** mock queue, verify status field computation. Test `--force` re-enqueue logic against mock storage.

---

## Story 3.6: HNSW Index Migration and pgvector Schema

**Description:** Write the goose migration that adds the `embeddings` table with a `vector` column (pgvector) and creates the HNSW index. Also adds the `embed_status` column to `chunks` and the sqlc queries used by Stories 3.2 through 3.5. This story is a prerequisite for all other Epic 3 stories and should be implemented first.

**Covers:** FR-29 (provider config stores model/dimension), FR-36 (embed_status column)

**Applies:** AR-2 (pgvector HNSW index, pinned `pgvector/pgvector:0.8.2-pg17`), AR-8 (sqlc), AR-9 (goose migration)

**Complexity:** S

**Acceptance Criteria:**

- Given a fresh PostgreSQL 17 + pgvector 0.8.2 instance, when the goose migration runs, then the `embeddings` table exists with columns `id` (UUID PK), `chunk_id` (UUID FK → chunks), `workspace_hash` (text NOT NULL), `embedding` (vector), `created_at`, `updated_at`.
- Given the migration runs, then an HNSW index exists on `embeddings(embedding)` with `vector_cosine_ops` operator class.
- Given the migration runs, then the `chunks` table has an `embed_status` column (text, default `'pending'`, constraint: one of `'pending'`, `'embedded'`, `'embed_failed'`).
- Given the migration runs twice (idempotency), then no error is raised (goose up-to-date behavior).
- Given sqlc.yaml is configured and `sqlc generate` is run, then type-safe Go code exists for: `InsertEmbedding`, `GetPendingChunks`, `MarkChunkEmbedFailed`, `MarkChunkEmbedded`, `VectorSearch`, `CountPendingChunks`.
- Given a `workspace_hash` is passed to `VectorSearch`, then the generated SQL includes `WHERE workspace_hash = $1` (isolation invariant verified at query level, NFR-2).

**Unit tests:** run `go test ./internal/storage/...` with real PG to verify all sqlc queries compile and execute. Verify HNSW index creation via `\d embeddings` query in test setup.
