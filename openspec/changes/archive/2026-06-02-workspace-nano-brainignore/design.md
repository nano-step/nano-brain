# Design — Workspace-local `.nano-brainignore`

## Context

The watcher's `fileFilter` (`internal/watcher/filter.go`) already implements three layered ignore mechanisms:

1. **Default exclude dirs** — hardcoded set (`node_modules`, `.git`, `dist`, etc.) at `filter.go:11-33`.
2. **Global `.nano-brainignore`** — loaded once at startup by `LoadGlobalIgnore(homeDir)` and injected per collection via `fw.SetGlobalIgnore(gi)` (`cmd/nano-brain/main.go:321-330`).
3. **Per-collection `.gitignore`** — auto-discovered inside `newFileFilter` at construction time (`filter.go:60-65`).

The architecture is sound. Adding workspace-local `.nano-brainignore` follows mechanism 3 exactly — auto-discover at the same construction point, using the same library, with the same error tolerance.

## Architecture

### Field shape

Add ONE new field to `fileFilter`:

```go
type fileFilter struct {
    gitignoreMatcher  *gitignore.GitIgnore
    globalIgnore      *gitignore.GitIgnore
    localIgnore       *gitignore.GitIgnore  // NEW — <rootDir>/.nano-brainignore
    excludePatterns   []string
    allowedExtensions map[string]bool
    rootDir           string
}
```

Why a separate field (not unioned into `globalIgnore`):
- Clear provenance for debugging and logs.
- Matches the existing per-collection vs global separation (`gitignoreMatcher` vs `globalIgnore`).
- Composite matchers would require allocating a merged pattern list per collection — no benefit for v1 scope.

### Loading site

Inline inside `newFileFilter`, mirroring the existing `.gitignore` block at `filter.go:60-65`. Reason: `.gitignore` proves the per-collection inline pattern is the right shape. No new public `LoadLocalNanoIgnore(rootDir)` function — there is no caller for it.

### Constructor signature change

`newFileFilter` returns `(*fileFilter, error)` instead of `*fileFilter`.

**Why a change is required**: To log WARN on a malformed local ignore file we need a logger. `newFileFilter` has no logger. The caller (`(*Watcher).WatchWithFilter` at `watcher.go:154`) has `w.logger`. The cleanest fix is to return the parse error and let the caller log + decide to continue.

This is consistent with the existing global-ignore precedent: `LoadGlobalIgnore` returns `(*gitignore.GitIgnore, string, error)` so `cmd/nano-brain/main.go:321-330` can log.

**Call sites updated** — exactly one:

| File | Line | Change |
|---|---|---|
| `internal/watcher/watcher.go` | ~154 | Handle new `error` return from `newFileFilter`. Log Debug on success-with-load, Warn on parse error, continue regardless (nil matcher = same as missing file). |

A grep for `newFileFilter(` returns exactly one production call site (this one) plus test files. The signature change is mechanically safe.

### Error handling matrix

**Note on library behavior**: `github.com/sabhiram/go-gitignore`'s `CompileIgnoreFile` calls `os.ReadFile` then `CompileIgnoreLines`. `CompileIgnoreLines` **never returns an error** — it tolerates any pattern content (including binary garbage). So `CompileIgnoreFile` only returns errors from `os.ReadFile` (permission denied, path-is-a-directory, IO failure). There is no "malformed content" failure mode at the library level.

| File state | Library outcome | Behavior |
|---|---|---|
| File absent | `os.Stat` returns `IsNotExist` | Silent, nil matcher. No log. |
| File present, readable | `CompileIgnoreFile` returns matcher | DEBUG log `loaded workspace .nano-brainignore` with `dir` + `path`. Matcher used. |
| File present, unreadable (chmod 0000, is-a-directory, IO error) | `os.ReadFile` returns error → `CompileIgnoreFile` propagates it | `newFileFilter` returns the error. Caller logs WARN with `err` + `path`. Nil matcher. Collection works as if file were absent. |
| File present, "garbage" content (e.g. random bytes) | `CompileIgnoreLines` silently compiles (no error) — matches nothing useful | Treated as valid-but-empty. DEBUG log fires; matcher is non-nil but has no effective patterns. This is library behavior, not nano-brain behavior. |

The "no log when absent" choice is deliberate. `WatchWithFilter` is called once per collection (multiple per workspace). At INFO level this would emit 3+ lines per `/api/v1/init` call for nothing useful. The global file logs at INFO because it loads once at startup.

### Precedence in `shouldSkip`

Current order in `filter.go:70-117`:

1. `defaultExcludeDirs` (`filter.go:77-81`)
2. `globalIgnore` (`filter.go:88-90`)
3. `gitignoreMatcher` (`filter.go:92-94`)
4. `excludePatterns` (`filter.go:96-107`)
5. `allowedExtensions` (`filter.go:109-114`)

New order (insertion at position 3):

1. `defaultExcludeDirs`
2. `globalIgnore`
3. **`localIgnore`** ← NEW
4. `gitignoreMatcher`
5. `excludePatterns`
6. `allowedExtensions`

**Semantic note**: Because `shouldSkip` is short-circuit OR (any matcher returning true → skip), the relative order between `globalIgnore`, `localIgnore`, and `gitignoreMatcher` has zero impact on which files are skipped — only on which check "wins" first for logging/debugging purposes. The chosen position reflects intent: nano-brain-specific rules (global, then local) are evaluated before the generic `.gitignore`. This matches how a user thinks about layers.

### Reload semantics (v1)

`newFileFilter` is invoked from `(*Watcher).WatchWithFilter`, which runs on:
- Server startup (`cmd/nano-brain/main.go:~380`)
- Workspace registration (`internal/server/handlers/workspace.go:~170` via `/api/v1/init`)
- Collection add (`internal/server/handlers/collection.go:~140` and `~244`)

`POST /api/reload-config` does NOT rebuild fileFilters (it only updates `searchCfg` + log level — `server.go:263-278`). So a `.nano-brainignore` change is NOT picked up by `reload-config`.

To pick up changes in v1:
- Restart the server, OR
- Re-register the workspace via `POST /api/v1/init` (this overwrites the collection's entry in `collections` map at `watcher.go:147`).

This matches the global ignore behavior exactly. Hot-reload via fsnotify is a follow-up.

### Per-collection rootDir mapping

The watcher creates multiple `fileFilter` instances per workspace, one per collection:

| Collection | rootDir | `.nano-brainignore` typically present? |
|---|---|---|
| `code` | `<workspace_root>` | YES — this is the intended use case |
| `memory` | `~/.nano-brain/memory` | No (but allowed) |
| `sessions` | `~/.nano-brain/sessions` | No (but allowed) |

The filesystem naturally scopes this — no special-casing needed. README must clarify the file is loaded from each collection's rootDir; the typical user case is the `code` collection.

## File-Level Touch Points (MVA)

| File | Lines added | Change summary |
|---|---|---|
| `internal/watcher/filter.go` | ~10 | Add `localIgnore` field; inline-load `.nano-brainignore` in `newFileFilter`; return `(*fileFilter, error)`; add check between `globalIgnore` and `gitignoreMatcher` in `shouldSkip`. |
| `internal/watcher/watcher.go` | ~5 | Update single call site to handle `error` return; log Debug on load, Warn on parse failure. |
| `internal/watcher/filter_test.go` | ~80 | 5 new unit tests (see below). |
| `README.md` | ~20 | Restructure "Global ignore patterns" → "Ignore patterns" with both global and workspace-local subsections; update precedence table. |
| `openspec/specs/watcher-file-filtering/spec.md` | delta | Add ADDED Requirement after archive (handled by archive step, not implementation). |

Total: ~115 LOC across 4 production+test+docs files, no schema/migration changes.

## Tests

### Unit (`internal/watcher/filter_test.go`)

5 new scenarios following the existing `TestFileFilter_GlobalIgnoreApplies` and `TestFileFilter_GlobalIgnoreCombinesWithPerCollection` patterns:

| Test name | What it validates |
|---|---|
| `TestFileFilter_LocalNanoBrainIgnoreApplies` | File exists with pattern `*.tmp`; `shouldSkip("foo.tmp", false)` returns true. |
| `TestFileFilter_LocalNanoBrainIgnoreMissing` | No file present; nil matcher; existing behavior preserved. |
| `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGlobal` | Global has `*.log`, local has `*.tmp`; both apply independently. |
| `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGitignore` | `.gitignore` has `tmp/`, `.nano-brainignore` has `*.snap`; both apply independently. |
| `TestFileFilter_LocalNanoBrainIgnoreUnreadable` | File exists but `os.ReadFile` fails (use chmod 0000 with Linux-only guard, or make `.nano-brainignore` a directory). `newFileFilter` returns the IO error; `localIgnore` nil; other filter layers still operate. Note: `go-gitignore` tolerates any pattern content, so the only way to trigger an error is IO-level. |

### Integration

None required. The change is pure in-memory + filesystem reads, fully exercised by unit tests.

### Smoke (harness `smoke:e2e` — required for `user-feature`)

Documented in `tasks.md` step. Manual sequence:
1. Build binary: `go build -o /tmp/nano-brain ./cmd/nano-brain`
2. Start: `DATABASE_URL=... /tmp/nano-brain` (with a fresh DB)
3. Create `/tmp/smoke317/{foo.go,skip.snap,.nano-brainignore}` where `.nano-brainignore` contains `*.snap`
4. `POST /api/v1/init` with `root_path=/tmp/smoke317`
5. Wait for indexing (or trigger reindex)
6. `POST /api/v1/search` query `skip` → MUST NOT return `skip.snap`
7. `POST /api/v1/search` query `foo` → MUST return `foo.go`

Evidence recorded under `docs/evidence/self-review-feat-317-workspace-nano-brainignore.md`.

## Risks & Mitigations

| Risk | Severity | Mitigation |
|---|---|---|
| Signature change of `newFileFilter` breaks something | LOW | One production call site (`watcher.go:~154`), grep-verified; LSP/`go build` catches any miss. |
| User places `*` and loses entire index | MEDIUM | README warning, same risk as misconfigured `.gitignore`. Out of nano-brain's control. |
| User confused why memory/sessions collections don't honor a file at workspace_root | MEDIUM | README explicitly clarifies the file is loaded per collection root; "code" collection root = workspace_root. |
| Already-indexed files matching new patterns remain in PG (no tombstone) | LOW | Documented as known limitation, matches existing TODO at `watcher.go:299-302`. |
| Logging noise from 3+ collections per workspace | LOW | DEBUG-only on load, no log on absence. Matches existing pattern. |

## Decision Log

| Decision | Considered alternatives | Chosen approach | Rationale |
|---|---|---|---|
| Where to load the file | Public `LoadLocalNanoIgnore` function vs inline in `newFileFilter` | Inline | No external caller; matches `.gitignore` pattern at `filter.go:60-65` |
| Field shape | Union into `globalIgnore` vs separate field | Separate `localIgnore` field | Clear provenance, no merge allocation cost |
| Error handling | Silent skip (like `.gitignore`) vs WARN log | WARN via caller on IO errors only | `.nano-brainignore` is user-authored nano-brain-specific; silent IO failure = confusing. Matches global ignore precedent at `main.go:323`. Library cannot report content errors (see Error handling matrix). |
| Success logging | INFO vs DEBUG vs nothing | DEBUG with `dir`+`path` | Per-collection (3+ per workspace) → INFO too noisy; nothing makes load unverifiable |
| Absence logging | DEBUG vs nothing | Nothing | Most collections won't have one; even DEBUG would be noise |
| Lane | tiny (small diff) vs normal (user-feature) | Normal | HARNESS.md classifies by user-visibility, not LOC. User-feature → forces escalation from tiny. |
