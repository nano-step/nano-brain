-- +goose Up
ALTER TABLE documents ADD COLUMN supersedes_id UUID REFERENCES documents(id);
CREATE INDEX idx_documents_supersedes_id ON documents(supersedes_id);

-- +goose Down
DROP INDEX IF EXISTS idx_documents_supersedes_id;
ALTER TABLE documents DROP COLUMN IF EXISTS supersedes_id;
