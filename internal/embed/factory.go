package embed

import (
	"fmt"
	"os"

	"github.com/nano-brain/nano-brain/internal/config"
)

func NewFromConfig(cfg config.EmbeddingConfig) (Embedder, error) {
	switch cfg.Provider {
	case "ollama":
		url := cfg.URL
		if url == "" {
			url = "http://localhost:11434"
		}
		model := cfg.Model
		if model == "" {
			model = "nomic-embed-text"
		}
		return NewOllamaEmbedder(url, model, cfg.Dimension), nil

	case "voyageai":
		apiKey := cfg.VoyageAPIKey
		if apiKey == "" {
			apiKey = os.Getenv("VOYAGE_API_KEY")
		}
		model := cfg.Model
		if model == "" {
			model = "voyage-3"
		}
		return NewVoyageAIEmbedder(apiKey, model, cfg.URL, cfg.Dimension)

	default:
		return nil, fmt.Errorf("unknown embedding provider: %q (supported: ollama, voyageai)", cfg.Provider)
	}
}
