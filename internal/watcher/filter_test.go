package watcher

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	gitignore "github.com/sabhiram/go-gitignore"
)

func TestShouldSkip_DefaultExcludeDirs(t *testing.T) {
	root := t.TempDir()
	f, _ := newFileFilter(root, nil, nil, nil)

	cases := []struct {
		path string
		skip bool
	}{
		{filepath.Join(root, "node_modules", "lodash", "index.js"), true},
		{filepath.Join(root, ".git", "config"), true},
		{filepath.Join(root, "dist", "bundle.js"), true},
		{filepath.Join(root, "vendor", "pkg.go"), true},
		{filepath.Join(root, "src", "main.go"), false},
		{filepath.Join(root, "README.md"), false},
	}

	for _, tc := range cases {
		got := f.shouldSkip(tc.path, false)
		if got != tc.skip {
			t.Errorf("shouldSkip(%q) = %v, want %v", tc.path, got, tc.skip)
		}
	}
}

func TestShouldSkip_ExcludePatterns(t *testing.T) {
	root := t.TempDir()
	f, _ := newFileFilter(root, []string{"*.lock", "*.log"}, nil, nil)

	cases := []struct {
		path string
		skip bool
	}{
		{filepath.Join(root, "package-lock.json"), false},
		{filepath.Join(root, "yarn.lock"), true},
		{filepath.Join(root, "server.log"), true},
		{filepath.Join(root, "main.go"), false},
	}

	for _, tc := range cases {
		got := f.shouldSkip(tc.path, false)
		if got != tc.skip {
			t.Errorf("shouldSkip(%q) = %v, want %v", tc.path, got, tc.skip)
		}
	}
}

func TestShouldSkip_AllowedExtensions(t *testing.T) {
	root := t.TempDir()
	f, _ := newFileFilter(root, nil, []string{".go", ".md"}, nil)

	cases := []struct {
		path string
		skip bool
	}{
		{filepath.Join(root, "main.go"), false},
		{filepath.Join(root, "README.md"), false},
		{filepath.Join(root, "index.js"), true},
		{filepath.Join(root, "style.css"), true},
		{filepath.Join(root, "data.json"), true},
	}

	for _, tc := range cases {
		got := f.shouldSkip(tc.path, false)
		if got != tc.skip {
			t.Errorf("shouldSkip(%q) = %v, want %v", tc.path, got, tc.skip)
		}
	}
}

func TestShouldSkip_AllowedExtensionsNoDot(t *testing.T) {
	root := t.TempDir()
	f, _ := newFileFilter(root, nil, []string{"go", "ts"}, nil)

	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should not be skipped when go is allowed")
	}
	if !f.shouldSkip(filepath.Join(root, "index.js"), false) {
		t.Error("index.js should be skipped when only go,ts are allowed")
	}
}

func TestShouldSkip_Gitignore(t *testing.T) {
	root := t.TempDir()

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.secret\nbuild/\n"), 0644); err != nil {
		t.Fatal(err)
	}

	f, _ := newFileFilter(root, nil, nil, nil)

	cases := []struct {
		path string
		skip bool
	}{
		{filepath.Join(root, "config.secret"), true},
		{filepath.Join(root, "build", "output.js"), true},
		{filepath.Join(root, "main.go"), false},
	}

	for _, tc := range cases {
		got := f.shouldSkip(tc.path, false)
		if got != tc.skip {
			t.Errorf("shouldSkip(%q) = %v, want %v", tc.path, got, tc.skip)
		}
	}
}

func TestShouldSkip_NoGitignore(t *testing.T) {
	root := t.TempDir()
	f, _ := newFileFilter(root, nil, nil, nil)

	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should not be skipped when no .gitignore")
	}
}

func TestLoadGlobalIgnore_MissingFileReturnsNil(t *testing.T) {
	home := t.TempDir() // empty — no .nano-brain/ exists
	gi, _, err := LoadGlobalIgnore(home)
	if err != nil {
		t.Fatalf("expected nil error when file missing, got %v", err)
	}
	if gi != nil {
		t.Error("expected nil matcher when file missing")
	}
}

func TestLoadGlobalIgnore_LoadsPatterns(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".nano-brain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "*.png\n!keep.png\nbuild/\n"
	if err := os.WriteFile(filepath.Join(dir, ".nano-brainignore"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	gi, path, err := LoadGlobalIgnore(home)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if gi == nil {
		t.Fatal("expected non-nil matcher")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("expected absolute path, got %q", path)
	}
	if !gi.MatchesPath("foo.png") {
		t.Error("foo.png should match *.png pattern")
	}
	if gi.MatchesPath("keep.png") {
		t.Error("keep.png should be un-matched (negation pattern)")
	}
	if !gi.MatchesPath("build/output.txt") {
		t.Error("build/output.txt should match build/ pattern")
	}
	if gi.MatchesPath("main.go") {
		t.Error("main.go should not match any global pattern")
	}
}

func TestFileFilter_GlobalIgnoreApplies(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	dir := filepath.Join(home, ".nano-brain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".nano-brainignore"), []byte("*.png\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gi, _, err := LoadGlobalIgnore(home)
	if err != nil || gi == nil {
		t.Fatalf("setup: %v, gi=%v", err, gi)
	}

	f, _ := newFileFilter(root, nil, nil, gi)
	if !f.shouldSkip(filepath.Join(root, "screenshot.png"), false) {
		t.Error("screenshot.png should be skipped via global ignore")
	}
	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should NOT be skipped (no global match)")
	}
}

func TestFileFilter_GlobalIgnoreMatchesDirectoryWithTrailingSlash(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	gdir := filepath.Join(home, ".nano-brain")
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// gitignore pattern with trailing slash matches dirs only — common Obsidian/build pattern.
	if err := os.WriteFile(filepath.Join(gdir, ".nano-brainignore"), []byte("custom_build/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gi, _, err := LoadGlobalIgnore(home)
	if err != nil || gi == nil {
		t.Fatalf("setup: %v", err)
	}

	f, _ := newFileFilter(root, nil, nil, gi)
	if !f.shouldSkip(filepath.Join(root, "custom_build"), true) {
		t.Error("custom_build directory must be skipped (gitignore 'custom_build/' pattern requires trailing slash)")
	}
	if !f.shouldSkip(filepath.Join(root, "custom_build", "output.bin"), false) {
		t.Error("file inside custom_build/ should also be skipped")
	}
	if f.shouldSkip(filepath.Join(root, "src"), true) {
		t.Error("src directory should NOT be skipped")
	}
}

func TestFileFilter_GlobalIgnoreCombinesWithPerCollection(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	gdir := filepath.Join(home, ".nano-brain")
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gdir, ".nano-brainignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gi, _, err := LoadGlobalIgnore(home)
	if err != nil || gi == nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("temp/\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, _ := newFileFilter(root, nil, nil, gi)
	if !f.shouldSkip(filepath.Join(root, "app.log"), false) {
		t.Error("app.log should be skipped via global ignore")
	}
	if !f.shouldSkip(filepath.Join(root, "temp", "cache.json"), false) {
		t.Error("temp/cache.json should be skipped via per-collection .gitignore")
	}
	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should NOT be skipped")
	}
}

func TestFileFilter_LocalNanoBrainIgnoreApplies(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".nano-brainignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := newFileFilter(root, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.localIgnore == nil {
		t.Fatal("expected non-nil localIgnore matcher")
	}
	if !f.shouldSkip(filepath.Join(root, "foo.tmp"), false) {
		t.Error("foo.tmp should be skipped via workspace .nano-brainignore")
	}
	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should NOT be skipped")
	}
}

func TestFileFilter_LocalNanoBrainIgnoreMissing(t *testing.T) {
	root := t.TempDir()

	f, err := newFileFilter(root, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error when file absent: %v", err)
	}
	if f.localIgnore != nil {
		t.Error("expected nil localIgnore matcher when file missing")
	}
	if f.shouldSkip(filepath.Join(root, "anything.tmp"), false) {
		t.Error("anything.tmp must NOT be skipped without a local matcher")
	}
}

func TestFileFilter_LocalNanoBrainIgnoreCombinesWithGlobal(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	gdir := filepath.Join(home, ".nano-brain")
	if err := os.MkdirAll(gdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gdir, ".nano-brainignore"), []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gi, _, err := LoadGlobalIgnore(home)
	if err != nil || gi == nil {
		t.Fatalf("setup global: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nano-brainignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := newFileFilter(root, nil, nil, gi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.shouldSkip(filepath.Join(root, "app.log"), false) {
		t.Error("app.log should be skipped via global ignore")
	}
	if !f.shouldSkip(filepath.Join(root, "scratch.tmp"), false) {
		t.Error("scratch.tmp should be skipped via local ignore")
	}
	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should NOT be skipped")
	}
}

func TestFileFilter_LocalNanoBrainIgnoreCombinesWithGitignore(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("tmp/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nano-brainignore"), []byte("*.snap\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := newFileFilter(root, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !f.shouldSkip(filepath.Join(root, "tmp", "x.go"), false) {
		t.Error("tmp/x.go should be skipped via .gitignore")
	}
	if !f.shouldSkip(filepath.Join(root, "fixture.snap"), false) {
		t.Error("fixture.snap should be skipped via workspace .nano-brainignore")
	}
	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should NOT be skipped")
	}
}

func TestFileFilter_LocalNanoBrainIgnoreUnreadable(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".nano-brainignore"), 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := newFileFilter(root, nil, nil, nil)
	if err == nil {
		t.Fatal("expected IO error when .nano-brainignore is a directory")
	}
	if f == nil {
		t.Fatal("expected non-nil *fileFilter even on error (callers continue with localIgnore nil)")
	}
	if f.localIgnore != nil {
		t.Error("localIgnore must be nil when file load failed")
	}
	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should NOT be skipped — other filter layers still operate")
	}
}

func TestFileFilter_LocalNanoBrainIgnorePermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 0000 semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file mode permissions")
	}

	root := t.TempDir()
	ignPath := filepath.Join(root, ".nano-brainignore")
	if err := os.WriteFile(ignPath, []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ignPath, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ignPath, 0o644) })

	f, err := newFileFilter(root, nil, nil, nil)
	if err == nil {
		t.Fatal("expected IO error when .nano-brainignore is unreadable (chmod 0000)")
	}
	if f == nil {
		t.Fatal("expected non-nil *fileFilter even on error")
	}
	if f.localIgnore != nil {
		t.Error("localIgnore must be nil when file load failed")
	}
	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should NOT be skipped — other filter layers still operate")
	}
}

func TestGitignoreStack_Push(t *testing.T) {
	root := t.TempDir()
	stack := &gitignoreStack{}

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gi, err := gitignore.CompileIgnoreFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	stack.Push(root, gi)

	if len(stack.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(stack.entries))
	}
	if stack.entries[0].dirPath != root {
		t.Errorf("expected dirPath %q, got %q", root, stack.entries[0].dirPath)
	}
}

func TestGitignoreStack_PopAbove(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "sub")
	deepdir := filepath.Join(subdir, "deep")

	stack := &gitignoreStack{}
	gi1 := gitignore.CompileIgnoreLines("*.a")
	gi2 := gitignore.CompileIgnoreLines("*.b")
	gi3 := gitignore.CompileIgnoreLines("*.c")

	stack.Push(root, gi1)
	stack.Push(subdir, gi2)
	stack.Push(deepdir, gi3)

	if len(stack.entries) != 3 {
		t.Fatalf("expected 3 entries after pushes, got %d", len(stack.entries))
	}

	stack.PopAbove(filepath.Join(subdir, "file.txt"))

	if len(stack.entries) != 2 {
		t.Errorf("expected 2 entries after PopAbove(subdir/file.txt), got %d", len(stack.entries))
	}

	stack.PopAbove(filepath.Join(root, "other.txt"))

	if len(stack.entries) != 1 {
		t.Errorf("expected 1 entry after PopAbove(root/other.txt), got %d", len(stack.entries))
	}
	if stack.entries[0].dirPath != root {
		t.Errorf("expected remaining entry to be root, got %q", stack.entries[0].dirPath)
	}
}

func TestGitignoreStack_Matches(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "sub")

	stack := &gitignoreStack{}
	gi1 := gitignore.CompileIgnoreLines("*.log")
	gi2 := gitignore.CompileIgnoreLines("*.tmp")

	stack.Push(root, gi1)
	stack.Push(subdir, gi2)

	cases := []struct {
		path  string
		match bool
	}{
		{filepath.Join(root, "app.log"), true},
		{filepath.Join(subdir, "data.tmp"), true},
		{filepath.Join(subdir, "other.log"), true},
		{filepath.Join(root, "main.go"), false},
		{filepath.Join(subdir, "file.go"), false},
	}

	for _, tc := range cases {
		got := stack.Matches(tc.path)
		if got != tc.match {
			t.Errorf("Matches(%q) = %v, want %v", tc.path, got, tc.match)
		}
	}
}

func TestGitignoreStack_NestedGitignoreExclusion(t *testing.T) {
	root := t.TempDir()
	subRepo := filepath.Join(root, "sub-repo")
	if err := os.MkdirAll(subRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subRepo, ".gitignore"), []byte("*.sub\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stack := &gitignoreStack{}
	giRoot, _ := gitignore.CompileIgnoreFile(filepath.Join(root, ".gitignore"))
	giSub, _ := gitignore.CompileIgnoreFile(filepath.Join(subRepo, ".gitignore"))

	stack.Push(root, giRoot)
	stack.Push(subRepo, giSub)

	cases := []struct {
		path  string
		match bool
	}{
		{filepath.Join(root, "file.root"), true},
		{filepath.Join(subRepo, "file.root"), true},
		{filepath.Join(subRepo, "file.sub"), true},
		{filepath.Join(root, "file.sub"), false},
		{filepath.Join(root, "main.go"), false},
		{filepath.Join(subRepo, "main.go"), false},
	}

	for _, tc := range cases {
		got := stack.Matches(tc.path)
		if got != tc.match {
			t.Errorf("Matches(%q) = %v, want %v", tc.path, got, tc.match)
		}
	}
}
