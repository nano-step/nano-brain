package reranking_test

import (
	"context"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/search/reranking"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopReranker(t *testing.T) {
	cfg := config.RerankingConfig{Enabled: false}
	logger := zerolog.Nop()
	reranker := reranking.NewReranker(cfg, logger)

	docs := []search.Result{
		{ID: "1", Snippet: "doc1", Score: 0.5},
		{ID: "2", Snippet: "doc2", Score: 0.3},
	}

	result, err := reranker.Rerank(context.Background(), "test query", docs, 10)
	require.NoError(t, err)
	assert.Equal(t, docs, result)
}

func TestUnknownProvider(t *testing.T) {
	cfg := config.RerankingConfig{
		Enabled:  true,
		Provider: "unknown",
		APIKey:   "test-key",
		TopK:     20,
	}
	logger := zerolog.Nop()
	reranker := reranking.NewReranker(cfg, logger)

	docs := []search.Result{
		{ID: "1", Snippet: "doc1", Score: 0.5},
		{ID: "2", Snippet: "doc2", Score: 0.3},
	}

	result, err := reranker.Rerank(context.Background(), "test query", docs, 10)
	require.NoError(t, err)
	assert.Equal(t, docs, result)
}

func TestCohereNoAPIKey(t *testing.T) {
	cfg := config.RerankingConfig{
		Enabled:  true,
		Provider: "cohere",
		APIKey:   "",
		TopK:     20,
	}
	logger := zerolog.Nop()
	reranker := reranking.NewReranker(cfg, logger)

	docs := []search.Result{
		{ID: "1", Snippet: "doc1", Score: 0.5},
		{ID: "2", Snippet: "doc2", Score: 0.3},
	}

	result, err := reranker.Rerank(context.Background(), "test query", docs, 10)
	require.NoError(t, err)
	assert.Equal(t, docs, result)
}

func TestEmptyDocs(t *testing.T) {
	cfg := config.RerankingConfig{
		Enabled:  true,
		Provider: "cohere",
		APIKey:   "test-key",
		TopK:     20,
	}
	logger := zerolog.Nop()
	reranker := reranking.NewReranker(cfg, logger)

	result, err := reranker.Rerank(context.Background(), "test query", nil, 10)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestJinaNoAPIKey(t *testing.T) {
	cfg := config.RerankingConfig{
		Enabled:  true,
		Provider: "jina",
		APIKey:   "",
		TopK:     20,
	}
	logger := zerolog.Nop()
	reranker := reranking.NewReranker(cfg, logger)

	docs := []search.Result{
		{ID: "1", Snippet: "doc1", Score: 0.5},
	}

	result, err := reranker.Rerank(context.Background(), "test query", docs, 10)
	require.NoError(t, err)
	assert.Equal(t, docs, result)
}

func TestJinaCustomEndpoint(t *testing.T) {
	cfg := config.RerankingConfig{
		Enabled:    true,
		Provider:   "jina",
		ProviderURL: "https://custom.example.com/rerank",
		APIKey:      "test-key",
		Model:       "jina-reranker-v1-turbo",
		TopK:        10,
		MinScore:    0.5,
		MaxLatencyMs: 3000,
	}
	logger := zerolog.Nop()
	reranker := reranking.NewReranker(cfg, logger)

	docs := []search.Result{
		{ID: "1", Snippet: "doc1", Score: 0.5},
	}

	result, err := reranker.Rerank(context.Background(), "test query", docs, 10)
	require.NoError(t, err)
	assert.Equal(t, docs, result)
}
