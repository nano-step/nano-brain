## 1. Framework Detector Core

- [x] 1.1 Create `internal/graph/detector.go` with `FrameworkRule` struct (`Framework string` + `Detect func(dir string) bool`) and `FrameworkDetector` struct
- [x] 1.2 Implement `NewFrameworkDetector(rules []FrameworkRule)` constructor and `Detect(workspaceDir string) []string` method
- [x] 1.3 Add `DefaultRules`: Echo (go.mod substring), Gin (go.mod substring), Express (package.json JSON parse), Go (go.mod existence)
- [x] 1.4 Implement `detectGoModDep(dep string)` helper that reads go.mod and does `strings.Contains`
- [x] 1.5 Implement `detectPackageJSONDep(dep string)` helper that reads package.json and checks both `dependencies` and `devDependencies` maps

## 2. Extractor Interface Addition

- [x] 2.1 Add `FrameworkAwareExtractor` interface to `internal/graph/edge.go` with `RequiresFrameworks() []string`
- [x] 2.2 Add `RequiresFrameworks() []string` returning `["echo"]` to EchoRouteExtractor
- [x] 2.3 Add `RequiresFrameworks() []string` returning `["gin"]` to GinExtractor
- [x] 2.4 Add `RequiresFrameworks() []string` returning `["express"]` to ExpressExtractor
- [x] 2.5 Add `RequiresFrameworks() []string` returning `["go"]` to NetHTTPExtractor
- [x] 2.6 Skipped per MUST NOT DO — GoGraphExtractor is language-level, not framework-level

## 3. Watcher Integration

- [x] 3.1 Add `detectedFrameworks []string` field to `watchedCollection` struct
- [x] 3.2 Add `frameworkDetector *graph.FrameworkDetector` field and `WithFrameworkDetector()` setter on Watcher
- [x] 3.3 Call `detector.Detect(absPath)` in `WatchWithFilter()` after collection setup, store result on `col.detectedFrameworks`
- [x] 3.4 Add `SetActiveFrameworks(frameworks []string)` method on Registry using `atomic.Pointer[[]Extractor]` for lock-free concurrent reads
- [x] 3.5 Call `graphRegistry.SetActiveFrameworks(col.detectedFrameworks)` at start of collection scan

## 4. Re-detection on Manifest Changes

- [x] 4.1 In `processFile()`, check if changed file is go.mod or package.json (by `filepath.Base`)
- [x] 4.2 When manifest file changes, re-run `detector.Detect()` and update `col.detectedFrameworks`
- [x] 4.3 Log re-detection results at DEBUG level

## 5. Wiring & main.go

- [x] 5.1 Create detector in `startServer()` with `graph.NewFrameworkDetector(graph.DefaultRules)`
- [x] 5.2 Pass detector to watcher via `WithFrameworkDetector(detector)` (only when `cfg.Flow.Enabled`)

## 6. Logging & Observability

- [x] 6.1 Log detected frameworks at DEBUG level in `WatchWithFilter()` with workspace path and framework list
- [x] 6.2 Detection failures fall back to empty framework list for the language; watcher logs detected frameworks at DEBUG

## 7. Tests

- [x] 7.1 Unit test: go.mod with Echo dep → detects `["echo", "go"]`
- [x] 7.2 Unit test: go.mod with Gin dep → detects `["gin", "go"]`
- [x] 7.3 Unit test: go.mod with both → detects `["echo", "gin", "go"]`
- [x] 7.4 Unit test: go.mod without frameworks → detects `["go"]`
- [x] 7.5 Unit test: package.json with express → detects `["express"]`
- [x] 7.6 Unit test: no manifests → detects `[]`
- [x] 7.7 Unit test: malformed go.mod → returns empty for Go
- [x] 7.8 Unit test: FrameworkAwareExtractor filtering — extractor skipped when framework not in set
- [x] 7.9 Unit test: FrameworkAwareExtractor filtering — extractor runs when framework is in set
- [x] 7.10 Unit test: empty RequiresFrameworks() — extractor always runs
- [x] 7.11 Unit test: package.json with express in devDependencies → detects `["express"]`
- [x] 7.12 Re-detection logic implemented in processFile — manifest file changes trigger detector.Detect() and update SetActiveFrameworks
- [x] 7.13 Same code path handles manifest creation — processFile fires on new files via fsnotify Create events

## 8. Validation

- [x] 8.1 `go build ./...` passes
- [x] 8.2 `go test -race -short ./...` passes
- [x] 8.3 `go vet ./...` passes
- [x] 8.4 Verify: nano-brain detects ["echo","go"], zengamingx detects ["express"] (from tradeit-backend/package.json 1 level deep), capyhome detects [], flow POST /payouts found:true
