## ADDED Requirements

### Requirement: Extract Echo HTTP routes as `http` edges
The system SHALL statically extract HTTP route registrations from Go source using the Echo framework and emit one `http` graph edge per route, from a `"<METHOD> <path>"` entry node to the route's handler symbol, with `{method, path}` in edge metadata.

#### Scenario: Plain route registration
- **WHEN** a Go file contains `e.POST("/api/topup", handlers.HandleTopup)`
- **THEN** an `http` edge is emitted with source node `"POST /api/topup"`, target the resolved handler symbol, and metadata `{method:"POST", path:"/api/topup"}`

#### Scenario: All supported verbs
- **WHEN** a route is registered with `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD`, or `OPTIONS`
- **THEN** the emitted `http` edge's method matches the verb used

### Requirement: Resolve route group prefixes
The system SHALL resolve the full path of routes registered on Echo route groups by accumulating group prefixes, including nested groups.

#### Scenario: Single group prefix
- **WHEN** `g := e.Group("/api")` and `g.GET("/balance", h.GetBalance)`
- **THEN** the emitted entry node is `"GET /api/balance"`

#### Scenario: Nested groups
- **WHEN** `g := e.Group("/api")`, `v1 := g.Group("/v1")`, and `v1.POST("/query", h.Query)`
- **THEN** the emitted entry node is `"POST /api/v1/query"`

### Requirement: Extract middleware as `middleware` edges
The system SHALL emit a `middleware` graph edge from each middleware symbol to the handler(s) it applies to, for global (`e.Use`), group-scoped (`g.Use`), and per-route trailing middleware arguments.

#### Scenario: Per-route middleware
- **WHEN** `g.POST("/topup", h.HandleTopup, AuthMW)`
- **THEN** a `middleware` edge is emitted from `AuthMW` to `h.HandleTopup`

#### Scenario: Group-scoped middleware
- **WHEN** `g.Use(AuthMW)` precedes routes registered on `g`
- **THEN** a `middleware` edge from `AuthMW` is emitted to each handler registered on `g`

### Requirement: Extract handler name from factory calls, method values, and identifiers
The handler argument is most commonly a **factory call** that returns an `echo.HandlerFunc` (e.g. `handlers.WriteDocument(deps...)`), not a bare handler reference. The system SHALL extract the bare callee name as the `http` edge `target_node` (matching the bare-name convention of `calls` targets), and SHALL NOT silently drop a route when no name is extractable.

#### Scenario: Factory-call handler
- **WHEN** a route is registered as `write.POST("/write", handlers.WriteDocument(s.queries, s.db))`
- **THEN** the `http` edge target is the bare name `WriteDocument`

#### Scenario: Method-value handler
- **WHEN** a route is registered as `e.GET("/graph", h.HandleGraph)`
- **THEN** the `http` edge target is the bare name `HandleGraph`

#### Scenario: Bare identifier handler
- **WHEN** a route is registered as `e.POST("/topup", HandleTopup)`
- **THEN** the `http` edge target is `HandleTopup`

#### Scenario: Inline closure handler
- **WHEN** a route is registered with an inline `func(c echo.Context) error { ... }`
- **THEN** the `http` edge is still emitted with the entry node and no resolvable target, AND a log entry at WARN/DEBUG records file and line, AND no panic occurs

#### Scenario: Non-local receiver
- **WHEN** routes register on a struct-field receiver (e.g. `s.echo.POST(...)`, `s.echo.Group(...)`) rather than a local `echo.New()` variable
- **THEN** the routes and groups are still recognized (matching is by method name, not by a known variable)

### Requirement: Preserve extractor-supplied edge metadata
The persistence path SHALL preserve metadata supplied by an extractor on an `Edge`, merging in `line` and `language`, rather than overwriting metadata with only `{line, language}`.

#### Scenario: HTTP edge retains method and path
- **WHEN** an `http` edge with metadata `{method, path}` is persisted by the watcher
- **THEN** the stored `graph_edges.metadata` JSONB contains `method`, `path`, `line`, and `language`

#### Scenario: Existing extractors unaffected
- **WHEN** a `calls`/`imports`/`contains` edge carrying no extractor metadata is persisted
- **THEN** the stored metadata contains `line` and `language` exactly as before this change
