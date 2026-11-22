package main

import (
	"strings"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
)

func TestEmbedInitLogFields_RedactsURL(t *testing.T) {
	ec := config.EmbeddingConfig{Provider: "ollama", URL: "https://u:EMBEDSECRET@ollama.example/v1?api_key=QUERYSECRET"}
	provider, url := embedInitLogFields(ec)
	if provider != "ollama" {
		t.Errorf("provider = %q, want %q", provider, "ollama")
	}
	for _, secret := range []string{"EMBEDSECRET", "QUERYSECRET"} {
		if strings.Contains(url, secret) {
			t.Errorf("logged url still contains %s: %q", secret, url)
		}
	}
}
