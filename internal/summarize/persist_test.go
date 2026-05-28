package summarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

const persistTestDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"

func setupPersistTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(persistTestDSN)
	if err != nil {
		t.Skip("postgres not available: parse config: " + err.Error())
	}

	schema := fmt.Sprintf("test_%x", sha256.Sum256([]byte(t.Name())))[:18]
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Skip("postgres not available: connect: " + err.Error())
	}

	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Skip("postgres not available: create schema: " + err.Error())
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatal("goose dialect: " + err.Error())
	}
	goose.SetTableName(schema + "_goose_version")
	migrateDB := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, migrateDB, "."); err != nil {
		migrateDB.Close()
		pool.Close()
		t.Fatal("goose migrate: " + err.Error())
	}
	migrateDB.Close()
	goose.SetTableName("goose_db_version")

	pgDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		pgDB.Close()
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})

	return pgDB
}

func TestBuildSourcePath(t *testing.T) {
	meta := SessionMetadata{Source: SourceOpenCode, SessionID: "abc123"}
	got := buildSourcePath(meta)
	want := "summary://opencode/abc123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	meta2 := SessionMetadata{Source: SourceClaude, SessionID: "xyz789"}
	got2 := buildSourcePath(meta2)
	want2 := "summary://claude/xyz789"
	if got2 != want2 {
		t.Errorf("got %q, want %q", got2, want2)
	}
}

func TestPersister_Save_UsesWorkspaceHashFromMeta(t *testing.T) {
	pgDB := setupPersistTestPG(t)

	logger := zerolog.Nop()
	p := NewPersister(pgDB, nil, logger)

	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ws-hash-test-123",
		Title:         "WS Hash Test",
		CreatedAt:     time.Now(),
		WorkspaceHash: "test-ws-hash-abc",
	}

	if err := p.Save(context.Background(), "# Test Summary\n\nContent here.", meta); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	q := sqlc.New(pgDB)
	doc, err := q.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/ws-hash-test-123",
		WorkspaceHash: "test-ws-hash-abc",
	})
	if err != nil {
		t.Fatalf("doc not found with workspace_hash=test-ws-hash-abc: %v", err)
	}

	if doc.Collection != "session-summary" {
		t.Errorf("collection = %q, want %q", doc.Collection, "session-summary")
	}
	if doc.ContentHash == "" {
		t.Error("content_hash should not be empty")
	}

	// Verify doc is NOT found with empty workspace_hash
	_, err = q.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/ws-hash-test-123",
		WorkspaceHash: "",
	})
	if err == nil {
		t.Error("expected no doc at empty workspace_hash, but found one")
	}
}
