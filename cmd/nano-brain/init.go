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

	if cfg, err := config.Load(configPath); err == nil {
		if cfg.Database.URL != "" {
			dbURL = cfg.Database.URL
		}
		if cfg.Embedding.Provider != "" {
			provider = cfg.Embedding.Provider
		}
		if cfg.Embedding.URL != "" {
			embURL = cfg.Embedding.URL
		}
		if cfg.Embedding.Model != "" {
			model = cfg.Embedding.Model
		}
		if cfg.Server.Port > 0 {
			port = cfg.Server.Port
		}
	}

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

	var harvesterSessionDir string
	if detected := detectOpenCodeStorageDir(); detected != "" {
		fmt.Printf("  OpenCode detected at %s\n", detected)
		answer := promptWithDefault(scanner, "Enable session harvesting?", "Y")
		if answer != "n" && answer != "N" {
			harvesterSessionDir = detected
		}
	}

	var harvesterBlock string
	if harvesterSessionDir != "" {
		harvesterBlock = fmt.Sprintf("\nharvester:\n  opencode:\n    session_dir: %s\n  claudecode:\n    enabled: false\n    session_dir: \"\"\n", harvesterSessionDir)
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
%s`, port, dbURL, embBlock, harvesterBlock)

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
