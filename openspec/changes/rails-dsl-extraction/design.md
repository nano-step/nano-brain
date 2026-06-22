## Context

The Ruby extraction pipeline has three layers:

1. **Per-file extractors** (`ruby_extractor.go`, `rails_extractor.go`) — parse individual `.rb` files with tree-sitter and emit edges (contains, calls, http).
2. **Post-extraction resolver** (`ruby_resolver.go`) — regex-based pass that tries to resolve cross-file calls via a class→file index (`ruby_class_index.go`).
3. **Flow builder** — BFS over the edge graph starting from HTTP handler edges.

Current state:
- The `ruby_extractor.go` emits `SourceNode = "file.rb::method"` with bare `TargetNode = "method"` for same-file calls, and the resolver attempts to qualify cross-file targets.
- The flow builder starts BFS from `bySymbol["UsersController#create"]` (HTTP edge TargetNode) but can't enter the call graph because Ruby source nodes use `file.rb::method` format, not `Controller#action` format.
- The resolver uses two regexes (`rubyQualifiedCallRe`, `rubyClassMethodRe`) that only match `ClassName.method` patterns — missing bare calls like `user.save`, `render json:`, `redirect_to ...`.
- The CFG extractor only handles `method` nodes, not `singleton_method` (class methods like `scope`, `self.find`).
- `railsConventionPath()` only knows `app/controllers/` and `app/models/`.

## Goals / Non-Goals

**Goals:**
- Change SourceNode format to `file.rb::ClassName#methodName` for all call edges
- Fix the cross-file resolver to handle bare method calls via AST scanning
- Add `singleton_method` support to CFG extractor
- Build a new `RailsDSLEdgeExtractor` for associations, callbacks, concerns, Sidekiq
- Expand `railsConventionPath()` to handle services, jobs, mailers, workers
- DB migration to relax edge_type CHECK constraint

**Non-Goals (deferred to v2):**
- FK inference or query chain analysis (ActiveRecord `where`, `joins`, `includes`)
- Deep concern resolution (flattening included methods into the including class)
- Metaprogramming (`method_missing`, `define_method`, `send`)
- Non-Rails Ruby gems (保持 `RequiresFrameworks: ["rails"]` gate)
- Performance profiling of large Rails codebases

## Decisions

### Decision 1: Node Format Change to `file.rb::ClassName#method`

**Choice**: Change SourceNode from `file.rb::method` to `file.rb::ClassName#methodName` for all Ruby call edges. The TargetNode stays bare for same-file calls and becomes qualified for cross-file resolved calls.

**Rationale**: The flow builder's BFS starts with `bySymbol["UsersController#create"]` (from HTTP edge TargetNode). With the new format, `symbolPart("app/controllers/users_controller.rb::UsersController#create")` returns `"UsersController#create"` which matches the HTTP handler format. This eliminates the need for reconcile edges in many cases.

**Alternatives considered**:
- Keep old format + reconcile edges: Works but adds edge count and complexity. The reconcile approach was already tried and only partially works.
- Use `ClassName.method` format (no #): Less standard, doesn't distinguish class vs instance methods.

**Implementation detail**: The class name must be determined during extraction. The tree-sitter AST provides parent class/module context. Walk upward from each method node to find the enclosing class name.

### Decision 2: AST-Based Resolver Replace Regex

**Choice**: Rewrite `ruby_resolver.go` to scan AST nodes instead of regex patterns. Walk the tree-sitter AST for each file, find all call expressions, and resolve them via the class index.

**Rationale**: The regex approach misses:
- Bare method calls inside blocks (`users.each { |u| u.save }`)
- Method calls on local variables (`order.process_payment`)
- Chained calls (`User.where(active: true).order(:name)`)
- Calls inside string interpolation

AST scanning finds ALL call nodes and attempts resolution for each.

**Alternatives considered**:
- Improve regex patterns: Rejected — regex can't handle nested expressions or blocks
- Keep regex + add more patterns: Rejected — infinite cases to cover
- Query-time resolution: Rejected — every query pays the cost

### Decision 3: RailsDSLEdgeExtractor as Separate File

**Choice**: Create `internal/graph/rails_dsl_extractor.go` (~250 lines) implementing `FrameworkAwareExtractor`. Register alongside existing `RailsExtractor` and `RubyGraphExtractor`.

**Rationale**: Separation of concerns. The existing `rails_extractor.go` handles routes. The existing `ruby_extractor.go` handles call graphs. The new extractor handles Rails-specific DSL patterns (associations, callbacks, concerns). Each file stays focused.

**Alternative considered**: Extend `ruby_extractor.go` — rejected because it's already 206 lines and handles generic Ruby, not Rails-specific DSL.

### Decision 4: Edge Kinds for DSL Patterns

**Choice**: Map DSL patterns to existing edge kinds:
- Associations (`has_many`, `belongs_to`) → `EdgeCalls` with metadata `{"dsl": "association", "type": "has_many", "target": "Order"}`
- Callbacks (`before_action`, `after_commit`) → `EdgeMiddleware` with metadata `{"dsl": "callback", "type": "before_action", "target": "set_user"}`
- Concerns (`include`, `extend`) → `EdgeCalls` with metadata `{"dsl": "concern", "type": "include", "target": "Authenticatable"}`
- Sidekiq (`perform_async`) → `EdgeIntegration` with metadata `{"dsl": "sidekiq", "target": "OrderProcessor"}`

**Rationale**: No new edge kinds needed. The existing kinds convey the right semantics. Metadata carries the DSL-specific details.

**Alternative considered**: New `EdgeDSL` kind — rejected because it adds complexity to every query and filter that checks edge kinds.

### Decision 5: DB Migration Relax CHECK Constraint

**Choice**: The current CHECK constraint (`00027`) allows: `contains, imports, calls, references, http, middleware, integration, reconcile`. No new kinds needed in Phase 1-2, so the CHECK constraint doesn't need updating until Phase 3 (if we add new kinds).

Wait — we're using existing edge kinds. No migration needed unless we discover a new kind is required. The CHECK constraint already covers all kinds we'll use.

**Revised**: No migration needed for this change. If integration extraction in Phase 3 requires a new kind, we'll add it then.

### Decision 6: Expanded Convention Path Map

**Choice**: Extend `railsConventionPath()` in `ruby_class_index.go` to handle:
- `*Service` → `app/services/snake.rb`
- `*Job` / `*Worker` → `app/jobs/snake.rb` / `app/workers/snake.rb`
- `*Mailer` → `app/mailers/snake.rb`
- `*Policy` → `app/policies/snake.rb` (common Rails pattern)
- `*Serializer` → `app/serializers/snake.rb` (common Rails pattern)

**Rationale**: These are standard Rails directory conventions. The resolver's fallback already handles controllers and models; expanding it catches more cases without requiring exact index matches.

**Alternative considered**: Full Zeitwerk resolver — rejected for v1, too complex.

## Component Design

### RailsDSLEdgeExtractor

```
type RailsDSLEdgeExtractor struct {
    lang    *gotreesitter.Language
    queries map[string]*gotreesitter.Query  // pre-compiled queries
}
```

Tree-sitter queries:
1. **Association query**: Match `(call method: (identifier) @method arguments: (argument_list (simple_symbol) @target))` where method is in the association set.
2. **Callback query**: Match `(call method: (identifier) @method arguments: (argument_list ...))` where method is in the callback set.
3. **Concern query**: Match `(call method: (identifier) @method arguments: (argument_list (constant) @target))` where method is `include` or `extend`.

### Metadata Format

Association edge:
```json
{
    "dsl": "association",
    "type": "has_many",
    "target_model": "Order",
    "foreign_key": "user_id"
}
```

Callback edge:
```json
{
    "dsl": "callback",
    "type": "before_action",
    "target_method": "set_user",
    "prepend": false
}
```

Concern edge:
```json
{
    "dsl": "concern",
    "type": "include",
    "target_module": "Authenticatable"
}
```

Sidekiq edge:
```json
{
    "dsl": "sidekiq",
    "queue": "default",
    "target_class": "OrderProcessor"
}
```

### Singleton Method CFG Support

Add `(singleton_method name: (identifier) @fn_name body: (body_statement) @body) @fn_decl` to `rubyGraphCallFuncQuery` in `ruby_extractor.go` (already present in `rubyGraphContainsQuery`). The CFG extractor's `walkNodes` needs to also visit `singleton_method` nodes.

### Resolver Rewrite

The new resolver:
1. Parse each file's AST with tree-sitter
2. Walk all `call` nodes
3. For each call, extract the receiver (if any) and method name
4. Look up the receiver's class via the class index
5. Emit qualified edges: `SourceNode::ClassName#method → TargetFile::ClassName#method`
6. Emit bare edges + `{"unresolved": true}` for unresolvable calls

## Integration Points

- **Registry**: Add `RailsDSLEdgeExtractor` to `graphExtractors` in `main.go` alongside existing Ruby extractors
- **Watcher**: The resolver's `resolveRubyEdges()` already runs after extraction — update it to use the new AST-based resolver
- **MCP tools**: `memory_graph`, `memory_trace`, `memory_impact` all query `graph_edges` — new edges appear automatically
- **Flow builder**: BFS now works with the new node format — no changes needed to flow builder itself
- **Symbol extractor**: `internal/symbol/ruby_extractor.go` already captures class names — no changes needed

## Tradeoffs

### Breaking Node Format (High Impact, One-Time Cost)
Every existing workspace must reindex after this change. All graph edges will be rebuilt with the new format. This is acceptable because:
- nano-brain is pre-1.0
- The current Ruby extraction is effectively broken (0% resolver hit rate)
- The reindex is automatic on next watch cycle

### Regex vs AST for Resolver (Moderate Impact, Ongoing)
The AST-based resolver is slower per-file than regex but catches 100% of call patterns vs ~20% for regex. The resolver runs once at index time, not at query time, so per-file latency matters less.

### DSL Scope Creep (Low Impact, Bounded)
Rails DSL methods are well-defined (associations, callbacks, concerns). We're not trying to extract arbitrary metaprogramming. The set of DSL methods is finite and documented.

### gem DSL (Out of Scope)
Third-party gem DSLs (Devise, Sidekiq, ActiveJob) are partially covered. Devise's `devise :database_authenticatable` is a callback-style DSL but we're not extracting it in v1. Sidekiq's `sidekiq_options` is a configuration, not an edge. We only extract `perform_async`/`perform_in` as integration edges.
