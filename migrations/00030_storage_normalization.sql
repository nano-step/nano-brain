-- +goose Up

CREATE TABLE IF NOT EXISTS workspace_ids (
    short_id SERIAL PRIMARY KEY,
    workspace_hash TEXT NOT NULL UNIQUE REFERENCES workspaces(hash) ON DELETE CASCADE
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_workspace_id() RETURNS trigger AS $$
BEGIN
    INSERT INTO workspace_ids (workspace_hash)
    VALUES (NEW.hash)
    ON CONFLICT (workspace_hash) DO NOTHING;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_workspaces_sync_id
    AFTER INSERT ON workspaces
    FOR EACH ROW
    EXECUTE FUNCTION sync_workspace_id();

INSERT INTO workspace_ids (workspace_hash)
SELECT hash FROM workspaces
ON CONFLICT (workspace_hash) DO NOTHING;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION resolve_workspace_short_id(p_hash TEXT) RETURNS INT AS $$
    SELECT short_id FROM workspace_ids WHERE workspace_hash = p_hash;
$$ LANGUAGE sql STABLE;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_workspace_short_id(p_hash TEXT) RETURNS INT AS $$
DECLARE
    v_id INT;
BEGIN
    SELECT short_id INTO v_id FROM workspace_ids WHERE workspace_hash = p_hash;
    IF v_id IS NOT NULL THEN
        RETURN v_id;
    END IF;
    INSERT INTO workspaces (hash, name, path)
    VALUES (p_hash, p_hash, '')
    ON CONFLICT (hash) DO NOTHING;
    INSERT INTO workspace_ids (workspace_hash)
    VALUES (p_hash)
    ON CONFLICT (workspace_hash) DO NOTHING
    RETURNING short_id INTO v_id;
    IF v_id IS NULL THEN
        SELECT short_id INTO v_id FROM workspace_ids WHERE workspace_hash = p_hash;
    END IF;
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS content_blobs (
    id BIGSERIAL PRIMARY KEY,
    sha256_bytes BYTEA NOT NULL UNIQUE,
    content TEXT NOT NULL
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION intern_content_blob(p_content TEXT) RETURNS BIGINT AS $$
DECLARE
    v_sha BYTEA := sha256(convert_to(coalesce(p_content, ''), 'UTF8'));
    v_id BIGINT;
BEGIN
    SELECT id INTO v_id FROM content_blobs WHERE sha256_bytes = v_sha;
    IF v_id IS NOT NULL THEN
        RETURN v_id;
    END IF;
    INSERT INTO content_blobs (sha256_bytes, content)
    VALUES (v_sha, coalesce(p_content, ''))
    ON CONFLICT (sha256_bytes) DO NOTHING
    RETURNING id INTO v_id;
    IF v_id IS NULL THEN
        SELECT id INTO v_id FROM content_blobs WHERE sha256_bytes = v_sha;
    END IF;
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS embedding_vectors (
    id BIGSERIAL PRIMARY KEY,
    blob_id BIGINT NOT NULL REFERENCES content_blobs(id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT 'ollama',
    model TEXT NOT NULL DEFAULT '',
    embedding halfvec(768) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_embedding_vectors_blob_provider_model UNIQUE (blob_id, provider, model)
);

CREATE TABLE IF NOT EXISTS graph_nodes (
    id BIGSERIAL PRIMARY KEY,
    node_key TEXT NOT NULL UNIQUE,
    bare_symbol TEXT GENERATED ALWAYS AS (split_part(node_key, '::', 2)) STORED,
    is_js_ts_qualified BOOLEAN GENERATED ALWAYS AS (
        node_key ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::'
    ) STORED
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION intern_graph_node(p_key TEXT) RETURNS BIGINT AS $$
DECLARE
    v_id BIGINT;
BEGIN
    SELECT id INTO v_id FROM graph_nodes WHERE node_key = p_key;
    IF v_id IS NOT NULL THEN
        RETURN v_id;
    END IF;
    INSERT INTO graph_nodes (node_key)
    VALUES (p_key)
    ON CONFLICT (node_key) DO NOTHING
    RETURNING id INTO v_id;
    IF v_id IS NULL THEN
        SELECT id INTO v_id FROM graph_nodes WHERE node_key = p_key;
    END IF;
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS entity_dictionary (
    id SERIAL PRIMARY KEY,
    entity_name TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    CONSTRAINT uq_entity_dictionary_name_type UNIQUE (entity_name, entity_type)
);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION intern_entity(p_name TEXT, p_type TEXT) RETURNS INT AS $$
DECLARE
    v_id INT;
BEGIN
    SELECT id INTO v_id FROM entity_dictionary WHERE entity_name = p_name AND entity_type = p_type;
    IF v_id IS NOT NULL THEN
        RETURN v_id;
    END IF;
    INSERT INTO entity_dictionary (entity_name, entity_type)
    VALUES (p_name, p_type)
    ON CONFLICT (entity_name, entity_type) DO NOTHING
    RETURNING id INTO v_id;
    IF v_id IS NULL THEN
        SELECT id INTO v_id FROM entity_dictionary WHERE entity_name = p_name AND entity_type = p_type;
    END IF;
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    INSERT INTO content_blobs (sha256_bytes, content)
    SELECT DISTINCT ON (sha) sha, content
    FROM (
        SELECT sha256(convert_to(coalesce(content, ''), 'UTF8')) AS sha, coalesce(content, '') AS content FROM documents
        UNION ALL
        SELECT sha256(convert_to(coalesce(content, ''), 'UTF8')) AS sha, coalesce(content, '') AS content FROM chunks
    ) src
    ON CONFLICT (sha256_bytes) DO NOTHING;

    INSERT INTO embedding_vectors (blob_id, provider, model, embedding, created_at)
    SELECT DISTINCT ON (cb.id, e.provider, e.model)
        cb.id, e.provider, e.model, e.embedding::halfvec(768), e.created_at
    FROM embeddings e
    JOIN chunks c ON c.id = e.chunk_id
    JOIN content_blobs cb ON cb.sha256_bytes = sha256(convert_to(coalesce(c.content, ''), 'UTF8'))
    WHERE e.embedding IS NOT NULL
    ORDER BY cb.id, e.provider, e.model, e.updated_at DESC
    ON CONFLICT (blob_id, provider, model) DO NOTHING;

    INSERT INTO graph_nodes (node_key)
    SELECT DISTINCT node_key FROM (
        SELECT source_node AS node_key FROM graph_edges
        UNION
        SELECT target_node AS node_key FROM graph_edges
        UNION
        SELECT source_file AS node_key FROM graph_edges
    ) gn
    ON CONFLICT (node_key) DO NOTHING;

    INSERT INTO entity_dictionary (entity_name, entity_type)
    SELECT DISTINCT entity_name, entity_type FROM chunk_entities
    ON CONFLICT (entity_name, entity_type) DO NOTHING;

    CREATE TEMP TABLE _mig30_documents ON COMMIT DROP AS
    SELECT d.id, d.workspace_hash, d.content_hash, cb.id AS blob_id, d.title, d.source_path, d.collection,
           d.tags, d.metadata, d.created_at, d.updated_at, d.supersedes_id, d.mod_time, d.file_size
    FROM documents d
    JOIN content_blobs cb ON cb.sha256_bytes = sha256(convert_to(coalesce(d.content, ''), 'UTF8'))
    JOIN workspaces w ON w.hash = d.workspace_hash;

    CREATE TEMP TABLE _mig30_chunks ON COMMIT DROP AS
    SELECT DISTINCT ON (c.document_id, c.content_hash)
           c.id, c.document_id, c.workspace_hash, c.content_hash, cb.id AS blob_id, c.chunk_index,
           c.start_line, c.end_line, c.metadata, c.created_at, c.embed_status, c.search_vector,
           c.symbol_name, c.symbol_kind, c.language, c.line_start, c.line_end,
           c.chunk_type, c.embedding_strategy, c.summary, c.summary_hash, c.graph_context_hash
    FROM chunks c
    JOIN content_blobs cb ON cb.sha256_bytes = sha256(convert_to(coalesce(c.content, ''), 'UTF8'))
    JOIN _mig30_documents md ON md.id = c.document_id
    ORDER BY c.document_id, c.content_hash, c.created_at DESC;

    CREATE TEMP TABLE _mig30_embeddings ON COMMIT DROP AS
    SELECT e.id, e.chunk_id, wi.short_id AS workspace_id, ev.id AS vector_id, e.created_at, e.updated_at
    FROM embeddings e
    JOIN _mig30_chunks mc ON mc.id = e.chunk_id
    JOIN workspace_ids wi ON wi.workspace_hash = mc.workspace_hash
    JOIN embedding_vectors ev ON ev.blob_id = mc.blob_id AND ev.provider = e.provider AND ev.model = e.model;

    CREATE TEMP TABLE _mig30_chunk_entities ON COMMIT DROP AS
    SELECT DISTINCT ce.chunk_id, wi.short_id AS workspace_id, ed.id AS entity_id
    FROM chunk_entities ce
    JOIN _mig30_chunks mc ON mc.id = ce.chunk_id
    JOIN workspace_ids wi ON wi.workspace_hash = mc.workspace_hash
    JOIN entity_dictionary ed ON ed.entity_name = ce.entity_name AND ed.entity_type = ce.entity_type;

    CREATE TEMP TABLE _mig30_graph_edges ON COMMIT DROP AS
    SELECT DISTINCT ON (wi.short_id, sn.id, tn.id, ge.edge_type)
           ge.id, wi.short_id AS workspace_id, sn.id AS source_node_id, tn.id AS target_node_id,
           ge.edge_type, sf.id AS source_file_id, ge.metadata, ge.created_at
    FROM graph_edges ge
    JOIN workspace_ids wi ON wi.workspace_hash = ge.workspace_hash
    JOIN graph_nodes sn ON sn.node_key = ge.source_node
    JOIN graph_nodes tn ON tn.node_key = ge.target_node
    JOIN graph_nodes sf ON sf.node_key = ge.source_file
    ORDER BY wi.short_id, sn.id, tn.id, ge.edge_type, ge.created_at DESC;
END $$;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_documents_title_propagate ON documents;
DROP TRIGGER IF EXISTS trg_chunks_search_vector ON chunks;

DROP TABLE chunk_entities;
DROP TABLE embeddings;
DROP TABLE chunks;
DROP TABLE documents;
DROP TABLE graph_edges;

CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_hash TEXT NOT NULL REFERENCES workspaces(hash) ON DELETE CASCADE,
    content_hash TEXT NOT NULL,
    blob_id BIGINT NOT NULL REFERENCES content_blobs(id),
    title TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    collection TEXT NOT NULL DEFAULT 'default',
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    supersedes_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    mod_time TIMESTAMPTZ,
    file_size BIGINT
);

CREATE TABLE chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    workspace_hash TEXT NOT NULL REFERENCES workspaces(hash) ON DELETE CASCADE,
    content_hash TEXT NOT NULL,
    blob_id BIGINT NOT NULL REFERENCES content_blobs(id),
    chunk_index INT NOT NULL DEFAULT 0,
    start_line INT,
    end_line INT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    embed_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (embed_status IN ('pending', 'embedded', 'embed_failed')),
    search_vector TSVECTOR,
    symbol_name TEXT,
    symbol_kind TEXT,
    language TEXT,
    line_start INT,
    line_end INT,
    chunk_type TEXT NOT NULL DEFAULT 'raw',
    embedding_strategy TEXT NOT NULL DEFAULT 'raw_code',
    summary TEXT,
    summary_hash TEXT,
    graph_context_hash TEXT,
    CONSTRAINT uq_chunks_document_content_hash UNIQUE (document_id, content_hash)
);

CREATE TABLE embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id UUID NOT NULL UNIQUE REFERENCES chunks(id) ON DELETE CASCADE,
    workspace_id INT NOT NULL REFERENCES workspace_ids(short_id) ON DELETE CASCADE,
    vector_id BIGINT NOT NULL REFERENCES embedding_vectors(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chunk_entities (
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    workspace_id INT NOT NULL REFERENCES workspace_ids(short_id) ON DELETE CASCADE,
    entity_id INT NOT NULL REFERENCES entity_dictionary(id) ON DELETE CASCADE,
    PRIMARY KEY (chunk_id, entity_id)
);

CREATE TABLE graph_edges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id INT NOT NULL REFERENCES workspace_ids(short_id) ON DELETE CASCADE,
    source_node_id BIGINT NOT NULL REFERENCES graph_nodes(id),
    target_node_id BIGINT NOT NULL REFERENCES graph_nodes(id),
    edge_type TEXT NOT NULL
        CHECK (edge_type IN ('contains', 'imports', 'calls', 'references', 'http', 'middleware', 'integration', 'reconcile')),
    source_file_id BIGINT NOT NULL REFERENCES graph_nodes(id),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_graph_edge UNIQUE (workspace_id, source_node_id, target_node_id, edge_type)
);

CREATE VIEW document_views AS
SELECT d.id, d.workspace_hash, d.content_hash, d.title, cb.content, d.source_path,
       d.collection, d.tags, d.metadata, d.created_at, d.updated_at,
       d.supersedes_id, d.mod_time, d.file_size
FROM documents d
JOIN content_blobs cb ON cb.id = d.blob_id;

CREATE VIEW chunk_views AS
SELECT c.id, c.document_id, c.workspace_hash, c.content_hash, cb.content,
       c.chunk_index, c.start_line, c.end_line, c.metadata, c.created_at,
       c.embed_status, c.search_vector, c.symbol_name, c.symbol_kind, c.language,
       c.line_start, c.line_end, c.chunk_type, c.embedding_strategy,
       c.summary, c.summary_hash, c.graph_context_hash
FROM chunks c
JOIN content_blobs cb ON cb.id = c.blob_id;

CREATE VIEW graph_edge_views AS
SELECT ge.id, wi.workspace_hash, sn.node_key AS source_node, tn.node_key AS target_node,
       ge.edge_type, sf.node_key AS source_file, ge.metadata, ge.created_at
FROM graph_edges ge
JOIN workspace_ids wi ON wi.short_id = ge.workspace_id
JOIN graph_nodes sn ON sn.id = ge.source_node_id
JOIN graph_nodes tn ON tn.id = ge.target_node_id
JOIN graph_nodes sf ON sf.id = ge.source_file_id;

-- +goose StatementBegin
DO $$
BEGIN
    INSERT INTO documents (
        id, workspace_hash, content_hash, blob_id, title, source_path, collection,
        tags, metadata, created_at, updated_at, supersedes_id, mod_time, file_size
    )
    SELECT m.id, m.workspace_hash, m.content_hash, m.blob_id, m.title, m.source_path, m.collection,
           m.tags, m.metadata, m.created_at, m.updated_at,
           CASE WHEN EXISTS (SELECT 1 FROM _mig30_documents m2 WHERE m2.id = m.supersedes_id)
                THEN m.supersedes_id ELSE NULL END,
           m.mod_time, m.file_size
    FROM _mig30_documents m;

    INSERT INTO chunks (
        id, document_id, workspace_hash, content_hash, blob_id, chunk_index, start_line, end_line,
        metadata, created_at, embed_status, search_vector, symbol_name, symbol_kind,
        language, line_start, line_end, chunk_type, embedding_strategy, summary,
        summary_hash, graph_context_hash
    )
    SELECT id, document_id, workspace_hash, content_hash, blob_id, chunk_index, start_line, end_line,
           metadata, created_at, embed_status, search_vector, symbol_name, symbol_kind,
           language, line_start, line_end, chunk_type, embedding_strategy, summary,
           summary_hash, graph_context_hash
    FROM _mig30_chunks;

    INSERT INTO embeddings (id, chunk_id, workspace_id, vector_id, created_at, updated_at)
    SELECT id, chunk_id, workspace_id, vector_id, created_at, updated_at
    FROM _mig30_embeddings;

    INSERT INTO chunk_entities (chunk_id, workspace_id, entity_id)
    SELECT chunk_id, workspace_id, entity_id
    FROM _mig30_chunk_entities;

    INSERT INTO graph_edges (
        id, workspace_id, source_node_id, target_node_id, edge_type, source_file_id, metadata, created_at
    )
    SELECT id, workspace_id, source_node_id, target_node_id, edge_type, source_file_id, metadata, created_at
    FROM _mig30_graph_edges;
END $$;
-- +goose StatementEnd

CREATE UNIQUE INDEX uq_documents_source_path_workspace
    ON documents (source_path, workspace_hash) WHERE source_path <> '';
CREATE INDEX idx_documents_workspace_hash ON documents (workspace_hash);
CREATE INDEX idx_documents_blob_id ON documents (blob_id);
CREATE INDEX idx_documents_created_at ON documents (created_at);
CREATE INDEX idx_documents_updated_at ON documents (updated_at);
CREATE INDEX idx_documents_supersedes_id ON documents (supersedes_id);
CREATE INDEX idx_documents_tags ON documents USING gin (tags);

CREATE INDEX idx_chunks_document_id ON chunks (document_id);
CREATE INDEX idx_chunks_workspace_hash ON chunks (workspace_hash);
CREATE INDEX idx_chunks_blob_id ON chunks (blob_id);
CREATE INDEX idx_chunks_embed_status ON chunks (embed_status, created_at)
    WHERE embed_status IN ('pending', 'embed_failed');
CREATE INDEX idx_chunks_search_vector ON chunks USING gin (search_vector);
CREATE INDEX idx_chunks_symbol_name ON chunks (symbol_name) WHERE symbol_name IS NOT NULL;

CREATE INDEX idx_embedding_vectors_hnsw ON embedding_vectors
    USING hnsw (embedding halfvec_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX idx_embeddings_ws_vector ON embeddings (workspace_id, vector_id);

CREATE INDEX idx_chunk_entities_ws_entity ON chunk_entities (workspace_id, entity_id);

CREATE INDEX idx_graph_nodes_bare_symbol ON graph_nodes (bare_symbol) WHERE bare_symbol <> '';
CREATE INDEX idx_graph_edges_target ON graph_edges (workspace_id, target_node_id, edge_type);
CREATE INDEX idx_graph_edges_source ON graph_edges (workspace_id, source_node_id, edge_type);
CREATE INDEX idx_graph_edges_file ON graph_edges (workspace_id, source_file_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_default_blob_id() RETURNS trigger AS $$
BEGIN
    IF NEW.blob_id IS NULL OR NEW.blob_id = 0 THEN
        NEW.blob_id := intern_content_blob('');
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_documents_default_blob
    BEFORE INSERT ON documents
    FOR EACH ROW
    EXECUTE FUNCTION ensure_default_blob_id();

CREATE TRIGGER trg_chunks_default_blob
    BEFORE INSERT ON chunks
    FOR EACH ROW
    EXECUTE FUNCTION ensure_default_blob_id();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
DECLARE
    doc_title text;
    chunk_text text;
BEGIN
    IF pg_trigger_depth() > 1 AND NEW.search_vector IS NOT NULL THEN
        RETURN NEW;
    END IF;
    SELECT title INTO doc_title FROM documents WHERE id = NEW.document_id;
    SELECT content INTO chunk_text FROM content_blobs WHERE id = NEW.blob_id;
    NEW.search_vector :=
        setweight(to_tsvector(get_tsvector_config(), coalesce(doc_title, '')), 'A') ||
        setweight(to_tsvector(get_tsvector_config(), coalesce(chunk_text, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_chunks_search_vector
    BEFORE INSERT OR UPDATE OF blob_id ON chunks
    FOR EACH ROW
    EXECUTE FUNCTION chunks_search_vector_update();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION documents_title_propagate() RETURNS trigger AS $$
BEGIN
    IF NEW.title IS DISTINCT FROM OLD.title THEN
        UPDATE chunks c
        SET search_vector =
            setweight(to_tsvector(get_tsvector_config(), coalesce(NEW.title, '')), 'A') ||
            setweight(to_tsvector(get_tsvector_config(), coalesce(cb.content, '')), 'B')
        FROM content_blobs cb
        WHERE c.document_id = NEW.id AND cb.id = c.blob_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_documents_title_propagate
    AFTER UPDATE OF title ON documents
    FOR EACH ROW
    EXECUTE FUNCTION documents_title_propagate();

-- +goose Down

DROP VIEW IF EXISTS graph_edge_views;
DROP VIEW IF EXISTS chunk_views;
DROP VIEW IF EXISTS document_views;
DROP FUNCTION IF EXISTS intern_entity(TEXT, TEXT);
DROP FUNCTION IF EXISTS intern_graph_node(TEXT);
DROP FUNCTION IF EXISTS intern_content_blob(TEXT);
DROP FUNCTION IF EXISTS ensure_workspace_short_id(TEXT);
DROP FUNCTION IF EXISTS resolve_workspace_short_id(TEXT);
