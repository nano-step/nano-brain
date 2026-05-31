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

// initOpenCodeFileHarvester returns a configured legacy JSON-file OpenCode harvester,
// or (nil, nil) if it should not be started. Reasons to skip:
//   - cfg.SessionDir is empty
//   - cfg.SessionDir does not exist on disk
//   - WorkspaceHash(cfg.SessionDir) computation failed
//   - the computed workspace_hash is not registered in the workspaces table
//
// (nil, nil) means "do not start, no error". Only returns a non-nil error
// for unexpected programming-level failures (currently none).
//
// Mirrors initClaudeCodeHarvester — extracted for the same registration guard.
// See issue #238.
func initOpenCodeFileHarvester(
	ctx context.Context,
	cfg config.OpenCodeHarvesterConfig,
	db *sql.DB,
	logger zerolog.Logger,
) (*harvest.OpenCodeHarvester, error) {
	if cfg.SessionDir == "" {
		return nil, nil
	}

	if _, err := os.Stat(cfg.SessionDir); os.IsNotExist(err) {
		logger.Warn().
			Str("session_dir", cfg.SessionDir).
			Msg("opencode session harvester enabled but session_dir does not exist, skipping")
		return nil, nil
	}

	wsHash, err := storage.WorkspaceHash(cfg.SessionDir)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to compute workspace hash for opencode session harvester")
		return nil, nil
	}

	q := sqlc.New(db)
	if _, lookupErr := q.GetWorkspaceByHash(ctx, wsHash); lookupErr != nil {
		if errors.Is(lookupErr, sql.ErrNoRows) {
			logger.Warn().
				Str("session_dir", cfg.SessionDir).
				Str("computed_workspace_hash", wsHash).
				Msg("opencode session_dir is not a registered workspace; harvester disabled. Run nano-brain init --root=<path> to register.")
			return nil, nil
		}
		logger.Error().Err(lookupErr).
			Str("session_dir", cfg.SessionDir).
			Msg("opencode session_dir workspace lookup failed; harvester disabled")
		return nil, nil
	}

	return harvest.NewOpenCodeHarvester(db, logger, cfg.SessionDir, wsHash), nil
}
