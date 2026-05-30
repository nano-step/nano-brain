package summarize

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const workspaceFallbackPrefix = "ws-"

// expandTilde resolves a leading "~/" in p to the user's home directory.
// Returns p unchanged when it does not start with "~/".
//
// Required because YAML config commonly uses "~/.nano-brain/summaries"
// and Go's os.Open does not expand tilde itself (issue #258).
func ExpandTilde(p string) (string, error) {
	if !strings.HasPrefix(p, "~/") && p != "~" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("expand tilde: %w", err)
	}
	if p == "~" {
		return home, nil
	}
	return filepath.Join(home, p[2:]), nil
}

// workspaceFolderName returns the directory name used for the workspace
// in the disk layout. Prefers the human-readable name from the workspaces
// table; falls back to "ws-<hash[:12]>" when name is empty.
//
// The name is also slugified to guarantee filesystem safety (handles names
// with spaces, slashes, or other special characters).
func WorkspaceFolderName(name, hash string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if len(hash) >= 12 {
			return workspaceFallbackPrefix + hash[:12]
		}
		return workspaceFallbackPrefix + hash
	}
	return Slugify(name)
}

// buildDiskPath returns the full filesystem path where a summary should
// be persisted. The path layout is:
//
//	<outputDir>/<workspaceFolder>/<source>_<slug-title>_<YYYY-MM-DD>.md
//
// outputDir MUST already be tilde-expanded (caller responsibility).
// All path components are sanitized to be filesystem-safe.
func BuildDiskPath(outputDir, workspaceName, workspaceHash, source, title string, date time.Time) string {
	ws := WorkspaceFolderName(workspaceName, workspaceHash)
	titleSlug := Slugify(title)
	dateStr := date.UTC().Format("2006-01-02")
	srcSlug := Slugify(source)
	filename := fmt.Sprintf("%s_%s_%s.md", srcSlug, titleSlug, dateStr)
	return filepath.Join(outputDir, ws, filename)
}
