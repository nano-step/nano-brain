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
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

func TestReextractEdgesForWorkspaceReplacesSourceEdges(t *testing.T) {
	ctx := context.Background()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	root := t.TempDir()
	api := filepath.Join(root, "repo-a", "lib", "api.ts")
	consumer := filepath.Join(root, "repo-a", "consumer.ts")
	collision := filepath.Join(root, "repo-b", "lib", "api.ts")
	for path, content := range map[string]string{
		api:       "export function run() { return 1 }\n",
		collision: "export function run() { return 2 }\n",
		consumer:  "import { run } from './lib/api'; export function caller() { return run() }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ws, err := storage.WorkspaceHash(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: ws, Path: root, Name: "source-scoped-fixture"}); err != nil {
		t.Fatal(err)
	}
	reg := graph.NewRegistry(mustJavaScriptExtractor(t), mustTypeScriptExtractor(t))
	w := New(db, q, zerolog.Nop(), config.Config{Storage: config.StorageConfig{MaxFileSize: 1 << 20}}).WithGraphRegistry(reg, q)
	if err := w.WatchWithFilter("code", root, ws, "**/*", nil, []string{".ts"}); err != nil {
		t.Fatal(err)
	}
	col := watchedCollection{dirPath: root, workspaceHash: ws, name: "code"}
	w.scanCollection(ctx, col)
	assertConsumerTarget(t, q, ctx, ws, "repo-a/lib/api.ts::run")
	first := countSourceEdges(t, q, ctx, ws, "repo-a/consumer.ts")
	if got := w.ReextractEdgesForWorkspace(ctx, ws); got == 0 {
		t.Fatal("first re-extraction processed no files")
	}
	if got := w.ReextractEdgesForWorkspace(ctx, ws); got == 0 {
		t.Fatal("second re-extraction processed no files")
	}
	if got := countSourceEdges(t, q, ctx, ws, "repo-a/consumer.ts"); got != first {
		t.Fatalf("idempotent re-extraction duplicated edges: before=%d after=%d", first, got)
	}

	if err := os.WriteFile(consumer, []byte("import { run } from './lib/api'; export function caller() { return missing() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.handleFSEventForTest(ctx, consumer, fsnotify.Write)
	assertConsumerTarget(t, q, ctx, ws, "<unresolved>")
	if got := countSourceEdges(t, q, ctx, ws, "repo-b/lib/api.ts"); got == 0 {
		t.Fatal("collision exporter edge was not preserved")
	}

	if err := os.WriteFile(consumer, []byte("import { run } from './lib/api'; export function caller() { return run() }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.handleFSEventForTest(ctx, consumer, fsnotify.Write)
	assertConsumerTarget(t, q, ctx, ws, "repo-a/lib/api.ts::run")

	if err := os.Rename(api, api+".gone"); err != nil {
		t.Fatal(err)
	}
	w.handleFSEventForTest(ctx, api, fsnotify.Rename)
	assertConsumerTarget(t, q, ctx, ws, "<unresolved>")

	if err := os.WriteFile(api, []byte("export function run() { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.handleFSEventForTest(ctx, api, fsnotify.Create)
	assertConsumerTarget(t, q, ctx, ws, "repo-a/lib/api.ts::run")

	if err := os.Remove(api); err != nil {
		t.Fatal(err)
	}
	w.handleFSEventForTest(ctx, api, fsnotify.Remove)
	assertConsumerTarget(t, q, ctx, ws, "<unresolved>")
}

func mustJavaScriptExtractor(t *testing.T) graph.Extractor {
	t.Helper()
	ex, err := graph.NewJavaScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	return ex
}
func mustTypeScriptExtractor(t *testing.T) graph.Extractor {
	t.Helper()
	ex, err := graph.NewTypeScriptGraphExtractor()
	if err != nil {
		t.Fatal(err)
	}
	return ex
}

func (w *Watcher) handleFSEventForTest(ctx context.Context, path string, op fsnotify.Op) {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	w.handleFSEvent(fsnotify.Event{Name: path, Op: op}, timer)
	w.processDirty(ctx)
}

func assertConsumerTarget(t *testing.T, q *sqlc.Queries, ctx context.Context, ws, want string) {
	t.Helper()
	rows, err := q.ListAllEdgesByWorkspace(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range rows {
		if e.SourceFile == "repo-a/consumer.ts" && e.EdgeType == "calls" {
			if e.TargetNode != want {
				t.Fatalf("consumer target=%q, want %q", e.TargetNode, want)
			}
			return
		}
	}
	t.Fatalf("no consumer calls edge found")
}

func countSourceEdges(t *testing.T, q *sqlc.Queries, ctx context.Context, ws, source string) int {
	t.Helper()
	rows, err := q.ListAllEdgesByWorkspace(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range rows {
		if e.SourceFile == source {
			n++
		}
	}
	return n
}
