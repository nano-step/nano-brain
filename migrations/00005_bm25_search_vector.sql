-- +goose Up
-- Add tsvector column for BM25 full-text search
ALTER TABLE chunks ADD COLUMN search_vector tsvector;

-- Create GIN index for fast full-text search
CREATE INDEX idx_chunks_search_vector ON chunks USING GIN (search_vector);

-- Populate tsvector for existing chunks
UPDATE chunks SET search_vector = to_tsvector('english', content);

-- Create trigger to auto-update tsvector on insert/update
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', NEW.content);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_chunks_search_vector
    BEFORE INSERT OR UPDATE OF content ON chunks
    FOR EACH ROW
    EXECUTE FUNCTION chunks_search_vector_update();

-- +goose Down
DROP TRIGGER IF EXISTS trg_chunks_search_vector ON chunks;
DROP FUNCTION IF EXISTS chunks_search_vector_update;
DROP INDEX IF EXISTS idx_chunks_search_vector;
ALTER TABLE chunks DROP COLUMN IF EXISTS search_vector;
