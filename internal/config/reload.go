package config

import "fmt"

// ReloadResult describes which settings changed and how.
type ReloadResult struct {
	Reloaded        []string `json:"reloaded"`
	Unchanged       []string `json:"unchanged"`
	RequiresRestart []string `json:"requires_restart"`
}

// Reload re-reads the config file, validates it, and classifies changes
// relative to current into reloaded, unchanged, or requires_restart.
func Reload(configPath string, current *Config) (*Config, *ReloadResult, error) {
	newCfg, err := Load(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("reload failed: %w", err)
	}

	result := &ReloadResult{}

	type field struct {
		name       string
		reloadable bool
		changed    bool
	}

	fields := []field{
		{"server.host", false, newCfg.Server.Host != current.Server.Host},
		{"server.port", false, newCfg.Server.Port != current.Server.Port},
		{"database.url", false, newCfg.Database.URL != current.Database.URL},
		{"embedding.provider", false, newCfg.Embedding.Provider != current.Embedding.Provider},
		{"embedding.url", false, newCfg.Embedding.URL != current.Embedding.URL},
		{"embedding.model", false, newCfg.Embedding.Model != current.Embedding.Model},
		{"embedding.dimension", false, newCfg.Embedding.Dimension != current.Embedding.Dimension},
		{"embedding.voyage_api_key", false, newCfg.Embedding.VoyageAPIKey != current.Embedding.VoyageAPIKey},
		{"embedding.concurrency", true, newCfg.Embedding.Concurrency != current.Embedding.Concurrency},
		{"logging.level", true, newCfg.Logging.Level != current.Logging.Level},
		{"logging.file", false, newCfg.Logging.File != current.Logging.File},
		{"search.rrf_k", true, newCfg.Search.RrfK != current.Search.RrfK},
		{"search.recency_weight", true, newCfg.Search.RecencyWeight != current.Search.RecencyWeight},
		{"search.recency_half_life_days", true, newCfg.Search.RecencyHalfLifeDays != current.Search.RecencyHalfLifeDays},
		{"search.limit", true, newCfg.Search.Limit != current.Search.Limit},
		{"storage.max_file_size", true, newCfg.Storage.MaxFileSize != current.Storage.MaxFileSize},
		{"storage.max_size", true, newCfg.Storage.MaxSize != current.Storage.MaxSize},
		{"watcher.debounce_ms", true, newCfg.Watcher.DebounceMs != current.Watcher.DebounceMs},
		{"watcher.reindex_interval", true, newCfg.Watcher.ReindexInterval != current.Watcher.ReindexInterval},
		{"intervals.session_poll", true, newCfg.Intervals.SessionPoll != current.Intervals.SessionPoll},
		{"harvester.opencode.session_dir", false, newCfg.Harvester.OpenCode.SessionDir != current.Harvester.OpenCode.SessionDir},
		{"harvester.claudecode.enabled", false, newCfg.Harvester.ClaudeCode.Enabled != current.Harvester.ClaudeCode.Enabled},
		{"harvester.claudecode.session_dir", false, newCfg.Harvester.ClaudeCode.SessionDir != current.Harvester.ClaudeCode.SessionDir},
	}

	for _, f := range fields {
		if !f.changed {
			result.Unchanged = append(result.Unchanged, f.name)
			continue
		}
		if f.reloadable {
			result.Reloaded = append(result.Reloaded, f.name)
		} else {
			result.RequiresRestart = append(result.RequiresRestart, f.name)
		}
	}

	return newCfg, result, nil
}
