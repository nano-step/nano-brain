// Package reranking provides cross-encoder reranking for search results.
// It can reorder candidate documents using API-based reranking models.
package reranking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/rs/zerolog"
)

// Config holds reranker configuration.
type Config struct {
	Enabled  bool   `koanf:"enabled" json:"enabled"`
	Provider string `koanf:"provider" json:"provider"`
	APIKey   string `koanf:"api_key" json:"api_key"`
	TopK     int    `koanf:"top_k" json:"top_k"`
}

// CohereReranker implements the search.Reranker interface using Cohere's rerank API.
type CohereReranker struct {
	apiKey     string
	model      string
	topK       int
	logger     zerolog.Logger
	httpClient *http.Client
}

// NewCohereReranker creates a new CohereReranker with the given configuration.
func NewCohereReranker(cfg Config, logger zerolog.Logger) *CohereReranker {
	if cfg.TopK <= 0 {
		cfg.TopK = 20
	}

	return &CohereReranker{
		apiKey: cfg.APIKey,
		model:  "rerank-v4.0-pro",
		topK:   cfg.TopK,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// Rerank implements the Reranker interface for Cohere.
func (r *CohereReranker) Rerank(ctx context.Context, query string, docs []search.Result, topK int) ([]search.Result, error) {
	// If disabled or no documents, return original
	if r.apiKey == "" || len(docs) == 0 {
		return docs, nil
	}

	// Use parameter topK if provided, otherwise use configured topK
	if topK <= 0 {
		topK = r.topK
	}
	if topK > len(docs) {
		topK = len(docs)
	}

	// Prepare documents for API call
	docTexts := make([]string, len(docs))
	for i, doc := range docs {
		// Prefer snippet if available, otherwise content
		if doc.Snippet != "" {
			docTexts[i] = doc.Snippet
		} else {
			docTexts[i] = doc.Content
		}
	}

	// Call Cohere API
	rerankedDocs, err := r.callAPI(ctx, query, docTexts, topK)
	if err != nil {
		r.logger.Warn().Err(err).Str("query", query).Msg("reranking failed, returning original results")
		return docs, nil // Graceful degradation: return original docs
	}

	// Map reranked indices back to original documents
	result := make([]search.Result, len(rerankedDocs))
	for i, idx := range rerankedDocs {
		if idx < 0 || idx >= len(docs) {
			// This shouldn't happen, but be safe
			r.logger.Warn().Int("index", idx).Int("max", len(docs)-1).Msg("invalid reranked index")
			return docs, nil
		}
		result[i] = docs[idx]
	}

	return result, nil
}

func (r *CohereReranker) callAPI(ctx context.Context, query string, documents []string, topN int) ([]int, error) {
	reqBody := map[string]interface{}{
		"model":             r.model,
		"query":             query,
		"documents":         documents,
		"top_n":             topN,
		"return_documents":  false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.cohere.com/v2/rerank", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("cohere API HTTP %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Results []struct {
			Index           int     `json:"index"`
			RelevanceScore  float64 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	indices := make([]int, len(apiResp.Results))
	for i, result := range apiResp.Results {
		if result.Index < 0 || result.Index >= len(documents) {
			return nil, fmt.Errorf("invalid index %d for document count %d", result.Index, len(documents))
		}
		indices[i] = result.Index
	}

	return indices, nil
}

func NewReranker(cfg config.RerankingConfig, logger zerolog.Logger) search.Reranker {
	if !cfg.Enabled {
		return &noopReranker{}
	}

	switch strings.ToLower(cfg.Provider) {
	case "cohere":
		return NewCohereReranker(Config{
			Enabled:  cfg.Enabled,
			Provider: cfg.Provider,
			APIKey:   cfg.APIKey,
			TopK:     cfg.TopK,
		}, logger)
	default:
		logger.Warn().Str("provider", cfg.Provider).Msg("unknown reranker provider, using noop")
		return &noopReranker{}
	}
}

type noopReranker struct{}

func (r *noopReranker) Rerank(ctx context.Context, query string, docs []search.Result, topK int) ([]search.Result, error) {
	return docs, nil
}