//go:build integration

package sqlc_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

func TestDeleteStaleRawOpenCodeDocs(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { db.Close() })

	q := sqlc.New(db)
	ctx := context.Background()

	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: "ws-cleanup-test",
		Name: "cleanup-test",
		Path: "/tmp/cleanup-test",
	}); err != nil {
		t.Fatalf("upsert workspace: %v", err)
	}

	raw := []struct {
		sourcePath, contentHash, title string
	}{
		{"opencode://session/sess-a", "ch-raw-a", "Raw A"},
		{"opencode://session/sess-b", "ch-raw-b", "Raw B"},
		{"opencode://session/sess-c", "ch-raw-c", "Raw C"},
		{"opencode://session/sess-d", "ch-raw-d", "Raw D (no summary)"},
		{"opencode://session/sess-e", "ch-raw-e", "Raw E (no summary)"},
	}
	for _, r := range raw {
		if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: "ws-cleanup-test",
			ContentHash:   r.contentHash,
			Title:         r.title,
			Content:       "raw content for " + r.sourcePath,
			SourcePath:    r.sourcePath,
			Collection:    "sessions",
		}); err != nil {
			t.Fatalf("upsert raw %s: %v", r.sourcePath, err)
		}
	}

	summaries := []struct {
		sourcePath, contentHash, title string
	}{
		{"summary://opencode/sess-a", "ch-sum-a", "Summary A"},
		{"summary://opencode/sess-b", "ch-sum-b", "Summary B"},
		{"summary://opencode/sess-c", "ch-sum-c", "Summary C"},
	}
	for _, s := range summaries {
		if _, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: "ws-cleanup-test",
			ContentHash:   s.contentHash,
			Title:         s.title,
			Content:       "summary content for " + s.sourcePath,
			SourcePath:    s.sourcePath,
			Collection:    "session-summary",
		}); err != nil {
			t.Fatalf("upsert summary %s: %v", s.sourcePath, err)
		}
	}

	count, err := q.CountStaleRawOpenCodeDocs(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("CountStaleRawOpenCodeDocs = %d, want 3 (a,b,c have summaries; d,e do not)", count)
	}

	n, err := q.DeleteStaleRawOpenCodeDocs(ctx)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if n != 3 {
		t.Errorf("DeleteStaleRawOpenCodeDocs returned %d, want 3", n)
	}

	after, err := q.CountStaleRawOpenCodeDocs(ctx)
	if err != nil {
		t.Fatalf("count after: %v", err)
	}
	if after != 0 {
		t.Errorf("idempotent re-run count = %d, want 0", after)
	}

	n2, err := q.DeleteStaleRawOpenCodeDocs(ctx)
	if err != nil {
		t.Fatalf("delete again: %v", err)
	}
	if n2 != 0 {
		t.Errorf("idempotent re-run deleted = %d, want 0", n2)
	}

	for _, sp := range []string{"opencode://session/sess-d", "opencode://session/sess-e"} {
		_, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sp,
			WorkspaceHash: "ws-cleanup-test",
		})
		if err != nil {
			t.Errorf("expected unmatched raw %s to survive, got err: %v", sp, err)
		}
	}

	for _, sp := range []string{"opencode://session/sess-a", "opencode://session/sess-b", "opencode://session/sess-c"} {
		_, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sp,
			WorkspaceHash: "ws-cleanup-test",
		})
		if err == nil {
			t.Errorf("expected raw %s to be deleted, but it still exists", sp)
		}
	}
}
