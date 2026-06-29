package storage

import "strings"

// SourceFromPath derives the agent source ("claude", "opencode", or "unknown")
// from a document source_path scheme. The harvest/summarize write paths encode
// the source in the scheme (e.g. "summary://claude/<id>", "opencode://session/<id>"),
// so this is the canonical, dependency-free way for read-side consumers (HTTP
// handlers, MCP tools) to recover the source without scanning tags.
func SourceFromPath(sourcePath string) string {
	switch {
	case strings.HasPrefix(sourcePath, "summary://claude/"):
		return "claude"
	case strings.HasPrefix(sourcePath, "summary://opencode/"):
		return "opencode"
	case strings.HasPrefix(sourcePath, "opencode://"):
		return "opencode"
	case strings.HasPrefix(sourcePath, "claude://"):
		return "claude"
	default:
		return "unknown"
	}
}
