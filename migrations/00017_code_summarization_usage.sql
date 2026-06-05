-- +goose Up
CREATE TABLE IF NOT EXISTS code_summarization_usage (
    workspace_hash TEXT NOT NULL,
    usage_date DATE NOT NULL DEFAULT CURRENT_DATE,
    request_count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (workspace_hash, usage_date)
);

-- +goose Down
DROP TABLE IF EXISTS code_summarization_usage;
