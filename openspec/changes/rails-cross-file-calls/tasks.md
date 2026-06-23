## 1. Contains Edge Enrichment

- [ ] 1.1 Add `(class name: (constant) @name)` and `(module name: (constant) @name)` to Ruby tree-sitter contains query in `ruby_extractor.go`
- [ ] 1.2 Walk AST upward from method nodes to capture enclosing class/module chain
- [ ] 1.3 Update contains edge extraction to produce class/module edges with TargetNode=`file.rb::ClassName`
- [ ] 1.4 Add test fixtures for class/module extraction (controller, model, service, nested classes)
- [ ] 1.5 Add unit tests for enriched contains extraction
- [ ] 1.6 Verify existing same-file tests still pass

## 2. Class→File Index

- [ ] 2.1 Create `internal/graph/ruby_class_index.go` with RubyClassIndex struct
- [ ] 2.2 Implement BuildClassIndex from contains edges (class name → file path mapping)
- [ ] 2.3 Handle module nesting (Api::V1::TokensController → TokensController)
- [ ] 2.4 Handle class reopening (multiple files defining same class)
- [ ] 2.5 Add Rails naming convention fallback (User → app/models/user.rb)
- [ ] 2.6 Create `internal/graph/ruby_class_index_test.go` with unit tests

## 3. Cross-File Resolver

- [ ] 3.1 Create `internal/graph/ruby_resolver.go` with RubyCrossFileResolver struct
- [ ] 3.2 Implement ResolveCalls: rewrite bare call targets using class index
- [ ] 3.3 Handle ClassName.method pattern (resolve class, then method)
- [ ] 3.4 Handle ClassName.new.method pattern (two-step resolution)
- [ ] 3.5 Emit qualified TargetNode (`file.rb::method`) for resolved calls
- [ ] 3.6 Emit bare TargetNode + metadata `{"unresolved": true}` for unresolvable calls
- [ ] 3.7 Add metadata `{"ambiguous": true}` when class resolves to multiple files
- [ ] 3.8 Handle ActiveRecord class-level methods (User.where, User.create, etc.)

## 4. FlowBuilder Reconcile Edges (Momus finding)

- [ ] 4.1 Add `EdgeReconcile` to EdgeKind enum in `edge.go`
- [ ] 4.2 Flow builder: treat reconcile edges as transparent pass-through in BFS
- [ ] 4.3 Resolver: emit reconcile edge for each controller action (Controller#action → file.rb::method)
- [ ] 4.4 Add unit tests for reconcile edge traversal in flow builder
- [ ] 4.5 Verify BFS walks: HTTP entry → reconcile → calls chain → service → model

## 5. Watcher Integration

- [ ] 5.1 Create `resolveRubyEdges(ctx, workspaceHash)` function
- [ ] 5.2 Wire resolver into watcher after scanCollection completes
- [ ] 5.3 Build class index from accumulated contains edges
- [ ] 5.4 Run resolver on all extracted edges, re-upsert resolved edges
- [ ] 5.5 Verify edges are persisted with qualified TargetNode and reconcile edges

## 6. Multi-File Test Fixtures

- [ ] 6.1 Create `testdata/ruby/multi_file/controller.rb` (UsersController with calls to User model and PaymentService)
- [ ] 6.2 Create `testdata/ruby/multi_file/user.rb` (User model with AR methods)
- [ ] 6.3 Create `testdata/ruby/multi_file/payment_service.rb` (service with business logic)
- [ ] 6.4 Create `testdata/ruby/multi_file/order.rb` (Order model)
- [ ] 6.5 Create `testdata/ruby/multi_file/routes.rb` (routes referencing controllers)

## 7. Integration Tests

- [ ] 7.1 Test controller→model resolution (UsersController → User.where → user.rb)
- [ ] 7.2 Test controller→service resolution (UsersController → PaymentService.new → payment_service.rb)
- [ ] 7.3 Test unresolved call fallback (external gem call → bare edge + metadata)
- [ ] 7.4 Test ambiguous class resolution (multiple files → edges to all + metadata)
- [ ] 7.5 Test reconcile edge traversal (HTTP entry → reconcile → calls chain)
- [ ] 7.6 Test end-to-end flow: extract → index → resolve → flow builder
- [ ] 7.7 Verify rails-app flows show 5+ nodes per controller action

## 8. Validation

- [ ] 8.1 Run `go build ./... && go test -race -short ./...`
- [ ] 8.2 Run harness validation ladder
- [ ] 8.3 Run smoke:e2e with test server on port 3199
- [ ] 8.4 Verify rails-app sequence diagrams show controller→service→model chains
- [ ] 8.5 Document results in PR description
