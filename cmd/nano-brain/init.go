package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
)

func detectOllama(url string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func runInteractiveInit(configPath string) {
	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	dbURL := "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev"
	provider := "ollama"
	embURL := "http://localhost:11434"
	model := "nomic-embed-text"
	port := 3100

	fmt.Print("\nnano-brain setup\n────────────────\n\n")

	scanner := bufio.NewScanner(os.Stdin)

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  Config exists at %s\n", configPath)
		answer := promptWithDefault(scanner, "Overwrite?", "Y")
		if answer == "n" || answer == "N" {
			fmt.Println("Aborted.")
			return
		}
		fmt.Println()
	}

	dbURL = promptWithDefault(scanner, "PostgreSQL URL", dbURL)
	provider = promptWithDefault(scanner, "Embedding provider (ollama/voyage)", provider)

	if provider == "voyage" {
		model = promptWithDefault(scanner, "Embedding model", "voyage-3")
		envKey := os.Getenv("VOYAGE_API_KEY")
		def := ""
		if envKey != "" {
			def = "(from env)"
		}
		_ = promptWithDefault(scanner, "Voyage API key", def)
	} else {
		if detectOllama(embURL) {
			fmt.Printf("  Ollama detected at %s\n", embURL)
		} else {
			embURL = promptWithDefault(scanner, "Ollama URL", embURL)
		}
		model = promptWithDefault(scanner, "Embedding model", model)
	}

	portStr := promptWithDefault(scanner, "Server port", strconv.Itoa(port))
	p, err := strconv.Atoi(portStr)
	if err != nil || p < 1 || p > 65535 {
		fmt.Fprintf(os.Stderr, "Invalid port: %s\n", portStr)
		os.Exit(1)
	}
	port = p

	var embBlock string
	if provider == "voyage" {
		embBlock = fmt.Sprintf("embedding:\n  provider: voyage\n  model: %s\n  concurrency: 3\n", model)
	} else {
		embBlock = fmt.Sprintf("embedding:\n  provider: %s\n  url: %s\n  model: %s\n  concurrency: 3\n", provider, embURL, model)
	}

	fmt.Print("\n── Harvester (session indexing) ──\n")
	detectedOC := detectOpenCodeStorageDir()
	if detectedOC != "" {
		fmt.Printf("  OpenCode storage auto-detected: %s\n", detectedOC)
	} else {
		fmt.Println("  OpenCode storage not found automatically.")
	}
	ocSessionDir := promptWithDefault(scanner, "OpenCode session_dir (leave blank to skip)", detectedOC)

	detectedCC := detectClaudeCodeStorageDir()
	if detectedCC != "" {
		fmt.Printf("  Claude Code storage auto-detected: %s\n", detectedCC)
	}
	ccSessionDir := promptWithDefault(scanner, "Claude Code session_dir (leave blank to skip)", detectedCC)

	var harvesterBlock string
	ocLine := "\"\""
	if ocSessionDir != "" {
		ocLine = ocSessionDir
	}
	ccEnabled := ccSessionDir != ""
	ccLine := "\"\""
	if ccSessionDir != "" {
		ccLine = ccSessionDir
	}
	harvesterBlock = fmt.Sprintf("\nharvester:\n  opencode:\n    session_dir: %s\n  claudecode:\n    enabled: %v\n    session_dir: %s\n", ocLine, ccEnabled, ccLine)

	fmt.Print("\n── Summarization (LLM session summaries) ──\n")
	sumEnabled := promptWithDefault(scanner, "Enable session summarization?", "n")
	sumEnabledBool := sumEnabled == "y" || sumEnabled == "Y"

	var summaryBlock string
	if sumEnabledBool {
		sumProviderURL := promptWithDefault(scanner, "LLM provider URL (OpenAI-compatible)", "")
		if sumProviderURL == "" {
			fmt.Fprintln(os.Stderr, "  provider_url is required when summarization is enabled.")
			os.Exit(1)
		}
		sumAPIKey := promptWithDefault(scanner, "LLM API key (or set NANO_BRAIN_SUMMARIZE_API_KEY env)", "")
		sumModel := promptWithDefault(scanner, "LLM model name", "claude-sonnet-4-5")
		sumMaxTokens := promptWithDefault(scanner, "Max tokens per summary", "4096")
		sumConcurrency := promptWithDefault(scanner, "Parallel LLM calls (map phase)", "3")
		sumRPS := promptWithDefault(scanner, "Rate limit (requests/second, 0 = unlimited)", "1")
		sumOutputDir := promptWithDefault(scanner, "Summary output directory", "~/.nano-brain/summaries")
		apiKeyLine := ""
		if sumAPIKey != "" {
			apiKeyLine = fmt.Sprintf("\n  api_key: %s", sumAPIKey)
		} else {
			apiKeyLine = "\n  # api_key: set NANO_BRAIN_SUMMARIZE_API_KEY env var"
		}
		summaryBlock = fmt.Sprintf("\nsummarization:\n  enabled: true\n  provider_url: %s%s\n  model: %s\n  max_tokens: %s\n  concurrency: %s\n  requests_per_second: %s\n  output_dir: %s\n",
			sumProviderURL, apiKeyLine, sumModel, sumMaxTokens, sumConcurrency, sumRPS, sumOutputDir)
	} else {
		summaryBlock = "\nsummarization:\n  enabled: false\n"
	}

	yaml := fmt.Sprintf(`server:
  host: localhost
  port: %d

database:
  url: %s

%s
search:
  rrf_k: 60
  recency_weight: 0.3
  recency_half_life_days: 180
  limit: 20

watcher:
  debounce_ms: 2000
  reindex_interval: 300

logging:
  level: info
%s%s`, port, dbURL, embBlock, harvesterBlock, summaryBlock)

	fmt.Println("\n── Config preview ──────────────")
	fmt.Print(yaml)
	fmt.Println("────────────────────────────────")

	answer := promptWithDefault(scanner, "Save this config?", "Y")
	if answer == "n" || answer == "N" {
		fmt.Println("Aborted.")
		return
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create config directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(configPath, []byte(yaml), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nConfig written to %s\n\n", configPath)

	runDoctorCmd([]string{}, configPath)

	cwd, _ := os.Getwd()
	wsDir := promptWithDefault(scanner, "Register workspace directory?", cwd)
	if wsDir != "" {
		fmt.Printf("\nTo register this workspace, start the server and run:\n")
		fmt.Printf("  nano-brain init --root %s\n\n", wsDir)
	}
}

func promptWithDefault(scanner *bufio.Scanner, prompt, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal
	}
	return input
}
