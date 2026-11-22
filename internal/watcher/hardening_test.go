package watcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestScanCollection_SymlinkTargetNestedIgnore: an alias is indexed only if a
// regular file at its target would be, including under the ignore files of the
// target's own directories (issue #675). Every case also pins that the walk
// itself honours the nested ignore file for the regular file. Every symlink
// target is a temp file.
func TestScanCollection_SymlinkTargetNestedIgnore(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	cases := []struct {
		name        string
		ignoreFile  string // relative to the root
		ignoreBody  string
		files       []string
		links       map[string]string // link (relative to root) -> target (relative to the link's directory)
		wantIndexed []string
		wantSkipped []string
	}{
		{
			name:        "target ignored by a nested .gitignore",
			ignoreFile:  "sub/.gitignore",
			ignoreBody:  "priv.txt\n",
			files:       []string{"sub/priv.txt", "sub/pub.txt"},
			links:       map[string]string{"alias.md": "sub/priv.txt"},
			wantIndexed: []string{"sub/.gitignore", "sub/pub.txt"},
			wantSkipped: []string{"sub/priv.txt", "alias.md"},
		},
		{
			name:        "target ignored by a nested .nano-brainignore",
			ignoreFile:  "sub/.nano-brainignore",
			ignoreBody:  "priv.txt\n",
			files:       []string{"sub/priv.txt", "sub/pub.txt"},
			links:       map[string]string{"alias.md": "sub/priv.txt"},
			wantIndexed: []string{"sub/.nano-brainignore", "sub/pub.txt"},
			wantSkipped: []string{"sub/priv.txt", "alias.md"},
		},
		{
			name:        "alias of a non-ignored file in the same subdirectory",
			ignoreFile:  "sub/.gitignore",
			ignoreBody:  "priv.txt\n",
			files:       []string{"sub/priv.txt", "sub/pub.txt"},
			links:       map[string]string{"alias.md": "sub/pub.txt"},
			wantIndexed: []string{"sub/.gitignore", "sub/pub.txt", "alias.md"},
			wantSkipped: []string{"sub/priv.txt"},
		},
		{
			name:        "negation in the nested file is respected",
			ignoreFile:  "sub/.gitignore",
			ignoreBody:  "*.txt\n!keep.txt\n",
			files:       []string{"sub/drop.txt", "sub/keep.txt"},
			links:       map[string]string{"kept.md": "sub/keep.txt", "dropped.md": "sub/drop.txt"},
			wantIndexed: []string{"sub/.gitignore", "sub/keep.txt", "kept.md"},
			wantSkipped: []string{"sub/drop.txt", "dropped.md"},
		},
		{
			name:        "ignore file in an intermediate directory, alias in a sibling directory",
			ignoreFile:  "a/.gitignore",
			ignoreBody:  "b/priv.txt\n",
			files:       []string{"a/b/priv.txt", "a/b/pub.txt"},
			links:       map[string]string{"docs/alias.md": "../a/b/priv.txt", "docs/ok.md": "../a/b/pub.txt"},
			wantIndexed: []string{"a/.gitignore", "a/b/pub.txt", "docs/ok.md"},
			wantSkipped: []string{"a/b/priv.txt", "docs/alias.md"},
		},
	}

	for _, tc := range cases {
		for _, withFilter := range []bool{true, false} {
			name := tc.name
			if !withFilter {
				name += ", no filter"
			}
			t.Run(name, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "ws")
				mustWrite(t, filepath.Join(root, filepath.FromSlash(tc.ignoreFile)), tc.ignoreBody)
				for _, rel := range tc.files {
					mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), "# "+rel+"\nnot a real secret")
				}
				for link, target := range tc.links {
					mustSymlink(t, filepath.FromSlash(target), filepath.Join(root, filepath.FromSlash(link)))
				}

				mq := newMockQuerier()
				w := newTestWatcher(mq, 2000, 300)
				col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*"}
				if withFilter {
					filter, err := NewFileFilter(root, nil, nil, nil)
					if err != nil {
						t.Fatal(err)
					}
					col.filter = filter
				}

				w.scanCollection(context.Background(), col)

				for _, rel := range tc.wantIndexed {
					if _, ok := mq.sourcePathHash[filepath.Join(root, filepath.FromSlash(rel))]; !ok {
						t.Errorf("%s: expected to be indexed, upserts=%v", rel, mq.sourcePathHash)
					}
				}
				for _, rel := range tc.wantSkipped {
					if _, ok := mq.sourcePathHash[filepath.Join(root, filepath.FromSlash(rel))]; ok {
						t.Errorf("%s: must not be indexed", rel)
					}
				}
				if got, want := mq.upsertDocCalls.Load(), int64(len(tc.wantIndexed)); got != want {
					t.Errorf("upserts = %d, want %d (%v)", got, want, mq.sourcePathHash)
				}
			})
		}
	}
}

// TestCheckResolvedPath_TargetUnderNestedIgnoredDirectory: a directory excluded
// by a nested ignore file excludes the alias targets beneath it, as the walk's
// SkipDir does for the regular files (issue #675).
func TestCheckResolvedPath_TargetUnderNestedIgnoredDirectory(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	root := filepath.Join(t.TempDir(), "ws")
	mustWrite(t, filepath.Join(root, "a", ".gitignore"), "b\n")
	mustWrite(t, filepath.Join(root, "a", "b", "c", "priv.txt"), "not a real secret")
	mustWrite(t, filepath.Join(root, "a", "pub.txt"), "public")
	mustSymlink(t, filepath.Join("a", "b", "c", "priv.txt"), filepath.Join(root, "alias.md"))
	mustSymlink(t, filepath.Join("a", "pub.txt"), filepath.Join(root, "ok.md"))
	filter, err := NewFileFilter(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		filter *FileFilter
		link   string
		want   error
	}{
		{"target under the ignored directory", filter, "alias.md", ErrTargetDenied},
		{"target under the ignored directory, no filter", nil, "alias.md", ErrTargetDenied},
		{"target beside the ignored directory", filter, "ok.md", nil},
		{"target beside the ignored directory, no filter", nil, "ok.md", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckResolvedPath(root, tc.filter, filepath.Join(root, tc.link))
			if tc.want == nil && got != nil || tc.want != nil && !errors.Is(got, tc.want) {
				t.Errorf("CheckResolvedPath = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCheckResolvedPath_RootInDifferentCase: on a case-insensitive filesystem
// a root registered in another casing than on disk still contains its own
// files, whatever spelling a symlink target uses, and still contains nothing
// else (issue #675). Skipped where the filesystem is case-sensitive.
func TestCheckResolvedPath_RootInDifferentCase(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(base, "Probe"), "probe")
	if _, err := os.Stat(filepath.Join(base, "probe")); err != nil {
		t.Skip("filesystem is case-sensitive")
	}

	onDisk := filepath.Join(base, "Root")
	root := filepath.Join(base, "root") // as registered
	mustWrite(t, filepath.Join(onDisk, "Docs", "real.md"), "# real\ninside the root")
	mustWrite(t, filepath.Join(onDisk, "Docs", ".gitignore"), "priv.txt\n")
	mustWrite(t, filepath.Join(onDisk, "Docs", "priv.txt"), "not a real secret")
	mustWrite(t, filepath.Join(onDisk, ".env"), "KEY=not-a-real-key")
	mustWrite(t, filepath.Join(base, "Outside", "secret.md"), "# secret\noutside the root")
	mustWrite(t, filepath.Join(base, "Root-evil", "secret.md"), "# secret\nsibling dir")
	mustSymlink(t, filepath.Join(onDisk, "Docs", "real.md"), filepath.Join(onDisk, "alias.md"))
	mustSymlink(t, filepath.Join(strings.ToUpper(onDisk), "DOCS", "REAL.MD"), filepath.Join(onDisk, "shouting.md"))
	mustSymlink(t, filepath.Join(base, "Outside", "secret.md"), filepath.Join(onDisk, "escape.md"))
	mustSymlink(t, filepath.Join(base, "Root-evil", "secret.md"), filepath.Join(onDisk, "sibling.md"))
	mustSymlink(t, filepath.Join(base, "root-evil", "secret.md"), filepath.Join(onDisk, "sibling-lower.md"))
	mustSymlink(t, filepath.Join(onDisk, ".env"), filepath.Join(onDisk, "setup.md"))
	mustSymlink(t, filepath.Join(onDisk, "Docs", "priv.txt"), filepath.Join(onDisk, "priv.md"))

	filter, err := NewFileFilter(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		path string
		want error
	}{
		{"absolute target in on-disk casing", filepath.Join(root, "alias.md"), nil},
		{"absolute target in a third casing", filepath.Join(root, "shouting.md"), nil},
		{"regular file addressed in on-disk casing", filepath.Join(onDisk, "Docs", "real.md"), nil},
		{"target outside the root", filepath.Join(root, "escape.md"), ErrOutsideRoot},
		{"sibling sharing the root's name prefix", filepath.Join(root, "sibling.md"), ErrOutsideRoot},
		{"sibling sharing the root's name prefix, lower case", filepath.Join(root, "sibling-lower.md"), ErrOutsideRoot},
		{"file outside addressed directly", filepath.Join(base, "Root-evil", "secret.md"), ErrOutsideRoot},
		{"parent directory", base, ErrOutsideRoot},
		{"credential target in on-disk casing", filepath.Join(root, "setup.md"), ErrTargetDenied},
		{"nested-ignored target in on-disk casing", filepath.Join(root, "priv.md"), ErrTargetDenied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckResolvedPath(root, filter, tc.path)
			if tc.want == nil && got != nil || tc.want != nil && !errors.Is(got, tc.want) {
				t.Errorf("CheckResolvedPath = %v, want %v", got, tc.want)
			}
		})
	}

	// End to end: the scan indexes the alias under the registered spelling.
	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)
	w.scanCollection(context.Background(), watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*", filter: filter})
	for rel, want := range map[string]bool{"alias.md": true, "shouting.md": true, "Docs/real.md": true, "escape.md": false, "sibling.md": false, "setup.md": false, "priv.md": false} {
		if _, ok := mq.sourcePathHash[filepath.Join(root, filepath.FromSlash(rel))]; ok != want {
			t.Errorf("%s: indexed = %v, want %v (upserts=%v)", rel, ok, want, mq.sourcePathHash)
		}
	}
}
