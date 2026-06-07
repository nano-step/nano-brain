// Package hyde implements Hypothetical Document Embedding (HyDE) generation.
package hyde

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
	"github.com/rs/zerolog"
)

// Generator implements HyDE (Hypothetical Document Embedding) generation.
// It takes a search query, asks an LLM to "write a passage that would be the ideal answer to this query",
// and returns that hypothetical document text for embedding.
type Generator struct {
	httpClient  *http.Client
	providerURL string
	apiKey      string
	model       string
	logger      zerolog.Logger
}

// NewGenerator creates a new HyDE generator from configuration.
func NewGenerator(cfg config.HyDEConfig, logger zerolog.Logger) *Generator {
	timeout := time.Duration(cfg.MaxLatencyMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &Generator{
		httpClient:  &http.Client{Timeout: timeout},
		providerURL: strings.TrimRight(cfg.ProviderURL, "/"),
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		logger:      logger,
	}
}

// Generate takes a search query and returns a hypothetical document text that would
// be the ideal answer to the query. On any error or timeout, returns an empty string.
func (g *Generator) Generate(ctx context.Context, query string) (string, error) {
	if g.providerURL == "" || g.model == "" {
		return "", nil
	}

	result, err := g.callLLM(ctx, query)
	if err != nil {
		g.logger.Warn().Err(err).Msg("hyde: LLM call failed, returning empty")
		return "", nil
	}

	return result, nil
}

const systemPrompt = `You are a technical documentation writer. Given a search query, write a concise paragraph (80-120 words) that would be the ideal answer to the query. Write in a factual, technical documentation style. Focus on explaining concepts clearly. Respond with ONLY the paragraph text, no JSON, no markdown.

Example:
Query: "How to handle rate limiting in Go"
Response: "Rate limiting in Go is commonly implemented using token bucket or leaky bucket algorithms. The standard library provides rate.Limiter which implements a token bucket. For distributed rate limiting, you can use Redis with Lua scripts to ensure atomic operations. Middleware patterns in web frameworks like Echo or Gin allow centralized rate limiting across all endpoints. Monitoring key metrics like request rate, throttle rate, and error rates is essential for tuning limits effectively."

Now, respond with only the paragraph for the given query.`

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (g *Generator) callLLM(ctx context.Context, query string) (string, error) {
	reqBody := chatRequest{
		Model: g.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: query},
		},
		Stream: false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("hyde: marshal request: %w", err)
	}

	endpoint := g.providerURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("hyde: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("hyde: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("hyde: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("hyde: decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("hyde: empty choices in response")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	return strings.TrimSpace(content), nil
}