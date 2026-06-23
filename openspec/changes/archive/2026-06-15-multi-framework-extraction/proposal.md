## Why

Flow visualization currently only supports Go frameworks (Echo, Gin, net/http). The express-app workspace—and many other TypeScript/JavaScript projects—use Express.js, NestJS, and Nuxt.js, which have no HTTP route extraction. Without route extraction, the Flow dashboard shows nothing for these projects, making the feature unusable for the majority of the user base.

## What Changes

- Add Express.js HTTP route extractor for `.ts`/`.js` files
- Extract routes from `app.get()`, `router.post()`, `app.use()` and similar Express patterns
- Create shared TypeScript/JavaScript router helpers (`ts_router_helpers.go`)
- Register new extractors behind `FlowConfig.Enabled` gate (same as Go extractors)
- Emit `EdgeHTTP` and `EdgeMiddleware` edges compatible with existing Flow builder

**Phase 2 (tracked separately in #432):** NestJS decorator-based extraction
**Phase 3 (tracked separately in #433):** Nuxt.js filesystem-based routing

## Capabilities

### New Capabilities
- `express-route-extraction`: Extract HTTP routes from Express.js applications using AST-based pattern matching on `app.get/post/put/delete` and `router.*` call expressions

### Modified Capabilities
- (none — this is additive; existing Go extractors and Flow builder are unchanged)

## Impact

- **Code**: New files `internal/graph/express_extractor.go`, `internal/graph/ts_router_helpers.go`, registration in `cmd/nano-brain/main.go`
- **API**: No REST/MCP API changes — extracted edges feed into existing Flow pipeline
- **Dependencies**: None — reuses existing `gotreesitter` with TS/JS grammars already loaded
- **Known limitation (Phase 1)**: Cross-file Express Router mounting (`app.use('/prefix', router)`) produces partial paths without prefix. Documented and deferred to Phase 2.
