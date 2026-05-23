package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

type mockWriter struct {
	mu   sync.Mutex
	docs []UpsertParams
	fail map[string]bool
}

func (w *mockWriter) UpsertDocument(_ context.Context, p UpsertParams) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fail[p.SourcePath] {
		return fmt.Errorf("injected error for %s", p.SourcePath)
	}
	w.docs = append(w.docs, p)
	return nil
}

func createV1DB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_path TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL DEFAULT '',
		tags TEXT DEFAULT '',
		collection TEXT DEFAULT '',
		created_at TEXT DEFAULT (datetime('now')),
		updated_at TEXT DEFAULT (datetime('now'))
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func insertDoc(t *testing.T, db *sql.DB, path, title, content, tags, col string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO documents (source_path, title, content, tags, collection) VALUES (?, ?, ?, ?, ?)`,
		path, title, content, tags, col,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func migratorFromDB(t *testing.T, db *sql.DB, w Writer) *V1Migrator {
	t.Helper()
	return &V1Migrator{srcDB: db, writer: w}
}

func TestMigrate_BasicDocuments(t *testing.T) {
	db := createV1DB(t)
	defer db.Close()

	insertDoc(t, db, "a.md", "Doc A", "hello world", "", "notes")
	insertDoc(t, db, "b.md", "Doc B", "goodbye", `["tag1","tag2"]`, "")

	w := &mockWriter{}
	m := migratorFromDB(t, db, w)

	res, err := m.Migrate(context.Background(), "ws123", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 {
		t.Fatalf("total: got %d, want 2", res.Total)
	}
	if res.Migrated != 2 {
		t.Fatalf("migrated: got %d, want 2", res.Migrated)
	}
	if res.Skipped != 0 {
		t.Fatalf("skipped: got %d, want 0", res.Skipped)
	}

	if len(w.docs) != 2 {
		t.Fatalf("writer docs: got %d, want 2", len(w.docs))
	}

	got := w.docs[0]
	if got.WorkspaceHash != "ws123" {
		t.Errorf("workspace_hash: got %q, want %q", got.WorkspaceHash, "ws123")
	}
	if got.Collection != "notes" {
		t.Errorf("collection: got %q, want %q", got.Collection, "notes")
	}
	if got.ContentHash != contentHash("hello world") {
		t.Errorf("content_hash mismatch")
	}

	got2 := w.docs[1]
	if len(got2.Tags) != 2 || got2.Tags[0] != "tag1" || got2.Tags[1] != "tag2" {
		t.Errorf("tags: got %v, want [tag1 tag2]", got2.Tags)
	}
	if got2.Collection != "v1-import" {
		t.Errorf("empty collection should default to v1-import, got %q", got2.Collection)
	}
}

func TestMigrate_EmptyDatabase(t *testing.T) {
	db := createV1DB(t)
	defer db.Close()

	w := &mockWriter{}
	m := migratorFromDB(t, db, w)

	res, err := m.Migrate(context.Background(), "ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 || res.Migrated != 0 {
		t.Fatalf("empty db: got total=%d migrated=%d", res.Total, res.Migrated)
	}
}

func TestMigrate_EmptyContentSkipped(t *testing.T) {
	db := createV1DB(t)
	defer db.Close()

	insertDoc(t, db, "empty.md", "Empty", "", "", "")

	w := &mockWriter{}
	m := migratorFromDB(t, db, w)

	res, err := m.Migrate(context.Background(), "ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Skipped != 1 {
		t.Fatalf("skipped: got %d, want 1", res.Skipped)
	}
	if len(w.docs) != 0 {
		t.Fatalf("writer should have 0 docs, got %d", len(w.docs))
	}
}

func TestMigrate_WriterErrorContinues(t *testing.T) {
	db := createV1DB(t)
	defer db.Close()

	insertDoc(t, db, "fail.md", "Fail", "content1", "", "")
	insertDoc(t, db, "ok.md", "OK", "content2", "", "")

	w := &mockWriter{fail: map[string]bool{"fail.md": true}}
	m := migratorFromDB(t, db, w)

	res, err := m.Migrate(context.Background(), "ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Migrated != 1 {
		t.Fatalf("migrated: got %d, want 1", res.Migrated)
	}
	if res.Failed != 1 {
		t.Fatalf("failed: got %d, want 1", res.Failed)
	}
	if res.Skipped != 0 {
		t.Fatalf("skipped: got %d, want 0", res.Skipped)
	}
	if len(res.Errors) != 1 {
		t.Fatalf("errors: got %d, want 1", len(res.Errors))
	}
}

func TestMigrate_NoDocumentsTable(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	w := &mockWriter{}
	m := migratorFromDB(t, db, w)

	_, err = m.Migrate(context.Background(), "ws", nil)
	if err == nil {
		t.Fatal("expected error for missing documents table")
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"  ", nil},
		{"[]", nil},
		{`["a","b"]`, []string{"a", "b"}},
		{"foo,bar,baz", []string{"foo", "bar", "baz"}},
		{" foo , bar ", []string{"foo", "bar"}},
		{`["single"]`, []string{"single"}},
	}
	for _, tt := range tests {
		got := parseTags(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseTags(%q): got %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("parseTags(%q)[%d]: got %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestMigrate_ProgressCallback(t *testing.T) {
	db := createV1DB(t)
	defer db.Close()

	insertDoc(t, db, "a.md", "A", "aaa", "", "")
	insertDoc(t, db, "b.md", "B", "bbb", "", "")
	insertDoc(t, db, "c.md", "C", "ccc", "", "")

	w := &mockWriter{}
	m := migratorFromDB(t, db, w)

	var calls []int
	res, err := m.Migrate(context.Background(), "ws", func(current, total int) {
		calls = append(calls, current)
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 3 || res.Migrated != 3 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(calls) != 3 || calls[0] != 1 || calls[1] != 2 || calls[2] != 3 {
		t.Fatalf("progress calls: got %v, want [1 2 3]", calls)
	}
}

func TestMigrate_CancelledContext(t *testing.T) {
	db := createV1DB(t)
	defer db.Close()

	insertDoc(t, db, "a.md", "A", "aaa", "", "")
	insertDoc(t, db, "b.md", "B", "bbb", "", "")

	w := &mockWriter{}
	m := migratorFromDB(t, db, w)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, err := m.Migrate(ctx, "ws", nil)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if res != nil && res.Migrated != 0 {
		t.Fatalf("migrated: got %d, want 0", res.Migrated)
	}
	if len(w.docs) != 0 {
		t.Fatalf("writer should have 0 docs, got %d", len(w.docs))
	}
}
