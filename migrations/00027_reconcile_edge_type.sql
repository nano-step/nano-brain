-- +goose Up
ALTER TABLE graph_edges DROP CONSTRAINT graph_edges_edge_type_check;
ALTER TABLE graph_edges ADD CONSTRAINT graph_edges_edge_type_check
    CHECK (edge_type IN ('contains', 'imports', 'calls', 'references', 'http', 'middleware', 'integration', 'reconcile'));

-- +goose Down
ALTER TABLE graph_edges DROP CONSTRAINT graph_edges_edge_type_check;
ALTER TABLE graph_edges ADD CONSTRAINT graph_edges_edge_type_check
    CHECK (edge_type IN ('contains', 'imports', 'calls', 'references', 'http', 'middleware', 'integration'));
