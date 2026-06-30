package summarize

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"tilde alone", "~", home},
		{"tilde slash", "~/", home + "/"},
		{"tilde subdir", "~/.nano-brain/summaries", filepath.Join(home, ".nano-brain/summaries")},
		{"no tilde absolute", "/tmp/foo", "/tmp/foo"},
		{"no tilde relative", "foo/bar", "foo/bar"},
		{"tilde not at start", "/tmp/~/foo", "/tmp/~/foo"},
		{"empty", "", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExpandTilde(c.input)
			if err != nil {
				t.Fatalf("ExpandTilde(%q) error: %v", c.input, err)
			}
			if c.name == "tilde slash" {
				if !strings.HasPrefix(got, home) {
					t.Errorf("ExpandTilde(%q) = %q, expected to start with %q", c.input, got, home)
				}
				return
			}
			if got != c.want {
				t.Errorf("ExpandTilde(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

func TestWorkspaceFolderName(t *testing.T) {
	cases := []struct {
		name, hash, want string
	}{
		{"nano-brain", "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f", "nano-brain"},
		{"My Workspace!", "abcdef1234567890", "my-workspace"},
		{"", "7f443561795a6fea64b6e8d35a9b06ed4d216b8a", "ws-7f443561795a"},
		{"", "abc", "ws-abc"},
		{"   ", "abcdef1234567890", "ws-abcdef123456"},
		{"workspace/with/slashes", "0123456789", "workspace-with-slashes"},
	}

	for _, c := range cases {
		subName := c.name + "|" + c.hash
		if len(c.hash) > 6 {
			subName = c.name + "|" + c.hash[:6]
		}
		t.Run(subName, func(t *testing.T) {
			got := WorkspaceFolderName(c.name, c.hash)
			if got != c.want {
				t.Errorf("WorkspaceFolderName(%q, %q) = %q, want %q", c.name, c.hash, got, c.want)
			}
		})
	}
}

func TestBuildDiskPath(t *testing.T) {
	outputDir := "/tmp/summaries"
	date := time.Date(2026, 5, 30, 14, 23, 0, 0, time.UTC)

	cases := []struct {
		name, wsName, wsHash, source, title, want string
	}{
		{
			"happy path",
			"nano-brain",
			"7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f",
			"opencode",
			"Watcher binary file filter root cause",
			"/tmp/summaries/nano-brain/2026-05-30--opencode-watcher-binary-file-filter-root-cause.md",
		},
		{
			"empty workspace name fallback",
			"",
			"7f443561795a6fea64b6e8d35a9b06ed4d216b8a",
			"claude",
			"Foo",
			"/tmp/summaries/ws-7f443561795a/2026-05-30--claude-foo.md",
		},
		{
			"empty title fallback",
			"nano-brain",
			"7f443561795a",
			"opencode",
			"",
			"/tmp/summaries/nano-brain/2026-05-30--opencode-untitled-session.md",
		},
		{
			"long title truncated",
			"nano-brain",
			"7f443561795a",
			"opencode",
			strings.Repeat("verylongword", 30),
			"/tmp/summaries/nano-brain/2026-05-30--opencode-" + strings.Repeat("verylongword", 6) + "verylong.md",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BuildDiskPath(outputDir, c.wsName, c.wsHash, c.source, c.title, date)
			if got != c.want {
				t.Errorf("BuildDiskPath() = %q\n                want %q", got, c.want)
			}
		})
	}
}

func TestBuildDiskPath_UsesUTCDate(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	dateLocal := time.Date(2026, 5, 30, 23, 30, 0, 0, loc)

	got := BuildDiskPath("/tmp", "ws", "hash", "opencode", "foo", dateLocal)
	if !strings.Contains(got, "2026-05-30") {
		t.Errorf("path %q should contain UTC date 2026-05-30 (Hanoi 23:30 + 7 = UTC 16:30 same day)", got)
	}

	dateLocalCrossesUTC := time.Date(2026, 5, 31, 2, 30, 0, 0, loc)
	got2 := BuildDiskPath("/tmp", "ws", "hash", "opencode", "foo", dateLocalCrossesUTC)
	if !strings.Contains(got2, "2026-05-30") {
		t.Errorf("path %q should contain UTC date 2026-05-30 (Hanoi May 31 02:30 = UTC May 30 19:30)", got2)
	}
}
