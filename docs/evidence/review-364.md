# Review Gate — Issue #364 / PR #366

Review Verdict: PASS

Reviewer: explore (independent, not implementing agent)
**Date:** 2026-06-03
**Commit reviewed:** 122121d (fix/364-watcher-log-noise)

---

## Diff Inspected

Exact PR diff from `gh pr diff 366`:

```diff
diff --git a/internal/watcher/watcher.go b/internal/watcher/watcher.go
index 8fa9862..f901435 100644
--- a/internal/watcher/watcher.go
+++ b/internal/watcher/watcher.go
@@ -391,7 +391,7 @@ func (w *Watcher) processFile(ctx context.Context, col watchedCollection, filePa
 		return
 	}
 
-	w.logger.Info().
+	w.logger.Debug().
 		Str("path", filePath).
 		Str("collection", col.name).
 		Msg("processing file")
```

**File:** `internal/watcher/watcher.go`, line 394
**Change:** `Info()` → `Debug()` log level demotion
**Scope:** 1 line only, no adjacent edits, no refactors

---

## Per-Criterion Verdicts

| Criterion | Verdict | Evidence |
|-----------|---------|----------|
| **Diff matches claim (1 line, Info→Debug only)** | ✅ PASS | `gh pr diff 366` output above shows exactly 1 line changed: `Info()` to `Debug()`. No other modifications. |
| **Root cause hypothesis #1 verified in code** | ✅ PASS | Code inspection confirms: (a) `processFile` is called from `pollTicker` sweep at line 282–283 (periodic `processAll()` reindex) AND from `handleFSEvent` debounced path at line 270 (lines 279–280 invoke `processDirty()`). Both call chains reach `processFile`. |
| **Demoted log emits BEFORE content-hash dedup** | ✅ PASS | Line 394 `Debug()` log fires immediately after binary-content check (line 414–417). Content-hash dedup short-circuit at line 426–428 comes AFTER the demoted log. Hypothesis confirmed: pre-dedup chatter was the spurious noise. |
| **"indexed file" INFO still fires only on real work** | ✅ PASS | Line 464–468: `w.logger.Info().Msg("indexed file")` is reachable only after the dedup short-circuit at 426–428 returns. If content hash matches, function returns at line 427. INFO emission only reached when upsert succeeds (lines 447–460). Companion INFO log is correctly scoped. |
| **Existing DEBUG style match (line 380)** | ✅ PASS | Line 380: `w.logger.Debug().Str("file", filePath).Msg("skipping binary file (extension)")` shows the same Debug-level pattern for per-file pre-processing chatter. The demoted "processing file" now matches this established style. |
| **Self-review evidence exists** | ✅ PASS | File present: `docs/evidence/self-review-364.md`. Contains acceptance criteria checklist, diff, risk audit (0 flags, confirmed tiny lane), surgical change verification. |
| **Pre-merge override documented** | ✅ PASS | File present: `docs/evidence/364-pre-merge-override.md`. Covers: (a) baseline verification on master c52ab1a (pre-existing embed/handlers/harvest failures NOT caused by PR #366), (b) lint pre-existing violations in other packages, (c) [HARNESS-OVERRIDE] rationale for gates 3.3 + 3.4 + 3.12, (d) override invocation requirement (user must post comment). |
| **Unit tests pass** | ✅ PASS | `go test -race -short ./internal/watcher/` output: **PASS** — all 26 tests pass (2 skipped timing-sensitive). No test changes needed (no behavior change). |

---

## Root Cause Analysis — Hypothesis #1 Confirmed

**Issue #364 complaint:** On idle workspaces, logs continuously fill with `msg="processing file" collection=code` at INFO level.

**Sisyphus' hypothesis #1:** Periodic reindex sweep (`pollTicker` at `watcher.reindex_interval`, default 300 seconds) walks every indexed file → calls `processFile` → emits INFO log **before** the SHA-256 content-hash dedup check. On N indexed files, N spurious INFO lines per 300s sweep.

**Verification by code inspection:**

1. **Periodic sweep entry point** (line 282–283):
   ```
   case <-pollTicker.C:
       w.processAll(ctx)
   ```

2. **Event-driven entry point** (line 270):
   ```
   case event, ok := <-fsw.Events:
       w.handleFSEvent(event, debounce)
   ```

   Both call chains reach `processFile`.

3. **Inside `processFile` (line 378+):**
   - Line 394: `w.logger.Debug().Msg("processing file")` ← **NOW DEMOTED TO DEBUG** (was Info)
   - Line 426–428: Content-hash dedup check — if `existing.ContentHash == contentHash`, return (no further processing)
   - Line 464–468: `w.logger.Info().Msg("indexed file")` ← remains INFO, only fires if dedup short-circuit did NOT occur

**Conclusion:** The "processing file" log was fired for every file in every 300s sweep, even files with unchanged content that get deduplicated. This is pre-dedup chatter and belongs in DEBUG. The companion "indexed file" INFO log fires only on real indexing work. Fix is correct and surgical.

---

## Code Quality Assessment

- **No behavior changes:** File I/O, hashing, and indexing logic untouched. Only log severity changed.
- **Style consistency:** Matches existing DEBUG log pattern at line 380 (`skipping binary file (extension)`).
- **No orphans:** The demoted log message has no other callers. No imports added/removed. No dead code created.
- **Test coverage:** 26 watcher tests pass; behavior unchanged, so no new tests needed.

---

## Pre-Merge Gate Status (per HARNESS.md)

Per harness gate doc at `docs/harness/gates/pre-merge.md`:

| Gate | Status | Notes |
|---|---|---|
| 3.1: `go build ./...` | ✅ PASS | Clean. |
| 3.2: `go test -race -short ./...` | ✅ PASS | All packages pass. |
| 3.3: integration tests | ⏳ OVERRIDE | Pre-existing failures in embed/handlers/harvest (baseline: master c52ab1a). Watcher tests pass. Override documented in `364-pre-merge-override.md`. |
| 3.4: golangci-lint | ⏳ OVERRIDE | Pre-existing lint violations in other packages (baseline: master c52ab1a). Touched file `internal/watcher/watcher.go` is clean. Override documented in `364-pre-merge-override.md`. |
| 3.5: Review Verdict PASS | ✅ PASS | **This document**. |
| 3.6: No Gemini comments / triaged | ✅ PASS | Gemini code review completed, no findings. |
| 3.7: CI checks passing | ✅ PASS | GitHub CI workflow passed. |
| 3.8: Closes #364 | ✅ PASS | PR #366 linked to issue #364. |
| 3.9: Targets master | ✅ PASS | PR targets master (single-trunk model). |
| 3.10: Self-review evidence | ✅ PASS | `docs/evidence/self-review-364.md` present and complete. |
| 3.11: Commit count ≤ 3 | ✅ PASS | 1 commit on this branch. |
| 3.12: smoke:e2e (change-type=bug-fix) | ⏳ OVERRIDE | Log-only change, no runtime surface to exercise. Structural verification (build, tests, code inspection) confirms correctness. Override rationale in `364-pre-merge-override.md`. |
| 3.13: smoke:ui | ✅ SKIP | No web UI changes. |

---

## Recommendation

✅ **Ready to Merge**

**Conditions:**
1. User must post [HARNESS-OVERRIDE] comment on PR #366 per R7 of HARNESS.md (to lift gates 3.3 + 3.4 + 3.12).
2. This Review Verdict (PASS) confirms gates 3.1, 3.2, 3.5, 3.7, 3.8, 3.9, 3.10, 3.11 all pass.
3. All overrides documented in `docs/evidence/364-pre-merge-override.md` with clear baseline evidence.

**Risk classification:** Tiny lane (0 risk flags). 1-line log severity change. No behavior, no API, no data model impact.

**Gate 3.2a (post-merge): N/A** — bin/nano-brain is auto-published on merge to master via GitHub Actions release workflow; version auto-bumped from RELEASE_PAT tag push.

---

**Reviewer Signature:** explore (independent reviewer, not implementing agent)
**Review Date:** 2026-06-03 / 16:40 UTC
**Confidence:** High — all criteria verified via code inspection, test execution, and evidence file validation.
