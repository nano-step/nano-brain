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

const (
	defaultVoyageAIDimension = 1024
	defaultVoyageAIURL       = "https://api.voyageai.com/v1/embeddings"
)

type VoyageAIEmbedder struct {
	url        string
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
}

func NewVoyageAIEmbedder(apiKey, model, url string, dimension int) (*VoyageAIEmbedder, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("voyageai: api key is required")
	}
	if url == "" {
		url = defaultVoyageAIURL
	}
	if dimension <= 0 {
		dimension = defaultVoyageAIDimension
	}
	return &VoyageAIEmbedder{
		url:       url,
		apiKey:    apiKey,
		model:     model,
		dimension: dimension,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}, nil
}

func (v *VoyageAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}{
		Model: v.model,
		Input: []string{text},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("voyageai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("voyageai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyageai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("voyageai: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("voyageai: decode response: %w", err)
	}

	if len(result.Data) == 0 || len(result.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("voyageai: empty embedding returned")
	}

	return result.Data[0].Embedding, nil
}

func (v *VoyageAIEmbedder) Dimension() int {
	return v.dimension
}
