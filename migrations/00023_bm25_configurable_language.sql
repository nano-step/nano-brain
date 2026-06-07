-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION get_tsvector_config() RETURNS regconfig AS $$
BEGIN
    -- Get the tsvector configuration from session-level GUC with fallback to 'english'
    -- The setting can be set with: SET nanobrain.tsvector_config = 'simple';
    -- Returns a valid PostgreSQL text search configuration name (regconfig)
    RETURN COALESCE(current_setting('nanobrain.tsvector_config', true)::regconfig, 'english'::regconfig);
END;
$$ LANGUAGE plpgsql IMMUTABLE;
-- +goose StatementEnd

-- Update chunks_search_vector_update to use configurable language
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
DECLARE
    doc_title text;
BEGIN
    IF pg_trigger_depth() > 1 AND NEW.search_vector IS NOT NULL THEN
        RETURN NEW;
    END IF;
    SELECT title INTO doc_title FROM documents WHERE id = NEW.document_id;
    NEW.search_vector :=
        setweight(to_tsvector(get_tsvector_config(), coalesce(doc_title, '')), 'A') ||
        setweight(to_tsvector(get_tsvector_config(), coalesce(NEW.content, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION documents_title_propagate() RETURNS trigger AS $$
BEGIN
    IF NEW.title IS DISTINCT FROM OLD.title THEN
        UPDATE chunks
        SET search_vector =
            setweight(to_tsvector(get_tsvector_config(), coalesce(NEW.title, '')), 'A') ||
            setweight(to_tsvector(get_tsvector_config(), coalesce(content, '')), 'B')
        WHERE document_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP FUNCTION IF EXISTS get_tsvector_config();

CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
DECLARE
    doc_title text;
BEGIN
    IF pg_trigger_depth() > 1 AND NEW.search_vector IS NOT NULL THEN
        RETURN NEW;
    END IF;
    SELECT title INTO doc_title FROM documents WHERE id = NEW.document_id;
    NEW.search_vector :=
        setweight(to_tsvector('english', coalesce(doc_title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(NEW.content, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION documents_title_propagate() RETURNS trigger AS $$
BEGIN
    IF NEW.title IS DISTINCT FROM OLD.title THEN
        UPDATE chunks
        SET search_vector =
            setweight(to_tsvector('english', coalesce(NEW.title, '')), 'A') ||
            setweight(to_tsvector('english', coalesce(content, '')), 'B')
        WHERE document_id = NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd