package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// encodeClaudeProjectsDir encodes a workspace root path to the directory name
// used by Claude Code under ~/.claude/projects/. Claude Code encodes the cwd
// by replacing every '/' with '-' (including the leading slash), producing a
// flat directory name like "-Users-alice-workspaces-myproject".
//
// This is a best-effort match: it mirrors how Claude Code names its project
// directories (replacing '/' with '-'). The encoding is NOT injective —
// workspace paths that contain '-' can collide (e.g. "/a/b-c" and "/a-b/c"
// both encode to "-a-b-c"), causing a silent miss: the workspace simply won't
// get a harvester and its sessions won't be harvested. No error or crash
// occurs. This is acceptable because the lookup direction is always
// workspace→dir and a miss is benign.
func encodeClaudeProjectsDir(workspacePath string) string {
	return strings.ReplaceAll(workspacePath, "/", "-")
}

// initClaudeCodeHarvesters scans cfg.SessionDir (the Claude Code projects
// parent directory, typically ~/.claude/projects/) for session directories
// that correspond to registered nano-brain workspaces.
//
// For each registered workspace, it encodes the workspace root path into the
// Claude directory name format and checks whether that directory exists under
// cfg.SessionDir. When a match is found, one ClaudeCodeHarvester is created
// for that workspace.
//
// Returns an empty slice (not an error) when:
//   - cfg.Enabled = false
//   - cfg.SessionDir does not exist on disk
//   - no registered workspaces have a matching Claude session directory
//
// Only returns a non-nil error for unexpected failures (currently none).
//
// This replaces the old single-harvester design that incorrectly hashed the
// Claude session directory path instead of the workspace root path, causing a
// workspace hash mismatch that silently disabled the harvester (issue #238).
func initClaudeCodeHarvesters(
	ctx context.Context,
	cfg config.ClaudeCodeHarvesterConfig,
	db *sql.DB,
	logger zerolog.Logger,
) ([]*harvest.ClaudeCodeHarvester, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if _, err := os.Stat(cfg.SessionDir); os.IsNotExist(err) {
		logger.Warn().
			Str("session_dir", cfg.SessionDir).
			Msg("claude code harvester enabled but session_dir does not exist, skipping")
		return nil, nil
	}

	q := sqlc.New(db)
	workspaces, err := q.ListWorkspaces(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("claude code harvester: failed to list workspaces, skipping")
		return nil, nil
	}

	var harvesters []*harvest.ClaudeCodeHarvester
	for _, ws := range workspaces {
		encoded := encodeClaudeProjectsDir(ws.Path)
		sessionDir := filepath.Join(cfg.SessionDir, encoded)
		if _, statErr := os.Stat(sessionDir); os.IsNotExist(statErr) {
			continue
		}
		h := harvest.NewClaudeCodeHarvester(db, logger, sessionDir, ws.Hash)
		harvesters = append(harvesters, h)
		logger.Info().
			Str("session_dir", sessionDir).
			Str("workspace", ws.Path).
			Str("workspace_hash", ws.Hash).
			Msg("claude code session harvester registered for workspace")
	}

	if len(harvesters) == 0 {
		logger.Info().
			Str("session_dir", cfg.SessionDir).
			Msg("claude code harvester: no registered workspaces have a matching Claude session directory")
	}

	return harvesters, nil
}
