//go:build integration

package watcher

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// TestWarmFileCache proves that:
//  1. A file indexed by watcher A has non-null mod_time/file_size in the DB.
//  2. A fresh watcher B (simulated restart) skips the unchanged file via DB-warmed cache (updated_at unchanged).
//  3. A real modification to the file causes watcher B to re-index it (updated_at advances).
func TestWarmFileCache(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	// Create a temp dir with one source file.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("initial content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	wsHash, err := storage.WorkspaceHash(dir)
	if err != nil {
		t.Fatalf("WorkspaceHash: %v", err)
	}

	// Register workspace and collection.
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Path: dir,
		Name: "test-warm",
	}); err != nil {
		t.Fatalf("UpsertWorkspace: %v", err)
	}

	cfg := config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs:      0,
			ReindexInterval: 3600,
			ChunkOverlap:    0,
		},
		Storage: config.StorageConfig{
			MaxFileSize: 10 * 1024 * 1024,
			MaxSize:     100 * 1024 * 1024,
		},
	}
	logger := zerolog.Nop()

	// --- Watcher A: initial index ---
	wA := New(db, q, logger, cfg)
	col := watchedCollection{
		name:          "code",
		dirPath:       dir,
		workspaceHash: wsHash,
	}
	wA.scanCollection(ctx, col)

	// Assert the document was indexed with non-null mod_time + file_size.
	doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    filePath,
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("GetDocumentBySourcePath after watcher A: %v", err)
	}
	// Query mod_time and file_size directly to verify they are set.
	var modTimeNull sql.NullTime
	var fileSizeNull sql.NullInt64
	row := db.QueryRowContext(ctx,
		`SELECT mod_time, file_size FROM documents WHERE id = $1`, doc.ID)
	if err := row.Scan(&modTimeNull, &fileSizeNull); err != nil {
		t.Fatalf("scan mod_time/file_size: %v", err)
	}
	if !modTimeNull.Valid {
		t.Error("expected mod_time to be non-null after first index")
	}
	if !fileSizeNull.Valid {
		t.Error("expected file_size to be non-null after first index")
	}

	// Capture updated_at before restart simulation.
	var updatedAtBefore time.Time
	rowUA := db.QueryRowContext(ctx, `SELECT updated_at FROM documents WHERE id = $1`, doc.ID)
	if err := rowUA.Scan(&updatedAtBefore); err != nil {
		t.Fatalf("scan updated_at before: %v", err)
	}

	// --- Watcher B: simulated restart (fresh watcher, same DB+queries) ---
	wB := New(db, q, logger, cfg)
	wB.scanCollection(ctx, col)

	// Assert the file cache was warmed (watcher B's cache has the entry).
	wB.fileCacheMu.RLock()
	_, cached := wB.fileCache[filePath]
	wB.fileCacheMu.RUnlock()
	if !cached {
		t.Error("expected watcher B fileCache to contain the file after warmFileCacheFromDB")
	}

	// Assert updated_at did NOT change (file was skipped — no re-index).
	var updatedAtAfterRestart time.Time
	rowUA2 := db.QueryRowContext(ctx, `SELECT updated_at FROM documents WHERE id = $1`, doc.ID)
	if err := rowUA2.Scan(&updatedAtAfterRestart); err != nil {
		t.Fatalf("scan updated_at after restart: %v", err)
	}
	if !updatedAtAfterRestart.Equal(updatedAtBefore) {
		t.Errorf("unchanged file was re-indexed on restart: updated_at changed from %v to %v",
			updatedAtBefore, updatedAtAfterRestart)
	}

	// --- Mutation: modify the file so mtime+size change ---
	// Sleep 1ms to ensure mtime differs (filesystem resolution).
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filePath, []byte("modified content — different"), 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}

	// Watcher B re-scans (same instance, warmed guard already true — but the
	// fast-path checks mtime+size vs actual os.Stat, so changed files still fall through).
	wB.scanCollection(ctx, col)

	var updatedAtAfterChange time.Time
	rowUA3 := db.QueryRowContext(ctx, `SELECT updated_at FROM documents WHERE id = $1`, doc.ID)
	if err := rowUA3.Scan(&updatedAtAfterChange); err != nil {
		t.Fatalf("scan updated_at after change: %v", err)
	}
	if !updatedAtAfterChange.After(updatedAtBefore) {
		t.Errorf("modified file was NOT re-indexed: updated_at %v not after %v",
			updatedAtAfterChange, updatedAtBefore)
	}
}
