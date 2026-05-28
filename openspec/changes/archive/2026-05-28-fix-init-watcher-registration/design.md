## Pattern Reference

`AddCollection` handler (collection.go:73) already does exactly this — it's the established pattern:

```go
func AddCollection(q CollectionQuerier, fw *watcher.Watcher, watcherCfg config.WatcherConfig, logger zerolog.Logger) echo.HandlerFunc {
    // ... after DB upsert:
    cfgExclude, cfgExtensions := watcherCfg.ResolveFilterForPath(col.Path)
    if err := fw.WatchWithFilter(col.Name, col.Path, col.WorkspaceHash, col.GlobPattern, excludePatterns, allowedExtensions); err != nil {
        logger.Warn().Err(err).Str("name", col.Name).Msg("failed to attach watcher")
    }
}
```

## InitWorkspace Fix

After the transaction commits (both `db != nil` and `db == nil` branches), fetch the created collections and register each with the watcher.

The `initWorkspace` function creates 3 default collections: `memory`, `sessions`, `code`. All 3 should be registered.

Since `initWorkspace` doesn't return the collections, options:
1. **Re-query after commit** — call `queries.ListCollections(ctx, ws.Hash)` post-commit
2. **Register inline** — call `fw.WatchWithFilter` for the 3 known paths directly

Option 2 is simpler and avoids an extra DB round-trip. The paths are deterministic (`memoryPath`, `sessionsPath`, `absPath`).

```go
// After transaction commit, register collections with live watcher
if fw != nil {
    type colSpec struct{ name, path, glob string }
    cols := []colSpec{
        {"memory",   memoryPath,   "**/*"},
        {"sessions", sessionsPath, "**/*"},
        {"code",     absPath,      "**/*"},
    }
    cfgExclude, cfgExtensions := watcherCfg.ResolveFilterForPath(absPath)
    for _, col := range cols {
        if err := fw.WatchWithFilter(col.name, col.path, hash, col.glob, cfgExclude, cfgExtensions); err != nil {
            logger.Warn().Err(err).Str("collection", col.name).Msg("failed to attach watcher after init")
        }
    }
}
```

`fw == nil` guard makes tests safe without a real watcher.

## routes.go Change

```go
// Before:
api.POST("/init", handlers.InitWorkspace(s.queries, s.db, s.logger))

// After:
api.POST("/init", handlers.InitWorkspace(s.queries, s.db, s.watcher, s.currentConfig().Watcher, s.logger))
```

## Test Files

Both `workspace_test.go` and `workspace_integration_test.go` call `InitWorkspace(q, nil, logger)` — update to `InitWorkspace(q, nil, nil, config.WatcherConfig{}, logger)`.
