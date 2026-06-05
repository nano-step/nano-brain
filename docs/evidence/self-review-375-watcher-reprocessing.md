# Self-Review: Issue #375 — Eliminate Unnecessary File Re-Processing

## Requirement Checklist

### Task 1: mtime+size cache

- [x] `fileState` struct added with `ModTime`, `Size`, `Hash` fields
- [x] `fileCache map[string]fileState` + `fileCacheMu sync.RWMutex` on Watcher struct
- [x] Initialized in `New()` constructor
- [x] Fast-path check in `processFile()` after `os.Stat`, before file read
- [x] Cache updated after successful indexing
- [x] Cache updated after hash-match early return (no-op path)
- [x] Cache evicted on Remove events in `handleFSEvent()`

### Task 2: Filter excluded dir events

- [x] `.git/`, `node_modules/`, etc. fsnotify events filtered in `handleFSEvent()`
- [x] Uses existing `defaultExcludeDirs` from `filter.go`
- [x] Returns early before marking parent directory dirty

### Task 3: Remove noisy debug log

- [x] Removed "processing file" debug log that fired for every file examined
- [x] "indexed file" Info log retained (fires only when actual work happens)

### Task 4: Smart polling

- [x] `hasNewEvents atomic.Bool` added to Watcher struct
- [x] Flag set in `handleFSEvent()` after excluded-dir filter
- [x] Poll ticker checks and clears flag; skips `processAll()` when no events

## Response Shape Verification

N/A — no API response changes in this fix.

## Staged Files Check

```
modified:   internal/watcher/watcher.go
modified:   internal/watcher/watcher_test.go
```

No unrelated files staged. No `.opencode/`, `package-lock.json`, or evidence files in commit.

## Test Evidence

```
ok  github.com/nano-brain/nano-brain/internal/watcher  1.139s (race enabled)
ok  github.com/nano-brain/nano-brain/...               all packages pass
```

3 new tests added:
- `TestProcessFile_SkipsUnchangedMtime` — verifies mtime cache prevents re-read
- `TestHandleFSEvent_SkipsGitEvents` — verifies .git/ events don't mark dirty
- `TestPollTicker_SkipsWhenNoEvents` — verifies idle polling skips processAll

## Potential Issues

- Cache is unbounded but naturally limited to files in watched collections (no eviction policy needed for typical workspace sizes)
- `defaultExcludeDirs` filter uses `strings.Contains` which could false-match paths containing dir names as substrings (e.g., a file named `my.git.backup`). Acceptable tradeoff — such paths are pathological edge cases.
