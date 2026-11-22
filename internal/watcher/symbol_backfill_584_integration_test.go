//go:build integration

package watcher

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/symbol"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// symbolRegistry584 mirrors the extractor set wired in cmd/nano-brain/main.go.
func symbolRegistry584(t *testing.T) *symbol.Registry {
	t.Helper()
	var ex []symbol.Extractor
	if goE, err := symbol.NewGoExtractor(); err != nil {
		t.Fatalf("go symbol extractor init: %v", err)
	} else {
		ex = append(ex, goE)
	}
	if tsE, err := symbol.NewTypeScriptExtractor(); err != nil {
		t.Fatalf("typescript symbol extractor init: %v", err)
	} else {
		ex = append(ex, tsE)
	}
	if pyE, err := symbol.NewPythonExtractor(); err != nil {
		t.Fatalf("python symbol extractor init: %v", err)
	} else {
		ex = append(ex, pyE)
	}
	if jsE, err := symbol.NewJavaScriptExtractor(); err != nil {
		t.Fatalf("javascript symbol extractor init: %v", err)
	} else {
		ex = append(ex, jsE)
	}
	if rbE, err := symbol.NewRubySymbolExtractor(); err != nil {
		t.Fatalf("ruby symbol extractor init: %v", err)
	} else {
		ex = append(ex, rbE)
	}
	if vueE, err := symbol.NewVueExtractor(); err != nil {
		t.Fatalf("vue symbol extractor init: %v", err)
	} else {
		ex = append(ex, vueE)
	}
	return symbol.NewRegistry(ex...)
}

func TestWatcherBackfillsStaleSymbolSpans(t *testing.T) {
	ctx := context.Background()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	lines := []string{"import { db } from './db';", ""}
	declLine := len(lines) + 1
	lines = append(lines, "export const foo = async (a: string, b: number, c: boolean): Promise<void> => {")
	for i := 0; i < 160; i++ {
		lines = append(lines, fmt.Sprintf("  const v%d = await db.query(%d);", i, i))
	}
	lines = append(lines, "};")
	endLine := len(lines)

	handlersPath := filepath.Join(root, "src", "handlers.ts")
	otherPath := filepath.Join(root, "src", "other.ts")
	if err := os.MkdirAll(filepath.Dir(handlersPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(handlersPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte("export function bar(x: number): number {\n  return x * 2;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ws := fmt.Sprintf("%x", sha256.Sum256([]byte("ws584_"+uuid.NewString())))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: ws, Name: "ws584", Path: root}); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		Watcher: config.WatcherConfig{DebounceMs: 10, ReindexInterval: 3600},
		Storage: config.StorageConfig{MaxFileSize: 1 << 20},
	}
	reg := symbolRegistry584(t)

	runWatcher := func() (context.CancelFunc, chan error) {
		t.Helper()
		w := New(db, q, zerolog.Nop(), cfg).WithSymbolRegistry(reg)
		if err := w.Watch("code", root, ws, "**/*"); err != nil {
			t.Fatal(err)
		}
		runCtx, cancel := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- w.Run(runCtx) }()
		return cancel, done
	}

	fooPath := handlersPath + "?symbol=foo&kind=function"
	barPath := otherPath + "?symbol=bar&kind=function"

	symbolMeta := func(sourcePath string) (map[string]string, error) {
		doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sourcePath,
			WorkspaceHash: ws,
		})
		if err != nil {
			return nil, err
		}
		var meta map[string]string
		if doc.Metadata.Valid {
			if err := json.Unmarshal(doc.Metadata.RawMessage, &meta); err != nil {
				return nil, err
			}
		}
		return meta, nil
	}

	waitSpan := func(sourcePath string) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for {
			meta, err := symbolMeta(sourcePath)
			if err == nil && meta["end_line"] != "" {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s to carry end_line (err=%v)", sourcePath, err)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}

	cancel1, done1 := runWatcher()
	waitSpan(fooPath)
	waitSpan(barPath)
	barBefore, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    barPath,
		WorkspaceHash: ws,
	})
	if err != nil {
		t.Fatal(err)
	}
	cancel1()
	if err := <-done1; err != nil {
		t.Fatalf("watcher: %v", err)
	}

	// Simulate a pre-#539 index: symbol docs written before line/end_line
	// metadata existed keep only the signature. documents is a table (v30+),
	// so UPDATE applies directly; starts_with avoids LIKE wildcards.
	if _, err := db.ExecContext(ctx,
		`UPDATE documents SET metadata = metadata - 'line' - 'end_line'
		 WHERE workspace_hash = $1 AND starts_with(source_path, $2)`,
		ws, handlersPath+"?symbol="); err != nil {
		t.Fatal(err)
	}
	if meta, err := symbolMeta(fooPath); err != nil || meta["end_line"] != "" {
		t.Fatalf("stale-span setup failed: meta=%v err=%v", meta, err)
	}

	// A fresh watcher on the unchanged tree must backfill the missing spans:
	// the content-hash fast path skips re-indexing, so only the startup
	// backfill can repair them.
	cancel2, done2 := runWatcher()
	t.Cleanup(func() {
		cancel2()
		if err := <-done2; err != nil {
			t.Errorf("watcher: %v", err)
		}
	})
	waitSpan(fooPath)

	meta, err := symbolMeta(fooPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := meta["line"]; got != strconv.Itoa(declLine) {
		t.Fatalf("line = %q, want %d", got, declLine)
	}
	if got := meta["end_line"]; got != strconv.Itoa(endLine) {
		t.Fatalf("end_line = %q, want %d", got, endLine)
	}

	// The untouched second file must not be re-extracted: a blanket
	// re-extract would bump its symbol document's updated_at.
	barAfter, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    barPath,
		WorkspaceHash: ws,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !barAfter.UpdatedAt.Equal(barBefore.UpdatedAt) {
		t.Fatalf("untouched file re-extracted: updated_at %v -> %v", barBefore.UpdatedAt, barAfter.UpdatedAt)
	}
}
