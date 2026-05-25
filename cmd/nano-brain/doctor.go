package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nano-brain/nano-brain/internal/config"
)

type checkResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

func runDoctorCmd(args []string, configPath string) {
	cliLog.Debug().Str("cmd", "doctor").Msg("cli command started")
	var jsonFlag bool
	for _, a := range args {
		if a == "--json" {
			jsonFlag = true
		}
	}

	if configPath == "" {
		configPath = config.DefaultConfigPath()
	}

	var results []checkResult

	cfg, cfgErr := config.Load(configPath)
	results = append(results, checkConfig(configPath, cfgErr))

	if cfgErr != nil {
		if cfg == nil {
			cfg = &config.Config{}
		}
	}

	pgResult, conn := checkPostgreSQL(cfg.Database.URL)
	results = append(results, pgResult)
	if conn != nil {
		results = append(results, checkPgvector(conn))
		conn.Close(context.Background())
	} else {
		results = append(results, checkResult{Name: "pgvector", Status: "skip", Detail: "no connection"})
	}

	provResult, ollamaBody := checkEmbeddingProvider(cfg.Embedding)
	results = append(results, provResult)
	results = append(results, checkEmbeddingModel(cfg.Embedding, ollamaBody))

	allPassed := true
	for _, r := range results {
		if r.Status == "fail" {
			allPassed = false
			break
		}
	}

	if jsonFlag {
		out := struct {
			Checks    []checkResult `json:"checks"`
			AllPassed bool          `json:"all_passed"`
		}{results, allPassed}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
	} else {
		fmt.Print("\nnano-brain doctor\n\n")
		for _, r := range results {
			label := padRight(r.Name, 22)
			detail := padRight(r.Detail, 28)
			status := "OK"
			if r.Status == "fail" {
				status = "FAIL"
			} else if r.Status == "skip" {
				status = "SKIP"
			}
			fmt.Printf("  %s %s %s\n", label, detail, status)
			if r.Status == "fail" && r.Hint != "" {
				for _, line := range strings.Split(r.Hint, "\n") {
					fmt.Printf("    → %s\n", line)
				}
			}
		}
		fmt.Println()
		if allPassed {
			fmt.Println("All checks passed.")
		} else {
			fmt.Println("Some checks failed.")
		}
		fmt.Println()
	}

	cliLog.Debug().Str("cmd", "doctor").Bool("all_passed", allPassed).Int("checks", len(results)).Msg("cli command completed")

	if !allPassed {
		os.Exit(1)
	}
}

func checkConfig(path string, err error) checkResult {
	if err != nil {
		return checkResult{Name: "Config", Status: "fail", Detail: path, Hint: "Run nano-brain to generate default config"}
	}
	return checkResult{Name: "Config", Status: "ok", Detail: path}
}

func checkPostgreSQL(dbURL string) (checkResult, *pgx.Conn) {
	if dbURL == "" {
		return checkResult{Name: "PostgreSQL", Status: "fail", Detail: "no URL configured", Hint: "Set database.url in config or DATABASE_URL env"}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	parsed, _ := url.Parse(dbURL)
	host := "unknown"
	if parsed != nil {
		host = parsed.Host
	}

	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return checkResult{Name: "PostgreSQL", Status: "fail", Detail: host, Hint: "Is PostgreSQL running?\nTry: docker compose up -d"}, nil
	}

	if err := conn.Ping(ctx); err != nil {
		conn.Close(ctx)
		return checkResult{Name: "PostgreSQL", Status: "fail", Detail: host, Hint: "Connection refused. Is PostgreSQL running?\nTry: docker compose up -d"}, nil
	}

	return checkResult{Name: "PostgreSQL", Status: "ok", Detail: host}, conn
}

func checkPgvector(conn *pgx.Conn) checkResult {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var version string
	err := conn.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&version)
	if err != nil {
		return checkResult{Name: "pgvector", Status: "fail", Detail: "not installed", Hint: "Install pgvector: CREATE EXTENSION vector"}
	}
	return checkResult{Name: "pgvector", Status: "ok", Detail: version}
}

func checkEmbeddingProvider(cfg config.EmbeddingConfig) (checkResult, []byte) {
	if cfg.Provider == "voyage" {
		key := cfg.VoyageAPIKey
		if key == "" {
			key = os.Getenv("VOYAGE_API_KEY")
		}
		if key == "" {
			return checkResult{Name: "Embedding provider", Status: "fail", Detail: "voyage", Hint: "Set VOYAGE_API_KEY environment variable"}, nil
		}
		return checkResult{Name: "Embedding provider", Status: "ok", Detail: "voyage (API key set)"}, nil
	}

	if cfg.Provider == "" {
		cfg.Provider = "ollama"
	}
	ollamaURL := cfg.URL
	if ollamaURL == "" {
		ollamaURL = "http://localhost:11434"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ollamaURL+"/api/tags", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		parsed, _ := url.Parse(ollamaURL)
		host := ollamaURL
		if parsed != nil {
			host = parsed.Host
		}
		return checkResult{Name: "Embedding provider", Status: "fail", Detail: host, Hint: "Is Ollama running? Try: ollama serve"}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	parsed, _ := url.Parse(ollamaURL)
	host := ollamaURL
	if parsed != nil {
		host = parsed.Host
	}

	if resp.StatusCode != 200 {
		return checkResult{Name: "Embedding provider", Status: "fail", Detail: host, Hint: "Ollama returned HTTP " + resp.Status}, body
	}
	return checkResult{Name: "Embedding provider", Status: "ok", Detail: host}, body
}

func checkEmbeddingModel(cfg config.EmbeddingConfig, ollamaBody []byte) checkResult {
	model := cfg.Model
	if model == "" {
		model = "nomic-embed-text"
	}

	if cfg.Provider == "voyage" {
		return checkResult{Name: "Embedding model", Status: "ok", Detail: model}
	}

	if ollamaBody == nil {
		return checkResult{Name: "Embedding model", Status: "skip", Detail: model}
	}

	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(ollamaBody, &resp); err != nil {
		return checkResult{Name: "Embedding model", Status: "fail", Detail: model, Hint: "Could not parse Ollama response"}
	}

	for _, m := range resp.Models {
		name := strings.TrimSuffix(m.Name, ":latest")
		if name == model || m.Name == model {
			return checkResult{Name: "Embedding model", Status: "ok", Detail: model}
		}
	}

	return checkResult{Name: "Embedding model", Status: "fail", Detail: model, Hint: "Pull model: ollama pull " + model}
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(".", n-len(s))
}
