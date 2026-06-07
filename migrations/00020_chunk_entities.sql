-- +goose Up
CREATE TABLE IF NOT EXISTS chunk_entities (
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    entity_name TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    workspace_hash TEXT NOT NULL,
    PRIMARY KEY (chunk_id, entity_name)
);
CREATE INDEX IF NOT EXISTS idx_chunk_entities_name ON chunk_entities(entity_name, workspace_hash);
CREATE INDEX IF NOT EXISTS idx_chunk_entities_chunk ON chunk_entities(chunk_id);

-- +goose Down
DROP TABLE IF EXISTS chunk_entities;
