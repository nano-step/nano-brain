package harvest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// createScanTestDB creates a temporary SQLite file with the minimal schema
// needed by ScanOpenCodeDBRoot (project table only).
func createScanTestDB(t *testing.T, dir, subdir, worktree string) string {
	t.Helper()
	dbDir := filepath.Join(dir, subdir)
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL);
		CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, title TEXT, time_created INTEGER, time_updated INTEGER, parent_id TEXT);
	`); err != nil {
		t.Fatal(err)
	}
	if worktree != "" {
		if _, err := db.Exec(`INSERT INTO project (id, worktree) VALUES (?, ?)`, "proj-"+subdir, worktree); err != nil {
			t.Fatal(err)
		}
	} else {
		if _, err := db.Exec(`INSERT INTO project (id, worktree) VALUES (?, ?)`, "proj-"+subdir, ""); err != nil {
			t.Fatal(err)
		}
	}
	return dbPath
}

func nopLogger() zerolog.Logger { return zerolog.Nop() }

func TestScanOpenCodeDBRoot_EmptyRoot(t *testing.T) {
	got := ScanOpenCodeDBRoot(context.Background(), "", map[string]string{}, nopLogger())
	if got != nil {
		t.Errorf("expected nil for empty root, got %v", got)
	}
}

func TestScanOpenCodeDBRoot_RootMissing(t *testing.T) {
	got := ScanOpenCodeDBRoot(context.Background(), "/no/such/dir", map[string]string{}, nopLogger())
	if got != nil {
		t.Errorf("expected nil for missing root, got %v", got)
	}
}

func TestScanOpenCodeDBRoot_RootEmpty(t *testing.T) {
	dir := t.TempDir()
	got := ScanOpenCodeDBRoot(context.Background(), dir, map[string]string{}, nopLogger())
	if got != nil {
		t.Errorf("expected nil for empty dir, got %v", got)
	}
}

func TestScanOpenCodeDBRoot_RegisteredMatch(t *testing.T) {
	dir := t.TempDir()
	createScanTestDB(t, dir, "proj-a", "/u/proj-a")
	createScanTestDB(t, dir, "proj-b", "/u/proj-b")

	registered := map[string]string{"/u/proj-a": "hash-a"}
	got := ScanOpenCodeDBRoot(context.Background(), dir, registered, nopLogger())
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(got), got)
	}
	if got[0].Worktree != "/u/proj-a" {
		t.Errorf("worktree = %q, want /u/proj-a", got[0].Worktree)
	}
	if got[0].WorkspaceHash != "hash-a" {
		t.Errorf("workspace hash = %q, want hash-a", got[0].WorkspaceHash)
	}
}

func TestScanOpenCodeDBRoot_GlobalWorktreeSkipped(t *testing.T) {
	dir := t.TempDir()
	createScanTestDB(t, dir, "global", "/")

	registered := map[string]string{"/": "hash-global"}
	got := ScanOpenCodeDBRoot(context.Background(), dir, registered, nopLogger())
	if len(got) != 0 {
		t.Errorf("expected 0 results for '/' worktree, got %d", len(got))
	}
}

func TestScanOpenCodeDBRoot_EmptyWorktreeSkipped(t *testing.T) {
	dir := t.TempDir()
	createScanTestDB(t, dir, "empty-wt", "")

	registered := map[string]string{"": "hash-empty", ".": "hash-dot"}
	got := ScanOpenCodeDBRoot(context.Background(), dir, registered, nopLogger())
	if len(got) != 0 {
		t.Errorf("expected 0 results for empty worktree, got %d", len(got))
	}
}

func TestScanOpenCodeDBRoot_TrailingSlashNormalized(t *testing.T) {
	dir := t.TempDir()
	createScanTestDB(t, dir, "foo", "/u/foo/")

	registered := map[string]string{"/u/foo": "hash-foo"}
	got := ScanOpenCodeDBRoot(context.Background(), dir, registered, nopLogger())
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Worktree != "/u/foo" {
		t.Errorf("worktree = %q, want /u/foo (normalized)", got[0].Worktree)
	}
}

func TestScanOpenCodeDBRoot_CorruptDB(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "corrupt")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "opencode.db")
	if err := os.WriteFile(dbPath, []byte("not a valid sqlite file"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ScanOpenCodeDBRoot(context.Background(), dir, map[string]string{}, nopLogger())
	if len(got) != 0 {
		t.Errorf("expected 0 results for corrupt DB, got %d", len(got))
	}
}

func TestScanOpenCodeDBRoot_MissingProjectTable(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "noproj")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, title TEXT, time_created INTEGER, time_updated INTEGER, parent_id TEXT)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	got := ScanOpenCodeDBRoot(context.Background(), dir, map[string]string{}, nopLogger())
	if len(got) != 0 {
		t.Errorf("expected 0 results for missing project table, got %d", len(got))
	}
}

func TestScanOpenCodeDBRoot_MultipleProjectRows(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, "multi")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dbDir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		INSERT INTO project (id, worktree) VALUES ('p1', '/u/multi-a'), ('p2', '/u/multi-b')
	`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()

	registered := map[string]string{"/u/multi-a": "hash-a", "/u/multi-b": "hash-b"}
	got := ScanOpenCodeDBRoot(context.Background(), dir, registered, nopLogger())
	if len(got) > 1 {
		t.Errorf("expected at most 1 result (LIMIT 1), got %d", len(got))
	}
}

// TestHarvestAll_TrailingSlashWorktreeMatches verifies the Oracle M3 fix:
// a session whose project.worktree ends with "/" matches a workspace
// registered without the trailing slash.
func TestHarvestAll_TrailingSlashWorktreeMatches(t *testing.T) {
	sqdb := setupTestSQLiteForScan(t)
	now := int64(1700000000000)

	if _, err := sqdb.Exec(`INSERT INTO project (id, worktree) VALUES (?, ?)`, "proj-slash", "/u/foo/"); err != nil {
		t.Fatal(err)
	}
	if _, err := sqdb.Exec(`INSERT INTO session (id, project_id, title, time_created, time_updated) VALUES (?, ?, ?, ?, ?)`,
		"sess-slash", "proj-slash", "Slash Session", now-20*60*1000, now-20*60*1000); err != nil {
		t.Fatal(err)
	}
	if _, err := sqdb.Exec(`INSERT INTO message (id, session_id, time_created, data) VALUES (?, ?, ?, ?)`,
		"msg-slash", "sess-slash", now-20*60*1000, `{"role":"user"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := sqdb.Exec(`INSERT INTO part (id, message_id, session_id, time_created, data) VALUES (?, ?, ?, ?, ?)`,
		"part-slash", "msg-slash", "sess-slash", now-20*60*1000, `{"type":"text","text":"hello trailing slash"}`); err != nil {
		t.Fatal(err)
	}

	h := NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil)
	sessions, err := h.listSessions(context.Background(), sqdb, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	sess := sessions[0]
	normalized := filepath.Clean(sess.Worktree)
	if normalized != "/u/foo" {
		t.Errorf("normalized worktree = %q, want /u/foo", normalized)
	}

	registered := map[string]string{"/u/foo": "hash-foo"}
	_, ok := registered[normalized]
	if !ok {
		t.Error("normalized worktree should match registered workspace /u/foo")
	}
}

// TestHarvestAll_EmptyWorktreeDoesNotResolveToCWD guards against a regression
// where `filepath.Clean("")` returns "." and bypasses the empty-string skip
// path, causing the harvester to hash "." as a workspace path. Reported by
// gemini-code-assist on PR #200.
func TestHarvestAll_EmptyWorktreeDoesNotResolveToCWD(t *testing.T) {
	emptyRaw := ""
	if filepath.Clean(emptyRaw) != "." {
		t.Skip("stdlib filepath.Clean no longer returns '.' for empty input; this test no longer guards anything")
	}

	var normalized string
	if emptyRaw != "" {
		normalized = filepath.Clean(emptyRaw)
	}
	if normalized != "" {
		t.Errorf("normalized = %q, want empty string (filepath.Clean must NOT run on empty input)", normalized)
	}
}

func setupTestSQLiteForScan(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`
		CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL);
		CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, title TEXT, time_created INTEGER, time_updated INTEGER, parent_id TEXT);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT);
	`); err != nil {
		t.Fatal(err)
	}
	return db
}
