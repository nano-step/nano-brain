package harvest

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
)

// dbStamp is the signal the harvest skip depends on. If it stops changing when
// the file changes, new sessions are silently never harvested; if it stops
// being stable, every cycle re-opens every DB and the page-cache churn returns.
func TestDBStamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.db")
	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := NewOpenCodeSQLiteHarvester(nil, zerolog.Nop(), path)

	first := h.dbStamp()
	if first == "" {
		t.Fatal("stamp is empty for a file that exists, which disables the skip entirely")
	}
	if again := h.dbStamp(); again != first {
		t.Fatalf("stamp is not stable for an unchanged file: %q then %q", first, again)
	}

	if err := os.WriteFile(path, []byte("one plus more"), 0o600); err != nil {
		t.Fatal(err)
	}
	if changed := h.dbStamp(); changed == first {
		t.Fatal("stamp did not change after the file was written; new sessions would never be harvested")
	}

	// An injected connection makes dbPath meaningless, so the skip must not
	// engage. dbStamp only tests the pointer for nil, so an empty DB suffices.
	h.sqdb = &sql.DB{}
	if s := h.dbStamp(); s != "" {
		t.Fatalf("stamp should be empty when a connection is injected, got %q", s)
	}
}
