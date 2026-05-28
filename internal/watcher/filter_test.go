package watcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShouldSkip_DefaultExcludeDirs(t *testing.T) {
	root := t.TempDir()
	f := newFileFilter(root, nil, nil)

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
	f := newFileFilter(root, []string{"*.lock", "*.log"}, nil)

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
	f := newFileFilter(root, nil, []string{".go", ".md"})

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
	f := newFileFilter(root, nil, []string{"go", "ts"})

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

	f := newFileFilter(root, nil, nil)

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
	f := newFileFilter(root, nil, nil)

	if f.shouldSkip(filepath.Join(root, "main.go"), false) {
		t.Error("main.go should not be skipped when no .gitignore")
	}
}
