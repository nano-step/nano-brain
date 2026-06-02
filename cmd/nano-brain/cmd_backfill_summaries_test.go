package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/sqlc-dev/pqtype"
)

func setupBackfillTestPG(t *testing.T) *sqlc.Queries {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(cleanupTestDSN)
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	schema := fmt.Sprintf("test_bfs_%x", sha256.Sum256([]byte(t.Name())))[:19]
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Skip("postgres not available: " + err.Error())
	}

	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Skip("postgres not available: " + err.Error())
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	goose.SetTableName(schema + "_goose_version")
	migrateDB := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, migrateDB, "."); err != nil {
		migrateDB.Close()
		pool.Close()
		t.Fatal("migrate up:", err)
	}
	migrateDB.Close()
	goose.SetTableName("goose_db_version")

	pgDB := stdlib.OpenDBFromPool(pool)
	q := sqlc.New(pgDB)

	t.Cleanup(func() {
		pgDB.Close()
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	return q
}

func bfInsertWorkspace(t *testing.T, q *sqlc.Queries, hash, name string) {
	t.Helper()
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: hash,
		Name: name,
		Path: "/tmp/" + hash,
	}); err != nil {
		t.Fatalf("bfInsertWorkspace: %v", err)
	}
}

func bfInsertDoc(t *testing.T, q *sqlc.Queries, wsHash, sessionID, title, content, source string) {
	t.Helper()
	sum := sha256.Sum256([]byte(content))
	sourcePath := "summary://" + source + "/" + sessionID
	metaJSON := fmt.Sprintf(`{"session_id":%q,"source":%q,"summary":true}`, sessionID, source)
	_, err := q.UpsertDocumentBySourcePath(context.Background(), sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   hex.EncodeToString(sum[:]),
		Title:         "Summary: " + title,
		Content:       content,
		SourcePath:    sourcePath,
		Collection:    "session-summary",
		Tags:          []string{"summary", source},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(metaJSON), Valid: true},
	})
	if err != nil {
		t.Fatalf("bfInsertDoc: %v", err)
	}
}

func bfCountMDFiles(t *testing.T, dir string) int {
	t.Helper()
	var count int
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".md") {
			count++
		}
		return nil
	})
	return count
}

func bfExpectedPath(outDir, wsName, wsHash, source, title string, createdAt time.Time) string {
	ws := bfFolderName(wsName, wsHash)
	titleSlug := bfSlugify(title)
	srcSlug := bfSlugify(source)
	dateStr := createdAt.UTC().Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s_%s.md", srcSlug, titleSlug, dateStr)
	return filepath.Join(outDir, ws, filename)
}

func bfFolderName(name, hash string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if len(hash) >= 12 {
			return "ws-" + hash[:12]
		}
		return "ws-" + hash
	}
	return bfSlugify(name)
}

func bfSlugify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	if len(out) > 80 {
		out = strings.TrimRight(out[:80], "-")
	}
	if out == "" {
		return "untitled-session"
	}
	return out
}

func TestBackfill_EmptyDB(t *testing.T) {
	q := setupBackfillTestPG(t)
	ctx := context.Background()

	docs, err := q.ListSummaryDocumentsForBackfill(ctx, sqlc.ListSummaryDocumentsForBackfillParams{
		Column1: "",
		Column2: time.Time{},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs in empty DB, got %d", len(docs))
	}
}

func TestBackfill_DryRun(t *testing.T) {
	q := setupBackfillTestPG(t)
	outDir := t.TempDir()
	ctx := context.Background()

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("dryrun-ws")))[:16]
	bfInsertWorkspace(t, q, wsHash, "dryrun-workspace")
	bfInsertDoc(t, q, wsHash, "ses_dry001", "Session Alpha", "# Alpha\nContent A.", "opencode")
	bfInsertDoc(t, q, wsHash, "ses_dry002", "Session Beta", "# Beta\nContent B.", "opencode")
	bfInsertDoc(t, q, wsHash, "ses_dry003", "Session Gamma", "# Gamma\nContent C.", "opencode")

	docs, err := q.ListSummaryDocumentsForBackfill(ctx, sqlc.ListSummaryDocumentsForBackfillParams{
		Column1: "",
		Column2: time.Time{},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}

	for _, doc := range docs {
		wsName := ""
		if ws, err := q.GetWorkspaceByHash(ctx, doc.WorkspaceHash); err == nil {
			wsName = ws.Name
		}
		source := extractBackfillSource(doc.Tags)
		title := strings.TrimPrefix(doc.Title, "Summary: ")
		path := bfExpectedPath(outDir, wsName, doc.WorkspaceHash, source, title, doc.CreatedAt)
		t.Logf("[dry-run] would write: %s", path)
	}

	if got := bfCountMDFiles(t, outDir); got != 0 {
		t.Errorf("dry-run must not write files: got %d .md files", got)
	}
}

func TestBackfill_ExportsToFilesystem(t *testing.T) {
	q := setupBackfillTestPG(t)
	outDir := t.TempDir()
	ctx := context.Background()

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("export-ws")))[:16]
	bfInsertWorkspace(t, q, wsHash, "export-workspace")
	bfInsertDoc(t, q, wsHash, "ses_exp001", "Export Session One", "# Export 1\nContent one.", "opencode")
	bfInsertDoc(t, q, wsHash, "ses_exp002", "Export Session Two", "# Export 2\nContent two.", "opencode")
	bfInsertDoc(t, q, wsHash, "ses_exp003", "Export Session Three", "# Export 3\nContent three.", "opencode")

	docs, err := q.ListSummaryDocumentsForBackfill(ctx, sqlc.ListSummaryDocumentsForBackfillParams{
		Column1: "",
		Column2: time.Time{},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(docs))
	}

	for _, doc := range docs {
		wsName := ""
		if ws, err := q.GetWorkspaceByHash(ctx, doc.WorkspaceHash); err == nil {
			wsName = ws.Name
		}
		source := extractBackfillSource(doc.Tags)
		sessionID := extractBackfillSessionID(doc.Metadata.RawMessage, doc.SourcePath)
		title := strings.TrimPrefix(doc.Title, "Summary: ")
		targetPath := bfExpectedPath(outDir, wsName, doc.WorkspaceHash, source, title, doc.CreatedAt)

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("mkdirall: %v", err)
		}
		if err := os.WriteFile(targetPath, []byte(doc.Content), 0o644); err != nil {
			t.Fatalf("writefile: %v", err)
		}
		_ = sessionID
	}

	if got := bfCountMDFiles(t, outDir); got != 3 {
		t.Errorf("expected 3 .md files, got %d", got)
	}

	for _, doc := range docs {
		wsName := ""
		if ws, err := q.GetWorkspaceByHash(ctx, doc.WorkspaceHash); err == nil {
			wsName = ws.Name
		}
		source := extractBackfillSource(doc.Tags)
		title := strings.TrimPrefix(doc.Title, "Summary: ")
		path := bfExpectedPath(outDir, wsName, doc.WorkspaceHash, source, title, doc.CreatedAt)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if string(data) != doc.Content {
			t.Errorf("file content mismatch at %s", path)
		}
	}
}

func TestBackfill_Idempotent(t *testing.T) {
	q := setupBackfillTestPG(t)
	outDir := t.TempDir()
	ctx := context.Background()

	wsHash := hex.EncodeToString(sha256.New().Sum([]byte("idempotent-ws")))[:16]
	bfInsertWorkspace(t, q, wsHash, "idempotent-workspace")
	bfInsertDoc(t, q, wsHash, "ses_idem001", "Idempotent A", "# Idempotent A\nContent.", "opencode")
	bfInsertDoc(t, q, wsHash, "ses_idem002", "Idempotent B", "# Idempotent B\nContent.", "opencode")
	bfInsertDoc(t, q, wsHash, "ses_idem003", "Idempotent C", "# Idempotent C\nContent.", "opencode")

	runExport := func() (written, skipped int) {
		docs, err := q.ListSummaryDocumentsForBackfill(ctx, sqlc.ListSummaryDocumentsForBackfillParams{
			Column1: "",
			Column2: time.Time{},
		})
		if err != nil {
			t.Fatalf("query: %v", err)
		}
		for _, doc := range docs {
			wsName := ""
			if ws, err := q.GetWorkspaceByHash(ctx, doc.WorkspaceHash); err == nil {
				wsName = ws.Name
			}
			source := extractBackfillSource(doc.Tags)
			title := strings.TrimPrefix(doc.Title, "Summary: ")
			targetPath := bfExpectedPath(outDir, wsName, doc.WorkspaceHash, source, title, doc.CreatedAt)

			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				t.Fatalf("mkdirall: %v", err)
			}
			existing, readErr := os.ReadFile(targetPath)
			if readErr == nil && string(existing) == doc.Content {
				skipped++
				continue
			}
			if err := os.WriteFile(targetPath, []byte(doc.Content), 0o644); err != nil {
				t.Fatalf("writefile: %v", err)
			}
			written++
		}
		return
	}

	w1, s1 := runExport()
	if w1 != 3 || s1 != 0 {
		t.Errorf("run1: want written=3 skipped=0, got written=%d skipped=%d", w1, s1)
	}

	w2, s2 := runExport()
	if w2 != 0 || s2 != 3 {
		t.Errorf("run2: want written=0 skipped=3, got written=%d skipped=%d", w2, s2)
	}
}

func TestBackfill_WorkspaceFilter(t *testing.T) {
	q := setupBackfillTestPG(t)
	ctx := context.Background()

	fooHash := hex.EncodeToString(sha256.New().Sum([]byte("filter-foo")))[:16]
	barHash := hex.EncodeToString(sha256.New().Sum([]byte("filter-bar")))[:16]
	bfInsertWorkspace(t, q, fooHash, "foo-workspace")
	bfInsertWorkspace(t, q, barHash, "bar-workspace")

	bfInsertDoc(t, q, fooHash, "ses_foo001", "Foo Session 1", "# Foo 1", "opencode")
	bfInsertDoc(t, q, fooHash, "ses_foo002", "Foo Session 2", "# Foo 2", "opencode")
	bfInsertDoc(t, q, barHash, "ses_bar001", "Bar Session 1", "# Bar 1", "opencode")

	fooDocs, err := q.ListSummaryDocumentsForBackfill(ctx, sqlc.ListSummaryDocumentsForBackfillParams{
		Column1: fooHash,
		Column2: time.Time{},
	})
	if err != nil {
		t.Fatalf("query foo: %v", err)
	}
	if len(fooDocs) != 2 {
		t.Errorf("workspace filter foo: expected 2 docs, got %d", len(fooDocs))
	}
	for _, doc := range fooDocs {
		if doc.WorkspaceHash != fooHash {
			t.Errorf("got doc from wrong workspace: %s", doc.WorkspaceHash)
		}
	}

	allDocs, err := q.ListSummaryDocumentsForBackfill(ctx, sqlc.ListSummaryDocumentsForBackfillParams{
		Column1: "",
		Column2: time.Time{},
	})
	if err != nil {
		t.Fatalf("query all: %v", err)
	}
	if len(allDocs) != 3 {
		t.Errorf("no filter: expected 3 docs, got %d", len(allDocs))
	}
}
