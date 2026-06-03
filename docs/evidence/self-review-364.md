# Self-Review: Issue #364 / PR #366 — watcher log noise

**Change Type:** bug-fix
**Story:** #364 (watcher continuously logs 'processing file collection=code' on idle workspaces)
**Lane:** tiny (single-file, single-line log severity change)
**Date:** 2026-06-03
**Reviewer (Self):** Sisyphus (implementing agent — independent review delegated separately per R: no self-review)

---

## Summary

Demoted `processing file` log emission from `Info` to `Debug` level in `internal/watcher/watcher.go:394`. No behavior change — only log severity.

## Acceptance Criteria (from issue #364)

- [x] **Root cause confirmed** — Hypothesis #1 from issue: periodic reindex sweep (`pollTicker` at `watcher.reindex_interval`, default 300s) walks every file → `processFile` → emitted INFO log per file BEFORE the content-hash dedup check at line 426. For N indexed files, N spurious INFO lines per sweep on idle workspaces.
- [x] **Spurious log lines stopped** — emission demoted to DEBUG level. Operators wanting per-file tracing can flip `logging.level: debug`.
- [x] **No real work avoided unnecessarily** — file I/O is required to compute SHA-256 dedup hash; dedup short-circuit at line 426 was already correct. Only log noise was the bug.

## Diff

```diff
- w.logger.Info().
+ w.logger.Debug().
   Str("path", filePath).
   Str("collection", col.name).
   Msg("processing file")
```

Single line, single file: `internal/watcher/watcher.go:394`.

## Why this is correct

- The companion `indexed file` INFO log at line 464 already only fires when real indexing happens (after SHA-256 dedup short-circuits no-ops at line 426). That remains the meaningful per-file signal at INFO level.
- Per-file pre-dedup chatter is exactly what DEBUG is for. Matches the existing `skipping binary file (extension)` DEBUG log in the same function (line 380).
- No behavior change — the file is still walked, stat'd, read, hashed, and indexed if changed.

## Validation

| Check | Result |
|---|---|
| `go build ./...` | ✅ clean |
| `go test -race -short ./...` | ✅ all packages pass |
| `go test -race -short ./internal/watcher/` | ✅ pass (cached, force-rerun also pass) |
| `golangci-lint run ./internal/watcher/...` (vs master) | ✅ clean — no new issues |
| Surgical change check | ✅ only 1 line modified, no orphans, no adjacent edits |

## Risk audit (per `docs/FEATURE_INTAKE.md`)

Risk flags this change carries:

- ✅ **Existing behavior**: log severity change only — additive (operators can recover detail via `logging.level: debug`)
- ✅ **External systems**: none touched
- ✅ **Data model**: none touched
- ✅ **Public API contract**: none — log lines are operator-facing, not API
- ✅ **Auth/security**: none touched
- ✅ **Search quality**: none touched
- ✅ **Embedding/vector provider**: none touched

**Total flags: 0** — confirmed **tiny lane**.

## Scope discipline

Touched files: `internal/watcher/watcher.go` (1 line). No adjacent edits, no refactors, no test changes needed (no behavior change to test).
