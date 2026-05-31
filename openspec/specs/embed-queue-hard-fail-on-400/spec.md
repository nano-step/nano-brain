# embed-queue-hard-fail-on-400 Specification

## Purpose
TBD - created by archiving change fix-chunker-ollama-hard-fail. Update Purpose after archive.
## Requirements
### Requirement: Embed worker hard-fails on 4xx provider errors

The embed queue worker SHALL classify errors returned by the embedding provider (`q.embedder.Embed`) into two categories:

- **Hard failures**: HTTP status codes 400, 401, 403, 413, 422. Detected by substring match on the error string (`"unexpected status 4XX"` — used by both ollama and voyageai providers in their error wrappers).
- **Transient errors**: anything else (network errors, timeouts, 5xx server errors, unparseable errors). These continue to use the existing `increaseBackoff()` + `handleRetry()` path.

On hard failure, the worker SHALL:
1. Emit an ERROR-level log (preserving existing behavior for diagnostic visibility)
2. Call `MarkChunkEmbedFailed` on the chunk (DB persists `embed_status = 'failed'` and the error message)
3. Decrement the pending counter
4. Clear the retry-counter map entry for the chunk
5. Return without invoking `increaseBackoff()` or `handleRetry()`

This prevents infinite retry on permanent failures (e.g., content too large for the provider's context window) while preserving existing retry behavior for transient errors.

#### Scenario: ollama 400 marks chunk failed and does not retry

- **GIVEN** an embed worker is processing chunk `c1`
- **WHEN** `q.embedder.Embed(...)` returns an error with message containing `"ollama: unexpected status 400: the input length exceeds the context length"`
- **THEN** the worker emits an ERROR log with the chunk_id and error
- **AND** the worker calls `MarkChunkEmbedFailed` with `ID=c1` and the error message
- **AND** the worker does NOT call `increaseBackoff` (backoff counter unchanged)
- **AND** the worker does NOT call `handleRetry` (no re-enqueue)
- **AND** the retry map entry for `c1` is cleared
- **AND** the pending counter is decremented by 1

#### Scenario: connection refused triggers retry, not hard-fail

- **GIVEN** an embed worker is processing chunk `c2`
- **WHEN** `q.embedder.Embed(...)` returns an error with message containing `"connection refused"`
- **THEN** the worker emits an ERROR log
- **AND** `increaseBackoff()` is called (existing transient-retry path)
- **AND** `handleRetry()` is called (re-enqueue with backoff)
- **AND** `MarkChunkEmbedFailed` is NOT called

#### Scenario: voyageai 401 also hard-fails

- **GIVEN** voyageai is the configured embedder
- **WHEN** `q.embedder.Embed(...)` returns `"voyageai: unexpected status 401: invalid api key"`
- **THEN** the worker applies the hard-fail path (same as ollama 4xx)
- **AND** chunk marked `embed_failed` with the auth error preserved in DB

#### Scenario: 5xx server errors retry

- **GIVEN** an embed worker is processing chunk `c3`
- **WHEN** the provider returns `"ollama: unexpected status 503: service unavailable"`
- **THEN** the worker follows the transient retry path (5xx is not in the hard-fail set)
- **AND** `MarkChunkEmbedFailed` is NOT called immediately
- **AND** retry counter increments
- **AND** chunk re-enqueues after backoff

### Requirement: Hard-failure classifier covers all configured providers

The `isHardFailureEmbedError(err)` helper SHALL match error strings produced by ALL supported embedding providers (currently ollama and voyageai). Both providers emit errors of the form `"<provider>: unexpected status <code>: <body>"` (verified in `internal/embed/ollama.go:64` and `internal/embed/voyageai.go:76`).

The classifier uses substring match on `"unexpected status 4XX"` rather than provider-specific parsing — provider-agnostic by design so new providers added later inherit the behavior automatically.

#### Scenario: Classifier table-driven cases

- **WHEN** `isHardFailureEmbedError` is called with each error string
- **THEN** the result matches expected:

| Input error message | Expected |
|---|---|
| `"ollama: unexpected status 400: input length exceeds"` | true |
| `"ollama: unexpected status 401: unauthorized"` | true |
| `"ollama: unexpected status 403: forbidden"` | true |
| `"ollama: unexpected status 413: payload too large"` | true |
| `"ollama: unexpected status 422: invalid input"` | true |
| `"voyageai: unexpected status 400: bad request"` | true |
| `"voyageai: unexpected status 401: invalid key"` | true |
| `"ollama: unexpected status 500: internal error"` | false |
| `"ollama: unexpected status 502: bad gateway"` | false |
| `"ollama: unexpected status 503: unavailable"` | false |
| `"connection refused"` | false |
| `"context deadline exceeded"` | false |
| `""` (empty) | false |
| `nil` | false |

