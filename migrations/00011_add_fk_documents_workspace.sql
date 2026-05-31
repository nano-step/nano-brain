-- +goose Up
-- IMPORTANT: Before applying this migration, run:
--   nano-brain cleanup-orphan-workspaces
-- This migration adds FK constraints from documents.workspace_hash and
-- chunks.workspace_hash to workspaces.hash. If any rows exist in documents
-- or chunks with workspace_hash values not in workspaces.hash, this migration
-- will FAIL with PostgreSQL error 23503 (foreign_key_violation). Run cleanup
-- first. See issue #238.
--
-- Both ALTER TABLE statements run in a single transaction (goose default).
-- On large tables (>1M rows), expect 30-60s of blocked writes during
-- constraint validation. For very large deployments, consider applying
-- ADD CONSTRAINT ... NOT VALID + VALIDATE CONSTRAINT manually.
ALTER TABLE documents
    ADD CONSTRAINT fk_documents_workspace
    FOREIGN KEY (workspace_hash) REFERENCES workspaces(hash) ON DELETE CASCADE;

ALTER TABLE chunks
    ADD CONSTRAINT fk_chunks_workspace
    FOREIGN KEY (workspace_hash) REFERENCES workspaces(hash) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE documents DROP CONSTRAINT IF EXISTS fk_documents_workspace;
ALTER TABLE chunks DROP CONSTRAINT IF EXISTS fk_chunks_workspace;
