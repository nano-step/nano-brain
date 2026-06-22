// Package reranking provides cross-encoder reranking for search results.
// Supports Cohere and Jina providers via their REST APIs.
package reranking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/rs/zerolog"
)

const (
	defaultTopK         = 20
	defaultTimeout      = 2 * time.Second
	defaultMaxRetries   = 3
	defaultMinScore     = 0.0
	cohereDefaultModel  = "rerank-v4.0-pro"
	jinaDefaultModel    = "jina-reranker-v2-base-multilingual"
	jinaDefaultEndpoint = "https://api.jina.ai/v1/rerank"
)

// apiReranker is a shared implementation for Cohere-compatible rerank APIs.
type apiReranker struct {
	provider   string
	apiKey     string
	endpoint   string
	model      string
	topK       int
	minScore   float64
	maxRetries int
	logger     zerolog.Logger
	httpClient *http.Client
}

func newAPIReranker(provider, apiKey, endpoint, model string, topK int, minScore float64, timeout time.Duration, logger zerolog.Logger) *apiReranker {
	if topK <= 0 {
		topK = defaultTopK
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &apiReranker{
		provider:   provider,
		apiKey:     apiKey,
		endpoint:   endpoint,
		model:      model,
		topK:       topK,
		minScore:   minScore,
		maxRetries: defaultMaxRetries,
		logger:     logger,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (r *apiReranker) Rerank(ctx context.Context, query string, docs []search.Result, topK int) ([]search.Result, error) {
	if r.apiKey == "" || len(docs) == 0 {
		return docs, nil
	}

	if topK <= 0 {
		topK = r.topK
	}
	if topK > len(docs) {
		topK = len(docs)
	}

	docTexts := make([]string, len(docs))
	for i, doc := range docs {
		text := doc.Snippet
		if text == "" {
			text = doc.Content
		}
		if doc.Title != "" {
			text = doc.Title + "\n" + text
		}
		docTexts[i] = text
	}

	type rerankResult struct {
		indices []int
		scores  []float64
		err     error
	}

	var lastErr error
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 500 * time.Millisecond
			select {
			case <-ctx.Done():
				return docs, ctx.Err()
			case <-time.After(backoff):
			}
		}

		indices, scores, err := r.callAPI(ctx, query, docTexts, topK)
		if err == nil {
			result := make([]search.Result, 0, len(indices))
			for i, idx := range indices {
				if idx < 0 || idx >= len(docs) {
					r.logger.Warn().Int("index", idx).Int("max", len(docs)-1).Msg("invalid reranked index")
					return docs, nil
				}
				if r.minScore > 0 && i < len(scores) && scores[i] < r.minScore {
					continue
				}
				result = append(result, docs[idx])
			}
			return result, nil
		}

		lastErr = err
		if !isRetryable(err) {
			break
		}
		r.logger.Warn().Err(err).Int("attempt", attempt+1).Msg("reranking failed, retrying")
	}

	r.logger.Warn().Err(lastErr).Str("provider", r.provider).Msg("reranking failed after retries, returning original results")
	return docs, nil
}

func (r *apiReranker) callAPI(ctx context.Context, query string, documents []string, topN int) ([]int, []float64, error) {
	reqBody := map[string]interface{}{
		"model":            r.model,
		"query":            query,
		"documents":        documents,
		"top_n":            topN,
		"return_documents": false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, nil, fmt.Errorf("%s API HTTP %d: %s", r.provider, resp.StatusCode, string(body))
	}

	var apiResp struct {
		Results []struct {
			Index          int     `json:"index"`
			RelevanceScore float64 `json:"relevance_score"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, nil, fmt.Errorf("decode response: %w", err)
	}

	indices := make([]int, len(apiResp.Results))
	scores := make([]float64, len(apiResp.Results))
	for i, result := range apiResp.Results {
		if result.Index < 0 || result.Index >= len(documents) {
			return nil, nil, fmt.Errorf("invalid index %d for document count %d", result.Index, len(documents))
		}
		indices[i] = result.Index
		scores[i] = result.RelevanceScore
	}

	return indices, scores, nil
}

func isRetryable(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "429") || strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") || strings.Contains(msg, "503") ||
		strings.Contains(msg, "timeout") || strings.Contains(msg, "connection refused")
}

// NewReranker creates a Reranker from config.
func NewReranker(cfg config.RerankingConfig, logger zerolog.Logger) search.Reranker {
	if !cfg.Enabled {
		return &noopReranker{}
	}

	timeout := time.Duration(cfg.MaxLatencyMs) * time.Millisecond
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	switch strings.ToLower(cfg.Provider) {
	case "cohere":
		endpoint := cfg.ProviderURL
		if endpoint == "" {
			endpoint = "https://api.cohere.com/v2/rerank"
		}
		model := cfg.Model
		if model == "" {
			model = cohereDefaultModel
		}
		return newAPIReranker("cohere", cfg.APIKey, endpoint, model, cfg.TopK, cfg.MinScore, timeout, logger)

	case "jina":
		endpoint := cfg.ProviderURL
		if endpoint == "" {
			endpoint = jinaDefaultEndpoint
		}
		model := cfg.Model
		if model == "" {
			model = jinaDefaultModel
		}
		return newAPIReranker("jina", cfg.APIKey, endpoint, model, cfg.TopK, cfg.MinScore, timeout, logger)

	default:
		logger.Warn().Str("provider", cfg.Provider).Msg("unknown reranker provider, using noop")
		return &noopReranker{}
	}
}

type noopReranker struct{}

func (r *noopReranker) Rerank(ctx context.Context, query string, docs []search.Result, topK int) ([]search.Result, error) {
	return docs, nil
}
