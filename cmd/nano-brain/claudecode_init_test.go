package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

const claudeInitTestDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"

func setupClaudeInitTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(claudeInitTestDSN)
	if err != nil {
		t.Skip("postgres not available: parse config: " + err.Error())
	}

	schema := fmt.Sprintf("test_cc_%x", sha256.Sum256([]byte(t.Name())))[:18]
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

func TestInitClaudeCodeHarvester_Disabled(t *testing.T) {
	cfg := config.ClaudeCodeHarvesterConfig{Enabled: false}
	ch, err := initClaudeCodeHarvester(context.Background(), cfg, nil, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ch != nil {
		t.Error("expected nil harvester when disabled")
	}
}

func TestInitClaudeCodeHarvester_SessionDirMissing(t *testing.T) {
	cfg := config.ClaudeCodeHarvesterConfig{
		Enabled:    true,
		SessionDir: "/tmp/nano-brain-cc-test-does-not-exist-" + t.Name(),
	}
	ch, err := initClaudeCodeHarvester(context.Background(), cfg, nil, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ch != nil {
		t.Error("expected nil harvester when session_dir missing")
	}
}

func TestInitClaudeCodeHarvester_UnregisteredWorkspace(t *testing.T) {
	pgDB := setupClaudeInitTestPG(t)
	sessionDir := filepath.Join(t.TempDir(), "claude-sessions-unregistered")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.ClaudeCodeHarvesterConfig{Enabled: true, SessionDir: sessionDir}
	ch, err := initClaudeCodeHarvester(context.Background(), cfg, pgDB, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ch != nil {
		t.Error("expected nil harvester when workspace not registered — leak #2 still open")
	}
}

func TestInitClaudeCodeHarvester_RegisteredWorkspace(t *testing.T) {
	pgDB := setupClaudeInitTestPG(t)
	sessionDir := filepath.Join(t.TempDir(), "claude-sessions-registered")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256([]byte(sessionDir))
	wsHash := hex.EncodeToString(h[:])
	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "test-cc",
		Path: sessionDir,
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	cfg := config.ClaudeCodeHarvesterConfig{Enabled: true, SessionDir: sessionDir}
	ch, err := initClaudeCodeHarvester(context.Background(), cfg, pgDB, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if ch == nil {
		t.Error("expected non-nil harvester when workspace is registered")
	}
}
