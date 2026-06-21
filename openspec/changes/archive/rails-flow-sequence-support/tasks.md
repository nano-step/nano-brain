## 1. CFG Entry Format Fix

- [ ] 1.1 Audit `internal/flow/cfg_loader.go` for hardcoded `::` separator assumptions
- [ ] 1.2 Update `GetFunctionFlowchartByHandler` SQL to handle `#` separator in Ruby handler names
- [ ] 1.3 Update `lastDottedSegment` or add Ruby-aware name resolution in cfg_loader
- [ ] 1.4 Write failing integration test for Ruby CFG entry matching
- [ ] 1.5 Verify existing JS/TS CFG tests still pass after SQL changes

## 2. Ruby CFG Extractor

- [ ] 2.1 Create `internal/graph/ruby_cflow.go` with RubyControlFlowExtractor struct
- [ ] 2.2 Implement SupportsCFG method (return true for ".rb")
- [ ] 2.3 Implement ExtractCFGs method using gotreesitter Ruby grammar
- [ ] 2.4 Implement control flow extraction for if/else statements
- [ ] 2.5 Implement control flow extraction for loops (while, until, for)
- [ ] 2.6 Implement control flow extraction for begin/rescue blocks
- [ ] 2.7 Add maxCFGNodes limit enforcement with truncation
- [ ] 2.8 Set Entry format to `file.rb::ControllerName#action` for Rails controllers
- [ ] 2.9 Create `internal/graph/ruby_cflow_test.go` with unit tests
- [ ] 2.10 Add test fixtures in `internal/graph/testdata/ruby/` directory

## 3. Ruby Call Graph Extractor

- [ ] 3.1 Create `internal/graph/ruby_extractor.go` with RubyExtractor struct
- [ ] 3.2 Implement Supports method (return true for ".rb")
- [ ] 3.3 Implement RequiresFrameworks method (return ["rails"])
- [ ] 3.4 Implement ExtractEdges for method definitions (contains edges)
- [ ] 3.5 Implement ExtractEdges for method calls (calls edges) — same-file only for v1
- [ ] 3.6 Add Language="ruby" metadata to all extracted edges
- [ ] 3.7 Create `internal/graph/ruby_extractor_test.go` with unit tests
- [ ] 3.8 Add test fixtures for Ruby controller, model, service patterns

## 4. Ruby Symbol Extractor

- [ ] 4.1 Create `internal/symbol/ruby_extractor.go` with RubySymbolExtractor struct
- [ ] 4.2 Implement extraction for method definitions (function, method kinds)
- [ ] 4.3 Implement extraction for class/module definitions (type kind)
- [ ] 4.4 Register RubySymbolExtractor in symbol registry
- [ ] 4.5 Create `internal/symbol/ruby_extractor_test.go` with unit tests

## 5. Flow Builder Fixes

- [ ] 5.1 Add "model" to `classifyRole` in `internal/flow/builder.go` (RoleRepo check)
- [ ] 5.2 Update `participantLabel` in `internal/flow/sequence.go` to strip `#method` from Ruby node names
- [ ] 5.3 Add Ruby logging patterns to `isNoiseExternal` in `internal/flow/builder.go` (Rails.logger, puts, p)
- [ ] 5.4 Add Ruby assignment patterns to `simplifyStepLabel` in `internal/flow/sequence.go` (@var, var =)

## 6. Registry Integration

- [ ] 6.1 Register RubyControlFlowExtractor in `internal/graph/registry.go`
- [ ] 6.2 Register RubyExtractor in `internal/graph/registry.go`
- [ ] 6.3 Verify Ruby extraction works with existing test suite

## 7. Flow Builder Integration

- [ ] 7.1 Test flow builder with Ruby edges (controller → service → model)
- [ ] 7.2 Verify Ruby node naming format ("ClassName#method_name")
- [ ] 7.3 Test role classification for Ruby nodes
- [ ] 7.4 Test flow materialization for Rails routes
- [ ] 7.5 Test sequence diagram rendering with Ruby flows

## 8. Benchmarks

- [ ] 8.1 Create `benchmarks/rails/` directory structure
- [ ] 8.2 Add route extraction accuracy benchmark (parse routes.rb, verify edge count)
- [ ] 8.3 Add CFG extraction completeness benchmark (parse controller, verify node/edge count)
- [ ] 8.4 Add flow builder end-to-end benchmark (entry → full flow)
- [ ] 8.5 Add comparison benchmark: Ruby vs JS/TS extraction speed
- [ ] 8.6 Run benchmarks against Phil-timeshel Rails project
- [ ] 8.7 Document benchmark results in PR description

## 9. Documentation

- [ ] 9.1 Update README.md with Ruby/Rails support section
- [ ] 9.2 Add example Ruby flow diagram to documentation
- [ ] 9.3 Add example Ruby sequence diagram to documentation
- [ ] 9.4 Document v1 limitations (dynamic metaprogramming, before_action, AR dynamic methods)

## 10. Integration Testing

- [ ] 10.1 Test with Phil-timeshel Rails project (routes.rb + controllers + models)
- [ ] 10.2 Test with at least one additional Rails project
- [ ] 10.3 Verify end-to-end flow: extract → build → render
- [ ] 10.4 Run full test suite to ensure no regressions
- [ ] 10.5 Run validation ladder: `go build ./... && go test -race -short ./...`
- [ ] 10.6 Run smoke:e2e test with Ruby flow/sequence endpoints
