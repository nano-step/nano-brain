## Why

The Ruby call graph extractor (PR #467) only resolves same-file calls, producing 2-node flows (`entry → handler`). This is significantly weaker than JS/TS which shows 5-10+ node chains through services, repositories, and models. Rails developers need to see what a controller action actually does — the controller → service → model chain.

## What Changes

- **Enrich Ruby `contains` query**: Capture class/module definitions (currently only captures methods)
- **Class→file index**: Build an in-memory lookup from contains edges after extraction
- **Post-extraction resolver**: Rewrite bare call targets to qualified `file.rb::method` format
- **ActiveRecord awareness**: Recognize `ClassName.method` patterns and resolve via class index
- **Integration test fixtures**: Multi-file Ruby controller + service + model test fixtures

### Deferred to v2
- ActiveRecord dynamic finders (`find_by_*`, `where`, `create!`) as fully resolved edges
- Concern/extend/include resolution
- `before_action`/`after_action` callback chains

## Capabilities

### Modified Capabilities
- `ruby-call-graph`: Now resolves cross-file calls (controller→service→model)
- `flow-visualization`: Rails flows show 5-10+ node chains instead of 2-node entry→handler

## Impact

- **Code affected**:
  - `internal/graph/ruby_cflow.go` — enrich `contains` tree-sitter query
  - `internal/graph/ruby_resolver.go` — new: cross-file call resolver
  - `internal/graph/ruby_extractor.go` — emit qualified TargetNode for resolved calls
  - `internal/graph/ruby_extractor_test.go` — cross-file resolution tests
  - `internal/graph/testdata/ruby/` — multi-file controller/service/model fixtures
  - `internal/watcher/watcher.go` — wire resolver into extraction pipeline

- **Dependencies**: Existing `contains` edges + symbol extractor output

- **Performance**: Resolution pass is O(calls) with hash index. Negligible overhead at index time.

- **Risk**: Ruby class reopening (multiple files defining same class) creates ambiguity. Mitigation: metadata flag for ambiguous resolutions.
