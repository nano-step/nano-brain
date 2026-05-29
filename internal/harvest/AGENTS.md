# internal/harvest

Session harvesting for OpenCode and Claude Code sessions.

## Multi-DB Discovery

OpenCode harvesters are selected at startup in priority order:

| Priority | Mode | Config key | Env override |
|----------|------|-----------|--------------|
| 1 | `db_root` | `harvester.opencode.db_root` | `OPENCODE_DB_ROOT` |
| 2 | `db_path` | `harvester.opencode.db_path` | `OPENCODE_DB_PATH` |
| 3 | `session_dir` | `harvester.opencode.session_dir` | `OPENCODE_STORAGE_DIR` |
| — | disabled | (none set) | — |

### db_root mode

`ScanOpenCodeDBRoot(ctx, root, registered, logger)` globs `<root>/*/opencode.db`,
opens each read-only, reads `project.worktree`, normalizes via `filepath.Clean`,
and matches against the registered workspace map (path → hash). One
`OpenCodeSQLiteHarvester` is instantiated per match. Zero matches falls through
to the next priority.

Auto-detected at `~/.ai-sandbox/opencode-dbs` on linux/darwin when not set.

### Env var table

| Variable | Effect |
|----------|--------|
| `OPENCODE_DB_ROOT` | Explicit db_root path (skips auto-detect) |
| `OPENCODE_DB_PATH` | Explicit single SQLite path |
| `OPENCODE_STORAGE_DIR` | Legacy session dir |
