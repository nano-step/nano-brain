package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveLauncherNanoBrainBin(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "nano-brain")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NANO_BRAIN_BIN", bin)
	t.Setenv("NANO_BRAIN_NPM_LAUNCHED", "true")
	t.Setenv("NANO_BRAIN_NPM_GLOBAL", "true")

	launcher, provenance, err := resolveLauncher()
	if err != nil {
		t.Fatalf("resolveLauncher: %v", err)
	}
	if len(launcher) != 1 || launcher[0] != bin {
		t.Errorf("launcher = %v, want [%s]", launcher, bin)
	}
	if provenance != "NANO_BRAIN_BIN" {
		t.Errorf("provenance = %q, want NANO_BRAIN_BIN", provenance)
	}
}

func TestResolveLauncherNanoBrainBinMissing(t *testing.T) {
	t.Setenv("NANO_BRAIN_BIN", "/nonexistent/nano-brain")
	if _, _, err := resolveLauncher(); err == nil || !strings.Contains(err.Error(), "NANO_BRAIN_BIN") {
		t.Errorf("err = %v, want NANO_BRAIN_BIN error", err)
	}
}

func TestResolveLauncherGlobalNpmWrapper(t *testing.T) {
	dir := t.TempDir()
	runjs := filepath.Join(dir, "npm", "run.js")
	if err := os.MkdirAll(filepath.Dir(runjs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runjs, []byte("#!/usr/bin/env node\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NANO_BRAIN_BIN", "")
	t.Setenv("NANO_BRAIN_NPM_RUNJS", runjs)
	t.Setenv("NANO_BRAIN_NPM_NODE", "/opt/homebrew/bin/node")
	t.Setenv("NANO_BRAIN_NPM_GLOBAL", "true")

	launcher, provenance, err := resolveLauncher()
	if err != nil {
		t.Fatalf("resolveLauncher: %v", err)
	}
	if len(launcher) != 2 || launcher[0] != "/opt/homebrew/bin/node" || launcher[1] != runjs {
		t.Errorf("launcher = %v, want [node run.js]", launcher)
	}
	if provenance != "global npm wrapper" {
		t.Errorf("provenance = %q", provenance)
	}
}

func TestResolveLauncherRejectsNpx(t *testing.T) {
	t.Setenv("NANO_BRAIN_BIN", "")
	t.Setenv("NANO_BRAIN_NPM_LAUNCHED", "true")
	t.Setenv("NANO_BRAIN_NPM_GLOBAL", "")
	if _, _, err := resolveLauncher(); err == nil || !strings.Contains(err.Error(), "global") {
		t.Errorf("err = %v, want migration message about global install", err)
	}
}

func TestResolveLauncherDirectBinary(t *testing.T) {
	t.Setenv("NANO_BRAIN_BIN", "")
	t.Setenv("NANO_BRAIN_NPM_LAUNCHED", "")
	t.Setenv("NANO_BRAIN_NPM_GLOBAL", "")
	launcher, provenance, err := resolveLauncher()
	if err != nil {
		t.Fatalf("resolveLauncher: %v", err)
	}
	if len(launcher) != 1 {
		t.Fatalf("launcher = %v, want single direct executable", launcher)
	}
	if provenance != "direct binary" {
		t.Errorf("provenance = %q, want direct binary", provenance)
	}
	if !isExecutableFile(launcher[0]) {
		t.Errorf("resolved launcher %q is not executable", launcher[0])
	}
}

func isExecutableFile(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0
}

func TestResolveInstalledConfig(t *testing.T) {
	t.Run("explicit path made absolute", func(t *testing.T) {
		got, err := resolveInstalledConfig("config.yml")
		if err != nil {
			t.Fatal(err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("resolveInstalledConfig returned non-absolute %q", got)
		}
		if !strings.HasSuffix(got, string(filepath.Separator)+"config.yml") {
			t.Errorf("resolveInstalledConfig = %q, want .../config.yml", got)
		}
	})
	t.Run("env var honored", func(t *testing.T) {
		t.Setenv("NANO_BRAIN_CONFIG", "/tmp/my-nano-brain.yml")
		got, err := resolveInstalledConfig("")
		if err != nil {
			t.Fatal(err)
		}
		if got != "/tmp/my-nano-brain.yml" {
			t.Errorf("resolveInstalledConfig = %q, want /tmp/my-nano-brain.yml", got)
		}
	})
}

func TestLauncherServeArgv(t *testing.T) {
	spec := serviceSpec{launcher: []string{"/usr/bin/node", "/npm/run.js"}, configPath: "/abs/config.yml"}
	got := launcherServeArgv(spec)
	want := []string{"/usr/bin/node", "/npm/run.js", "--config", "/abs/config.yml", "serve"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("launcherServeArgv = %v, want %v", got, want)
	}
}
