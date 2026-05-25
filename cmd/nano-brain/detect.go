package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// detectOpenCodeStorageDir returns the first existing OpenCode storage
// directory found via env var or platform-specific well-known paths.
// Returns "" if nothing found. Pure: only reads env/stat, no writes.
func detectOpenCodeStorageDir() string {
	if v := os.Getenv("OPENCODE_STORAGE_DIR"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	for _, p := range platformOpenCodePaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func platformOpenCodePaths() []string {
	switch runtime.GOOS {
	case "linux":
		var paths []string
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			paths = append(paths, filepath.Join(xdg, "opencode", "storage"))
		}
		if home := os.Getenv("HOME"); home != "" {
			paths = append(paths, filepath.Join(home, ".local", "share", "opencode", "storage"))
		}
		return paths
	case "darwin":
		if home := os.Getenv("HOME"); home != "" {
			return []string{filepath.Join(home, "Library", "Application Support", "opencode", "storage")}
		}
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return []string{filepath.Join(appdata, "opencode", "storage")}
		}
	}
	return nil
}
