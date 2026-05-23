//go:build integration

package testutil

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
)

const testDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"

// SetupTestDB connects to the test PG, creates an isolated schema, runs
// migrations, and registers cleanup. No import of internal/storage to avoid
// import cycles (storage tests use this helper).
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()

	poolCfg, err := pgxpool.ParseConfig(testDSN)
	if err != nil {
		t.Fatalf("SetupTestDB: parse config: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("SetupTestDB: connect: %v", err)
	}

	schema := testSchema(t.Name())
	mustExec(t, ctx, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)
	mustExec(t, ctx, pool, "SET search_path TO "+schema+", public")

	if err := runTestMigrations(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("SetupTestDB: migrate: %v", err)
	}

	t.Cleanup(func() {
		cleanCtx := context.Background()
		mustExec(t, cleanCtx, pool, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})

	return pool
}

func runTestMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	defer func(db *sql.DB) { _ = db.Close() }(db)
	return goose.UpContext(ctx, db, ".")
}

func testSchema(name string) string {
	h := sha256.Sum256([]byte(name))
	return "test_" + fmt.Sprintf("%x", h[:6])
}

func mustExec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql); err != nil {
		t.Fatalf("mustExec %q: %v", strings.TrimSpace(sql), err)
	}
}
