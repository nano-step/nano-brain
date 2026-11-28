//go:build integration

package harvest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	"github.com/rs/zerolog"
)

// TestClaudeDiscoveryCollisionAttribution proves that an encoded-leaf
// collision does not copy ambiguous cwd-less transcripts into every colliding
// workspace, while modern transcripts with cwd still reach the matching one.
func TestClaudeDiscoveryCollisionAttribution(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	q := sqlc.New(db)
	ctx := context.Background()

	root := t.TempDir()
	wsAPath := filepath.Join(root, "work space")
	wsBPath := filepath.Join(root, "work-space")
	for _, path := range []string{wsAPath, wsBPath} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	encodedA := EncodeClaudeProjectsDir(wsAPath)
	encodedB := EncodeClaudeProjectsDir(wsBPath)
	if encodedA != encodedB {
		t.Fatalf("test workspace paths do not encode to the same leaf: %q vs %q", encodedA, encodedB)
	}

	baseDir := filepath.Join(root, "projects")
	leafDir := filepath.Join(baseDir, encodedA)
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}

	wsAHash, err := storage.WorkspaceHash(wsAPath)
	if err != nil {
		t.Fatalf("workspace A hash: %v", err)
	}
	wsBHash, err := storage.WorkspaceHash(wsBPath)
	if err != nil {
		t.Fatalf("workspace B hash: %v", err)
	}
	for _, ws := range []struct {
		hash string
		path string
		name string
	}{
		{wsAHash, wsAPath, "collision-a"},
		{wsBHash, wsBPath, "collision-b"},
	} {
		if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{Hash: ws.hash, Path: ws.path, Name: ws.name}); err != nil {
			t.Fatalf("register %s: %v", ws.name, err)
		}
	}

	writeTranscript := func(name, cwd string) {
		t.Helper()
		record := map[string]any{
			"type":      "user",
			"timestamp": "2026-01-01T10:00:00Z",
			"message":   map[string]any{"role": "user", "content": name},
		}
		if cwd != "" {
			record["cwd"] = cwd
		}
		line, _ := json.Marshal(record)
		if err := os.WriteFile(filepath.Join(leafDir, name+".jsonl"), append(line, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	assertDocumentAbsent := func(workspaceHash, sessionID string) {
		t.Helper()
		_, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    "summary://claude/" + sessionID,
			WorkspaceHash: workspaceHash,
		})
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("%s in %s: err = %v, want sql.ErrNoRows", sessionID, workspaceHash, err)
		}
	}

	writeTranscript("ses_legacy", "")
	sum := &countingSummarizer{db: q}
	h := NewClaudeCodeDiscoveryHarvester(db, zerolog.Nop(), func() []string { return []string{baseDir} })
	h.setSummarizer(sum)

	harvested, _, errCount := h.HarvestAll(ctx, nil)
	if harvested != 0 || errCount != 0 {
		t.Fatalf("cwd-less collision harvest = harvested %d errors %d, want 0/0", harvested, errCount)
	}
	if got := sum.calls.Load(); got != 0 {
		t.Fatalf("cwd-less collision summarizer calls = %d, want 0", got)
	}
	assertDocumentAbsent(wsAHash, "ses_legacy")
	assertDocumentAbsent(wsBHash, "ses_legacy")

	writeTranscript("ses_modern", wsBPath)
	harvested, _, errCount = h.HarvestAll(ctx, nil)
	if harvested != 1 || errCount != 0 {
		t.Fatalf("modern collision harvest = harvested %d errors %d, want 1/0", harvested, errCount)
	}
	if got := sum.calls.Load(); got != 1 {
		t.Fatalf("modern collision summarizer calls = %d, want 1", got)
	}
	assertDocumentAbsent(wsAHash, "ses_modern")
	if _, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://claude/ses_modern",
		WorkspaceHash: wsBHash,
	}); err != nil {
		t.Fatalf("modern transcript missing from matching workspace: %v", err)
	}
}
