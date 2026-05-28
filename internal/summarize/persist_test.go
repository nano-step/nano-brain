package summarize

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTitleSlug(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"  Leading/Trailing  ", "leading-trailing"},
		{"Multiple---dashes", "multiple-dashes"},
		{"", "untitled"},
		{"UPPER CASE", "upper-case"},
	}
	for _, tc := range cases {
		got := titleSlug(tc.input)
		if got != tc.want {
			t.Errorf("titleSlug(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestTitleSlug_MaxLength(t *testing.T) {
	long := "this-is-a-very-long-title-that-exceeds-eighty-characters-and-should-be-truncated-here-yes"
	got := titleSlug(long)
	if len(got) > 80 {
		t.Errorf("slug length %d > 80 for %q", len(got), long)
	}
}

func TestBuildSourcePath(t *testing.T) {
	meta := SessionMetadata{Source: SourceOpenCode, SessionID: "abc123"}
	got := buildSourcePath(meta)
	want := "summary://opencode/abc123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	meta2 := SessionMetadata{Source: SourceClaude, SessionID: "xyz789"}
	got2 := buildSourcePath(meta2)
	want2 := "summary://claude/xyz789"
	if got2 != want2 {
		t.Errorf("got %q, want %q", got2, want2)
	}
}

func TestPersister_WriteFile(t *testing.T) {
	dir := t.TempDir()
	p := &Persister{outputDir: dir}

	meta := SessionMetadata{
		Source:    SourceOpenCode,
		Title:     "My Test Session",
		CreatedAt: time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
	}
	content := "# Summary\nThis is the content."

	if err := p.writeFile(content, meta); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	expected := filepath.Join(dir, "opencode_my-test-session_2026-05-26.md")
	data, err := os.ReadFile(expected)
	if err != nil {
		t.Fatalf("expected file %q not found: %v", expected, err)
	}
	if string(data) != content {
		t.Errorf("file content mismatch: got %q, want %q", string(data), content)
	}
}

func TestPersister_WriteFile_CreatesDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "sub", "dir")
	p := &Persister{outputDir: dir}

	meta := SessionMetadata{
		Source:    SourceClaude,
		Title:     "Claude Session",
		CreatedAt: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
	}

	if err := p.writeFile("content", meta); err != nil {
		t.Fatalf("writeFile should create dir: %v", err)
	}

	expected := filepath.Join(dir, "claude_claude-session_2026-01-15.md")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected file %q: %v", expected, err)
	}
}

func TestPersister_WriteFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	p := &Persister{outputDir: dir}

	meta := SessionMetadata{
		Source:    SourceOpenCode,
		Title:     "Repeated",
		CreatedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := p.writeFile("first", meta); err != nil {
		t.Fatal(err)
	}
	if err := p.writeFile("second", meta); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "opencode_repeated_2026-03-01.md"))
	if string(data) != "second" {
		t.Errorf("overwrite failed: got %q", string(data))
	}
}
