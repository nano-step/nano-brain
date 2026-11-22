//go:build linux

package watcher

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// /proc/self/pagemap is a regular file that reports size 0 yet reads for
// hundreds of GiB; a committed .gitignore or .nano-brainignore symlink to it
// must not exhaust the daemon's memory (#675). There is no equivalent file
// on macOS, so this test is Linux-only.
func TestIgnoreFiles_PagemapDoNotExhaust(t *testing.T) {
	const pagemap = "/proc/self/pagemap"
	info, err := os.Stat(pagemap)
	if err != nil || !info.Mode().IsRegular() {
		t.Skipf("%s unavailable: %v", pagemap, err)
	}

	for _, name := range []string{".gitignore", ".nano-brainignore"} {
		for _, dir := range []string{".", "sub", filepath.Join("sub", "deep")} {
			t.Run(filepath.Join(dir, name), func(t *testing.T) {
				root := t.TempDir()
				ignoreDir := filepath.Join(root, filepath.FromSlash(dir))
				if err := os.MkdirAll(ignoreDir, 0o755); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, filepath.Join(root, "ok.md"), "ok")
				mustWrite(t, filepath.Join(ignoreDir, "inner.md"), "inner")
				mustSymlink(t, pagemap, filepath.Join(ignoreDir, name))

				runtime.GC()
				var before runtime.MemStats
				runtime.ReadMemStats(&before)

				done := make(chan struct{})
				go func() {
					defer close(done)
					filter, ferr := NewFileFilter(root, nil, nil, nil)
					if ferr != nil {
						t.Errorf("NewFileFilter: %v", ferr)
						return
					}
					mq := newMockQuerier()
					w := newTestWatcher(mq, 2000, 300)
					w.scanCollection(context.Background(), watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*", filter: filter})
				}()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("scan did not return; an ignore file is being read without bound")
				}

				var after runtime.MemStats
				runtime.ReadMemStats(&after)
				// Reads are capped at 4 MiB per ignore file; an uncapped read
				// of pagemap grows the heap by gigabytes within seconds.
				if growth := int64(after.HeapAlloc) - int64(before.HeapAlloc); growth > 256<<20 {
					t.Errorf("heap grew by %d bytes; an ignore file was read without bound", growth)
				}
			})
		}
	}
}
