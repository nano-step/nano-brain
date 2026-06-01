# Gemini Triage — fix-stats-api-contract (#279)

PR: nano-step/nano-brain#280
Date: 2026-06-01
Bot reviewer: gemini-code-assist[bot]
Agent: Sisyphus

## Triage Table (R31)

| Comment ref | Verdict | Reasoning | Action |
|-------------|---------|-----------|--------|
| stats.go:L135 (workspace type assertion can panic) | VALID:medium | `c.Get("workspace").(string)` panics if value is nil or wrong type. Other handlers (e.g., query.go:35) use `, _ := ... .(string)` + empty check pattern. | Applied: use comma-ok type assertion and return 400 if empty. |
| stats.go:L195 (redundant total queries) | VALID:medium | `CountDocsByCollectionGrouped` and `CountChunksByEmbedStatus` already return aggregates that sum to docs_total and chunks_total. Two extra DB queries are redundant. | Applied: sum DocCount and ChunkCount in Go; removed CountDocumentsByWorkspace + CountChunksByWorkspace from interface; removed CountChunksByWorkspace SQL query (CountDocumentsByWorkspace stays since it has other callers). |

## Resolution Summary

- 2 VALID:medium findings addressed in 1 push cycle (R31 limit: 3)
- 0 FALSE_POSITIVE / DEFER / ACKNOWLEDGED findings

## Test Evidence Post-Fix

```
$ go build ./...                                                  exit 0
$ go vet ./...                                                    clean
$ go test -race -short ./internal/server/handlers/...             PASS
```

TestStats_ResponseShape still asserts `docs_total: 5, chunks_total: 13` — values match because the Go-computed totals (sum of cols[].DocCount = 5, sum of chunks[].ChunkCount = 10+2+1 = 13) equal what the removed queries would have returned.

## Loop count
1/3 push cycles.
