//go:build integration

package harvest_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

func createOnDiskSQLite(t *testing.T, dir, worktree string) string {
	t.Helper()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("create sqlite %s: %v", dbPath, err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL);
		CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, title TEXT, time_created INTEGER, time_updated INTEGER);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT);
	`)
	if err != nil {
		t.Fatalf("create tables %s: %v", dbPath, err)
	}
	if worktree != "" {
		_, err = db.Exec(`INSERT INTO project (id, worktree) VALUES (?, ?)`, "proj-1", worktree)
		if err != nil {
			t.Fatalf("insert project %s: %v", dbPath, err)
		}
	}
	oldMs := time.Now().Add(-15 * time.Minute).UnixMilli()
	_, err = db.Exec(`INSERT INTO session (id, project_id, title, time_created, time_updated) VALUES (?, ?, ?, ?, ?)`,
		"sess-1", "proj-1", "Test Session", oldMs, oldMs)
	if err != nil {
		t.Fatalf("insert session %s: %v", dbPath, err)
	}
	_, err = db.Exec(`INSERT INTO message (id, session_id, time_created, data) VALUES (?, ?, ?, ?)`,
		"msg-1", "sess-1", oldMs, `{"role":"user"}`)
	if err != nil {
		t.Fatalf("insert message %s: %v", dbPath, err)
	}
	_, err = db.Exec(`INSERT INTO part (id, message_id, session_id, time_created, data) VALUES (?, ?, '', 0, ?)`,
		"part-1", "msg-1", `{"type":"text","text":"hello from `+worktree+`"}`)
	if err != nil {
		t.Fatalf("insert part %s: %v", dbPath, err)
	}
	return dbPath
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

type testEnqueuer struct{ ids []uuid.UUID }

func (e *testEnqueuer) Enqueue(id uuid.UUID) bool {
	e.ids = append(e.ids, id)
	return true
}

func assertDocs(t *testing.T, q *sqlc.Queries, ctx context.Context, wsHash, label string) {
	t.Helper()
	docs, err := q.ListDocumentsByWorkspace(ctx, wsHash)
	if err != nil {
		t.Fatalf("%s: list docs: %v", label, err)
	}
	if len(docs) == 0 {
		t.Errorf("%s: expected >= 1 document in PG, got 0", label)
		return
	}
	for _, d := range docs {
		if strings.HasPrefix(d.SourcePath, "summary://opencode/") {
			return
		}
	}
	t.Errorf("%s: no document with source_path prefix 'summary://opencode/' found; paths: %v",
		label, sourcePaths(docs))
}

func sourcePaths(docs []sqlc.ListDocumentsByWorkspaceRow) []string {
	out := make([]string, len(docs))
	for i, d := range docs {
		out[i] = d.SourcePath
	}
	return out
}

func TestOpenCodeMultiDB_Integration(t *testing.T) {
	pgDB := setupIntegrationPG(t)

	ctx := context.Background()
	logger := zerolog.Nop()

	randSuffix := fmt.Sprintf("%08x", rand.Uint32())
	pathA := fmt.Sprintf("/tmp/proj-a-%s", randSuffix)
	pathB := fmt.Sprintf("/tmp/proj-b-%s", randSuffix)
	pathUnrelated := fmt.Sprintf("/tmp/unrelated-%s", randSuffix)

	hashA, err := storage.WorkspaceHash(pathA)
	if err != nil {
		t.Fatalf("hash proj-a: %v", err)
	}
	hashB, err := storage.WorkspaceHash(pathB)
	if err != nil {
		t.Fatalf("hash proj-b: %v", err)
	}

	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: hashA,
		Name: "proj-a-" + randSuffix,
		Path: pathA,
	}); err != nil {
		t.Fatalf("upsert workspace A: %v", err)
	}
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: hashB,
		Name: "proj-b-" + randSuffix,
		Path: pathB,
	}); err != nil {
		t.Fatalf("upsert workspace B: %v", err)
	}

	dbRoot := t.TempDir()
	dirA := filepath.Join(dbRoot, fmt.Sprintf("proj-a-%s", randSuffix))
	dirB := filepath.Join(dbRoot, fmt.Sprintf("proj-b-%s", randSuffix))
	dirC := filepath.Join(dbRoot, fmt.Sprintf("unrelated-%s", randSuffix))
	for _, d := range []string{dirA, dirB, dirC} {
		if err := mkdirAll(d); err != nil {
			t.Fatal(err)
		}
	}

	createOnDiskSQLite(t, dirA, pathA)
	createOnDiskSQLite(t, dirB, pathB)
	createOnDiskSQLite(t, dirC, pathUnrelated)

	registered := map[string]string{
		pathA: hashA,
		pathB: hashB,
	}

	discovered := harvest.ScanOpenCodeDBRoot(ctx, dbRoot, registered, logger)

	if len(discovered) != 2 {
		t.Fatalf("ScanOpenCodeDBRoot returned %d entries, want 2; got: %v", len(discovered), discovered)
	}

	byPath := make(map[string]harvest.DiscoveredDB, 2)
	for _, d := range discovered {
		byPath[d.DBPath] = d
	}

	dbPathA := filepath.Join(dirA, "opencode.db")
	dbPathB := filepath.Join(dirB, "opencode.db")
	dbPathC := filepath.Join(dirC, "opencode.db")

	if _, ok := byPath[dbPathA]; !ok {
		t.Errorf("proj-a DB (%s) missing from discovered", dbPathA)
	}
	if _, ok := byPath[dbPathB]; !ok {
		t.Errorf("proj-b DB (%s) missing from discovered", dbPathB)
	}
	if _, ok := byPath[dbPathC]; ok {
		t.Errorf("unrelated DB (%s) should NOT be in discovered", dbPathC)
	}

	if d, ok := byPath[dbPathA]; ok && d.WorkspaceHash != hashA {
		t.Errorf("proj-a hash = %q, want %q", d.WorkspaceHash, hashA)
	}
	if d, ok := byPath[dbPathB]; ok && d.WorkspaceHash != hashB {
		t.Errorf("proj-b hash = %q, want %q", d.WorkspaceHash, hashB)
	}

	enqueuerA := &testEnqueuer{}
	enqueuerB := &testEnqueuer{}
	enqueuersByPath := map[string]*testEnqueuer{
		dbPathA: enqueuerA,
		dbPathB: enqueuerB,
	}

	for _, d := range discovered {
		h := harvest.NewOpenCodeSQLiteHarvester(pgDB, logger, d.DBPath)
		enq := enqueuersByPath[d.DBPath]
		harvested, _, errCount := h.HarvestAll(ctx, enq)
		if harvested < 1 {
			t.Errorf("DB %s: harvested = %d, want >= 1", d.DBPath, harvested)
		}
		if errCount != 0 {
			t.Errorf("DB %s: errCount = %d, want 0", d.DBPath, errCount)
		}
	}

	qSQL := sqlc.New(pgDB)
	assertDocs(t, qSQL, ctx, hashA, "proj-a")
	assertDocs(t, qSQL, ctx, hashB, "proj-b")

	hashUnrelated, hashErr := storage.WorkspaceHash(pathUnrelated)
	if hashErr != nil {
		t.Fatalf("hash unrelated: %v", hashErr)
	}
	docsUnrelated, listErr := qSQL.ListDocumentsByWorkspace(ctx, hashUnrelated)
	if listErr != nil {
		t.Fatalf("list docs for unrelated: %v", listErr)
	}
	if len(docsUnrelated) != 0 {
		t.Errorf("unrelated: expected 0 documents in PG, got %d", len(docsUnrelated))
	}

	if len(enqueuerA.ids) == 0 {
		t.Errorf("proj-a: no chunk IDs enqueued")
	}
	if len(enqueuerB.ids) == 0 {
		t.Errorf("proj-b: no chunk IDs enqueued")
	}
}

func TestScanOpenCodeDBRoot_Integration(t *testing.T) {
	pgDB := setupIntegrationPG(t)

	root := t.TempDir()
	worktreeA := t.TempDir()
	worktreeB := t.TempDir()

	dirA := filepath.Join(root, "proj-a")
	dirB := filepath.Join(root, "proj-b")
	dirC := filepath.Join(root, "proj-c")
	for _, d := range []string{dirA, dirB, dirC} {
		if err := mkdirAll(d); err != nil {
			t.Fatal(err)
		}
	}

	_ = createOnDiskSQLite(t, dirA, worktreeA)
	_ = createOnDiskSQLite(t, dirB, worktreeB)

	dbC := filepath.Join(dirC, "opencode.db")
	db, err := sql.Open("sqlite", dbC)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec(`CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL)`)
	_, _ = db.Exec(`INSERT INTO project (id, worktree) VALUES (?, ?)`, "p", "/")
	db.Close()

	registered := map[string]string{
		worktreeA: wsHashFn(worktreeA),
	}

	logger := zerolog.Nop()
	discovered := harvest.ScanOpenCodeDBRoot(context.Background(), root, registered, logger)

	if len(discovered) != 1 {
		t.Fatalf("discovered = %d, want 1", len(discovered))
	}
	if discovered[0].Worktree != worktreeA {
		t.Errorf("worktree = %q, want %q", discovered[0].Worktree, worktreeA)
	}
	if discovered[0].WorkspaceHash != wsHashFn(worktreeA) {
		t.Errorf("workspace_hash mismatch")
	}

	for _, d := range discovered {
		h := harvest.NewOpenCodeSQLiteHarvester(pgDB, zerolog.Nop(), d.DBPath)
		harvested, _, errCount := h.HarvestAll(context.Background(), nil)
		if errCount != 0 {
			t.Errorf("db %s: errCount = %d, want 0", d.DBPath, errCount)
		}
		_ = harvested
	}
}

func wsHashFn(path string) string {
	h, err := storage.WorkspaceHash(path)
	if err != nil {
		panic(err)
	}
	return h
}
