-- +goose Up
CREATE UNIQUE INDEX uq_documents_source_path_workspace ON documents(source_path, workspace_hash) WHERE source_path != '';

-- +goose Down
DROP INDEX IF EXISTS uq_documents_source_path_workspace;
