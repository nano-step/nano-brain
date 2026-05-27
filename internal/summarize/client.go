// Package summarize provides an OpenAI-compatible chat completion client
// for session summarization via any OpenAI-compatible API endpoint.
package summarize

import (
	"bufio"
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
	"golang.org/x/time/rate"
)

// TokenUsage holds token counts from an LLM completion response.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Client is an OpenAI-compatible chat completion client.
type Client struct {
	providerURL string
	apiKey      string
	model       string
	maxTokens   int
	httpClient  *http.Client
	limiter     *rate.Limiter
	logger      zerolog.Logger

	backoff func(attempt int) time.Duration
}

// New creates a Client from config.
func New(cfg config.SummarizationConfig, logger zerolog.Logger) *Client {
	rps := cfg.RequestsPerSecond
	if rps <= 0 {
		rps = 1
	}
	return &Client{
		providerURL: strings.TrimRight(cfg.ProviderURL, "/"),
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		limiter: rate.NewLimiter(rate.Limit(rps), 1),
		logger:  logger,
		backoff: func(attempt int) time.Duration {
			return time.Duration(1<<uint(attempt)) * time.Second
		},
	}
}

type chatRequest struct {
	Model     string        `json:"model"`
	MaxTokens int           `json:"max_tokens"`
	Stream    bool          `json:"stream"`
	Messages  []chatMessage `json:"messages"`
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
	Usage tokenUsageJSON `json:"usage"`
}

type sseChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *tokenUsageJSON `json:"usage,omitempty"`
}

type tokenUsageJSON struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletion sends a chat completion request and returns the response text,
// token usage, and any error. It handles both streaming (SSE) and non-streaming
// responses, and retries on transient errors (429, 5xx).
func (c *Client) ChatCompletion(ctx context.Context, systemPrompt, userPrompt string) (string, TokenUsage, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return "", TokenUsage{}, fmt.Errorf("summarize: rate limiter: %w", err)
	}

	start := time.Now()

	reqBody := chatRequest{
		Model:     c.model,
		MaxTokens: c.maxTokens,
		Stream:    true,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("summarize: marshal request: %w", err)
	}

	endpoint := c.providerURL + "/chat/completions"

	var lastErr error
	maxAttempts := 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			delay := c.backoff(attempt - 1)
			select {
			case <-ctx.Done():
				return "", TokenUsage{}, ctx.Err()
			case <-time.After(delay):
			}
		}

		content, usage, err := c.doRequest(ctx, endpoint, bodyBytes)
		if err == nil {
			latency := time.Since(start).Milliseconds()
			c.logger.Info().
				Str("model", c.model).
				Int("prompt_tokens", usage.PromptTokens).
				Int("completion_tokens", usage.CompletionTokens).
				Int64("latency_ms", latency).
				Msg("summarize: chat completion succeeded")
			return content, usage, nil
		}

		lastErr = err

		if !isRetryable(err) {
			break
		}
	}

	latency := time.Since(start).Milliseconds()
	attempts := maxAttempts
	if !isRetryable(lastErr) {
		attempts = 1
	}
	c.logger.Warn().
		Str("model", c.model).
		Err(lastErr).
		Int("attempts", attempts).
		Int64("latency_ms", latency).
		Msg("summarize: chat completion failed")

	return "", TokenUsage{}, lastErr
}

type statusError struct {
	code    int
	message string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("summarize: HTTP %d: %s", e.code, e.message)
}

// isRetryable returns true for 429 and 5xx errors.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	se, ok := err.(*statusError)
	if !ok {
		return true
	}
	return se.code == 429 || se.code >= 500
}

func (c *Client) doRequest(ctx context.Context, endpoint string, body []byte) (string, TokenUsage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("summarize: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", TokenUsage{}, fmt.Errorf("summarize: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", TokenUsage{}, &statusError{
			code:    resp.StatusCode,
			message: string(respBody),
		}
	}

	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
		return c.parseSSE(resp.Body)
	}
	return c.parseJSON(resp.Body)
}

func (c *Client) parseJSON(body io.Reader) (string, TokenUsage, error) {
	var resp chatResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return "", TokenUsage{}, fmt.Errorf("summarize: decode response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", TokenUsage{}, fmt.Errorf("summarize: empty choices in response")
	}
	usage := TokenUsage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}
	return resp.Choices[0].Message.Content, usage, nil
}

func (c *Client) parseSSE(body io.Reader) (string, TokenUsage, error) {
	var content strings.Builder
	var usage TokenUsage

	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		if data == "[DONE]" {
			break
		}

		var chunk sseChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			content.WriteString(chunk.Choices[0].Delta.Content)
		}

		if chunk.Usage != nil {
			usage = TokenUsage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", TokenUsage{}, fmt.Errorf("summarize: read SSE stream: %w", err)
	}

	return content.String(), usage, nil
}
