# Self-Review: feat-322-embed-status-index-and-inflight-dedup

Issue: [#322](https://github.com/nano-step/nano-brain/issues/322)
Lane: normal | Change type: user-feature + index-schema
Branch: `feat/322-embed-status-index-and-inflight-dedup`
Commits: `d42ce52` (proposal), `e98d5d2` (implementation)

## Actions Taken
- Ran 4-stream research (2 explore + librarian + Oracle cross-check) audit of harvest → embed pipeline; Oracle rejected 10/15 candidate optimizations as premature at current scale, kept 2.
- Created GitHub issue #322 with full context (Oracle verdict, scope, AC) before classification.
- Ran PRE-WORK gate; [HARNESS-OVERRIDE] documented for gate 1.1 (unrelated open PR #321, zero file overlap).
- Created worktree `.opencode/worktrees/feat-322-embed-status-index/` and branch.
- Wrote OpenSpec proposal (proposal.md + design.md + spec deltas + tasks.md); validated `--strict --no-interactive` PASS.
- Ran Metis + Oracle deep-design pass in parallel; Oracle identified BLOCKER (handleRetry side-channel) + Metis identified 5 naming/signature inaccuracies.
- Revised design.md with D12 (conditional defer driven by handleRetry bool), D13 (first-use NO TRANSACTION + recovery procedure), D14 (deferred CHECK constraint fix); revised spec.md with new requirement covering handleRetry contract; revised tasks.md with 3 additional D12-coverage tests.
- Re-validated proposal; committed as `d42ce52`.
- Delegated Go implementation to Sisyphus-Junior with full 6-section task brief.
- Subagent created migration 00014, modified queue.go (Queue + Enqueue + processChunk + handleRetry + minor scanByStatus fix to distinguish dedup-skip from queue-full), added 9 unit tests + 2 integration tests, updated CHANGELOG.md + internal/embed/AGENTS.md.
- Verified all changes: `go build ./... && go test -race -short ./...` PASS; `go test -race -tags=integration ./internal/embed/...` PASS; EXPLAIN ANALYZE confirms `Index Scan using idx_chunks_embed_status`.

## Files Changed
- `migrations/00014_add_chunks_embed_status_index.sql` — new partial composite index via CONCURRENTLY + NO TRANSACTION
- `internal/embed/queue.go` — add `inflight sync.Map`; rewrite Enqueue with LoadOrStore; rewrite handleRetry → bool; conditional defer in processChunk; scanByStatus distinguishes dedup-skip from queue-full
- `internal/embed/queue_test.go` — 9 new unit tests (dedup, panic, channel-full, backpressure, re-enqueue, 3 handleRetry scenarios)
- `internal/embed/queue_integration_test.go` — new file; 2 tests (scanByStatus skip, migration index exists)
- `CHANGELOG.md` — `[Unreleased] ### Performance` entries
- `internal/embed/AGENTS.md` — 1 paragraph documenting `inflight sync.Map` invariant
- `openspec/changes/embed-status-index-and-inflight-dedup/` — proposal + design + spec + tasks
- `docs/evidence/322-pre-work-gate.md` — PRE-WORK gate output + override rationale
- `docs/evidence/322-explain-analyze.txt` — EXPLAIN ANALYZE proof (Index Scan, not Seq Scan)
- `docs/evidence/self-review-feat-322-embed-status-index.md` — this file

## Findings Summary
- Critical: 1 (Oracle BLOCKER — handleRetry side-channel)
- Major: 5 (Metis: Enqueue signature, rejectionThreshold name, chunkID rename, EXPLAIN automated check, handleRetry test coverage)
- Minor: 3 (Metis: struct field placement, prior NO TRANSACTION claim, pre-existing CHECK constraint mismatch)

## Critical
| Finding | Status | Reasoning |
| --- | --- | --- |
| Oracle BLOCKER — `handleRetry` bypasses `Enqueue`'s LoadOrStore by sending directly to `q.ch`; unconditional `defer inflight.Delete` would violate invariant I1 (chunk in channel but not in inflight set) → next scanByStatus would double-enqueue | FIXED in `e98d5d2` | Decision D12 added to design.md. `handleRetry` returns `bool`; processChunk uses conditional defer `if !requeued { inflight.Delete }`. Test `TestQueue_HandleRetry_KeepsInflightOnSuccessfulReenqueue` enforces. |

## Major
| Finding | Status | Reasoning |
| --- | --- | --- |
| Metis: Enqueue signature in proposal showed void, actual returns `bool` | FIXED in `d42ce52` | Updated design.md code blocks; ensured all paths return correct bool. |
| Metis: constant name `maxPendingThreshold` doesn't exist (actual: `rejectionThreshold`) | FIXED in `d42ce52` | Find-replace in design.md + spec.md. |
| Metis: parameter `id` should be `chunkID` to match existing convention | FIXED in `d42ce52` + `e98d5d2` | Renamed throughout proposal + implementation. |
| Metis: AC#4 EXPLAIN ANALYZE not agent-executable | FIXED in `e98d5d2` | Added `TestMigration_EmbedStatusIndex_Exists` querying `pg_indexes`; EXPLAIN remains manual evidence. |
| Metis: test plan missing handleRetry coverage | FIXED in `d42ce52` + `e98d5d2` | Added 3 tests: keepsInflightOnSuccessfulReenqueue, deletesInflightOnChannelFull, deletesInflightOnMaxRetries. |

## Minor
| Finding | Status | Reasoning |
| --- | --- | --- |
| Struct field placement unspecified | FIXED in `e98d5d2` | `inflight sync.Map` placed after `pending atomic.Int64` per directive. |
| `"verified by grep"` claim about prior NO TRANSACTION use was wrong | FIXED in `d42ce52` | Re-verified: 0 prior uses. D13 corrected + interrupted-CONCURRENTLY recovery procedure documented. |
| Pre-existing CHECK constraint mismatch (`embed_permanently_failed` not in CHECK from migration 00004) | DEFERRED | D14: out of scope for #322. Will file follow-up issue post-merge. Migration 00004 CHECK only allows `('pending','embedded','embed_failed')` but `MarkChunkEmbedPermanentlyFailed` writes `'embed_permanently_failed'` → PG would raise 23514 silently. Not introduced by this PR. |

## Gemini Verification Triage
| Comment ref | Agent verdict | Reasoning | Action |
| --- | --- | --- | --- |
| (PR not yet opened — Gemini cycle pending) | N/A | Will fill in after PR open + Gemini bot review | N/A |

## Resolution Status
- All critical: FIXED (1/1)
- All major: FIXED (5/5)
- All minor: 2 FIXED, 1 DEFERRED (D14 — pre-existing, not regression, will file follow-up issue)
- Open items: D14 follow-up issue tracking

## validate:quick PASS
```
$ go build ./... && go test -race -short ./...
ok  github.com/nano-brain/nano-brain/cmd/nano-brain
ok  github.com/nano-brain/nano-brain/internal/embed  1.170s
[all packages PASS]
```

## test:integration PASS
```
$ go test -race -tags=integration ./internal/embed/... -v
--- PASS: TestQueue_ScanByStatus_SkipsInflightChunks
--- PASS: TestMigration_EmbedStatusIndex_Exists
[all 40+ tests PASS]
```

## EXPLAIN ANALYZE
See `docs/evidence/322-explain-analyze.txt`. Confirmed `Index Scan using idx_chunks_embed_status`, not `Seq Scan on chunks`.

## smoke:e2e SKIP
Per D11 in design.md: no API surface change (no new HTTP endpoints, CLI commands, or MCP tools). The migration creates a partial index via CONCURRENTLY (no table lock). The in-flight dedup is purely internal to the embed queue worker; `/api/status` does not expose `inflight` size by design (D10 — no metrics scope creep). Migration verified via `TestMigration_EmbedStatusIndex_Exists` integration test + manual EXPLAIN ANALYZE evidence.

## Friction
- `harness-check.sh` gate 1.2 has a false-positive bug: `openspec list` output doesn't match the script's grep pattern `active|pending|in-progress`, so the gate incorrectly reports PASS when there ARE active OpenSpec changes (e.g. `incremental-reindex` was active). Worked around with manual verification + override evidence.
- TRACE_SPEC Tier 2 filename convention `self-review-<slug>.md` was not propagated to the subagent implementation prompt; subagent created `322-self-review.md` initially, renamed during gate check.
