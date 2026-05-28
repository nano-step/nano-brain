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

	// ── Database ──────────────────────────────────────────────────────────
	fmt.Print("── Database ──\n")
	fmt.Println("  PostgreSQL connection string. Format: postgres://user:pass@host:port/db")
	dbURL = promptWithDefault(scanner, "PostgreSQL URL", dbURL)

	// ── Server ────────────────────────────────────────────────────────────
	fmt.Print("\n── Server ──\n")
	portStr := promptWithDefault(scanner, "Server port", strconv.Itoa(port))
	p, err := strconv.Atoi(portStr)
	if err != nil || p < 1 || p > 65535 {
		fmt.Fprintf(os.Stderr, "Invalid port: %s\n", portStr)
		os.Exit(1)
	}
	port = p

	// ── Embedding ─────────────────────────────────────────────────────────
	fmt.Print("\n── Embedding ──\n")
	fmt.Println("  Converts text chunks into vectors for semantic search.")
	fmt.Println("  Providers: ollama (local, free) | voyage (cloud, higher quality)")
	provider = promptWithDefault(scanner, "Embedding provider (ollama/voyage)", provider)

	var embBlock string
	if provider == "voyage" {
		model = promptWithDefault(scanner, "Embedding model", "voyage-3")
		envKey := os.Getenv("VOYAGE_API_KEY")
		def := ""
		if envKey != "" {
			def = "(from env)"
		}
		_ = promptWithDefault(scanner, "Voyage API key (or set VOYAGE_API_KEY env)", def)
		embConcurrency := promptWithDefault(scanner, "Embedding concurrency (parallel requests)", "3")
		embBlock = fmt.Sprintf("embedding:\n  provider: voyage\n  model: %s\n  concurrency: %s\n", model, embConcurrency)
	} else {
		if detectOllama(embURL) {
			fmt.Printf("  ✓ Ollama detected at %s\n", embURL)
		} else {
			fmt.Println("  Ollama not found at default URL. Make sure Ollama is running.")
			embURL = promptWithDefault(scanner, "Ollama URL", embURL)
		}
		fmt.Println("  Recommended embed models: nomic-embed-text (fast), mxbai-embed-large (better quality)")
		model = promptWithDefault(scanner, "Embedding model", model)
		embConcurrency := promptWithDefault(scanner, "Embedding concurrency (parallel Ollama requests)", "3")
		embBlock = fmt.Sprintf("embedding:\n  provider: %s\n  url: %s\n  model: %s\n  concurrency: %s\n", provider, embURL, model, embConcurrency)
	}

	// ── Harvester ─────────────────────────────────────────────────────────
	fmt.Print("\n── Harvester (session indexing) ──\n")
	fmt.Println("  Harvests AI coding sessions (OpenCode / Claude Code) into the memory index.")

	detectedOC := detectOpenCodeStorageDir()
	if detectedOC != "" {
		fmt.Printf("  ✓ OpenCode storage auto-detected: %s\n", detectedOC)
	} else {
		fmt.Println("  OpenCode storage not found automatically.")
	}
	ocSessionDir := promptWithDefault(scanner, "OpenCode session_dir (leave blank to skip)", detectedOC)

	detectedCC := detectClaudeCodeStorageDir()
	if detectedCC != "" {
		fmt.Printf("  ✓ Claude Code storage auto-detected: %s\n", detectedCC)
	} else {
		fmt.Println("  Claude Code storage not found automatically.")
	}
	ccSessionDir := promptWithDefault(scanner, "Claude Code session_dir (leave blank to skip)", detectedCC)

	ocLine := "\"\""
	if ocSessionDir != "" {
		ocLine = ocSessionDir
	}
	ccEnabled := ccSessionDir != ""
	ccLine := "\"\""
	if ccSessionDir != "" {
		ccLine = ccSessionDir
	}

	fmt.Println("  Session poll interval: how often (seconds) to check for new sessions.")
	sessionPollStr := promptWithDefault(scanner, "Session poll interval (seconds)", "120")
	sessionPoll, err2 := strconv.Atoi(sessionPollStr)
	if err2 != nil || sessionPoll < 10 {
		sessionPoll = 120
	}
	harvesterBlock := fmt.Sprintf("\nharvester:\n  opencode:\n    session_dir: %s\n  claudecode:\n    enabled: %v\n    session_dir: %s\n\nintervals:\n  session_poll: %d\n", ocLine, ccEnabled, ccLine, sessionPoll)

	// ── Summarization ─────────────────────────────────────────────────────
	fmt.Print("\n── Summarization (LLM session summaries) ──\n")
	fmt.Println("  Uses an LLM to summarize harvested sessions into condensed memory notes.")
	fmt.Println("  Requires an OpenAI-compatible chat endpoint (Ollama, OpenAI, Anthropic proxy, etc.)")

	sumEnabled := promptWithDefault(scanner, "Enable session summarization? (y/n)", "n")
	sumEnabledBool := sumEnabled == "y" || sumEnabled == "Y"

	var summaryBlock string
	if sumEnabledBool {
		// Auto-detect Ollama as default summarization provider
		defaultSumURL := ""
		defaultSumModel := ""
		if detectOllama(embURL) {
			// Ollama v1-compat endpoint
			defaultSumURL = strings.TrimSuffix(embURL, "/") + "/v1"
			fmt.Printf("  ✓ Ollama detected — defaulting provider_url to %s\n", defaultSumURL)
			fmt.Println("  Make sure you have a chat model pulled (e.g. ollama pull llama3.2)")
			defaultSumModel = "llama3.2"
		}

		sumProviderURL := promptWithDefault(scanner, "LLM provider URL (OpenAI-compatible /v1 base)", defaultSumURL)
		if sumProviderURL == "" {
			fmt.Fprintln(os.Stderr, "  provider_url is required when summarization is enabled.")
			os.Exit(1)
		}
		sumAPIKey := promptWithDefault(scanner, "LLM API key (leave blank to use NANO_BRAIN_SUMMARIZE_API_KEY env)", "")
		if defaultSumModel == "" {
			defaultSumModel = "gpt-4o-mini"
		}
		sumModel := promptWithDefault(scanner, "LLM model name", defaultSumModel)
		fmt.Println("  max_tokens: max output tokens per summary chunk (not input context).")
		sumMaxTokens := promptWithDefault(scanner, "Max tokens per summary", "1024")
		fmt.Println("  concurrency: parallel LLM calls during map phase. Keep low for local Ollama.")
		sumConcurrency := promptWithDefault(scanner, "Parallel LLM calls", "1")
		fmt.Println("  requests_per_second: rate limit to avoid overloading provider (0 = unlimited).")
		sumRPS := promptWithDefault(scanner, "Rate limit (requests/second, 0 = unlimited)", "1")

		apiKeyLine := ""
		if sumAPIKey != "" {
			apiKeyLine = fmt.Sprintf("\n  api_key: %s", sumAPIKey)
		} else {
			apiKeyLine = "\n  # api_key: set NANO_BRAIN_SUMMARIZE_API_KEY env var"
		}
		summaryBlock = fmt.Sprintf("\nsummarization:\n  enabled: true\n  provider_url: %s%s\n  model: %s\n  max_tokens: %s\n  concurrency: %s\n  requests_per_second: %s\n",
			sumProviderURL, apiKeyLine, sumModel, sumMaxTokens, sumConcurrency, sumRPS)
	} else {
		summaryBlock = "\nsummarization:\n  enabled: false\n"
	}

	// ── Search ────────────────────────────────────────────────────────────
	fmt.Print("\n── Search tuning ──\n")
	fmt.Println("  Controls how BM25 + vector results are fused and ranked.")
	fmt.Println("  rrf_k: Reciprocal Rank Fusion constant (higher = smoother blending, default 60).")
	rrfK := promptWithDefault(scanner, "RRF k constant", "60")
	fmt.Println("  recency_weight: boost for recent documents (0 = off, 1 = strong boost, default 0.3).")
	recencyWeight := promptWithDefault(scanner, "Recency weight (0.0–1.0)", "0.3")
	fmt.Println("  recency_half_life_days: days until recency boost halves (default 180).")
	recencyHalfLife := promptWithDefault(scanner, "Recency half-life (days)", "180")
	fmt.Println("  limit: default max results returned per query.")
	searchLimit := promptWithDefault(scanner, "Default result limit", "20")
	searchBlock := fmt.Sprintf("\nsearch:\n  rrf_k: %s\n  recency_weight: %s\n  recency_half_life_days: %s\n  limit: %s\n", rrfK, recencyWeight, recencyHalfLife, searchLimit)

	// ── Watcher ───────────────────────────────────────────────────────────
	fmt.Print("\n── Watcher (file indexing) ──\n")
	fmt.Println("  Watches collection directories for file changes and re-indexes automatically.")
	fmt.Println("  debounce_ms: wait after last file change before triggering re-index (default 2000ms).")
	debounceMs := promptWithDefault(scanner, "Debounce delay (ms)", "2000")
	fmt.Println("  reindex_interval: full re-scan interval in seconds (default 300 = 5 min).")
	reindexInterval := promptWithDefault(scanner, "Full reindex interval (seconds)", "300")
	watcherBlock := fmt.Sprintf("\nwatcher:\n  debounce_ms: %s\n  reindex_interval: %s\n", debounceMs, reindexInterval)

	// ── Logging ───────────────────────────────────────────────────────────
	fmt.Print("\n── Logging ──\n")
	fmt.Println("  level: trace | debug | info | warn | error")
	logLevel := promptWithDefault(scanner, "Log level", "info")
	fmt.Println("  file: path to log file. Uses default path if left blank.")
	logFile := promptWithDefault(scanner, "Log file path", defaultLogPath())
	loggingBlock := fmt.Sprintf("\nlogging:\n  level: %s\n  file: %s\n", logLevel, logFile)

	// ── Assemble YAML ─────────────────────────────────────────────────────
	yaml := fmt.Sprintf(`server:
  host: localhost
  port: %d

database:
  url: %s

%s%s%s%s%s%s`, port, dbURL, embBlock, harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock)

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

	wsDir := promptWithDefault(scanner, "Register workspace directory?", "")
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
