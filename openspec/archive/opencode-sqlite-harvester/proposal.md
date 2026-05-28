# OpenCode SQLite Harvester — Proposal

**Status**: proposed  
**Lane**: normal  
**GitHub Issue**: #176  
**Date**: 2026-05-25  

## Problem

OpenCode migrated from JSONL files to SQLite (`~/.local/share/opencode/opencode.db`). The current `OpenCodeHarvester` reads from a JSONL/JSON directory structure that no longer exists. Users with 6,744+ sessions cannot harvest them.

## Proposed Solution

Replace the file-based OpenCode harvester with a SQLite-based implementation. Use `modernc.org/sqlite` (already in go.mod) to read session/message/part tables. Produce the same markdown output format as the existing harvester for consistency.

## Key Details

- SQLite schema: `session`, `message`, `part`, `project`, `todo` tables
- Config: add `harvester.opencode.db_path` (default: `~/.local/share/opencode/opencode.db`)
- Keep `session_dir` config as fallback for users on old JSONL format
- Incremental: track last-harvested session by `updated_at` timestamp
- Same `Harvester` interface (`HarvestAll`) — drop-in replacement
- Output format: same markdown frontmatter + role/content sections

## Success Criteria

1. `CGO_ENABLED=0 go build ./...` passes
2. `CGO_ENABLED=0 go test -short ./...` passes
3. Unit tests with in-memory SQLite DB pass for session harvesting
4. Config `harvester.opencode.db_path` is read and used when set
5. When `db_path` is empty and `session_dir` is set, old behavior preserved (no regression)
