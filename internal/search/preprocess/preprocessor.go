package preprocess

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

type Intent string

const (
	IntentKeyword    Intent = "keyword"
	IntentConceptual Intent = "conceptual"
	IntentTemporal   Intent = "temporal"
)

type TimeFilter struct {
	After  *time.Time `json:"after,omitempty"`
	Before *time.Time `json:"before,omitempty"`
}

type PreprocessResult struct {
	OriginalQuery string     `json:"original_query"`
	EnglishQuery  string     `json:"english_query"`
	Intent        Intent     `json:"intent"`
	Expansions    []string   `json:"expansions"`
	TimeFilter    *TimeFilter `json:"time_filter,omitempty"`
	Language      string     `json:"language"`
}

type Preprocessor struct {
	httpClient  *http.Client
	providerURL string
	apiKey      string
	model       string
	logger      zerolog.Logger
}

func NewPreprocessor(cfg config.QueryPreprocessingConfig, logger zerolog.Logger) *Preprocessor {
	timeout := time.Duration(cfg.MaxLatencyMs) * time.Millisecond
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &Preprocessor{
		httpClient:  &http.Client{Timeout: timeout},
		providerURL: strings.TrimRight(cfg.ProviderURL, "/"),
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		logger:      logger,
	}
}

func (p *Preprocessor) Process(ctx context.Context, query string) *PreprocessResult {
	fallback := &PreprocessResult{
		OriginalQuery: query,
		EnglishQuery:  query,
		Intent:        IntentKeyword,
		Language:      "en",
	}

	if p.providerURL == "" || p.model == "" {
		return fallback
	}

	result, err := p.callLLM(ctx, query)
	if err != nil {
		p.logger.Warn().Err(err).Str("query", query).Msg("preprocess: LLM call failed, using fallback")
		return fallback
	}

	result.OriginalQuery = query
	return result
}

const systemPrompt = `You are a search query preprocessor for a code knowledge base. Given a user query (possibly in a non-English language), return a JSON object with:
- "language": ISO 639-1 code of the input language (e.g. "en", "vi", "ja")
- "english_query": the query translated to English (if already English, return as-is)
- "intent": one of "keyword" (exact match needed), "conceptual" (semantic/how-does-it-work), "temporal" (time-bounded question)
- "expansions": 0-5 related technical terms that would help retrieve relevant documents
- "time_filter": null, or {"after": "RFC3339", "before": "RFC3339"} if the query implies a time range

Respond with ONLY valid JSON, no markdown fences.`

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

type llmResponse struct {
	Language    string   `json:"language"`
	EnglishQuery string  `json:"english_query"`
	Intent      string   `json:"intent"`
	Expansions  []string `json:"expansions"`
	TimeFilter  *struct {
		After  string `json:"after,omitempty"`
		Before string `json:"before,omitempty"`
	} `json:"time_filter"`
}

func (p *Preprocessor) callLLM(ctx context.Context, query string) (*PreprocessResult, error) {
	reqBody := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: query},
		},
		Stream: false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("preprocess: marshal request: %w", err)
	}

	endpoint := p.providerURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("preprocess: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("preprocess: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("preprocess: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("preprocess: decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("preprocess: empty choices in response")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var llmResp llmResponse
	if err := json.Unmarshal([]byte(content), &llmResp); err != nil {
		return nil, fmt.Errorf("preprocess: parse JSON: %w", err)
	}

	intent := IntentKeyword
	switch llmResp.Intent {
	case "conceptual":
		intent = IntentConceptual
	case "temporal":
		intent = IntentTemporal
	case "keyword":
		intent = IntentKeyword
	}

	result := &PreprocessResult{
		EnglishQuery: llmResp.EnglishQuery,
		Intent:       intent,
		Expansions:   llmResp.Expansions,
		Language:     llmResp.Language,
	}

	if result.EnglishQuery == "" {
		result.EnglishQuery = query
	}
	if result.Language == "" {
		result.Language = "en"
	}

	if llmResp.TimeFilter != nil {
		tf := &TimeFilter{}
		if llmResp.TimeFilter.After != "" {
			t, err := time.Parse(time.RFC3339, llmResp.TimeFilter.After)
			if err == nil {
				tf.After = &t
			}
		}
		if llmResp.TimeFilter.Before != "" {
			t, err := time.Parse(time.RFC3339, llmResp.TimeFilter.Before)
			if err == nil {
				tf.Before = &t
			}
		}
		if tf.After != nil || tf.Before != nil {
			result.TimeFilter = tf
		}
	}

	return result, nil
}
