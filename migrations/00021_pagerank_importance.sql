-- +goose Up
CREATE TABLE pagerank_scores (
    workspace_hash TEXT NOT NULL,
    node_name      TEXT NOT NULL,
    score          DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    computed_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_hash, node_name)
);

CREATE INDEX idx_pagerank_scores_workspace ON pagerank_scores(workspace_hash);

-- +goose Down
DROP TABLE IF EXISTS pagerank_scores;
