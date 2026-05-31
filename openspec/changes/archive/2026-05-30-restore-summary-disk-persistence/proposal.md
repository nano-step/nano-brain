# Restore Summary Disk Persistence (Obsidian-compatible)

## Issue
[#258 — feat(summary): restore disk persistence for Obsidian/vault compatibility (default ON)](https://github.com/nano-step/nano-brain/issues/258)

## Lane
normal (2 risk flags: existing-behavior + weak-proof; 0 hard gates).

## Why
User reports inability to view AI session summaries in Obsidian and other markdown-based tools. Currently summaries persist only to PostgreSQL `documents.content` (since `harvest-summary-only` #192, May 2026). The legacy `output_dir` config key is silently ignored — operator config drift goes undetected.

User quote: "tôi cần file vì có thể support và hiển thị ở các hệ thống khác, ví dụ obsidian"

This is a valid filesystem-export use case that the DB-only architecture cannot serve. The 167 existing summary documents in the DB are inaccessible to Obsidian/ripgrep/fzf workflows.

## Desired Outcome

Summaries are persisted to BOTH layers:

1. **PostgreSQL** (source of truth, source of search) — unchanged behavior
2. **Filesystem** (derivative view) — new, default ON, opt-out via config

Disk layer is a **derivative view of DB**, not an authoritative store. DB always wins on conflict. Disk write failures log WARN but do NOT fail the DB transaction.

## Constraints

- Default ON: new installs write to disk automatically
- Honor existing `summarization.output_dir` config key (currently silently ignored — fixing the UX bug)
- New config key `summarization.write_to_disk: bool` (default `true`) for explicit opt-out
- Match existing nano-brain patterns: on-demand `q := sqlc.New(db)`, plain `func(args []string) error` for CLI
- DB stays source of truth — disk write failure must not roll back DB transaction
- Atomic writes (`.tmp` + `os.Rename`) — no partial files on crash
- File content === `documents.content` byte-for-byte (no transformation)
- No YAML frontmatter (per user choice — pure markdown body)
- Tilde expansion in `output_dir` (e.g. `~/.nano-brain/summaries` → absolute path)

## Out of Scope

- YAML frontmatter generation (deferred — user explicitly declined)
- Watch mode auto-sync (use cron or systemd timer if needed)
- File deletion when workspace removed (separate concern — orphan cleanup later)
- Custom filename template via config (hardcoded format for v1)
- Re-add `summarization.output_dir` warning when `write_to_disk: false` (low priority; can be lint follow-up)

## Acceptance Criteria

1. **Default ON**: Fresh install with default config writes summaries to `~/.nano-brain/summaries/`. Verifiable via integration test that ENV does NOT specify `write_to_disk`.

2. **Opt-out works**: `summarization.write_to_disk: false` → zero file system writes. DB persist unaffected.

3. **`output_dir` honored**: Operator-set `output_dir` is the target directory. Currently ignored — this fixes the silent-ignore bug.

4. **Tilde expansion**: `~/.nano-brain/summaries` → `/home/$USER/.nano-brain/summaries` (or `/Users/$USER/...` on macOS) at config load time.

5. **Path structure exact**: File written to `<output_dir>/<workspace_name>/<source>_<slug-title>_<YYYY-MM-DD>.md`.

6. **Workspace name resolution**: Pull from `workspaces.name`. If empty, fall back to first 12 chars of `workspace_hash` + a leading `ws-`.

7. **Slugify deterministic + safe**:
   - Lowercase
   - Replace non-alphanumeric with `-`
   - Collapse multiple `-` into single `-`
   - Trim leading/trailing `-`
   - Max 80 chars (truncate at word boundary if possible)
   - Empty title → `untitled-session`

8. **Atomic write**: Write content to `<final-path>.tmp`, then `os.Rename(tmp, final)`. Never leave partial files on disk if process crashes mid-write.

9. **Idempotent**: Same session_id + same date + same title produces same path. Re-write overwrites identically (no duplicate files).

10. **Collision-safe**: If path exists with DIFFERENT content (different session_id), append `_<sha8-of-session-id>` to filename. E.g. `opencode_foo_2026-05-30_a1b2c3d4.md`.

11. **DB-first ordering**: DB transaction commits FIRST. Only after `tx.Commit()` succeeds does disk write attempt. Disk failure → WARN log, NO DB rollback.

12. **Content fidelity**: File content === DB `documents.content` byte-for-byte. No frontmatter added, no transformation.

13. **Permission errors**: If `output_dir` not writable (perms, full disk, read-only mount), log WARN with specific error, mark disk-write as failed, continue serving DB-only.

14. **Backfill CLI**: `nano-brain backfill-summaries [--output-dir=<path>] [--workspace=<name|hash>] [--since=<YYYY-MM-DD>] [--dry-run]` exports existing DB summaries to disk using same path/slug logic. Pre-flight server-running check (similar to cleanup-orphan-workspaces).

15. **Unit tests**:
    - Slugify: 15+ cases (special chars, unicode, length limit, empty, vietnamese)
    - Path generation: workspace name lookup, fallback, tilde expansion
    - Atomic write: failure mid-write leaves no partial file
    - Idempotency: same input → same path → overwrite OK
    - Collision: different session same title same date → sha8 suffix

16. **Integration tests**:
    - Persister.Save with `write_to_disk: true` → both DB row AND file exist
    - Persister.Save with `write_to_disk: false` → DB row only, no file
    - `output_dir` not writable → DB succeeds, file fails, WARN logged
    - Backfill CLI: 5 DB rows → 5 files in correct paths
    - Re-run backfill (idempotent): same files, no duplicates

17. **No breaking changes**: Operators who explicitly set `write_to_disk: false` see identical behavior to current DB-only. No data migration required.

18. **Validate ladder**: `validate:quick` + `test:integration` + `smoke:e2e` all green.

19. **Review gate**: Gemini PR bot PASS + R31 triage clean.

## Risk Flags

- [x] Existing behavior change (partial revert of #192 — operators upgrading will see new files appear on disk) — 1 flag
- [x] Weak proof (no current tests for disk persistence path) — 1 flag

2 flags + 0 hard gates → **normal lane** confirmed.

## Migration Strategy

**No DB migration required.** Schema unchanged.

**Operator-facing changes:**

1. Default behavior changes: after upgrade, summaries start appearing in `~/.nano-brain/summaries/` (or operator's configured `output_dir`).
2. To preserve old DB-only behavior: add `summarization.write_to_disk: false` to config.
3. To backfill 167 existing summaries: run `nano-brain backfill-summaries`.
4. Release notes will call out this behavior change and provide both opt-out and backfill instructions.

## Why Re-Add Disk Writes (Architecture Justification)

#192 removed disk writes because at the time, the only consumer was nano-brain itself (which reads from PG). The DB-only architecture was correct for the system-internal use case.

#258 adds a NEW consumer: external filesystem-based tools (Obsidian, ripgrep, fzf, grep, git, file editors). These cannot connect to PostgreSQL. The disk layer is **not duplicate storage** — it is a **format adapter** that translates DB rows into the filesystem-native format expected by external tools.

Reverting to dual-write is correct because:
- DB remains source of truth (no architectural drift)
- Disk is read-only derivative (no inconsistency risk — DB is authoritative)
- Operator opt-out preserves DB-only for users who don't need disk
- Failure mode is graceful (disk failure ≠ DB failure)
