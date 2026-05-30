package summarize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureDir_CreatesNested(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "a", "b", "c", "file.md")

	if err := ensureDir(nestedPath); err != nil {
		t.Fatalf("ensureDir(%q) error: %v", nestedPath, err)
	}

	parentDir := filepath.Dir(nestedPath)
	if _, err := os.Stat(parentDir); err != nil {
		t.Errorf("parent directory %q not created: %v", parentDir, err)
	}
}

func TestEnsureDir_IdempotentExisting(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "subdir", "file.md")

	if err := ensureDir(filePath); err != nil {
		t.Fatalf("first ensureDir(%q) error: %v", filePath, err)
	}

	// Call again — should not error
	if err := ensureDir(filePath); err != nil {
		t.Fatalf("second ensureDir(%q) error: %v", filePath, err)
	}
}

func TestWriteFileAtomic_HappyPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	content := []byte("hello\n")

	if err := ensureDir(filePath); err != nil {
		t.Fatalf("ensureDir error: %v", err)
	}

	if err := writeFileAtomic(filePath, content); err != nil {
		t.Fatalf("writeFileAtomic error: %v", err)
	}

	// Verify file content
	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", string(got), string(content))
	}

	// Verify no leftover .tmp file
	tmpPath := filePath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf("leftover .tmp file exists: %s", tmpPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking .tmp: %v", err)
	}
}

func TestWriteFileAtomic_LargeContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "large.md")

	// Create 100KB content
	content := make([]byte, 100*1024)
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}

	if err := ensureDir(filePath); err != nil {
		t.Fatalf("ensureDir error: %v", err)
	}

	if err := writeFileAtomic(filePath, content); err != nil {
		t.Fatalf("writeFileAtomic error: %v", err)
	}

	got, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if len(got) != len(content) {
		t.Errorf("content length mismatch: got %d bytes, want %d", len(got), len(content))
	}
	for i := range content {
		if got[i] != content[i] {
			t.Errorf("content mismatch at byte %d: got %x, want %x", i, got[i], content[i])
			break
		}
	}
}

func TestResolveCollision_NoExisting(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nonexistent.md")
	content := []byte("test")
	sessionID := "ses_abc123"

	got, err := resolveCollision(filePath, content, sessionID)
	if err != nil {
		t.Fatalf("resolveCollision error: %v", err)
	}

	if got != filePath {
		t.Errorf("resolveCollision returned %q, want %q (no collision)", got, filePath)
	}
}

func TestResolveCollision_IdenticalContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	content := []byte("original content")
	sessionID := "ses_xyz"

	// Write original file
	if err := ensureDir(filePath); err != nil {
		t.Fatalf("ensureDir error: %v", err)
	}
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Call resolveCollision with same content
	got, err := resolveCollision(filePath, content, sessionID)
	if err != nil {
		t.Fatalf("resolveCollision error: %v", err)
	}

	if got != filePath {
		t.Errorf("resolveCollision returned %q, want %q (idempotent)", got, filePath)
	}
}

func TestResolveCollision_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	contentA := []byte("content A")
	contentB := []byte("content B")
	sessionID := "ses_XYZ"

	// Write original file with content A
	if err := ensureDir(filePath); err != nil {
		t.Fatalf("ensureDir error: %v", err)
	}
	if err := os.WriteFile(filePath, contentA, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Call resolveCollision with different content B
	got, err := resolveCollision(filePath, contentB, sessionID)
	if err != nil {
		t.Fatalf("resolveCollision error: %v", err)
	}

	if got == filePath {
		t.Errorf("resolveCollision returned original path, expected suffix")
	}

	// Verify suffix format: base_<8hex>.md
	expectedBase := filePath[:len(filePath)-3] // strip ".md"
	if !strings.HasPrefix(got, expectedBase) {
		t.Errorf("resolveCollision result %q does not start with base %q", got, expectedBase)
	}

	if !strings.HasSuffix(got, ".md") {
		t.Errorf("resolveCollision result %q does not end with .md", got)
	}

	// Extract suffix
	baseName := filepath.Base(got)
	if len(baseName) < 13 { // "base_12345678.md" minimum
		t.Errorf("resolveCollision result %q has unexpected format", baseName)
	}
}

func TestResolveCollision_DifferentSessionIDsProduceDifferentSuffixes(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	content := []byte("some content")

	// Write original file
	if err := ensureDir(filePath); err != nil {
		t.Fatalf("ensureDir error: %v", err)
	}
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Call with sessionID "ses_a"
	got1, err := resolveCollision(filePath, []byte("different"), "ses_a")
	if err != nil {
		t.Fatalf("resolveCollision(ses_a) error: %v", err)
	}

	// Call with sessionID "ses_b"
	got2, err := resolveCollision(filePath, []byte("different"), "ses_b")
	if err != nil {
		t.Fatalf("resolveCollision(ses_b) error: %v", err)
	}

	if got1 == got2 {
		t.Errorf("resolveCollision with different sessionIDs returned same path: %q", got1)
	}

	// Both should be different from original
	if got1 == filePath || got2 == filePath {
		t.Errorf("resolveCollision returned original path")
	}
}
