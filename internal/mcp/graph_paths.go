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

// normalizeNodeForQuery normalizes a node identifier for DB queries.
// The DB stores workspace-relative paths (e.g. "tradeit/composables/cart/useCart.js").
// If the agent passes an absolute path, strip the workspace root prefix.
// If already relative, return as-is.
func normalizeNodeForQuery(ctx context.Context, queries *sqlc.Queries, workspaceHash, node string) (string, error) {
	filePart, symbolSuffix := splitNodeSymbol(node)

	if path.IsAbs(filePart) {
		ws, err := queries.GetWorkspaceByHash(ctx, workspaceHash)
		if err != nil {
			return "", fmt.Errorf("workspace lookup failed: %w", err)
		}
		normalized := stripWorkspacePrefix(ws.Path, filePart)
		return normalized + symbolSuffix, nil
	}

	return node, nil
}

// Deprecated: resolveNodeAgainstWorkspace converts relative paths to absolute,
// which is the OPPOSITE of what the DB stores. Use normalizeNodeForQuery instead.
func resolveNodeAgainstWorkspace(ctx context.Context, queries *sqlc.Queries, workspaceHash, node string) (string, error) {
	filePart, symbolSuffix := splitNodeSymbol(node)
	if path.IsAbs(filePart) {
		return node, nil
	}
	if path.Ext(filePart) == "" {
		return node, nil
	}
	ws, err := queries.GetWorkspaceByHash(ctx, workspaceHash)
	if err != nil {
		return "", fmt.Errorf("workspace lookup failed: %w", err)
	}
	return path.Join(ws.Path, filePart) + symbolSuffix, nil
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

// sharedPathDepth counts the leading path segments two workspace-relative file
// paths have in common ("a/b/x.go" vs "a/b/y.go" -> 2; "a/x.go" vs "c/y.go" -> 0).
func sharedPathDepth(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	n := 0
	for n < len(as) && n < len(bs) && as[n] == bs[n] {
		n++
	}
	return n
}

// nearestSymbolMatch disambiguates same-named symbol candidates by directory
// proximity to callerFile: it returns the single candidate that shares the
// strictly-deepest path prefix with the caller, or ("", false) when the deepest
// prefix is tied across candidates (genuinely ambiguous) or callerFile is empty.
// Candidate files come from documents.source_path ("<relpath>?symbol=...", which
// is absolute in production) so each is normalized to workspace-relative via
// wsRoot before comparison. This scopes a bare call in a monorepo to the nearest
// definition (backend over frontend) without a re-index.
func nearestSymbolMatch(callerFile, wsRoot string, matches []sqlc.ResolveSymbolByNameRow) (sqlc.ResolveSymbolByNameRow, bool) {
	var zero sqlc.ResolveSymbolByNameRow
	if callerFile == "" {
		return zero, false
	}
	caller := stripWorkspacePrefix(wsRoot, callerFile)
	bestDepth, tie := -1, false
	var best sqlc.ResolveSymbolByNameRow
	for _, m := range matches {
		qIdx := strings.Index(m.SourcePath, "?")
		if qIdx < 0 {
			continue
		}
		cf := stripWorkspacePrefix(wsRoot, m.SourcePath[:qIdx])
		switch d := sharedPathDepth(caller, cf); {
		case d > bestDepth:
			bestDepth, best, tie = d, m, false
		case d == bestDepth:
			tie = true
		}
	}
	if bestDepth < 0 || tie {
		return zero, false
	}
	return best, true
}
