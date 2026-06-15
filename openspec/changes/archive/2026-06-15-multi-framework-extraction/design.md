## Context

Flow visualization (Phase 1) ships with Go-only HTTP route extractors (Echo, Gin, net/http). The TypeScript/JavaScript graph extractors handle `contains`/`imports`/`calls` edges but have **no HTTP route extraction**. This means the Flow dashboard shows nothing for Express.js, NestJS, or Nuxt.js projects.

The existing extractor architecture is clean: each framework gets its own file implementing `graph.Extractor` (`ExtractEdges` + `Supports`), registered under `FlowConfig.Enabled` in `main.go`. All extractors produce the same `EdgeHTTP`/`EdgeMiddleware` edge types consumed by the language-agnostic Flow builder.

Phase 1 scope: Express.js only. NestJS (#432) and Nuxt.js (#433) are tracked separately.

## Goals / Non-Goals

**Goals:**
- Extract HTTP routes from Express.js `.ts`/`.js` files via AST pattern matching
- Emit `EdgeHTTP` edges (method + path → handler name) compatible with Flow builder
- Emit `EdgeMiddleware` edges for `app.use()` / `router.use()` patterns
- Follow established extractor pattern (one file per framework, shared helpers)
- Reuse existing `gotreesitter` TS/JS grammars (no new dependencies)

**Non-Goals:**
- Cross-file Express Router mounting (`app.use('/prefix', router)`) — documented as Phase 1 limitation
- Nuxt.js filesystem routing — separate paradigm, tracked in #433
- NestJS decorator extraction — tracked in #432
- Express error-handler middleware (4-arg signature) — edge case, low value for v1
- Dynamic route registration (`routes.forEach(...)`) — not statically analyzable
- `app.route()` chaining (`app.route('/path').get(handler).post(handler)`) — uncommon pattern, defer to Phase 2 if needed
- Express `app.all()`, `app.head()`, `app.options()` — lower priority methods, can add later

## Decisions

### D1: One file per framework — `express_extractor.go`

**Decision:** Create `internal/graph/express_extractor.go` following the pattern of `echo_extractor.go`, `gin_extractor.go`, `nethttp_extractor.go`.

**Rationale:** Each framework parses the AST differently. Express uses call-expression patterns (`app.get(path, handler)`), which are structurally different from NestJS decorators. One file keeps each extractor independently testable and maintainable.

**Alternatives considered:**
- Single `typescript_http_extractor.go` for all JS/TS frameworks — rejected: NestJS (decorators) and Express (call expressions) are too different for a single extractor; would create complex conditional logic.

### D2: Match heuristic — import + call pattern detection

**Decision:** `Supports(ext)` returns true for `.ts`, `.tsx`, `.js`, `.jsx`. The `match()` function (new) checks for Express-specific signals: `require('express')` import, `express.Router()`, or `app.get/post/put/delete/patch` call patterns.

**Rationale:** Express detection must be heuristic since there's no single signal. The combination of import + call patterns reduces false positives. Config override available if auto-detection fails.

**Alternatives considered:**
- Config-only framework selection — rejected: zero-config is critical for UX; heuristic works for 90%+ of Express projects.
- Package.json dependency scan — rejected: slower, requires filesystem access beyond the current file.

### D3: Handler name convention — bare method names

**Decision:** Express handler targets use bare function/variable names (`getUser`, `userController.create`), NOT qualified names. This matches the existing Flow builder's symbol reconciliation which splits on `::` and matches the symbol part.

**Rationale:** The Flow builder's `BuildFlow` function reconciles `EdgeHTTP` targets against `EdgeCalls` source nodes via `bySymbol[bareName]`. Using bare names ensures compatibility without modifying the Flow builder.

**Alternatives considered:**
- Qualified names (`file.ts::getUser`) — rejected: breaks Flow builder reconciliation without code changes to the builder itself.

### D4: Shared helpers in `ts_router_helpers.go`

**Decision:** Create `internal/graph/ts_router_helpers.go` for JS/TS-specific AST helpers, analogous to `http_router_helpers.go` for Go.

**Rationale:** JS/TS AST node types (`arrow_function`, `member_expression`, `string`, `template_string`) differ from Go (`func_literal`, `selector_expression`, `interpreted_string_literal`). Go helpers cannot be reused.

**Helpers needed:**
- `tsStringArg(bt, node, lang, n)` — extract string argument at position N
- `tsVarArg(bt, node, lang, n)` — extract variable reference at position N
- `tsExtractHTTPMethod(bt, callNode)` — extract method from `app.get`/`router.post` etc.
- `tsExtractHandlerName(bt, funcNode)` — extract handler function/variable name

### D5: Middleware extraction — `app.use()` patterns

**Decision:** Emit `EdgeMiddleware` edges for `app.use(path?, middleware)` and `router.use(path?, middleware)` patterns where the middleware is a named function/variable.

**Rationale:** Middleware visibility is valuable for flow completeness. Named middleware (e.g., `app.use(auth)`) produces actionable flow edges. Anonymous middleware (`app.use((req, res, next) => ...)`) emits a synthetic name.

**Known limitation:** Router-level `router.use(mw)` applies middleware to ALL routes on that router, but Phase 1 emits per-use edges only (not fan-out to all routes). Fan-out is Phase 2 scope.

### D6: Route parameter normalization — path-to-regexp syntax

**Decision:** Store route paths in path-to-regexp format (`/users/:id`). Express already uses this format. NestJS (Phase 2) uses the same syntax in decorators.

**Rationale:** Consistent format across frameworks. The Flow builder and Mermaid renderer already handle `:param` syntax from Go extractors.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| Cross-file Router mounting produces partial paths | Document as Phase 1 known limitation. Phase 2 adds import tracing. |
| Anonymous closures produce dead-end flows | Emit `<anonymous_N>` synthetic names. Flow builder handles gracefully. |
| Express middleware vs handler disambiguation | Heuristic: last callable argument = handler. Document error-handler false positive. |
| Tree-sitter TS grammar decorator support varies | Not needed for Phase 1 (Express). Phase 2 (NestJS) will verify. |
| False positives (e.g., `axios.get()` mistaken for Express) | Filter by receiver name heuristic (`app`, `router`, `server`). Log warnings. |

## Migration Plan

No migration needed — this is additive. New extractors are registered behind `FlowConfig.Enabled` gate. Existing Go extractors and Flow pipeline are unchanged.

Rollback: Remove extractor registration from `main.go`. No data changes.

## Resolved Questions

1. **Express variable name detection:** Use configurable list with defaults `["app", "router", "server"]`. Config override available via `config.yml` if auto-detection fails. This balances zero-config UX with flexibility for non-standard variable names.

2. **Global middleware (`app.use()` without path):** Yes, emit `EdgeMiddleware` edges with empty path prefix. This captures global middleware like `app.use(cors)`, `app.use(helmet)` which are common in Express apps.

## Coexistence with TypeScriptGraphExtractor

The new `ExpressRouteExtractor` and existing `TypeScriptGraphExtractor` both match `.ts`/`.tsx` files but emit **different edge kinds**: `EdgeHTTP`/`EdgeMiddleware` vs `EdgeContains`/`EdgeImports`/`EdgeCalls`. This is intentional — the Registry runs ALL extractors for a given extension and concatenates edges. No conflict; complementary data.

## Status

**Phase 1 (Express.js): IMPLEMENTED**

Implemented files:
- `internal/graph/express_extractor.go` — Express route extractor
- `internal/graph/ts_router_helpers.go` — JS/TS AST helpers
- `internal/graph/express_extractor_test.go` — Unit tests (11 test cases)
- `internal/graph/express_integration_test.go` — Integration test with fixture
- `test/fixtures/express/app.ts` — Realistic Express application fixture

All 39 tasks completed. Validation passed:
- `go build ./...` ✓
- `go test -race -short ./...` ✓
- `go test -race -tags=integration ./internal/graph/...` ✓
- `go vet ./...` ✓

**Phase 2 (NestJS):** Tracked in #432 — TODO
**Phase 3 (Nuxt.js):** Tracked in #433 — TODO
