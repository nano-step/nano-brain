package main

import (
	"os"
	"path/filepath"
	"strings"
)

// resolveBinarySource returns (absPath, source, error).
// source categories (in precedence order):
//   "env-override"  — NANO_BRAIN_BIN is set and non-empty, and absPath equals it
//   "npm-local"     — npm_execpath != "" AND (path contains "/node_modules/.bin/" OR "/node_modules/@nano-step/nano-brain/")
//   "npm-global"    — path contains one of the global npm lib dirs
//   "dev-build"     — Version == "dev"
//   "path"          — fallback
func resolveBinarySource(execPath string) (absPath string, source string, err error) {
	absPath, err = filepath.Abs(execPath)
	if err != nil {
		return "", "", err
	}

	if envBin := os.Getenv("NANO_BRAIN_BIN"); envBin != "" {
		envAbs, absErr := filepath.Abs(envBin)
		if absErr == nil && absPath == envAbs {
			return absPath, "env-override", nil
		}
	}

	if os.Getenv("npm_execpath") != "" {
		if strings.Contains(absPath, "/node_modules/.bin/") ||
			strings.Contains(absPath, "/node_modules/@nano-step/nano-brain/") {
			return absPath, "npm-local", nil
		}
	}

	globalMarkers := []string{
		"/usr/local/lib/node_modules/",
		"/.npm-global/",
		"/.local/lib/node_modules/",
		"/lib/node_modules/@nano-step",
	}
	for _, marker := range globalMarkers {
		if strings.Contains(absPath, marker) {
			return absPath, "npm-global", nil
		}
	}

	if Version == "dev" {
		return absPath, "dev-build", nil
	}

	return absPath, "path", nil
}

// resolveBinaryPath returns the absolute path to the running binary.
// Uses os.Executable() as the basis.
func resolveBinaryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	abs, err := filepath.EvalSymlinks(exe)
	if err != nil {
		abs = exe
	}
	return abs
}
