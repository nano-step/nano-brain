//go:build integration

package watcher

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

func TestTriggerRescanWakesIdleWatcher(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	q := sqlc.New(db)
	root := t.TempDir()
	initialPath := filepath.Join(root, "initial.go")
	if err := os.WriteFile(initialPath, []byte("package fixture\nfunc Initial() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	workspace := "ws-reindex-wakeup"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: workspace, Name: "fixture", Path: root,
	}); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Watcher: config.WatcherConfig{DebounceMs: 10, ReindexInterval: 3600},
		Storage: config.StorageConfig{MaxFileSize: 1 << 20},
	}
	w := New(db, q, zerolog.Nop(), cfg)
	if err := w.Watch("code", root, workspace, "**/*"); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("watcher: %v", err)
		}
	})

	// Seeing the initial document alone is not a barrier: the initial scan
	// may still be in progress. The sentinel is consumed by Run only after
	// processAll returns, establishing that the first pass has finished.
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath: initialPath, WorkspaceHash: workspace,
		})
		if err == nil {
			break
		}
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("initial watcher scan did not ingest its marker file")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w.hotRegisterCh <- struct{}{}
	deadline = time.Now().Add(5 * time.Second)
	for len(w.hotRegisterCh) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("initial watcher scan did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	w.mu.Lock()
	if w.fsw == nil || !w.watchedDirs[root] {
		w.mu.Unlock()
		t.Fatal("watcher did not register source root")
	}
	err := w.fsw.Remove(root)
	delete(w.watchedDirs, root)
	w.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(root, "late.go")
	if err := os.WriteFile(path, []byte("package fixture\nfunc Late() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lookup := func() error {
		_, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath: path, WorkspaceHash: workspace,
		})
		return err
	}
	if err := lookup(); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("file was ingested before reindex without a filesystem watch: %v", err)
	}
	if !w.TriggerRescanByName("code", workspace) {
		t.Fatal("registered collection was not marked dirty")
	}
	deadline = time.Now().Add(5 * time.Second)
	for {
		if err := lookup(); err == nil {
			return
		} else if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("look up reindexed file: %v", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("reindex did not wake the idle watcher and ingest the file")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
