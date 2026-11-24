//go:build integration

package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// TestWorkspaceIsolation714 proves the #714 fixes end to end:
//   - two workspaces may register collections on the SAME path and are
//     indexed independently (no state collisions in collections/dirty/
//     warmed/fileCache);
//   - UnwatchWorkspace(A) removes only A's state and keeps B fully watched;
//   - ensure_workspace_short_id raises for an unregistered/deleted workspace
//     instead of resurrecting its workspaces row on a stale graph upsert.
func TestWorkspaceIsolation714(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	q := sqlc.New(db)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "shared.txt")
	if err := os.WriteFile(filePath, []byte("shared content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Two workspaces on the same path — synthetic hashes per repo privacy rules.
	const wsA, wsB = "ws-a", "ws-b"
	for _, h := range []string{wsA, wsB} {
		if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
			Hash: h, Name: "fixture-" + h, Path: dir,
		}); err != nil {
			t.Fatalf("UpsertWorkspace %s: %v", h, err)
		}
	}

	cfg := config.Config{
		Watcher: config.WatcherConfig{DebounceMs: 10, ReindexInterval: 3600},
		Storage: config.StorageConfig{MaxFileSize: 1 << 20},
	}
	w := New(db, q, zerolog.Nop(), cfg)

	// Real fsnotify watcher so Watch marks dirty + registers watchedDirs,
	// exactly as it does once Run() is live.
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("fsnotify.NewWatcher: %v", err)
	}
	defer fsw.Close()
	w.fsw = fsw

	if err := w.Watch("code", dir, wsA, "*"); err != nil {
		t.Fatalf("Watch ws-a: %v", err)
	}
	if err := w.Watch("code", dir, wsB, "*"); err != nil {
		t.Fatalf("Watch ws-b: %v", err)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	keyA := collectionKey{workspaceHash: wsA, dirPath: absDir}
	keyB := collectionKey{workspaceHash: wsB, dirPath: absDir}

	// Both collections registered; the physical dir watch is shared (added once).
	w.mu.Lock()
	if len(w.collections) != 2 {
		w.mu.Unlock()
		t.Fatalf("expected 2 watched collections, got %d", len(w.collections))
	}
	if !w.dirty[keyA] || !w.dirty[keyB] {
		w.mu.Unlock()
		t.Fatal("expected both composite dirty keys after Watch")
	}
	if !w.watchedDirs[absDir] {
		w.mu.Unlock()
		t.Fatal("expected shared dir watch to be registered")
	}
	w.mu.Unlock()

	// Drain dirty → both workspaces index the same file independently.
	w.processDirty(ctx)

	for _, h := range []string{wsA, wsB} {
		if _, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath: filePath, WorkspaceHash: h,
		}); err != nil {
			t.Fatalf("workspace %s did not index shared file: %v", h, err)
		}
	}

	w.fileCacheMu.RLock()
	inner := w.fileCache[filePath]
	w.fileCacheMu.RUnlock()
	if len(inner) != 2 {
		t.Fatalf("expected fileCache entry for both workspaces, got %v", inner)
	}
	for _, h := range []string{wsA, wsB} {
		if _, ok := inner[h]; !ok {
			t.Fatalf("fileCache missing workspace %s entry", h)
		}
	}

	w.mu.Lock()
	if !w.warmed[keyA] || !w.warmed[keyB] {
		w.mu.Unlock()
		t.Fatal("expected warmed entries for both workspaces")
	}
	w.mu.Unlock()

	// --- UnwatchWorkspace(A): B must remain fully watched ---
	if err := w.UnwatchWorkspace(wsA); err != nil {
		t.Fatalf("UnwatchWorkspace: %v", err)
	}

	w.mu.Lock()
	if _, ok := w.collections[keyA]; ok {
		w.mu.Unlock()
		t.Fatal("ws-a collection still registered after UnwatchWorkspace")
	}
	if _, ok := w.collections[keyB]; !ok {
		w.mu.Unlock()
		t.Fatal("ws-b collection was removed by UnwatchWorkspace(ws-a)")
	}
	if w.dirty[keyA] || w.warmed[keyA] {
		w.mu.Unlock()
		t.Fatal("ws-a dirty/warmed state survived UnwatchWorkspace")
	}
	if !w.watchedDirs[absDir] {
		w.mu.Unlock()
		t.Fatal("shared dir watch pruned while ws-b still needs it")
	}
	w.mu.Unlock()

	w.fileCacheMu.RLock()
	inner = w.fileCache[filePath]
	w.fileCacheMu.RUnlock()
	if _, ok := inner[wsA]; ok {
		t.Fatal("ws-a fileCache entry survived UnwatchWorkspace")
	}
	if _, ok := inner[wsB]; !ok {
		t.Fatal("ws-b fileCache entry was dropped by UnwatchWorkspace(ws-a)")
	}

	// A later fs event must dirty only ws-b's composite key.
	debounce := time.NewTimer(time.Hour)
	defer debounce.Stop()
	w.handleFSEvent(fsnotify.Event{Name: filePath, Op: fsnotify.Write}, debounce)
	w.mu.Lock()
	if w.dirty[keyA] {
		w.mu.Unlock()
		t.Fatal("event dirtied removed ws-a collection")
	}
	if !w.dirty[keyB] {
		w.mu.Unlock()
		t.Fatal("event did not dirty surviving ws-b collection")
	}
	w.mu.Unlock()
	w.processDirty(ctx)

	// --- ensure_workspace_short_id must not resurrect a deleted workspace ---
	const wsGhost = "ws-ghost"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsGhost, Name: "fixture-ghost", Path: dir,
	}); err != nil {
		t.Fatalf("UpsertWorkspace ghost: %v", err)
	}
	if err := q.DeleteWorkspace(ctx, wsGhost); err != nil {
		t.Fatalf("DeleteWorkspace ghost: %v", err)
	}

	// A stale graph upsert for the deleted workspace must fail, not recreate it.
	if err := q.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
		WorkspaceHash: wsGhost,
		SourceNode:    "ghost.go::Ghost",
		TargetNode:    "outside",
		EdgeType:      "calls",
		SourceFile:    "ghost.go",
		Metadata:      []byte("{}"),
	}); err == nil {
		t.Fatal("UpsertGraphEdge for deleted workspace succeeded — ensure_workspace_short_id resurrected it")
	}

	var resurrected int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM workspaces WHERE hash = $1`, wsGhost).Scan(&resurrected); err != nil {
		t.Fatalf("count workspaces: %v", err)
	}
	if resurrected != 0 {
		t.Fatalf("deleted workspace row was recreated: count=%d", resurrected)
	}

	// Direct check: the function itself raises for an unregistered hash.
	if _, err := db.ExecContext(ctx, `SELECT ensure_workspace_short_id($1)`, "ws-never-registered"); err == nil {
		t.Fatal("ensure_workspace_short_id did not raise for unregistered workspace")
	}
}
