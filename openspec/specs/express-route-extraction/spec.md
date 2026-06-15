# express-route-extraction Specification

## Purpose
TBD - created by archiving change multi-framework-extraction. Update Purpose after archive.
## Requirements
### Requirement: Express.js route extraction from call expressions
The system SHALL extract HTTP routes from Express.js `app.get()`, `app.post()`, `app.put()`, `app.delete()`, `app.patch()`, `router.get()`, `router.post()`, `router.put()`, `router.delete()`, `router.patch()` call expressions in `.ts`, `.tsx`, `.js`, `.jsx` files.

#### Scenario: Simple route extraction
- **WHEN** a file contains `app.get('/users', handler)`
- **THEN** the system emits an `EdgeHTTP` edge with method `GET`, path `/users`, and target handler name `handler`

#### Scenario: Parameterized route extraction
- **WHEN** a file contains `app.get('/users/:id', handler)`
- **THEN** the system emits an `EdgeHTTP` edge with method `GET`, path `/users/:id`, and target handler name `handler`

#### Scenario: Router-based route extraction
- **WHEN** a file contains `router.post('/items', createItem)`
- **THEN** the system emits an `EdgeHTTP` edge with method `POST`, path `/items`, and target handler name `createItem`

#### Scenario: Multiple HTTP methods
- **WHEN** a file contains routes with different HTTP methods (`app.get`, `app.post`, `app.put`, `app.delete`, `app.patch`)
- **THEN** the system emits separate `EdgeHTTP` edges for each route with the correct method

### Requirement: Handler name extraction for Express routes
The system SHALL extract handler function or variable names from Express route call expressions. The handler name SHALL be the bare name (not qualified with file path).

#### Scenario: Named function handler
- **WHEN** a file contains `app.get('/users', getUser)`
- **THEN** the handler target is `getUser`

#### Scenario: Variable reference handler
- **WHEN** a file contains `app.get('/users', userController.list)`
- **THEN** the handler target is `userController.list`

#### Scenario: Arrow function handler
- **WHEN** a file contains `app.get('/users', (req, res) => { ... })`
- **THEN** the handler target is `<anonymous_1>` (synthetic name)

### Requirement: Express middleware extraction
The system SHALL extract middleware from `app.use()` and `router.use()` call expressions as `EdgeMiddleware` edges.

#### Scenario: Named middleware
- **WHEN** a file contains `app.use(auth)`
- **THEN** the system emits an `EdgeMiddleware` edge with source `auth`

#### Scenario: Path-scoped middleware
- **WHEN** a file contains `app.use('/api', cors)`
- **THEN** the system emits an `EdgeMiddleware` edge with source `cors`

#### Scenario: Route-specific middleware
- **WHEN** a file contains `app.get('/users', auth, handler)`
- **THEN** the system emits an `EdgeMiddleware` edge with source `auth`

### Requirement: Extractor registration
The system SHALL register the Express route extractor in `main.go` behind the `FlowConfig.Enabled` gate, following the same pattern as Go extractors.

#### Scenario: Flow enabled
- **WHEN** `FlowConfig.Enabled` is `true`
- **THEN** the Express route extractor is registered and processes `.ts`/`.tsx`/`.js`/`.jsx` files

#### Scenario: Flow disabled
- **WHEN** `FlowConfig.Enabled` is `false`
- **THEN** the Express route extractor is not registered

### Requirement: Express detection heuristic
The system SHALL detect Express.js files using a combination of import signals and call patterns.

#### Scenario: Express import detected
- **WHEN** a file imports or requires `express`
- **THEN** the extractor matches the file

#### Scenario: Express Router detected
- **WHEN** a file contains `express.Router()` or `router.get/post/put/delete` patterns
- **THEN** the extractor matches the file

#### Scenario: Non-Express file
- **WHEN** a file has no Express imports or call patterns
- **THEN** the extractor does not match the file

### Requirement: Edge compatibility with Flow builder
The system SHALL emit `EdgeHTTP` and `EdgeMiddleware` edges that are compatible with the existing Flow builder's symbol reconciliation.

#### Scenario: Flow builder reconciliation
- **WHEN** an `EdgeHTTP` edge has target `getUser`
- **THEN** the Flow builder can reconcile it against `EdgeCalls` edges with source containing `getUser`

### Requirement: Known limitation — cross-file Router mounting
The system SHALL document that cross-file Express Router mounting (`app.use('/prefix', router)`) produces partial paths without the prefix in Phase 1.

#### Scenario: Cross-file Router
- **WHEN** a file defines routes on a router that is mounted with a prefix in another file
- **THEN** the extracted route path does NOT include the mount prefix
- **AND** the system logs a warning about the incomplete path

### Requirement: Error handling for unparseable files
The system SHALL gracefully handle files that cannot be parsed by tree-sitter.

#### Scenario: Syntax error in file
- **WHEN** a file contains syntax errors that prevent tree-sitter parsing
- **THEN** the extractor logs a warning and skips the file
- **AND** does not fail the batch extraction

#### Scenario: Binary file misidentified as code
- **WHEN** a binary file is passed to the extractor (e.g., `.ts` extension but binary content)
- **THEN** the extractor logs a warning and skips the file

### Requirement: Shared TypeScript/JavaScript router helpers
The system SHALL provide shared helper functions in `ts_router_helpers.go` for extracting string arguments, variable references, and HTTP method names from JS/TS AST nodes.

#### Scenario: String argument extraction
- **WHEN** a call expression has a string literal argument at position N
- **THEN** the helper returns the string value

#### Scenario: Variable reference extraction
- **WHEN** a call expression has a variable reference argument at position N
- **THEN** the helper returns the variable name

#### Scenario: HTTP method extraction
- **WHEN** a call expression is `app.get(...)` or `router.post(...)`
- **THEN** the helper extracts the method name (`GET`, `POST`) from the property identifier

