# Gemini Triage — fix-chunker-ollama-hard-fail (#260)

PR: nano-step/nano-brain#267
Date: 2026-05-31
Bot reviewer: gemini-code-assist[bot]
Agent: Sisyphus

## Triage Table (R31)

| Comment ref | Agent verdict | Reasoning | Action |
|-------------|---------------|-----------|--------|
| PR#267 queue.go:396 (false-positive risk on `unexpected status 400`) | VALID:medium | Substring match could falsely match payload sizes like `"4000 bytes received"` or `400 found in body`. The format string from both providers always includes `<code>:` (verified ollama.go:64 and voyageai.go:76). Appending `:` disambiguates without provider-specific parsing. | Applied: changed all 5 entries in `hardFailureStatusCodes` to include trailing `:` (e.g. `"unexpected status 400:"`). Added 2 false-positive test cases to TestIsHardFailureEmbedError ("ollama: unexpected status 4000 bytes" → false; "error code 400 found in body" → false). |
| PR#267 queue.go:276 (missing publishStatus on hard-fail) | VALID:medium | All other terminal paths in processChunk (line 281, 299) call publishStatus to notify embed-queue subscribers (web UI, metrics). Skipping it on hard-fail leaves status counter stale until next event. | Applied: added `q.publishStatus()` after `q.clearRetries(chunkID)` in the hard-fail branch. |

## Resolution Summary

- 2 VALID:medium findings addressed in single push
- 1 push cycle (under R31 limit of 3)
- 0 FALSE_POSITIVE / DEFER / ACKNOWLEDGED findings

## Test Evidence Post-Fix

```
$ go test -race -short -run "TestIsHardFailureEmbedError|TestProcessChunk_HardFailOn400|TestProcessChunk_TransientErrorRetries" ./internal/embed/... -v
=== RUN   TestIsHardFailureEmbedError
  ... 16 sub-cases (including 2 new false-positive guards) ALL PASS ...
=== RUN   TestProcessChunk_HardFailOn400          --- PASS
=== RUN   TestProcessChunk_TransientErrorRetries  --- PASS
PASS
```
