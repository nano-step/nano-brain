package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

// IsHex reports whether s contains only ASCII hex digits (0-9, a-f, A-F).
func IsHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// ResolveWorkspaceParam resolves a workspace input string to a registered workspace hash.
//
// Resolution priority:
//   - 64 hex chars → full hash returned as-is (no DB check; backward-compatible)
//   - 8–63 hex chars → hash prefix lookup (error if 0 or 2+ matches)
//   - other string → name lookup (case-insensitive)
func ResolveWorkspaceParam(ctx context.Context, q *sqlc.Queries, input string) (string, error) {
	if len(input) == 64 && IsHex(input) {
		return input, nil
	}
	if len(input) >= 8 && IsHex(input) {
		count, err := q.CountWorkspacesByHashPrefix(ctx, input+"%")
		if err != nil {
			return "", fmt.Errorf("prefix lookup failed: %w", err)
		}
		if count == 0 {
			return "", fmt.Errorf("no workspace matching prefix %q", input)
		}
		if count > 1 {
			return "", fmt.Errorf("ambiguous prefix %q matches %d workspaces; use a longer prefix", input, count)
		}
		ws, err := q.GetWorkspaceByHashPrefix(ctx, input+"%")
		if err != nil {
			return "", fmt.Errorf("prefix lookup failed: %w", err)
		}
		return ws.Hash, nil
	}
	ws, err := q.GetWorkspaceByName(ctx, input)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("workspace %q not found", input)
		}
		return "", fmt.Errorf("workspace lookup failed: %w", err)
	}
	return ws.Hash, nil
}
