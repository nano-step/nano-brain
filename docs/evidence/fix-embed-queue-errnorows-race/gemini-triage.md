# Gemini Triage — fix-embed-queue-errnorows-race (#259)

PR: nano-step/nano-brain#266
Date: 2026-05-31
Bot reviewer: gemini-code-assist[bot]
Agent: Sisyphus

## Triage Table (R31)

| Comment ref | Agent verdict | Reasoning | Action |
|-------------|---------------|-----------|--------|
| PR#266 queue.go:237 (memory leak in q.retries map) | VALID:medium | If a chunk had been retried before getting cascade-deleted, `q.retries[chunkID]` would remain in the map forever (no cleanup path). The fix matches the existing cleanup pattern at line 281 (post-success) and line 299 (terminal failure). Verified: `clearRetries` is the canonical cleanup. | Applied: added `q.clearRetries(chunkID)` after `q.pending.Add(-1)` in the ErrNoRows branch. Test updated to seed `q.retries[chunkID] = 2` and assert it's cleared post-processChunk. |

## Resolution Summary

- 1 VALID:medium finding addressed
- 1 push cycle (under R31 limit of 3)
- 0 FALSE_POSITIVE / DEFER / ACKNOWLEDGED findings

## Test Evidence Post-Fix

```
$ go test -race -short -run "TestProcessChunk_SkipsDeletedChunk|TestProcessChunk_OtherDBErrorStillLogsError" ./internal/embed/... -v
=== RUN   TestProcessChunk_SkipsDeletedChunk
--- PASS: TestProcessChunk_SkipsDeletedChunk (0.00s)
=== RUN   TestProcessChunk_OtherDBErrorStillLogsError
--- PASS: TestProcessChunk_OtherDBErrorStillLogsError (0.00s)
PASS
ok   github.com/nano-brain/nano-brain/internal/embed  1.021s
```

The TestProcessChunk_SkipsDeletedChunk test now also seeds the retries map and asserts cleanup — providing direct regression coverage for the memory-leak fix.
