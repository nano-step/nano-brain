-- +goose Up
ALTER TABLE graph_edges DROP CONSTRAINT graph_edges_edge_type_check;
ALTER TABLE graph_edges ADD CONSTRAINT graph_edges_edge_type_check
    CHECK (edge_type IN ('contains', 'imports', 'calls', 'references'));

-- +goose Down
-- NOTE: This will fail if rows with edge_type='references' exist.
-- Operator must DELETE them first via:
--   DELETE FROM graph_edges WHERE edge_type = 'references';
ALTER TABLE graph_edges DROP CONSTRAINT graph_edges_edge_type_check;
ALTER TABLE graph_edges ADD CONSTRAINT graph_edges_edge_type_check
    CHECK (edge_type IN ('contains', 'imports', 'calls'));
