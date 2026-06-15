# Implementation Tasks

## Phase 1: NestJS Extractor

### Task 1.1: Create NestJS Extractor Structure
- [ ] Create `internal/graph/nestjs_extractor.go`
- [ ] Define `NestJSExtractor` struct implementing `Extractor` interface
- [ ] Implement `Supports(ext string) bool` for `.ts`, `.tsx` files
- [ ] Implement `ExtractEdges(filePath string, content []byte) ([]Edge, error)`

### Task 1.2: Parse Controller Decorators
- [ ] Add tree-sitter query for `@Controller('prefix')` pattern
- [ ] Extract controller class name and route prefix
- [ ] Handle multiple decorators on same class

### Task 1.3: Parse Method Decorators
- [ ] Add tree-sitter query for `@Get()`, `@Post()`, `@Put()`, `@Delete()`, `@Patch()`
- [ ] Extract HTTP method and route path
- [ ] Combine with controller prefix (e.g., `@Controller('users')` + `@Get(':id')` = `GET /users/:id`)

### Task 1.4: Handle Route Parameters
- [ ] Parse `@Param('id')` decorators
- [ ] Convert NestJS params to Express-style (`:id`)
- [ ] Handle optional params (`@Param('id?')`)

### Task 1.5: Extract Service Dependencies
- [ ] Parse constructor injection (`constructor(private service: UserService)`)
- [ ] Generate `EdgeIntegration` from controller to service
- [ ] Handle multiple dependencies

### Task 1.6: Unit Tests
- [ ] Create `internal/graph/nestjs_extractor_test.go`
- [ ] Test basic controller extraction
- [ ] Test nested routes (`@Controller('api/v1/users')`)
- [ ] Test multiple HTTP methods
- [ ] Test service injection

## Phase 2: Express.js Extractor

### Task 2.1: Create Express Extractor Structure
- [ ] Create `internal/graph/express_extractor.go`
- [ ] Define `ExpressExtractor` struct implementing `Extractor` interface
- [ ] Implement `Supports(ext string) bool` for `.ts`, `.tsx`, `.js`, `.jsx` files

### Task 2.2: Parse Route Registration
- [ ] Add tree-sitter query for `router.get()`, `router.post()`, etc.
- [ ] Add tree-sitter query for `app.get()`, `app.post()`, etc.
- [ ] Extract route path and handler function

### Task 2.3: Handle Router Prefixes
- [ ] Parse `app.use('/prefix', router)` patterns
- [ ] Prepend prefix to all routes in that router
- [ ] Handle nested routers

### Task 2.4: Extract Middleware Chains
- [ ] Parse middleware arguments in route registration
- [ ] Generate `EdgeMiddleware` edges
- [ ] Handle inline middleware functions

### Task 2.5: Unit Tests
- [ ] Create `internal/graph/express_extractor_test.go`
- [ ] Test basic route extraction
- [ ] Test router prefixes
- [ ] Test middleware chains

## Phase 3: Nuxt.js / Next.js Extractor

### Task 3.1: Create File-Based Route Extractor
- [ ] Create `internal/graph/nuxt_extractor.go`
- [ ] Parse `pages/` directory structure
- [ ] Handle dynamic routes (`[id].vue`, `[...slug].vue`)

### Task 3.2: Handle API Routes
- [ ] Parse `server/api/` directory
- [ ] Handle `*.get.ts`, `*.post.ts` naming conventions
- [ ] Extract HTTP method from filename

### Task 3.3: Unit Tests
- [ ] Create `internal/graph/nuxt_extractor_test.go`
- [ ] Test page route generation
- [ ] Test API route generation
- [ ] Test dynamic routes

## Phase 4: Integration

### Task 4.1: Update Registry
- [ ] Add `NewNestJSExtractor()` to `internal/graph/registry.go`
- [ ] Add `NewExpressExtractor()` to registry
- [ ] Add `NewNuxtExtractor()` to registry
- [ ] Verify extractor priority order

### Task 4.2: Integration Testing
- [ ] Test with zengamingx workspace
- [ ] Verify flow dashboard shows extracted routes
- [ ] Test materialization completes successfully
- [ ] Verify LLM summaries are generated

### Task 4.3: Documentation
- [ ] Update README.md with supported frameworks
- [ ] Add extractor configuration examples
- [ ] Document limitations and edge cases

## Phase 5: Polish

### Task 5.1: Error Handling
- [ ] Handle malformed TypeScript/JavaScript files
- [ ] Handle files with syntax errors
- [ ] Log extraction failures gracefully

### Task 5.2: Performance
- [ ] Benchmark extraction speed
- [ ] Optimize tree-sitter queries
- [ ] Handle large files efficiently

### Task 5.3: Edge Cases
- [ ] Handle custom decorators
- [ ] Handle dynamic imports
- [ ] Handle conditional routes
