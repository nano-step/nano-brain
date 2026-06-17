-- +goose Up
CREATE TABLE function_flowcharts (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT        NOT NULL,
    entry          TEXT        NOT NULL,   -- "relfile::funcName"
    source_file    TEXT        NOT NULL,
    start_line     INTEGER     NOT NULL,
    end_line       INTEGER     NOT NULL,
    status         TEXT        NOT NULL DEFAULT 'complete'
        CHECK (status IN ('complete', 'truncated', 'parse_error', 'unsupported')),
    cfg            JSONB       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_function_flowchart UNIQUE (workspace_hash, source_file, start_line, end_line)
);

CREATE INDEX idx_function_flowcharts_entry ON function_flowcharts(workspace_hash, entry);
CREATE INDEX idx_function_flowcharts_file  ON function_flowcharts(workspace_hash, source_file);

-- +goose Down
DROP TABLE IF EXISTS function_flowcharts;
