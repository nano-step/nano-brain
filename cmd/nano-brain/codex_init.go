package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/rs/zerolog"
)

// resolveCodexHome picks the Codex home directory: explicit config value,
// then $CODEX_HOME, then ~/.codex. The config value is used as-is (no ~
// expansion), matching PiHarvesterConfig.SessionDir's precedent.
func resolveCodexHome(cfgHome string) string {
	if cfgHome != "" {
		return cfgHome
	}
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE") // windows fallback
	}
	return filepath.Join(home, ".codex")
}

// initCodexHarvesters returns a single CodexHarvester when enabled and the
// resolved Codex home contains a sessions/ or archived_sessions/ directory.
// Unlike initPiHarvesters/initClaudeCodeHarvesters there is no per-workspace
// fan-out: Codex keeps one global session tree and the harvester attributes
// each session to a workspace from session_meta.payload.cwd at harvest time.
//
// Returns nil (not an error) when disabled, when the home has no session
// directories, or when neither directory exists on disk.
func initCodexHarvesters(
	ctx context.Context,
	cfg config.CodexHarvesterConfig,
	db *sql.DB,
	logger zerolog.Logger,
) []*harvest.CodexHarvester {
	if !cfg.Enabled {
		return nil
	}

	codexHome := resolveCodexHome(cfg.CodexHome)
	sessionsDir := filepath.Join(codexHome, "sessions")
	archivedDir := filepath.Join(codexHome, "archived_sessions")

	hasSessions := dirExists(sessionsDir)
	hasArchived := dirExists(archivedDir)
	if !hasSessions && !hasArchived {
		logger.Warn().
			Str("codex_home", codexHome).
			Msg("codex harvester enabled but no sessions/ or archived_sessions/ directory found, skipping")
		return nil
	}

	h := harvest.NewCodexHarvester(db, logger, codexHome)
	logger.Info().
		Str("codex_home", codexHome).
		Bool("sessions_dir", hasSessions).
		Bool("archived_dir", hasArchived).
		Msg("codex session harvester registered")
	return []*harvest.CodexHarvester{h}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
