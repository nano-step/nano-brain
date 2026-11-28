package harvest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
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

func TestDiscoverClaudeLeafBindingsGroupsEncodedCollisions(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "projects")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wsA := filepath.Join(root, "work space")
	wsB := filepath.Join(root, "work-space")
	encodedA := EncodeClaudeProjectsDir(wsA)
	encodedB := EncodeClaudeProjectsDir(wsB)
	if encodedA != encodedB {
		t.Fatalf("test paths do not collide: %q vs %q", encodedA, encodedB)
	}
	leafDir := filepath.Join(baseDir, encodedA)
	if err := os.MkdirAll(leafDir, 0o755); err != nil {
		t.Fatal(err)
	}

	bindings := discoverClaudeLeafBindings([]string{baseDir}, []sqlc.Workspace{
		{Hash: "hash-a", Path: wsA},
		{Hash: "hash-b", Path: wsB},
	})
	if len(bindings) != 1 {
		t.Fatalf("discoverClaudeLeafBindings() returned %d bindings, want 1", len(bindings))
	}
	if len(bindings[0].workspaces) != 2 {
		t.Fatalf("collision binding has %d workspaces, want 2", len(bindings[0].workspaces))
	}
	if bindings[0].workspaces["hash-a"].Path != wsA || bindings[0].workspaces["hash-b"].Path != wsB {
		t.Fatalf("collision binding workspaces = %#v", bindings[0].workspaces)
	}
}

func TestDiscoverClaudeLeafBindingsKeepsDistinctLeavesSeparate(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "projects")
	wsA := filepath.Join(root, "alpha")
	wsB := filepath.Join(root, "beta")
	for _, ws := range []string{wsA, wsB} {
		if err := os.MkdirAll(filepath.Join(baseDir, EncodeClaudeProjectsDir(ws)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	bindings := discoverClaudeLeafBindings([]string{baseDir}, []sqlc.Workspace{
		{Hash: "hash-a", Path: wsA},
		{Hash: "hash-b", Path: wsB},
	})
	if len(bindings) != 2 {
		t.Fatalf("discoverClaudeLeafBindings() returned %d bindings, want 2", len(bindings))
	}
	for _, binding := range bindings {
		if len(binding.workspaces) != 1 {
			t.Fatalf("distinct leaf binding has %d workspaces, want 1", len(binding.workspaces))
		}
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
