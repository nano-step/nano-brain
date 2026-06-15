# Multi-Framework HTTP Route Extraction

## Problem

The flow visualization feature currently only supports Go frameworks (Echo, Gin, net/http). TypeScript/JavaScript projects like NestJS, Express, and Nuxt.js have no HTTP route extraction, making flow visualization impossible for these codebases.

Real-world projects like zengamingx contain multiple frameworks across repositories:
- NestJS (backend APIs)
- Express.js (middleware, services)
- Nuxt.js (SSR frontend)
- Redis/WebSocket (real-time)
- MySQL/PostgreSQL (persistence)

## Goal

Enable flow visualization for TypeScript/JavaScript projects by adding HTTP route extractors for popular Node.js frameworks, with proper handling of multi-repo workspaces.

## Proposed Extractors

### Priority 1: NestJS
- Parse `@Controller('prefix')` class decorators
- Parse `@Get()`, `@Post()`, `@Put()`, `@Delete()` method decorators
- Handle `@Module()` imports to resolve controller → service dependencies
- Support `@Injectable()` services

### Priority 2: Express.js
- Parse `router.get()`, `router.post()`, `app.get()`, `app.post()` calls
- Handle `express.Router()` middleware chains
- Support route prefixes via `app.use('/prefix', router)`

### Priority 3: Nuxt.js / Next.js
- Parse file-based routing (`pages/` directory structure)
- Handle API routes (`server/api/`, `pages/api/`)
- Support middleware registration

## Architecture

### Extractor Interface
```typescript
interface RouteExtractor {
  supports(ext: string): boolean;
  extractEdges(filePath: string, content: string): Edge[];
}
```

### Edge Types
- `EdgeHTTP` - HTTP route to handler (e.g., `POST /api/users → UsersController.create`)
- `EdgeMiddleware` - Middleware dependencies (e.g., `AuthMiddleware → UsersController`)
- `EdgeIntegration` - External service calls (e.g., `UserService → Redis`, `OrderService → MySQL`)

## Success Criteria

1. NestJS projects show HTTP routes in flow dashboard
2. Express.js routes are extracted and visualized
3. Multi-framework workspaces show cross-framework dependencies
4. Flow materialization completes without errors for TypeScript projects

## Out of Scope

- GraphQL schema extraction (future work)
- gRPC proto extraction (future work)
- WebSocket event routing (future work)

## References

- Current Go extractors: `internal/graph/echo_extractor.go`, `gin_extractor.go`, `nethttp_extractor.go`
- TypeScript code graph: `internal/graph/typescript_extractor.go`
- Flow materializer: `internal/flow/materializer.go`
