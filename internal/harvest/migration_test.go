package harvest_test

import (
	"context"
	"database/sql"
	"reflect"
	"sort"
	"testing"

	"github.com/lib/pq"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// setupMigrationTestDB creates an in-memory SQLite database with a minimal
// documents table compatible with BackfillSessionTicketTags.
// tags is stored as TEXT; pq.Array scanning parses the {a,b} PG wire format
// from the stored string, making the test fully in-process with no PG needed.
func setupMigrationTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE documents (
			id         TEXT PRIMARY KEY,
			collection TEXT NOT NULL,
			content    TEXT NOT NULL DEFAULT '',
			tags       TEXT NOT NULL DEFAULT '{}'
		)
	`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

// queryTags reads the tags column for the given doc id and returns them as a
// sorted slice, using pq.Array to parse the PostgreSQL array wire format.
func queryTags(t *testing.T, db *sql.DB, id string) []string {
	t.Helper()
	var tags []string
	row := db.QueryRow(`SELECT tags FROM documents WHERE id = ?`, id)
	if err := row.Scan(pq.Array(&tags)); err != nil {
		t.Fatalf("queryTags(%s): %v", id, err)
	}
	sort.Strings(tags)
	return tags
}

func newBackfillExtractor(t *testing.T) *harvest.TicketExtractor {
	t.Helper()
	te, err := harvest.NewTicketExtractor(nil)
	if err != nil {
		t.Fatalf("NewTicketExtractor: %v", err)
	}
	return te
}

// TestBackfillSessionTicketTags_TagsUntaggedDoc verifies that a session doc
// whose content mentions a ticket ID and currently has no ticket tag gets
// tagged after the backfill.
func TestBackfillSessionTicketTags_TagsUntaggedDoc(t *testing.T) {
	db := setupMigrationTestDB(t)
	ctx := context.Background()

	_, err := db.Exec(
		`INSERT INTO documents (id, collection, content, tags) VALUES (?, ?, ?, ?)`,
		"doc-1", "sessions", "Working on DEV-1234 today.", "{}",
	)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	te := newBackfillExtractor(t)
	tagged, err := harvest.BackfillSessionTicketTags(ctx, db, te, zerolog.Nop())
	if err != nil {
		t.Fatalf("BackfillSessionTicketTags: %v", err)
	}
	if tagged != 1 {
		t.Errorf("tagged: got %d, want 1", tagged)
	}

	got := queryTags(t, db, "doc-1")
	want := []string{"ticket:DEV-1234"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tags after backfill: got %v, want %v", got, want)
	}
}

// TestBackfillSessionTicketTags_Idempotent verifies that a doc already carrying
// a ticket tag is skipped on a second run (tagged=0).
func TestBackfillSessionTicketTags_Idempotent(t *testing.T) {
	db := setupMigrationTestDB(t)
	ctx := context.Background()

	// Pre-tagged doc — should not be touched.
	_, err := db.Exec(
		`INSERT INTO documents (id, collection, content, tags) VALUES (?, ?, ?, ?)`,
		"doc-2", "sessions", "Working on DEV-1234 today.", `{"ticket:DEV-1234"}`,
	)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	te := newBackfillExtractor(t)
	tagged, err := harvest.BackfillSessionTicketTags(ctx, db, te, zerolog.Nop())
	if err != nil {
		t.Fatalf("BackfillSessionTicketTags: %v", err)
	}
	if tagged != 0 {
		t.Errorf("idempotent: tagged=%d, want 0", tagged)
	}

	// Tags must be unchanged.
	got := queryTags(t, db, "doc-2")
	want := []string{"ticket:DEV-1234"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tags after idempotent run: got %v, want %v", got, want)
	}
}

// TestBackfillSessionTicketTags_NonSessionSkipped verifies that documents in
// collections other than "sessions" are not touched.
func TestBackfillSessionTicketTags_NonSessionSkipped(t *testing.T) {
	db := setupMigrationTestDB(t)
	ctx := context.Background()

	_, err := db.Exec(
		`INSERT INTO documents (id, collection, content, tags) VALUES (?, ?, ?, ?)`,
		"doc-3", "memory", "Working on DEV-9999 today.", "{}",
	)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	te := newBackfillExtractor(t)
	tagged, err := harvest.BackfillSessionTicketTags(ctx, db, te, zerolog.Nop())
	if err != nil {
		t.Fatalf("BackfillSessionTicketTags: %v", err)
	}
	if tagged != 0 {
		t.Errorf("non-session skip: tagged=%d, want 0", tagged)
	}
}

// TestBackfillSessionTicketTags_ExistingTagsPreserved verifies that existing
// non-ticket tags are kept when ticket tags are appended.
func TestBackfillSessionTicketTags_ExistingTagsPreserved(t *testing.T) {
	db := setupMigrationTestDB(t)
	ctx := context.Background()

	_, err := db.Exec(
		`INSERT INTO documents (id, collection, content, tags) VALUES (?, ?, ?, ?)`,
		"doc-4", "sessions", "Fixed PROJ-55 as discussed.", `{"bug-fix","feature"}`,
	)
	if err != nil {
		t.Fatalf("insert doc: %v", err)
	}

	te := newBackfillExtractor(t)
	tagged, err := harvest.BackfillSessionTicketTags(ctx, db, te, zerolog.Nop())
	if err != nil {
		t.Fatalf("BackfillSessionTicketTags: %v", err)
	}
	if tagged != 1 {
		t.Errorf("tagged: got %d, want 1", tagged)
	}

	got := queryTags(t, db, "doc-4")
	want := []string{"bug-fix", "feature", "ticket:PROJ-55"}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tags after backfill: got %v, want %v", got, want)
	}
}
