-- +goose Up
ALTER TABLE chunks ADD COLUMN symbol_name TEXT;
ALTER TABLE chunks ADD COLUMN symbol_kind TEXT;
ALTER TABLE chunks ADD COLUMN language TEXT;
ALTER TABLE chunks ADD COLUMN line_start INTEGER;
ALTER TABLE chunks ADD COLUMN line_end INTEGER;
ALTER TABLE chunks ADD COLUMN chunk_type TEXT NOT NULL DEFAULT 'raw';
ALTER TABLE chunks ADD COLUMN embedding_strategy TEXT NOT NULL DEFAULT 'raw_code';

UPDATE chunks SET chunk_type = 'raw' WHERE chunk_type IS NULL;
UPDATE chunks SET embedding_strategy = 'raw_code' WHERE embedding_strategy IS NULL;

CREATE INDEX idx_chunks_symbol_name ON chunks (symbol_name) WHERE symbol_name IS NOT NULL;
CREATE INDEX idx_chunks_chunk_type ON chunks (chunk_type);

-- +goose Down
DROP INDEX IF EXISTS idx_chunks_chunk_type;
DROP INDEX IF EXISTS idx_chunks_symbol_name;

ALTER TABLE chunks DROP COLUMN IF EXISTS embedding_strategy;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_type;
ALTER TABLE chunks DROP COLUMN IF EXISTS line_end;
ALTER TABLE chunks DROP COLUMN IF EXISTS line_start;
ALTER TABLE chunks DROP COLUMN IF EXISTS language;
ALTER TABLE chunks DROP COLUMN IF EXISTS symbol_kind;
ALTER TABLE chunks DROP COLUMN IF EXISTS symbol_name;
