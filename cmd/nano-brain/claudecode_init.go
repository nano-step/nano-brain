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

// initClaudeCodeHarvesters scans every resolved session dir (cfg.SessionDirs
// plus the legacy cfg.SessionDir, after "~" and glob expansion — see
// ClaudeCodeHarvesterConfig.SessionDirsResolved) for session directories that
// correspond to registered nano-brain workspaces.
//
// For each registered workspace, it encodes the workspace root path into the
// Claude directory name format and checks whether that directory exists under
// each session dir. When a match is found, one ClaudeCodeHarvester is created
// for that (dir, workspace) pair, so transcripts under multiple projects dirs
// (e.g. ~/.claude/projects and ~/.ccs/instances/<name>/projects) are all
// harvested (#643).
//
// Returns an empty slice (not an error) when:
//   - cfg.Enabled = false
//   - no session dirs are configured/resolved
//   - every resolved dir is missing on disk
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

	baseDirs := cfg.SessionDirsResolved()
	if len(baseDirs) == 0 {
		logger.Warn().Msg("claude code harvester enabled but no session_dir/session_dirs configured, skipping")
		return nil, nil
	}
	existingDirs := baseDirs[:0]
	for _, baseDir := range baseDirs {
		if _, err := os.Stat(baseDir); err != nil {
			logger.Warn().
				Str("session_dir", baseDir).
				Msg("claude code harvester session dir does not exist, skipping")
			continue
		}
		existingDirs = append(existingDirs, baseDir)
	}
	if len(existingDirs) == 0 {
		return nil, nil
	}

	q := sqlc.New(db)
	workspaces, err := q.ListWorkspaces(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("claude code harvester: failed to list workspaces, skipping")
		return nil, nil
	}

	var harvesters []*harvest.ClaudeCodeHarvester
	seenSessionDirs := make(map[string]struct{})
	for _, baseDir := range existingDirs {
		for _, ws := range workspaces {
			encoded := encodeClaudeProjectsDir(ws.Path)
			sessionDir := filepath.Join(baseDir, encoded)
			if _, statErr := os.Stat(sessionDir); os.IsNotExist(statErr) {
				continue
			}
			if _, dup := seenSessionDirs[sessionDir]; dup {
				continue
			}
			seenSessionDirs[sessionDir] = struct{}{}
			h := harvest.NewClaudeCodeHarvester(db, logger, sessionDir, ws.Hash)
			harvesters = append(harvesters, h)
			logger.Info().
				Str("session_dir", sessionDir).
				Str("workspace", ws.Path).
				Str("workspace_hash", ws.Hash).
				Msg("claude code session harvester registered for workspace")
		}
	}

	if len(harvesters) == 0 {
		logger.Info().
			Strs("session_dirs", existingDirs).
			Msg("claude code harvester: no registered workspaces have a matching Claude session directory")
	}

	return harvesters, nil
}
