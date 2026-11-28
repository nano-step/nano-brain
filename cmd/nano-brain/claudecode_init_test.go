package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

func claudeInitTestDSN() string {
	if v := os.Getenv("NANO_BRAIN_TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"
}

func setupClaudeInitTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(claudeInitTestDSN())
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

func registerClaudeFixtureWorkspace(t *testing.T, q *sqlc.Queries, workspacePath string) string {
	t.Helper()
	h := sha256.Sum256([]byte(workspacePath))
	wsHash := hex.EncodeToString(h[:])
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: filepath.Base(workspacePath),
		Path: workspacePath,
	}); err != nil {
		t.Fatalf("seed workspace %s: %v", workspacePath, err)
	}
	return wsHash
}

func writeClaudeFixtureSession(t *testing.T, leafDir, sessionID, cwd, token string) {
	t.Helper()
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}
	message, _ := json.Marshal(map[string]any{"role": "user", "content": "remember " + token})
	line, _ := json.Marshal(map[string]any{
		"type":      "user",
		"timestamp": "2026-01-01T10:00:00Z",
		"cwd":       cwd,
		"message":   json.RawMessage(message),
	})
	content := string(line) + "\n"
	if err := os.WriteFile(filepath.Join(leafDir, sessionID+".jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initDynamicClaudeHarvester(t *testing.T, cfg config.ClaudeCodeHarvesterConfig, pgDB *sql.DB) harvest.Harvester {
	t.Helper()
	chs, err := initClaudeCodeHarvesters(context.Background(), cfg, pgDB, zerolog.Nop())
	if err != nil {
		t.Fatalf("initClaudeCodeHarvesters() error = %v", err)
	}
	if len(chs) != 1 {
		t.Fatalf("initClaudeCodeHarvesters() returned %d harvesters, want 1 dynamic harvester", len(chs))
	}
	return chs[0]
}

func requireSessionDocument(t *testing.T, q *sqlc.Queries, wsHash, sessionID, token string) {
	t.Helper()
	doc, err := q.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://claude/" + sessionID,
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("expected harvested session document: %v", err)
	}
	if !strings.Contains(doc.Content, token) {
		t.Fatalf("harvested document does not contain fixture token %q", token)
	}
}

func TestInitClaudeCodeHarvesters_Disabled(t *testing.T) {
	cfg := config.ClaudeCodeHarvesterConfig{Enabled: false}
	chs, err := initClaudeCodeHarvesters(context.Background(), cfg, nil, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(chs) != 0 {
		t.Error("expected no harvesters when disabled")
	}
}

func TestInitClaudeCodeHarvesters_MissingDirsRemainDynamic(t *testing.T) {
	cfg := config.ClaudeCodeHarvesterConfig{
		Enabled:    true,
		SessionDir: "/tmp/nano-brain-cc-test-does-not-exist-" + t.Name(),
	}
	chs, err := initClaudeCodeHarvesters(context.Background(), cfg, nil, zerolog.Nop())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(chs) != 1 {
		t.Fatalf("expected dynamic harvester even when the dir is currently missing, got %d", len(chs))
	}
}

func TestClaudeDiscovery_FirstSessionAfterStartup(t *testing.T) {
	pgDB := setupClaudeInitTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	instancesRoot := t.TempDir()
	projectsPattern := filepath.Join(instancesRoot, "instances", "*", "projects")
	profileProjects := filepath.Join(instancesRoot, "instances", "work", "projects")
	workspacePath := "/fake/workspace/dynamic"
	wsHash := registerClaudeFixtureWorkspace(t, q, workspacePath)
	h := initDynamicClaudeHarvester(t, config.ClaudeCodeHarvesterConfig{
		Enabled:     true,
		SessionDirs: []string{projectsPattern},
	}, pgDB)

	harvested, skipped, errCount := h.HarvestAll(ctx, nil)
	if harvested != 0 || skipped != 0 || errCount != 0 {
		t.Fatalf("initial harvest = (%d,%d,%d), want zeros before session leaf exists", harvested, skipped, errCount)
	}

	sessionID := "ses-dynamic-first"
	token := "ccs-dynamic-first-token"
	leafDir := filepath.Join(profileProjects, encodeClaudeProjectsDir(workspacePath))
	writeClaudeFixtureSession(t, leafDir, sessionID, workspacePath, token)

	harvested, _, errCount = h.HarvestAll(ctx, nil)
	if errCount != 0 || harvested != 1 {
		t.Fatalf("second harvest harvested=%d errors=%d, want 1/0 without restart", harvested, errCount)
	}
	requireSessionDocument(t, q, wsHash, sessionID, token)
	rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
		Query:         "ccs dynamic first token",
		WorkspaceHash: wsHash,
		MaxResults:    10,
	})
	if err != nil {
		t.Fatalf("BM25 fixture query: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("BM25 fixture query returned no rows for harvested CCS session")
	}
}

func TestClaudeDiscovery_WorkspaceRegisteredAfterStartup(t *testing.T) {
	pgDB := setupClaudeInitTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	projectsDir := filepath.Join(t.TempDir(), "projects")
	workspacePath := "/fake/workspace/late-registration"
	leafDir := filepath.Join(projectsDir, encodeClaudeProjectsDir(workspacePath))
	sessionID := "ses-late-registration"
	token := "ccs-late-registration-token"
	writeClaudeFixtureSession(t, leafDir, sessionID, workspacePath, token)

	h := initDynamicClaudeHarvester(t, config.ClaudeCodeHarvesterConfig{
		Enabled:    true,
		SessionDir: projectsDir,
	}, pgDB)
	harvested, _, errCount := h.HarvestAll(ctx, nil)
	if harvested != 0 || errCount != 0 {
		t.Fatalf("harvest before registration harvested=%d errors=%d, want 0/0", harvested, errCount)
	}

	wsHash := registerClaudeFixtureWorkspace(t, q, workspacePath)
	harvested, _, errCount = h.HarvestAll(ctx, nil)
	if errCount != 0 || harvested != 1 {
		t.Fatalf("harvest after registration harvested=%d errors=%d, want 1/0", harvested, errCount)
	}
	requireSessionDocument(t, q, wsHash, sessionID, token)
}

func TestClaudeDiscovery_GlobDedupesExplicitDuplicate(t *testing.T) {
	pgDB := setupClaudeInitTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	instancesRoot := t.TempDir()
	profileProjects := filepath.Join(instancesRoot, "instances", "work", "projects")
	workspacePath := "/fake/workspace/dedup"
	wsHash := registerClaudeFixtureWorkspace(t, q, workspacePath)
	leafDir := filepath.Join(profileProjects, encodeClaudeProjectsDir(workspacePath))
	sessionID := "ses-glob-dedup"
	token := "ccs-glob-dedup-token"
	writeClaudeFixtureSession(t, leafDir, sessionID, workspacePath, token)

	h := initDynamicClaudeHarvester(t, config.ClaudeCodeHarvesterConfig{
		Enabled:     true,
		SessionDir:  profileProjects,
		SessionDirs: []string{filepath.Join(instancesRoot, "instances", "*", "projects")},
	}, pgDB)
	harvested, skipped, errCount := h.HarvestAll(ctx, nil)
	if errCount != 0 || harvested != 1 || skipped != 0 {
		t.Fatalf("harvest = (%d harvested,%d skipped,%d errors), want (1,0,0) with duplicate leaf deduped", harvested, skipped, errCount)
	}
	requireSessionDocument(t, q, wsHash, sessionID, token)
}

func TestClaudeDiscovery_RejectsTranscriptCwdMismatch(t *testing.T) {
	pgDB := setupClaudeInitTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	projectsDir := filepath.Join(t.TempDir(), "projects")
	workspacePath := "/fake/workspace/bound"
	wsHash := registerClaudeFixtureWorkspace(t, q, workspacePath)
	leafDir := filepath.Join(projectsDir, encodeClaudeProjectsDir(workspacePath))
	sessionID := "ses-cwd-mismatch"
	writeClaudeFixtureSession(t, leafDir, sessionID, "/fake/workspace/other", "ccs-cwd-mismatch-token")

	h := initDynamicClaudeHarvester(t, config.ClaudeCodeHarvesterConfig{
		Enabled:    true,
		SessionDir: projectsDir,
	}, pgDB)
	harvested, skipped, errCount := h.HarvestAll(ctx, nil)
	if harvested != 0 || skipped != 1 || errCount != 0 {
		t.Fatalf("harvest = (%d,%d,%d), want (0,1,0) for cwd mismatch", harvested, skipped, errCount)
	}
	_, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://claude/" + sessionID,
		WorkspaceHash: wsHash,
	})
	if err == nil {
		t.Fatal("mismatched-cwd session must not be persisted")
	}
}

func TestClaudeDiscovery_EncodedCollisionUsesTranscriptCwd(t *testing.T) {
	pgDB := setupClaudeInitTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	projectsDir := filepath.Join(t.TempDir(), "projects")
	workspaceA := "/fake/workspace/a-b/c"
	workspaceB := "/fake/workspace/a/b-c"
	if encodeClaudeProjectsDir(workspaceA) != encodeClaudeProjectsDir(workspaceB) {
		t.Fatalf("fixture paths do not collide: %q vs %q", encodeClaudeProjectsDir(workspaceA), encodeClaudeProjectsDir(workspaceB))
	}
	hashA := registerClaudeFixtureWorkspace(t, q, workspaceA)
	hashB := registerClaudeFixtureWorkspace(t, q, workspaceB)

	leafDir := filepath.Join(projectsDir, encodeClaudeProjectsDir(workspaceA))
	writeClaudeFixtureSession(t, leafDir, "ses-collision-a", workspaceA, "ccs-collision-a-token")
	writeClaudeFixtureSession(t, leafDir, "ses-collision-b", workspaceB, "ccs-collision-b-token")

	h := initDynamicClaudeHarvester(t, config.ClaudeCodeHarvesterConfig{
		Enabled:    true,
		SessionDir: projectsDir,
	}, pgDB)
	harvested, _, errCount := h.HarvestAll(ctx, nil)
	if errCount != 0 || harvested != 2 {
		t.Fatalf("harvest harvested=%d errors=%d, want 2/0 — each colliding workspace should receive only its own cwd-matching session", harvested, errCount)
	}
	requireSessionDocument(t, q, hashA, "ses-collision-a", "ccs-collision-a-token")
	requireSessionDocument(t, q, hashB, "ses-collision-b", "ccs-collision-b-token")
}

func TestClaudeDiscovery_SymlinkAliasedWorkspaceDiscoversCanonicalLeaf(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation often requires extra privilege on windows")
	}
	pgDB := setupClaudeInitTestPG(t)
	q := sqlc.New(pgDB)
	ctx := context.Background()

	root := t.TempDir()
	targetWorkspace := filepath.Join(root, "target-workspace")
	if err := os.MkdirAll(targetWorkspace, 0o755); err != nil {
		t.Fatal(err)
	}
	aliasWorkspace := filepath.Join(root, "alias-workspace")
	if err := os.Symlink(targetWorkspace, aliasWorkspace); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	wsHash := registerClaudeFixtureWorkspace(t, q, aliasWorkspace)

	canonicalTarget, err := filepath.EvalSymlinks(targetWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	projectsDir := filepath.Join(t.TempDir(), "projects")
	leafDir := filepath.Join(projectsDir, encodeClaudeProjectsDir(canonicalTarget))
	writeClaudeFixtureSession(t, leafDir, "ses-symlink-alias", canonicalTarget, "ccs-symlink-alias-token")

	h := initDynamicClaudeHarvester(t, config.ClaudeCodeHarvesterConfig{
		Enabled:    true,
		SessionDir: projectsDir,
	}, pgDB)
	harvested, _, errCount := h.HarvestAll(ctx, nil)
	if errCount != 0 || harvested != 1 {
		t.Fatalf("harvest harvested=%d errors=%d, want 1/0 for canonical leaf behind symlink-registered workspace", harvested, errCount)
	}
	requireSessionDocument(t, q, wsHash, "ses-symlink-alias", "ccs-symlink-alias-token")
}
