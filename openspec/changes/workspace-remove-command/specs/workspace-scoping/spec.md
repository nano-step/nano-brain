## MODIFIED Requirements

### Requirement: Workspace data isolation

All workspace-scoped data SHALL be deletable by project_hash. The store SHALL provide a `removeWorkspace(projectHash)` method that deletes all rows from every workspace-scoped table (documents, content, content_vectors, vectors_vec, llm_cache, file_edges, symbols, code_symbols, symbol_edges, execution_flows, flow_steps, document_tags) in a single atomic transaction. This method is separate from the existing `clearWorkspace()` which handles only document-level data.

#### Scenario: removeWorkspace deletes from all tables
- **WHEN** `store.removeWorkspace(projectHash)` is called
- **THEN** all rows with the given project_hash are deleted from: flow_steps (via cascade), execution_flows, symbol_edges, code_symbols, symbols, file_edges, document_tags (via cascade from documents), documents, content_vectors, vectors_vec, llm_cache, and orphaned content rows
- **THEN** the method returns a summary object with deletion counts per table

#### Scenario: removeWorkspace is atomic
- **WHEN** `store.removeWorkspace(projectHash)` encounters an error mid-deletion
- **THEN** no rows are deleted from any table (transaction rollback)
