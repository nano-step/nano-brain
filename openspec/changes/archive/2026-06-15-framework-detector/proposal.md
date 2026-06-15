## Why

Graph extractors (Echo, Gin, net/http, Express) run on every matching file via AST self-filtering, wasting CPU when the framework isn't present in the workspace. A Go project using only net/http still pays tree-sitter parsing cost for Echo and Gin extractors on every `.go` file. A JS project without Express still runs the Express extractor on every `.ts/.js` file. As more framework extractors are added (future: Django, Rails, Spring, Laravel), this waste scales linearly.

## What Changes

- **NEW**: `FrameworkDetector` — reads manifest files (`go.mod`, `package.json`) once per workspace to detect which frameworks are present
- **NEW**: `FrameworkAwareExtractor` interface — extractors optionally declare which frameworks they target via `RequiresFrameworks() []string`
- **MODIFIED**: Watcher integration — detection runs at `WatchWithFilter()` time, cached on `watchedCollection`, re-runs when manifest files change
- **MODIFIED**: Registry filtering — extractors whose framework isn't detected are skipped (fail-open: all extractors run if detection fails)
- **ANNOTATED**: Echo, Gin, Express, net/http extractors gain `RequiresFrameworks()` methods

## Capabilities

### New Capabilities
- `framework-detection`: Manifest-based auto-detection of web frameworks in a workspace. Reads go.mod (Go frameworks) and package.json (JS frameworks) to determine which graph extractors should run. Includes re-detection on manifest file changes.

### Modified Capabilities
- `express-route-extraction`: ExpressExtractor gains `RequiresFrameworks() ["express"]` — runs only when Express is detected in package.json. No extraction logic changes.

## Impact

- **Code**: `internal/graph/detector.go` (new), `internal/graph/edge.go` (interface addition), `internal/watcher/watcher.go` (detection integration), `cmd/nano-brain/main.go` (wiring), 4 extractor files (annotation)
- **APIs**: None — no user-facing API changes
- **Dependencies**: None — uses Go stdlib only (`os`, `encoding/json`, `strings`, `filepath`)
- **Systems**: Watcher gains manifest file change detection for re-triggering framework detection
