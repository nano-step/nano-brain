//go:build integration

package storage_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

func setupWorkspace(t *testing.T, queries *sqlc.Queries, ctx context.Context, hash, name, path string) {
	t.Helper()
	if _, err := queries.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: hash,
		Name: name,
		Path: path,
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}
}

func TestResolveWorkspaceParam_Name(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	queries := sqlc.New(db)

	hash := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	setupWorkspace(t, queries, ctx, hash, "resolve-name-test-ws", "/projects/resolve-name-test")

	got, err := storage.ResolveWorkspaceParam(ctx, queries, "resolve-name-test-ws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != hash {
		t.Errorf("got %q, want %q", got, hash)
	}
}

func TestResolveWorkspaceParam_CaseInsensitive(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	queries := sqlc.New(db)

	hash := "b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3"
	setupWorkspace(t, queries, ctx, hash, "Resolve-CI-Test-WS", "/projects/resolve-ci-test")

	got, err := storage.ResolveWorkspaceParam(ctx, queries, "resolve-ci-test-ws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != hash {
		t.Errorf("got %q, want %q", got, hash)
	}
}

func TestResolveWorkspaceParam_Prefix(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	queries := sqlc.New(db)

	hash := "c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4"
	setupWorkspace(t, queries, ctx, hash, "prefix-test-ws", "/projects/prefix-test")

	got, err := storage.ResolveWorkspaceParam(ctx, queries, "c3d4e5f6")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != hash {
		t.Errorf("got %q, want %q", got, hash)
	}
}

func TestResolveWorkspaceParam_AmbiguousPrefix(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	queries := sqlc.New(db)

	// Two hashes sharing the same 8-char prefix "dddddddd"
	hash1 := "dddddddd11111111111111111111111111111111111111111111111111111111"
	hash2 := "dddddddd22222222222222222222222222222222222222222222222222222222"
	setupWorkspace(t, queries, ctx, hash1, "ws-ambig-1", "/projects/ambig1")
	setupWorkspace(t, queries, ctx, hash2, "ws-ambig-2", "/projects/ambig2")

	_, err := storage.ResolveWorkspaceParam(ctx, queries, "dddddddd")
	if err == nil {
		t.Fatal("expected error for ambiguous prefix, got nil")
	}
}

func TestResolveWorkspaceParam_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	queries := sqlc.New(db)

	_, err := storage.ResolveWorkspaceParam(ctx, queries, "does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown workspace name, got nil")
	}
}

func TestResolveWorkspaceParam_PrefixNotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	queries := sqlc.New(db)

	_, err := storage.ResolveWorkspaceParam(ctx, queries, "00000000")
	if err == nil {
		t.Fatal("expected error for unknown hash prefix, got nil")
	}
}
