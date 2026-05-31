package watcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldSkip_DefaultExcludeDirs(t *testing.T) {
	root := t.TempDir()
	f := newFileFilter(root, nil, nil, nil)

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
	f := newFileFilter(root, []string{"*.lock", "*.log"}, nil, nil)

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
	f := newFileFilter(root, nil, []string{".go", ".md"}, nil)

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
	f := newFileFilter(root, nil, []string{"go", "ts"}, nil)

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

	f := newFileFilter(root, nil, nil, nil)

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
	f := newFileFilter(root, nil, nil, nil)

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

	f := newFileFilter(root, nil, nil, gi)
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

	f := newFileFilter(root, nil, nil, gi)
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

	f := newFileFilter(root, nil, nil, gi)
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
