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

	// Wait for the initial scan to install its watch, then remove only the
	// fsnotify registration so file creation cannot wake the idle watcher.
	deadline := time.Now().Add(5 * time.Second)
	for {
		w.mu.Lock()
		ready := w.fsw != nil && w.watchedDirs[root]
		if ready {
			err := w.fsw.Remove(root)
			delete(w.watchedDirs, root)
			w.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			break
		}
		w.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("initial watcher scan did not register the source root")
		}
		time.Sleep(10 * time.Millisecond)
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
