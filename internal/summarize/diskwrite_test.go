package summarize

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureDir_CreatesNested(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "a", "b", "c", "file.md")

	if err := EnsureDir(nestedPath); err != nil {
		t.Fatalf("EnsureDir(%q) error: %v", nestedPath, err)
	}

	parentDir := filepath.Dir(nestedPath)
	if _, err := os.Stat(parentDir); err != nil {
		t.Errorf("parent directory %q not created: %v", parentDir, err)
	}
}

func TestEnsureDir_IdempotentExisting(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "subdir", "file.md")

	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("first EnsureDir(%q) error: %v", filePath, err)
	}

	// Call again — should not error
	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("second EnsureDir(%q) error: %v", filePath, err)
	}
}

func TestWriteFileAtomic_HappyPath(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	content := []byte("hello\n")

	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}

	if err := WriteFileAtomic(filePath, content); err != nil {
		t.Fatalf("WriteFileAtomic error: %v", err)
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

	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}

	if err := WriteFileAtomic(filePath, content); err != nil {
		t.Fatalf("WriteFileAtomic error: %v", err)
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

	got, err := ResolveCollision(filePath, content, sessionID)
	if err != nil {
		t.Fatalf("ResolveCollision error: %v", err)
	}

	if got != filePath {
		t.Errorf("ResolveCollision returned %q, want %q (no collision)", got, filePath)
	}
}

func TestResolveCollision_IdenticalContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	content := []byte("original content")
	sessionID := "ses_xyz"

	// Write original file
	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Call ResolveCollision with same content
	got, err := ResolveCollision(filePath, content, sessionID)
	if err != nil {
		t.Fatalf("ResolveCollision error: %v", err)
	}

	if got != filePath {
		t.Errorf("ResolveCollision returned %q, want %q (idempotent)", got, filePath)
	}
}

func TestResolveCollision_DifferentContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	contentA := []byte("content A")
	contentB := []byte("content B")
	sessionID := "ses_XYZ"

	// Write original file with content A
	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}
	if err := os.WriteFile(filePath, contentA, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Call ResolveCollision with different content B
	got, err := ResolveCollision(filePath, contentB, sessionID)
	if err != nil {
		t.Fatalf("ResolveCollision error: %v", err)
	}

	if got == filePath {
		t.Errorf("ResolveCollision returned original path, expected suffix")
	}

	// Verify suffix format: base_<8hex>.md
	expectedBase := filePath[:len(filePath)-3] // strip ".md"
	if !strings.HasPrefix(got, expectedBase) {
		t.Errorf("ResolveCollision result %q does not start with base %q", got, expectedBase)
	}

	if !strings.HasSuffix(got, ".md") {
		t.Errorf("ResolveCollision result %q does not end with .md", got)
	}

	// Extract suffix
	baseName := filepath.Base(got)
	if len(baseName) < 13 { // "base_12345678.md" minimum
		t.Errorf("ResolveCollision result %q has unexpected format", baseName)
	}
}

func TestResolveCollision_DifferentSessionIDsProduceDifferentSuffixes(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")
	content := []byte("some content")

	// Write original file
	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Call with sessionID "ses_a"
	got1, err := ResolveCollision(filePath, []byte("different"), "ses_a")
	if err != nil {
		t.Fatalf("ResolveCollision(ses_a) error: %v", err)
	}

	// Call with sessionID "ses_b"
	got2, err := ResolveCollision(filePath, []byte("different"), "ses_b")
	if err != nil {
		t.Fatalf("ResolveCollision(ses_b) error: %v", err)
	}

	if got1 == got2 {
		t.Errorf("ResolveCollision with different sessionIDs returned same path: %q", got1)
	}

	// Both should be different from original
	if got1 == filePath || got2 == filePath {
		t.Errorf("ResolveCollision returned original path")
	}
}

// #659: summaries can contain transcript content, so neither the files nor the
// directories created for them may be readable by group or other. Assert on
// the absence of those bits (not an exact mode) so a restrictive umask passes.
func TestEnsureDir_NewDirsNotGroupOrWorldAccessible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	dir := t.TempDir()
	filePath := filepath.Join(dir, "ws", "nested", "file.md")

	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("EnsureDir error: %v", err)
	}

	for _, created := range []string{filepath.Join(dir, "ws"), filepath.Join(dir, "ws", "nested")} {
		info, err := os.Stat(created)
		if err != nil {
			t.Fatalf("stat %q: %v", created, err)
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			t.Errorf("dir %q mode = %o, want no group/other bits (0700)", created, perm)
		}
		if perm := info.Mode().Perm(); perm&0o700 != 0o700 {
			t.Errorf("dir %q mode = %o, want owner rwx", created, perm)
		}
	}
}

func TestWriteFileAtomic_FileNotGroupOrWorldAccessible(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	dir := t.TempDir()
	filePath := filepath.Join(dir, "summary.md")

	if err := WriteFileAtomic(filePath, []byte("# transcript-derived summary\n")); err != nil {
		t.Fatalf("WriteFileAtomic error: %v", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("file mode = %o, want no group/other bits (0600)", perm)
	}
	if perm := info.Mode().Perm(); perm&0o600 != 0o600 {
		t.Errorf("file mode = %o, want owner rw", perm)
	}
}

// A pre-planted <path>.tmp symlink must not be clobbered: the temp file gets a
// unique name (os.CreateTemp), so a predictable sibling name is never opened.
func TestWriteFileAtomic_DoesNotFollowPredictableTmpSymlink(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "summary.md")

	sentinel := filepath.Join(dir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("sentinel content"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpLink := filePath + ".tmp"
	if err := os.Symlink(sentinel, tmpLink); err != nil {
		t.Skipf("symlink not permitted: %v", err)
	}

	if err := WriteFileAtomic(filePath, []byte("real content\n")); err != nil {
		t.Fatalf("WriteFileAtomic error: %v", err)
	}

	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("read sentinel: %v", err)
	}
	if string(got) != "sentinel content" {
		t.Errorf("sentinel was clobbered through the symlink: %q", got)
	}
	if got, err := os.ReadFile(filePath); err != nil || string(got) != "real content\n" {
		t.Errorf("filePath = %q, %v", got, err)
	}
	// The pre-planted link itself was never opened or replaced.
	if target, err := os.Readlink(tmpLink); err != nil || target != sentinel {
		t.Errorf(".tmp path changed: readlink = %q, %v", target, err)
	}
}
