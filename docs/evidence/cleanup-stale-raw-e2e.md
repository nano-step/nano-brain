# cleanup-stale-raw End-to-End Evidence

**Date**: 2026-05-29
**Issue**: #190
**Branch**: b-main

## Setup

Seeded `ws-cleanup-e2e` workspace with:
- 2 raw docs (`opencode://session/x1`, `opencode://session/x2`) — collection `sessions`
- 2 matching summary docs (`summary://opencode/x1`, `summary://opencode/x2`) — collection `session-summary`
- 1 orphan raw doc (`opencode://session/x3`) with NO matching summary

before: raw_count=3

## Dry-run

```
$ nano-brain cleanup-stale-raw --dry-run
Dry run: 2 stale raw OpenCode session document(s) would be deleted.
No changes written.
```
✅ Correctly reports 2 (x1+x2 have matching summaries; x3 does not).

## Destructive run

```
$ nano-brain cleanup-stale-raw
Deleted 2 stale raw OpenCode session document(s).
```
✅ Logs `deleted_count=2`.

## Post-state

```
raw_remaining=1 (want 1 — the orphan without matching summary)
  remaining: opencode://session/x3        # orphan preserved
  remaining: summary://opencode/x1        # summary preserved
  remaining: summary://opencode/x2        # summary preserved
```
✅ Only the orphan raw doc survived. Summaries untouched.

## Idempotent re-run

```
$ nano-brain cleanup-stale-raw
No stale raw OpenCode session documents found. Nothing to do.
```
✅ `stale_count=0`, exit 0.
