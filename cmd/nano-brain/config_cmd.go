package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/nano-brain/nano-brain/internal/config"
	"gopkg.in/yaml.v3"
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
	// The key must end in "password" (sslpassword included) and start after
	// whitespace or a URL query delimiter; an unquoted value runs to the next
	// whitespace so `&` and `"` inside it are masked too.
	maskDSNPassword = regexp.MustCompile(`(?i)(^|[\s?&])(\w*password\s*=\s*)('(?:[^'\\]|\\.)*'|[^\s]+)`)
	// dsnParamStart finds a `&key=` parameter boundary inside a matched
	// keyword-DSN value so the tail is preserved rather than eaten as
	// password text.
	dsnParamStart = regexp.MustCompile(`&[A-Za-z0-9_]+=`)
	maskURL       = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s"'` + "`" + `]+`)
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
			maskYAMLNode(node.Content[i])
			keyNode := node.Content[i]
			if keyNode.Kind == yaml.AliasNode && keyNode.Alias != nil {
				keyNode = keyNode.Alias
			}
			// koanf/mapstructure match keys case-insensitively (a `Password_Hash`
			// inside a users list item loads), so the check must be too.
			key, value := strings.ToLower(keyNode.Value), node.Content[i+1]
			if key == "tokens" || key == "password_hash" || strings.HasSuffix(key, "api_key") {
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
			maskDSNPassword.ReplaceAllStringFunc(
				maskPostgresPassword.ReplaceAllString(node.Value, "${1}***${3}"), maskDSNValue),
			config.RedactURLCredentials)
	}
}

// maskDSNValue builds the replacement for a keyword-DSN password match: the
// key stays, the value becomes ***, and a `&key=` tail (a URL query parameter
// or the next keyword pair) is kept.
func maskDSNValue(m string) string {
	g := maskDSNPassword.FindStringSubmatchIndex(m)
	if g == nil || g[6] < 0 {
		return m
	}
	v := m[g[6]:g[7]]
	tail := ""
	if !strings.HasPrefix(v, "'") {
		if loc := dsnParamStart.FindStringIndex(v); loc != nil {
			tail = v[loc[0]:]
		}
	}
	return m[:g[6]] + "***" + tail
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
		maskDSNPassword.ReplaceAllStringFunc(
			maskPostgresPassword.ReplaceAllString(dsn, "${1}***${3}"), maskDSNValue),
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
