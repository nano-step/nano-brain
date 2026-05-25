# Proposal: Harvester SQLite Support

## Problem

OpenCode migrated from JSON file storage to SQLite (`opencode.db`) around Mar 6, 2026. The harvester only reads JSON files (`storage/session/`, `storage/message/`, `storage/part/`), so all sessions created after the migration are invisible. 25,673+ messages are unharvested.

## Solution

Add SQLite-based harvesting that reads directly from `opencode.db`. DB-first with JSON fallback for environments that haven't migrated yet.

## Approach

- New `harvestFromDb()` function opens `opencode.db` read-only
- Queries `session` → `message` → `part` tables
- Same state tracking (`.harvest-state.json`), same markdown output, same incremental append
- `harvestSessions()` tries DB first; if DB missing/empty, falls back to existing JSON logic
- DB path derived from sessionDir: `{sessionDir}/../opencode.db`
- State key for DB sessions uses session ID directly (no `.json` suffix) — no collision with legacy JSON keys

## Non-goals

- Writing to opencode.db
- Removing JSON harvester code (kept as fallback)
- Changing output format
