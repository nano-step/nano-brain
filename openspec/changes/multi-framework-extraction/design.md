# Technical Design: Multi-Framework Route Extraction

## Current State

### Existing Extractors (Go)
1. **EchoRouteExtractor** - Parses `e.GET()`, `e.POST()`, `e.Group()` patterns
2. **GinRouteExtractor** - Parses `r.GET()`, `r.POST()`, `r.Group()` patterns
3. **NetHTTPExtractor** - Parses `http.HandleFunc()`, `http.Handle()` patterns

### TypeScript/JavaScript Extractor
- **TypeScriptGraphExtractor** - Only extracts:
  - Contains relationships (class → method)
  - Import relationships
  - Call relationships
  - **No HTTP route extraction**

## Proposed Implementation

### Phase 1: NestJS Extractor

#### Parser Strategy
Use tree-sitter TypeScript grammar to parse decorators:

```typescript
// Pattern to match:
@Controller('users')
class UsersController {
  @Get(':id')
  findOne(@Param('id') id: string) { ... }
  
  @Post()
  create(@Body() dto: CreateUserDto) { ... }
}
```

#### Edge Generation
```
EdgeHTTP: "GET /users/:id" → "UsersController.findOne"
EdgeHTTP: "POST /users" → "UsersController.create"
EdgeContains: "UsersController" → "UsersController.findOne"
EdgeIntegration: "UsersController" → "UsersService" (via constructor injection)
```

#### Implementation Plan
1. Create `internal/graph/nestjs_extractor.go`
2. Add tree-sitter query for `@Controller` decorator
3. Add tree-sitter query for `@Get`, `@Post`, `@Put`, `@Delete` decorators
4. Extract route prefix from controller decorator
5. Extract HTTP method + path from method decorators
6. Handle module imports for service dependency resolution

### Phase 2: Express.js Extractor

#### Parser Strategy
Parse function calls with string literals:

```javascript
// Pattern to match:
router.get('/users/:id', authMiddleware, controller.get);
router.post('/users', validateBody, controller.create);
app.use('/api', apiRouter);
```

#### Edge Generation
```
EdgeHTTP: "GET /api/users/:id" → "controller.get"
EdgeHTTP: "POST /api/users" → "controller.create"
EdgeMiddleware: "GET /api/users/:id" → "authMiddleware"
```

### Phase 3: Nuxt.js / Next.js Extractor

#### File-Based Routing
Parse directory structure:
```
pages/
  users/
    index.vue        → GET /users
    [id].vue         → GET /users/:id
    create.vue       → GET /users/create
  api/
    users/
      index.get.ts   → GET /api/users
      [id].put.ts    → PUT /api/users/:id
```

## Integration Points

### Registry Updates
Add new extractors to `internal/graph/registry.go`:

```go
func NewRegistry() *Registry {
    r := &Registry{}
    r.Register(NewGoExtractor())      // existing
    r.Register(NewTypeScriptExtractor()) // existing
    r.Register(NewNestJSExtractor())  // NEW
    r.Register(NewExpressExtractor()) // NEW
    return r
}
```

### Watcher Updates
No changes needed - watcher already calls `graphRegistry.ExtractEdges()` for all registered extractors.

## Testing Strategy

### Unit Tests
- Create `internal/graph/nestjs_extractor_test.go`
- Test decorator parsing with real NestJS code samples
- Test edge generation accuracy

### Integration Tests
- Test with zengamingx workspace
- Verify flow dashboard shows extracted routes
- Test materialization completes successfully

## Risk Assessment

### Low Risk
- Tree-sitter parsing is well-tested
- Existing extractor pattern is proven
- No changes to core materializer logic

### Medium Risk
- NestJS decorator variations (custom decorators)
- Express middleware chaining complexity
- Multi-repo workspace edge deduplication

## Timeline

- Phase 1 (NestJS): 2-3 days
- Phase 2 (Express): 1-2 days
- Phase 3 (Nuxt/Next): 2-3 days
- Testing + Integration: 1-2 days

**Total: 6-10 days**
