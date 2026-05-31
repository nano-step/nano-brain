# Tasks: Global .nano-brainignore Support

Tracking: #263

## Phase A — Loader

- [ ] **A1** Add `loadGlobalIgnore(homeDir string, logger zerolog.Logger) *gitignore.GitIgnore` to `internal/watcher/filter.go`:
  - Resolve `path := filepath.Join(homeDir, ".nano-brain", ".nano-brainignore")`
  - `os.Stat(path)` — if missing, return `nil` + DEBUG log
  - `gitignore.CompileIgnoreFile(path)` — if compile error, return `nil` + WARN log with error
  - On success: INFO log with `path` + number of lines loaded + return matcher
  - Returns nil if file missing or malformed (defensive — don't kill the watcher)

- [ ] **A2** Extend `fileFilter` struct with `globalIgnore *gitignore.GitIgnore` field (separate from `gitignoreMatcher` which is per-collection).

- [ ] **A3** Update `newFileFilter` signature: `newFileFilter(rootDir string, excludePatterns, allowedExtensions []string, globalIgnore *gitignore.GitIgnore) *fileFilter`. Pass globalIgnore into struct.

- [ ] **A4** Update `shouldSkip()` to check `globalIgnore` BEFORE the per-collection `gitignoreMatcher`:
  ```go
  if f.globalIgnore != nil && f.globalIgnore.MatchesPath(rel) {
      return true
  }
  ```

## Phase B — Wire into server startup

- [ ] **B1** In `cmd/nano-brain/main.go` (where watcher starts, around line 320):
  - Once at startup, before any `WatchWithFilter` call: `globalIgnore := loadGlobalIgnore(homeDir, logger)`
  - Pass `globalIgnore` to `WatchWithFilter` as a new param OR store on `Watcher` struct
  - Lowest-impact path: store on `Watcher` struct + thread to each new `fileFilter` via `newFileFilter` call inside `WatchWithFilter`

- [ ] **B2** Update `Watcher.WatchWithFilter` to receive/use `globalIgnore` (via struct field). All existing call sites continue to work.

## Phase C — Tests

- [ ] **C1** `TestLoadGlobalIgnore_MissingFileReturnsNil` — temp homeDir, no .nano-brainignore, assert nil + no error.

- [ ] **C2** `TestLoadGlobalIgnore_LoadsPatterns` — write `*.png\n!keep.png\n` to temp `.nano-brain/.nano-brainignore`, call loader, assert matcher returns true for `foo.png` and false for `keep.png`.

- [ ] **C3** `TestLoadGlobalIgnore_MalformedFileReturnsNil` — write `[invalid syntax` (force CompileIgnoreFile to fail), assert nil + WARN log.

- [ ] **C4** `TestFileFilter_GlobalIgnoreApplies` — fileFilter with globalIgnore set, no per-collection gitignore, no excludePatterns; shouldSkip returns true for matching files.

- [ ] **C5** `TestFileFilter_GlobalIgnoreCombinesWithPerCollection` — both global + per-collection gitignore set, both contribute to skip decision.

## Phase D — Validate ladder

- [ ] **D1** `validate:quick`: `go build ./... && go test -race -short ./...` → green.

## Phase E — README + PR

- [ ] **E1** Update README.md: add section under "Configuration" titled "Global ignore patterns" documenting:
  - Path: `~/.nano-brain/.nano-brainignore`
  - Format: gitignore syntax (link to spec)
  - Restart required after edits
  - Order: defaults → global → per-collection .gitignore → per-collection excludePatterns

- [ ] **E2** Commit:
  ```
  feat(watcher): support ~/.nano-brain/.nano-brainignore for global ignore patterns (#263)

  Adds a global gitignore-style file that applies patterns across ALL watched
  collections without per-collection config repetition. Complements existing
  defaultExcludeDirs, per-collection .gitignore, and per-collection excludePatterns.

  - loadGlobalIgnore: reads file at startup, returns nil gracefully if missing
  - fileFilter: new globalIgnore field, evaluated before per-collection .gitignore
  - main.go: loads once at startup, threaded into all watchers
  - Tests: 5 cases covering missing, loaded, malformed, combined-with-collection
  ```

- [ ] **E3** Push, PR with `Closes #263`, Gemini triage, squash merge.

- [ ] **E4** Archive openspec, cleanup worktree.
