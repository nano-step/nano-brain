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

// encodePiSessionsDir encodes a workspace root path to the directory name Pi
// uses under ~/.pi/agent/sessions/. Pi strips the leading '/', replaces every
// remaining '/' with '-', and wraps the result in a leading and trailing
// "--" — e.g. "/Users/a/b" -> "--Users-a-b--". Distinct from Claude Code's
// encodeClaudeProjectsDir, which replaces the leading '/' too and does not
// wrap (producing "-Users-a-b" instead).
//
// Not injective, mirroring encodeClaudeProjectsDir's accepted precedent: a
// workspace path containing a literal '-' can collide with another path's
// encoding, causing a silent miss — the workspace simply gets no Pi
// harvester. No error or crash occurs; the lookup direction is always
// workspace→dir, and a miss is benign. See
// openspec/changes/pi-session-harvesting/design.md Decision 2.
func encodePiSessionsDir(workspacePath string) string {
	trimmed := strings.TrimPrefix(workspacePath, "/")
	return "--" + strings.ReplaceAll(trimmed, "/", "-") + "--"
}

// initPiHarvesters scans cfg.SessionDir (typically ~/.pi/agent/sessions/) for
// session directories that correspond to registered nano-brain workspaces.
// Mirrors initClaudeCodeHarvesters exactly.
//
// Returns an empty slice (not an error) when:
//   - cfg.Enabled = false
//   - cfg.SessionDir does not exist on disk
//   - no registered workspaces have a matching Pi session directory
func initPiHarvesters(
	ctx context.Context,
	cfg config.PiHarvesterConfig,
	db *sql.DB,
	logger zerolog.Logger,
) ([]*harvest.PiHarvester, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if _, err := os.Stat(cfg.SessionDir); os.IsNotExist(err) {
		logger.Warn().
			Str("session_dir", cfg.SessionDir).
			Msg("pi harvester enabled but session_dir does not exist, skipping")
		return nil, nil
	}

	q := sqlc.New(db)
	workspaces, err := q.ListWorkspaces(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("pi harvester: failed to list workspaces, skipping")
		return nil, nil
	}

	var harvesters []*harvest.PiHarvester
	for _, ws := range workspaces {
		encoded := encodePiSessionsDir(ws.Path)
		sessionDir := filepath.Join(cfg.SessionDir, encoded)
		if _, statErr := os.Stat(sessionDir); os.IsNotExist(statErr) {
			continue
		}
		h := harvest.NewPiHarvester(db, logger, sessionDir, ws.Hash)
		harvesters = append(harvesters, h)
		logger.Info().
			Str("session_dir", sessionDir).
			Str("workspace", ws.Path).
			Str("workspace_hash", ws.Hash).
			Msg("pi session harvester registered for workspace")
	}

	if len(harvesters) == 0 {
		logger.Info().
			Str("session_dir", cfg.SessionDir).
			Msg("pi harvester: no registered workspaces have a matching Pi session directory")
	}

	return harvesters, nil
}
