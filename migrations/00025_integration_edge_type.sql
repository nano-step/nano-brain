-- +goose Up
ALTER TABLE graph_edges DROP CONSTRAINT graph_edges_edge_type_check;
ALTER TABLE graph_edges ADD CONSTRAINT graph_edges_edge_type_check
    CHECK (edge_type IN ('contains', 'imports', 'calls', 'references', 'http', 'middleware', 'integration'));

-- +goose Down
-- NOTE: This will fail if rows with edge_type = 'integration' exist.
-- Operator must DELETE them first via:
--   DELETE FROM graph_edges WHERE edge_type = 'integration';
ALTER TABLE graph_edges DROP CONSTRAINT graph_edges_edge_type_check;
ALTER TABLE graph_edges ADD CONSTRAINT graph_edges_edge_type_check
    CHECK (edge_type IN ('contains', 'imports', 'calls', 'references', 'http', 'middleware'));
