# smoke:e2e — Issue #747 (BM25 OR fallback ignores time range)

Change-type: bug-fix. This ran on an isolated **:3199 / nanobrain_test** server. The dev DB and :3100 were not touched.

## Setup
I seeded nanobrain_test with a throwaway workspace (`PLACEHOLDER_HASH`) and two docs. Both have the content "review notes":
- `doc-2026-11-27`: created and updated 1 hour ago (inside the window)
- `doc-2026-09-28`: created and updated 60 days ago (outside the window)

Request (`created_after` = now − 24h). The query has 3 words, the AND tsquery matches nothing, so the OR fallback runs:

```
curl -si -X POST http://localhost:3199/api/v1/query -H 'Content-Type: application/json' \
  -d '{"workspace":"PLACEHOLDER_HASH","query":"review deploy rollback","created_after":"<now-24h>","max_results":40}'
```

## Before: origin/master binary
```
HTTP/1.1 200 OK
titles: ["doc-2026-11-27", "doc-2026-09-28"]   total=2   <- the 60-day-old doc leaks
```

## After: fix/747 binary
```
HTTP/1.1 200 OK
titles: ["doc-2026-11-27"]   total=1   <- the out-of-window doc is filtered out
```

## Isolation / cleanup
Each server was started on :3199 and killed by its captured PID. The seeded workspace, docs and chunks were deleted afterwards.
