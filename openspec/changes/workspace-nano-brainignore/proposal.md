# Workspace-local `.nano-brainignore`

## Issue
[#317 — Workspace-local .nano-brainignore support (watcher per-collection ignore file)](https://github.com/nano-step/nano-brain/issues/317)

## Lane
normal — user-feature change type. Harness rules (`docs/HARNESS.md`) force escalation from `tiny` for any user-visible behavior change, regardless of diff size. No hard gates triggered (no auth, data-model, search-quality, embedding-provider, public-api-contract, audit, authorization, or external-provider).

## Why
Today only ONE ignore file affects the watcher: `~/.nano-brain/.nano-brainignore` (issue #263), shared globally across every registered collection. This is one-size-fits-all and lives on the operator's machine — not shareable with a team and not aware of project specifics.

Per-project ignore needs are real:

- A docs repo wants `*.md` indexed; a code repo wants `*.lock`, `*.snap`, large fixtures skipped.
- Generated files (`*.generated.go`, fixtures committed to git for build-correctness) should be skipped by nano-brain but kept tracked by git — so `.gitignore` is not the right place.
- Teams want ignore rules that travel with the repo via version control.

The `.gitignore` per-collection mechanism (auto-loaded at `internal/watcher/filter.go:60-65`) already proves the infrastructure works. This change adds a sibling file using the **same library** (`github.com/sabhiram/go-gitignore`) and the **same loading pattern**.

## Desired Outcome
A user can place a `.nano-brainignore` file at the root of a registered workspace and have its patterns honored by the watcher — additively with the global file and `.gitignore` — without touching server config, restarting beyond the normal "load on collection registration" path, or running any CLI command.

## Constraints
- Backward compatible: existing collections without a `.nano-brainignore` file behave identically.
- No new external dependencies. Reuse `github.com/sabhiram/go-gitignore`.
- No new config fields. File is discovered by convention.
- No regression in watcher hot path (`shouldSkip` adds one nil-checked `MatchesPath` call — same cost as the existing global matcher).
- Must NOT modify the existing `LoadGlobalIgnore` function or its call site in `cmd/nano-brain/main.go`.

## Out of Scope (deferred to follow-up issues)
- **Hot-reload on file change** — fsnotify watch on the ignore file itself. v1 matches the existing global-ignore behavior: load once at collection registration. To pick up changes, restart the server OR re-register the workspace via `POST /api/v1/init`.
- **Cross-file negation semantics** — a `!pattern` in workspace-local cannot un-exclude a path matched by global. Each ignore file evaluates independently; `shouldSkip` is short-circuit OR.
- **Tombstone / reconciliation** — if the user adds the file and restarts, already-indexed documents that now match new patterns remain in PostgreSQL. Same behavior as the existing TODO at `internal/watcher/watcher.go:299-302` for deleted-file cleanup. Documented limitation, separate feature.
- **Per-subdirectory hierarchical ignore files** (git-style) — out of scope, large complexity increase, no signal from users.

## Acceptance Criteria
1. **File honored**: A workspace with `.nano-brainignore` containing `*.tmp` causes any `*.tmp` file under that workspace root to be skipped on next reindex/walk.
2. **File missing = no-op**: Collections without the file behave exactly as before this change. No log noise, no allocations.
3. **Compose with global**: When both `~/.nano-brain/.nano-brainignore` (with `*.log`) and `<root>/.nano-brainignore` (with `*.tmp`) exist, both pattern sets apply independently.
4. **Compose with `.gitignore`**: Workspace-local `.nano-brainignore` and the existing per-collection `.gitignore` apply additively in `shouldSkip`.
5. **Unreadable file**: If `.nano-brainignore` exists but `os.ReadFile` fails (permission denied, path is a directory, IO error), the watcher logs WARN with the IO error and continues with a nil local matcher (collection works as if the file were absent). Server MUST NOT fail to start. Note: the `github.com/sabhiram/go-gitignore` library tolerates any pattern content — there is no "malformed content" error mode.
6. **Successful load is observable**: On DEBUG log level, a successful load emits one log line with `dir` and `path` fields. No log when the file is absent (avoids noise — many collections won't have one).
7. **Precedence documented**: README "Ignore patterns" section reflects the new layer between global and `.gitignore`.
8. **Re-registration picks up changes**: Calling `POST /api/v1/init` again with the same `root_path` rebuilds the `fileFilter` and picks up an updated `.nano-brainignore` without server restart.
