package mcp_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

func mcpSecTestDSN() string {
	if v := os.Getenv("NANO_BRAIN_TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable"
}

func setupMCPSecTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(mcpSecTestDSN())
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	schema := fmt.Sprintf("test_mcp_%x", sha256.Sum256([]byte(t.Name())))[:18]
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

func setupMCPSecClient(t *testing.T, pgDB *sql.DB) (*mcpsdk.ClientSession, context.Context) {
	t.Helper()
	server := internalmcp.NewMCPServer("test-sec")
	adapter := internalmcp.NewAdapter(sqlc.New(pgDB), pgDB, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)

	ctx := context.Background()
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "sec-client", Version: "v0.0.1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session, ctx
}

func TestMemoryWrite_RejectsUnregisteredWorkspace(t *testing.T) {
	pgDB := setupMCPSecTestPG(t)
	session, ctx := setupMCPSecClient(t, pgDB)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_write",
		Arguments: map[string]any{
			"workspace": "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			"content":   "leak attempt",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for unregistered workspace — MCP bypass still open (leak #7)")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "workspace_not_registered") {
		t.Errorf("expected workspace_not_registered, got: %s", text)
	}
}

func TestMemoryWrite_AcceptsRegisteredWorkspace(t *testing.T) {
	pgDB := setupMCPSecTestPG(t)

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("registered-mcp-write")))
	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "mcp-write-positive",
		Path: "/tmp/mcp-write-positive",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	session, ctx := setupMCPSecClient(t, pgDB)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_write",
		Arguments: map[string]any{
			"workspace": wsHash,
			"content":   "# Positive test\nThis should write.",
			"title":     "Positive Write",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		text := result.Content[0].(*mcpsdk.TextContent).Text
		t.Fatalf("expected success for registered workspace, got error: %s", text)
	}
}

func TestMemoryUpdate_RejectsUnregisteredWorkspace(t *testing.T) {
	pgDB := setupMCPSecTestPG(t)
	session, ctx := setupMCPSecClient(t, pgDB)

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "memory_update",
		Arguments: map[string]any{
			"workspace": "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for unregistered workspace in memory_update")
	}
	text := result.Content[0].(*mcpsdk.TextContent).Text
	if !strings.Contains(text, "workspace_not_registered") {
		t.Errorf("expected workspace_not_registered, got: %s", text)
	}
}
