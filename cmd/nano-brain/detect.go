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

func detectClaudeCodeStorageDir() string {
	if v := os.Getenv("CLAUDECODE_STORAGE_DIR"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	for _, p := range platformClaudeCodePaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func platformClaudeCodePaths() []string {
	home := os.Getenv("HOME")
	if home == "" {
		return nil
	}
	candidates := []string{
		filepath.Join(home, ".claude", "projects"),
	}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			filepath.Join(home, "Library", "Application Support", "Claude", "projects"),
		)
	case "linux":
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			candidates = append(candidates, filepath.Join(xdg, "claude", "projects"))
		}
		candidates = append(candidates, filepath.Join(home, ".local", "share", "claude", "projects"))
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			candidates = append(candidates, filepath.Join(appdata, "Claude", "projects"))
		}
	}
	return candidates
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

func detectOpenCodeDBPath() string {
	if v := os.Getenv("OPENCODE_DB_PATH"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	for _, p := range platformOpenCodeDBPaths() {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func platformOpenCodeDBPaths() []string {
	switch runtime.GOOS {
	case "linux":
		var paths []string
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			paths = append(paths, filepath.Join(xdg, "opencode", "opencode.db"))
		}
		if home := os.Getenv("HOME"); home != "" {
			paths = append(paths, filepath.Join(home, ".local", "share", "opencode", "opencode.db"))
		}
		return paths
	case "darwin":
		if home := os.Getenv("HOME"); home != "" {
			return []string{
				filepath.Join(home, ".local", "share", "opencode", "opencode.db"),
				filepath.Join(home, "Library", "Application Support", "opencode", "opencode.db"),
			}
		}
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return []string{filepath.Join(appdata, "opencode", "opencode.db")}
		}
	}
	return nil
}

// detectOpenCodeDBRoot returns the first existing OpenCode per-project DB
// root directory found via env var or platform-specific well-known paths.
// The path must exist AND be a directory; files at the path are rejected.
// Returns "" if nothing found. Pure: only reads env/stat, no writes.
func detectOpenCodeDBRoot() string {
	if v := os.Getenv("OPENCODE_DB_ROOT"); v != "" {
		if info, err := os.Stat(v); err == nil && info.IsDir() {
			return v
		}
	}
	for _, p := range platformOpenCodeDBRootPaths() {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return ""
}

func platformOpenCodeDBRootPaths() []string {
	switch runtime.GOOS {
	case "linux", "darwin":
		if home := os.Getenv("HOME"); home != "" {
			return []string{filepath.Join(home, ".ai-sandbox", "opencode-dbs")}
		}
	}
	return nil
}
