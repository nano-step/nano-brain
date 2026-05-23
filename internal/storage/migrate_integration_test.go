//go:build integration

package storage

import (
	"context"
	"testing"

	"github.com/nano-brain/nano-brain/internal/testutil"
)

func TestMigrateCreatesTables(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()

	tables := []string{"workspaces", "documents", "chunks", "embeddings", "collections", "telemetry_logs"}
	for _, table := range tables {
		var exists bool
		err := pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=$1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("query table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s does not exist after migration", table)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	logger := testutil.NopLogger()

	if err := RunMigrations(ctx, pool, logger); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}
}

func TestVectorExtensionEnabled(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()

	var exists bool
	err := pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname='vector')",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("query vector extension: %v", err)
	}
	if !exists {
		t.Error("pgvector extension not enabled after migration")
	}
}
