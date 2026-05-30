package main

import (
	"context"
	"database/sql"
	"errors"
	"os"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// initClaudeCodeHarvester returns a configured Claude Code harvester, or
// (nil, nil) if it should not be started. Reasons to skip:
//   - cfg.Enabled = false
//   - cfg.SessionDir does not exist on disk
//   - WorkspaceHash(cfg.SessionDir) computation failed
//   - the computed workspace_hash is not registered in the workspaces table
//
// (nil, nil) means "do not start, no error". Only returns a non-nil error
// for unexpected programming-level failures (currently none).
//
// This was extracted from main.go to be unit-testable. See issue #238.
func initClaudeCodeHarvester(
	ctx context.Context,
	cfg config.ClaudeCodeHarvesterConfig,
	db *sql.DB,
	logger zerolog.Logger,
) (*harvest.ClaudeCodeHarvester, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if _, err := os.Stat(cfg.SessionDir); os.IsNotExist(err) {
		logger.Warn().
			Str("session_dir", cfg.SessionDir).
			Msg("claude code harvester enabled but session_dir does not exist, skipping")
		return nil, nil
	}

	wsHash, err := storage.WorkspaceHash(cfg.SessionDir)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to compute workspace hash for claude code harvester")
		return nil, nil
	}

	q := sqlc.New(db)
	if _, lookupErr := q.GetWorkspaceByHash(ctx, wsHash); lookupErr != nil {
		if errors.Is(lookupErr, sql.ErrNoRows) {
			logger.Warn().
				Str("session_dir", cfg.SessionDir).
				Str("computed_workspace_hash", wsHash).
				Msg("claude code session_dir is not a registered workspace; harvester disabled. Run nano-brain init --root=<path> to register.")
			return nil, nil
		}
		logger.Error().Err(lookupErr).
			Str("session_dir", cfg.SessionDir).
			Msg("claude code workspace lookup failed; harvester disabled")
		return nil, nil
	}

	return harvest.NewClaudeCodeHarvester(db, logger, cfg.SessionDir, wsHash), nil
}
