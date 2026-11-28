package main

import (
	"context"
	"database/sql"
	"path/filepath"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/harvest"
	"github.com/rs/zerolog"
)

// encodeClaudeProjectsDir is kept as the cmd-local test seam for Claude's
// projects leaf encoding. The implementation lives in harvest so production
// discovery and validation share one definition.
func encodeClaudeProjectsDir(workspacePath string) string {
	return harvest.EncodeClaudeProjectsDir(workspacePath)
}

func configuredClaudeCodeSessionDirs(cfg config.ClaudeCodeHarvesterConfig) []string {
	var out []string
	if cfg.SessionDir != "" {
		out = append(out, cfg.SessionDir)
	}
	return append(out, cfg.SessionDirs...)
}

// applyClaudeCodeStorageAutoDetection populates stable auto-detected projects
// parent patterns only for the zero-config startup path. Any explicit
// session_dir/session_dirs entry is a complete replacement list and disables
// auto-detection, as does serve_only or a disabled harvester.
func applyClaudeCodeStorageAutoDetection(cfg *config.Config) []string {
	if cfg == nil || cfg.Server.ServeOnly || !cfg.Harvester.ClaudeCode.Enabled ||
		cfg.Harvester.ClaudeCode.SessionDir != "" || len(cfg.Harvester.ClaudeCode.SessionDirs) != 0 {
		return nil
	}
	detected := withFutureClaudeCodeStoragePatterns(detectClaudeCodeStoragePatterns())
	if len(detected) != 0 {
		cfg.Harvester.ClaudeCode.SessionDirs = detected
	}
	return detected
}

func withFutureClaudeCodeStoragePatterns(detected []string) []string {
	seen := make(map[string]struct{}, len(detected)+2)
	out := make([]string, 0, len(detected)+2)
	add := func(pattern string) {
		if pattern == "" {
			return
		}
		pattern = filepath.Clean(pattern)
		if _, dup := seen[pattern]; dup {
			return
		}
		seen[pattern] = struct{}{}
		out = append(out, pattern)
	}
	for _, pattern := range detected {
		add(pattern)
	}
	for _, pattern := range platformClaudeCodePaths() {
		add(pattern)
	}
	if home := userHomeDir(); home != "" {
		add(filepath.Join(home, ".ccs", "instances", "*", "projects"))
	}
	return out
}

// initClaudeCodeHarvesters registers dynamic Claude projects-root discovery.
// The returned harvester re-expands configured globs and re-reads registered
// workspaces on every harvest cycle, so first sessions, new CCS profiles, and
// workspaces registered after daemon startup do not require a daemon restart.
// The resolver itself closes over startup-loaded config; YAML edits remain
// restart-required.
//
// A harvester is registered whenever enabled raw directory configuration is
// present, even if it currently matches nothing. Missing future glob matches
// are therefore discoverable later. Returns an empty slice when disabled or
// when neither session_dir nor session_dirs is configured.
func initClaudeCodeHarvesters(
	_ context.Context,
	cfg config.ClaudeCodeHarvesterConfig,
	db *sql.DB,
	logger zerolog.Logger,
) ([]harvest.Harvester, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.SessionDir == "" && len(cfg.SessionDirs) == 0 {
		logger.Warn().Msg("claude code harvester enabled but no session_dir/session_dirs configured, skipping")
		return nil, nil
	}

	resolver := cfg.SessionDirsResolved
	return []harvest.Harvester{
		harvest.NewClaudeCodeDiscoveryHarvester(db, logger, resolver),
	}, nil
}
