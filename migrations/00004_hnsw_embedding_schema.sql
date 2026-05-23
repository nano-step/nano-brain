-- +goose Up

-- Add embed_status to chunks table (tracks embedding pipeline state)
ALTER TABLE chunks ADD COLUMN embed_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (embed_status IN ('pending', 'embedded', 'embed_failed'));

-- Add updated_at to embeddings table
ALTER TABLE embeddings ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Add unique constraint on chunk_id (required for ON CONFLICT in InsertEmbedding)
ALTER TABLE embeddings ADD CONSTRAINT embeddings_chunk_id_unique UNIQUE (chunk_id);

-- Create HNSW index for cosine similarity search
-- Using vector_cosine_ops for cosine distance (1 - cosine_similarity)
CREATE INDEX idx_embeddings_hnsw ON embeddings
    USING hnsw (embedding vector_cosine_ops)
    WITH (m = 16, ef_construction = 64);

-- +goose Down
DROP INDEX IF EXISTS idx_embeddings_hnsw;
ALTER TABLE embeddings DROP CONSTRAINT IF EXISTS embeddings_chunk_id_unique;
ALTER TABLE embeddings DROP COLUMN IF EXISTS updated_at;
ALTER TABLE chunks DROP COLUMN IF EXISTS embed_status;
