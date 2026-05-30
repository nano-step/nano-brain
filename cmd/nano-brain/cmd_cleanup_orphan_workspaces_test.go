package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/sqlc-dev/pqtype"
)

const cleanupTestDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"

func setupCleanupTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cleanupTestDSN)
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	schema := fmt.Sprintf("test_cl_%x", sha256.Sum256([]byte(t.Name())))[:18]
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

func insertOrphanDoc(t *testing.T, pgDB *sql.DB, wsHash, sourcePath string) {
	t.Helper()
	q := sqlc.New(pgDB)
	_, err := q.UpsertDocumentBySourcePath(context.Background(), sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   hex.EncodeToString(sha256.New().Sum([]byte(sourcePath))),
		Title:         "orphan-" + sourcePath,
		Content:       "orphan content for " + sourcePath,
		SourcePath:    sourcePath,
		Collection:    "session-summary",
		Tags:          []string{"orphan-test"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	if err != nil {
		t.Fatalf("insert orphan doc: %v", err)
	}
}

func insertOrphanChunk(t *testing.T, pgDB *sql.DB, wsHash string) {
	t.Helper()
	q := sqlc.New(pgDB)

	docID, err := uuid.NewRandom()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pgDB.Exec(
		`INSERT INTO documents (id, workspace_hash, source_path, title, content, content_hash, collection, tags, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
		docID, wsHash, "/orphan-chunk-doc-"+docID.String(), "orphan-chunk-doc", "x", hex.EncodeToString(sha256.New().Sum([]byte(docID.String()))), "session-summary", "{}",
	); err != nil {
		t.Fatalf("insert orphan doc for chunk: %v", err)
	}

	if _, err := q.UpsertChunk(context.Background(), sqlc.UpsertChunkParams{
		DocumentID:    docID,
		WorkspaceHash: wsHash,
		ContentHash:   hex.EncodeToString(sha256.New().Sum([]byte("chunk-" + wsHash))),
		Content:       "orphan chunk content",
		ChunkIndex:    0,
		StartLine:     sql.NullInt32{Int32: 1, Valid: true},
		EndLine:       sql.NullInt32{Int32: 1, Valid: true},
		Metadata:      pqtype.NullRawMessage{},
	}); err != nil {
		t.Fatalf("insert orphan chunk: %v", err)
	}
}

func countDocs(t *testing.T, pgDB *sql.DB) int64 {
	t.Helper()
	var n int64
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM documents").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCleanupOrphanWorkspaces_EmptyDB(t *testing.T) {
	pgDB := setupCleanupTestPG(t)
	q := sqlc.New(pgDB)

	orphans, err := q.ListOrphanDocumentWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(orphans) != 0 {
		t.Errorf("expected 0 orphans, got %d", len(orphans))
	}

	deletedDocs, err := q.DeleteOrphanDocuments(context.Background())
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deletedDocs != 0 {
		t.Errorf("expected 0 deleted, got %d", deletedDocs)
	}
}

func TestCleanupOrphanWorkspaces_DeletesOnlyOrphans(t *testing.T) {
	pgDB := setupCleanupTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	registeredHash := hex.EncodeToString(sha256.New().Sum([]byte("registered-cleanup")))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: registeredHash,
		Name: "registered",
		Path: "/tmp/registered-cleanup",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: registeredHash,
		ContentHash:   "keep-hash",
		Title:         "keep-me",
		Content:       "I should not be deleted",
		SourcePath:    "/keep/me.md",
		Collection:    "memory",
		Tags:          []string{"keep"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	}); err != nil {
		t.Fatalf("insert registered doc: %v", err)
	}

	insertOrphanDoc(t, pgDB, "orphan-hash-1", "/orphan/1.md")
	insertOrphanDoc(t, pgDB, "orphan-hash-1", "/orphan/2.md")
	insertOrphanDoc(t, pgDB, "orphan-hash-2", "/orphan/3.md")

	orphans, err := q.ListOrphanDocumentWorkspaces(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(orphans) != 2 {
		t.Errorf("expected 2 orphan workspace_hash values, got %d", len(orphans))
	}

	totalOrphanDocs, err := q.CountOrphanDocuments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if totalOrphanDocs != 3 {
		t.Errorf("expected 3 orphan docs, got %d", totalOrphanDocs)
	}

	before := countDocs(t, pgDB)
	if before != 4 {
		t.Errorf("expected 4 docs before cleanup, got %d", before)
	}

	deletedDocs, err := q.DeleteOrphanDocuments(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deletedDocs != 3 {
		t.Errorf("expected 3 deleted docs, got %d", deletedDocs)
	}

	after := countDocs(t, pgDB)
	if after != 1 {
		t.Errorf("expected 1 doc remaining (the registered one), got %d", after)
	}

	var keepCount int64
	if err := pgDB.QueryRow("SELECT COUNT(*) FROM documents WHERE workspace_hash = $1", registeredHash).Scan(&keepCount); err != nil {
		t.Fatal(err)
	}
	if keepCount != 1 {
		t.Errorf("registered workspace doc was affected: %d remaining (want 1)", keepCount)
	}
}

func TestCleanupOrphanWorkspaces_HandlesChunks(t *testing.T) {
	pgDB := setupCleanupTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	insertOrphanChunk(t, pgDB, "orphan-chunk-ws")

	chunkBefore, err := q.CountOrphanChunks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if chunkBefore != 1 {
		t.Errorf("expected 1 orphan chunk, got %d", chunkBefore)
	}

	deletedChunks, err := q.DeleteOrphanChunks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deletedChunks != 1 {
		t.Errorf("expected 1 deleted chunk, got %d", deletedChunks)
	}

	chunkAfter, err := q.CountOrphanChunks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if chunkAfter != 0 {
		t.Errorf("expected 0 orphan chunks after delete, got %d", chunkAfter)
	}
}

func TestCleanupOrphanWorkspaces_PreflightWarnIsNonBlocking(t *testing.T) {
	start := time.Now()
	warnIfServerRunning()
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("preflight took %v, want <2s — should be non-blocking", elapsed)
	}
}
