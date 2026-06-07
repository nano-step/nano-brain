# Design: Token-Aware Batching + Retry

## Architecture

```
RunOnce(ctx, workspaceHash)
    │
    ├─ Query unsummarized symbols
    ├─ Filter oversized (>max_symbol_lines)
    │
    ├─ Split into initial batches (batch_size)
    │
    └─ For each batch:
         │
         ├─ estimateTokens(batch)
         │    └─ if > max_batch_tokens → splitBatch(batch) recursively
         │
         └─ sendWithRetry(ctx, sub-batch)
              ├─ attempt 1 → success → upsert + enqueue
              ├─ attempt 1 → transient error → backoff 1s
              ├─ attempt 2 → transient error → backoff 3s
              ├─ attempt 3 → transient error → backoff 9s → mark FAILED
              └─ attempt N → permanent error → mark FAILED immediately
```

## Decision Log

### D1 — Token estimation strategy

**Chosen: chars/4 heuristic**

- Go code: ~3.5-4.5 chars/token average
- Prompt overhead: 150 tokens fixed + 25 tokens/symbol header
- Formula: `estimatedTokens = len(prompt_template)/4 + sum(len(symbol.Code)/4 + 25)`
- Good enough for splitting decision — not used for billing

### D2 — Auto-split algorithm

**Chosen: Recursive binary split**

```go
func (s *Service) splitAndSend(ctx, batch) {
    estimated := estimateTokens(batch)
    if estimated <= s.cfg.MaxBatchTokens || len(batch) == 1 {
        return s.sendWithRetry(ctx, batch)
    }
    mid := len(batch) / 2
    s.splitAndSend(ctx, batch[:mid])
    s.splitAndSend(ctx, batch[mid:])
}
```

- Base case: batch of 1 symbol always sent (even if large — max_symbol_lines already filters >500 lines)
- Max recursion depth: log2(30) = ~5 levels
- Each split = separate LLM request (costs RPD budget)

### D3 — Retry with error classification

**Chosen: 3 retries, exponential backoff, error classification**

| HTTP Status | Classification | Action |
|-------------|---------------|--------|
| 429 | Transient (rate limit) | Retry with backoff |
| 408, 500, 502, 503, 504 | Transient (server error) | Retry with backoff |
| Timeout (context deadline) | Transient | Retry with backoff |
| 400 | Permanent (bad request) | Skip, mark failed |
| 401, 403 | Permanent (auth) | Skip, mark failed |
| Invalid JSON after 3 retries | Permanent | Skip, mark failed |

Backoff formula: `time.Sleep(backoffBase * time.Duration(attempt*attempt))`
- Attempt 1: 1s
- Attempt 2: 4s  (1 * 2^2)
- Attempt 3: 9s  (1 * 3^2)

### D4 — Failed symbol tracking

**Chosen: New DB table `code_summarization_failures`**

```sql
CREATE TABLE code_summarization_failures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT NOT NULL,
    symbol_name TEXT NOT NULL,
    symbol_kind TEXT,
    source_file TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    error_reason TEXT NOT NULL,
    error_type TEXT NOT NULL,        -- 'transient_exhausted' | 'permanent' | 'token_overflow'
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,         -- set when retry succeeds
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_csf_workspace_unresolved 
    ON code_summarization_failures(workspace_hash) 
    WHERE resolved_at IS NULL;
```

- On permanent failure or 3 retries exhausted → insert row
- On successful retry (manual or auto) → set `resolved_at`
- Query for UI: `WHERE workspace_hash = $1 AND resolved_at IS NULL`

### D5 — Web UI endpoints (PR2)

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v1/code/summarize/status?workspace=X` | Counts: processed, pending, failed, skipped |
| `GET /api/v1/code/summarize/failures?workspace=X` | List unresolved failures |
| `POST /api/v1/code/summarize/retry` | Retry specific symbols: `{"workspace":"X","symbol_ids":["uuid1",...]}` |
| `POST /api/v1/code/summarize/retry-all` | Retry all unresolved failures for workspace |

### D6 — Logging

Every retry logs:
```json
{"level":"warn","component":"code-summarize-service","attempt":2,"max":3,"backoff_s":4,"error":"HTTP 429","batch_size":15,"msg":"retrying batch"}
```

Every permanent failure logs:
```json
{"level":"error","component":"code-summarize-service","symbol":"gitignoreStack","file":"internal/watcher/filter.go","error_type":"permanent","reason":"HTTP 400: invalid request","msg":"symbol summarization permanently failed"}
```

## Config

```yaml
code_summarization:
  # Existing fields...
  max_batch_tokens: 100000      # auto-split threshold
  max_retries: 3                # per sub-batch  
  retry_backoff_seconds: 1      # base (multiplied by attempt^2)
```

## PR Split

### PR1: Backend (this PR)
- Token estimation (`internal/codesummarize/tokens.go`)
- Auto-split in service.go (`splitAndSend` replacing direct batch loop)
- Retry with backoff (`internal/codesummarize/retry.go`)
- Error classification
- Failed symbol tracking (migration + sqlc queries)
- Config fields added

### PR2: Web UI
- Status endpoint + failures list endpoint
- Retry/retry-all endpoints
- Web UI page (if webui exists) or just API endpoints
