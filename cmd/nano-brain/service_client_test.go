package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartManagedServiceIfRegistered(t *testing.T) {
	origPlatform := newServicePlatformFn
	origHealthy := serviceHealthyWaitFn
	t.Cleanup(func() {
		newServicePlatformFn = origPlatform
		serviceHealthyWaitFn = origHealthy
	})
	serviceHealthyWaitFn = func(time.Duration) error { return nil }

	t.Run("no definition falls back to legacy", func(t *testing.T) {
		home := t.TempDir()
		withHome(t, home)
		runner := &fakeRunner{}
		newServicePlatformFn = func() servicePlatform { return newLaunchdPlatform(runner) }
		managed, started := startManagedServiceIfRegistered()
		if managed || started {
			t.Errorf("got (%v, %v), want (false, false)", managed, started)
		}
		if len(runner.calls) != 0 {
			t.Errorf("no manager calls expected, got %v", runner.commandList())
		}
	})

	t.Run("registered service restarted through manager", func(t *testing.T) {
		home := t.TempDir()
		withHome(t, home)
		runner := &fakeRunner{}
		newServicePlatformFn = func() servicePlatform { return newLaunchdPlatform(runner) }
		defPath := newLaunchdPlatform(runner).definitionPath()
		if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(defPath, []byte("plist"), 0o644); err != nil {
			t.Fatal(err)
		}

		managed, started := startManagedServiceIfRegistered()
		if !managed || !started {
			t.Errorf("got (%v, %v), want (true, true)", managed, started)
		}
		cmds := runner.commandList()
		if len(cmds) != 3 || !strings.Contains(cmds[2], "kickstart") {
			t.Errorf("register sequence expected, got %v", cmds)
		}
	})

	t.Run("manager failure must not fall back", func(t *testing.T) {
		home := t.TempDir()
		withHome(t, home)
		runner := &fakeRunner{fn: func(argv []string) (string, string, error) {
			if argv[1] == "bootstrap" {
				return "", "bootstrap failed", errors.New("exit status 1")
			}
			return "", "", nil
		}}
		newServicePlatformFn = func() servicePlatform { return newLaunchdPlatform(runner) }
		defPath := newLaunchdPlatform(runner).definitionPath()
		if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(defPath, []byte("plist"), 0o644); err != nil {
			t.Fatal(err)
		}

		managed, started := startManagedServiceIfRegistered()
		if !managed || started {
			t.Errorf("got (%v, %v), want (true, false) — recovery must not start legacy", managed, started)
		}
	})

	t.Run("restarted but not healthy", func(t *testing.T) {
		home := t.TempDir()
		withHome(t, home)
		runner := &fakeRunner{}
		newServicePlatformFn = func() servicePlatform { return newLaunchdPlatform(runner) }
		serviceHealthyWaitFn = func(time.Duration) error { return errors.New("not healthy") }
		defPath := newLaunchdPlatform(runner).definitionPath()
		if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(defPath, []byte("plist"), 0o644); err != nil {
			t.Fatal(err)
		}

		managed, started := startManagedServiceIfRegistered()
		if !managed || started {
			t.Errorf("got (%v, %v), want (true, false)", managed, started)
		}
	})

	t.Run("unusable platform falls back", func(t *testing.T) {
		newServicePlatformFn = func() servicePlatform { return unsupportedPlatform{} }
		managed, started := startManagedServiceIfRegistered()
		if managed || started {
			t.Errorf("got (%v, %v), want (false, false)", managed, started)
		}
	})
}

func TestRecoverFromConnectionRefusedManagedPath(t *testing.T) {
	// The managed path must be taken BEFORE any TTY/prompt gate: a registered
	// service restarts through its supervisor without an interactive prompt.
	origPlatform := newServicePlatformFn
	origHealthy := serviceHealthyWaitFn
	origDaemon := runServeDaemonFn
	origIsTTY := isTTYFn
	t.Cleanup(func() {
		newServicePlatformFn = origPlatform
		serviceHealthyWaitFn = origHealthy
		runServeDaemonFn = origDaemon
		isTTYFn = origIsTTY
	})
	serviceHealthyWaitFn = func(time.Duration) error { return nil }
	isTTYFn = func() bool { return false } // non-TTY must still restart a managed service
	runServeDaemonFn = func(string) { t.Fatal("legacy daemon must not launch when a managed service is registered") }

	home := t.TempDir()
	withHome(t, home)
	runner := &fakeRunner{}
	newServicePlatformFn = func() servicePlatform { return newLaunchdPlatform(runner) }
	defPath := newLaunchdPlatform(runner).definitionPath()
	if err := os.MkdirAll(filepath.Dir(defPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defPath, []byte("plist"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := recoverFromConnectionRefused("localhost", 3100); !got {
		t.Error("managed recovery should report success when the service restarted healthy")
	}
	cmds := runner.commandList()
	if len(cmds) != 3 {
		t.Errorf("expected the manager register sequence, got %v", cmds)
	}
}

func TestRecoverFromConnectionRefusedLegacyFallback(t *testing.T) {
	origPlatform := newServicePlatformFn
	origHealthy := serviceHealthyWaitFn
	origDaemon := runServeDaemonFn
	origIsTTY := isTTYFn
	origReader := promptReader
	origWriter := promptWriter
	t.Cleanup(func() {
		newServicePlatformFn = origPlatform
		serviceHealthyWaitFn = origHealthy
		runServeDaemonFn = origDaemon
		isTTYFn = origIsTTY
		promptReader = origReader
		promptWriter = origWriter
	})
	serviceHealthyWaitFn = func(time.Duration) error { return nil }
	isTTYFn = func() bool { return true }
	promptReader = bytes.NewBufferString("Y\n")
	promptWriter = &bytes.Buffer{}
	runServeDaemonFn = func(string) {}
	newServicePlatformFn = func() servicePlatform { return newLaunchdPlatform(&fakeRunner{}) }

	withHome(t, t.TempDir()) // no definition file → legacy path
	if got := recoverFromConnectionRefused("localhost", 3100); !got {
		t.Error("legacy recovery should report success when the daemon started healthy")
	}
}
