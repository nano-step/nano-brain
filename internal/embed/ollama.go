package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultOllamaDimension = 768

type OllamaEmbedder struct {
	url        string
	model      string
	dimension  int
	httpClient *http.Client
}

func NewOllamaEmbedder(url, model string, dimension int) *OllamaEmbedder {
	if dimension <= 0 {
		dimension = defaultOllamaDimension
	}
	return &OllamaEmbedder{
		url:       url,
		model:     model,
		dimension: dimension,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (o *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{
		Model: o.model,
		Input: []string{text},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.url+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("ollama: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	if len(result.Embeddings) == 0 || len(result.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama: empty embedding returned")
	}

	return result.Embeddings[0], nil
}

func (o *OllamaEmbedder) Dimension() int {
	return o.dimension
}
