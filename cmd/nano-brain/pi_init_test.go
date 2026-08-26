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

func piInitTestDSN() string {
	if v := os.Getenv("NANO_BRAIN_TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"
}

func setupPiInitTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(piInitTestDSN())
	if err != nil {
		t.Skip("postgres not available: parse config: " + err.Error())
	}

	schema := fmt.Sprintf("test_pi_%x", sha256.Sum256([]byte(t.Name())))[:18]
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

func TestInitPiHarvesters_Disabled(t *testing.T) {
	cfg := config.PiHarvesterConfig{Enabled: false}
	phs, err := initPiHarvesters(context.Background(), cfg, nil, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(phs) != 0 {
		t.Error("expected no harvesters when disabled")
	}
}

func TestInitPiHarvesters_SessionDirMissing(t *testing.T) {
	cfg := config.PiHarvesterConfig{
		Enabled:    true,
		SessionDir: "/tmp/nano-brain-pi-test-does-not-exist-" + t.Name(),
	}
	phs, err := initPiHarvesters(context.Background(), cfg, nil, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(phs) != 0 {
		t.Error("expected no harvesters when session_dir missing")
	}
}

func TestInitPiHarvesters_NoRegisteredWorkspaceMatch(t *testing.T) {
	pgDB := setupPiInitTestPG(t)
	// sessionsRoot is the parent ~/.pi/agent/sessions/ equivalent.
	// No workspace is registered whose encoded path exists under sessionsRoot.
	sessionsRoot := filepath.Join(t.TempDir(), "pi-sessions")
	if err := os.MkdirAll(sessionsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.PiHarvesterConfig{Enabled: true, SessionDir: sessionsRoot}
	phs, err := initPiHarvesters(context.Background(), cfg, pgDB, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(phs) != 0 {
		t.Error("expected no harvesters when no registered workspace has a matching Pi session dir")
	}
}

func TestInitPiHarvesters_RegisteredWorkspace(t *testing.T) {
	pgDB := setupPiInitTestPG(t)

	sessionsRoot := filepath.Join(t.TempDir(), "pi-sessions")
	if err := os.MkdirAll(sessionsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	// workspacePath is an absolute path that will be registered as a workspace.
	// Its encoded form (replace / with -, wrap in leading/trailing --) must
	// exist as a subdir under sessionsRoot.
	workspacePath := "/fake/workspace/myproject"
	encoded := encodePiSessionsDir(workspacePath)
	sessionDir := filepath.Join(sessionsRoot, encoded)
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256([]byte(workspacePath))
	wsHash := hex.EncodeToString(h[:])
	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "test-pi",
		Path: workspacePath,
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	cfg := config.PiHarvesterConfig{Enabled: true, SessionDir: sessionsRoot}
	phs, err := initPiHarvesters(context.Background(), cfg, pgDB, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(phs) != 1 {
		t.Errorf("expected 1 harvester for registered workspace, got %d", len(phs))
	}
	if got := phs[0].WorkspaceHash(); got != wsHash {
		t.Errorf("harvester workspace hash = %q, want %q", got, wsHash)
	}
}

func TestInitPiHarvesters_MultipleWorkspaces(t *testing.T) {
	pgDB := setupPiInitTestPG(t)

	sessionsRoot := filepath.Join(t.TempDir(), "pi-sessions")
	if err := os.MkdirAll(sessionsRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	// Two workspaces with matching Pi session dirs.
	workspacePaths := []string{
		"/fake/workspace/alpha",
		"/fake/workspace/beta",
	}
	q := sqlc.New(pgDB)
	for _, wsPath := range workspacePaths {
		encoded := encodePiSessionsDir(wsPath)
		sessionDir := filepath.Join(sessionsRoot, encoded)
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256([]byte(wsPath))
		wsHash := hex.EncodeToString(h[:])
		if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
			Hash: wsHash,
			Name: filepath.Base(wsPath),
			Path: wsPath,
		}); err != nil {
			t.Fatalf("seed workspace %s: %v", wsPath, err)
		}
	}

	// Third workspace registered but no Pi session dir for it.
	h := sha256.Sum256([]byte("/fake/workspace/gamma"))
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: hex.EncodeToString(h[:]),
		Name: "gamma",
		Path: "/fake/workspace/gamma",
	}); err != nil {
		t.Fatalf("seed workspace gamma: %v", err)
	}

	cfg := config.PiHarvesterConfig{Enabled: true, SessionDir: sessionsRoot}
	phs, err := initPiHarvesters(context.Background(), cfg, pgDB, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(phs) != 2 {
		t.Errorf("expected 2 harvesters (alpha + beta), got %d", len(phs))
	}
}

func TestEncodePiSessionsDir(t *testing.T) {
	got := encodePiSessionsDir("/Users/tamlh/workspaces/self/AI/Tools/nano-brain")
	want := "--Users-tamlh-workspaces-self-AI-Tools-nano-brain--"
	if got != want {
		t.Errorf("encodePiSessionsDir = %q, want %q", got, want)
	}
}
