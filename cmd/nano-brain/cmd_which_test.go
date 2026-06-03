package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveBinarySource_EnvOverride(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "nano-brain-*")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	absPath := tmp.Name()

	t.Setenv("NANO_BRAIN_BIN", absPath)

	gotPath, gotSource, err := resolveBinarySource(absPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSource != "env-override" {
		t.Errorf("source = %q, want env-override", gotSource)
	}
	if gotPath != absPath {
		t.Errorf("path = %q, want %q", gotPath, absPath)
	}
}

func TestResolveBinarySource_NpmLocal(t *testing.T) {
	t.Setenv("NANO_BRAIN_BIN", "")
	t.Setenv("npm_execpath", "/usr/local/lib/node_modules/npm/bin/npm-cli.js")

	fakePath := filepath.Join(t.TempDir(), "node_modules", ".bin", "nano-brain")
	if err := os.MkdirAll(filepath.Dir(fakePath), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(fakePath)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	_, gotSource, err := resolveBinarySource(fakePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSource != "npm-local" {
		t.Errorf("source = %q, want npm-local", gotSource)
	}
}

func TestResolveBinarySource_DevBuild(t *testing.T) {
	t.Setenv("NANO_BRAIN_BIN", "")
	t.Setenv("npm_execpath", "")

	origVersion := Version
	Version = "dev"
	defer func() { Version = origVersion }()

	tmp, err := os.CreateTemp(t.TempDir(), "nano-brain-*")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	_, gotSource, err := resolveBinarySource(tmp.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSource != "dev-build" {
		t.Errorf("source = %q, want dev-build", gotSource)
	}
}

func TestResolveBinarySource_Path(t *testing.T) {
	t.Setenv("NANO_BRAIN_BIN", "")
	t.Setenv("npm_execpath", "")

	origVersion := Version
	Version = "v1.2.3"
	defer func() { Version = origVersion }()

	tmp, err := os.CreateTemp(t.TempDir(), "nano-brain-*")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	_, gotSource, err := resolveBinarySource(tmp.Name())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotSource != "path" {
		t.Errorf("source = %q, want path", gotSource)
	}
}
