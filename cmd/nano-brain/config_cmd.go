package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/nano-brain/nano-brain/internal/config"
)

func runConfigCmd(args []string, configPath string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain config <show|check> [--json]")
		os.Exit(1)
	}
	jsonFlag := false
	for _, a := range args[1:] {
		if a == "--json" {
			jsonFlag = true
		}
	}
	switch args[0] {
	case "show":
		runConfigShow(configPath, jsonFlag)
	case "check":
		runConfigCheck(configPath, jsonFlag)
	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", args[0])
		fmt.Fprintln(os.Stderr, "Usage: nano-brain config <show|check> [--json]")
		os.Exit(1)
	}
}

func runConfigShow(configPath string, jsonFlag bool) {
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if jsonFlag {
		data := configToMap(cfg, configPath)
		j, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(j))
		return
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("# No config file at %s — showing defaults:\n\n", configPath)
		printConfigDefaults(cfg)
		return
	}
	fmt.Printf("# %s\n\n", configPath)
	fmt.Print(maskSensitive(string(content)))
}

func runConfigCheck(configPath string, jsonFlag bool) {
	var args []string
	if jsonFlag {
		args = append(args, "--json")
	}
	runDoctorCmd(args, configPath)
}

func maskSensitive(s string) string {
	re := regexp.MustCompile(`(postgres(?:ql)?://[^:]+:)([^@]+)(@)`)
	s = re.ReplaceAllString(s, "${1}***${3}")
	re2 := regexp.MustCompile(`(voyage_api_key:\s*).+`)
	s = re2.ReplaceAllString(s, "${1}***")
	return s
}

func configToMap(cfg *config.Config, configPath string) map[string]interface{} {
	return map[string]interface{}{
		"config_path": configPath,
		"server": map[string]interface{}{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		},
		"database": map[string]interface{}{
			"url": maskSensitive(cfg.Database.URL),
		},
		"embedding": map[string]interface{}{
			"provider":    cfg.Embedding.Provider,
			"url":         cfg.Embedding.URL,
			"model":       cfg.Embedding.Model,
			"concurrency": cfg.Embedding.Concurrency,
		},
		"search": map[string]interface{}{
			"rrf_k":                  cfg.Search.RrfK,
			"recency_weight":         cfg.Search.RecencyWeight,
			"recency_half_life_days": cfg.Search.RecencyHalfLifeDays,
			"limit":                  cfg.Search.Limit,
		},
		"logging": map[string]interface{}{
			"level": cfg.Logging.Level,
			"file":  cfg.Logging.File,
		},
	}
}

func printConfigDefaults(cfg *config.Config) {
	fmt.Printf("server:\n  host: %s\n  port: %d\n\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("database:\n  url: %s\n\n", maskSensitive(cfg.Database.URL))
	fmt.Printf("embedding:\n  provider: %s\n  url: %s\n  model: %s\n  concurrency: %d\n\n",
		cfg.Embedding.Provider, cfg.Embedding.URL, cfg.Embedding.Model, cfg.Embedding.Concurrency)
	fmt.Printf("search:\n  rrf_k: %.0f\n  limit: %d\n\n", cfg.Search.RrfK, cfg.Search.Limit)
	fmt.Printf("logging:\n  level: %s\n", cfg.Logging.Level)
}
