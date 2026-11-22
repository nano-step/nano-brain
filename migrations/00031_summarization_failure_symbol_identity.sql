-- +goose Up
DROP INDEX idx_csf_workspace_content_unresolved;

-- The target encoding is fixed, so this wrapper is immutable even though
-- convert_to itself is classified STABLE by PostgreSQL.
-- +goose StatementBegin
CREATE FUNCTION code_summarization_identity_sha256(value TEXT)
RETURNS BYTEA LANGUAGE sql IMMUTABLE STRICT
AS $$ SELECT sha256(convert_to(value, 'UTF8')) $$;
-- +goose StatementEnd

-- Source paths and symbol names can exceed PostgreSQL's btree tuple limit.
CREATE UNIQUE INDEX idx_csf_workspace_symbol_unresolved
    ON code_summarization_failures (
        workspace_hash, content_hash,
        (code_summarization_identity_sha256(source_file)),
        (code_summarization_identity_sha256(symbol_name))
    )
    WHERE resolved_at IS NULL;

-- +goose Down
DROP INDEX idx_csf_workspace_symbol_unresolved;
DROP FUNCTION code_summarization_identity_sha256(TEXT);

-- Rolling back after distinct same-hash symbols fail will reject this index
-- rather than silently discard a failure record.
CREATE UNIQUE INDEX idx_csf_workspace_content_unresolved
    ON code_summarization_failures(workspace_hash, content_hash)
    WHERE resolved_at IS NULL;
