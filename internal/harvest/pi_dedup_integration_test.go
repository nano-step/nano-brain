//go:build integration

package harvest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// countingPiSummarizer records how many times SummarizeAndPersist runs and,
// like the real persister, writes a summary document so the next harvest
// cycle can detect it and skip (presence-based dedup).
type countingPiSummarizer struct {
	db    *sqlc.Queries
	calls atomic.Int64
}

func (c *countingPiSummarizer) SummarizeAndPersist(ctx context.Context, content string, meta SummaryMeta) error {
	c.calls.Add(1)
	_, err := c.db.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: meta.WorkspaceHash,
		ContentHash:   "summary-hash-" + meta.SessionID,
		Title:         "Summary: " + meta.Title,
		Content:       "# summary\nstored",
		SourcePath:    piSourcePath(meta.SessionID),
		Collection:    "sessions",
	})
	return err
}

// TestPiHarvester_PresenceBasedSkip proves that once a Pi session has been
// summarized, a second harvest cycle skips it (does NOT re-run the
// summarizer) — mirrors TestClaudeHarvester_PresenceBasedSkip.
func TestPiHarvester_PresenceBasedSkip(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	dir := t.TempDir()
	wsHash, err := storage.WorkspaceHash(dir)
	if err != nil {
		t.Fatalf("workspace hash: %v", err)
	}
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: wsHash, Path: dir, Name: "pi-dedup-test"}); err != nil {
		t.Fatalf("register workspace: %v", err)
	}

	header, _ := json.Marshal(map[string]any{
		"type": "session", "version": 3, "id": "pi-ses-dedup", "timestamp": "2026-01-01T10:00:00Z", "cwd": dir,
	})
	msg, _ := json.Marshal(map[string]any{
		"type": "message", "timestamp": "2026-01-01T10:00:01Z",
		"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "text", "text": "hello"}}},
	})
	content := append(header, '\n')
	content = append(content, msg...)
	content = append(content, '\n')
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01T10-00-00_pi-ses-dedup.jsonl"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	sum := &countingPiSummarizer{db: q}
	h := NewPiHarvester(db, zerolog.Nop(), dir, wsHash).WithSummarizer(sum)

	// First cycle: no existing doc → summarize once.
	h.HarvestAll(ctx, nil)
	if got := sum.calls.Load(); got != 1 {
		t.Fatalf("first cycle: summarizer calls = %d, want 1", got)
	}

	// Second cycle: doc now exists → presence-based skip, summarizer NOT called again.
	_, skipped, _ := h.HarvestAll(ctx, nil)
	if got := sum.calls.Load(); got != 1 {
		t.Errorf("second cycle re-summarized (calls=%d) — presence-based skip not working", got)
	}
	if skipped < 1 {
		t.Errorf("second cycle skipped=%d, want >=1", skipped)
	}
}

// TestPiHarvester_NULByteInToolResult proves a session whose tool output
// contains a NUL character is still persisted: Postgres TEXT rejects 0x00, so
// before #648 the upsert failed and the whole session was lost on every cycle.
func TestPiHarvester_NULByteInToolResult(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	dir := t.TempDir()
	wsHash, err := storage.WorkspaceHash(dir)
	if err != nil {
		t.Fatalf("workspace hash: %v", err)
	}
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: wsHash, Path: dir, Name: "pi-nul-test"}); err != nil {
		t.Fatalf("register workspace: %v", err)
	}

	header, _ := json.Marshal(map[string]any{
		"type": "session", "version": 3, "id": "pi-ses-nul", "timestamp": "2026-01-01T10:00:00Z", "cwd": dir,
	})
	msg, _ := json.Marshal(map[string]any{
		"type": "message", "timestamp": "2026-01-01T10:00:01Z",
		"message": map[string]any{"role": "toolResult", "content": []map[string]any{{"type": "text", "text": "binary\x00output"}}},
	})
	content := append(header, '\n')
	content = append(content, msg...)
	content = append(content, '\n')
	if err := os.WriteFile(filepath.Join(dir, "2026-01-01T10-00-00_pi-ses-nul.jsonl"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewPiHarvester(db, zerolog.Nop(), dir, wsHash)
	if _, _, errCount := h.HarvestAll(ctx, nil); errCount != 0 {
		t.Fatalf("HarvestAll errCount = %d, want 0", errCount)
	}

	doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    piSourcePath("pi-ses-nul"),
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("session was not persisted: %v", err)
	}
	if !strings.Contains(doc.Content, "binaryoutput") {
		t.Errorf("persisted content lost the text around the NUL byte: %q", doc.Content)
	}
}

// Two raw ids that sanitizeText maps to one value ("pi\x00x" and "pix" both
// become "pix") must still produce two documents: a bare sanitize would let
// whichever session is read first shadow the other one forever (#675).
// sanitizeID appends ~<sha8 of the raw id> to dirty ids, keeping the mapping
// injective.
func TestPiHarvester_SanitizedIDCollisionKeepsBothSessions(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	dir := t.TempDir()
	wsHash, err := storage.WorkspaceHash(dir)
	if err != nil {
		t.Fatalf("workspace hash: %v", err)
	}
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: wsHash, Path: dir, Name: "pi-collision-test"}); err != nil {
		t.Fatalf("register workspace: %v", err)
	}

	for _, s := range []struct{ file, id, text string }{
		{"2026-01-01T10-00-00_a.jsonl", "pi\x00x", "SECRET-FROM-A"},
		{"2026-01-01T10-00-00_b.jsonl", "pix", "LEGIT-FROM-B"},
	} {
		header, _ := json.Marshal(map[string]any{
			"type": "session", "version": 3, "id": s.id, "timestamp": "2026-01-01T10:00:00Z", "cwd": dir,
		})
		msg, _ := json.Marshal(map[string]any{
			"type": "message", "timestamp": "2026-01-01T10:00:01Z",
			"message": map[string]any{"role": "user", "content": []map[string]any{{"type": "text", "text": s.text}}},
		})
		content := append(append(append(header, '\n'), msg...), '\n')
		if err := os.WriteFile(filepath.Join(dir, s.file), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := NewPiHarvester(db, zerolog.Nop(), dir, wsHash)
	if harvested, _, errCount := h.HarvestAll(ctx, nil); harvested != 2 || errCount != 0 {
		t.Fatalf("first cycle: harvested=%d errCount=%d, want 2/0", harvested, errCount)
	}

	// Both documents exist and carry their own session's text.
	docA, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath: piSourcePath(sanitizeID("pi\x00x")), WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("dirty-id document was not persisted: %v", err)
	}
	docB, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath: piSourcePath(sanitizeID("pix")), WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("clean-id document was not persisted: %v", err)
	}
	if docA.SourcePath == docB.SourcePath {
		t.Fatalf("colliding ids mapped to one source path %q", docA.SourcePath)
	}
	if !strings.Contains(docA.Content, "SECRET-FROM-A") {
		t.Errorf("dirty-id doc holds wrong content: %q", docA.Title)
	}
	if !strings.Contains(docB.Content, "LEGIT-FROM-B") {
		t.Errorf("clean-id doc was shadowed: %q", docB.Title)
	}

	if harvested, skipped, errCount := h.HarvestAll(ctx, nil); harvested != 0 || skipped != 2 || errCount != 0 {
		t.Errorf("second cycle: harvested=%d skipped=%d errCount=%d, want 0/2/0 (dedup)", harvested, skipped, errCount)
	}
}
