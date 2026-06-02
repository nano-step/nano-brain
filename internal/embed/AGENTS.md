# internal/embed — Embedding Queue

**Purpose:** Embedding queue — async worker that generates vector embeddings via Ollama or Voyage AI.

## Architecture

Chunks enqueued by ID → buffered channel (cap 10,000) → N goroutine workers → `Embedder.Embed` → `InsertEmbedding` into pgvector column. Documents are written first; embedding is background-only. On startup `Run` scans for pending/failed chunks; a periodic rescan (5 min) recovers stragglers.

## Files

| File | Responsibility |
|------|---------------|
| `embed.go` | `Embedder` interface: `Embed(ctx, text) ([]float32, error)` + `Dimension() int` |
| `factory.go` | `NewFromConfig` — selects Ollama or VoyageAI from `config.EmbeddingConfig` |
| `ollama.go` | `OllamaEmbedder` — POST `/api/embed`; default model `nomic-embed-text`, dim 768 |
| `voyageai.go` | `VoyageAIEmbedder` — POST Voyage AI REST API; default model `voyage-3`, dim 1024 |
| `queue.go` | `Queue` — channel, semaphore workers, exponential backoff, retry, DB scan |

## Provider Abstraction

Both providers implement `Embedder`. Ollama is local, no key required. VoyageAI requires `VOYAGE_API_KEY`; constructor returns error if absent. `dimension: 0` uses provider default (768 Ollama, 1024 Voyage).

## Concurrency and Backoff

- Semaphore limits concurrent embed calls to `concurrency` (default 4).
- Backpressure: enqueues rejected when pending backlog >= 50,000.
- Embed timeout: 2 min per chunk. Content truncated to 4,000 chars (fits 2k-token model window).
- Retry limit: 3; then `MarkChunkEmbedFailed`. Backoff: 60 s base, 1.5x multiplier, 300 s cap.
- FK violation on `InsertEmbedding` (chunk deleted mid-flight) is silently skipped.

## Config (`config.EmbeddingConfig`)

| Field | Default | Notes |
|-------|---------|-------|
| `provider` | `ollama` | `ollama` or `voyageai` |
| `url` | `http://localhost:11434` | Base URL for Ollama; full endpoint for Voyage |
| `model` | `nomic-embed-text` | Provider-specific model |
| `dimension` | `0` | 0 = use provider default |
| `concurrency` | `4` | Parallel worker count |

## Key Types

- `Queue` — owns channel, workers, backoff state (`Queue.mu`), retry map (`retriesMu`), `inflight sync.Map`.
- `QueueQuerier` — narrow 6-method DB interface injected into `Queue`.

## In-Flight Dedup Invariant

`Queue.inflight` (`sync.Map`) tracks chunk IDs currently in the channel buffer or being processed. `Enqueue` uses `LoadOrStore` to skip duplicates; `processChunk` uses a conditional `defer` to clean up — skipping cleanup only when `handleRetry` successfully re-enqueues the chunk (the next `processChunk` invocation owns that cleanup).
