-- +goose Up
CREATE TABLE graph_edges (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT        NOT NULL,
    source_node    TEXT        NOT NULL,
    target_node    TEXT        NOT NULL,
    edge_type      TEXT        NOT NULL CHECK (edge_type IN ('contains', 'imports', 'calls')),
    source_file    TEXT        NOT NULL,
    metadata       JSONB       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_graph_edge UNIQUE (workspace_hash, source_node, target_node, edge_type)
);

CREATE INDEX idx_graph_edges_source ON graph_edges(workspace_hash, source_node, edge_type);
CREATE INDEX idx_graph_edges_target ON graph_edges(workspace_hash, target_node, edge_type);
CREATE INDEX idx_graph_edges_file   ON graph_edges(workspace_hash, source_file);

-- +goose Down
DROP TABLE IF EXISTS graph_edges;
