-- +goose Up
CREATE TABLE code_summarization_failures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT NOT NULL,
    symbol_name TEXT NOT NULL,
    symbol_kind TEXT,
    source_file TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    error_reason TEXT NOT NULL,
    error_type TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_csf_workspace_unresolved
    ON code_summarization_failures(workspace_hash)
    WHERE resolved_at IS NULL;

CREATE UNIQUE INDEX idx_csf_workspace_content_unresolved
    ON code_summarization_failures(workspace_hash, content_hash)
    WHERE resolved_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS code_summarization_failures;
