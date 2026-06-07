# Token-Aware Batching + Retry + Web UI Actions

Tracking: #399 (parent: #397)

## Problem

Code summarization has 3 gaps:
1. Batch fails forever if total tokens > model context limit (no token estimation)
2. No explicit retry — transient API errors (429, timeout) cause permanent skips
3. No visibility into failed symbols — can't manually retry or inspect

## Solution

### PR1: Backend — Token estimation + auto-split + retry

1. **Token estimation**: Before sending batch, estimate tokens (chars/4 heuristic). If estimated > `max_batch_tokens`, recursively split batch in half until under threshold.
2. **Retry with backoff**: Max 3 retries per sub-batch. Exponential backoff (1s, 3s, 9s). Classify transient vs permanent errors.
3. **Failed symbol tracking**: Persist failed symbols in DB with error reason for UI consumption.

### PR2: Web UI — Status dashboard + retry actions

1. Status view: processed / pending / failed / skipped counts per workspace
2. Failed symbols list with error reason
3. Retry button (per symbol or per batch)
4. Add-to-batch action (manually queue symbols)

## Config additions

```yaml
code_summarization:
  max_batch_tokens: 100000      # auto-split threshold (default 100K)
  max_retries: 3                # per sub-batch
  retry_backoff_seconds: 1      # base backoff (1s * attempt^2)
```

## Non-Goals (this change)

- Accurate tokenizer (tiktoken/sentencepiece) — chars/4 is sufficient
- Priority-based symbol ordering
- Background polling service (PR2 of #397)
