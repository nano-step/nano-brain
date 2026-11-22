//go:build integration

package watcher

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
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
	"github.com/sqlc-dev/pqtype"
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

type env584 struct {
	ctx    context.Context
	q      *sqlc.Queries
	db     *sql.DB
	ws     string
	root   string
	cfg    config.Config
	reg    *symbol.Registry
	cancel context.CancelFunc
	done   chan error
}

func newEnv584(t *testing.T) *env584 {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)

	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ws := fmt.Sprintf("%x", sha256.Sum256([]byte("ws584_"+uuid.NewString())))
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{Hash: ws, Name: "ws584", Path: root}); err != nil {
		t.Fatal(err)
	}
	return &env584{
		ctx:  context.Background(),
		q:    q,
		db:   db,
		ws:   ws,
		root: root,
		cfg: config.Config{
			Watcher: config.WatcherConfig{DebounceMs: 10, ReindexInterval: 3600},
			Storage: config.StorageConfig{MaxFileSize: 1 << 20},
		},
		reg: symbolRegistry584(t),
	}
}

func (e *env584) writeFile(t *testing.T, rel, content string) string {
	t.Helper()
	p := filepath.Join(e.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *env584) start(t *testing.T) {
	t.Helper()
	w := New(e.db, e.q, zerolog.Nop(), e.cfg).WithSymbolRegistry(e.reg)
	if err := w.Watch("code", e.root, e.ws, "**/*"); err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(e.ctx)
	e.done = make(chan error, 1)
	go func() { e.done <- w.Run(runCtx) }()
	e.cancel = cancel
}

func (e *env584) stop(t *testing.T) {
	t.Helper()
	e.cancel()
	if err := <-e.done; err != nil {
		t.Fatalf("watcher: %v", err)
	}
	e.cancel = nil
}

func (e *env584) doc(t *testing.T, sourcePath string) sqlc.DocumentView {
	t.Helper()
	doc, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: e.ws,
	})
	if err != nil {
		t.Fatalf("doc %s: %v", sourcePath, err)
	}
	return doc
}

func (e *env584) symbolMeta(t *testing.T, sourcePath string) map[string]string {
	t.Helper()
	doc := e.doc(t, sourcePath)
	var meta map[string]string
	if doc.Metadata.Valid {
		if err := json.Unmarshal(doc.Metadata.RawMessage, &meta); err != nil {
			t.Fatal(err)
		}
	}
	return meta
}

// waitSpan polls until every doc under the symbol source_path prefix carries
// end_line. Deadlines beat sleeps: the watcher writes asynchronously.
func (e *env584) waitSpan(t *testing.T, symbolSourcePath string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		doc, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    symbolSourcePath,
			WorkspaceHash: e.ws,
		})
		if err == nil {
			var meta map[string]string
			if doc.Metadata.Valid {
				_ = json.Unmarshal(doc.Metadata.RawMessage, &meta)
			}
			if meta["end_line"] != "" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s to carry end_line (err=%v)", symbolSourcePath, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitDoc polls until a document exists at sourcePath.
func (e *env584) waitDoc(t *testing.T, sourcePath string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sourcePath,
			WorkspaceHash: e.ws,
		})
		if err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for doc %s (err=%v)", sourcePath, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitDocGone polls until no document exists at sourcePath. The startup
// backfill deletes orphan stale docs synchronously after the initial scan,
// so completion of this wait means the backfill has run for that file.
func (e *env584) waitDocGone(t *testing.T, sourcePath string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sourcePath,
			WorkspaceHash: e.ws,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return
			}
			t.Fatalf("doc %s: %v", sourcePath, err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for doc %s to be deleted", sourcePath)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// insertStaleSymbolDoc writes a pre-#539 style symbol doc: signature content
// only, metadata without line/end_line.
func (e *env584) insertStaleSymbolDoc(t *testing.T, filePath, name, kind string) {
	t.Helper()
	meta, err := json.Marshal(map[string]string{
		"source_type": "symbol",
		"kind":        kind,
		"language":    "typescript",
		"signature":   "export function " + name + "() {}",
	})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filePath + "?symbol=" + name + "&kind=" + kind
	sum := sha256.Sum256([]byte(sourcePath + name))
	if _, err := e.q.UpsertDocumentBySourcePath(e.ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: e.ws,
		ContentHash:   fmt.Sprintf("%x", sum[:]),
		Title:         name,
		Content:       "export function " + name + "() {}",
		SourcePath:    sourcePath,
		Collection:    "code",
		Tags:          []string{"symbol", "typescript", kind},
		Metadata:      pqtype.NullRawMessage{RawMessage: meta, Valid: true},
	}); err != nil {
		t.Fatal(err)
	}
}

// stripSpans removes line/end_line from every symbol doc under a file,
// simulating a pre-#539 index. documents is a table (v30+), so UPDATE
// applies directly; starts_with avoids LIKE wildcards.
func (e *env584) stripSpans(t *testing.T, filePath string) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE documents SET metadata = metadata - 'line' - 'end_line'
		 WHERE workspace_hash = $1 AND starts_with(source_path, $2)`,
		e.ws, filePath+"?symbol="); err != nil {
		t.Fatal(err)
	}
}

func (e *env584) symbolDocCount(t *testing.T, filePath string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT count(*) FROM documents
		 WHERE workspace_hash = $1 AND starts_with(source_path, $2)
		   AND metadata->>'source_type' = 'symbol'`,
		e.ws, filePath+"?symbol=").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWatcherBackfillsStaleSymbolSpans(t *testing.T) {
	e := newEnv584(t)

	lines := []string{"import { db } from './db';", ""}
	declLine := len(lines) + 1
	lines = append(lines, "export const foo = async (a: string, b: number, c: boolean): Promise<void> => {")
	for i := 0; i < 160; i++ {
		lines = append(lines, fmt.Sprintf("  const v%d = await db.query(%d);", i, i))
	}
	lines = append(lines, "};")
	endLine := len(lines)

	handlersPath := e.writeFile(t, "src/handlers.ts", strings.Join(lines, "\n")+"\n")
	otherPath := e.writeFile(t, "src/other.ts", "export function bar(x: number): number {\n  return x * 2;\n}\n")

	fooPath := handlersPath + "?symbol=foo&kind=function"
	barPath := otherPath + "?symbol=bar&kind=function"

	e.start(t)
	e.waitSpan(t, fooPath)
	e.waitSpan(t, barPath)
	barBefore := e.doc(t, barPath)
	e.stop(t)

	e.stripSpans(t, handlersPath)
	if meta := e.symbolMeta(t, fooPath); meta["end_line"] != "" {
		t.Fatalf("stale-span setup failed: meta=%v", meta)
	}
	// An orphan stale doc for a symbol that no longer exists: the backfill
	// must delete it rather than re-extract it on every restart.
	gonePath := handlersPath + "?symbol=gone&kind=function"
	e.insertStaleSymbolDoc(t, handlersPath, "gone", "function")

	// A fresh watcher on the unchanged tree must backfill the missing spans:
	// the content-hash fast path skips re-indexing, so only the startup
	// backfill can repair them.
	e.start(t)
	e.waitSpan(t, fooPath)
	e.waitDocGone(t, gonePath)
	e.stop(t)

	meta := e.symbolMeta(t, fooPath)
	if got := meta["line"]; got != strconv.Itoa(declLine) {
		t.Fatalf("line = %q, want %d", got, declLine)
	}
	if got := meta["end_line"]; got != strconv.Itoa(endLine) {
		t.Fatalf("end_line = %q, want %d", got, endLine)
	}
	fooAfter := e.doc(t, fooPath)

	// The untouched second file must not be re-extracted: a blanket
	// re-extract would bump its symbol document's updated_at.
	barAfter := e.doc(t, barPath)
	if !barAfter.UpdatedAt.Equal(barBefore.UpdatedAt) {
		t.Fatalf("untouched file re-extracted: updated_at %v -> %v", barBefore.UpdatedAt, barAfter.UpdatedAt)
	}

	// Converged state: a third start must not touch the converged file. A
	// stale canary on the OTHER file gives a deterministic signal — its
	// deletion means the backfill finished its only row (the query orders
	// by file_path), while handlers.ts had no stale docs at all.
	canaryPath := otherPath + "?symbol=gone2&kind=function"
	e.insertStaleSymbolDoc(t, otherPath, "gone2", "function")
	e.start(t)
	defer e.stop(t)
	e.waitDocGone(t, canaryPath)

	if got := e.doc(t, fooPath); !got.UpdatedAt.Equal(fooAfter.UpdatedAt) {
		t.Fatalf("converged file re-extracted on third start: updated_at %v -> %v", fooAfter.UpdatedAt, got.UpdatedAt)
	}
}

// TestWatcherBackfillDropsStaleSymbolsForIgnoredFile covers the admission
// bypass from the #584 review: a nested .gitignore that newly ignores a file
// deletes its file doc during the initial scan but leaves its stale symbol
// docs behind. The backfill must delete those docs without reading the file.
func TestWatcherBackfillDropsStaleSymbolsForIgnoredFile(t *testing.T) {
	e := newEnv584(t)
	xPath := e.writeFile(t, "src/secret/x.ts", "export function oldName() {\n  return 1;\n}\n")
	oldPath := xPath + "?symbol=oldName&kind=function"

	e.start(t)
	e.waitSpan(t, oldPath)
	e.stop(t)

	e.stripSpans(t, xPath)
	e.writeFile(t, "src/secret/.gitignore", "x.ts\n")
	e.writeFile(t, "src/secret/x.ts", "export function leakedSecretName() {\n  return 2;\n}\n")

	e.start(t)
	defer e.stop(t)
	e.waitDocGone(t, oldPath)

	if n := e.symbolDocCount(t, xPath); n != 0 {
		t.Fatalf("ignored file still parents %d symbol docs (want 0): leakedSecretName must not be indexed", n)
	}
}

// TestWatcherBackfillKeepsStaleSymbolsForMissingIndexedFile covers the F1
// loss: the file doc exists but the file itself is missing at startup
// (moved out and back, a branch switch, an empty mounted root). The stale
// symbol docs must be kept — once the file returns unchanged the hash fast
// path never re-extracts it, so deleting them would lose them permanently.
func TestWatcherBackfillKeepsStaleSymbolsForMissingIndexedFile(t *testing.T) {
	e := newEnv584(t)
	src := "export function removed() {\n  return 1;\n}\n"
	gonePath := e.writeFile(t, "src/gone.ts", src)
	otherPath := e.writeFile(t, "src/other.ts", "export function bar(x: number): number {\n  return x * 2;\n}\n")
	symPath := gonePath + "?symbol=removed&kind=function"

	e.start(t)
	e.waitSpan(t, symPath)
	e.stop(t)

	e.stripSpans(t, gonePath)
	if err := os.Remove(gonePath); err != nil {
		t.Fatal(err)
	}
	// ListFilesWithStaleSymbolSpans orders by file_path, and src/other.ts
	// sorts after src/gone.ts, so the missing row was already visited.
	canaryPath := otherPath + "?symbol=gone&kind=function"
	e.insertStaleSymbolDoc(t, otherPath, "gone", "function")

	e.start(t)
	e.waitDocGone(t, canaryPath)
	e.stop(t)

	if _, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    symPath,
		WorkspaceHash: e.ws,
	}); err != nil {
		t.Fatalf("stale doc for missing-but-indexed file must be kept, got err=%v", err)
	}

	// Restore the file unchanged: the content hash keeps it off every later
	// scan, so only the backfill can repair the spans.
	if err := os.WriteFile(gonePath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	e.start(t)
	defer e.stop(t)
	e.waitSpan(t, symPath)
}

// TestWatcherBackfillKeepsStaleSymbolsWhenOversized covers the max_file_size
// case: a file over the current limit keeps its stale docs, so lowering the
// limit cannot permanently lose symbols that a later, higher limit repairs.
func TestWatcherBackfillKeepsStaleSymbolsWhenOversized(t *testing.T) {
	e := newEnv584(t)
	var b strings.Builder
	b.WriteString("export function bigSym() {\n  return 1;\n}\n")
	for i := 0; i < 80; i++ {
		b.WriteString("// padding padding padding padding padding\n")
	}
	bigPath := e.writeFile(t, "src/big.ts", b.String())
	goPath := e.writeFile(t, "src/zz.go", "package zz\n")
	symPath := bigPath + "?symbol=bigSym&kind=function"

	e.start(t)
	e.waitSpan(t, symPath)
	e.stop(t)

	e.stripSpans(t, bigPath)
	canaryPath := goPath + "?symbol=gone&kind=function"
	e.insertStaleSymbolDoc(t, goPath, "gone", "function")

	// Second start under a small limit: big.ts is oversized and kept; the
	// tiny zz.go canary still deletes, proving the row order was visited —
	// src/big.ts sorts before src/zz.go.
	e.cfg.Storage.MaxFileSize = 128
	e.start(t)
	e.waitDocGone(t, canaryPath)
	e.stop(t)

	if _, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    symPath,
		WorkspaceHash: e.ws,
	}); err != nil {
		t.Fatalf("stale doc for oversized file must be kept, got err=%v", err)
	}

	e.cfg.Storage.MaxFileSize = 1 << 20
	e.start(t)
	defer e.stop(t)
	e.waitSpan(t, symPath)
}

// TestWatcherBackfillKeepsStaleSymbolsWhenBinary covers the F3 bypass: the
// backfill must apply the same binary-content gate as processFile, so a
// file that became binary is never read for symbols and its stale docs are
// kept.
func TestWatcherBackfillKeepsStaleSymbolsWhenBinary(t *testing.T) {
	e := newEnv584(t)
	orig := "export function oldSym() {\n  return 1;\n}\n"
	tsPath := e.writeFile(t, "src/turned.ts", orig)
	otherPath := e.writeFile(t, "src/zother.ts", "export function bar(x: number): number {\n  return x * 2;\n}\n")
	symPath := tsPath + "?symbol=oldSym&kind=function"
	sneakyPath := tsPath + "?symbol=sneakyNew&kind=function"

	e.start(t)
	e.waitSpan(t, symPath)
	e.stop(t)

	e.stripSpans(t, tsPath)
	if err := os.WriteFile(tsPath, []byte(orig+"export function sneakyNew() {\n  return 2;\n}\n\x00\x00"), 0o644); err != nil {
		t.Fatal(err)
	}
	// src/zother.ts sorts after src/turned.ts, so its canary deletion means
	// the binary row was already visited.
	canaryPath := otherPath + "?symbol=gone&kind=function"
	e.insertStaleSymbolDoc(t, otherPath, "gone", "function")

	e.start(t)
	e.waitDocGone(t, canaryPath)
	e.stop(t)

	if _, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    symPath,
		WorkspaceHash: e.ws,
	}); err != nil {
		t.Fatalf("stale doc for binary file must be kept, got err=%v", err)
	}
	if _, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sneakyPath,
		WorkspaceHash: e.ws,
	}); err == nil {
		t.Fatal("binary file must not be read for symbols: sneakyNew was indexed")
	} else if !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
}

// TestWatcherBackfillDropsStaleSymbolsWhenExtractorYieldsNone covers the
// #703 shape: the current extractor emits no symbols for a file that once
// had them, so re-extraction upserts nothing and the stale doc must be
// deleted for the backfill to converge.
func TestWatcherBackfillDropsStaleSymbolsWhenExtractorYieldsNone(t *testing.T) {
	e := newEnv584(t)
	narrowPath := e.writeFile(t, "src/narrow.ts", "const f = (p: P) => 1;\n")

	// The file yields no symbols today, so index just the file doc.
	e.start(t)
	e.waitDoc(t, narrowPath)
	e.stop(t)

	symPath := narrowPath + "?symbol=f&kind=function"
	e.insertStaleSymbolDoc(t, narrowPath, "f", "function")

	e.start(t)
	defer e.stop(t)
	e.waitDocGone(t, symPath)
}

// TestWatcherBackfillKeepsStaleSymbolsWhenFileUnreadable covers the
// transient-failure half of the rework: a file that cannot be read (EACCES
// here) is not provably orphaned, so its stale symbol docs must be kept for
// the next start rather than deleted permanently.
func TestWatcherBackfillKeepsStaleSymbolsWhenFileUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read 000-permission files")
	}
	e := newEnv584(t)
	lockedPath := e.writeFile(t, "src/locked.ts", "export function keepMe() {\n  return 1;\n}\n")
	otherPath := e.writeFile(t, "src/other.ts", "export function bar(x: number): number {\n  return x * 2;\n}\n")
	keepPath := lockedPath + "?symbol=keepMe&kind=function"

	e.start(t)
	e.waitSpan(t, keepPath)
	e.stop(t)

	e.stripSpans(t, lockedPath)
	if err := os.Chmod(lockedPath, 0o000); err != nil {
		t.Fatal(err)
	}
	// Canary on a readable file: its deletion proves the backfill ran.
	// ListFilesWithStaleSymbolSpans orders by file_path, and src/other.ts
	// sorts after src/locked.ts, so the locked row was already visited.
	canaryPath := otherPath + "?symbol=gone&kind=function"
	e.insertStaleSymbolDoc(t, otherPath, "gone", "function")

	e.start(t)
	e.waitDocGone(t, canaryPath)
	e.stop(t)

	if _, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    keepPath,
		WorkspaceHash: e.ws,
	}); err != nil {
		t.Fatalf("stale doc for unreadable file must be kept, got err=%v", err)
	}

	// Once the file is readable again the next start repairs the spans.
	if err := os.Chmod(lockedPath, 0o644); err != nil {
		t.Fatal(err)
	}
	e.start(t)
	defer e.stop(t)
	e.waitSpan(t, keepPath)
}

// TestWatcherBackfillKeepsStaleSymbolsWhenExtractorMissing covers the case
// where no registered extractor supports the file's extension: the stale
// docs may belong to a symbol kind a future or differently-configured
// binary emits, so they are kept rather than deleted.
func TestWatcherBackfillKeepsStaleSymbolsWhenExtractorMissing(t *testing.T) {
	e := newEnv584(t)
	tsPath := e.writeFile(t, "src/handler.ts", "export function tsOnly() {\n  return 1;\n}\n")
	goPath := e.writeFile(t, "src/main.go", "package main\n\nfunc goSym() {}\n")
	tsSymPath := tsPath + "?symbol=tsOnly&kind=function"
	goSymPath := goPath + "?symbol=goSym&kind=function"

	e.start(t)
	e.waitSpan(t, tsSymPath)
	e.waitSpan(t, goSymPath)
	e.stop(t)

	e.stripSpans(t, tsPath)
	canaryPath := goPath + "?symbol=gone&kind=function"
	e.insertStaleSymbolDoc(t, goPath, "gone", "function")

	// Second start with a registry that lacks the TypeScript extractor: the
	// .go canary still proves the backfill ran while the .ts docs are kept.
	goE, err := symbol.NewGoExtractor()
	if err != nil {
		t.Fatal(err)
	}
	e.reg = symbol.NewRegistry(goE)
	e.start(t)
	defer e.stop(t)
	// ListFilesWithStaleSymbolSpans orders by file_path, and src/main.go
	// sorts after src/handler.ts, so the .ts row was already visited.
	e.waitDocGone(t, canaryPath)

	if _, err := e.q.GetDocumentBySourcePath(e.ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    tsSymPath,
		WorkspaceHash: e.ws,
	}); err != nil {
		t.Fatalf("stale doc for unsupported extension must be kept, got err=%v", err)
	}
}
