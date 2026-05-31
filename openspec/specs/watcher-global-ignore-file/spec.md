# watcher-global-ignore-file Specification

## Purpose
TBD - created by archiving change global-nano-brain-ignore. Update Purpose after archive.
## Requirements
### Requirement: Watcher loads global ignore file at startup

The watcher SHALL look for a file at `<user-home>/.nano-brain/.nano-brainignore` at server startup. The path is resolved via `os.UserHomeDir()` (not hardcoded). If the file exists and is parseable as gitignore syntax (per the `github.com/sabhiram/go-gitignore` library), the watcher SHALL compile it once and share the matcher across all watched collections.

If the file is missing, the watcher SHALL start normally with a DEBUG-level log entry and no global ignore matcher (existing per-collection behavior unchanged). If the file exists but is malformed, the watcher SHALL log a WARN with the parse error and continue with no global ignore matcher (defensive — server must not fail to start because of one bad ignore file).

#### Scenario: File exists with valid patterns

- **GIVEN** `~/.nano-brain/.nano-brainignore` contains:
  ```
  *.png
  *.jpg
  !important.png
  build/
  ```
- **WHEN** the server starts
- **THEN** the watcher emits an INFO log `loaded global ignore file: <path> (4 patterns)`
- **AND** the matcher is shared across all subsequently-created `fileFilter` instances

#### Scenario: File does not exist

- **GIVEN** `~/.nano-brain/.nano-brainignore` does not exist
- **WHEN** the server starts
- **THEN** the watcher emits a DEBUG log `.nano-brainignore not found, skipping`
- **AND** all collections operate exactly as before this feature

#### Scenario: File is malformed

- **GIVEN** `~/.nano-brain/.nano-brainignore` contains content that `gitignore.CompileIgnoreFile` rejects
- **WHEN** the server starts
- **THEN** the watcher emits a WARN log with the parse error
- **AND** the server starts normally (no global matcher, but all other functionality works)

### Requirement: Global patterns apply to all collections

When a global `.nano-brainignore` matcher is present, every `fileFilter.shouldSkip` decision SHALL evaluate the global matcher against the path RELATIVE to the collection root (same path representation used for the per-collection `.gitignore` matcher). The global matcher is checked AFTER `defaultExcludeDirs` (which short-circuits directory descent) but BEFORE the per-collection `.gitignore` and `excludePatterns`.

This ordering preserves existing per-collection behavior (which can still override via specific paths or negation in per-collection rules).

#### Scenario: Global *.png pattern applies to all workspaces

- **GIVEN** the global file contains `*.png`
- **AND** workspace A contains `screenshot.png`, workspace B contains `icon.png`
- **WHEN** the watcher processes file events for both workspaces
- **THEN** `screenshot.png` and `icon.png` are both skipped
- **AND** the watcher emits one DEBUG `skipping binary file (extension)` for each (or `skipping per global ignore` depending on which check fires first)

#### Scenario: Per-collection rules still apply on top of global

- **GIVEN** the global file contains `*.log`
- **AND** workspace A has per-collection `excludePatterns: ["temp/**"]`
- **WHEN** the watcher processes `workspace-A/app.log` and `workspace-A/temp/cache.json`
- **THEN** `app.log` is skipped due to the global rule
- **AND** `temp/cache.json` is skipped due to the per-collection rule

