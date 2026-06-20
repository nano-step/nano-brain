## Why

Ruby/Rails is a major web framework, but nano-brain's flow and sequence diagram capabilities currently only support JS/TS for control flow analysis. While Rails route extraction works (HTTP edges from `config/routes.rb`), we cannot visualize the internal flow of Rails controller actions, service calls, or model interactions. This limits the value of flow diagrams for Rails developers who need to understand request handling beyond just route mapping.

## What Changes (v1 — single PR)

- **New Ruby CFG extractor**: Parse Ruby method bodies to extract control flow graphs (if/else, loops, begin/rescue blocks)
- **New Ruby call graph extractor**: Extract method definitions and calls from Rails controllers, models, and services
- **Ruby symbol extractor**: Enable `memory_symbols` to find Ruby functions/methods
- **Fix CFG entry format mismatch**: Update SQL query and flow builder to handle Ruby's `#` separator in controller#action names
- **Benchmark tests**: Add Ruby/Rails extraction benchmarks
- **Documentation updates**: Add Ruby/Rails examples to README.md

### Deferred to v2
- Full call graph with concerns, mixins, ActiveRecord dynamic methods
- `before_action`/`after_action` callback extraction
- Block/yield resolution
- Metaprogramming handling (`method_missing`, `define_method`, `send`)

## Capabilities

### New Capabilities
- `ruby-cfg-extraction`: Control flow graph extraction for Ruby methods (if/else, loops, exception handling)
- `ruby-call-graph`: Method definition and call extraction from Ruby classes (controllers, models, services)
- `ruby-symbol-extraction`: Symbol search for Ruby functions/methods via `memory_symbols`

### Modified Capabilities
- `flow-visualization`: Extend to support Ruby/Rails controller-to-service-to-model flows
- `sequence-diagrams`: Render Rails request flows with controller actions and service calls

## Impact

- **Code affected**:
  - `internal/graph/ruby_cflow.go` — new: Ruby CFG extractor implementing `ControlFlowExtractor` interface
  - `internal/graph/ruby_extractor.go` — new: Ruby call graph extractor implementing `Extractor` interface
  - `internal/graph/registry.go` — register new Ruby extractors
  - `internal/storage/sqlc/flowcharts.sql` — update `GetFunctionFlowchartByHandler` to handle `#` separator
  - `internal/flow/cfg_loader.go` — update `lastDottedSegment` or add Ruby-aware name resolution
  - `internal/symbol/ruby_extractor.go` — new: Ruby symbol extractor
  - `benchmarks/rails/` — new: Rails extraction benchmarks
  - `README.md` — document Ruby/Rails support with examples

- **Dependencies**: Uses existing `gotreesitter` library with Ruby grammar (already vendored)

- **Performance**: Ruby parsing is CPU-bound but runs at index time (watcher). No runtime impact on search/query latency.

- **Known Limitations (v1)**:
  - Ruby call graphs will be sparser than JS/TS due to dynamic dispatch
  - `before_action`/`after_action` callbacks not captured as middleware edges
  - ActiveRecord dynamic methods (`find_by_*`, `where`) not fully resolved
  - Metaprogramming (`method_missing`, `define_method`) not handled

- **Risk**: Main risk is Ruby's dynamic metaprogramming (method_missing, eval) which static analysis cannot fully resolve. Mitigation: focus on standard Rails patterns (controller actions, service objects, model callbacks) rather than trying to parse all Ruby idioms. Set clear expectations in documentation about v1 limitations.
