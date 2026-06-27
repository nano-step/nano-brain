package mcp

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// splitNodeSymbol splits a node identifier into its file part and optional ::symbol suffix.
// The separator "::" is the project convention (see internal/symbol).
// Returns (filePart, symbolSuffix) where symbolSuffix is empty when absent.
// symbolSuffix retains the leading "::" so the caller can recompose with a simple concat.
func splitNodeSymbol(node string) (string, string) {
	if idx := strings.Index(node, "::"); idx >= 0 {
		return node[:idx], node[idx:]
	}
	return node, ""
}

// resolveNodeAgainstWorkspace canonicalizes a node identifier to its
// workspace-relative form (e.g. "internal/storage/migrate.go::RunMigrations"),
// matching how stored source/target nodes are written since #450.
//
// Absolute inputs are stripped of the workspace root (so callers that still
// pass absolute paths intersect the relative storage). Already-relative inputs
// and non-path tokens (extensionless import specifiers like "context") are
// returned unchanged, so the function is safe to call unconditionally.
//
// An invalid workspace hash returns an error so the caller surfaces a clear
// message to the agent instead of silently returning zero rows. The lookup is
// only performed for absolute inputs that actually need stripping.
func resolveNodeAgainstWorkspace(ctx context.Context, queries *sqlc.Queries, workspaceHash, node string) (string, error) {
	filePart, symbolSuffix := splitNodeSymbol(node)
	if path.Ext(filePart) == "" {
		return node, nil
	}
	if !path.IsAbs(filePart) {
		return node, nil
	}
	ws, err := queries.GetWorkspaceByHash(ctx, workspaceHash)
	if err != nil {
		return "", fmt.Errorf("workspace lookup failed: %w", err)
	}
	return stripWorkspacePrefix(ws.Path, filePart) + symbolSuffix, nil
}

// stripWorkspacePrefix removes the workspace root prefix from a node identifier
// so callers receive workspace-relative paths. Tokens that do not start with
// the workspace root (e.g. import paths like "context", external symbols)
// are returned unchanged.
func stripWorkspacePrefix(workspaceRoot, node string) string {
	if workspaceRoot == "" {
		return node
	}
	prefix := workspaceRoot
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	if !strings.HasPrefix(node, prefix) {
		return node
	}
	return node[len(prefix):]
}

// lookupWorkspaceRoot returns the absolute filesystem root for a workspace hash,
// or empty string when the lookup fails. Callers use the result with
// stripWorkspacePrefix; an empty root makes strip a no-op.
func lookupWorkspaceRoot(ctx context.Context, queries *sqlc.Queries, workspaceHash string) string {
	ws, err := queries.GetWorkspaceByHash(ctx, workspaceHash)
	if err != nil {
		return ""
	}
	return ws.Path
}
