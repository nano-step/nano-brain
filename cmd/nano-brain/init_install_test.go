package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func withChdir(t *testing.T, dir string) {
	t.Helper()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("restore chdir to %q: %v", orig, err)
		}
	})
}

func TestRunInitCmd_InstallOpenCodeTarget(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	withChdir(t, workspaceRoot)
	t.Setenv("HOME", homeDir)

	homeConfigDir := filepath.Join(homeDir, ".nano-brain")
	if err := os.MkdirAll(homeConfigDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", homeConfigDir, err)
	}
	homeConfigPath := filepath.Join(homeConfigDir, "config.yml")
	sentinel := []byte("database:\n  url: postgres://sentinel\n")
	if err := os.WriteFile(homeConfigPath, sentinel, 0o644); err != nil {
		t.Fatalf("seed home config: %v", err)
	}

	out := captureStdout(t, func() {
		runInitCmd([]string{"--", "opencode"}, "")
	})

	installPath := filepath.Join(workspaceRoot, ".opencode", "commands", "nano-brain.md")
	data, err := os.ReadFile(installPath)
	if err != nil {
		t.Fatalf("read installed command: %v", err)
	}
	if !bytes.Contains(data, []byte(".nanobrain/config.yml")) {
		t.Fatalf("installed command missing workspace-local config path:\n%s", data)
	}
	if !bytes.Contains(data, []byte("~/.nano-brain/config.yml")) {
		t.Fatalf("installed command missing home-config seed reference:\n%s", data)
	}
	if !strings.Contains(out, installPath) {
		t.Fatalf("stdout %q does not mention install path %q", out, installPath)
	}
	if !strings.Contains(out, "Run /nano-brain . in OpenCode") {
		t.Fatalf("stdout %q does not mention the OpenCode bootstrap hint", out)
	}

	homeAfter, err := os.ReadFile(homeConfigPath)
	if err != nil {
		t.Fatalf("read home config after install: %v", err)
	}
	if !bytes.Equal(homeAfter, sentinel) {
		t.Fatalf("home config changed during install:\nbefore=%s\nafter=%s", sentinel, homeAfter)
	}
}

func TestInstallInitTarget_UnsupportedTarget(t *testing.T) {
	workspaceRoot := t.TempDir()
	_, err := installInitTarget("imaginary-agent", workspaceRoot)
	if err == nil {
		t.Fatal("installInitTarget() error = nil, want unsupported target error")
	}
	if !strings.Contains(err.Error(), "unsupported init target") {
		t.Fatalf("installInitTarget() error = %v, want unsupported-target message", err)
	}
}

func TestInstallOpenCodeBootstrap_Idempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test only checks byte-stable file writes and path handling on unix-like temp dirs")
	}
	workspaceRoot := t.TempDir()
	withChdir(t, workspaceRoot)

	first, err := installOpenCodeBootstrap(workspaceRoot)
	if err != nil {
		t.Fatalf("first installOpenCodeBootstrap() error = %v", err)
	}
	firstBytes, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read first bootstrap: %v", err)
	}

	second, err := installOpenCodeBootstrap(workspaceRoot)
	if err != nil {
		t.Fatalf("second installOpenCodeBootstrap() error = %v", err)
	}
	secondBytes, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read second bootstrap: %v", err)
	}

	if first != second {
		t.Fatalf("install paths differ across runs: first=%q second=%q", first, second)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("bootstrap content changed across runs:\nfirst=%s\nsecond=%s", firstBytes, secondBytes)
	}
}
