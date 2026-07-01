package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func clearOpenCodeEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OPENCODE_STORAGE_DIR", "")
	t.Setenv("OPENCODE_DB_PATH", "")
	t.Setenv("OPENCODE_DB_ROOT", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("APPDATA", "")
}

func TestDetectOpenCodeStorageDir_EnvVar(t *testing.T) {
	clearOpenCodeEnv(t)
	dir := t.TempDir()
	t.Setenv("OPENCODE_STORAGE_DIR", dir)

	got := detectOpenCodeStorageDir()
	if got != dir {
		t.Errorf("detectOpenCodeStorageDir() = %q, want %q", got, dir)
	}
}

func TestDetectOpenCodeStorageDir_EnvVarMissing(t *testing.T) {
	clearOpenCodeEnv(t)
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("OPENCODE_STORAGE_DIR", missing)

	got := detectOpenCodeStorageDir()
	if got == missing {
		t.Errorf("detectOpenCodeStorageDir() returned non-existent env path %q", got)
	}
	if got != "" {
		t.Errorf("detectOpenCodeStorageDir() = %q, want \"\" (no other paths set)", got)
	}
}

func TestDetectOpenCodeStorageDir_XDGDataHome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_DATA_HOME only used on linux")
	}
	clearOpenCodeEnv(t)
	xdg := t.TempDir()
	want := filepath.Join(xdg, "opencode", "storage")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("XDG_DATA_HOME", xdg)

	got := detectOpenCodeStorageDir()
	if got != want {
		t.Errorf("detectOpenCodeStorageDir() = %q, want %q", got, want)
	}
}

func TestDetectOpenCodeStorageDir_HomeLinuxFallback(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only path")
	}
	clearOpenCodeEnv(t)
	home := t.TempDir()
	want := filepath.Join(home, ".local", "share", "opencode", "storage")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("HOME", home)

	got := detectOpenCodeStorageDir()
	if got != want {
		t.Errorf("detectOpenCodeStorageDir() = %q, want %q", got, want)
	}
}

func TestDetectOpenCodeStorageDir_HomeMac(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only path")
	}
	clearOpenCodeEnv(t)
	home := t.TempDir()
	want := filepath.Join(home, "Library", "Application Support", "opencode", "storage")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("HOME", home)

	got := detectOpenCodeStorageDir()
	if got != want {
		t.Errorf("detectOpenCodeStorageDir() = %q, want %q", got, want)
	}
}

func TestDetectOpenCodeStorageDir_NoneFound(t *testing.T) {
	clearOpenCodeEnv(t)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())

	got := detectOpenCodeStorageDir()
	if got != "" {
		t.Errorf("detectOpenCodeStorageDir() = %q, want \"\"", got)
	}
}

func TestDetectOpenCodeStorageDir_EnvVarPriority(t *testing.T) {
	clearOpenCodeEnv(t)
	envDir := t.TempDir()
	t.Setenv("OPENCODE_STORAGE_DIR", envDir)

	switch runtime.GOOS {
	case "linux":
		xdg := t.TempDir()
		if err := os.MkdirAll(filepath.Join(xdg, "opencode", "storage"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv("XDG_DATA_HOME", xdg)
	case "darwin":
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, "Library", "Application Support", "opencode", "storage"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv("HOME", home)
	case "windows":
		appdata := t.TempDir()
		if err := os.MkdirAll(filepath.Join(appdata, "opencode", "storage"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv("APPDATA", appdata)
	}

	got := detectOpenCodeStorageDir()
	if got != envDir {
		t.Errorf("detectOpenCodeStorageDir() = %q, want %q (env var must take priority)", got, envDir)
	}
}

func TestDetectOpenCodeDBRoot_EnvVar(t *testing.T) {
	clearOpenCodeEnv(t)
	dir := t.TempDir()
	t.Setenv("OPENCODE_DB_ROOT", dir)

	got := detectOpenCodeDBRoot()
	if got != dir {
		t.Errorf("detectOpenCodeDBRoot() = %q, want %q", got, dir)
	}
}

func TestDetectOpenCodeDBRoot_EnvVarMissing(t *testing.T) {
	clearOpenCodeEnv(t)
	t.Setenv("OPENCODE_DB_ROOT", "/nonexistent/path/should/not/exist")

	got := detectOpenCodeDBRoot()
	if got == "/nonexistent/path/should/not/exist" {
		t.Errorf("detectOpenCodeDBRoot() returned non-existent env path %q", got)
	}
	if got != "" {
		t.Errorf("detectOpenCodeDBRoot() = %q, want \"\" (no other paths set)", got)
	}
}

func TestDetectOpenCodeDBRoot_EnvVarFileNotDir(t *testing.T) {
	clearOpenCodeEnv(t)
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("regular file"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	t.Setenv("OPENCODE_DB_ROOT", filePath)

	got := detectOpenCodeDBRoot()
	if got != "" {
		t.Errorf("detectOpenCodeDBRoot() = %q, want \"\" (file not dir must be rejected)", got)
	}
}

func TestDetectOpenCodeDBRoot_PlatformDefault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows has no platform default for db_root")
	}
	clearOpenCodeEnv(t)
	home := t.TempDir()
	want := filepath.Join(home, ".ai-sandbox", "opencode-dbs")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Setenv("HOME", home)

	got := detectOpenCodeDBRoot()
	if got != want {
		t.Errorf("detectOpenCodeDBRoot() = %q, want %q", got, want)
	}
}

func TestDetectOpenCodeDBRoot_NoneFound(t *testing.T) {
	clearOpenCodeEnv(t)
	t.Setenv("HOME", t.TempDir())

	got := detectOpenCodeDBRoot()
	if got != "" {
		t.Errorf("detectOpenCodeDBRoot() = %q, want \"\"", got)
	}
}

func TestDetectOpenCodeDBRoot_EnvVarPriority(t *testing.T) {
	clearOpenCodeEnv(t)
	envDir := t.TempDir()
	t.Setenv("OPENCODE_DB_ROOT", envDir)

	if runtime.GOOS != "windows" {
		home := t.TempDir()
		if err := os.MkdirAll(filepath.Join(home, ".ai-sandbox", "opencode-dbs"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv("HOME", home)
	}

	got := detectOpenCodeDBRoot()
	if got != envDir {
		t.Errorf("detectOpenCodeDBRoot() = %q, want %q (env var must take priority)", got, envDir)
	}
}

func TestPlatformOpenCodeDBRootPaths_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only path check")
	}
	clearOpenCodeEnv(t)
	got := platformOpenCodeDBRootPaths()
	if got != nil {
		t.Errorf("platformOpenCodeDBRootPaths() on windows = %v, want nil (no default)", got)
	}
}

func TestDetectClaudeCodeConfigPath(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, ".mcp.json")
	got := detectClaudeCodeConfigPath(root)
	if got != want {
		t.Errorf("detectClaudeCodeConfigPath(%q) = %q, want %q", root, got, want)
	}
}

func TestDetectOpenCodeConfigPath_ProjectLocalDefault(t *testing.T) {
	t.Setenv("OPENCODE_CONFIG", "")
	root := t.TempDir()
	want := filepath.Join(root, "opencode.json")
	got := detectOpenCodeConfigPath(root)
	if got != want {
		t.Errorf("detectOpenCodeConfigPath(%q) = %q, want %q", root, got, want)
	}
}

func TestDetectOpenCodeConfigPath_EnvOverride(t *testing.T) {
	root := t.TempDir()
	envFile := filepath.Join(t.TempDir(), "custom-opencode.json")
	if err := os.WriteFile(envFile, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}
	t.Setenv("OPENCODE_CONFIG", envFile)

	got := detectOpenCodeConfigPath(root)
	if got != envFile {
		t.Errorf("detectOpenCodeConfigPath(%q) = %q, want %q (env override)", root, got, envFile)
	}
}

func TestDetectOpenCodeConfigPath_EnvOverrideMissingFallsBack(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	t.Setenv("OPENCODE_CONFIG", missing)

	want := filepath.Join(root, "opencode.json")
	got := detectOpenCodeConfigPath(root)
	if got != want {
		t.Errorf("detectOpenCodeConfigPath(%q) = %q, want %q (env points at nonexistent file, should fall back)", root, got, want)
	}
}

func TestDetectCodexConfigPath_Default(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)

	want := filepath.Join(home, ".codex", "config.toml")
	got := detectCodexConfigPath()
	if got != want {
		t.Errorf("detectCodexConfigPath() = %q, want %q", got, want)
	}
}

func TestDetectCodexConfigPath_CodexHomeOverride(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", t.TempDir())

	want := filepath.Join(codexHome, "config.toml")
	got := detectCodexConfigPath()
	if got != want {
		t.Errorf("detectCodexConfigPath() = %q, want %q (CODEX_HOME override)", got, want)
	}
}

func TestDetectCodexConfigPath_CodexHomeMissingFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "does-not-exist"))

	want := filepath.Join(home, ".codex", "config.toml")
	got := detectCodexConfigPath()
	if got != want {
		t.Errorf("detectCodexConfigPath() = %q, want %q (nonexistent CODEX_HOME should fall back)", got, want)
	}
}
