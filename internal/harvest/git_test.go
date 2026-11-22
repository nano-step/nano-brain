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
	commits, _, err := h.gitLog(tmpDir, "", 10)
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
	commits, _, err := h.gitLog(tmpDir, firstSHA, 10)
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

// The since marker is read from a stored document. A value that is not a
// commit id must never reach git, where "--output=<path>" would make
// git log write a file (#656).
func TestGitLogIgnoresSinceThatIsNotACommitID(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file1.txt")
	runGit("commit", "-m", "first commit")

	h := &GitHarvester{logger: testLogger()}
	for _, since := range []string{"--output=" + filepath.Join(tmpDir, "written-by-git"), "-p", "--all", "HEAD~1", "abc", "ABCDEF1", "deadbeef;x"} {
		t.Run(since, func(t *testing.T) {
			commits, _, err := h.gitLog(tmpDir, since, 0)
			if err != nil {
				t.Fatalf("gitLog failed: %v", err)
			}
			if len(commits) != 1 {
				t.Errorf("got %d commits, want the full history (1)", len(commits))
			}
		})
	}
	if written, _ := filepath.Glob(filepath.Join(tmpDir, "written-by-git*")); len(written) > 0 {
		t.Errorf("git log wrote %v, the file named in the since marker", written)
	}
}

// Runs under -short (no git needed), so CI pins which markers reach git.
func TestGitObjectID(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"abbreviated id", "abc1234", true},
		{"sha-1", strings.Repeat("a1", 20), true},
		{"sha-256", strings.Repeat("a1", 32), true},
		{"empty", "", false},
		{"six characters", "abc123", false},
		{"sixty-five characters", strings.Repeat("a", 65), false},
		{"uppercase", "ABCDEF1", false},
		{"option", "--output=/tmp/x", false},
		{"short option", "-p", false},
		{"relative revision", "HEAD~1", false},
		{"range", "abc1234..def5678", false},
		{"trailing newline", "abc1234\n", false},
		{"embedded NUL then option", "abc1234\x00--all", false},
		{"leading space", " abc1234", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gitObjectID.MatchString(tt.value); got != tt.want {
				t.Errorf("gitObjectID.MatchString(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// newGitTestRepo creates a repository in a fresh temp dir with n linear
// commits ("commit 1" … "commit n") and returns its path and the commit ids,
// oldest first. Callers must skip under -short before calling it.
func newGitTestRepo(t *testing.T, n int) (string, []string) {
	t.Helper()

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

	shas := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		name := "file" + itoa(i) + ".txt"
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit("add", name)
		runGit("commit", "-m", "commit "+itoa(i))
		shas = append(shas, runGit("rev-parse", "HEAD"))
	}
	return tmpDir, shas
}

// A marker that is a well-formed id but names no commit in the repository
// (rewritten history, hand-edited marker) used to make "git log <id>..HEAD"
// fail on every cycle, and the marker was never repaired (#675).
func TestGitLogRecoversFromMarkerMissingInRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	tmpDir, shas := newGitTestRepo(t, 2)
	const missing = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	h := &GitHarvester{logger: testLogger()}

	t.Run("gitCommitMissing", func(t *testing.T) {
		tests := []struct {
			name string
			id   string
			want bool
		}{
			{"existing commit", shas[0], false},
			{"well-formed id not in the repo", missing, true},
			{"empty marker", "", false},
			{"malformed marker never reaches git", "--output=x", false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if got := gitCommitMissing(tmpDir, tt.id); got != tt.want {
					t.Errorf("gitCommitMissing(%q) = %v, want %v", tt.id, got, tt.want)
				}
			})
		}
	})

	tests := []struct {
		name         string
		since        string
		maxCommits   int
		wantSubjects []string
	}{
		{"missing marker restarts from the first commit", missing, 0, []string{"commit 1", "commit 2"}},
		{"missing marker with a limit restarts from the first commit", missing, 1, []string{"commit 1"}},
		{"valid marker is honoured", shas[0], 0, []string{"commit 2"}},
		{"no marker", "", 0, []string{"commit 1", "commit 2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commits, _, err := h.gitLogRecovering(tmpDir, tt.since, tt.maxCommits)
			if err != nil {
				t.Fatalf("gitLogRecovering failed: %v", err)
			}
			if got := commitSubjects(commits); strings.Join(got, "|") != strings.Join(tt.wantSubjects, "|") {
				t.Errorf("got commits %q, want %q", got, tt.wantSubjects)
			}
		})
	}

	// Without the recovery the failure is what harvestRepo saw every cycle.
	if _, _, err := h.gitLog(tmpDir, missing, 0); err == nil {
		t.Errorf("gitLog with a marker missing from the repo succeeded; the recovery test proves nothing")
	}
}

// "git log --reverse -nN" keeps the NEWEST N commits, so with more than N
// pending the marker jumped to HEAD and the older ones were never harvested
// (#675). Each batch must be the oldest N pending commits.
func TestGitLogLimitTakesOldestPendingCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	tmpDir, shas := newGitTestRepo(t, 5)
	h := &GitHarvester{logger: testLogger()}

	wantBatches := [][]string{
		{"commit 1", "commit 2"},
		{"commit 3", "commit 4"},
		{"commit 5"},
		nil,
	}
	marker := ""
	var seen []string
	for i, want := range wantBatches {
		commits, tip, err := h.gitLog(tmpDir, marker, 2)
		if err != nil {
			t.Fatalf("batch %d: gitLog failed: %v", i+1, err)
		}
		if got := commitSubjects(commits); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Fatalf("batch %d (marker %q): got %q, want %q", i+1, marker, got, want)
		}
		for _, c := range commits {
			seen = append(seen, c.SHA)
		}
		if len(commits) > 0 {
			// harvestRepo stores the batch end git itself reported as the marker.
			marker = tip
		}
	}
	if strings.Join(seen, "|") != strings.Join(shas, "|") {
		t.Errorf("harvested %q, want every commit exactly once in order %q", seen, shas)
	}

	t.Run("limit larger than the pending set", func(t *testing.T) {
		commits, _, err := h.gitLog(tmpDir, shas[2], 10)
		if err != nil {
			t.Fatalf("gitLog failed: %v", err)
		}
		if got := commitSubjects(commits); strings.Join(got, "|") != "commit 4|commit 5" {
			t.Errorf("got %q, want commits 4 and 5", got)
		}
	})
}

func commitSubjects(commits []gitCommit) []string {
	var subjects []string
	for _, c := range commits {
		subjects = append(subjects, c.Subject)
	}
	return subjects
}

// runGitIn runs git in dir with the test identity and returns trimmed stdout.
func runGitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
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

// gitCommitFile commits a new file with msg as the commit message and returns
// the commit id.
func gitCommitFile(t *testing.T, dir, name, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, dir, "add", name)
	runGitIn(t, dir, "commit", "-m", msg)
	return runGitIn(t, dir, "rev-parse", "HEAD")
}

// A marker that still exists but is not an ancestor of HEAD — after a
// force-push, reset, rebase or a switch to a branch that forked before the
// marker — makes the --ancestry-path walk empty, so with a limit the
// harvester reported "no new commits" forever while plain since..HEAD had
// pending work (#675). The cycle must fall back to the plain walk; every
// batch end is an ancestor of HEAD, so the next cycle is back on the
// ancestry path.
func TestGitLogLimitMarkerNotAncestorOfHead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	h := &GitHarvester{logger: testLogger()}

	// Each scenario returns the repo, the stale marker and the pending
	// commits that must be harvested exactly once.
	scenarios := []struct {
		name  string
		setup func(t *testing.T) (repo, marker string, want []string)
	}{
		{"force-push rewrote the branch", func(t *testing.T) (string, string, []string) {
			repo, shas := newGitTestRepo(t, 3) // P, Q, old
			runGitIn(t, repo, "reset", "--hard", shas[1])
			new1 := gitCommitFile(t, repo, "new1.txt", "new1")
			new2 := gitCommitFile(t, repo, "new2.txt", "new2")
			return repo, shas[2], []string{new1, new2}
		}},
		{"checkout moved to a branch without the marker", func(t *testing.T) (string, string, []string) {
			repo, _ := newGitTestRepo(t, 1) // P on the default branch
			defaultBranch := runGitIn(t, repo, "rev-parse", "--abbrev-ref", "HEAD")
			runGitIn(t, repo, "checkout", "-b", "feat")
			marker := gitCommitFile(t, repo, "f.txt", "F")
			runGitIn(t, repo, "checkout", defaultBranch)
			y1 := gitCommitFile(t, repo, "y1.txt", "Y1")
			y2 := gitCommitFile(t, repo, "y2.txt", "Y2")
			return repo, marker, []string{y1, y2}
		}},
	}

	for _, sc := range scenarios {
		for _, limit := range []int{1, 2} {
			t.Run(sc.name+"/limit "+itoa(limit), func(t *testing.T) {
				repo, marker, want := sc.setup(t)
				headSHA := runGitIn(t, repo, "rev-parse", "HEAD")
				seen := map[string]int{}
				settled := false
				for cycle := 0; cycle < 6; cycle++ {
					commits, tip, err := h.gitLog(repo, marker, limit)
					if err != nil {
						t.Fatalf("cycle %d (marker %.7s): gitLog failed: %v", cycle+1, marker, err)
					}
					if len(commits) == 0 {
						if cycle == 0 {
							t.Fatal("first cycle returned nothing; commits are pending")
						}
						settled = true
						break
					}
					for _, c := range commits {
						seen[c.SHA]++
						if seen[c.SHA] > 1 {
							t.Fatalf("commit %.7s harvested twice", c.SHA)
						}
					}
					// Even the cycle that falls back to the plain walk picks a
					// batch end that is an ancestor of HEAD, so the following
					// cycle is back on the ancestry path.
					if !gitIsAncestor(repo, tip, headSHA) {
						t.Fatalf("cycle %d: batch end %.7s is not an ancestor of HEAD", cycle+1, tip)
					}
					marker = tip
				}
				if !settled {
					t.Error("did not settle within 6 cycles")
				}
				if len(seen) != len(want) {
					t.Fatalf("harvested %d distinct commits, want %d", len(seen), len(want))
				}
				for _, sha := range want {
					if seen[sha] != 1 {
						t.Errorf("commit %.7s harvested %d times, want exactly once", sha, seen[sha])
					}
				}
			})
		}
	}
}

// A commit landing while a cycle runs must be picked up by the NEXT cycle,
// not skipped for good: HEAD is resolved once and that SHA bounds the batch
// log, the record filter and the stored marker. Re-reading HEAD after the
// batch would move the marker past a commit that was never harvested (#675).
// A git shim commits "raced" during the rev-list call — after the batch log
// already ran — to land a commit mid-cycle.
func TestGitLogMarkerIsHeadResolvedAtCycleStart(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available in PATH")
	}

	repo, shas := newGitTestRepo(t, 2) // C1, C2
	c1, c2 := shas[0], shas[1]

	sentinel := filepath.Join(t.TempDir(), "injected")
	shimDir := t.TempDir()
	shim := "#!/bin/sh\n" +
		"if [ \"$1\" = \"rev-list\" ] && [ ! -f \"" + sentinel + "\" ]; then\n" +
		"  touch \"" + sentinel + "\"\n" +
		"  echo raced > raced.txt\n" +
		"  \"" + realGit + "\" add raced.txt\n" +
		"  \"" + realGit + "\" commit -qm RACED\n" +
		"fi\n" +
		"exec \"" + realGit + "\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	h := &GitHarvester{logger: testLogger()}
	commits, tip, err := h.gitLog(repo, c1, 0)
	if err != nil {
		t.Fatalf("first cycle: gitLog failed: %v", err)
	}
	if tip != c2 {
		t.Fatalf("tip = %.7s, want the head resolved at cycle start %.7s", tip, c2)
	}
	if got := commitSubjects(commits); strings.Join(got, "|") != "commit 2" {
		t.Fatalf("first cycle commits = %v, want only commit 2; the mid-cycle commit belongs to the next cycle", got)
	}

	// The commit that landed mid-cycle is pending now.
	raced := runGitIn(t, repo, "rev-parse", "HEAD")
	commits, tip, err = h.gitLog(repo, c2, 0)
	if err != nil {
		t.Fatalf("second cycle: gitLog failed: %v", err)
	}
	if tip != raced {
		t.Errorf("second cycle tip = %.7s, want %.7s", tip, raced)
	}
	if got := commitSubjects(commits); strings.Join(got, "|") != "RACED" {
		t.Errorf("second cycle commits = %v, want the mid-cycle commit RACED", got)
	}
}

// The stored marker must come from git — the head the cycle resolved or the
// batch end derived from it — never from a record parsed out of the log
// body: a commit message can end with a forged \x1e record, making the fake
// hash the last thing parseGitLog sees. Stored as the marker it would poison
// the next cycle (#675).
func TestGitLogMarkerIsBatchEndNotParsedRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}

	repo, _ := newGitTestRepo(t, 1)
	fakeSHA := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	// C2's body ends with a forged record, so it is the last parsed entry.
	body := "C2\n\nbody\x1e" + fakeSHA + "\x1fforged author\x1ff@e.example\x1f2020-01-01T00:00:00Z\x1fforged subject"
	if err := os.WriteFile(filepath.Join(repo, "c2.txt"), []byte("2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "msg.txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitIn(t, repo, "add", "c2.txt")
	runGitIn(t, repo, "commit", "--cleanup=verbatim", "-F", "msg.txt")
	head := runGitIn(t, repo, "rev-parse", "HEAD")

	h := &GitHarvester{logger: testLogger()}
	_, tip, err := h.gitLog(repo, "", 0)
	if err != nil {
		t.Fatalf("gitLog failed: %v", err)
	}
	if tip != head {
		t.Errorf("tip = %.7s, want the batch end git resolved (%.7s), not the forged last parsed record %.7s", tip, head, fakeSHA)
	}
}

// newGitMergeRepo builds the reviewer repro: P -> C1 -> S -> A -> M on the
// default branch, with P -> B -> M merged in from a side branch. The marker
// starts at S, so A, M and B are pending but B does not descend from S.
func newGitMergeRepo(t *testing.T) (tmpDir string, shas map[string]string) {
	t.Helper()
	tmpDir = t.TempDir()
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
	commit := func(name, msg string) string {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(msg), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit("add", name)
		runGit("commit", "-m", msg)
		return runGit("rev-parse", "HEAD")
	}

	runGit("init")
	runGit("config", "user.name", "Test Author")
	runGit("config", "user.email", "test@example.com")
	// Works on the unborn HEAD before the first commit, unlike rev-parse.
	main := runGit("symbolic-ref", "--short", "HEAD")

	shas = map[string]string{}
	shas["P"] = commit("p.txt", "P")
	runGit("checkout", "-b", "side")
	shas["B"] = commit("b.txt", "B")
	runGit("checkout", main)
	shas["C1"] = commit("c1.txt", "C1")
	shas["S"] = commit("s.txt", "S")
	shas["A"] = commit("a.txt", "A")
	runGit("merge", "side", "-m", "M")
	shas["M"] = runGit("rev-parse", "HEAD")
	return tmpDir, shas
}

// A merge that pulls in a branch forking BEFORE the marker regressed the
// marker forever: the nth commit of plain since..HEAD need not descend from
// since, so storing it re-listed since's ancestors on every cycle (#675).
// With --ancestry-path the batch end is always a descendant of the marker.
func TestGitLogLimitMarkerMovesForwardOnMergeHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	tmpDir, shas := newGitMergeRepo(t)
	h := &GitHarvester{logger: testLogger()}

	for _, limit := range []int{1, 2} {
		t.Run("limit "+itoa(limit), func(t *testing.T) {
			marker := shas["S"]
			seen := map[string]int{}
			settled := false
			for cycle := 0; cycle < 8; cycle++ {
				commits, tip, err := h.gitLog(tmpDir, marker, limit)
				if err != nil {
					t.Fatalf("cycle %d (marker %.7s): gitLog failed: %v", cycle+1, marker, err)
				}
				if len(commits) == 0 {
					if cycle == 0 {
						t.Fatal("first cycle returned nothing; S has pending commits")
					}
					settled = true
					break
				}
				for _, c := range commits {
					seen[c.SHA]++
					if seen[c.SHA] > 1 {
						t.Fatalf("commit %.7s harvested twice", c.SHA)
					}
				}
				// The marker may only move forward: the previous marker must be
				// an ancestor of the new batch end.
				cmd := exec.Command("git", "merge-base", "--is-ancestor", marker, tip)
				cmd.Dir = tmpDir
				if err := cmd.Run(); err != nil {
					t.Fatalf("cycle %d: marker moved backwards %.7s -> %.7s", cycle+1, marker, tip)
				}
				marker = tip
			}
			if !settled {
				t.Error("did not settle within 8 cycles")
			}
			for _, name := range []string{"A", "B", "M"} {
				if seen[shas[name]] != 1 {
					t.Errorf("commit %s (%.7s) harvested %d times, want exactly once", name, shas[name], seen[shas[name]])
				}
			}
			if len(seen) != 3 {
				t.Errorf("harvested %d distinct commits, want exactly A, B and M", len(seen))
			}
		})
	}
}

// A commit message may contain the \x1e/\x1f separators parseGitLog splits
// on: a crafted body embeds a forged "record" claiming any hash. Forged
// records must be dropped so they cannot overwrite another commit's document
// or be stored as the marker (#675).
func TestGitLogDropsForgedRecords(t *testing.T) {
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
	commitFile := func(name, msgFile string) string {
		runGit("add", name)
		runGit("commit", "--cleanup=verbatim", "-F", msgFile)
		return runGit("rev-parse", "HEAD")
	}

	runGit("init")
	runGit("config", "user.name", "Test Author")
	runGit("config", "user.email", "test@example.com")

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("c1.txt", "1")
	runGit("add", "c1.txt")
	runGit("commit", "-m", "C1")
	c1 := runGit("rev-parse", "HEAD")
	write("c2.txt", "2")
	runGit("add", "c2.txt")
	runGit("commit", "-m", "C2")
	c2 := runGit("rev-parse", "HEAD")

	fakeSHA := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	// C3's body embeds two forged records: one naming a hash that is not a
	// commit in this repo, and one naming the real C2 inside the same range.
	forged := "C3\n\nbody" +
		"\x1e" + fakeSHA + "\x1fforged author\x1ff@e.example\x1f2020-01-01T00:00:00Z\x1fforged subject" +
		"\x1e" + c2 + "\x1fforged author\x1ff@e.example\x1f2020-01-01T00:00:00Z\x1fforged subject as " + c2[:7]
	write("c3.txt", "3")
	write("msg.txt", forged)
	c3 := commitFile("c3.txt", "msg.txt")

	h := &GitHarvester{logger: testLogger()}

	t.Run("unbounded", func(t *testing.T) {
		commits, tip, err := h.gitLog(tmpDir, "", 0)
		if err != nil {
			t.Fatalf("gitLog failed: %v", err)
		}
		if tip != c3 {
			t.Errorf("tip = %.7s, want the real batch end %.7s (git rev-parse HEAD)", tip, c3)
		}
		var got []string
		for _, c := range commits {
			got = append(got, c.SHA)
			if strings.Contains(c.Subject, "forged") {
				t.Errorf("forged record survived: sha %.7s subject %q", c.SHA, c.Subject)
			}
		}
		// The forged record duplicated real C2's hash, so both copies are
		// dropped rather than risk an overwrite; C1 and C3 remain.
		if strings.Join(got, "|") != strings.Join([]string{c1, c3}, "|") {
			t.Errorf("commits = %v, want exactly C1 and C3 (ambiguous C2 dropped)", got)
		}
	})

	t.Run("marker in place, forged in-range duplicates are dropped", func(t *testing.T) {
		commits, tip, err := h.gitLog(tmpDir, c1, 0)
		if err != nil {
			t.Fatalf("gitLog failed: %v", err)
		}
		if tip != c3 {
			t.Errorf("tip = %.7s, want %.7s", tip, c3)
		}
		for _, c := range commits {
			if c.SHA == fakeSHA {
				t.Errorf("forged non-repo hash survived")
			}
			if strings.Contains(c.Subject, "forged") {
				t.Errorf("forged record survived for sha %.7s", c.SHA)
			}
		}
		if len(commits) != 1 || commits[0].SHA != c3 {
			t.Errorf("commits = %v, want only C3 (ambiguous C2 dropped)", commitSubjects(commits))
		}
	})
}
