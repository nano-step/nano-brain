package harvest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExistingClaudeBaseDirsDedupesSymlinkAliases(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation often requires extra privilege on windows")
	}
	root := t.TempDir()
	realDir := filepath.Join(root, "real", "projects")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(root, "real"), alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	aliasProjects := filepath.Join(alias, "projects")

	got := existingClaudeBaseDirs([]string{realDir, aliasProjects, realDir})
	if len(got) != 1 || got[0] != realDir {
		t.Fatalf("existingClaudeBaseDirs() = %v, want only [%s]", got, realDir)
	}
}

func TestSameFilesystemPath_NormalizesLexicalAndSymlinkPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation often requires extra privilege on windows")
	}
	root := t.TempDir()
	realDir := filepath.Join(root, "workspace")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "workspace-link")
	if err := os.Symlink(realDir, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	if !sameFilesystemPath(realDir, filepath.Join(realDir, ".")) {
		t.Fatal("sameFilesystemPath should match lexical equivalents")
	}
	if !sameFilesystemPath(realDir, alias) {
		t.Fatal("sameFilesystemPath should match symlink aliases")
	}
	if sameFilesystemPath(realDir, filepath.Join(root, "other")) {
		t.Fatal("sameFilesystemPath should reject different paths")
	}
}

func TestEncodeClaudeProjectsDirMatchesClaudeCharacterRules(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/fake/workspace/plain", "-fake-workspace-plain"},
		{"/fake/work space/my_project.v2", "-fake-work-space-my-project-v2"},
		{`C:\fake\work tree`, "C--fake-work-tree"},
	}
	for _, tt := range tests {
		if got := EncodeClaudeProjectsDir(tt.path); got != tt.want {
			t.Errorf("EncodeClaudeProjectsDir(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
	// Claude Code's JavaScript encoder operates on UTF-16 code units, so a
	// non-BMP emoji contributes two dashes plus the preceding separator dash.
	if got, want := EncodeClaudeProjectsDir("/x/😀"), "-x---"; got != want {
		t.Errorf("EncodeClaudeProjectsDir emoji path = %q, want %q", got, want)
	}

	atLimit := "/" + strings.Repeat("a", 199)
	if got := EncodeClaudeProjectsDir(atLimit); len(got) != 200 || strings.Contains(got, "djaaup") {
		t.Errorf("EncodeClaudeProjectsDir 200-unit path length = %d with unexpected hash suffix %q", len(got), got)
	}
	overLimit := "/" + strings.Repeat("a", 210)
	got := EncodeClaudeProjectsDir(overLimit)
	if len(got) != 207 || !strings.HasSuffix(got, "-djaaup") {
		t.Errorf("EncodeClaudeProjectsDir over-limit path = length %d suffix %q, want length 207 and Java-hash suffix -djaaup", len(got), got[len(got)-7:])
	}
}
