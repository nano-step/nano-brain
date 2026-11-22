package watcher

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/symbol"
	"github.com/rs/zerolog"
)

func skipSymlinkTestOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink tests need privileges on Windows")
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

// TestScanCollection_SymlinkContainment drives the real scan (WalkDir →
// processFile) over temp directories and checks which source paths reach
// UpsertDocumentBySourcePath (issue #655). Every symlink target is a temp file.
func TestScanCollection_SymlinkContainment(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	cases := []struct {
		name string
		// setup builds the tree under base and returns the collection root.
		setup       func(t *testing.T, base string) string
		wantIndexed []string // relative to the returned root
		wantSkipped []string
	}{
		{
			name: "file symlink pointing outside the root is not indexed",
			setup: func(t *testing.T, base string) string {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(root, "real.md"), "# real\ninside the root")
				mustWrite(t, filepath.Join(base, "outside", "secret.md"), "# secret\noutside the root")
				mustSymlink(t, filepath.Join(base, "outside", "secret.md"), filepath.Join(root, "notes.md"))
				return root
			},
			wantIndexed: []string{"real.md"},
			wantSkipped: []string{"notes.md"},
		},
		{
			name: "file symlink pointing inside the root is indexed",
			setup: func(t *testing.T, base string) string {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(root, "docs", "real.md"), "# real\ninside the root")
				mustSymlink(t, filepath.Join(root, "docs", "real.md"), filepath.Join(root, "alias.md"))
				mustSymlink(t, filepath.Join("docs", "real.md"), filepath.Join(root, "relative-alias.md"))
				return root
			},
			wantIndexed: []string{"docs/real.md", "alias.md", "relative-alias.md"},
		},
		{
			name: "broken symlink is skipped",
			setup: func(t *testing.T, base string) string {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(root, "real.md"), "# real\ninside the root")
				mustSymlink(t, filepath.Join(root, "missing.md"), filepath.Join(root, "broken.md"))
				return root
			},
			wantIndexed: []string{"real.md"},
			wantSkipped: []string{"broken.md"},
		},
		{
			name: "root reached through a symlinked parent still indexes its files",
			setup: func(t *testing.T, base string) string {
				mustWrite(t, filepath.Join(base, "real", "ws", "a.md"), "# a\nregular file")
				mustWrite(t, filepath.Join(base, "real", "ws", "sub", "b.md"), "# b\nregular file")
				mustSymlink(t, filepath.Join(base, "real", "ws", "a.md"), filepath.Join(base, "real", "ws", "alias.md"))
				mustSymlink(t, filepath.Join(base, "real"), filepath.Join(base, "link"))
				return filepath.Join(base, "link", "ws")
			},
			wantIndexed: []string{"a.md", "sub/b.md", "alias.md"},
		},
		{
			name: "symlink into a sibling directory sharing the root's name prefix is not indexed",
			setup: func(t *testing.T, base string) string {
				root := filepath.Join(base, "root")
				mustWrite(t, filepath.Join(root, "real.md"), "# real\ninside the root")
				mustWrite(t, filepath.Join(base, "root-evil", "secret.md"), "# secret\nsibling dir")
				mustSymlink(t, filepath.Join(base, "root-evil", "secret.md"), filepath.Join(root, "abs.md"))
				mustSymlink(t, filepath.Join("..", "root-evil", "secret.md"), filepath.Join(root, "rel.md"))
				return root
			},
			wantIndexed: []string{"real.md"},
			wantSkipped: []string{"abs.md", "rel.md"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.setup(t, t.TempDir())
			mq := newMockQuerier()
			w := newTestWatcher(mq, 2000, 300)
			col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*"}

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

// TestProcessFile_SymlinkContainment covers paths that do not come from the
// WalkDir scan: a collection root that is itself a symlink, and a regular file
// reached through a symlinked directory.
func TestProcessFile_SymlinkContainment(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	cases := []struct {
		name        string
		setup       func(t *testing.T, base string) (root, file string)
		wantIndexed bool
	}{
		{
			name: "root that is itself a symlink to the real directory",
			setup: func(t *testing.T, base string) (string, string) {
				mustWrite(t, filepath.Join(base, "real", "a.md"), "# a\nregular file")
				mustSymlink(t, filepath.Join(base, "real"), filepath.Join(base, "ws"))
				return filepath.Join(base, "ws"), filepath.Join(base, "ws", "a.md")
			},
			wantIndexed: true,
		},
		{
			name: "regular file under a symlinked directory leaving the root",
			setup: func(t *testing.T, base string) (string, string) {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(base, "shared", "doc.md"), "# doc\noutside the root")
				mustSymlink(t, filepath.Join(base, "shared"), filepath.Join(root, "shared"))
				return root, filepath.Join(root, "shared", "doc.md")
			},
			wantIndexed: false,
		},
		{
			name: "regular file under a symlinked directory staying inside the root",
			setup: func(t *testing.T, base string) (string, string) {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(root, "docs", "doc.md"), "# doc\ninside the root")
				mustSymlink(t, filepath.Join(root, "docs"), filepath.Join(root, "alias"))
				return root, filepath.Join(root, "alias", "doc.md")
			},
			wantIndexed: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, file := tc.setup(t, t.TempDir())
			mq := newMockQuerier()
			w := newTestWatcher(mq, 2000, 300)
			col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*"}

			w.processFile(context.Background(), col, file)

			if _, got := mq.sourcePathHash[file]; got != tc.wantIndexed {
				t.Errorf("indexed = %v, want %v", got, tc.wantIndexed)
			}
		})
	}
}

func TestCheckResolvedPath(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	base := t.TempDir()
	root := filepath.Join(base, "root")
	mustWrite(t, filepath.Join(root, "sub", "in.md"), "in")
	mustWrite(t, filepath.Join(root, ".env"), "KEY=not-a-real-key")
	mustWrite(t, filepath.Join(root, ".gitignore"), "tok.txt\n")
	mustWrite(t, filepath.Join(root, "tok.txt"), "not-a-real-token")
	mustWrite(t, filepath.Join(base, "root-evil", "out.md"), "out")
	mustSymlink(t, root, filepath.Join(base, "root-link"))
	mustSymlink(t, filepath.Join(base, "root-evil", "out.md"), filepath.Join(root, "escape.md"))
	mustSymlink(t, filepath.Join(root, "sub", "in.md"), filepath.Join(root, "stay.md"))
	mustSymlink(t, filepath.Join(root, "nowhere"), filepath.Join(root, "broken.md"))
	mustSymlink(t, filepath.Join(root, ".env"), filepath.Join(root, "sub", "setup.md"))
	mustSymlink(t, filepath.Join(root, "tok.txt"), filepath.Join(root, "readme2.md"))
	mustSymlink(t, "loop.md", filepath.Join(root, "loop.md"))

	filter, err := NewFileFilter(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	linkFilter, err := NewFileFilter(filepath.Join(base, "root-link"), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		root   string
		filter *FileFilter
		path   string
		want   error // nil, a sentinel, or errAny for "some other error"
	}{
		{"regular file", root, filter, filepath.Join(root, "sub", "in.md"), nil},
		{"root itself", root, filter, root, nil},
		{"symlink staying inside", root, filter, filepath.Join(root, "stay.md"), nil},
		{"symlink escaping", root, filter, filepath.Join(root, "escape.md"), ErrOutsideRoot},
		{"sibling with shared name prefix", root, filter, filepath.Join(base, "root-evil", "out.md"), ErrOutsideRoot},
		{"parent directory", root, filter, base, ErrOutsideRoot},
		{"symlinked root, regular file", filepath.Join(base, "root-link"), linkFilter, filepath.Join(base, "root-link", "sub", "in.md"), nil},
		{"symlinked root, real path", filepath.Join(base, "root-link"), linkFilter, filepath.Join(root, "sub", "in.md"), nil},
		{"alias of a credential file", root, filter, filepath.Join(root, "sub", "setup.md"), ErrTargetDenied},
		{"alias of a credential file, no filter", root, nil, filepath.Join(root, "sub", "setup.md"), ErrTargetDenied},
		{"alias of a credential file, symlinked root", filepath.Join(base, "root-link"), linkFilter, filepath.Join(base, "root-link", "sub", "setup.md"), ErrTargetDenied},
		{"alias of a gitignored file", root, filter, filepath.Join(root, "readme2.md"), ErrTargetDenied},
		{"alias of a gitignored file, symlinked root", filepath.Join(base, "root-link"), linkFilter, filepath.Join(base, "root-link", "readme2.md"), ErrTargetDenied},
		{"alias of a gitignored file, no filter", root, nil, filepath.Join(root, "readme2.md"), nil},
		{"broken symlink", root, filter, filepath.Join(root, "broken.md"), fs.ErrNotExist},
		{"symlink loop", root, filter, filepath.Join(root, "loop.md"), errAny},
		{"missing root", filepath.Join(base, "absent"), nil, filepath.Join(root, "sub", "in.md"), fs.ErrNotExist},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckResolvedPath(tc.root, tc.filter, tc.path)
			switch {
			case tc.want == nil:
				if got != nil {
					t.Errorf("CheckResolvedPath = %v, want nil", got)
				}
			case tc.want == errAny:
				if got == nil || errors.Is(got, ErrOutsideRoot) || errors.Is(got, ErrTargetDenied) || errors.Is(got, fs.ErrNotExist) {
					t.Errorf("CheckResolvedPath = %v, want a resolve error other than not-exist", got)
				}
			default:
				if !errors.Is(got, tc.want) {
					t.Errorf("CheckResolvedPath = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

// errAny marks a test case that expects an error without a specific sentinel.
var errAny = errors.New("any error")

// The re-extract walk and the Ruby pre-read read files without going through
// processFile, so each carries its own containment check.
func TestReextractSymbols_SkipsSymlinkOutsideRoot(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	base := t.TempDir()
	root := filepath.Join(base, "ws")
	mustWrite(t, filepath.Join(root, "main.go"), "package main\nfunc Foo() {}\n")
	mustWrite(t, filepath.Join(base, "outside", "secret.go"), "package secret\nfunc Bar() {}\n")
	mustSymlink(t, filepath.Join(base, "outside", "secret.go"), filepath.Join(root, "leak.go"))
	mustSymlink(t, filepath.Join(root, "main.go"), filepath.Join(root, "alias.go"))

	w := newTestWatcher(newMockQuerier(), 2000, 300)
	w.WithSymbolRegistry(symbol.NewRegistry(fakeExtractor{ext: ".go"}))
	if err := w.Watch("code", root, "ws", "*"); err != nil {
		t.Fatalf("Watch: %v", err)
	}

	if got := w.ReextractSymbolsForWorkspace(context.Background(), "ws"); got != 2 {
		t.Fatalf("files re-extracted = %d, want 2 (main.go and alias.go, not leak.go)", got)
	}
}

func TestCollectRubyContents_SkipsSymlinkOutsideRoot(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	base := t.TempDir()
	root := filepath.Join(base, "ws")
	mustWrite(t, filepath.Join(root, "app.rb"), "class App; end\n")
	mustWrite(t, filepath.Join(base, "outside", "secret.rb"), "SECRET = 1\n")
	mustSymlink(t, filepath.Join(base, "outside", "secret.rb"), filepath.Join(root, "leak.rb"))
	mustSymlink(t, filepath.Join(root, "app.rb"), filepath.Join(root, "alias.rb"))

	got := collectRubyContents(root)

	if _, ok := got[filepath.Join(root, "leak.rb")]; ok {
		t.Error("leak.rb resolves outside the root and must not be read")
	}
	for _, name := range []string{"app.rb", "alias.rb"} {
		if _, ok := got[filepath.Join(root, name)]; !ok {
			t.Errorf("%s should be read", name)
		}
	}
}

// purgeTrackingQuerier gives each source path a stable document ID and records
// which documents had their chunks / row deleted, so a test can tell whose
// document a cleanup removed (the base mock only counts calls).
type purgeTrackingQuerier struct {
	*mockQuerier
	idByPath      map[string]uuid.UUID
	pathByID      map[uuid.UUID]string
	chunksDeleted map[string]int
	docsDeleted   map[string]int
}

func newPurgeTrackingQuerier() *purgeTrackingQuerier {
	return &purgeTrackingQuerier{
		mockQuerier:   newMockQuerier(),
		idByPath:      make(map[string]uuid.UUID),
		pathByID:      make(map[uuid.UUID]string),
		chunksDeleted: make(map[string]int),
		docsDeleted:   make(map[string]int),
	}
}

// seed registers a document as already indexed for path.
func (q *purgeTrackingQuerier) seed(path string) {
	id := uuid.New()
	q.idByPath[path] = id
	q.pathByID[id] = path
	q.sourcePathHash[path] = "stale-hash"
}

func (q *purgeTrackingQuerier) GetDocumentBySourcePath(_ context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.DocumentView, error) {
	id, ok := q.idByPath[arg.SourcePath]
	if !ok {
		return sqlc.DocumentView{}, sql.ErrNoRows
	}
	return sqlc.DocumentView{ID: id, ContentHash: q.sourcePathHash[arg.SourcePath]}, nil
}

func (q *purgeTrackingQuerier) DeleteChunksByDocumentID(_ context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error {
	q.chunksDeleted[q.pathByID[arg.DocumentID]]++
	return nil
}

func (q *purgeTrackingQuerier) DeleteDocumentByIDAndWorkspace(_ context.Context, arg sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error) {
	q.docsDeleted[q.pathByID[arg.ID]]++
	delete(q.idByPath, q.pathByID[arg.ID])
	return 1, nil
}

// TestScanCollection_PurgesDocumentsForEscapingSymlinks: a document indexed by
// an earlier version for a path that is a symlink resolving outside the root
// (or that can no longer be resolved) is deleted, with its chunks and its
// file-cache entry, on the next scan; documents of in-root symlinks and regular
// files are left alone (issue #655). Graph rows are deleted by the same
// cleanupIgnoredDocument call but need a real *sql.DB, so they are covered by
// the integration test TestDeleteGraphRowsForFile rather than here.
func TestScanCollection_PurgesDocumentsForEscapingSymlinks(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	cases := []struct {
		name string
		// setup builds the tree and returns the root plus the file under test.
		setup func(t *testing.T, base string) (root, file string)
		// viaProcessFile calls processFile on the file instead of scanning the
		// root, for paths WalkDir never yields (under a symlinked directory).
		viaProcessFile bool
		wantPurged     bool
	}{
		{
			name: "symlink resolving outside the root is purged",
			setup: func(t *testing.T, base string) (string, string) {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(base, "outside", "secret.md"), "# secret\noutside the root")
				mustSymlink(t, filepath.Join(base, "outside", "secret.md"), filepath.Join(root, "notes.md"))
				return root, filepath.Join(root, "notes.md")
			},
			wantPurged: true,
		},
		{
			name: "broken symlink is purged",
			setup: func(t *testing.T, base string) (string, string) {
				root := filepath.Join(base, "ws")
				mustSymlink(t, filepath.Join(root, "missing.md"), filepath.Join(root, "broken.md"))
				return root, filepath.Join(root, "broken.md")
			},
			wantPurged: true,
		},
		{
			name: "regular file under a symlinked directory leaving the root is purged",
			setup: func(t *testing.T, base string) (string, string) {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(base, "shared", "doc.md"), "# doc\noutside the root")
				mustSymlink(t, filepath.Join(base, "shared"), filepath.Join(root, "shared"))
				return root, filepath.Join(root, "shared", "doc.md")
			},
			viaProcessFile: true,
			wantPurged:     true,
		},
		{
			name: "symlink resolving inside the root is kept",
			setup: func(t *testing.T, base string) (string, string) {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(root, "docs", "real.md"), "# real\ninside the root")
				mustSymlink(t, filepath.Join(root, "docs", "real.md"), filepath.Join(root, "alias.md"))
				return root, filepath.Join(root, "alias.md")
			},
			wantPurged: false,
		},
		{
			name: "regular file is kept",
			setup: func(t *testing.T, base string) (string, string) {
				root := filepath.Join(base, "ws")
				mustWrite(t, filepath.Join(root, "real.md"), "# real\ninside the root")
				return root, filepath.Join(root, "real.md")
			},
			wantPurged: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, file := tc.setup(t, t.TempDir())
			q := newPurgeTrackingQuerier()
			q.seed(file)
			w := New(nil, q, testLogger(), testConfig(2000, 300))
			// A stale fingerprint, as warmFileCacheFromDB would load for a
			// previously indexed path; it must not survive a purge.
			w.fileCache[file] = fileState{Size: -1, Hash: "stale-hash"}
			col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*"}

			if tc.viaProcessFile {
				w.processFile(context.Background(), col, file)
			} else {
				w.scanCollection(context.Background(), col)
			}

			wantDeletes := 0
			if tc.wantPurged {
				wantDeletes = 1
			}
			if got := q.docsDeleted[file]; got != wantDeletes {
				t.Errorf("document deletes = %d, want %d", got, wantDeletes)
			}
			if got := q.chunksDeleted[file]; got != wantDeletes {
				t.Errorf("chunk deletes by document = %d, want %d", got, wantDeletes)
			}
			if len(q.docsDeleted) > wantDeletes {
				t.Errorf("unexpected document deletes: %v", q.docsDeleted)
			}
			_, cached := w.fileCache[file]
			if tc.wantPurged {
				if cached {
					t.Error("file cache entry must be dropped with the purged document")
				}
				if q.sourcePathHash[file] != "stale-hash" {
					t.Error("a purged path must not be re-indexed")
				}
				return
			}
			if q.sourcePathHash[file] == "stale-hash" {
				t.Error("an in-root file must be re-indexed, not left stale")
			}
			if !cached {
				t.Error("an in-root file must keep a file cache entry")
			}
		})
	}
}

// TestScanCollection_SymlinkAliasOfExcludedFile: an in-root symlink must not
// carry the content of a file that is excluded under its own name — by the
// credential deny list or by the user's ignore rules (issue #655 review).
func TestScanCollection_SymlinkAliasOfExcludedFile(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	root := filepath.Join(t.TempDir(), "ws")
	mustWrite(t, filepath.Join(root, ".gitignore"), "tok.txt\n")
	mustWrite(t, filepath.Join(root, ".env"), "API_KEY=not-a-real-key")
	mustWrite(t, filepath.Join(root, "certs", "server.key"), "not a real key")
	mustWrite(t, filepath.Join(root, "tok.txt"), "not-a-real-token")
	mustWrite(t, filepath.Join(root, "real.md"), "# real\nordinary file")
	mustSymlink(t, filepath.Join("..", ".env"), filepath.Join(root, "docs", "setup.md"))
	mustSymlink(t, ".env", filepath.Join(root, "notes.md"))
	mustSymlink(t, filepath.Join("certs", "server.key"), filepath.Join(root, "link.txt"))
	mustSymlink(t, "tok.txt", filepath.Join(root, "readme2.md"))
	mustSymlink(t, "real.md", filepath.Join(root, "alias.md"))
	// Dangling for the first scan; its target appears before the second.
	mustSymlink(t, filepath.Join("late", ".env.production"), filepath.Join(root, "later.md"))

	q := newPurgeTrackingQuerier()
	// Documents an earlier version indexed for two of the aliases.
	q.seed(filepath.Join(root, "notes.md"))
	q.seed(filepath.Join(root, "readme2.md"))
	w := New(nil, q, testLogger(), testConfig(2000, 300))
	filter, err := NewFileFilter(root, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*", filter: filter}

	wantIndexed := []string{".gitignore", "real.md", "alias.md"}
	wantSkipped := []string{
		".env", "certs/server.key", "tok.txt",
		"docs/setup.md", "notes.md", "link.txt", "readme2.md", "later.md",
	}
	check := func(scan string) {
		t.Helper()
		for _, rel := range wantIndexed {
			if h, ok := q.sourcePathHash[filepath.Join(root, filepath.FromSlash(rel))]; !ok || h == "stale-hash" {
				t.Errorf("%s: %s should be indexed", scan, rel)
			}
		}
		for _, rel := range wantSkipped {
			if h, ok := q.sourcePathHash[filepath.Join(root, filepath.FromSlash(rel))]; ok && h != "stale-hash" {
				t.Errorf("%s: %s must not be indexed", scan, rel)
			}
		}
		if got, want := q.upsertDocCalls.Load(), int64(len(wantIndexed)); got != want {
			t.Errorf("%s: upserts = %d, want %d", scan, got, want)
		}
	}

	w.scanCollection(context.Background(), col)
	check("first scan")
	for _, rel := range []string{"notes.md", "readme2.md"} {
		if got := q.docsDeleted[filepath.Join(root, rel)]; got != 1 {
			t.Errorf("%s: previously indexed document deletes = %d, want 1", rel, got)
		}
	}

	mustWrite(t, filepath.Join(root, "late", ".env.production"), "DB_PASSWORD=not-a-real-password")
	w.scanCollection(context.Background(), col)
	check("second scan, dangling link now live")
}

// TestProcessFile_SymlinkSkipLoggingAndPurge pins which skips purge and which
// warn: a directory symlink does neither, a file symlink leaving the root warns
// once per process and purges, and an unresolvable-but-existing link (a loop)
// warns without purging.
func TestProcessFile_SymlinkSkipLoggingAndPurge(t *testing.T) {
	skipSymlinkTestOnWindows(t)

	cases := []struct {
		name      string
		setup     func(t *testing.T, base, root string) string // returns the link
		wantWarns []int                                        // per scan
		wantPurge bool
	}{
		{
			name: "directory symlink leaving the root",
			setup: func(t *testing.T, base, root string) string {
				mustWrite(t, filepath.Join(base, "out", "bin", "x.md"), "# x")
				mustSymlink(t, filepath.Join(base, "out", "bin"), filepath.Join(root, "bazel-bin"))
				return filepath.Join(root, "bazel-bin")
			},
			wantWarns: []int{0, 0},
			wantPurge: false,
		},
		{
			name: "file symlink leaving the root",
			setup: func(t *testing.T, base, root string) string {
				mustWrite(t, filepath.Join(base, "out", "secret.md"), "# secret")
				mustSymlink(t, filepath.Join(base, "out", "secret.md"), filepath.Join(root, "notes.md"))
				return filepath.Join(root, "notes.md")
			},
			wantWarns: []int{1, 0},
			wantPurge: true,
		},
		{
			name: "symlink loop",
			setup: func(t *testing.T, base, root string) string {
				mustSymlink(t, "loop.md", filepath.Join(root, "loop.md"))
				return filepath.Join(root, "loop.md")
			},
			wantWarns: []int{1, 1},
			wantPurge: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "ws")
			link := tc.setup(t, base, root)

			var logs bytes.Buffer
			q := newPurgeTrackingQuerier()
			q.seed(link)
			w := New(nil, q, zerolog.New(&logs).Level(zerolog.WarnLevel), testConfig(2000, 300))
			col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*"}

			for i, want := range tc.wantWarns {
				logs.Reset()
				w.scanCollection(context.Background(), col)
				if got := strings.Count(logs.String(), `"level":"warn"`); got != want {
					t.Errorf("scan %d: warn lines = %d, want %d\n%s", i+1, got, want, logs.String())
				}
			}
			purged := q.docsDeleted[link] > 0
			if purged != tc.wantPurge {
				t.Errorf("purged = %v, want %v", purged, tc.wantPurge)
			}
			if q.upsertDocCalls.Load() != 0 {
				t.Errorf("nothing should be indexed, got %v", q.sourcePathHash)
			}
		})
	}
}
