# Fix Watcher Binary File Indexing

## Issue
[#252 — fix(watcher): skip binary files to prevent UTF-8 encoding errors on upsert](https://github.com/nano-step/nano-brain/issues/252)

## Lane
normal (2 risk flags: existing behavior + weak proof). No hard gates.

## Why
The file watcher reads every file in registered workspaces and passes content to `UpsertDocumentBySourcePath`. The PostgreSQL `documents.content` column is `TEXT` type, which rejects any byte sequence that is not valid UTF-8. When the watcher encounters binary files (PNG, JPEG, PDF, archives, etc.), the upsert fails with `SQLSTATE 22021`:

```
ERR index failed
  error="upsert document: ERROR: invalid byte sequence for encoding \"UTF8\": 0x89 (SQLSTATE 22021)"
  component=watcher
  file=/path/to/file.png
```

Impact:
- **Noise**: Every binary file in a watched directory logs an ERROR on every fsnotify event. For projects with assets (images, PDFs, archives), this floods logs and obscures real errors.
- **Wasted work**: PG round-trip + transaction + rollback per binary file.
- **No data corruption**: PG correctly rejects the insert, so DB state remains clean — but the watcher should never attempt the write.

Root cause: `internal/watcher/watcher.go` has no binary-file detection. The existing filter (`internal/watcher/filter.go`) only checks gitignore + glob exclude patterns + optional allowed-extensions whitelist. None of these catch binary files when no whitelist is configured.

## Desired Outcome
After this change, binary files in watched directories MUST be skipped before any DB write attempt. Two-layer defense:

1. **Extension blacklist (cheap, pre-read)**: Skip files with known binary extensions before calling `os.ReadFile`. Avoids disk read for known-bad cases.
2. **UTF-8 validity check (safety net, post-read)**: After reading content, validate `utf8.Valid(content)`. Catches binaries with text-like or unknown extensions.

Both checks happen in `processFile()` before any DB call. Skipped files emit a single INFO log line; no ERROR logs are produced.

## Constraints
- Backward compatible for text files — no behavior change for `.go`, `.md`, `.ts`, `.yml`, `.sql`, etc.
- No config schema change required for v1 (hardcoded extension list is sufficient).
- No DB migration required.
- Match existing codebase patterns: `processFile()` style, `w.logger.Info()` for skip events, no shared interface for the helper.
- Single-package change — only `internal/watcher/watcher.go` and a new `internal/watcher/binary.go` helper file.

## Out of Scope
- Configurable extension blacklist via YAML (deferred — hardcoded list is fine for v1).
- OCR / PDF text extraction (separate feature, separate proposal).
- Magic-byte detection for unknown extensions (UTF-8 validity check is sufficient and simpler).
- Refactoring `filter.go` to handle binaries (binary detection is content-aware; filter.go is path/glob-aware — different concern).

## Acceptance Criteria
1. **Extension blacklist skips known binaries**: `.png .jpg .jpeg .gif .webp .bmp .ico .tiff .pdf .zip .tar .gz .7z .rar .bz2 .xz .mp4 .mov .avi .mkv .webm .mp3 .wav .flac .ogg .aac .m4a .wasm .exe .dll .so .dylib .o .a .bin .obj .woff .woff2 .ttf .otf .eot .heic .heif .psd .ai .sketch .keychain .db .sqlite .sqlite3` skipped without disk read.
2. **UTF-8 validity check catches unlabeled binaries**: A file with `.txt` extension containing PNG bytes is skipped with WARN log mentioning "non-UTF8 content".
3. **Text files unchanged**: Markdown, Go, TypeScript, YAML, SQL, JSON, TOML, plain text files continue to be indexed exactly as before.
4. **No ERROR logs from PG UTF-8 violations**: Drop a real PNG into a watched workspace → 0 occurrences of `SQLSTATE 22021` in logs.
5. **Unit test for `isBinaryExtension(path)`**: covers known binary extensions + known text extensions + unknown extension (returns false).
6. **Unit test for `isBinaryContent([]byte)`**: covers valid UTF-8, PNG magic bytes, JPEG SOI, mixed bytes with invalid sequences.
7. **Integration test against real PostgreSQL**: drop fixtures (PNG, JPG, valid .md) into a `t.TempDir()` watched directory → assert 1 document created (the .md), 0 `index failed` log occurrences.
8. **Existing watcher tests pass unchanged**: `go test -race -short ./internal/watcher/...` PASS.
9. **Validate ladder**: `validate:quick` + `test:integration` green.
10. **Review Gate**: Gemini bot review triaged per R31; PR ready for merge.

## Risk Flags
- [x] Existing behavior (filters out file types previously attempted but failing) — 1 flag
- [x] Weak proof (no current test for binary file handling in watcher) — 1 flag

2 flags + 0 hard gates → **normal lane** confirmed.

## Why a hardcoded list (not config-driven for v1)
- Operators almost always want the same exclusion list (images, archives, audio/video, binaries)
- A config-driven list adds reload semantics, validation, default fallback, migration concerns
- UTF-8 validity check is the universal safety net — catches everything else
- Follow-up proposal can add `watcher.binary_extensions` config override if needed (low priority)
