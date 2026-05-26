package harvest_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/harvest"
	_ "modernc.org/sqlite"
)

func setupTestSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (
			id TEXT PRIMARY KEY,
			title TEXT,
			time_created INTEGER
		);
		CREATE TABLE message (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			time_created INTEGER,
			data TEXT
		);
		CREATE TABLE part (
			id TEXT PRIMARY KEY,
			message_id TEXT,
			session_id TEXT,
			time_created INTEGER,
			data TEXT
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func insertTestSession(t *testing.T, db *sql.DB, id, title string, createdMs int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO session (id, title, time_created) VALUES (?, ?, ?)`, id, title, createdMs)
	if err != nil {
		t.Fatal(err)
	}
}

func insertTestMessage(t *testing.T, db *sql.DB, id, sessionID, role string, createdMs int64) {
	t.Helper()
	data := `{"role":"` + role + `"}`
	_, err := db.Exec(`INSERT INTO message (id, session_id, time_created, data) VALUES (?, ?, ?, ?)`, id, sessionID, createdMs, data)
	if err != nil {
		t.Fatal(err)
	}
}

func insertTestPart(t *testing.T, db *sql.DB, id, messageID, partType, content string) {
	t.Helper()
	data := `{"type":"` + partType + `","text":"` + content + `"}`
	_, err := db.Exec(`INSERT INTO part (id, message_id, session_id, time_created, data) VALUES (?, ?, '', 0, ?)`, id, messageID, data)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderSQLiteMarkdown_Format(t *testing.T) {
	sqdb := setupTestSQLiteDB(t)
	now := time.Now().UnixMilli()

	insertTestSession(t, sqdb, "sess1", "My Session", now)
	insertTestMessage(t, sqdb, "msg1", "sess1", "user", now)
	insertTestMessage(t, sqdb, "msg2", "sess1", "assistant", now+1000)
	insertTestPart(t, sqdb, "p1", "msg1", "text", "Hello!")
	insertTestPart(t, sqdb, "p2", "msg2", "text", "World!")

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil, "test-workspace")

	md, err := h.RenderSession(context.Background(), "sess1", "My Session", time.UnixMilli(now))
	if err != nil {
		t.Fatal(err)
	}

	if md == "" {
		t.Error("expected non-empty markdown")
	}
	for _, want := range []string{"session_id: sess1", "source: opencode", "## user", "Hello!", "## assistant", "World!"} {
		if !contains(md, want) {
			t.Errorf("markdown missing %q\ngot:\n%s", want, md)
		}
	}
}

func TestOpenCodeSQLiteHarvester_EmptyDB(t *testing.T) {
	sqdb := setupTestSQLiteDB(t)
	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil, "test-workspace")
	sessions, err := h.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestOpenCodeSQLiteHarvester_ListSessions(t *testing.T) {
	sqdb := setupTestSQLiteDB(t)
	now := time.Now().UnixMilli()
	insertTestSession(t, sqdb, "s1", "Session One", now)
	insertTestSession(t, sqdb, "s2", "Session Two", now+1000)

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil, "test-workspace")
	sessions, err := h.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func contains(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && (s == sub || len(s) >= len(sub) && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
