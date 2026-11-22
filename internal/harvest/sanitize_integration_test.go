//go:build integration

package harvest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
	_ "modernc.org/sqlite"
)

// setupSanitizeDB returns a PG handle on an isolated schema plus a workspace
// registered for a fresh temp dir.
func setupSanitizeDB(t *testing.T) (db *sql.DB, q *sqlc.Queries, dir, wsHash string) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	db = stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q = sqlc.New(db)

	dir = t.TempDir()
	wsHash, err := storage.WorkspaceHash(dir)
	if err != nil {
		t.Fatalf("workspace hash: %v", err)
	}
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{Hash: wsHash, Path: dir, Name: "sanitize-test"}); err != nil {
		t.Fatalf("register workspace: %v", err)
	}
	return db, q, dir, wsHash
}

// persistingSummarizer stands in for internal/summarize: like the real
// persister it writes title, source path, tags and JSONB metadata straight
// from the SummaryMeta it is handed, so a dirty meta fails the upsert.
type persistingSummarizer struct {
	q     *sqlc.Queries
	calls int
	metas []SummaryMeta
}

func (p *persistingSummarizer) SummarizeAndPersist(ctx context.Context, _ string, meta SummaryMeta) error {
	p.calls++
	p.metas = append(p.metas, meta)
	metaBytes, _ := json.Marshal(map[string]any{
		"source": meta.Source, "session_id": meta.SessionID, "parent_id": meta.ParentID,
		"branch": meta.Branch, "cwd": meta.Cwd,
	})
	_, err := p.q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: meta.WorkspaceHash,
		ContentHash:   "summary-hash",
		Title:         "Summary: " + meta.Title,
		Content:       "# summary\n- Branch: " + meta.Branch + "\n- Cwd: " + meta.Cwd,
		SourcePath:    "summary://" + meta.Source + "/" + meta.SessionID,
		Collection:    "sessions",
		Tags:          append([]string{"summary", meta.Source}, meta.Tags...),
		Metadata:      pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true},
	})
	return err
}

// assertStorableDoc fetches a session document and checks every text column.
func assertStorableDoc(t *testing.T, q *sqlc.Queries, sourcePath, wsHash string) sqlc.DocumentView {
	t.Helper()
	doc, err := q.GetDocumentBySourcePath(context.Background(), sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("document %q was not persisted: %v", sourcePath, err)
	}
	fields := map[string]string{
		"title": doc.Title, "content": doc.Content, "source_path": doc.SourcePath, "metadata": string(doc.Metadata.RawMessage),
	}
	for i, tag := range doc.Tags {
		fields["tags["+itoa(i)+"]"] = tag
	}
	for field, v := range fields {
		if !storable(v) || strings.Contains(v, `\u0000`) {
			t.Errorf("stored %s is not clean: %q", field, v)
		}
	}
	return doc
}

// An OpenCode SQLite session.title is a plain TEXT column, not JSON, so it can
// hold a NUL or bytes that are not UTF-8. Postgres rejects both (SQLSTATE
// 22021); before #675 the upsert failed and the session was lost every cycle.
func TestOpenCodeSQLite_DirtySessionStringsArePersisted(t *testing.T) {
	sessions := []struct {
		name  string
		id    string
		title string
	}{
		{"title with NUL", "ses_title_nul", "before\x00after"},
		{"title with invalid UTF-8", "ses_title_utf8", invalidUTF8},
		{"id with NUL", "ses_id\x00_nul", "clean title"},
		{"clean", "ses_clean_1", "tiêu đề sạch"},
	}

	for _, mode := range []string{"raw fallback", "summarizer"} {
		t.Run(mode, func(t *testing.T) {
			db, q, dir, wsHash := setupSanitizeDB(t)
			ctx := context.Background()

			sqdb, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqdb.Close() })
			sqdb.SetMaxOpenConns(1) // one connection, or each would get its own :memory: DB
			if _, err := sqdb.Exec(`
				CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL);
				CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT, parent_id TEXT, title TEXT, time_created INTEGER, time_updated INTEGER);
				CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT, time_created INTEGER, data TEXT);
				CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT, session_id TEXT, time_created INTEGER, data TEXT);
			`); err != nil {
				t.Fatal(err)
			}
			oldMs := time.Now().Add(-15 * time.Minute).UnixMilli()
			mustExecSQLite(t, sqdb, `INSERT INTO project (id, worktree) VALUES (?, ?)`, "proj", dir)
			for i, s := range sessions {
				msgID := "msg_" + itoa(i)
				mustExecSQLite(t, sqdb, `INSERT INTO session (id, project_id, title, time_created, time_updated) VALUES (?, ?, ?, ?, ?)`,
					s.id, "proj", s.title, oldMs, oldMs)
				mustExecSQLite(t, sqdb, `INSERT INTO message (id, session_id, time_created, data) VALUES (?, ?, ?, ?)`,
					msgID, s.id, oldMs, `{"role":"user"}`)
				mustExecSQLite(t, sqdb, `INSERT INTO part (id, message_id, session_id, time_created, data) VALUES (?, ?, '', 0, ?)`,
					"part_"+itoa(i), msgID, `{"type":"text","text":"hello"}`)
			}

			// Guard: the fixture really carries the dirt into the harvester.
			var gotTitle string
			if err := sqdb.QueryRow(`SELECT title FROM session WHERE id = ?`, "ses_title_nul").Scan(&gotTitle); err != nil || gotTitle != "before\x00after" {
				t.Fatalf("SQLite fixture did not keep the NUL title: %q (err %v)", gotTitle, err)
			}

			h := NewOpenCodeSQLiteHarvesterFromDB(sqdb, db)
			sum := &persistingSummarizer{q: q}
			if mode == "summarizer" {
				h.WithSummarizer(sum)
			}

			harvested, skipped, errCount := h.HarvestAll(ctx, nil)
			if harvested != len(sessions) || skipped != 0 || errCount != 0 {
				t.Fatalf("first cycle: harvested=%d skipped=%d errCount=%d, want %d/0/0", harvested, skipped, errCount, len(sessions))
			}

			for _, s := range sessions {
				doc := assertStorableDoc(t, q, "summary://opencode/"+sanitizeID(s.id), wsHash)
				if !strings.Contains(doc.Title, sanitizeText(s.title)) {
					t.Errorf("%s: stored title %q lost the clean part of %q", s.name, doc.Title, s.title)
				}
			}

			// Clean data is stored under exactly the path and title it had before.
			clean := assertStorableDoc(t, q, "summary://opencode/ses_clean_1", wsHash)
			if wantTitle := map[string]string{"raw fallback": "tiêu đề sạch", "summarizer": "Summary: tiêu đề sạch"}[mode]; clean.Title != wantTitle {
				t.Errorf("clean session title = %q, want %q", clean.Title, wantTitle)
			}

			// Dedup: the lookup uses the same sanitized source path as the write.
			harvested, skipped, errCount = h.HarvestAll(ctx, nil)
			if harvested != 0 || skipped != len(sessions) || errCount != 0 {
				t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/%d/0", harvested, skipped, errCount, len(sessions))
			}
			if mode == "summarizer" && sum.calls != len(sessions) {
				t.Errorf("summarizer ran %d times over two cycles, want %d", sum.calls, len(sessions))
			}
		})
	}
}

func mustExecSQLite(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("sqlite exec: %v", err)
	}
}

// A NUL in the Pi header id reaches source_path, title and the JSONB metadata
// session_id. The session must be persisted once and then skipped: a lookup on
// the raw id would never find the sanitized row and re-harvest it forever.
func TestPiHarvester_NULInHeaderID(t *testing.T) {
	for _, mode := range []string{"raw fallback", "summarizer"} {
		t.Run(mode, func(t *testing.T) {
			db, q, dir, wsHash := setupSanitizeDB(t)
			ctx := context.Background()

			header, _ := json.Marshal(map[string]any{
				"type": "session", "version": 3, "id": "pi-ses\x00-dirty", "timestamp": "2026-01-01T10:00:00Z", "cwd": dir,
			})
			if !strings.Contains(string(header), `\u0000`) {
				t.Fatalf("fixture header does not carry \\u0000: %s", header)
			}
			msg, _ := json.Marshal(map[string]any{
				"type": "message", "timestamp": "2026-01-01T10:00:01Z",
				"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "text", "text": "hello"}}},
			})
			content := append(append(append(header, '\n'), msg...), '\n')
			if err := os.WriteFile(filepath.Join(dir, "2026-01-01T10-00-00_pi-dirty.jsonl"), content, 0o644); err != nil {
				t.Fatal(err)
			}

			h := NewPiHarvester(db, zerolog.Nop(), dir, wsHash)
			sum := &persistingSummarizer{q: q}
			if mode == "summarizer" {
				h.WithSummarizer(sum)
			}

			harvested, skipped, errCount := h.HarvestAll(ctx, nil)
			if harvested != 1 || skipped != 0 || errCount != 0 {
				t.Fatalf("first cycle: harvested=%d skipped=%d errCount=%d, want 1/0/0", harvested, skipped, errCount)
			}
			assertStorableDoc(t, q, piSourcePath(sanitizeID("pi-ses\x00-dirty")), wsHash)

			harvested, skipped, errCount = h.HarvestAll(ctx, nil)
			if harvested != 0 || skipped != 1 || errCount != 0 {
				t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/1/0 (dedup)", harvested, skipped, errCount)
			}
			if mode == "summarizer" && sum.calls != 1 {
				t.Errorf("summarizer ran %d times over two cycles, want 1", sum.calls)
			}
		})
	}
}

// fixedSource is a SessionSource that returns the sessions it was given.
type fixedSource struct {
	wsHash   string
	sessions []NormalizedSession
}

func (s fixedSource) Name() string { return "opencode" }

func (s fixedSource) Discover(context.Context, map[string]string) ([]Location, error) {
	return []Location{{WorkspaceHash: s.wsHash}}, nil
}

func (s fixedSource) Read(context.Context, Location) ([]NormalizedSession, error) {
	return s.sessions, nil
}

// On the Engine path the header fields (session id, parent id, branch, cwd,
// title, role, tool name) are rendered outside the sanitized message bodies
// and copied into SummaryMeta.
func TestEngine_DirtyHeaderFieldsArePersisted(t *testing.T) {
	for _, dirt := range []struct{ name, s string }{{"nul", "x\x00y"}, {"invalid utf8", invalidUTF8}} {
		for _, mode := range []string{"raw fallback", "summarizer"} {
			t.Run(dirt.name+"/"+mode, func(t *testing.T) {
				db, q, _, wsHash := setupSanitizeDB(t)
				ctx := context.Background()
				d := dirt.s

				src := fixedSource{wsHash: wsHash, sessions: []NormalizedSession{{
					Source: "opencode", SessionID: "ses" + d, ParentID: "par" + d, WorkspaceHash: wsHash,
					Branch: "feat/DEV-1234" + d, Cwd: "/cwd" + d, Title: "title" + d,
					CreatedAt: time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
					Messages: []NormalizedMessage{
						{Role: "user" + d, Content: "hello"},
						{Role: "assistant", ToolName: "bash" + d, Content: "ls"},
					},
				}}}

				sum := &persistingSummarizer{q: q}
				var summarizer SessionSummarizer
				if mode == "summarizer" {
					summarizer = sum
				}
				e := NewEngine(src, db, summarizer, zerolog.Nop())

				harvested, skipped, errCount := e.HarvestAll(ctx, nil)
				if harvested != 1 || skipped != 0 || errCount != 0 {
					t.Fatalf("first cycle: harvested=%d skipped=%d errCount=%d, want 1/0/0", harvested, skipped, errCount)
				}
				doc := assertStorableDoc(t, q, "summary://opencode/"+sanitizeID("ses"+d), wsHash)
				if mode == "raw fallback" {
					contentSum := sha256.Sum256([]byte(doc.Content))
					if got := hex.EncodeToString(contentSum[:]); got != doc.ContentHash {
						t.Errorf("content hash %s is not the hash of the stored content (%s)", doc.ContentHash, got)
					}
				}
				for _, m := range sum.metas {
					for field, v := range map[string]string{
						"SessionID": m.SessionID, "ParentID": m.ParentID, "Title": m.Title, "Branch": m.Branch, "Cwd": m.Cwd,
					} {
						if !storable(v) {
							t.Errorf("SummaryMeta.%s = %q is not storable", field, v)
						}
					}
				}

				harvested, skipped, errCount = e.HarvestAll(ctx, nil)
				if harvested != 0 || skipped != 1 || errCount != 0 {
					t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/1/0 (dedup)", harvested, skipped, errCount)
				}
			})
		}
	}
}

// The boundary pass must not change anything for a clean Claude Code session:
// same source path, same title, and a content hash equal to the hash of the
// renderer's output, which is how the hash was computed before #675.
func TestClaudeHarvester_CleanSessionStoredUnchanged(t *testing.T) {
	db, q, dir, wsHash := setupSanitizeDB(t)
	ctx := context.Background()

	lines := ""
	for _, m := range []map[string]any{
		{"type": "user", "timestamp": "2026-01-01T10:00:00Z", "message": map[string]any{"role": "user", "content": "xin chào — hello"}},
		{"type": "assistant", "timestamp": "2026-01-01T10:00:01Z", "message": map[string]any{"role": "assistant", "content": []map[string]any{{"type": "text", "text": "done 🙂"}}}},
	} {
		line, _ := json.Marshal(m)
		lines += string(line) + "\n"
	}
	file := filepath.Join(dir, "ses_clean.jsonl")
	if err := os.WriteFile(file, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewClaudeCodeHarvester(db, zerolog.Nop(), dir, wsHash)
	if harvested, _, errCount := h.HarvestAll(ctx, nil); harvested != 1 || errCount != 0 {
		t.Fatalf("HarvestAll: harvested=%d errCount=%d, want 1/0", harvested, errCount)
	}

	msgs, err := parseJSONLFile(file)
	if err != nil {
		t.Fatal(err)
	}
	wantContent := renderClaudeCodeMarkdown("ses_clean", msgs)
	wantSum := sha256.Sum256([]byte(wantContent))

	doc := assertStorableDoc(t, q, "summary://claude/ses_clean", wsHash)
	if doc.ContentHash != hex.EncodeToString(wantSum[:]) {
		t.Errorf("content hash = %s, want the hash of the rendered markdown %s", doc.ContentHash, hex.EncodeToString(wantSum[:]))
	}
	if doc.Content != wantContent {
		t.Errorf("stored content differs from the rendered markdown")
	}
	if doc.Title != "Claude Code Session ses_clean" {
		t.Errorf("title = %q", doc.Title)
	}
	if want := []string{"claude_code", "session", "fallback"}; strings.Join(doc.Tags, ",") != strings.Join(want, ",") {
		t.Errorf("tags = %v, want %v", doc.Tags, want)
	}
	var meta map[string]any
	if err := json.Unmarshal(doc.Metadata.RawMessage, &meta); err != nil || meta["session_id"] != "ses_clean" {
		t.Errorf("metadata = %s (err %v), want session_id ses_clean", doc.Metadata.RawMessage, err)
	}
}

// AutoMemoryExtractor keys its documents on the session id and stores text cut
// out of the session content.
func TestAutoMemory_DirtySessionIDAndContent(t *testing.T) {
	db, q, _, wsHash := setupSanitizeDB(t)
	ctx := context.Background()

	e := NewAutoMemoryExtractor(db, wsHash, zerolog.Nop())
	const content = "Decision: Use Postgres\x00 for storage " + invalidUTF8 + "\n"
	if stored := e.ExtractAndStore(ctx, "ses\x00-am", content, nil); stored != 1 {
		t.Fatalf("first run stored %d memories, want 1", stored)
	}
	if stored := e.ExtractAndStore(ctx, "ses\x00-am", content, nil); stored != 0 {
		t.Errorf("second run stored %d memories, want 0 (dedup)", stored)
	}

	memory := "Use Postgres for storage a\uFFFDb"
	assertStorableDoc(t, q, "automemory://"+sanitizeID("ses\x00-am")+"/decision/"+shortHash(memory), wsHash)
}

// The write functions are the persistence boundary: whatever a caller hands
// them, the title, source path, tags and JSONB metadata they store must be
// accepted by Postgres. Called directly so each one is covered on its own,
// independent of the sanitizing its harvester does upstream.
func TestRawFallbackWriters_SanitizeAtTheBoundary(t *testing.T) {
	const md = "---\nsession_id: s\n---\n\n## human ()\n\nhello\n"
	mdSum := sha256.Sum256([]byte(md))
	mdHash := hex.EncodeToString(mdSum[:])

	for _, dirt := range []struct{ name, s string }{{"nul", "x\x00y"}, {"invalid utf8", invalidUTF8}} {
		d := dirt.s
		sourcePath := "summary://test/ses" + d
		writers := []struct {
			name  string
			write func(db *sql.DB, wsHash string) error
		}{
			{"persistRawFallback", func(db *sql.DB, wsHash string) error {
				_, err := persistRawFallback(context.Background(), db, wsHash, sourcePath, "title"+d, md, mdHash,
					[]string{"pi", "tag" + d}, map[string]any{"source": "pi", "session_id": "ses" + d, "message_count": 1}, nil)
				return err
			}},
			{"claude", func(db *sql.DB, wsHash string) error {
				h := NewClaudeCodeHarvester(db, zerolog.Nop(), t.TempDir(), wsHash)
				return h.writeRawFallback(context.Background(), "ses"+d, md, mdHash, "title"+d, sourcePath, 1, nil)
			}},
			{"opencode sqlite", func(db *sql.DB, wsHash string) error {
				h := NewOpenCodeSQLiteHarvesterFromDB(nil, db)
				return h.writeRawFallback(context.Background(), SqSession{ID: "ses" + d}, md, wsHash, "title"+d, sourcePath, 1, nil)
			}},
			{"engine", func(db *sql.DB, wsHash string) error {
				e := NewEngine(fixedSource{}, db, nil, zerolog.Nop())
				return e.writeRawFallback(context.Background(), NormalizedSession{SessionID: "ses" + d, Title: "title" + d},
					md, mdHash, wsHash, sourcePath, []string{"ticket:DEV-1" + d}, nil)
			}},
			{"git commit", func(db *sql.DB, wsHash string) error {
				h := &GitHarvester{db: db, queries: sqlc.New(db), logger: zerolog.Nop()}
				return h.upsertCommit(context.Background(), wsHash,
					gitCommit{SHA: "abc1234", Subject: "title" + d, Author: "au" + d},
					md, mdHash, sourcePath, nil)
			}},
		}
		for _, w := range writers {
			t.Run(dirt.name+"/"+w.name, func(t *testing.T) {
				db, q, _, wsHash := setupSanitizeDB(t)
				if err := w.write(db, wsHash); err != nil {
					t.Fatalf("write failed: %v", err)
				}
				doc := assertStorableDoc(t, q, sanitizeText(sourcePath), wsHash)
				if doc.Content != md || doc.ContentHash != mdHash {
					t.Errorf("clean content or its hash changed: hash %s, want %s", doc.ContentHash, mdHash)
				}
			})
		}
	}
}

// A Claude Code session id is its file name, which need not be valid UTF-8.
// The file is opened by its raw name; the stored id is the sanitized one.
func TestClaudeHarvester_InvalidUTF8FileName(t *testing.T) {
	db, q, dir, wsHash := setupSanitizeDB(t)
	ctx := context.Background()

	line, _ := json.Marshal(map[string]any{
		"type": "user", "timestamp": "2026-01-01T10:00:00Z",
		"message": map[string]any{"role": "user", "content": "hello"},
	})
	if err := os.WriteFile(filepath.Join(dir, "ses_"+invalidUTF8+".jsonl"), append(line, '\n'), 0o644); err != nil {
		t.Skipf("file system refuses a file name that is not UTF-8: %v", err)
	}

	h := NewClaudeCodeHarvester(db, zerolog.Nop(), dir, wsHash)
	if harvested, _, errCount := h.HarvestAll(ctx, nil); harvested != 1 || errCount != 0 {
		t.Fatalf("first cycle: harvested=%d errCount=%d, want 1/0", harvested, errCount)
	}
	assertStorableDoc(t, q, "summary://claude/"+sanitizeID("ses_"+invalidUTF8), wsHash)
	if harvested, skipped, errCount := h.HarvestAll(ctx, nil); harvested != 0 || skipped != 1 || errCount != 0 {
		t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/1/0 (dedup)", harvested, skipped, errCount)
	}
}

// Legacy OpenCode JSON sessions: the title comes from a JSON string and is
// stored as the document title.
func TestOpenCodeJSON_NULInTitle(t *testing.T) {
	db, q, dir, wsHash := setupSanitizeDB(t)
	ctx := context.Background()

	sessionFile := filepath.Join(dir, "session", "proj", "ses_json.json")
	writeJSON(t, sessionFile, map[string]any{
		"id": "ses_json", "title": "before\x00after", "projectID": "proj", "directory": dir,
		"time": map[string]any{"created": 1700000000000, "updated": 1700000100000},
	})
	writeJSON(t, filepath.Join(dir, "message", "ses_json", "msg_1.json"), map[string]any{
		"id": "msg_1", "sessionID": "ses_json", "role": "user", "time": map[string]any{"created": 1700000000000},
	})
	writeJSON(t, filepath.Join(dir, "part", "msg_1", "prt_1.json"), map[string]any{
		"id": "prt_1", "type": "text", "text": "hello", "messageID": "msg_1",
	})

	h := NewOpenCodeHarvester(db, zerolog.Nop(), dir, wsHash)
	if harvested, _, errCount := h.HarvestAll(ctx, nil); harvested != 1 || errCount != 0 {
		t.Fatalf("first cycle: harvested=%d errCount=%d, want 1/0", harvested, errCount)
	}
	// A clean file path is stored as is.
	doc := assertStorableDoc(t, q, sessionFile, wsHash)
	if doc.Title != "beforeafter" {
		t.Errorf("title = %q, want %q", doc.Title, "beforeafter")
	}
	if harvested, skipped, errCount := h.HarvestAll(ctx, nil); harvested != 0 || skipped != 1 || errCount != 0 {
		t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/1/0 (dedup)", harvested, skipped, errCount)
	}
}

// The OpenCode JSON harvester builds the stored source path from the session
// file path; a file name that is not valid UTF-8 must be sanitized (and
// disambiguated by sanitizeID) before it reaches Postgres or the write fails
// with SQLSTATE 22021 on every cycle (#675).
func TestOpenCodeJSON_InvalidUTF8SourcePath(t *testing.T) {
	db, q, dir, wsHash := setupSanitizeDB(t)
	ctx := context.Background()

	sessionFile := filepath.Join(dir, "session", "proj", "ses_"+invalidUTF8+".json")
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionFile, []byte("{}"), 0o644); err != nil {
		t.Skipf("file system refuses a file name that is not UTF-8: %v", err)
	}
	writeJSON(t, sessionFile, map[string]any{
		"id": "ses_json_utf8", "title": "title", "projectID": "proj", "directory": dir,
		"time": map[string]any{"created": 1700000000000, "updated": 1700000100000},
	})
	writeJSON(t, filepath.Join(dir, "message", "ses_json_utf8", "msg_1.json"), map[string]any{
		"id": "msg_1", "sessionID": "ses_json_utf8", "role": "user", "time": map[string]any{"created": 1700000000000},
	})
	writeJSON(t, filepath.Join(dir, "part", "msg_1", "prt_1.json"), map[string]any{
		"id": "prt_1", "type": "text", "text": "hello", "messageID": "msg_1",
	})

	h := NewOpenCodeHarvester(db, zerolog.Nop(), dir, wsHash)
	if harvested, _, errCount := h.HarvestAll(ctx, nil); harvested != 1 || errCount != 0 {
		t.Fatalf("first cycle: harvested=%d errCount=%d, want 1/0 (source path must be storable)", harvested, errCount)
	}
	assertStorableDoc(t, q, sanitizeID(sessionFile), wsHash)
	if harvested, skipped, errCount := h.HarvestAll(ctx, nil); harvested != 0 || skipped != 1 || errCount != 0 {
		t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/1/0 (dedup)", harvested, skipped, errCount)
	}
}

// The OpenCode JSON source path is the session FILE path, and file names may
// carry invalid UTF-8. Two names that sanitizeText maps to one string
// ("ses_\xffz" and "ses_\xfez" both become "ses_\ufffdz") must still produce
// two documents: a bare sanitize would let whichever file is read last
// overwrite the other one's document (#675). sanitizeID appends ~<sha8 of
// the raw name> to dirty paths, keeping the mapping injective.
func TestOpenCodeJSON_SanitizedSourcePathCollisionKeepsBothSessions(t *testing.T) {
	db, q, dir, wsHash := setupSanitizeDB(t)
	ctx := context.Background()

	files := []struct{ name, id, text string }{
		{"ses_\xffz.json", "ses_oc_a", "TEXT-FROM-DIRTY-A"},
		{"ses_\xfez.json", "ses_oc_b", "TEXT-FROM-DIRTY-B"},
	}
	paths := make([]string, len(files))
	for i, s := range files {
		paths[i] = filepath.Join(dir, "session", "proj", s.name)
	}
	if err := os.MkdirAll(filepath.Dir(paths[0]), 0o755); err != nil {
		t.Fatal(err)
	}
	for i, s := range files {
		if err := os.WriteFile(paths[i], []byte("{}"), 0o644); err != nil {
			t.Skipf("file system refuses a file name that is not UTF-8: %v", err)
		}
		writeJSON(t, paths[i], map[string]any{
			"id": s.id, "title": s.id, "projectID": "proj", "directory": dir,
			"time": map[string]any{"created": 1700000000000, "updated": 1700000100000},
		})
		msgID := "msg_" + s.id
		writeJSON(t, filepath.Join(dir, "message", s.id, msgID+".json"), map[string]any{
			"id": msgID, "sessionID": s.id, "role": "user", "time": map[string]any{"created": 1700000000000},
		})
		writeJSON(t, filepath.Join(dir, "part", msgID, "prt_"+s.id+".json"), map[string]any{
			"id": "prt_" + s.id, "type": "text", "text": s.text, "messageID": msgID,
		})
	}

	h := NewOpenCodeHarvester(db, zerolog.Nop(), dir, wsHash)
	if harvested, _, errCount := h.HarvestAll(ctx, nil); harvested != 2 || errCount != 0 {
		t.Fatalf("first cycle: harvested=%d errCount=%d, want 2/0", harvested, errCount)
	}

	docA := assertStorableDoc(t, q, sanitizeID(paths[0]), wsHash)
	docB := assertStorableDoc(t, q, sanitizeID(paths[1]), wsHash)
	if docA.SourcePath == docB.SourcePath {
		t.Fatalf("colliding file names mapped to one source path %q", docA.SourcePath)
	}
	if !strings.Contains(docA.Content, "TEXT-FROM-DIRTY-A") {
		t.Errorf("dirty-name doc A holds the wrong session's content")
	}
	if !strings.Contains(docB.Content, "TEXT-FROM-DIRTY-B") {
		t.Errorf("dirty-name doc B was shadowed by the colliding file")
	}

	if harvested, skipped, errCount := h.HarvestAll(ctx, nil); harvested != 0 || skipped != 2 || errCount != 0 {
		t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/2/0 (dedup)", harvested, skipped, errCount)
	}
}
