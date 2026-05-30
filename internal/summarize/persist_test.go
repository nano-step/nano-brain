package summarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
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

	ctx := context.Background()
	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: "test-ws-hash-abc",
		Name: "persist-test-meta",
		Path: "/tmp/persist-test-meta",
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ws-hash-test-123",
		Title:         "WS Hash Test",
		CreatedAt:     time.Now(),
		WorkspaceHash: "test-ws-hash-abc",
	}

	if err := p.Save(ctx, "# Test Summary\n\nContent here.", meta); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
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

	_, err = q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/ws-hash-test-123",
		WorkspaceHash: "",
	})
	if err == nil {
		t.Error("expected no doc at empty workspace_hash, but found one")
	}
}

func TestPersister_Save_RejectsUnregisteredWorkspace(t *testing.T) {
	pgDB := setupPersistTestPG(t)

	p := NewPersister(pgDB, nil, zerolog.Nop())
	ctx := context.Background()

	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "leak-test-001",
		Title:         "Unregistered Workspace Test",
		CreatedAt:     time.Now(),
		WorkspaceHash: "not-a-registered-workspace-hash-xyz",
	}

	err := p.Save(ctx, "# Should Not Persist\n\nbody", meta)
	if err == nil {
		t.Fatal("expected error, got nil — leak #3 still open")
	}
	if !errors.Is(err, ErrWorkspaceNotRegistered) {
		t.Errorf("expected ErrWorkspaceNotRegistered, got: %v", err)
	}

	q := sqlc.New(pgDB)
	_, getErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/leak-test-001",
		WorkspaceHash: "not-a-registered-workspace-hash-xyz",
	})
	if getErr == nil {
		t.Error("document was persisted under unregistered workspace_hash — defense breached")
	}
}

func TestPersister_Save_AcceptsRegisteredWorkspace(t *testing.T) {
	pgDB := setupPersistTestPG(t)

	p := NewPersister(pgDB, nil, zerolog.Nop())
	ctx := context.Background()
	q := sqlc.New(pgDB)

	registeredHash := "registered-ws-hash-for-positive-test"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: registeredHash,
		Name: "registered-positive-test",
		Path: "/tmp/persist-positive-test",
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "positive-test-001",
		Title:         "Registered Workspace Positive Test",
		CreatedAt:     time.Now(),
		WorkspaceHash: registeredHash,
	}

	if err := p.Save(ctx, "# Should Persist\n\nbody", meta); err != nil {
		t.Fatalf("Save with registered workspace failed: %v", err)
	}

	if _, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/positive-test-001",
		WorkspaceHash: registeredHash,
	}); err != nil {
		t.Errorf("document not found after successful Save: %v", err)
	}
}
