# Gemini Review Triage: PR #388 (issue #375 — watcher performance fix)

**Branch:** `fix/375-watcher-unnecessary-reprocessing`  
**Reviewer:** gemini-code-assist[bot]  
**Date:** 2026-06-05

## Findings

| # | File | Line | Severity | Verdict | Action |
|---|------|------|----------|---------|--------|
| 1 | watcher.go | 91 | MEDIUM | INVALID:premature-optimization | Precomputing path segments saves one `string(os.PathSeparator)` alloc per excluded dir per event. In practice, fsnotify fires <10 events/sec and `defaultExcludeDirs` has ~8 entries. The optimization saves nanoseconds on a millisecond-scale path. Not worth the complexity of maintaining parallel slices. |
| 2 | watcher.go | 324 | MEDIUM | INVALID:same-as-1 | Companion to finding #1 — same rationale. |
| 3 | watcher.go | 339 | MEDIUM | VALID:medium | Rename events should also evict from fileCache. Valid correctness fix. |
| 4 | watcher_test.go | 504 | MEDIUM | VALID:medium | Test uses `.git/index` whose parent is `.git/` (not watched). Should use `.git` directly so parent is the watched dir. Valid test robustness fix. |

## Resolution

- Findings 1-2: INVALID — premature optimization for a cold path (<10 events/sec × 8 dirs = ~80 string ops/sec). No action.
- Finding 3: Fixed — added `fsnotify.Rename` to cache eviction condition.
- Finding 4: Fixed — test now uses `filepath.Join(dir, ".git")` so parent = watched dir.
