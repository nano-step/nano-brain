//go:build integration

package harvest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
