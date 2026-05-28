-- +goose Up
-- Drop incorrect unique constraint on (content_hash, workspace_hash).
-- Two different files can have identical content (e.g. multiple .openspec.yaml files).
-- Document identity is (source_path, workspace_hash), already enforced by
-- uq_documents_source_path_workspace index (migration 00003).
ALTER TABLE documents DROP CONSTRAINT IF EXISTS uq_documents_content_hash_workspace;

-- +goose Down
ALTER TABLE documents ADD CONSTRAINT uq_documents_content_hash_workspace UNIQUE (content_hash, workspace_hash);
