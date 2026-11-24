-- +goose Up
-- ensure_workspace_short_id must never resurrect a deleted workspace: it now
-- raises when the workspace is not registered instead of INSERTing into
-- workspaces/workspace_ids on any graph upsert (issue #714).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_workspace_short_id(p_hash TEXT) RETURNS INT AS $$
DECLARE
    v_id INT;
BEGIN
    SELECT short_id INTO v_id FROM workspace_ids WHERE workspace_hash = p_hash;
    IF v_id IS NULL THEN
        RAISE EXCEPTION 'workspace % is not registered; register it before writing graph rows', p_hash;
    END IF;
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
-- Restores the 00030 auto-insert body verbatim.
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
