## 1. Fix Association Edge Extraction

- [ ] 1.1 Modify `extractAssociation` to resolve symbol arguments to model class names (capitalize + singularize)
- [ ] 1.2 Add `singularize` helper function for Rails inflection rules
- [ ] 1.3 Add `capitalize` helper for class name formatting
- [ ] 1.4 Update association edge metadata to include original symbol for debugging
- [ ] 1.5 Add unit tests for association resolution (has_many, belongs_to, has_one, has_and_belongs_to_many)

## 2. Fix Callback Edge Extraction

- [ ] 2.1 Modify `extractCallback` to qualify method names with controller class
- [ ] 2.2 Update callback edge target format from `:method_name` to `ClassName#method_name`
- [ ] 2.3 Add unit tests for callback resolution (before_action, after_action, before_save, etc.)

## 3. Fix Memory Flowchart for Ruby

- [ ] 3.1 Update `memory_flowchart` tool to accept `file.rb::ClassName#method` format
- [ ] 3.2 Add format detection logic (check for `::` separator and `#` for Ruby methods)
- [ ] 3.3 Update flowchart lookup query to handle Ruby format
- [ ] 3.4 Add unit tests for Ruby flowchart format support

## 4. Verify Graph Edges Exist

- [ ] 4.1 Run re-indexing for Rails workspaces to extract association edges
- [ ] 4.2 Verify `memory_graph` returns associations (has_many, belongs_to)
- [ ] 4.3 Verify `memory_impact` returns results after edges exist
- [ ] 4.4 Test `memory_flow` traces deeper into controller method bodies

## 5. Integration Testing

- [ ] 5.1 Run all graph tool tests against Rails test workspace
- [ ] 5.2 Verify `memory_graph` returns 16,537+ edges for Rails project
- [ ] 5.3 Verify `memory_trace` shows BillingWorker#perform call chain
- [ ] 5.4 Verify `memory_flow` shows POST /users → UsersController#create flow
