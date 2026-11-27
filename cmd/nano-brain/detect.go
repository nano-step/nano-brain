package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// detectClaudeCodeStoragePatterns returns stable Claude Code projects-parent
// patterns worth harvesting from: the CLAUDECODE_STORAGE_DIR override,
// platform candidates whose Claude config home/projects dir exists, and CCS
// instance globs for every known CCS root. CCS root candidates are scanned as
// a union in precedence order because harvesting is retrospective — users may
// have historical sessions under more than one root. Results are deduplicated
// in first-seen order.
func detectClaudeCodeStoragePatterns() []string {
	var patterns []string
	seen := make(map[string]struct{})
	add := func(p string) {
		if p == "" {
			return
		}
		p = filepath.Clean(p)
		if _, dup := seen[p]; dup {
			return
		}
		seen[p] = struct{}{}
		patterns = append(patterns, p)
	}

	if v := os.Getenv("CLAUDECODE_STORAGE_DIR"); v != "" {
		if info, err := os.Stat(v); err == nil && info.IsDir() {
			add(v)
		}
	}
	for _, p := range platformClaudeCodePaths() {
		// Include a platform projects dir even before the first session if its
		// Claude config parent already exists; dynamic discovery can pick up the
		// projects dir when Claude creates it later.
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			add(p)
			continue
		}
		if info, err := os.Stat(filepath.Dir(p)); err == nil && info.IsDir() {
			add(p)
		}
	}
	for _, candidate := range ccsRootCandidates() {
		if !candidate.explicit {
			if info, err := os.Stat(candidate.root); err != nil || !info.IsDir() {
				continue
			}
		}
		add(filepath.Join(candidate.root, "instances", "*", "projects"))
	}
	return patterns
}

type ccsRootCandidate struct {
	root     string
	explicit bool
}

// ccsRootCandidates returns known CCS config roots in precedence order:
// CCS_DIR, then CCS_HOME/.ccs, then the default home .ccs. Relative env paths
// are ignored rather than guessed against the daemon process cwd; operators can
// use absolute paths or explicit session_dirs for those cases.
func ccsRootCandidates() []ccsRootCandidate {
	var out []ccsRootCandidate
	seen := make(map[string]struct{})
	add := func(root string, explicit bool) {
		normalized, ok := normalizeCCSRoot(root)
		if !ok {
			return
		}
		if _, dup := seen[normalized]; dup {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, ccsRootCandidate{root: normalized, explicit: explicit})
	}

	if v := os.Getenv("CCS_DIR"); v != "" {
		add(v, true)
	}
	if v := os.Getenv("CCS_HOME"); v != "" {
		if root, ok := normalizeCCSRoot(v); ok {
			add(filepath.Join(root, ".ccs"), true)
		}
	}
	if home := userHomeDir(); home != "" {
		add(filepath.Join(home, ".ccs"), false)
	}
	return out
}

func normalizeCCSRoot(root string) (string, bool) {
	if root == "" {
		return "", false
	}
	if root == "~" || strings.HasPrefix(root, "~/") || strings.HasPrefix(root, `~\`) {
		home := userHomeDir()
		if home == "" {
			return "", false
		}
		if root == "~" {
			root = home
		} else {
			root = filepath.Join(home, root[2:])
		}
	}
	if !filepath.IsAbs(root) {
		return "", false
	}
	return filepath.Clean(root), true
}

func userHomeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return os.Getenv("USERPROFILE")
}

func platformClaudeCodePaths() []string {
	home := userHomeDir()
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

// detectClaudeCodeConfigPath returns the project-local Claude Code MCP
// config path (.mcp.json in the project root). Claude Code has no env-var
// override or platform candidate list for this file — it is always
// project-root-relative (RESEARCH Code Examples). The candidate path is
// returned even when it does not yet exist; absence does not mean the
// client isn't installed.
func detectClaudeCodeConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".mcp.json")
}

// detectOpenCodeConfigPath returns the project-local OpenCode MCP config
// path (opencode.json in the project root), honoring an OPENCODE_CONFIG
// env override when it points at a file that exists. Project-local config
// has the highest precedence per OpenCode docs. The candidate path is
// returned even when it does not yet exist.
func detectOpenCodeConfigPath(projectRoot string) string {
	if v := os.Getenv("OPENCODE_CONFIG"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	return filepath.Join(projectRoot, "opencode.json")
}

// detectCodexConfigPath returns the global Codex CLI config path
// (~/.codex/config.toml), honoring a CODEX_HOME env override when it
// points at a directory that exists. Global config is used deliberately
// (not project-scoped .codex/config.toml) to avoid Codex's trusted-project
// gate silently voiding an auto-written config (RESEARCH Pitfall 2).
func detectCodexConfigPath() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		if info, err := os.Stat(v); err == nil && info.IsDir() {
			return filepath.Join(v, "config.toml")
		}
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE") // windows fallback
	}
	return filepath.Join(home, ".codex", "config.toml")
}

// detectCodexProjectConfigPath returns the project-scoped Codex CLI config
// path (.codex/config.toml in the project root). Codex only loads a
// project-scoped config from a TRUSTED directory, so callers must warn the
// user to trust the project; the tradeoff buys a per-project ?workspace=
// binding that the single global config cannot carry.
func detectCodexProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".codex", "config.toml")
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
