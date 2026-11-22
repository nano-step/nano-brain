//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestWaitDaemonStart(t *testing.T) {
	// after(n) reports true from its nth call on; never() is always false.
	after := func(n int) func() bool {
		calls := 0
		return func() bool { calls++; return calls >= n }
	}
	never := func() bool { return false }

	tests := []struct {
		name        string
		childExited func() bool
		healthy     func() bool
		want        daemonStartOutcome
	}{
		{"child dies immediately", after(1), never, daemonStartExited},
		{"child dies after a few polls", after(3), never, daemonStartExited},
		{"server answers", never, after(2), daemonStartHealthy},
		{"dead child wins over an answering port", after(1), after(1), daemonStartExited},
		{"child dies before the server would answer", after(2), after(3), daemonStartExited},
		{"neither within the window", never, never, daemonStartUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := waitDaemonStart(tt.childExited, tt.healthy, 50*time.Millisecond, time.Millisecond)
			if got != tt.want {
				t.Errorf("waitDaemonStart() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDaemonLogTail(t *testing.T) {
	const earlier = "old run line 1\nold run line 2\n"
	tests := []struct {
		name    string
		written string
		n       int
		want    []string
	}{
		{"nothing written", "", 5, nil},
		{"fewer lines than n", "a\nb\n", 5, []string{"a", "b"}},
		{"more lines than n", "a\nb\nc\nd\n", 2, []string{"c", "d"}},
		{"blank lines and CRLF dropped", "a\r\n\n\nb", 5, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nano-brain.log")
			if err := os.WriteFile(path, []byte(earlier+tt.written), 0o600); err != nil {
				t.Fatal(err)
			}
			got := daemonLogTail(path, int64(len(earlier)), tt.n)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("daemonLogTail() = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("missing file", func(t *testing.T) {
		if got := daemonLogTail(filepath.Join(t.TempDir(), "absent.log"), 0, 5); got != nil {
			t.Errorf("daemonLogTail() = %q, want nil", got)
		}
	})
	t.Run("file shorter than the start offset", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nano-brain.log")
		if err := os.WriteFile(path, []byte("a\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := daemonLogTail(path, 100, 5); got != nil {
			t.Errorf("daemonLogTail() = %q, want nil", got)
		}
	})
}

func TestReportDaemonStartFailure_RemovesPIDFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".nano-brain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	pidPath := filepath.Join(dir, "nano-brain.pid")
	if err := os.WriteFile(pidPath, []byte("4321\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "nano-brain.log")

	reportDaemonStartFailure(4321, logPath, 0)

	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("PID file still exists after child death: %v", err)
	}
}
