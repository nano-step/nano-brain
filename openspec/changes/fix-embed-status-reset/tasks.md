## 1. SQL Query Modification

- [x] 1.1 Modify `UpsertChunk` query in `internal/storage/queries/chunks.sql` to conditionally reset `embed_status` based on content comparison

## 2. Regenerate sqlc Code

- [x] 2.1 Run `sqlc generate` to regenerate Go code from updated SQL queries

## 3. Verification

- [x] 3.1 Verify that existing tests pass with the modified query
- [x] 3.2 Test that unchanged chunks preserve their `embed_status` during upsert
- [x] 3.3 Test that changed chunks get `embed_status` reset to `'pending'`
