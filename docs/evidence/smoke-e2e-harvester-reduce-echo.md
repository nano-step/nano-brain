# smoke:e2e — Issue #550 (harvester reduce-echo pollution)

Change-type: bug-fix. Verified over the real HTTP transport on an isolated
**:3199 / nanobrain_test** server (dev DB / :3100 never touched), against a
**real reduce call** — not a mocked pipeline unit test.

## Setup (isolated)

Since the bug is model-chattiness-dependent (a small proxy model echoing its
own prompt), reproducing it deterministically over a live call requires
controlling the LLM's response. Started a minimal local OpenAI-compatible
mock server (`/v1/chat/completions`) that always returns the exact echoed
scaffolding text from the issue report (reduce instruction + intermediate
chunk drafts + output-format template, then the real "## Goal" block).
Pointed nano-brain's summarization provider at it
(`NANO_BRAIN_SUMMARIZATION_PROVIDER_URL=http://127.0.0.1:8091/v1`). Seeded a
~20KB raw session document into `nanobrain_test`
(`collection=sessions`, `source_path=opencode://session/smoke-session-001`)
long enough to force the map-reduce path (`SingleShotThreshold=4000`).

## Real HTTP call

```
POST /api/v1/summarize?workspace=<hash>  {"limit":5,"force":true}
  → 200 {"summarized":1,"skipped":0,"errors":0}
```

## Persisted document (queried directly from nanobrain_test)

```sql
SELECT content FROM documents WHERE source_path = 'summary://opencode/smoke-session-001';
```

```
# Session: smoke test session

- Date: 2026-07-08
- Source: opencode
- Session ID: smoke-session-001


## Goal
Fixed the real auth bug end to end via live smoke test.

## Decisions Made
- Used JWT.

## Files Touched
- auth.go

## Problems Encountered
- None.

## Key Learnings
- Validate tokens early.
```

**Result:** zero scaffolding leaked through the real HTTP → pipeline → reduce
→ persist path. No "Merge two chunk summaries…", no "Bullet list.", no
duplicate draft "Goal:" lines — exactly the clean 5-section format, even
though the mock LLM deliberately echoed all of that noise on every call
(both map and the final reduce).

## Isolation / cleanup

Server on :3199 / nanobrain_test only; both the nano-brain server and the
mock LLM server killed by their captured PIDs (no broad kill). Seeded
workspace/document deleted, temp binary and mock server script removed.
