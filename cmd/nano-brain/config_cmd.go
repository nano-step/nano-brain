package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage"
	"gopkg.in/yaml.v3"
)

var (
	clientConfigWarnMu   sync.Mutex
	lastClientConfigWarn string
)

func resetClientConfigWarnState() {
	clientConfigWarnMu.Lock()
	lastClientConfigWarn = ""
	clientConfigWarnMu.Unlock()
}

// resolveCLIConfigPath resolves the config file path via ResolveConfigPathStrict,
// prints any missing-explicit-config warning to stderr, and updates deduplication
// state (clearing it when warn is empty).
func resolveCLIConfigPath(flagValue string) (string, string) {
	path, warn := config.ResolveConfigPathStrict(flagValue)
	clientConfigWarnMu.Lock()
	lastClientConfigWarn = warn
	clientConfigWarnMu.Unlock()
	if warn != "" {
		fmt.Fprintln(os.Stderr, warn)
	}
	return path, warn
}

// resolveConfigPathOnce resolves flagValue via ResolveConfigPathStrict and
// emits any missing-explicit-config warning to stderr at most once per unique
// warning string until cleared (by warn == "" or resetClientConfigWarnState).
func resolveConfigPathOnce(flagValue string) string {
	path, warn := config.ResolveConfigPathStrict(flagValue)
	clientConfigWarnMu.Lock()
	if warn == "" {
		lastClientConfigWarn = ""
		clientConfigWarnMu.Unlock()
		return path
	}
	shouldPrint := warn != lastClientConfigWarn
	lastClientConfigWarn = warn
	clientConfigWarnMu.Unlock()
	if shouldPrint {
		fmt.Fprintln(os.Stderr, warn)
	}
	return path
}

func resolveClientConfigPath() string {
	return resolveConfigPathOnce(cliConfigPath)
}

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
	configPath, _ = resolveCLIConfigPath(configPath)
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
	maskURL              = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'` + "`" + `]+`)
)

// maskSensitive parses YAML before masking so flow collections, quoted keys and
// block scalars cannot put credentials outside a line-oriented regex's reach.
func maskSensitive(s string) string {
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(s), &doc); err != nil {
		return "<redacted: invalid YAML>\n"
	}
	maskYAMLNode(&doc)
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return "<redacted: invalid YAML>\n"
	}
	return string(out)
}

func maskYAMLNode(node *yaml.Node) {
	// Comments are not part of the parsed config, and may themselves contain
	// copied credentials; never echo them through the redacted view.
	node.HeadComment, node.LineComment, node.FootComment = "", "", ""
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			maskYAMLNode(child)
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			// The key's loaded name must be decided before the key node is
			// rewritten: a !!binary key carries its name base64-encoded, and
			// an alias key can point at an anchor the pass already masked.
			name, known := mapKeyName(node.Content[i])
			maskYAMLNode(node.Content[i])
			value := node.Content[i+1]
			// koanf/mapstructure match keys case-insensitively (a `Password_Hash`
			// inside a users list item loads), so the check must be too.
			if !known || name == "tokens" || name == "password_hash" || strings.HasSuffix(name, "api_key") {
				value.HeadComment, value.LineComment, value.FootComment = "", "", ""
				maskSecretNode(value)
				continue
			}
			maskYAMLNode(value)
		}
	case yaml.ScalarNode:
		// koanf decodes !!binary to the real value, so the base64 text itself
		// is the secret; nothing inside it can be partially masked.
		if node.Tag == "!!binary" {
			node.Tag, node.Value = "!!str", "<redacted>"
			return
		}
		node.Value = maskURL.ReplaceAllStringFunc(
			maskPostgresPassword.ReplaceAllString(
				storage.MaskDSNPasswords(node.Value), "${1}***${3}"),
			config.RedactURLCredentials)
	}
}

// mapKeyName returns the lower-cased name a mapping key loads under. Alias
// keys are resolved to their anchor, and a !!binary key is base64-decoded —
// koanf decodes it the same way before matching a config field. known=false
// means the name cannot be determined (undecodable base64, or an anchor the
// redaction pass has already rewritten to a sentinel), and the pair must be
// masked.
func mapKeyName(keyNode *yaml.Node) (name string, known bool) {
	if keyNode.Kind == yaml.AliasNode && keyNode.Alias != nil {
		keyNode = keyNode.Alias
	}
	switch {
	case keyNode.Tag == "!!binary":
		decoded, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(keyNode.Value), ""))
		if err != nil {
			return "", false
		}
		return strings.ToLower(string(decoded)), true
	case keyNode.Value == "***" || keyNode.Value == "<redacted>":
		return "", false
	default:
		return strings.ToLower(keyNode.Value), true
	}
}

func maskSecretNode(node *yaml.Node) {
	node.HeadComment, node.LineComment, node.FootComment = "", "", ""
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		maskSecretNode(node.Alias)
	}
	if node.Kind == yaml.SequenceNode {
		for _, child := range node.Content {
			maskSecretNode(child)
		}
		return
	}
	node.Kind, node.Tag, node.Value, node.Content, node.Style = yaml.ScalarNode, "!!str", "***", nil, 0
}

func maskDatabaseURL(dsn string) string {
	return maskURL.ReplaceAllStringFunc(
		maskPostgresPassword.ReplaceAllString(
			storage.MaskDSNPasswords(dsn), "${1}***${3}"),
		config.RedactURLCredentials)
}

func configToMap(cfg *config.Config, configPath string) map[string]interface{} {
	return map[string]interface{}{
		"config_path": configPath,
		"server": map[string]interface{}{
			"host": cfg.Server.Host,
			"port": cfg.Server.Port,
		},
		"database": map[string]interface{}{
			"url": maskDatabaseURL(cfg.Database.URL),
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
	fmt.Printf("database:\n  url: %s\n\n", maskDatabaseURL(cfg.Database.URL))
	fmt.Printf("embedding:\n  provider: %s\n  url: %s\n  model: %s\n  concurrency: %d\n\n",
		cfg.Embedding.Provider, config.RedactURLCredentials(cfg.Embedding.URL), cfg.Embedding.Model, cfg.Embedding.Concurrency)
	fmt.Printf("search:\n  rrf_k: %.0f\n  limit: %d\n\n", cfg.Search.RrfK, cfg.Search.Limit)
	fmt.Printf("logging:\n  level: %s\n", cfg.Logging.Level)
}
