//go:build integration

package mcp_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/zerolog"

	"github.com/nano-brain/nano-brain/internal/config"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/symbol"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/nano-brain/nano-brain/internal/watcher"
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

// arrowBody584 returns n lines of indented statement body.
func arrowBody584(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("  const v%d = await db.query(%d);", i, i))
	}
	return out
}

// handlersFile584 builds src/handlers.ts: an optional object literal whose
// `foo` property collides with the exported const `foo` below it, then the
// 150+-line arrow body. Returns the source and foo's 1-based line span.
func handlersFile584(withObjectDup bool) (string, int, int) {
	lines := []string{"import { db } from './db';", ""}
	if withObjectDup {
		lines = append(lines,
			"export const handlers = {",
			"  foo: async (a) => {",
			"    return a;",
			"  },",
			"};",
			"",
		)
	}
	declLine := len(lines) + 1
	lines = append(lines, "export const foo = async (a: string, b: number, c: boolean): Promise<void> => {")
	lines = append(lines, arrowBody584(160)...)
	lines = append(lines, "};")
	return strings.Join(lines, "\n") + "\n", declLine, len(lines)
}

type mcp584Env struct {
	ctx    context.Context
	q      *sqlc.Queries
	db     *sql.DB
	wsHash string
	root   string
	cfg    config.Config
	reg    *symbol.Registry
	call   func(string, map[string]any) *mcpsdk.CallToolResult
}

func setup584(t *testing.T, files map[string]string) *mcp584Env {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })
	q := sqlc.New(db)

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wsHash := fmt.Sprintf("%x", sha256.Sum256([]byte("ws584_"+uuid.New().String())))
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: wsHash, Name: "ws584", Path: root}); err != nil {
		t.Fatal(err)
	}

	server := internalmcp.NewMCPServer("test")
	adapter := internalmcp.NewAdapter(q, db, nil, nil, nil, config.EmbeddingConfig{}, config.SearchConfig{}, config.FlowConfig{}, nil, zerolog.Nop())
	internalmcp.RegisterTools(server, adapter)
	ct, st := mcpsdk.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "c", Version: "v0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })

	return &mcp584Env{
		ctx: ctx, q: q, db: db, wsHash: wsHash, root: root,
		cfg: config.Config{
			Watcher: config.WatcherConfig{DebounceMs: 10, ReindexInterval: 3600},
			Storage: config.StorageConfig{MaxFileSize: 1 << 20},
		},
		reg: symbolRegistry584(t),
		call: func(name string, args map[string]any) *mcpsdk.CallToolResult {
			t.Helper()
			r, err := session.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool(%s): %v", name, err)
			}
			return r
		},
	}
}

// index runs a production-wired watcher over root until the kind=function
// symbol doc for sym in relFile carries end_line, then stops it. The wait
// targets the exact doc — a `?symbol=foo` prefix can complete early while a
// same-name sibling doc is still being upserted.
func (e *mcp584Env) index(t *testing.T, relFile, sym string) {
	t.Helper()
	w := watcher.New(e.db, e.q, zerolog.Nop(), e.cfg).WithSymbolRegistry(e.reg)
	if err := w.Watch("code", e.root, e.wsHash, "**/*"); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(e.ctx)
	done := make(chan error, 1)
	go func() { done <- w.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("watcher: %v", err)
		}
	})
	e.waitSpan(t, filepath.Join(e.root, relFile)+"?symbol="+sym+"&kind=function")
}

func (e *mcp584Env) symbolMeta(t *testing.T, sourcePath string) map[string]string {
	t.Helper()
	doc, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: e.wsHash,
	})
	if err != nil {
		return nil
	}
	var meta map[string]string
	if doc.Metadata.Valid {
		_ = json.Unmarshal(doc.Metadata.RawMessage, &meta)
	}
	return meta
}

func (e *mcp584Env) waitSpan(t *testing.T, symbolSourcePathPrefix string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		rows, err := e.db.QueryContext(e.ctx,
			`SELECT metadata FROM documents WHERE workspace_hash = $1 AND starts_with(source_path, $2)`,
			e.wsHash, symbolSourcePathPrefix)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		complete := true
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var meta map[string]string
			_ = json.Unmarshal(raw, &meta)
			count++
			if meta["end_line"] == "" {
				complete = false
			}
		}
		rows.Close()
		if count > 0 && complete {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s* to carry end_line", symbolSourcePathPrefix)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// getContent calls memory_get and returns the JSON result's content string.
func (e *mcp584Env) getContent(t *testing.T, path string) string {
	t.Helper()
	res := e.call("memory_get", map[string]any{"workspace": e.wsHash, "path": path})
	text := res.Content[0].(*mcpsdk.TextContent).Text
	if res.IsError {
		t.Fatalf("memory_get(%s) error: %s", path, text)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("memory_get result: %v", err)
	}
	content, _ := out["content"].(string)
	return content
}

func assertFooBody584(t *testing.T, content string, wantLines int) {
	t.Helper()
	lines := strings.Split(content, "\n")
	if len(lines) != wantLines {
		t.Fatalf("content has %d lines, want %d; first=%q", len(lines), wantLines, lines[0])
	}
	if !strings.HasPrefix(lines[0], "export const foo") {
		t.Fatalf("first line = %q, want prefix %q", lines[0], "export const foo")
	}
	if last := lines[len(lines)-1]; last != "};" {
		t.Fatalf("last line = %q, want %q", last, "};")
	}
}

// TestMemoryGetSymbolPrefersFunctionOverSameNameMethod covers issue #584's
// ordering half: when an object-property method shares its name with a
// top-level const function in the same file, `file::foo` must resolve to the
// function, not whichever row Postgres happens to return first.
func TestMemoryGetSymbolPrefersFunctionOverSameNameMethod(t *testing.T) {
	src, declLine, endLine := handlersFile584(true)
	e := setup584(t, map[string]string{"src/handlers.ts": src})
	e.index(t, "src/handlers.ts", "foo")

	wantLines := endLine - declLine + 1
	for i := 0; i < 5; i++ {
		assertFooBody584(t, e.getContent(t, "src/handlers.ts::foo"), wantLines)
	}
}

// TestMemoryGetStaleSymbolBackfilledAfterRestart covers issue #584's stale
// half end to end: strip the span metadata like a pre-#539 index, restart the
// watcher on the unchanged tree, then memory_get must return the full body.
func TestMemoryGetStaleSymbolBackfilledAfterRestart(t *testing.T) {
	src, declLine, endLine := handlersFile584(false)
	e := setup584(t, map[string]string{"src/handlers.ts": src})
	e.index(t, "src/handlers.ts", "foo")

	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE documents SET metadata = metadata - 'line' - 'end_line'
		 WHERE workspace_hash = $1 AND starts_with(source_path, $2)`,
		e.wsHash, filepath.Join(e.root, "src/handlers.ts")+"?symbol="); err != nil {
		t.Fatal(err)
	}
	if meta := e.symbolMeta(t, filepath.Join(e.root, "src/handlers.ts")+"?symbol=foo&kind=function"); meta["end_line"] != "" {
		t.Fatalf("stale-span setup failed: meta=%v", meta)
	}

	// A second watcher over the unchanged tree: the content-hash fast path
	// skips re-indexing, so only the startup backfill can restore the spans.
	e.index(t, "src/handlers.ts", "foo")

	wantLines := endLine - declLine + 1
	assertFooBody584(t, e.getContent(t, "src/handlers.ts::foo"), wantLines)
}
