package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/nano-brain/nano-brain/internal/config"
)

func runConfigCmd(args []string, configPath string) {
	cliLog.Debug().Str("cmd", "config").Msg("cli command started")
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
	cliLog.Debug().Str("cmd", "config."+args[0]).Msg("cli command completed")
}

func runConfigShow(configPath string, jsonFlag bool) {
	if configPath == "" {
		configPath = config.ResolveConfigPath("")
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		cliLog.Error().Err(err).Str("cmd", "config.show").Msg("config load failed")
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

var (
	maskPostgresPassword = regexp.MustCompile(`(postgres(?:ql)?://[^:]+:)([^@]+)(@)`)
	maskAPIKey           = regexp.MustCompile(`(api_key:\s*).+`)
	maskPasswordHash     = regexp.MustCompile(`(password_hash:\s*).+`)
	maskDSNPassword      = regexp.MustCompile(`(?i)\b(password\s*=\s*)('(?:[^'\\]|\\.)*'|[^\s&"]+)`)
	maskURL              = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'` + "`" + `]+`)
	tokensKey            = regexp.MustCompile(`^(\s*(?:-\s+)?["']?tokens["']?\s*:)\s*(.*)$`)
	tokensItem           = regexp.MustCompile(`^(\s*-\s+)\S.*$`)
)

// maskSensitive hides credentials in config YAML, one line at a time: API
// keys, password hashes, auth tokens, DSN passwords, and the userinfo and
// credential query parameters of any URL.
func maskSensitive(s string) string {
	lines := strings.Split(s, "\n")
	inTokens := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := tokensKey.FindStringSubmatch(line); m != nil {
			// "tokens:" opens a block list; "tokens: [a, b]" carries the
			// values on the same line.
			inTokens = m[2] == "" || strings.HasPrefix(m[2], "#")
			if !inTokens {
				lines[i] = m[1] + " ***"
			}
			continue
		}
		if inTokens && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if tokensItem.MatchString(line) {
				lines[i] = tokensItem.ReplaceAllString(line, "${1}***")
				continue
			}
			inTokens = false
		}
		line = maskPostgresPassword.ReplaceAllString(line, "${1}***${3}")
		line = maskAPIKey.ReplaceAllString(line, "${1}***")
		line = maskPasswordHash.ReplaceAllString(line, "${1}***")
		line = maskDSNPassword.ReplaceAllString(line, "${1}***")
		lines[i] = maskURL.ReplaceAllStringFunc(line, config.RedactURLCredentials)
	}
	return strings.Join(lines, "\n")
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
			"url":         config.RedactURLCredentials(cfg.Embedding.URL),
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
		cfg.Embedding.Provider, config.RedactURLCredentials(cfg.Embedding.URL), cfg.Embedding.Model, cfg.Embedding.Concurrency)
	fmt.Printf("search:\n  rrf_k: %.0f\n  limit: %d\n\n", cfg.Search.RrfK, cfg.Search.Limit)
	fmt.Printf("logging:\n  level: %s\n", cfg.Logging.Level)
}
