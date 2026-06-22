## Why

Ruby code intelligence tools (graph, flow, flowchart, trace, impact) return empty or noisy results for Rails projects. The cross-file resolver has 0% hit rate in benchmarks. Only text search works.

Three compounding problems:

1. **Node format mismatch** — SourceNode uses `file.rb::method` but the flow builder expects class-qualified names like `file.rb::ClassName#method`. This breaks BFS traversal from HTTP handler edges (which use `Controller#action` format) into the call graph.

2. **Broken cross-file resolver** — The regex-based resolver (`ruby_resolver.go`) scans for `ClassName.method` patterns but most Rails calls are bare method calls (`user.save`, `render json:`, `redirect_to ...`) that never match the regex. The resolver adds edges nobody consumes.

3. **No Rails DSL extraction** — `before_action`, `has_many`, `after_commit`, `include Concern`, and Sidekiq `perform_async` calls are invisible to the graph. These represent the actual wiring between controllers, models, services, and background jobs.

## What Changes

### Phase 1: Foundation (3-4 days)

- Change node format from `file.rb::method` to `file.rb::ClassName#methodName`
- Fix cross-file resolver to actually resolve bare method calls via the class index
- Add `singleton_method` support to CFG extractor (class methods, scopes, callbacks)
- DB migration to relax edge_type CHECK constraint for new edge types
- Expand `railsConventionPath()` to handle `app/services/`, `app/jobs/`, `app/mailers/`, `app/workers/`

### Phase 2: DSL Extraction (2-3 days)

- New `RailsDSLEdgeExtractor` (~250 lines) as a `FrameworkAwareExtractor`
- Extract associations: `has_many`, `belongs_to`, `has_one`, `has_and_belongs_to_many` → EdgeCalls with metadata
- Extract callbacks: `before_action`, `after_commit`, `before_save` → EdgeMiddleware
- Extract concerns: `include`/`extend` → EdgeCalls
- Sidekiq detection: `perform_async`/`perform_in` → EdgeIntegration
- Register in `cmd/nano-brain/main.go`

### Phase 3: Polish (1-2 days)

- Flow builder role classification for Rails (controller vs model vs service)
- Ruby integration extraction (Net::HTTP, Faraday outbound calls)

## User Stories

### US-1: Controller-to-model chain visible in flow diagrams
**As a** Rails developer using nano-brain,
**I want** `POST /users` to show `UsersController#create → User.create → User.validates?` in the flow diagram,
**So that** I can understand the full request lifecycle without reading source.

### US-2: Association edges visible in graph queries
**As a** Rails developer,
**I want** `memory_graph` to show `User has_many Orders` edges,
**So that** I can see model relationships without reading schema.rb.

### US-3: Callback chains visible in trace
**As a** Rails developer,
**I want** `memory_trace` from `User#save` to show `before_save → validate → after_commit`,
**So that** I can trace side effects and middleware.

### US-4: Sidekiq job wiring visible
**As a** Rails developer,
**I want** `memory_graph` from a controller to show `OrderService.process → OrderProcessor.perform_async`,
**So that** I can trace async job dispatch from controllers.

### US-5: Concern inclusion visible
**As a** Rails developer,
**I want** to see that `UsersController` includes `Devise::Controllers::Rememberable` via the graph,
**So that** I understand inherited behavior.

## Capabilities

### New Capabilities
- `rails-dsl-extraction`: Extract Rails DSL calls (associations, callbacks, concerns, Sidekiq) as graph edges

### Modified Capabilities
- `ruby-call-graph`: Node format changes from `file.rb::method` to `file.rb::ClassName#method`
- `ruby-cross-file-resolution`: Resolver now handles bare method calls, not just `ClassName.method`
- `ruby-cfg`: Adds `singleton_method` support for class methods
- `flow-visualization`: Rails flows now show 5-10+ node chains through associations and callbacks
- `ruby-convention-path`: Expands to handle services, jobs, mailers, workers

## Impact

- **Code affected**:
  - `internal/graph/ruby_extractor.go` — node format change, class-qualified SourceNode
  - `internal/graph/ruby_resolver.go` — rewrite resolver for bare method calls
  - `internal/graph/ruby_class_index.go` — expand convention path map
  - `internal/graph/ruby_cflow.go` — add `singleton_method` support
  - `internal/graph/rails_dsl_extractor.go` — new: ~250 lines
  - `internal/graph/edge.go` — no new edge kinds (reuses existing)
  - `cmd/nano-brain/main.go` — register new extractor
  - `internal/watcher/watcher.go` — wire new extractor + updated resolver
  - `migrations/00028_relax_edge_type_check.sql` — relax CHECK constraint
  - `internal/graph/testdata/ruby/` — new test fixtures
  - `internal/graph/rails_dsl_extractor_test.go` — new tests

- **Dependencies**: None new. Reuses gotreesitter, existing edge kinds, existing PG schema.

- **Performance**: New extractor runs per-file (~250 lines, tree-sitter based). Resolver O(calls) with hash index. Both negligible at index time.

- **Risk**: HIGH — Node format change is breaking. Pre-1.0 so forced reindex is acceptable. All existing graphs will be rebuilt.

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Node format change breaks existing graphs | High | Forced reindex. Pre-1.0 so acceptable. |
| Regex resolver still misses some patterns | Medium | AST-based resolver in Phase 1 fixes most cases. |
| DSL extractor false positives on method names | Low | Only match known DSL methods. Metadata flag for confidence. |
| Concern/extend resolution scope creep | Medium | v1: emit edges only, no deep resolution. |
| Singleton method CFG extraction complexity | Low | Tree-sitter handles `def self.method` natively. |
