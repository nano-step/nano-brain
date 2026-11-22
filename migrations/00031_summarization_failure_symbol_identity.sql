-- +goose Up
DROP INDEX idx_csf_workspace_content_unresolved;

CREATE UNIQUE INDEX idx_csf_workspace_symbol_unresolved
    ON code_summarization_failures(workspace_hash, content_hash, source_file, symbol_name)
    WHERE resolved_at IS NULL;

-- +goose Down
DROP INDEX idx_csf_workspace_symbol_unresolved;

-- Rolling back after distinct same-hash symbols fail will reject this index
-- rather than silently discard a failure record.
CREATE UNIQUE INDEX idx_csf_workspace_content_unresolved
    ON code_summarization_failures(workspace_hash, content_hash)
    WHERE resolved_at IS NULL;
