# Tasks: reindex-cfg-ignores

## 1. Export watcher filter for reuse

- [ ] 1.1 Export `fileFilter` struct by renaming to `FileFilter` (capital F)
- [ ] 1.2 Export `newFileFilter` by renaming to `NewFileFilter` (constructor)
- [ ] 1.3 Export `gitignoreStack` struct and its methods
- [ ] 1.4 Keep `defaultExcludeDirs` and `defaultExcludeFiles` unexported — `FileFilter` encapsulates them

## 2. Fix watcher to load nested `.nano-brainignore` during walk

Note: `fullWalkAndExtract` in `reindex_cfg.go` already loads nested `.nano-brainignore` (lines 249-254). This task aligns the **watcher's** `scanCollection` to match that behavior.

- [ ] 2.1 In `scanCollection` (watcher.go:437-444), after loading nested `.gitignore`, also check for `.nano-brainignore` and push onto stack
- [ ] 2.2 Add test: nested `.nano-brainignore` in subdirectory is respected during watcher walk

## 3. Update reindex_cfg.go to use watcher filter

- [ ] 3.1 Import `github.com/nano-brain/nano-brain/internal/watcher` package
- [ ] 3.2 Remove local `defaultExcludeDirs`, `defaultExcludeFiles`, `gitignoreStack`, `shouldSkip` definitions (~90 lines)
- [ ] 3.3 In `fullWalkAndExtract`, call `watcher.LoadGlobalIgnore(homeDir)` to get global ignore
- [ ] 3.4 Call `watcher.NewFileFilter(codeRoot, nil, nil, globalIgnore)` to create filter (no allowedExtensions — keep jsTSExts in walk body)
- [ ] 3.5 Use `filter.ShouldSkip(path, d.IsDir())` instead of local `shouldSkip`
- [ ] 3.6 Use exported `gitignoreStack` for nested `.gitignore`/`.nano-brainignore` during walk
- [ ] 3.7 Fix `filepath.Rel(".", path)` → `filepath.Rel(codeRoot, path)` bug in `ShouldSkip`

## 4. Add filter check to incrementalExtract

- [ ] 4.1 In `incrementalExtract`, create `FileFilter` using `watcher.NewFileFilter`
- [ ] 4.2 Before processing each document, call `filter.ShouldSkip(doc.SourcePath, false)` — skip if true
- [ ] 4.3 Log skipped documents at DEBUG level
- [ ] 4.4 Add test: now-ignored file in DB is skipped during incremental reindex

## 5. Update progress logging

- [ ] 5.1 In `fullWalkAndExtract`, add `skipped` counter for paths skipped by ignore rules
- [ ] 5.2 Update 10-second progress log to include `skipped` count alongside `files_processed` and `cfgs_extracted`

## 6. Verify and test

- [ ] 6.1 Build: `go build ./...`
- [ ] 6.2 Test: `go test -race -short ./internal/server/handlers/...`
- [ ] 6.3 Test: `go test -race -short ./internal/watcher/...`
- [ ] 6.4 Manual test: run full reindex on express-app, verify `docker-data` and other ignored dirs are skipped
- [ ] 6.5 Manual test: run incremental reindex, verify now-ignored files are skipped
