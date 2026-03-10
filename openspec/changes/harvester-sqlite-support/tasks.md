# Tasks: Harvester SQLite Support

## T1: Extract JSON harvester into `harvestFromJson()`

Refactor: move the existing JSON file processing loop from `harvestSessions()` into a separate `harvestFromJson()` function. `harvestSessions()` becomes the dispatcher.

**Files:** `src/harvester.ts`

## T2: Implement `harvestFromDb()`

New function that reads from `opencode.db`:
- Open DB read-only with better-sqlite3
- Query sessions, messages, parts
- Apply same skip/state logic (keyed by session ID)
- Same markdown output format
- Same incremental append support
- Track counters (processed, skipped, incremental, errors)
- Close DB after harvest

**Files:** `src/harvester.ts`

## T3: Add DB-first dispatch in `harvestSessions()`

- Derive DB path from sessionDir: `join(dirname(sessionDir), 'opencode.db')`
- Check if DB exists and has sessions
- If yes → call `harvestFromDb()`
- If no → call `harvestFromJson()`
- Merge results, timing, and logging

**Files:** `src/harvester.ts`

## T4: Verify and publish

- Run lsp_diagnostics on harvester.ts
- Bump version to 2026.4.8
- npm publish

**Files:** `src/harvester.ts`, `package.json`
