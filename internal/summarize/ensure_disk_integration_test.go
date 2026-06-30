//go:build integration

package summarize

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// TestEnsureSummaryOnDisk_BackfillsAndIsIdempotent verifies:
//  1. EnsureSummaryOnDisk writes a .md file at the expected BuildDiskPath location
//     when the file is absent (backfill case).
//  2. A second call with identical content does NOT rewrite the file (idempotent):
//     the file's mtime is unchanged.
func TestEnsureSummaryOnDisk_BackfillsAndIsIdempotent(t *testing.T) {
	pgDB := setupPersistTestPG(t)
	ctx := context.Background()
	q := sqlc.New(pgDB)
	outputDir := t.TempDir()

	// Register a workspace so GetWorkspaceByHash succeeds inside persistToDisk.
	wsHash := "test-ws-ensure-disk-001"
	wsName := "ensure-disk-workspace"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: wsName,
		Path: "/tmp/ensure-disk-test-" + wsHash,
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	// First, insert the summary document into the DB (simulates the case where
	// the summary was created when write_to_disk was false).
	const summaryMarkdown = "# Backfill Test\n\nThis summary was in DB but not on disk.\n"
	createdAt := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)

	meta := SessionMetadata{
		Source:        SourceClaude,
		SessionID:     "ses-ensure-disk-001",
		Title:         "Claude Code Session ses-ensure-disk-001",
		CreatedAt:     createdAt,
		WorkspaceHash: wsHash,
	}

	// Construct persister with writeToDisk=true.
	p := NewPersister(pgDB, nil, true, outputDir, zerolog.Nop())

	// Confirm the file does NOT exist yet.
	expectedPath := filepath.Join(outputDir, wsName,
		BuildDiskPath("", wsName, wsHash, string(SourceClaude), meta.Title, createdAt)[len(""):])
	// Build the expected path the same way BuildDiskPath does.
	expectedPath = BuildDiskPath(outputDir, wsName, wsHash, string(SourceClaude), meta.Title, createdAt)

	if _, err := os.Stat(expectedPath); err == nil {
		t.Fatalf("precondition failed: file already exists at %s", expectedPath)
	}

	// --- Call 1: should create the file ---
	p.EnsureSummaryOnDisk(ctx, summaryMarkdown, meta)

	info1, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("EnsureSummaryOnDisk did not create file at %s: %v", expectedPath, err)
	}
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != summaryMarkdown {
		t.Errorf("file content mismatch\n got:  %q\n want: %q", string(content), summaryMarkdown)
	}

	mtime1 := info1.ModTime()

	// Small sleep so that any rewrite would produce a visibly different mtime.
	time.Sleep(20 * time.Millisecond)

	// --- Call 2: identical content — must NOT rewrite ---
	p.EnsureSummaryOnDisk(ctx, summaryMarkdown, meta)

	info2, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("file disappeared after second call: %v", err)
	}
	mtime2 := info2.ModTime()

	if !mtime2.Equal(mtime1) {
		t.Errorf("idempotency violated: file was rewritten on second call\n  mtime1=%v\n  mtime2=%v", mtime1, mtime2)
	}
}

// TestEnsureSummaryOnDisk_NoopWhenDisabled verifies that EnsureSummaryOnDisk
// is a no-op when writeToDisk is false, leaving outputDir empty.
func TestEnsureSummaryOnDisk_NoopWhenDisabled(t *testing.T) {
	pgDB := setupPersistTestPG(t)
	ctx := context.Background()
	q := sqlc.New(pgDB)
	outputDir := t.TempDir()

	wsHash := "test-ws-ensure-disk-002"
	wsName := "ensure-disk-disabled-workspace"
	if _, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: wsHash,
		Name: wsName,
		Path: "/tmp/ensure-disk-test-" + wsHash,
	}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}

	p := NewPersister(pgDB, nil, false, outputDir, zerolog.Nop())

	meta := SessionMetadata{
		Source:        SourceClaude,
		SessionID:     "ses-ensure-disk-002",
		Title:         "Claude Code Session ses-ensure-disk-002",
		CreatedAt:     time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
		WorkspaceHash: wsHash,
	}

	p.EnsureSummaryOnDisk(ctx, "# Should not appear\n", meta)

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty outputDir, found %d entries", len(entries))
	}
}
