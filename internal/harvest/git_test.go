package harvest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestRenderCommitMarkdown(t *testing.T) {
	commit := gitCommit{
		SHA:     "abc123def456",
		Author:  "John Doe",
		Email:   "john@example.com",
		Date:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Subject: "feat: add new feature",
		Body:    "This is a longer description\nwith multiple lines.",
		Files:   []string{"file1.go", "file2.go", "dir/file3.go"},
	}

	md := renderCommitMarkdown(commit)

	if !strings.Contains(md, "# feat: add new feature") {
		t.Errorf("missing subject heading in markdown")
	}
	if !strings.Contains(md, "**Author:** John Doe <john@example.com>") {
		t.Errorf("missing author in markdown")
	}
	if !strings.Contains(md, "**SHA:** abc123def456") {
		t.Errorf("missing SHA in markdown")
	}
	if !strings.Contains(md, "This is a longer description") {
		t.Errorf("missing body in markdown")
	}
	if !strings.Contains(md, "## Changed Files") {
		t.Errorf("missing changed files section")
	}
	if !strings.Contains(md, "- file1.go") {
		t.Errorf("missing file1.go in changed files")
	}
	if !strings.Contains(md, "- dir/file3.go") {
		t.Errorf("missing dir/file3.go in changed files")
	}
}

func TestParseGitLog(t *testing.T) {
	output := "\x1eabc123\x1fJohn Doe\x1fjohn@example.com\x1f2024-01-15T10:30:00Z\x1ffeat: add feature\x1fDetailed body\n\nfile1.go\nfile2.go\n" +
		"\x1edef456\x1fJane Smith\x1fjane@example.com\x1f2024-01-16T11:00:00Z\x1ffix: bug fix\x1fMore details here\n\nfile3.go\n"

	commits, err := parseGitLog(output)
	if err != nil {
		t.Fatalf("parseGitLog failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	c1 := commits[0]
	if c1.SHA != "abc123" {
		t.Errorf("commit 1 SHA: expected abc123, got %s", c1.SHA)
	}
	if c1.Author != "John Doe" {
		t.Errorf("commit 1 author: expected John Doe, got %s", c1.Author)
	}
	if c1.Subject != "feat: add feature" {
		t.Errorf("commit 1 subject: expected 'feat: add feature', got %s", c1.Subject)
	}
	if len(c1.Files) != 2 {
		t.Errorf("commit 1 files: expected 2, got %d", len(c1.Files))
	}

	c2 := commits[1]
	if c2.SHA != "def456" {
		t.Errorf("commit 2 SHA: expected def456, got %s", c2.SHA)
	}
	if c2.Author != "Jane Smith" {
		t.Errorf("commit 2 author: expected Jane Smith, got %s", c2.Author)
	}
}

func TestParseGitLogEmpty(t *testing.T) {
	commits, err := parseGitLog("")
	if err != nil {
		t.Fatalf("parseGitLog with empty output failed: %v", err)
	}
	if len(commits) != 0 {
		t.Errorf("expected 0 commits for empty output, got %d", len(commits))
	}
}

func TestIsBotCommit(t *testing.T) {
	tests := []struct {
		name     string
		author   string
		email    string
		expected bool
	}{
		{"dependabot", "dependabot[bot]", "dependabot@users.noreply.github.com", true},
		{"renovate", "Renovate Bot", "bot@renovateapp.com", true},
		{"github-actions", "github-actions[bot]", "actions@github.com", true},
		{"regular user", "John Doe", "john@example.com", false},
		{"bot keyword in renovate email", "Renovate", "renovate@example.com", true},
		{"case insensitive", "DEPENDABOT[bot]", "DEPENDABOT@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBotCommit(tt.author, tt.email)
			if result != tt.expected {
				t.Errorf("isBotCommit(%q, %q) = %v, want %v", tt.author, tt.email, result, tt.expected)
			}
		})
	}
}

func TestGitLogIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	tmpDir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test Author",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test Author",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	runGit("init")
	runGit("config", "user.name", "Test Author")
	runGit("config", "user.email", "test@example.com")

	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file1.txt")
	runGit("commit", "-m", "first commit")

	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file2.txt")
	runGit("commit", "-m", "second commit\n\nWith a body")

	h := &GitHarvester{logger: testLogger()}
	commits, err := h.gitLog(tmpDir, "", 10)
	if err != nil {
		t.Fatalf("gitLog failed: %v", err)
	}

	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}

	c1 := commits[0]
	if c1.Subject != "first commit" {
		t.Errorf("commit 1 subject: expected 'first commit', got %q", c1.Subject)
	}
	if !contains(c1.Files, "file1.txt") {
		t.Errorf("commit 1 should contain file1.txt, got %v", c1.Files)
	}

	c2 := commits[1]
	if c2.Subject != "second commit" {
		t.Errorf("commit 2 subject: expected 'second commit', got %q", c2.Subject)
	}
	if c2.Body != "With a body" {
		t.Errorf("commit 2 body: expected 'With a body', got %q", c2.Body)
	}
	if !contains(c2.Files, "file2.txt") {
		t.Errorf("commit 2 should contain file2.txt, got %v", c2.Files)
	}
}

func TestGitLogWithSince(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	tmpDir := t.TempDir()

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test Author",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test Author",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init")
	runGit("config", "user.name", "Test Author")
	runGit("config", "user.email", "test@example.com")

	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file1.txt")
	runGit("commit", "-m", "first commit")
	firstSHA := runGit("rev-parse", "HEAD")

	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file2.txt")
	runGit("commit", "-m", "second commit")

	h := &GitHarvester{logger: testLogger()}
	commits, err := h.gitLog(tmpDir, firstSHA, 10)
	if err != nil {
		t.Fatalf("gitLog with since failed: %v", err)
	}

	if len(commits) != 1 {
		t.Fatalf("expected 1 commit after firstSHA, got %d", len(commits))
	}

	if commits[0].Subject != "second commit" {
		t.Errorf("expected 'second commit', got %q", commits[0].Subject)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func testLogger() zerolog.Logger {
	return zerolog.New(os.Stdout).Level(zerolog.Disabled)
}
