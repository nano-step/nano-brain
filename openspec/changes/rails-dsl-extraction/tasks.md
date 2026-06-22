## 1. Node Format Change

- [ ] 1.1 Add `extractEnclosingClass` helper to `ruby_extractor.go` — walk AST upward from method node to find enclosing class/module name
- [ ] 1.2 Modify `extractCalls` to set `SourceNode = "file.rb::ClassName#methodName"` (with class name) instead of `"file.rb::methodName"`
- [ ] 1.3 Modify `extractContains` to emit class-qualified method targets: `"file.rb::ClassName#method"` instead of `"file.rb::method"`
- [ ] 1.4 Update `rubyGraphContainsQuery` to capture enclosing class context for methods
- [ ] 1.5 Handle nested classes/modules: `Api::V1::UsersController#create` format
- [ ] 1.6 Handle methods outside any class (top-level methods): use file name as class surrogate
- [ ] 1.7 Add test fixtures for node format: controller with class methods, model with scopes, nested modules
- [ ] 1.8 Update existing unit tests to expect new node format
- [ ] 1.9 Verify same-file call deduplication still works with new format

## 2. Fix Cross-File Resolver

- [ ] 2.1 Rewrite `ruby_resolver.go` to use tree-sitter AST scanning instead of regex
- [ ] 2.2 Add `resolveFileAST` method: parse file, walk all `call` nodes, extract receiver + method
- [ ] 2.3 Handle bare method calls: `user.save` → look up User class, emit qualified edge
- [ ] 2.4 Handle chained calls: `User.where(active: true).order(:name)` → resolve each segment
- [ ] 2.5 Handle method calls in blocks: `users.each { |u| u.save }` → resolve via variable type inference (best-effort)
- [ ] 2.6 Keep `BuildReconcileEdges` working with new node format
- [ ] 2.7 Add test fixtures for bare call resolution: controller calling model methods, service methods
- [ ] 2.8 Add unit tests for resolver: resolve, unresolvable, ambiguous cases

## 3. Singleton Method CFG Support

- [ ] 3.1 Add `(singleton_method ...)` to `walkNodes` call in `ruby_cflow.go` `ExtractCFGs`
- [ ] 3.2 Handle `def self.method_name` in CFG builder — extract method name correctly
- [ ] 3.3 Add test fixtures: class method with if/else, scope with query chain
- [ ] 3.4 Add unit tests for singleton method CFG extraction

## 4. Expand Convention Path Map

- [ ] 4.1 Add `*Service` → `app/services/` path mapping to `railsConventionPath()`
- [ ] 4.2 Add `*Job` → `app/jobs/` path mapping
- [ ] 4.3 Add `*Worker` → `app/workers/` path mapping
- [ ] 4.4 Add `*Mailer` → `app/mailers/` path mapping
- [ ] 4.5 Add `*Policy` → `app/policies/` path mapping
- [ ] 4.6 Add `*Serializer` → `app/serializers/` path mapping
- [ ] 4.7 Add unit tests for expanded convention paths

## 5. Rails DSL Edge Extractor

- [ ] 5.1 Create `internal/graph/rails_dsl_extractor.go` with `RailsDSLEdgeExtractor` struct
- [ ] 5.2 Implement `Supports(ext)` → `.rb` only
- [ ] 5.3 Implement `RequiresFrameworks()` → `["rails"]`
- [ ] 5.4 Define association method set: `has_many`, `has_one`, `belongs_to`, `has_and_belongs_to_many`
- [ ] 5.5 Define callback method set: `before_action`, `after_action`, `after_commit`, `before_save`, `after_save`, `before_create`, `after_create`, `before_update`, `after_update`, `before_destroy`, `after_destroy`
- [ ] 5.6 Define concern method set: `include`, `extend`
- [ ] 5.7 Implement association extraction: tree-sitter query for call nodes with association method names
- [ ] 5.8 Implement callback extraction: tree-sitter query for call nodes with callback method names
- [ ] 5.9 Implement concern extraction: tree-sitter query for `include`/`extend` with constant arguments
- [ ] 5.10 Implement Sidekiq detection: `perform_async`/`perform_in` calls → EdgeIntegration
- [ ] 5.11 Add metadata to each edge type (dsl, type, target)
- [ ] 5.12 Create `internal/graph/rails_dsl_extractor_test.go` with unit tests
- [ ] 5.13 Add test fixtures: model with associations, controller with callbacks, service with concerns

## 6. Registration and Wiring

- [ ] 6.1 Register `RailsDSLEdgeExtractor` in `cmd/nano-brain/main.go` alongside existing Ruby extractors
- [ ] 6.2 Verify `SetActiveFrameworks` filters correctly for Rails-only extractors
- [ ] 6.3 Update watcher to use new AST-based resolver (replace regex resolver)
- [ ] 6.4 Verify resolver runs after DSL extraction (correct edge ordering)

## 7. Integration Tests

- [ ] 7.1 Test controller→model chain: `POST /users → UsersController#create → User.create`
- [ ] 7.2 Test association visibility: `User has_many Orders` edge appears in graph
- [ ] 7.3 Test callback chain: `before_action → set_user → @user = User.find(params[:id])`
- [ ] 7.4 Test concern inclusion: `include Authenticatable` edge appears
- [ ] 7.5 Test Sidekiq: `OrderProcessor.perform_async(order.id)` edge appears
- [ ] 7.6 Test flow diagram: controller action shows 5+ nodes through associations
- [ ] 7.7 Test trace: `User#save` shows callback chain
- [ ] 7.8 Verify no false positives on non-Rails Ruby

## 8. Validation

- [ ] 8.1 Run `go build ./... && go test -race -short ./...`
- [ ] 8.2 Run harness validation ladder
- [ ] 8.3 Run smoke:e2e with test server on port 3199
- [ ] 8.4 Verify Ruby flow diagrams show 5+ node chains
- [ ] 8.5 Document results in PR description
