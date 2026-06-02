package harvest_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/migrations"
	"github.com/pressly/goose/v3"
	"github.com/sqlc-dev/pqtype"
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
		CREATE TABLE project (
			id TEXT PRIMARY KEY,
			worktree TEXT NOT NULL
		);
		CREATE TABLE session (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			title TEXT,
			time_created INTEGER,
			time_updated INTEGER
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

func insertTestProject(t *testing.T, db *sql.DB, id, worktree string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO project (id, worktree) VALUES (?, ?)`, id, worktree)
	if err != nil {
		t.Fatal(err)
	}
}

func insertTestSession(t *testing.T, db *sql.DB, id, projectID, title string, createdMs int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO session (id, project_id, title, time_created) VALUES (?, ?, ?, ?)`, id, projectID, title, createdMs)
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

// seedRegisteredWorkspace registers a workspace in PG so OpenCode harvest will
// not skip sessions for this worktree. Required since #238 removed the
// auto-registration behavior — workspaces must be explicitly registered.
func seedRegisteredWorkspace(t *testing.T, pgDB *sql.DB, worktree string) string {
	t.Helper()
	h := sha256.Sum256([]byte(worktree))
	wsHash := hex.EncodeToString(h[:])
	q := sqlc.New(pgDB)
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: "test-" + wsHash[:8],
		Path: worktree,
	}); err != nil {
		t.Fatalf("seedRegisteredWorkspace: %v", err)
	}
	return wsHash
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

	insertTestProject(t, sqdb, "proj1", "/home/user/app")
	insertTestSession(t, sqdb, "sess1", "proj1", "My Session", now)
	insertTestMessage(t, sqdb, "msg1", "sess1", "user", now)
	insertTestMessage(t, sqdb, "msg2", "sess1", "assistant", now+1000)
	insertTestPart(t, sqdb, "p1", "msg1", "text", "Hello!")
	insertTestPart(t, sqdb, "p2", "msg2", "text", "World!")

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil)

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
	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil)
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
	insertTestProject(t, sqdb, "proj1", "/home/user/app")
	insertTestSession(t, sqdb, "s1", "proj1", "Session One", now)
	insertTestSession(t, sqdb, "s2", "proj1", "Session Two", now+1000)

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil)
	sessions, err := h.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestOpenCodeSQLiteHarvester_PerProjectWorkspace(t *testing.T) {
	sqdb := setupTestSQLiteDB(t)
	insertTestProject(t, sqdb, "proj-a", "/home/user/app-a")
	insertTestProject(t, sqdb, "proj-b", "/home/user/app-b")
	now := time.Now().UnixMilli()
	insertTestSession(t, sqdb, "s-a", "proj-a", "Session A", now)
	insertTestSession(t, sqdb, "s-b", "proj-b", "Session B", now+1000)

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil)
	sessions, err := h.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	var sA, sB harvest.SqSession
	for _, s := range sessions {
		if s.ID == "s-a" {
			sA = s
		}
		if s.ID == "s-b" {
			sB = s
		}
	}
	if sA.Worktree != "/home/user/app-a" {
		t.Errorf("s-a worktree = %q, want /home/user/app-a", sA.Worktree)
	}
	if sB.Worktree != "/home/user/app-b" {
		t.Errorf("s-b worktree = %q, want /home/user/app-b", sB.Worktree)
	}
}

func TestOpenCodeSQLiteHarvester_OrphanedSession(t *testing.T) {
	sqdb := setupTestSQLiteDB(t)
	now := time.Now().UnixMilli()
	_, err := sqdb.Exec(`INSERT INTO session (id, project_id, title, time_created) VALUES (?, ?, ?, ?)`,
		"orphan", "nonexistent-proj", "Orphan", now)
	if err != nil {
		t.Fatal(err)
	}

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil)
	sessions, err := h.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Worktree != "" {
		t.Errorf("orphaned session worktree = %q, want empty string", sessions[0].Worktree)
	}
}

func TestOpenCodeSQLite_AgeGate_SkipsActiveSessions(t *testing.T) {
	sqdb := setupTestSQLiteDB(t)
	now := time.Now().UnixMilli()

	insertTestProject(t, sqdb, "proj1", "/home/user/app")

	// Session updated 5 minutes ago (active)
	activeSessionID := "sess-active"
	activeUpdatedMs := now - 5*60*1000 // 5 minutes ago
	insertTestSession(t, sqdb, activeSessionID, "proj1", "Active Session", now)
	_, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, activeUpdatedMs, activeSessionID)
	if err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg1", activeSessionID, "user", now)
	insertTestPart(t, sqdb, "p1", "msg1", "text", "Hello!")

	// Session updated 15 minutes ago (inactive)
	inactiveSessionID := "sess-inactive"
	inactiveUpdatedMs := now - 15*60*1000 // 15 minutes ago
	insertTestSession(t, sqdb, inactiveSessionID, "proj1", "Inactive Session", now-20*60*1000)
	_, err = sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, inactiveUpdatedMs, inactiveSessionID)
	if err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg2", inactiveSessionID, "user", now-20*60*1000)
	insertTestPart(t, sqdb, "p2", "msg2", "text", "World!")

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, nil)

	// Check that active session is marked with UpdatedAt
	sessions, err := h.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	var activeSess, inactiveSess harvest.SqSession
	for _, s := range sessions {
		if s.ID == activeSessionID {
			activeSess = s
		}
		if s.ID == inactiveSessionID {
			inactiveSess = s
		}
	}

	// Verify active session has UpdatedAt set to ~5 minutes ago
	if activeSess.UpdatedAt.IsZero() {
		t.Error("active session UpdatedAt should not be zero")
	}
	activeDiff := time.Since(activeSess.UpdatedAt)
	if activeDiff < 4*time.Minute || activeDiff > 6*time.Minute {
		t.Errorf("active session time diff = %v, want ~5 minutes", activeDiff)
	}

	// Verify inactive session has UpdatedAt set to ~15 minutes ago
	if inactiveSess.UpdatedAt.IsZero() {
		t.Error("inactive session UpdatedAt should not be zero")
	}
	inactiveDiff := time.Since(inactiveSess.UpdatedAt)
	if inactiveDiff < 14*time.Minute || inactiveDiff > 16*time.Minute {
		t.Errorf("inactive session time diff = %v, want ~15 minutes", inactiveDiff)
	}
}

func TestOpenCodeSQLite_AgeGate_HarvestSkipsActive(t *testing.T) {
	pgDB := setupTestPG(t)
	sqdb := setupTestSQLiteDB(t)

	now := time.Now()
	activeMs := now.Add(-5 * time.Minute).UnixMilli()
	inactiveMs := now.Add(-15 * time.Minute).UnixMilli()

	insertTestProject(t, sqdb, "proj1", "/home/user/app-age")
	seedRegisteredWorkspace(t, pgDB, "/home/user/app-age")

	// Active session (updated 5min ago)
	insertTestSession(t, sqdb, "sess-active", "proj1", "Active", activeMs)
	if _, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, activeMs, "sess-active"); err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg-a", "sess-active", "user", activeMs)
	insertTestPart(t, sqdb, "p-a", "msg-a", "text", "Hello active")

	// Inactive session (updated 15min ago)
	insertTestSession(t, sqdb, "sess-old", "proj1", "Old", inactiveMs)
	if _, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, inactiveMs, "sess-old"); err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg-o", "sess-old", "user", inactiveMs)
	insertTestPart(t, sqdb, "p-o", "msg-o", "text", "Hello old")

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB)

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)

	// Only the inactive session should be harvested (raw fallback, no summarizer)
	if harvested != 1 {
		t.Errorf("harvested = %d, want 1 (only inactive)", harvested)
	}
	// Active session is silently skipped (not counted in skipped — that's for content_hash matches)
	if errCount != 0 {
		t.Errorf("errCount = %d, want 0", errCount)
	}
	_ = skipped
}

func upsertMockSummary(ctx context.Context, pgDB *sql.DB, meta harvest.SummaryMeta) error {
	q := sqlc.New(pgDB)
	_, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: meta.WorkspaceHash,
		ContentHash:   "fake-summary-hash-" + meta.SessionID,
		Title:         "Summary: " + meta.Title,
		Content:       "# Mock Summary\n\nSummarized content for " + meta.SessionID,
		SourcePath:    "summary://opencode/" + meta.SessionID,
		Collection:    "session-summary",
		Tags:          []string{"summary", "opencode"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{"summary":true}`), Valid: true},
	})
	return err
}

func TestOpenCodeSQLite_SummarizerHappyPath_OnlySummaryDoc(t *testing.T) {
	pgDB := setupTestPG(t)
	sqdb := setupTestSQLiteDB(t)

	now := time.Now()
	oldMs := now.Add(-15 * time.Minute).UnixMilli()

	insertTestProject(t, sqdb, "proj1", "/home/user/test-happy")
	seedRegisteredWorkspace(t, pgDB, "/home/user/test-happy")
	insertTestSession(t, sqdb, "sess-happy1", "proj1", "Happy Session", oldMs)
	if _, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, oldMs, "sess-happy1"); err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg1", "sess-happy1", "user", oldMs)
	insertTestPart(t, sqdb, "p1", "msg1", "text", "Hello from happy test!")

	successSummarizer := &stubSummarizer{fn: func(ctx context.Context, md string, meta harvest.SummaryMeta) error {
		return upsertMockSummary(ctx, pgDB, meta)
	}}

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB).
		WithSummarizer(successSummarizer)

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)

	if harvested != 1 {
		t.Errorf("harvested = %d, want 1", harvested)
	}
	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	if errCount != 0 {
		t.Errorf("errCount = %d, want 0", errCount)
	}

	ctx := context.Background()
	q := sqlc.New(pgDB)

	// Verify summary doc at unified path
	var foundSummary bool
	rows, err := pgDB.QueryContext(ctx,
		`SELECT source_path, collection FROM documents WHERE source_path = $1`,
		"summary://opencode/sess-happy1",
	)
	if err != nil {
		t.Fatal("query summary doc: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var sp, coll string
		if err := rows.Scan(&sp, &coll); err != nil {
			t.Fatal("scan: " + err.Error())
		}
		foundSummary = true
		if coll != "session-summary" {
			t.Errorf("collection = %q, want %q", coll, "session-summary")
		}
	}
	if !foundSummary {
		t.Error("no doc found at summary://opencode/sess-happy1")
	}

	// Verify NO doc at old path
	_, lookupErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "opencode://session/sess-happy1",
		WorkspaceHash: "",
	})
	if lookupErr == nil {
		t.Error("expected no doc at old path opencode://session/sess-happy1, but found one")
	}
}

func TestOpenCodeSQLite_SkipCheck_UnifiedPath(t *testing.T) {
	pgDB := setupTestPG(t)
	sqdb := setupTestSQLiteDB(t)

	now := time.Now()
	oldMs := now.Add(-15 * time.Minute).UnixMilli()
	worktree := "/home/user/test-skip"

	insertTestProject(t, sqdb, "proj1", worktree)
	wsHash := seedRegisteredWorkspace(t, pgDB, worktree)
	insertTestSession(t, sqdb, "sess-skip1", "proj1", "Skip Session", oldMs)
	if _, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, oldMs, "sess-skip1"); err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg1", "sess-skip1", "user", oldMs)
	insertTestPart(t, sqdb, "p1", "msg1", "text", "Hello from skip test!")

	// Pre-insert doc at unified path to trigger skip
	q := sqlc.New(pgDB)
	_, err := q.UpsertDocumentBySourcePath(context.Background(), sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   "pre-existing-hash",
		Title:         "Pre-existing doc",
		Content:       "# Existing content",
		SourcePath:    "summary://opencode/sess-skip1",
		Collection:    "sessions",
		Tags:          []string{"opencode"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	if err != nil {
		t.Fatal("pre-insert doc: " + err.Error())
	}

	// Test without summarizer
	harvester := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB)
	harvested, skipped, errCount := harvester.HarvestAll(context.Background(), nil)

	if harvested != 0 {
		t.Errorf("without summarizer: harvested = %d, want 0", harvested)
	}
	if skipped != 1 {
		t.Errorf("without summarizer: skipped = %d, want 1", skipped)
	}
	if errCount != 0 {
		t.Errorf("without summarizer: errCount = %d, want 0", errCount)
	}

	// Test with summarizer — same result expected
	harvester2 := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB).
		WithSummarizer(failingSummarizer{})
	harvested2, skipped2, errCount2 := harvester2.HarvestAll(context.Background(), nil)

	if harvested2 != 0 {
		t.Errorf("with summarizer: harvested = %d, want 0", harvested2)
	}
	if skipped2 != 1 {
		t.Errorf("with summarizer: skipped = %d, want 1", skipped2)
	}
	if errCount2 != 0 {
		t.Errorf("with summarizer: errCount = %d, want 0", errCount2)
	}
}

func TestOpenCodeSQLite_MultiSession_Counters(t *testing.T) {
	pgDB := setupTestPG(t)
	sqdb := setupTestSQLiteDB(t)

	now := time.Now()
	oldMs := now.Add(-15 * time.Minute).UnixMilli()
	worktree := "/home/user/test-multi"

	insertTestProject(t, sqdb, "proj1", worktree)
	wsHash := seedRegisteredWorkspace(t, pgDB, worktree)

	wsH := sha256.Sum256([]byte(worktree))
	if wsHash != hex.EncodeToString(wsH[:]) {
		t.Fatalf("workspace hash mismatch: helper=%s, expected=%s", wsHash, hex.EncodeToString(wsH[:]))
	}

	sessionIDs := []string{"success-1", "success-2", "success-3", "fail-1", "fail-2", "skip-1"}
	for i, sid := range sessionIDs {
		ms := oldMs - int64(i*1000)
		insertTestSession(t, sqdb, sid, "proj1", "Session "+sid, ms)
		if _, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, ms, sid); err != nil {
			t.Fatal(err)
		}
		insertTestMessage(t, sqdb, "msg-"+sid, sid, "user", ms)
		insertTestPart(t, sqdb, "p-"+sid, "msg-"+sid, "text", "Content for "+sid)
	}

	// Pre-insert doc for skip-1
	q := sqlc.New(pgDB)
	_, err := q.UpsertDocumentBySourcePath(context.Background(), sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   "pre-existing-skip-hash",
		Title:         "Pre-existing skip doc",
		Content:       "# Skip content",
		SourcePath:    "summary://opencode/skip-1",
		Collection:    "sessions",
		Tags:          []string{"opencode"},
		Metadata:      pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true},
	})
	if err != nil {
		t.Fatal("pre-insert skip doc: " + err.Error())
	}

	selectiveFn := func(ctx context.Context, md string, meta harvest.SummaryMeta) error {
		if len(meta.SessionID) >= 4 && meta.SessionID[:4] == "fail" {
			return fmt.Errorf("simulated fail for %s", meta.SessionID)
		}
		return upsertMockSummary(ctx, pgDB, meta)
	}

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB).
		WithSummarizer(&stubSummarizer{fn: selectiveFn})

	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)

	// 3 success + 2 fallback = 5 harvested, 1 skipped, 0 errors
	if harvested != 5 {
		t.Errorf("harvested = %d, want 5 (3 success + 2 fallback)", harvested)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1", skipped)
	}
	if errCount != 0 {
		t.Errorf("errCount = %d, want 0", errCount)
	}

	ctx := context.Background()

	// Verify success sessions have collection="session-summary"
	for _, sid := range []string{"success-1", "success-2", "success-3"} {
		rows, err := pgDB.QueryContext(ctx,
			`SELECT collection FROM documents WHERE source_path = $1`,
			"summary://opencode/"+sid,
		)
		if err != nil {
			t.Fatalf("query %s: %v", sid, err)
		}
		var found bool
		for rows.Next() {
			var coll string
			if err := rows.Scan(&coll); err != nil {
				t.Fatal(err)
			}
			found = true
			if coll != "session-summary" {
				t.Errorf("%s collection = %q, want %q", sid, coll, "session-summary")
			}
		}
		rows.Close()
		if !found {
			t.Errorf("no doc found for %s", sid)
		}
	}

	// Verify fail sessions have collection="sessions" with metadata.fallback=true
	for _, sid := range []string{"fail-1", "fail-2"} {
		rows, err := pgDB.QueryContext(ctx,
			`SELECT collection, metadata FROM documents WHERE source_path = $1`,
			"summary://opencode/"+sid,
		)
		if err != nil {
			t.Fatalf("query %s: %v", sid, err)
		}
		var found bool
		for rows.Next() {
			var coll string
			var metaRaw []byte
			if err := rows.Scan(&coll, &metaRaw); err != nil {
				t.Fatal(err)
			}
			found = true
			if coll != "sessions" {
				t.Errorf("%s collection = %q, want %q", sid, coll, "sessions")
			}
			var meta map[string]any
			if err := json.Unmarshal(metaRaw, &meta); err != nil {
				t.Fatalf("%s metadata unmarshal: %v", sid, err)
			}
			if fb, ok := meta["fallback"]; !ok || fb != true {
				t.Errorf("%s metadata.fallback = %v, want true", sid, fb)
			}
		}
		rows.Close()
		if !found {
			t.Errorf("no doc found for %s", sid)
		}
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

type failingSummarizer struct{}

func (failingSummarizer) SummarizeAndPersist(_ context.Context, _ string, _ harvest.SummaryMeta) error {
	return fmt.Errorf("simulated LLM failure")
}

type stubSummarizer struct {
	fn func(ctx context.Context, md string, meta harvest.SummaryMeta) error
}

func (s *stubSummarizer) SummarizeAndPersist(ctx context.Context, md string, meta harvest.SummaryMeta) error {
	return s.fn(ctx, md, meta)
}

const testDSN = "postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable"

func setupTestPG(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping postgres-dependent test in -short mode")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(testDSN)
	if err != nil {
		t.Skip("postgres not available: parse config: " + err.Error())
	}

	schema := fmt.Sprintf("test_%x", sha256.Sum256([]byte(t.Name())))[:18]
	poolCfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("SET search_path TO %s, public", schema))
		return err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Skip("postgres not available: connect: " + err.Error())
	}

	_, _ = pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Skip("postgres not available: create schema: " + err.Error())
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		pool.Close()
		t.Fatal("goose dialect: " + err.Error())
	}
	goose.SetTableName(schema + "_goose_version")
	migrateDB := stdlib.OpenDBFromPool(pool)
	if err := goose.UpContext(ctx, migrateDB, "."); err != nil {
		migrateDB.Close()
		pool.Close()
		t.Fatal("goose migrate: " + err.Error())
	}
	migrateDB.Close()
	goose.SetTableName("goose_db_version")

	pgDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() {
		pgDB.Close()
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})

	return pgDB
}

func TestOpenCodeSQLite_LLMFailure_FallbackUnifiedPath(t *testing.T) {
	pgDB := setupTestPG(t)
	sqdb := setupTestSQLiteDB(t)

	now := time.Now()
	oldMs := now.Add(-15 * time.Minute).UnixMilli()

	insertTestProject(t, sqdb, "proj1", "/home/user/test-app")
	seedRegisteredWorkspace(t, pgDB, "/home/user/test-app")
	insertTestSession(t, sqdb, "sess-fb1", "proj1", "Fallback Session", oldMs)
	if _, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, oldMs, "sess-fb1"); err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg1", "sess-fb1", "user", oldMs)
	insertTestPart(t, sqdb, "p1", "msg1", "text", "Hello from fallback test!")

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB).
		WithSummarizer(failingSummarizer{})

	harvested, _, errCount := h.HarvestAll(context.Background(), nil)

	if harvested != 1 {
		t.Errorf("harvested = %d, want 1", harvested)
	}
	if errCount != 0 {
		t.Errorf("errCount = %d, want 0", errCount)
	}

	q := sqlc.New(pgDB)
	ctx := context.Background()

	var foundDoc bool
	rows, err := pgDB.QueryContext(ctx,
		`SELECT source_path, collection, metadata FROM documents WHERE source_path = $1`,
		"summary://opencode/sess-fb1",
	)
	if err != nil {
		t.Fatal("query fallback doc: " + err.Error())
	}
	defer rows.Close()
	for rows.Next() {
		var sp, coll string
		var metaRaw []byte
		if err := rows.Scan(&sp, &coll, &metaRaw); err != nil {
			t.Fatal("scan: " + err.Error())
		}
		foundDoc = true
		if coll != "sessions" {
			t.Errorf("collection = %q, want %q", coll, "sessions")
		}
		var meta map[string]any
		if err := json.Unmarshal(metaRaw, &meta); err != nil {
			t.Fatal("unmarshal metadata: " + err.Error())
		}
		fb, ok := meta["fallback"]
		if !ok || fb != true {
			t.Errorf("metadata.fallback = %v, want true", fb)
		}
	}
	if !foundDoc {
		t.Error("no doc found at summary://opencode/sess-fb1")
	}

	_, lookupErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "opencode://session/sess-fb1",
		WorkspaceHash: "",
	})
	if lookupErr == nil {
		t.Error("expected no doc at old path opencode://session/sess-fb1, but found one")
	}

	harvested2, skipped2, errCount2 := h.HarvestAll(context.Background(), nil)
	if harvested2 != 0 {
		t.Errorf("second harvest: harvested = %d, want 0 (should skip)", harvested2)
	}
	if skipped2 < 1 {
		t.Errorf("second harvest: skipped = %d, want >= 1", skipped2)
	}
	if errCount2 != 0 {
		t.Errorf("second harvest: errCount = %d, want 0", errCount2)
	}
}

func TestOpenCodeSQLite_OrphanSession_NoWorktree_Skipped(t *testing.T) {
	pgDB := setupTestPG(t)
	sqdb := setupTestSQLiteDB(t)

	now := time.Now()
	oldMs := now.Add(-15 * time.Minute).UnixMilli()

	if _, err := sqdb.Exec(
		`INSERT INTO session (id, project_id, title, time_created, time_updated) VALUES (?, ?, ?, ?, ?)`,
		"orphan-001", nil, "Orphan Session", oldMs, oldMs,
	); err != nil {
		t.Fatalf("insert orphan session: %v", err)
	}
	insertTestMessage(t, sqdb, "msg-orphan", "orphan-001", "user", oldMs)
	insertTestPart(t, sqdb, "p-orphan", "msg-orphan", "text", "orphan content")

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB)
	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)

	if harvested != 0 {
		t.Errorf("harvested = %d, want 0 (orphan session must be skipped)", harvested)
	}
	if skipped < 1 {
		t.Errorf("skipped = %d, want >= 1", skipped)
	}
	if errCount != 0 {
		t.Errorf("errCount = %d, want 0", errCount)
	}

	q := sqlc.New(pgDB)
	_, lookupErr := q.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/orphan-001",
		WorkspaceHash: "",
	})
	if lookupErr == nil {
		t.Error("orphan session document leaked to DB — leak #5 still open")
	}
}

func TestOpenCodeSQLite_UnregisteredWorktree_Skipped(t *testing.T) {
	pgDB := setupTestPG(t)
	sqdb := setupTestSQLiteDB(t)

	now := time.Now()
	oldMs := now.Add(-15 * time.Minute).UnixMilli()
	unregisteredWorktree := "/tmp/never-registered-via-init"

	insertTestProject(t, sqdb, "proj-unreg", unregisteredWorktree)
	insertTestSession(t, sqdb, "sess-unreg-001", "proj-unreg", "Unregistered Worktree Session", oldMs)
	if _, err := sqdb.Exec(`UPDATE session SET time_updated = ? WHERE id = ?`, oldMs, "sess-unreg-001"); err != nil {
		t.Fatal(err)
	}
	insertTestMessage(t, sqdb, "msg-unreg", "sess-unreg-001", "user", oldMs)
	insertTestPart(t, sqdb, "p-unreg", "msg-unreg", "text", "unregistered content")

	h := harvest.NewOpenCodeSQLiteHarvesterFromDB(sqdb, pgDB)
	harvested, skipped, errCount := h.HarvestAll(context.Background(), nil)

	if harvested != 0 {
		t.Errorf("harvested = %d, want 0 (unregistered worktree must be skipped)", harvested)
	}
	if skipped < 1 {
		t.Errorf("skipped = %d, want >= 1", skipped)
	}
	if errCount != 0 {
		t.Errorf("errCount = %d, want 0", errCount)
	}

	wsH := sha256.Sum256([]byte(unregisteredWorktree))
	wsHash := hex.EncodeToString(wsH[:])

	q := sqlc.New(pgDB)
	if _, err := q.GetWorkspaceByHash(context.Background(), wsHash); err == nil {
		t.Errorf("workspace was auto-registered for %q — auto-registration not removed (leak #5)", unregisteredWorktree)
	}

	_, lookupErr := q.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/sess-unreg-001",
		WorkspaceHash: wsHash,
	})
	if lookupErr == nil {
		t.Error("document persisted under unregistered workspace_hash — leak still open")
	}
}
