-- +goose Up
-- Include document title in BM25 tsvector with rank A boost (vs content rank B).
-- Without this, searches by file/doc name produce silent zero results when the
-- title word doesn't appear in chunk content (issue #305).
--
-- Strategy: replace the trigger function to look up the parent document title and
-- concatenate setweight(title, 'A') || setweight(content, 'B'). When document
-- title changes, also re-trigger downstream chunks.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
DECLARE
    doc_title text;
BEGIN
    SELECT title INTO doc_title FROM documents WHERE id = NEW.document_id;
    NEW.search_vector :=
        setweight(to_tsvector('english', coalesce(doc_title, '')), 'A') ||
        setweight(to_tsvector('english', coalesce(NEW.content, '')), 'B');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Re-populate existing chunks via the new function.
UPDATE chunks c
SET search_vector =
    setweight(to_tsvector('english', coalesce(d.title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(c.content, '')), 'B')
FROM documents d
WHERE c.document_id = d.id;

-- Trigger to re-rank chunks when document title changes.
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

CREATE TRIGGER trg_documents_title_propagate
    AFTER UPDATE OF title ON documents
    FOR EACH ROW
    EXECUTE FUNCTION documents_title_propagate();

-- +goose Down
DROP TRIGGER IF EXISTS trg_documents_title_propagate ON documents;
DROP FUNCTION IF EXISTS documents_title_propagate;

-- Restore previous trigger (content-only).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION chunks_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', NEW.content);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

UPDATE chunks SET search_vector = to_tsvector('english', content);
