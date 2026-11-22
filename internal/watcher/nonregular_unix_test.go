//go:build !windows

package watcher

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/symbol"
)

// mustMkfifo creates a named pipe. Reading one blocks until a writer opens it,
// which is what a scan must never do (issue #675).
func mustMkfifo(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(path, 0o644); err != nil {
		t.Skipf("mkfifo not available here: %v", err)
	}
}

// runOrFailOnBlock runs fn and fails the test, instead of hanging it, when fn
// is still running after 5s. Each blocked reader of a FIFO is released by
// opening the write end, so no goroutine is left stuck in a read.
func runOrFailOnBlock(t *testing.T, what string, fifos []string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
		return
	case <-time.After(5 * time.Second):
	}
	t.Errorf("%s blocked on a non-regular file", what)
	for release := true; release; {
		for _, fifo := range fifos {
			if f, err := os.OpenFile(fifo, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
				f.Close()
			}
		}
		select {
		case <-done:
			release = false
		case <-time.After(50 * time.Millisecond):
		}
	}
	t.FailNow()
}

// nonRegularTree builds a root holding regular files, a FIFO, and a symlink to
// the FIFO, all with the given extension, and returns the root and the FIFO.
func nonRegularTree(t *testing.T, ext, content string) (root, fifo string) {
	t.Helper()
	root = filepath.Join(t.TempDir(), "ws")
	mustWrite(t, filepath.Join(root, "a"+ext), content)
	mustWrite(t, filepath.Join(root, "sub", "b"+ext), content+"\n")
	fifo = filepath.Join(root, "sub", "pipe"+ext)
	mustMkfifo(t, fifo)
	mustSymlink(t, filepath.Join("sub", "pipe"+ext), filepath.Join(root, "pipe-alias"+ext))
	mustSymlink(t, "a"+ext, filepath.Join(root, "alias"+ext))
	return root, fifo
}

// TestNonRegularFilesAreSkipped: every path that reads workspace files returns
// promptly with a FIFO in the tree, reads the regular files, and reads neither
// the FIFO nor a symlink pointing at it (issue #675).
func TestNonRegularFilesAreSkipped(t *testing.T) {
	cases := []struct {
		name string
		ext  string
		// run performs the read path over root and returns the paths it read.
		run func(t *testing.T, root string) map[string]bool
	}{
		{
			name: "scanCollection",
			ext:  ".md",
			run: func(t *testing.T, root string) map[string]bool {
				mq := newMockQuerier()
				w := newTestWatcher(mq, 2000, 300)
				w.scanCollection(context.Background(), watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*"})
				// A purge starts with a document lookup; indexing a file costs one.
				if got := mq.getDocBySourcePathCalls.Load() - mq.upsertDocCalls.Load(); got != 0 {
					t.Errorf("document lookups beyond the indexed files = %d, want 0 (a skipped non-regular file is not purged)", got)
				}
				read := map[string]bool{}
				for path := range mq.sourcePathHash {
					read[path] = true
				}
				return read
			},
		},
		{
			name: "ReextractSymbolsForWorkspace",
			ext:  ".go",
			run: func(t *testing.T, root string) map[string]bool {
				rec := &recordingExtractor{read: map[string]bool{}}
				w := newTestWatcher(newMockQuerier(), 2000, 300)
				w.WithSymbolRegistry(symbol.NewRegistry(rec))
				if err := w.Watch("code", root, "ws", "*"); err != nil {
					t.Fatalf("Watch: %v", err)
				}
				w.ReextractSymbolsForWorkspace(context.Background(), "ws")
				return rec.read
			},
		},
		{
			name: "collectRubyContents",
			ext:  ".rb",
			run: func(t *testing.T, root string) map[string]bool {
				read := map[string]bool{}
				for path := range collectRubyContents(root) {
					read[path] = true
				}
				return read
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, fifo := nonRegularTree(t, tc.ext, "package main\n")
			var read map[string]bool
			runOrFailOnBlock(t, tc.name, []string{fifo}, func() { read = tc.run(t, root) })

			want := []string{"a" + tc.ext, filepath.Join("sub", "b"+tc.ext), "alias" + tc.ext}
			for _, rel := range want {
				if !read[filepath.Join(root, rel)] {
					t.Errorf("%s: regular file should be read, got %v", rel, read)
				}
			}
			for _, rel := range []string{filepath.Join("sub", "pipe"+tc.ext), "pipe-alias" + tc.ext} {
				if read[filepath.Join(root, rel)] {
					t.Errorf("%s: non-regular file must not be read", rel)
				}
			}
			if len(read) != len(want) {
				t.Errorf("read %d paths, want %d: %v", len(read), len(want), read)
			}
		})
	}
}

// recordingExtractor records which files the symbol re-extract walk read.
type recordingExtractor struct{ read map[string]bool }

func (r *recordingExtractor) Supports(ext string) bool { return ext == ".go" }

func (r *recordingExtractor) Extract(filePath string, _ []byte) ([]symbol.Symbol, error) {
	r.read[filePath] = true
	return nil, nil
}
