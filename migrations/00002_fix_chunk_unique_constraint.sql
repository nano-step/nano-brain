-- +goose Up
ALTER TABLE chunks DROP CONSTRAINT IF EXISTS uq_chunks_content_hash_workspace_hash;
ALTER TABLE chunks ADD CONSTRAINT uq_chunks_content_hash_workspace_hash_document_id UNIQUE (content_hash, workspace_hash, document_id);

-- +goose Down
ALTER TABLE chunks DROP CONSTRAINT IF EXISTS uq_chunks_content_hash_workspace_hash_document_id;
ALTER TABLE chunks ADD CONSTRAINT uq_chunks_content_hash_workspace_hash UNIQUE (content_hash, workspace_hash);
