## Context

nano-brain's flow visualization system uses graph extractors to identify HTTP routes in codebases. Currently, all framework-specific extractors (Echo, Gin, net/http, Express) are registered globally and run on every matching file. Each extractor does AST self-filtering — parsing the full file with tree-sitter, then checking for framework-specific patterns. This wastes CPU when the framework isn't present in the workspace.

The watcher (`internal/watcher/watcher.go`) calls `graphRegistry.ExtractEdges(filePath, content)` for every file. The registry iterates all extractors, checks `Supports(ext)`, and runs matching ones. There's no workspace-level awareness of which frameworks are used.

The existing `Extractor` interface is minimal:
```go
type Extractor interface {
    ExtractEdges(filePath string, content []byte) ([]Edge, error)
    Supports(ext string) bool
}
```

## Goals / Non-Goals

**Goals:**
- Detect which web frameworks are present in a workspace by reading manifest files (go.mod, package.json)
- Skip graph extractors whose framework isn't detected, reducing wasted AST parsing
- Re-detect when manifest files change (go.mod/package.json modified)
- Fail open: if detection fails, run all extractors (current behavior)
- Support future framework detectors (Django, Rails, Spring, Laravel) with minimal code

**Non-Goals:**
- Per-directory detection within a monorepo (per-workspace only for v1)
- Config overrides (`flow.frameworks.force` / `flow.frameworks.ignore`)
- Import scanning or AST-based detection (manifest-only)
- Detection confidence scoring
- Python/Ruby/Java/PHP framework detection (no extractors exist yet)
- Shared AST parsing across extractors (separate optimization)

## Decisions

### D1: Detection at WatchWithFilter time, cached on watchedCollection

**Decision**: Run `FrameworkDetector.Detect(workspaceDir)` once when a workspace is registered via `WatchWithFilter()`. Store result on `watchedCollection.detectedFrameworks`. Re-run when go.mod/package.json changes.

**Alternatives considered**:
- *Per-file detection in ExtractEdges*: Rejected — detection is workspace-scoped, not file-scoped. Would re-read manifest on every file change.
- *At server startup only*: Rejected — workspace directories aren't known at startup. They come via `POST /api/v1/init`.
- *In main.go*: Rejected — main.go doesn't have workspace directory paths.

**Rationale**: `WatchWithFilter()` already has the workspace directory and is the natural lifecycle hook for workspace initialization.

### D2: FrameworkAwareExtractor opt-in interface

**Decision**: Add a new interface alongside the existing one. Extractors opt in by implementing `RequiresFrameworks() []string`. Empty slice = always run.

```go
type FrameworkAwareExtractor interface {
    Extractor
    RequiresFrameworks() []string  // empty = always runs
}
```

**Alternatives considered**:
- *Add Frameworks() to Extractor interface*: Rejected — breaks all 13 existing extractor implementations. Forces every extractor to return something.
- *Registry-level mapping (map[Extractor][]string)*: Rejected — requires maintaining a parallel registration structure in main.go. Brittle.

**Rationale**: Type assertion (`if fa, ok := ex.(FrameworkAwareExtractor)`) is zero-cost and backward compatible. Existing extractors continue working without changes.

### D3: Manifest-only detection with substring/JSON matching

**Decision**: Read go.mod and package.json, use `strings.Contains` and `encoding/json` to detect framework dependencies.

| Framework | Detection Rule |
|-----------|---------------|
| Echo | go.mod contains `github.com/labstack/echo` |
| Gin | go.mod contains `github.com/gin-gonic/gin` |
| Express | package.json `dependencies.express` or `devDependencies.express` exists |
| Go (stdlib) | go.mod exists at all |

**Alternatives considered**:
- *AST import scanning*: Rejected — reads every file, same cost we're trying to eliminate.
- *Regex on manifest*: Rejected — `strings.Contains` is simpler and sufficient for module paths.

**Rationale**: go.mod and package.json are the source of truth for dependencies. Substring matching handles versioned paths (`echo/v4`, `echo/v3`) without a real parser.

### D4: Store on watchedCollection, not Registry

**Decision**: Detection result lives on `watchedCollection.detectedFrameworks`. The watcher passes it through existing `col` context in `extractAndUpsertEdges()`. Registry gets a `SetActiveFrameworks()` method that creates a filtered copy.

**Alternatives considered**:
- *Pass per ExtractEdges call*: Rejected — redundant allocation per file, code smell.
- *Store on Registry directly*: Rejected — Registry is shared across workspaces with different frameworks.

**Rationale**: `watchedCollection` is already per-workspace. The watcher has `col` context in every `processFile` call.

**Concurrency note**: `SetActiveFrameworks()` replaces the extractor slice on the shared `*Registry`. Since `ExtractEdges()` is called concurrently from multiple `processFile` goroutines, this requires synchronization. Use `atomic.Pointer[[]Extractor]` for lock-free reads on the hot path and safe pointer replacement on the write path.

### D5: net/http always runs for Go projects

**Decision**: If go.mod exists, include `"go"` in detected frameworks. The net/http extractor's `RequiresFrameworks()` returns `["go"]`, so it always runs for Go projects.

**Rationale**: net/http is Go stdlib. Projects can register routes via `http.HandleFunc` without importing any framework. False positives are harmless (extractor finds nothing, returns nil).

## Risks / Trade-offs

| Risk | Confidence | Mitigation |
|------|------------|------------|
| **Monorepo = zero benefit**: Workspace with Echo + Gin + net/http in same go.mod runs all extractors anyway | HIGH | Document limitation. Per-directory detection deferred to v2. |
| **Stale detection after adding framework**: User adds Echo to go.mod but doesn't re-register workspace | HIGH | Watch go.mod/package.json for changes, re-run detection on modification |
| **Partial detection failure**: Go detection fails but Node succeeds → Go extractors silently don't run | MEDIUM | Per-language fallback: if any manifest read fails for a language, run all extractors for that language's file extensions |
| **False positive**: Framework imported but not used for routing (e.g., express-session without routes) | LOW | Harmless — extractor runs, finds nothing, returns nil |
| **net/http false positive**: Go library with no HTTP routes runs net/http extractor | LOW | Harmless — same as above |
