-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hash TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_workspaces_hash ON workspaces(hash);

CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    collection TEXT NOT NULL DEFAULT 'default',
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_documents_content_hash_workspace UNIQUE (content_hash, workspace_hash)
);
CREATE INDEX idx_documents_workspace_hash ON documents(workspace_hash);

CREATE TABLE chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    workspace_hash TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    content TEXT NOT NULL,
    chunk_index INT NOT NULL DEFAULT 0,
    start_line INT,
    end_line INT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_chunks_content_hash_workspace_hash UNIQUE (content_hash, workspace_hash)
);
CREATE INDEX idx_chunks_document_id ON chunks(document_id);
CREATE INDEX idx_chunks_workspace_hash ON chunks(workspace_hash);

CREATE TABLE embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    workspace_hash TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT 'ollama',
    model TEXT NOT NULL DEFAULT '',
    embedding vector(768),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_embeddings_chunk_id ON embeddings(chunk_id);
CREATE INDEX idx_embeddings_workspace_hash ON embeddings(workspace_hash);

CREATE TABLE collections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL,
    glob_pattern TEXT NOT NULL DEFAULT '**/*',
    update_mode TEXT NOT NULL DEFAULT 'auto',
    exclude_patterns TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_collections_name_workspace UNIQUE (name, workspace_hash)
);
CREATE INDEX idx_collections_workspace_hash ON collections(workspace_hash);

CREATE TABLE telemetry_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_telemetry_logs_workspace_hash ON telemetry_logs(workspace_hash);

-- +goose Down
DROP TABLE IF EXISTS telemetry_logs;
DROP TABLE IF EXISTS embeddings;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS collections;
DROP TABLE IF EXISTS workspaces;
DROP EXTENSION IF EXISTS vector;
