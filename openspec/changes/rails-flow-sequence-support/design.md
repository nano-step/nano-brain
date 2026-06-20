## Context

nano-brain currently supports flow and sequence diagrams for JS/TS codebases through:
- `JSControlFlowExtractor` — extracts control flow graphs from JS/TS functions
- Framework-specific extractors (Express, NestJS, Echo, Gin, etc.) — extract HTTP route edges
- `FlowBuilder` — constructs call chains from edges stored in `graph_edges` table
- Sequence diagram renderer — visualizes flows as Mermaid sequence diagrams

For Rails, we have:
- ✅ `RailsExtractor` — extracts HTTP edges from `config/routes.rb` (route → controller#action)
- ❌ No Ruby CFG extraction (cannot visualize control flow within methods)
- ❌ No Ruby call graph extraction (cannot trace controller → service → model calls)
- ❌ No Ruby symbol extraction (`memory_symbols` can't find Ruby functions)
- ❌ CFG entry format mismatch — Ruby handler names use `#` (e.g., `TokensController#signup`) but SQL/flow builder assume `::` separator

The `gotreesitter` library already includes a Ruby grammar, so we can parse Ruby files. The challenge is building extractors that understand Rails conventions and fixing the entry format mismatch.

## Goals / Non-Goals (v1 — single PR)

**Goals:**
- Extract control flow graphs from Ruby methods (if/else, loops, begin/rescue)
- Extract method definitions and calls from Rails controllers, models, and services (same-file only)
- Fix CFG entry format mismatch for Ruby's `#` separator
- Add Ruby symbol extractor for `memory_symbols`
- Integrate Ruby edges with existing flow builder and sequence diagram renderers
- Add benchmarks for Rails extraction

**Non-Goals (deferred to v2):**
- Cross-file call resolution (concerns, mixins, ActiveRecord dynamic methods)
- `before_action`/`after_action` callback extraction
- Block/yield resolution
- Metaprogramming handling (`method_missing`, `define_method`, `send`)
- Support non-Rails Ruby frameworks (Sinatra, Hanami)
- Extract database schema or migration flows

## Decisions

### Decision 1: CFG Entry Format — Option A (ControllerName#action)

**Choice**: Ruby CFG entries use format `file.rb::ControllerName#action`

**Rationale**: This is semantically correct for Ruby and aligns with Rails HTTP handler names (e.g., `TokensController#signup`). The SQL query `split_part(entry, '::', 2)` will extract `ControllerName#action`, which matches the HTTP edge target.

**Implementation**: Update `GetFunctionFlowchartByHandler` SQL to also split on `#` when the second part contains `#`. Update `cfg_loader.go` to handle `#`-separated names.

**Alternatives considered**:
- Option B (`file.rb::action`): Rejected — loses controller context, makes debugging harder

### Decision 2: Separate CFG and Call Graph Extractors

**Choice**: Create two separate extractors — `RubyControlFlowExtractor` (CFG) and `RubyExtractor` (call graph)

**Rationale**: This matches the JS/TS architecture where `JSControlFlowExtractor` handles function-level CFGs and `JSExtractor` handles file-level call graphs. Separation of concerns:
- CFG extractor: focused on control flow within method bodies
- Call graph extractor: focused on method definitions and inter-method calls

**Alternatives considered**:
- Single combined extractor: Rejected — would violate single responsibility and make testing harder
- Extend `RailsExtractor`: Rejected — route extraction is fundamentally different from call graph extraction

### Decision 3: Same-File Call Graph (v1)

**Choice**: v1 extracts method calls only within the same file. Cross-file resolution deferred to v2.

**Rationale**: Ruby's dynamic dispatch (`User.find`, `render json:`, `current_user`) makes cross-file resolution complex. Same-file extraction is reliable and covers the most common patterns (controller → service calls within the same file). Cross-file resolution (concerns, mixins, AR) requires class→file mapping which is a significant complexity jump.

**Alternatives considered**:
- Full cross-file resolution: Deferred to v2 — too complex for single PR
- No call graph: Rejected — would make flow diagrams useless

### Decision 4: Reuse gotreesitter Ruby Grammar

**Choice**: Use the existing `gotreesitter/grammars.RubyLanguage()` for parsing

**Rationale**: The Ruby grammar is already vendored and tested. No new dependencies needed.

**Alternatives considered**:
- Add Ruby AST gem (e.g., Parser gem): Rejected — would require CGO or external process, violates static binary constraint
- Manual regex parsing: Rejected — error-prone for nested structures

### Decision 5: Store Ruby Edges with Language Metadata

**Choice**: Set `Language: "ruby"` on all extracted edges and include Rails-specific metadata (controller, action, model)

**Rationale**: Enables filtering and display in flow diagrams. Metadata allows sequence diagrams to show controller names and action names.

**Alternatives considered**:
- No language metadata: Rejected — would make it impossible to filter by language in multi-language codebases
- Separate table for Ruby edges: Rejected — unnecessary complexity when metadata column suffices

## Risks / Trade-offs

### Risk 1: Dynamic Metaprogramming
**Risk**: Ruby's metaprogramming (method_missing, eval, define_method) cannot be statically analyzed.
**Mitigation**: Document as v1 limitation. Focus on standard Rails patterns. Users can add manual annotations for dynamic code. Defer to v2.

### Risk 2: Performance Impact
**Risk**: Parsing Ruby files adds CPU overhead at index time.
**Mitigation**: Ruby parsing is bounded by file count and runs in the watcher goroutine. No impact on search/query latency. Can add `--skip-ruby` flag if needed.

### Risk 3: Sparse Call Graphs
**Risk**: Ruby call graphs will be sparser than JS/TS due to dynamic dispatch, `send`, ActiveRecord.
**Mitigation**: Same-file extraction for v1 is reliable. Set clear expectations in documentation. Cross-file resolution in v2.

### Risk 4: `before_action` Callbacks Invisible
**Risk**: Flow diagrams for Rails controllers will miss authentication/authorization steps.
**Mitigation**: Document as v1 limitation. Defer callback extraction to v2.

### Risk 5: CFG Entry Format Mismatch
**Risk**: Ruby handler names use `#` (e.g., `TokensController#signup`) but SQL/flow builder assume `::` separator.
**Mitigation**: Fix SQL query to handle `#` separator. Update `cfg_loader.go` for Ruby-aware name resolution.

### Trade-off: Completeness vs Reliability
We choose reliable extraction of standard patterns over attempting (and failing) to extract everything. This means some Ruby code won't have complete flow diagrams, but what we extract will be accurate. Cross-file resolution and dynamic dispatch handling deferred to v2.
