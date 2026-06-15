package codesummarize

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

type LLMProvider struct {
	httpClient     *http.Client
	providerURL    string
	apiKey         string
	model          string
	maxOutputTokens int
	logger         zerolog.Logger
}

func NewLLMProvider(cfg config.CodeSummarizationConfig, logger zerolog.Logger) *LLMProvider {
	timeout := 600 * time.Second
	if cfg.RequestTimeout > 0 {
		timeout = time.Duration(cfg.RequestTimeout) * time.Second
	}
	return &LLMProvider{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		providerURL:     strings.TrimRight(cfg.ProviderURL, "/"),
		apiKey:          cfg.APIKey,
		model:           cfg.Model,
		maxOutputTokens: cfg.MaxOutputTokens,
		logger:          logger,
	}
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
	Stream         bool           `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (p *LLMProvider) SummarizeBatch(ctx context.Context, symbols []SymbolForSummary, graphContexts map[string]*SymbolGraphContext) ([]SymbolSummary, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	prompt := BuildBatchPromptWithContext(symbols, graphContexts)

	reqBody := chatRequest{
		Model: p.model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a code documentation assistant."},
			{Role: "user", Content: prompt},
		},
		ResponseFormat: responseFormat{Type: "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	endpoint := p.providerURL + "/chat/completions"

	p.logger.Info().
		Str("endpoint", endpoint).
		Str("model", p.model).
		Int("symbol_count", len(symbols)).
		Msg("codesummarize: sending LLM request")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in response")
	}

	content := chatResp.Choices[0].Message.Content

	summaries, err := parseJSONArray(content)
	if err != nil {
		return nil, fmt.Errorf("parse JSON array: %w", err)
	}

	matched := matchSummariesToSymbols(summaries, symbols)

	p.logger.Info().
		Int("requested", len(symbols)).
		Int("received", len(summaries)).
		Int("matched", len(matched)).
		Msg("codesummarize: batch completed")

	return matched, nil
}

type summaryResponse struct {
	Summaries []SymbolSummary `json:"summaries"`
}

func parseJSONArray(content string) ([]SymbolSummary, error) {
	content = strings.TrimSpace(content)

	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var wrapper summaryResponse
	if err := json.Unmarshal([]byte(content), &wrapper); err != nil {
		return nil, err
	}

	return wrapper.Summaries, nil
}

func matchSummariesToSymbols(summaries []SymbolSummary, symbols []SymbolForSummary) []SymbolSummary {
	symbolMap := make(map[string]SymbolForSummary)
	for _, sym := range symbols {
		key := sym.Name + "|" + sym.File
		symbolMap[key] = sym
	}

	var matched []SymbolSummary
	for _, summary := range summaries {
		key := summary.Name + "|" + summary.File
		if _, exists := symbolMap[key]; exists {
			matched = append(matched, summary)
		}
	}

	return matched
}
