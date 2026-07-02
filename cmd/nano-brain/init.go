package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/health/doctor"
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

// Orchestrator seams (D-01..D-04, D-16, D-17): each defaults to the real
// Wave-1/2 step function so production behavior is unchanged, but tests can
// override them to drive runInteractiveInit through canned outcomes without
// touching Docker, a real DB, the daemon, or the network.
var (
	stepDatabaseFn          = stepDatabase
	stepEmbeddingFn         = stepEmbedding
	runDoctorChecksFn       = defaultRunDoctorChecks
	stepServeFn             = stepServe
	registerWorkspaceFn     = registerWorkspace
	promptMCPClientConfigFn = promptMCPClientConfig
)

// defaultRunDoctorChecks loads config and runs doctor.RunAll, printing the
// same per-check table runDoctorCmd prints — but, unlike runDoctorCmd, it
// never os.Exit(1)s on a failing check. The wizard needs the checks (in
// particular whether PostgreSQL is healthy) without being killed before it
// can reach the serve gate (stepServe), which already knows how to react to
// a PostgreSQL failure (serveAborted).
func defaultRunDoctorChecks(configPath string) []doctor.Check {
	cfg, cfgErr := config.Load(configPath)
	results := doctor.RunAll(configPath, cfg, cfgErr, resolveBinaryPath())

	fmt.Print("\nnano-brain doctor\n\n")
	for _, r := range results {
		label := padRight(r.Name, 22)
		detail := padRight(r.Detail, 28)
		status := "OK"
		if r.Status == "fail" {
			status = "FAIL"
		} else if r.Status == "skip" {
			status = "SKIP"
		} else if r.Status == "warn" {
			status = "WARN"
		}
		fmt.Printf("  %s %s %s\n", label, detail, status)
		if (r.Status == "fail" || r.Status == "warn") && r.Hint != "" {
			for _, line := range strings.Split(r.Hint, "\n") {
				fmt.Printf("    → %s\n", line)
			}
		}
	}
	fmt.Println()

	return results
}

// runInteractiveInit is the "one command" wizard orchestrator (RESEARCH
// System Architecture Diagram): TTY gate (D-04) → config-exists [k]eep/
// [o]verwrite gate (D-03) → database step → embedding step → advanced gate
// (D-02) → assemble+write → doctor → serve step → register step → MCP
// config (D-16, once) → summary (D-17). It composes the Wave-1/2 step
// functions via the seams above; it does not re-implement their logic.
func runInteractiveInit(configPath string) {
	if configPath == "" {
		configPath = config.ResolveConfigPath("")
	}

	// D-04: interactive init requires a TTY. Non-interactive callers (CI,
	// scripts, piped stdin) get pointed at the flag-driven path instead of
	// silently hanging on a prompt read or writing a config with no consent.
	if !isTTYFn() {
		fmt.Println("nano-brain init needs an interactive terminal (TTY).")
		fmt.Println("For non-interactive setup, use: nano-brain init --root <path> --json")
		return
	}

	dbURL := "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev"
	embURL := "http://localhost:11434"
	model := "nomic-embed-text"
	port := 3100

	fmt.Print("\nnano-brain setup\n────────────────\n\n")

	scanner := bufio.NewScanner(os.Stdin)

	// D-03: config-exists gate defaults to KEEP (not overwrite). Keep skips
	// straight to the service steps (doctor → serve → register → MCP) with
	// zero config questions.
	keep := false
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  Config exists at %s\n", configPath)
		answer, ok := promptConsequential(scanner, "[k]eep/[o]verwrite", "keep")
		if !ok || (!strings.EqualFold(answer, "o") && !strings.EqualFold(answer, "overwrite")) {
			keep = true
			fmt.Println("  Keeping existing config.")
		}
		fmt.Println()
	}

	if !keep {
		// ── Database ──────────────────────────────────────────────────────
		var dbOK bool
		dbURL, dbOK = stepDatabaseFn(scanner, dbURL)
		if !dbOK {
			fmt.Println("Aborted.")
			return
		}

		// ── Embedding ─────────────────────────────────────────────────────
		var notes bytes.Buffer
		embBlock := stepEmbeddingFn(scanner, &notes, embURL, model)
		if notes.Len() > 0 {
			fmt.Print(notes.String())
		}

		// ── Advanced gate (D-02) ────────────────────────────────────────
		var harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock string
		advAnswer, advOK := promptConsequential(scanner, "Advanced settings?", "N")
		if advOK && isAffirmative(advAnswer) {
			harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock = stepAdvanced(scanner, embURL)
		} else {
			harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock = defaultAdvancedBlocks()
		}

		// ── Assemble + preview + write ────────────────────────────────────
		yaml := fmt.Sprintf(`server:
  host: localhost
  port: %d

database:
  url: %s

%s%s%s%s%s%s`, port, dbURL, embBlock, harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock)

		fmt.Println("\n── Config preview ──────────────")
		fmt.Print(yaml)
		fmt.Println("────────────────────────────────")

		answer, ok := promptConsequential(scanner, "Save this config?", "Y")
		if !ok || !isAffirmative(answer) {
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
	}

	// ── Doctor ──────────────────────────────────────────────────────────
	checks := runDoctorChecksFn(configPath)

	// ── Serve (D-14) ──────────────────────────────────────────────────────
	outcome := stepServeFn(scanner, checks, configPath)
	if outcome == serveAborted {
		os.Exit(1)
	}

	// ── Register (D-15) ───────────────────────────────────────────────────
	var res initResult
	registered := false
	if outcome == serveStarted || outcome == serveAlreadyRunning {
		cwd, _ := os.Getwd()
		wsDir, ok := promptConsequential(scanner, "Register this directory as a workspace?", cwd)
		if ok && wsDir != "" {
			var err error
			res, err = registerWorkspaceFn(wsDir, "", false)
			if err != nil {
				fmt.Printf("  Registration failed: %v\n", err)
			} else {
				registered = true
			}
		} else {
			fmt.Println("  Skipped registration — MCP client config needs a registered workspace.")
		}
	}

	// ── Summary (D-17) ────────────────────────────────────────────────────
	// Never echo the DB password or any API key here — only the host/port
	// via getBaseURL() and the workspace name/hash are printed.
	fmt.Println("\n── Summary ──────────────────────")
	fmt.Printf("  Server: %s\n", getBaseURL())
	if registered {
		fmt.Printf("  Workspace: %s (%s)\n", res.Name, res.WorkspaceHash)
		fmt.Println("  MCP clients you selected above are now configured for this workspace.")
	} else {
		fmt.Println("  No workspace registered — MCP client config was skipped.")
	}
	fmt.Println("  Next: restart your AI client to pick up the new MCP configuration.")
	fmt.Println("──────────────────────────────────")
}

// defaultAdvancedBlocks returns the silent-default YAML blocks used when the
// D-02 advanced gate is declined: harvester auto-detects storage dirs with
// no prompt, summarization is disabled, and search/watcher/logging fall
// back to the same defaults the detailed prompts otherwise offer.
func defaultAdvancedBlocks() (harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock string) {
	detectedOC := detectOpenCodeStorageDir()
	detectedCC := detectClaudeCodeStorageDir()

	ocLine := "\"\""
	if detectedOC != "" {
		ocLine = detectedOC
	}
	ccEnabled := detectedCC != ""
	ccLine := "\"\""
	if detectedCC != "" {
		ccLine = detectedCC
	}

	harvesterBlock = fmt.Sprintf("\nharvester:\n  opencode:\n    session_dir: %s\n  claudecode:\n    enabled: %v\n    session_dir: %s\n\nintervals:\n  session_poll: %d\n", ocLine, ccEnabled, ccLine, 120)
	summaryBlock = "\nsummarization:\n  enabled: false\n"
	searchBlock = fmt.Sprintf("\nsearch:\n  rrf_k: %s\n  recency_weight: %s\n  recency_half_life_days: %s\n  limit: %s\n", "60", "0.3", "180", "20")
	watcherBlock = fmt.Sprintf("\nwatcher:\n  debounce_ms: %s\n  reindex_interval: %s\n", "2000", "300")
	loggingBlock = fmt.Sprintf("\nlogging:\n  level: %s\n  file: %s\n", "info", defaultLogPath())
	return harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock
}

// stepAdvanced runs the detailed harvester/summarization/search/watcher/
// logging prompt sequence, preserved verbatim (byte-for-byte behavior) from
// the pre-restructure runInteractiveInit, now gated behind the D-02
// "Advanced settings?" confirmation instead of always running.
func stepAdvanced(scanner *bufio.Scanner, embURL string) (harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock string) {
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
	harvesterBlock = fmt.Sprintf("\nharvester:\n  opencode:\n    session_dir: %s\n  claudecode:\n    enabled: %v\n    session_dir: %s\n\nintervals:\n  session_poll: %d\n", ocLine, ccEnabled, ccLine, sessionPoll)

	// ── Summarization ─────────────────────────────────────────────────────
	fmt.Print("\n── Summarization (LLM session summaries) ──\n")
	fmt.Println("  Uses an LLM to summarize harvested sessions into condensed memory notes.")
	fmt.Println("  Requires an OpenAI-compatible chat endpoint (Ollama, OpenAI, Anthropic proxy, etc.)")

	sumEnabled := promptWithDefault(scanner, "Enable session summarization? (y/n)", "n")
	sumEnabledBool := sumEnabled == "y" || sumEnabled == "Y"

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
	searchBlock = fmt.Sprintf("\nsearch:\n  rrf_k: %s\n  recency_weight: %s\n  recency_half_life_days: %s\n  limit: %s\n", rrfK, recencyWeight, recencyHalfLife, searchLimit)

	// ── Watcher ───────────────────────────────────────────────────────────
	fmt.Print("\n── Watcher (file indexing) ──\n")
	fmt.Println("  Watches collection directories for file changes and re-indexes automatically.")
	fmt.Println("  debounce_ms: wait after last file change before triggering re-index (default 2000ms).")
	debounceMs := promptWithDefault(scanner, "Debounce delay (ms)", "2000")
	fmt.Println("  reindex_interval: full re-scan interval in seconds (default 300 = 5 min).")
	reindexInterval := promptWithDefault(scanner, "Full reindex interval (seconds)", "300")
	watcherBlock = fmt.Sprintf("\nwatcher:\n  debounce_ms: %s\n  reindex_interval: %s\n", debounceMs, reindexInterval)

	// ── Logging ───────────────────────────────────────────────────────────
	fmt.Print("\n── Logging ──\n")
	fmt.Println("  level: trace | debug | info | warn | error")
	logLevel := promptWithDefault(scanner, "Log level", "info")
	fmt.Println("  file: path to log file. Uses default path if left blank.")
	logFile := promptWithDefault(scanner, "Log file path", defaultLogPath())
	loggingBlock = fmt.Sprintf("\nlogging:\n  level: %s\n  file: %s\n", logLevel, logFile)

	return harvesterBlock, summaryBlock, searchBlock, watcherBlock, loggingBlock
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
