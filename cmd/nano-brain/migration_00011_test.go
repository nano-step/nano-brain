package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/sqlc-dev/pqtype"
)

func errorsAs(err error, target interface{}) bool {
	return errors.As(err, target)
}

func setupMigration00011TestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(testDSNValue())
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	schema := fmt.Sprintf("test_m11_%x", sha256.Sum256([]byte(t.Name())))[:18]
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Skip("postgres not available: " + err.Error())
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	goose.SetTableName(schema + "_goose_version")
	migrateDB := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, migrateDB, "."); err != nil {
		migrateDB.Close()
		pool.Close()
		t.Fatal(err)
	}
	migrateDB.Close()
	goose.SetTableName("goose_db_version")

	pgDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		pgDB.Close()
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return pgDB
}

const fkConstraintDocuments = "fk_documents_workspace"
const fkConstraintChunks = "fk_chunks_workspace"

func TestMigration00011_FKRejectsOrphanInsert(t *testing.T) {
	pgDB := setupMigration00011TestPG(t)

	_, err := pgDB.Exec(`INSERT INTO documents (
		id, workspace_hash, source_path, title, content, content_hash,
		collection, tags, metadata, created_at, updated_at
	) VALUES (
		gen_random_uuid(), 'not-registered-xyz', '/fk-test/orphan', 'orphan',
		'content', 'hash', 'session-summary', '{}', '{}'::jsonb, NOW(), NOW()
	)`)

	if err == nil {
		t.Fatal("expected FK violation, got nil — migration 00011 not applied or FK missing")
	}

	var pgErr *pgconn.PgError
	if !errorsAs(err, &pgErr) {
		t.Fatalf("expected *pgconn.PgError, got %T: %v", err, err)
	}
	if pgErr.Code != pgerrcode.ForeignKeyViolation {
		t.Errorf("expected SQLSTATE 23503, got %s: %s", pgErr.Code, pgErr.Message)
	}
	if !strings.Contains(pgErr.ConstraintName, fkConstraintDocuments) {
		t.Errorf("expected constraint %q, got %q", fkConstraintDocuments, pgErr.ConstraintName)
	}
}

func TestMigration00011_FKRejectsOrphanUpdate(t *testing.T) {
	pgDB := setupMigration00011TestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("registered-update-test")))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "update-test",
		Path: "/tmp/update-test",
	}); err != nil {
		t.Fatal(err)
	}

	doc, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   "doc-hash",
		Title:         "doc",
		Content:       "content",
		SourcePath:    "/fk-update/doc",
		Collection:    "memory",
		Tags:          []string{"test"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pgDB.ExecContext(ctx, `UPDATE documents SET workspace_hash = 'not-registered-xyz' WHERE id = $1`, doc.ID)
	if err == nil {
		t.Fatal("expected FK violation on UPDATE, got nil")
	}

	var pgErr *pgconn.PgError
	if !errorsAs(err, &pgErr) {
		t.Fatalf("expected *pgconn.PgError, got %T: %v", err, err)
	}
	if pgErr.Code != pgerrcode.ForeignKeyViolation {
		t.Errorf("expected SQLSTATE 23503 on UPDATE, got %s", pgErr.Code)
	}
}

func TestMigration00011_CascadeDeletesDocsAndChunks(t *testing.T) {
	pgDB := setupMigration00011TestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("cascade-test")))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "cascade",
		Path: "/tmp/cascade",
	}); err != nil {
		t.Fatal(err)
	}

	doc, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   "cascade-hash",
		Title:         "cascade",
		Content:       "content",
		SourcePath:    "/cascade/doc",
		Collection:    "memory",
		Tags:          []string{"test"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
		DocumentID:    doc.ID,
		WorkspaceHash: wsHash,
		ContentHash:   "chunk-hash",
		Content:       "chunk content",
		ChunkIndex:    0,
		StartLine:     sql.NullInt32{Int32: 1, Valid: true},
		EndLine:       sql.NullInt32{Int32: 1, Valid: true},
		Metadata:      pqtype.NullRawMessage{},
	}); err != nil {
		t.Fatal(err)
	}

	var docsBefore, chunksBefore int64
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM documents WHERE workspace_hash = $1", wsHash).Scan(&docsBefore); err != nil {
		t.Fatal(err)
	}
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM chunks WHERE workspace_hash = $1", wsHash).Scan(&chunksBefore); err != nil {
		t.Fatal(err)
	}
	if docsBefore != 1 || chunksBefore != 1 {
		t.Fatalf("setup: expected 1 doc + 1 chunk, got %d + %d", docsBefore, chunksBefore)
	}

	if err := q.DeleteWorkspace(ctx, wsHash); err != nil {
		t.Fatal(err)
	}

	var docsAfter, chunksAfter int64
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM documents WHERE workspace_hash = $1", wsHash).Scan(&docsAfter); err != nil {
		t.Fatal(err)
	}
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM chunks WHERE workspace_hash = $1", wsHash).Scan(&chunksAfter); err != nil {
		t.Fatal(err)
	}
	if docsAfter != 0 {
		t.Errorf("docs not cascaded: %d remaining, want 0", docsAfter)
	}
	if chunksAfter != 0 {
		t.Errorf("chunks not cascaded: %d remaining, want 0", chunksAfter)
	}
}

func TestMigration00011_FKRejectsOrphanChunkInsert(t *testing.T) {
	pgDB := setupMigration00011TestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("chunk-fk-test")))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "chunk-fk",
		Path: "/tmp/chunk-fk",
	}); err != nil {
		t.Fatal(err)
	}
	doc, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   "doc-hash",
		Title:         "doc",
		Content:       "content",
		SourcePath:    "/chunk-fk/doc",
		Collection:    "memory",
		Tags:          []string{"test"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = pgDB.ExecContext(ctx, `INSERT INTO chunks (
		id, document_id, workspace_hash, content, content_hash, chunk_index, start_line, end_line, metadata, created_at
	) VALUES (
		gen_random_uuid(), $1, 'not-registered-chunk-xyz', 'content', 'hash', 0, 1, 1, '{}'::jsonb, NOW()
	)`, doc.ID)
	if err == nil {
		t.Fatal("expected FK violation on chunk INSERT, got nil")
	}

	var pgErr *pgconn.PgError
	if !errorsAs(err, &pgErr) {
		t.Fatalf("expected *pgconn.PgError, got %T: %v", err, err)
	}
	if pgErr.Code != pgerrcode.ForeignKeyViolation {
		t.Errorf("expected SQLSTATE 23503, got %s", pgErr.Code)
	}
	if !strings.Contains(pgErr.ConstraintName, fkConstraintChunks) {
		t.Errorf("expected constraint %q, got %q", fkConstraintChunks, pgErr.ConstraintName)
	}
}
