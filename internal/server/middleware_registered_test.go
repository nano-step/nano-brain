package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
)

const wsRegTestDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"

func setupWsRegTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(wsRegTestDSN)
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	schema := fmt.Sprintf("test_mw_%x", sha256.Sum256([]byte(t.Name())))[:18]
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

func chainWorkspaceMiddlewares(db *sql.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return workspaceMiddleware(nil)(workspaceRegisteredMiddleware(db)(next))
	}
}

func TestWorkspaceRegisteredMiddleware_AcceptsRegistered(t *testing.T) {
	pgDB := setupWsRegTestPG(t)

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("registered")))
	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "mw-accepts",
		Path: "/tmp/mw-accepts",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	mw := chainWorkspaceMiddlewares(pgDB)
	body := fmt.Sprintf(`{"workspace":%q}`, wsHash)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusOK {
		t.Errorf("got HTTP %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceRegisteredMiddleware_RejectsUnregistered(t *testing.T) {
	pgDB := setupWsRegTestPG(t)

	mw := chainWorkspaceMiddlewares(pgDB)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"workspace":"unregistered-xyz"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got HTTP %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "workspace_not_registered" {
		t.Errorf("error = %q, want workspace_not_registered", resp["error"])
	}
}

func TestWorkspaceRegisteredMiddleware_RejectsAll(t *testing.T) {
	pgDB := setupWsRegTestPG(t)

	mw := chainWorkspaceMiddlewares(pgDB)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"workspace":"all"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got HTTP %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "workspace_all_not_supported" {
		t.Errorf("error = %q, want workspace_all_not_supported", resp["error"])
	}
}

func TestWorkspaceRegisteredMiddleware_RejectsEmpty(t *testing.T) {
	pgDB := setupWsRegTestPG(t)

	mw := chainWorkspaceMiddlewares(pgDB)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := applyMiddleware(mw, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got HTTP %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}
