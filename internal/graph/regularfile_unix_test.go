//go:build !windows

package graph_test

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/graph"
)

// runBounded fails the test instead of hanging when fn is still running after
// the deadline — a FIFO read or an unbounded /dev/zero read shows up as a
// timeout, not a stuck suite. release, when non-nil, is invoked on timeout to
// unblock a regressed reader (e.g. opening a FIFO's write end) so the suite
// can finish (#675).
func runBounded(t *testing.T, timeout time.Duration, release func(), fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
		return
	case <-time.After(timeout):
	}
	t.Error("call did not return; a non-regular file is being read without bound")
	if release != nil {
		release()
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.FailNow()
	}
}

// fifoWriter returns a release func that opens path's write end once, freeing
// a reader blocked in open(2) on the FIFO.
func fifoWriter(path string) func() {
	return func() {
		if f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
			f.Close()
		}
	}
}

func TestDetect_SkipsFIFOManifest(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "go.mod")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo not available here: %v", err)
	}

	d := graph.NewFrameworkDetector(graph.DefaultRules)
	runBounded(t, 2*time.Second, fifoWriter(fifo), func() {
		if got := d.Detect(root); len(got) != 0 {
			t.Errorf("Detect = %v, want no frameworks", got)
		}
	})
}

func TestDetect_SkipsDevZeroSymlinkManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("/dev/zero", filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	d := graph.NewFrameworkDetector(graph.DefaultRules)
	runBounded(t, 2*time.Second, nil, func() {
		if got := d.Detect(root); len(got) != 0 {
			t.Errorf("Detect = %v, want no frameworks", got)
		}
	})

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	// A symlinked /dev/zero read without the size cap grows heap without bound
	// (measured ~500 MiB in 150ms). Allow generous headroom for test noise.
	const maxGrowth = 64 << 20
	if after.HeapAlloc-before.HeapAlloc > maxGrowth {
		t.Errorf("Detect read an unbounded special file: heap grew by %d bytes", after.HeapAlloc-before.HeapAlloc)
	}
}

func TestBuildAliasIndex_SkipsFIFOTSConfig(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "pkg")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(pkg, "tsconfig.json")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo not available here: %v", err)
	}

	runBounded(t, 2*time.Second, fifoWriter(fifo), func() {
		if _, err := graph.BuildAliasIndex(root); err != nil {
			t.Errorf("BuildAliasIndex: %v", err)
		}
	})
}

func TestReadRegularFile(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "go.mod")
	if err := os.WriteFile(regular, []byte("module example.com/x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if data, err := graph.ReadRegularFile(regular, graph.MaxAuxFileSize); err != nil || string(data) != "module example.com/x\n" {
		t.Errorf("regular file: data=%q err=%v", data, err)
	}
	if _, err := graph.ReadRegularFile(filepath.Join(dir, "missing"), graph.MaxAuxFileSize); err == nil {
		t.Error("missing file: expected error")
	}
	if data, err := graph.ReadRegularFile(regular, 6); err == nil {
		t.Error("oversized file: expected error")
	} else if data != nil {
		t.Errorf("oversized file: data=%q, want nil", data)
	}

	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo not available here: %v", err)
	}
	if _, err := graph.ReadRegularFile(fifo, graph.MaxAuxFileSize); err == nil {
		t.Error("FIFO: expected error")
	}

	if err := os.Symlink("/dev/zero", filepath.Join(dir, "zero")); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.ReadRegularFile(filepath.Join(dir, "zero"), graph.MaxAuxFileSize); err == nil {
		t.Error("symlink to /dev/zero: expected error")
	}
}
