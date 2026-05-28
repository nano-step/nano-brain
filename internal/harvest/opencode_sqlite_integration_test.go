//go:build integration

package harvest_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/sqlc-dev/pqtype"
	_ "modernc.org/sqlite"
)

func setupIntegrationPG(t *testing.T) *sql.DB {
	t.Helper()

	ctx := context.Background()
	dsn := "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	schema := fmt.Sprintf("inttest_%x", sha256.Sum256([]byte(t.Name())))[:22]
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

func setupIntegrationSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL);
		CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, title TEXT, time_created INTEGER, time_updated INTEGER);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOpenCodeSQLite_Integration_RealPostgres(t *testing.T) {
	pgDB := setupIntegrationPG(t)
	sqdb := setupIntegrationSQLite(t)

	now := time.Now()
	oldMs := now.Add(-15 * time.Minute).UnixMilli()
	worktree := "/home/user/integration-test-app"

	_, err := sqdb.Exec(`INSERT INTO project (id, worktree) VALUES (?, ?)`, "proj-int", worktree)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqdb.Exec(`INSERT INTO session (id, project_id, title, time_created, time_updated) VALUES (?, ?, ?, ?, ?)`,
		"int-sess-1", "proj-int", "Integration Session", oldMs, oldMs)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqdb.Exec(`INSERT INTO message (id, session_id, time_created, data) VALUES (?, ?, ?, ?)`,
		"int-msg-1", "int-sess-1", oldMs, `{"role":"user"}`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = sqdb.Exec(`INSERT INTO part (id, message_id, session_id, time_created, data) VALUES (?, ?, '', 0, ?)`,
		"int-p-1", "int-msg-1", `{"type":"text","text":"Integration test content"}`)
	if err != nil {
		t.Fatal(err)
	}

	wsH := sha256.Sum256([]byte(worktree))
	wsHash := hex.EncodeToString(wsH[:])

	successFn := func(ctx context.Context, md string, meta harvest.SummaryMeta) error {
		q := sqlc.New(pgDB)
		_, uErr := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: meta.WorkspaceHash,
			ContentHash:   "int-test-summary-hash",
			Title:         "Summary: " + meta.Title,
			Content:       "# Integration Summary\n\nSummarized.",
			SourcePath:    "summary://opencode/" + meta.SessionID,
			Collection:    "session-summary",
			Tags:          []string{"summary", "opencode"},
			Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{"summary":true}`), Valid: true},
		})
		return uErr
	}

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB).
		WithSummarizer(&stubIntSummarizer{fn: successFn})

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)

	if harvested != 1 {
		t.Errorf("harvested = %d, want 1", harvested)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	if errCount != 0 {
		t.Errorf("errCount = %d, want 0", errCount)
	}

	q := sqlc.New(pgDB)
	doc, lookupErr := q.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/int-sess-1",
		WorkspaceHash: wsHash,
	})
	if lookupErr != nil {
		t.Fatalf("summary doc not found: %v", lookupErr)
	}
	if doc.Collection != "session-summary" {
		t.Errorf("collection = %q, want %q", doc.Collection, "session-summary")
	}
	if doc.WorkspaceHash != wsHash {
		t.Errorf("workspace_hash = %q, want %q", doc.WorkspaceHash, wsHash)
	}
}

type stubIntSummarizer struct {
	fn func(ctx context.Context, md string, meta harvest.SummaryMeta) error
}

func (s *stubIntSummarizer) SummarizeAndPersist(ctx context.Context, md string, meta harvest.SummaryMeta) error {
	return s.fn(ctx, md, meta)
}
