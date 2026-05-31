package summarize

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

func seedWorkspaceForDisk(t *testing.T, q *sqlc.Queries, hash, name string) {
	t.Helper()
	if _, err := q.UpsertWorkspace(context.Background(), sqlc.UpsertWorkspaceParams{
		Hash: hash,
		Name: name,
		Path: "/tmp/disk-test-" + hash,
	}); err != nil {
		t.Fatalf("seed workspace %s: %v", hash, err)
	}
}

func TestPersist_WritesToDisk_WhenEnabled(t *testing.T) {
	pgDB := setupPersistTestPG(t)
	outputDir := t.TempDir()
	ctx := context.Background()
	q := sqlc.New(pgDB)

	seedWorkspaceForDisk(t, q, "test-ws-disk-001", "test-disk-workspace")

	p := NewPersister(pgDB, nil, true, outputDir, zerolog.Nop())
	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ses_disk001",
		Title:         "My Test Session",
		CreatedAt:     time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		WorkspaceHash: "test-ws-disk-001",
	}
	if err := p.Save(ctx, "# Summary\n\nbody", meta); err != nil {
		t.Fatalf("Save: %v", err)
	}

	expectedPath := filepath.Join(outputDir, "test-disk-workspace", "opencode_my-test-session_2026-05-30.md")
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("file not created at %s: %v", expectedPath, err)
	}
	if string(content) != "# Summary\n\nbody" {
		t.Errorf("file content mismatch: got %q", string(content))
	}
}

func TestPersist_NoFile_WhenWriteToDiskDisabled(t *testing.T) {
	pgDB := setupPersistTestPG(t)
	outputDir := t.TempDir()
	ctx := context.Background()
	q := sqlc.New(pgDB)

	seedWorkspaceForDisk(t, q, "test-ws-disk-002", "disk-disabled-workspace")

	p := NewPersister(pgDB, nil, false, outputDir, zerolog.Nop())
	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ses_nodisk002",
		Title:         "No Disk Session",
		CreatedAt:     time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		WorkspaceHash: "test-ws-disk-002",
	}
	if err := p.Save(ctx, "# Summary\n\nbody", meta); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files in outputDir, found %d entries", len(entries))
	}
}

func TestPersist_DBSucceeds_WhenDiskFails(t *testing.T) {
	pgDB := setupPersistTestPG(t)
	ctx := context.Background()
	q := sqlc.New(pgDB)

	seedWorkspaceForDisk(t, q, "test-ws-disk-003", "disk-fail-workspace")

	p := NewPersister(pgDB, nil, true, "/nonexistent/readonly/path/xyz123", zerolog.Nop())
	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ses_diskfail003",
		Title:         "Disk Fail Session",
		CreatedAt:     time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		WorkspaceHash: "test-ws-disk-003",
	}

	err := p.Save(ctx, "# Summary\n\nbody", meta)
	if err != nil {
		t.Fatalf("Save must return nil even on disk failure, got: %v", err)
	}

	doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    "summary://opencode/ses_diskfail003",
		WorkspaceHash: "test-ws-disk-003",
	})
	if err != nil {
		t.Fatalf("DB row not found after disk failure: %v", err)
	}
	if doc.Collection != "session-summary" {
		t.Errorf("collection = %q, want session-summary", doc.Collection)
	}
}

func TestPersist_Idempotent_SamePathOverwrite(t *testing.T) {
	pgDB := setupPersistTestPG(t)
	outputDir := t.TempDir()
	ctx := context.Background()
	q := sqlc.New(pgDB)

	seedWorkspaceForDisk(t, q, "test-ws-disk-004", "idempotent-workspace")

	p := NewPersister(pgDB, nil, true, outputDir, zerolog.Nop())
	meta := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ses_idem004",
		Title:         "Idempotent Session",
		CreatedAt:     time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC),
		WorkspaceHash: "test-ws-disk-004",
	}
	body := "# Summary\n\nbody"

	if err := p.Save(ctx, body, meta); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := p.Save(ctx, body, meta); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	expectedPath := filepath.Join(outputDir, "idempotent-workspace", "opencode_idempotent-session_2026-05-30.md")
	entries, err := filepath.Glob(filepath.Join(outputDir, "idempotent-workspace", "*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 file, found %d: %v", len(entries), entries)
	}
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content) != body {
		t.Errorf("content mismatch: got %q", string(content))
	}
}

func TestPersist_CollisionDifferentContent(t *testing.T) {
	pgDB := setupPersistTestPG(t)
	outputDir := t.TempDir()
	ctx := context.Background()
	q := sqlc.New(pgDB)

	seedWorkspaceForDisk(t, q, "test-ws-disk-005", "collision-workspace")

	p := NewPersister(pgDB, nil, true, outputDir, zerolog.Nop())
	sharedDate := time.Date(2026, 5, 30, 12, 0, 0, 0, time.UTC)

	metaA := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ses_coll_a005",
		Title:         "Collision Title",
		CreatedAt:     sharedDate,
		WorkspaceHash: "test-ws-disk-005",
	}
	metaB := SessionMetadata{
		Source:        SourceOpenCode,
		SessionID:     "ses_coll_b005",
		Title:         "Collision Title",
		CreatedAt:     sharedDate,
		WorkspaceHash: "test-ws-disk-005",
	}

	if err := p.Save(ctx, "# Summary A\n\nbody A", metaA); err != nil {
		t.Fatalf("Save A: %v", err)
	}
	if err := p.Save(ctx, "# Summary B\n\nbody B", metaB); err != nil {
		t.Fatalf("Save B: %v", err)
	}

	entries, err := filepath.Glob(filepath.Join(outputDir, "collision-workspace", "*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 files (original + collision suffix), found %d: %v", len(entries), entries)
	}

	basePath := filepath.Join(outputDir, "collision-workspace", "opencode_collision-title_2026-05-30.md")
	hasBase := false
	hasSuffix := false
	for _, e := range entries {
		if e == basePath {
			hasBase = true
		} else if strings.Contains(filepath.Base(e), "opencode_collision-title_2026-05-30_") {
			hasSuffix = true
		}
	}
	if !hasBase {
		t.Errorf("expected base file %s to exist; files: %v", basePath, entries)
	}
	if !hasSuffix {
		t.Errorf("expected collision-suffixed file; files: %v", entries)
	}
}
