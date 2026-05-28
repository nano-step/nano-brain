## Fix 1: Replace filepath.Glob with fs.WalkDir

### Current (broken)
```go
func (w *Watcher) scanCollection(ctx context.Context, col watchedCollection) {
    pattern := filepath.Join(col.dirPath, col.globPattern)
    matches, err := filepath.Glob(pattern)  // ← only 1 level deep
    if err != nil { ... }
    for _, filePath := range matches {
        if col.filter != nil && col.filter.shouldSkip(filePath) { continue }
        w.processFile(ctx, col, filePath)
    }
}
```

### Fixed
```go
func (w *Watcher) scanCollection(ctx context.Context, col watchedCollection) {
    err := filepath.WalkDir(col.dirPath, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            w.logger.Warn().Err(err).Str("path", path).Msg("walk error, skipping")
            return nil
        }
        if ctx.Err() != nil {
            return ctx.Err()
        }
        if col.filter != nil && col.filter.shouldSkip(path) {
            if d.IsDir() {
                return filepath.SkipDir  // prune entire subtree (e.g. .git, node_modules)
            }
            return nil
        }
        if !d.IsDir() {
            w.processFile(ctx, col, path)
        }
        return nil
    })
    if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
        w.logger.Error().Err(err).Str("dir", col.dirPath).Msg("walk failed")
    }
}
```

**Key properties:**
- `filepath.SkipDir` when dir is excluded → prunes `.git`, `node_modules`, etc. efficiently
- Context cancellation propagates cleanly
- No `filepath.Glob` pattern needed — WalkDir always recurses fully
- `globPattern` field becomes unused for scanning (keep in DB schema for future use)

**Imports to add:** `"errors"`, `"io/fs"` (both stdlib)

---

## Fix 2: Expand tilde in initWorkspace

### Current (broken)
```go
memoryPath := "~/.nano-brain/memory/"
sessionsPath := "~/.nano-brain/sessions/"
```

### Fixed
```go
home, err := os.UserHomeDir()
if err != nil {
    return sqlc.Workspace{}, fmt.Errorf("failed to get home directory: %w", err)
}
memoryPath := filepath.Join(home, ".nano-brain", "memory")
sessionsPath := filepath.Join(home, ".nano-brain", "sessions")
```

**Imports to add:** `"os"` (already imported in package? check), `"path/filepath"` (already present)

This also fixes `WatchWithFilter` calls in `InitWorkspace` handler — they read from the DB or from local vars, both of which are now expanded.

---

## Files Touched

| File | Change |
|---|---|
| `internal/watcher/watcher.go` | `scanCollection`: filepath.Glob → fs.WalkDir; add `errors`, `io/fs` imports |
| `internal/server/handlers/workspace.go` | `initWorkspace`: expand tilde for memoryPath/sessionsPath; add `os` import if missing |
| Tests | Minimal — mock querier paths don't go through watcher scan |
