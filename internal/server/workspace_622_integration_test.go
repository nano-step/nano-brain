//go:build integration

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

// TestFreshWorkspaceHTTP covers workspace middleware and statistics as well
// as watcher persistence, which a direct handler test cannot exercise.
func TestFreshWorkspaceHTTP(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	for name, content := range map[string]string{
		"repo.js":       "export const price = 42;\n",
		"service.ts":    "export function getPrice() { return 42; }\n",
		"component.vue": "<template><p>price</p></template>\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	cfg := config.Config{
		Watcher: config.WatcherConfig{DebounceMs: 10, ReindexInterval: 3600},
		Storage: config.StorageConfig{MaxFileSize: 1 << 20},
	}
	w := watcher.New(db, q, zerolog.Nop(), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("watcher: %v", err)
		}
	})
	s := New(&cfg, "", pool, db, q, w, nil, nil, nil, zerolog.Nop(), "test", 30, nil)
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)

	post := func(path string, payload any) map[string]any {
		t.Helper()
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.Post(ts.URL+path, "application/json", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var data map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			t.Fatalf("%s returned %d: %v", path, resp.StatusCode, data)
		}
		return data
	}

	init := post("/api/v1/init", map[string]string{"root_path": root})
	hash, ok := init["workspace_hash"].(string)
	if !ok || hash == "" {
		t.Fatalf("init returned no hash: %v", init)
	}
	reindex := post("/api/v1/reindex", map[string]string{"workspace": hash})
	if reindex["chunks_enqueued"] != float64(0) || reindex["embedded"] != float64(0) {
		t.Fatalf("asynchronous reindex claimed completed/queued chunks: %v", reindex)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("%s/api/v1/stats?workspace=%s", ts.URL, hash))
		if err != nil {
			t.Fatal(err)
		}
		var stats map[string]any
		err = json.NewDecoder(resp.Body).Decode(&stats)
		_ = resp.Body.Close()
		if err != nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("stats returned %d: %v, %v", resp.StatusCode, stats, err)
		}
		if stats["docs_total"].(float64) >= 3 && stats["chunks_total"].(float64) >= 3 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("a fresh registered workspace did not persist its three source files")
}
