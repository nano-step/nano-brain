//go:build integration

package harvest_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/rs/zerolog"
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

func wsHash(path string) string {
	h := sha256.Sum256([]byte(path))
	return hex.EncodeToString(h[:])
}

func TestScanOpenCodeDBRoot_Integration(t *testing.T) {
	pgDB := setupIntegrationPG(t)
	_ = pgDB

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
		worktreeA: wsHash(worktreeA),
	}

	logger := zerolog.Nop()
	discovered := harvest.ScanOpenCodeDBRoot(context.Background(), root, registered, logger)

	if len(discovered) != 1 {
		t.Fatalf("discovered = %d, want 1", len(discovered))
	}
	if discovered[0].Worktree != worktreeA {
		t.Errorf("worktree = %q, want %q", discovered[0].Worktree, worktreeA)
	}
	if discovered[0].WorkspaceHash != wsHash(worktreeA) {
		t.Errorf("workspace_hash mismatch")
	}

	for _, d := range discovered {
		h := harvest.NewOpenCodeSQLiteHarvester(pgDB, zerolog.Nop(), d.DBPath)
		harvested, _, errCount := h.HarvestAll(context.Background(), nil)
		if harvested < 1 {
			t.Errorf("db-A: harvested = %d, want >= 1", harvested)
		}
		if errCount != 0 {
			t.Errorf("db-A: errCount = %d, want 0", errCount)
		}
	}
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}
