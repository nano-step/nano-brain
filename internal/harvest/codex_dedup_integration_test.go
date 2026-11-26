//go:build integration

package harvest

import (
	"context"
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

// countingCodexSummarizer mirrors countingPiSummarizer: records calls and
// writes a summary document at the codex source path so the next cycle dedups.
type countingCodexSummarizer struct {
	db    *sqlc.Queries
	calls atomic.Int64
}

func (c *countingCodexSummarizer) SummarizeAndPersist(ctx context.Context, content string, meta SummaryMeta) error {
	c.calls.Add(1)
	_, err := c.db.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: meta.WorkspaceHash,
		ContentHash:   "summary-hash-" + meta.SessionID,
		Title:         "Summary: " + meta.Title,
		Content:       "# summary\nstored",
		SourcePath:    codexSourcePath(meta.SessionID),
		Collection:    "sessions",
	})
	return err
}

func writeCodexRollout(t *testing.T, codexHome, rel, sessionID, cwd string) {
	t.Helper()
	lines := []string{
		codexMetaLine(sessionID, cwd, "2026-01-01T10:00:00Z"),
		codexItemLine("2026-01-01T10:00:01Z", "message", map[string]any{
			"type": "message", "role": "user",
			"content": []map[string]any{{"type": "input_text", "text": "hello codex"}},
		}),
		codexItemLine("2026-01-01T10:00:02Z", "message", map[string]any{
			"type": "message", "role": "assistant",
			"content": []map[string]any{{"type": "output_text", "text": "hi"}},
		}),
	}
	path := filepath.Join(codexHome, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestCodexHarvester_PresenceBasedSkip proves end-to-end that a rollout in a
// date-partitioned sessions/ tree is attributed to the workspace matching its
// session_meta cwd, summarized once, and skipped on the second cycle.
func TestCodexHarvester_PresenceBasedSkip(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	wsDir := t.TempDir()
	wsHash, err := storage.WorkspaceHash(wsDir)
	if err != nil {
		t.Fatalf("workspace hash: %v", err)
	}
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: wsHash, Path: wsDir, Name: "codex-dedup-test"}); err != nil {
		t.Fatalf("register workspace: %v", err)
	}

	codexHome := t.TempDir()
	writeCodexRollout(t, codexHome, filepath.Join("sessions", "2026", "01", "01", "rollout-2026-01-01T10-00-00-sess-dedup.jsonl"), "sess-dedup", wsDir)
	// Unregistered workspace session — must be skipped, not ingested.
	writeCodexRollout(t, codexHome, filepath.Join("sessions", "2026", "01", "01", "rollout-2026-01-01T11-00-00-sess-foreign.jsonl"), "sess-foreign", "/unregistered/dir")

	sum := &countingCodexSummarizer{db: q}
	h := NewCodexHarvester(db, zerolog.Nop(), codexHome).WithSummarizer(sum)

	harvested, skipped, errs := h.HarvestAll(ctx, nil)
	if errs != 0 {
		t.Fatalf("errors = %d", errs)
	}
	if harvested != 1 {
		t.Fatalf("harvested = %d, want 1 (foreign session skipped)", harvested)
	}
	if skipped != 1 {
		t.Fatalf("skipped = %d, want 1 (unregistered workspace)", skipped)
	}
	if got := sum.calls.Load(); got != 1 {
		t.Fatalf("summarizer calls = %d, want 1", got)
	}

	// Verify the document landed under the codex source path.
	doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    codexSourcePath("sess-dedup"),
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("expected document at summary://codex/sess-dedup: %v", err)
	}
	if doc.Collection != "sessions" {
		t.Fatalf("collection = %q", doc.Collection)
	}

	// Second cycle: presence dedup — no re-summarize.
	harvested2, skipped2, errs2 := h.HarvestAll(ctx, nil)
	if errs2 != 0 {
		t.Fatalf("second cycle errors = %d", errs2)
	}
	if harvested2 != 0 {
		t.Fatalf("second cycle harvested = %d, want 0", harvested2)
	}
	if skipped2 < 2 {
		t.Fatalf("second cycle skipped = %d, want >= 2", skipped2)
	}
	if got := sum.calls.Load(); got != 1 {
		t.Fatalf("summarizer calls after second cycle = %d, want 1", got)
	}
}

// TestCodexHarvester_RawFallbackNoSummarizer stores rendered markdown directly.
func TestCodexHarvester_RawFallbackNoSummarizer(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	wsDir := t.TempDir()
	wsHash, err := storage.WorkspaceHash(wsDir)
	if err != nil {
		t.Fatalf("workspace hash: %v", err)
	}
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: wsHash, Path: wsDir, Name: "codex-raw-test"}); err != nil {
		t.Fatalf("register workspace: %v", err)
	}

	codexHome := t.TempDir()
	writeCodexRollout(t, codexHome, filepath.Join("archived_sessions", "rollout-old-sess-raw.jsonl"), "sess-raw", wsDir)

	h := NewCodexHarvester(db, zerolog.Nop(), codexHome)
	harvested, _, errs := h.HarvestAll(ctx, nil)
	if errs != 0 || harvested != 1 {
		t.Fatalf("harvested=%d errs=%d, want 1/0", harvested, errs)
	}

	doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    codexSourcePath("sess-raw"),
		WorkspaceHash: wsHash,
	})
	if err != nil {
		t.Fatalf("raw doc missing: %v", err)
	}
	for _, want := range []string{"source: codex", "## user", "hello codex", "## assistant"} {
		if !strings.Contains(doc.Content, want) {
			t.Fatalf("raw doc missing %q:\n%s", want, doc.Content[:min(500, len(doc.Content))])
		}
	}
}
