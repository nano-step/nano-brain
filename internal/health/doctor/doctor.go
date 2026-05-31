// Package doctor runs prerequisite health checks as pure functions.
package doctor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nano-brain/nano-brain/internal/config"
)

// Check is a single doctor result.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Hint   string `json:"hint,omitempty"`
}

// RunAll executes all checks and returns results.
func RunAll(configPath string, cfg *config.Config, cfgErr error) []Check {
	var results []Check

	results = append(results, CheckConfig(configPath, cfgErr))

	if cfgErr != nil && cfg == nil {
		cfg = &config.Config{}
	}

	pgResult, conn := CheckPostgreSQL(cfg.Database.URL)
	results = append(results, pgResult)
	if conn != nil {
		results = append(results, CheckPgvector(conn))
		conn.Close(context.Background())
	} else {
		results = append(results, Check{Name: "pgvector", Status: "skip", Detail: "no connection"})
	}

	provResult, ollamaBody := CheckEmbeddingProvider(cfg.Embedding)
	results = append(results, provResult)
	results = append(results, CheckEmbeddingModel(cfg.Embedding, ollamaBody))

	return results
}

func CheckConfig(path string, err error) Check {
	if err != nil {
		return Check{Name: "Config", Status: "fail", Detail: path, Hint: "Run nano-brain to generate default config"}
	}
	return Check{Name: "Config", Status: "ok", Detail: path}
}

func CheckPostgreSQL(dbURL string) (Check, *pgx.Conn) {
	if dbURL == "" {
		return Check{Name: "PostgreSQL", Status: "fail", Detail: "no URL configured", Hint: "Set database.url in config or DATABASE_URL env"}, nil
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
		return Check{Name: "PostgreSQL", Status: "fail", Detail: host, Hint: "Is PostgreSQL running?\nTry: docker compose up -d"}, nil
	}

	if err := conn.Ping(ctx); err != nil {
		conn.Close(ctx)
		return Check{Name: "PostgreSQL", Status: "fail", Detail: host, Hint: "Connection refused. Is PostgreSQL running?\nTry: docker compose up -d"}, nil
	}

	return Check{Name: "PostgreSQL", Status: "ok", Detail: host}, conn
}

func CheckPgvector(conn *pgx.Conn) Check {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var version string
	err := conn.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname = 'vector'").Scan(&version)
	if err != nil {
		return Check{Name: "pgvector", Status: "fail", Detail: "not installed", Hint: "Install pgvector: CREATE EXTENSION vector"}
	}
	return Check{Name: "pgvector", Status: "ok", Detail: version}
}

func CheckEmbeddingProvider(cfg config.EmbeddingConfig) (Check, []byte) {
	if cfg.Provider == "voyage" {
		key := cfg.VoyageAPIKey
		if key == "" {
			return Check{Name: "Embedding provider", Status: "fail", Detail: "voyage", Hint: "Set VOYAGE_API_KEY environment variable"}, nil
		}
		return Check{Name: "Embedding provider", Status: "ok", Detail: "voyage (API key set)"}, nil
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
		return Check{Name: "Embedding provider", Status: "fail", Detail: host, Hint: "Is Ollama running? Try: ollama serve"}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	parsed, _ := url.Parse(ollamaURL)
	host := ollamaURL
	if parsed != nil {
		host = parsed.Host
	}

	if resp.StatusCode != 200 {
		return Check{Name: "Embedding provider", Status: "fail", Detail: host, Hint: "Ollama returned HTTP " + resp.Status}, body
	}
	return Check{Name: "Embedding provider", Status: "ok", Detail: host}, body
}

func CheckEmbeddingModel(cfg config.EmbeddingConfig, ollamaBody []byte) Check {
	model := cfg.Model
	if model == "" {
		model = "nomic-embed-text"
	}

	if cfg.Provider == "voyage" {
		return Check{Name: "Embedding model", Status: "ok", Detail: model}
	}

	if ollamaBody == nil {
		return Check{Name: "Embedding model", Status: "skip", Detail: model}
	}

	var resp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(ollamaBody, &resp); err != nil {
		return Check{Name: "Embedding model", Status: "fail", Detail: model, Hint: "Could not parse Ollama response"}
	}

	for _, m := range resp.Models {
		name := strings.TrimSuffix(m.Name, ":latest")
		if name == model || m.Name == model {
			return Check{Name: "Embedding model", Status: "ok", Detail: model}
		}
	}

	return Check{Name: "Embedding model", Status: "fail", Detail: model, Hint: "Pull model: ollama pull " + model}
}
