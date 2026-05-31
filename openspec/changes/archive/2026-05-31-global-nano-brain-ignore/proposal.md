# Global .nano-brainignore Support

## Issue
[#263 — feat(config): global ignore patterns for files/folders/extensions](https://github.com/nano-step/nano-brain/issues/263)

## Lane
normal (2 risk flags: existing-behavior + weak-proof).

## Why
Issue #263 asks for global ignore patterns to reduce per-collection config repetition. Investigation shows MOST of #263 is already implemented:

| Capability | Status |
|---|---|
| Global `watcher.exclude_patterns` (applies to all collections) | ✅ Exists (`config.go:100`, merged at `main.go:320`) |
| Global `watcher.allowed_extensions` (applies to all collections) | ✅ Exists |
| Default excludes (`node_modules/`, `.git/`, `dist/`, etc.) | ✅ Exists (`filter.go` `defaultExcludeDirs` — 21 entries baked in) |
| Per-collection overrides | ✅ Exists |
| `.gitignore` per-workspace respect | ✅ Exists |
| **`.nano-brainignore` global file at `~/.nano-brain/.nano-brainignore`** | ❌ MISSING |
| Binary file detection | ✅ Exists (issue #252 binary extension blacklist + UTF-8 check) |

The single missing capability is the **`.nano-brainignore` global file** — an operator-managed gitignore-style file at `~/.nano-brain/.nano-brainignore` whose patterns apply to ALL collections (loaded once at server start). This is the cleanest UX for "I want to ignore X across all my workspaces" without editing config.yml.

## Desired Outcome
At server startup, the watcher loads `~/.nano-brain/.nano-brainignore` if it exists. Its patterns are gitignore-syntax (one per line, supports `**`, `!negation`, blank lines, `#` comments). The patterns merge with the existing per-collection `.gitignore` and `excludePatterns`.

Order of evaluation (existing + new):
1. `defaultExcludeDirs` (hardcoded, 21 entries) — most aggressive, blocks dir descent
2. **NEW: `.nano-brainignore` (global gitignore from `~/.nano-brain/`)** — applies to all collections
3. Per-collection `.gitignore` (in collection root) — existing
4. Per-collection `excludePatterns` (config-level) — existing
5. `allowedExtensions` (whitelist after all exclusions) — existing

## Constraints
- File location: `~/.nano-brain/.nano-brainignore` (use `os.UserHomeDir()` not hardcoded `/Users/...`)
- File is OPTIONAL — server starts normally if missing (single info log: "no .nano-brainignore found, skipping")
- Use existing `github.com/sabhiram/go-gitignore` library (already imported by `filter.go`)
- Path resolution: patterns in `.nano-brainignore` match against paths RELATIVE to the collection root (not absolute), same as `.gitignore`. This works because each watched collection gets a fresh `fileFilter` with its own `rootDir`.
- Compile the global ignore ONCE at startup (not per-collection), share immutable matcher across all `fileFilter` instances
- No config schema changes — purely additive at filesystem level
- Hot-reload: changes to `.nano-brainignore` require server restart (defer auto-reload to follow-up)

## Out of Scope
- Auto-reload on `.nano-brainignore` changes (operator must restart)
- Per-workspace `.nano-brainignore` (only global one; existing per-collection `.gitignore` covers per-workspace needs)
- New CLI to edit/test the ignore file
- Migrating `defaultExcludeDirs` to ship as a default `.nano-brainignore` (keep hardcoded for now — operator override possible via patterns like `!node_modules` in their `.nano-brainignore`)

## Acceptance Criteria
1. **Loads `.nano-brainignore` at startup**: When `~/.nano-brain/.nano-brainignore` exists, the watcher loads it via `gitignore.CompileIgnoreFile`. INFO log shows the path + pattern count.
2. **Skips file gracefully when missing**: When the file does not exist, server starts normally with a DEBUG log (no warning, no error).
3. **Patterns apply to all collections**: A pattern `*.png` in the global ignore file causes PNG files to be skipped in ALL registered collections, without per-collection config.
4. **Negation works**: `!important.png` in the global file un-ignores that specific file (gitignore standard).
5. **Existing per-collection rules still merge**: A collection's own `.gitignore` + `excludePatterns` continue to be applied IN ADDITION to the global file.
6. **No regression**: existing tests pass unchanged. `validate:quick` green.
7. **Tests**:
   - Unit test for `globalIgnoreLoader` (success / missing / malformed file)
   - Filter test: combined global + per-collection rules behave correctly
   - Integration test: drop a `.nano-brainignore` in test home dir, watch a temp collection, assert matching files skipped
8. **README updated**: documents the new file location, gitignore syntax support, restart requirement.

## Risk Flags
- [x] Existing behavior (file watcher behavior change for operators who unknowingly have a stale `.nano-brainignore`)
- [x] Weak proof (no current test for global ignore file)

2 flags + 0 hard gates → **normal lane** confirmed.
