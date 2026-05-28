## AC-1: Recursive scan finds subdirectory files

**Given** a collection with path `/workspace/project` and glob `**/*`
**When** `scanCollection` runs
**Then** files in `/workspace/project/internal/server/handlers/health.go` are discovered
**And** files at all nesting depths are indexed

## AC-2: .git and node_modules are skipped efficiently

**Given** a collection path containing `.git/` and `node_modules/`
**When** `scanCollection` runs
**Then** the filter prunes those directories entirely (SkipDir)
**And** no files inside them are processed

## AC-3: memory/sessions paths are real home-relative paths

**Given** `initWorkspace` is called
**When** it stores the memory and sessions collections in the DB
**Then** `path` = `/Users/<user>/.nano-brain/memory` (not `~/.nano-brain/memory/`)
**And** `filepath.Abs(path)` returns the same path unchanged

## AC-4: watcher attaches to correct memory/sessions paths

**Given** server starts with a registered workspace
**When** watcher loads collections from DB
**Then** `WatchWithFilter` is called with real absolute paths
**And** no "path not found, skipping watch" warnings for memory/sessions

## AC-5: document_count increases after init

**Given** a workspace is registered via `init --root <path> --force`
**When** the watcher performs its initial `processAll`
**Then** `GET /api/v1/workspaces` shows `document_count > 0` within poll interval
