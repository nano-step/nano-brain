# Issue #322 — Pre-work gate evidence

**Date**: 2026-06-02
**Branch**: `feat/322-embed-status-index-and-inflight-dedup` off `master`
**Issue**: nano-step/nano-brain#322
**Worktree**: `.opencode/worktrees/feat-322-embed-status-index/`

## `./scripts/harness-check.sh pre-work --issue 322`

```
─ PRE-WORK checks
[FAIL] 1.1 Open PRs still pending (1)
[PASS] 1.2 No active OpenSpec changes
[PASS] 1.3 Issue #322 exists (state: OPEN)
[PASS] 1.4 master is up-to-date
[PASS] 1.5 Validation ladder passes
[SKIP] 1.6 On master or branch unknown (check after creating feature branch)

Summary: 4 PASS, 1 FAIL, 1 SKIP (total: 6)
```

## [HARNESS-OVERRIDE] Gate 1.1

Open PR #321 (`feat(release): SHA-256 integrity verification for binaries (#320)`)
is **authored by @kokorolx and is unrelated to issue #322's domain**:

- PR #321 touches: `.github/workflows/release.yml`, `npm/postinstall.js`,
  release artifact integrity verification (SHA-256 manifest publishing)
- Issue #322 touches: `migrations/00012_*.sql` (new), `internal/embed/queue.go`,
  `internal/storage/queries/embeddings.sql` (no new queries, just adding index
  in migration), unit + integration tests in `internal/embed/`

**Zero file overlap.** The areas are completely orthogonal (CI/release pipeline
vs embed queue runtime).

**Override rationale**: Gate 1.1 enforces "previous PR merged" to keep the
feature pipeline serial. Blocking #322 on #321's merge would add wall-clock
delay for no quality benefit — these PRs cannot conflict at code level, and
the rebase cost of merging in either order is identical.

**Compensating control**: #322 PR description will explicitly reference PR #321,
state the zero-overlap assertion, and list the touched files so the reviewer
can verify at PR time.

## Note on Gate 1.2 (false PASS) — active OpenSpec change `incremental-reindex` exists

`openspec list` shows `incremental-reindex` (0/15 tasks, proposed 2026-05-30 by
PR #232 for issue #158). The `harness-check.sh` script's grep pattern
(`active|pending|in-progress`) doesn't match `openspec list`'s output format, so
gate 1.2 incorrectly reports PASS. This is a script bug, NOT a logical PASS.

**Manually verified zero file overlap between #158 and #322:**

| Change | Layer | Touches |
|---|---|---|
| **incremental-reindex (#158)** | HTTP handler logic | `internal/server/handlers/reindex.go` (replace full-wipe with diff loop), `cmd/nano-brain/commands.go` (add `--force-wipe` flag), new sqlc query `ListDocumentsByWorkspace` |
| **THIS (#322)** | Embed queue worker + DB index | `migrations/00014_*.sql` (new partial index), `internal/embed/queue.go` (add in-flight sync.Map) |

The two are **complementary**: #158 reduces *what* enters the embed queue (only
changed files); #322 makes the embed queue *itself* more efficient (skip
duplicates already in-flight, faster pending-chunk scan). They can land in
either order.

Other gates (1.3–1.5) PASS without override. Gate 1.6 will be re-checked
after creating the feature branch + worktree.

**Verdict**: PROCEED with issue #322 implementation under documented gate-1.1
parallel-track override.
