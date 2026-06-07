package hyde

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewGenerator(t *testing.T) {
	cfg := config.HyDEConfig{
		Enabled:      true,
		ProviderURL:  "http://example.com",
		APIKey:       "test-key",
		Model:        "test-model",
		MaxLatencyMs: 1000,
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	assert.NotNil(t, generator)
	assert.Equal(t, "http://example.com", generator.providerURL)
	assert.Equal(t, "test-model", generator.model)
	assert.Equal(t, "test-key", generator.apiKey)
}

func TestGenerate_EmptyProviderURL(t *testing.T) {
	cfg := config.HyDEConfig{
		Enabled: true,
		Model:   "test-model",
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	text, err := generator.Generate(context.Background(), "test query")
	assert.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestGenerate_EmptyModel(t *testing.T) {
	cfg := config.HyDEConfig{
		Enabled:     true,
		ProviderURL: "http://example.com",
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	text, err := generator.Generate(context.Background(), "test query")
	assert.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestGenerate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		response := `{
			"choices": [{
				"message": {
					"content": "This is a hypothetical document about rate limiting in Go. Rate limiting is implemented using token bucket algorithms and middleware patterns."
				}
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := config.HyDEConfig{
		Enabled:      true,
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		MaxLatencyMs: 5000,
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	text, err := generator.Generate(context.Background(), "How to handle rate limiting in Go")
	assert.NoError(t, err)
	assert.Contains(t, text, "hypothetical document")
	assert.Contains(t, text, "rate limiting")
	assert.Contains(t, text, "token bucket")
}

func TestGenerate_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	cfg := config.HyDEConfig{
		Enabled:      true,
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		MaxLatencyMs: 5000,
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	text, err := generator.Generate(context.Background(), "test query")
	assert.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestGenerate_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		response := `{"choices": [{"message": {"content": "response"}}]}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := config.HyDEConfig{
		Enabled:      true,
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		MaxLatencyMs: 50,
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	text, err := generator.Generate(context.Background(), "test query")
	assert.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestGenerate_EmptyChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{"choices": []}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := config.HyDEConfig{
		Enabled:      true,
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		MaxLatencyMs: 5000,
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	text, err := generator.Generate(context.Background(), "test query")
	assert.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestGenerate_JSONParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	cfg := config.HyDEConfig{
		Enabled:      true,
		ProviderURL:  server.URL,
		APIKey:       "test-key",
		Model:        "test-model",
		MaxLatencyMs: 5000,
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	text, err := generator.Generate(context.Background(), "test query")
	assert.NoError(t, err)
	assert.Equal(t, "", text)
}

func TestGenerate_DefaultTimeout(t *testing.T) {
	cfg := config.HyDEConfig{
		Enabled:     true,
		ProviderURL: "http://example.com",
		APIKey:      "test-key",
		Model:       "test-model",
	}
	logger := zerolog.Nop()

	generator := NewGenerator(cfg, logger)

	assert.Equal(t, 500*time.Millisecond, generator.httpClient.Timeout)
}