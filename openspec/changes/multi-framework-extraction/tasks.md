## 1. Shared TypeScript/JavaScript Router Helpers

- [ ] 1.1 Create `internal/graph/ts_router_helpers.go` with helper functions
- [ ] 1.2 Implement `tsStringArg(bt, node, lang, n)` — extract string argument at position N
- [ ] 1.3 Implement `tsVarArg(bt, node, lang, n)` — extract variable reference at position N
- [ ] 1.4 Implement `tsExtractHTTPMethod(bt, callNode)` — extract method from `app.get`/`router.post` etc.
- [ ] 1.5 Implement `tsExtractHandlerName(bt, funcNode)` — extract handler function/variable name
- [ ] 1.6 Add unit tests for all helpers in `internal/graph/ts_router_helpers_test.go`

## 2. Express Route Extractor

- [ ] 2.1 Create `internal/graph/express_extractor.go` implementing `graph.Extractor` interface
- [ ] 2.2 Implement `Supports(ext)` — return true for `.ts`, `.tsx`, `.js`, `.jsx`
- [ ] 2.3 Implement `ExtractEdges(filePath, content)` — walk AST and extract Express routes
- [ ] 2.4 Extract `app.get/post/put/delete/patch` and `router.get/post/put/delete/patch` call expressions
- [ ] 2.5 Emit `EdgeHTTP` edges with method, path, and handler name
- [ ] 2.6 Extract `app.use()` and `router.use()` middleware patterns
- [ ] 2.7 Emit `EdgeMiddleware` edges for named middleware
- [ ] 2.8 Implement Express detection heuristic (import + call pattern matching)
- [ ] 2.9 Handle anonymous function handlers with synthetic `<anonymous_N>` names
- [ ] 2.10 Log warnings for template string paths (not statically analyzable)

## 3. Extractor Registration

- [ ] 3.1 Register Express extractor in `cmd/nano-brain/main.go` under `FlowConfig.Enabled` gate
- [ ] 3.2 Add info log message when extractor is enabled
- [ ] 3.3 Add warning log when extractor init fails

## 4. Unit Tests

- [ ] 4.1 Create `internal/graph/express_extractor_test.go`
- [ ] 4.2 Write table-driven tests for simple route extraction (`app.get('/path', handler)`)
- [ ] 4.3 Write tests for parameterized routes (`/users/:id`)
- [ ] 4.4 Write tests for router-based routes (`router.post(...)`)
- [ ] 4.5 Write tests for multiple HTTP methods
- [ ] 4.6 Write tests for handler name extraction (named function, variable ref, arrow function)
- [ ] 4.7 Write tests for middleware extraction (`app.use(...)`)
- [ ] 4.8 Write tests for Express detection heuristic (positive and negative cases)
- [ ] 4.9 Write tests for edge cases (template strings, missing arguments, chained calls)

## 5. Integration Tests

- [ ] 5.1 Create test fixture files in `test/fixtures/express/` with realistic Express code
- [ ] 5.2 Write integration test that indexes fixture files and verifies correct edges
- [ ] 5.3 Verify edges are compatible with Flow builder (end-to-end flow materialization)

## 6. Documentation

- [ ] 6.1 Update known limitations in `openspec/changes/multi-framework-extraction/design.md`
- [ ] 6.2 Document cross-file Router mounting limitation in design.md
- [ ] 6.3 Reference Phase 2 (#432) and Phase 3 (#433) tracking issues

## 7. Validation

- [ ] 7.1 Run `go build ./...` — passes
- [ ] 7.2 Run `go test -race -short ./...` — passes (unit tests)
- [ ] 7.3 Run `go test -race -tags=integration ./internal/graph/...` — passes (integration tests)
- [ ] 7.4 Run `go vet ./...` — passes
- [ ] 7.5 Verify no lint errors with project linter
