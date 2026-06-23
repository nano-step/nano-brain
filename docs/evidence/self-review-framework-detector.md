# Self-Review: PR #436 — Generic Framework Detector

## Changed Files

| File | Change | Lines |
|------|--------|-------|
| `internal/graph/detector.go` | NEW — FrameworkDetector, rules, detection helpers | +120 |
| `internal/graph/detector_test.go` | NEW — 13 unit tests | +180 |
| `internal/graph/edge.go` | ADD — FrameworkAwareExtractor interface | +5 |
| `internal/graph/registry.go` | MOD — atomic.Pointer, SetActiveFrameworks, hasIntersection | +40 |
| `internal/graph/echo_extractor.go` | ADD — RequiresFrameworks() ["echo"] | +4 |
| `internal/graph/gin_extractor.go` | ADD — RequiresFrameworks() ["gin"] | +4 |
| `internal/graph/express_extractor.go` | ADD — RequiresFrameworks() ["express"] | +4 |
| `internal/graph/nethttp_extractor.go` | ADD — RequiresFrameworks() ["go"] | +4 |
| `internal/watcher/watcher.go` | MOD — detection in WatchWithFilter, re-detection in processFile | +40 |
| `cmd/nano-brain/main.go` | MOD — wire detector to watcher | +6 |

## Review Sections

### Correctness
- ✅ Framework detection reads go.mod (substring match) and package.json (JSON parse)
- ✅ Searches 1 level deep for monorepo support
- ✅ atomic.Pointer for lock-free concurrent reads in Registry
- ✅ Fail-open: empty detection = all extractors run
- ✅ Re-detection on go.mod/package.json changes in processFile
- ✅ Both dependencies and devDependencies checked in package.json

### Edge Cases
- ✅ Malformed go.mod → returns false (checked via "module " content)
- ✅ No manifests → empty framework list → all extractors run
- ✅ Detection failure → warn log, empty result → all extractors run
- ✅ nil activeExtractors → fallback to full extractor list

### Thread Safety
- ✅ atomic.Pointer[[]Extractor] for Registry.activeExtractors
- ✅ mu.Lock around watchedCollection map updates
- ✅ col passed by value in processFile (correct — map update is separate)

### Test Coverage
- ✅ 13 unit tests: Echo, Gin, Express, devDependencies, malformed, empty, filtering
- ✅ go test -race -short passes
- ✅ Live verification: nano-brain=["echo","go"], express-app=["express"], next-app=[]

### Pre-existing Issues (not from this PR)
- ⚠️ 3.3: express_integration_test.go middleware count assertion (pre-existing)
- ⚠️ 3.4: httpVerbs unused var in http_router_helpers.go (pre-existing)

## Verdict: PASS
