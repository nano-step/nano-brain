package embed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
)

func TestNewFromConfig_Ollama(t *testing.T) {
	cfg := config.EmbeddingConfig{Provider: "ollama", URL: "http://myhost:11434", Model: "test-model"}
	e, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oe, ok := e.(*OllamaEmbedder)
	if !ok {
		t.Fatalf("expected *OllamaEmbedder, got %T", e)
	}
	if oe.url != "http://myhost:11434" {
		t.Errorf("url = %q, want %q", oe.url, "http://myhost:11434")
	}
	if oe.model != "test-model" {
		t.Errorf("model = %q, want %q", oe.model, "test-model")
	}
}

func TestNewFromConfig_Ollama_Defaults(t *testing.T) {
	cfg := config.EmbeddingConfig{Provider: "ollama"}
	e, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oe := e.(*OllamaEmbedder)
	if oe.url != "http://localhost:11434" {
		t.Errorf("url = %q, want default", oe.url)
	}
	if oe.model != "nomic-embed-text" {
		t.Errorf("model = %q, want default", oe.model)
	}
}

func TestNewFromConfig_VoyageAI(t *testing.T) {
	cfg := config.EmbeddingConfig{Provider: "voyageai", VoyageAPIKey: "test-key", Model: "voyage-3"}
	e, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ve, ok := e.(*VoyageAIEmbedder)
	if !ok {
		t.Fatalf("expected *VoyageAIEmbedder, got %T", e)
	}
	if ve.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", ve.apiKey, "test-key")
	}
}

func TestNewFromConfig_VoyageAI_MissingKey(t *testing.T) {
	t.Setenv("VOYAGE_API_KEY", "")
	cfg := config.EmbeddingConfig{Provider: "voyageai"}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestNewFromConfig_UnknownProvider(t *testing.T) {
	cfg := config.EmbeddingConfig{Provider: "unknown"}
	_, err := NewFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestOllamaEmbedder_Embed(t *testing.T) {
	want := []float32{0.1, 0.2, 0.3}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %s, want /api/embed", r.URL.Path)
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "test-model" {
			t.Errorf("model = %q, want %q", req.Model, "test-model")
		}
		if len(req.Input) != 1 || req.Input[0] != "hello world" {
			t.Errorf("input = %v, want [hello world]", req.Input)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][][]float32{"embeddings": {want}})
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model", 0)
	got, err := e.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %f, want %f", i, got[i], want[i])
		}
	}
}

func TestOllamaEmbedder_Dimension(t *testing.T) {
	e := NewOllamaEmbedder("http://localhost:11434", "test", 0)
	if d := e.Dimension(); d != 768 {
		t.Errorf("Dimension() = %d, want 768", d)
	}
}

func TestOllamaEmbedder_CustomDimension(t *testing.T) {
	e := NewOllamaEmbedder("http://localhost:11434", "test", 384)
	if d := e.Dimension(); d != 384 {
		t.Errorf("Dimension() = %d, want 384", d)
	}
}

func TestOllamaEmbedder_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	e := NewOllamaEmbedder(srv.URL, "test-model", 0)
	_, err := e.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestVoyageAIEmbedder_Embed(t *testing.T) {
	want := []float32{0.4, 0.5, 0.6}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want %q", auth, "Bearer test-key")
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "voyage-3" {
			t.Errorf("model = %q, want %q", req.Model, "voyage-3")
		}
		if len(req.Input) != 1 || req.Input[0] != "hello world" {
			t.Errorf("input = %v, want [hello world]", req.Input)
		}
		resp := map[string]interface{}{
			"data": []map[string]interface{}{
				{"embedding": want},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	ve, err := NewVoyageAIEmbedder("test-key", "voyage-3", srv.URL, 0)
	if err != nil {
		t.Fatalf("unexpected error creating embedder: %v", err)
	}
	got, err := ve.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %f, want %f", i, got[i], want[i])
		}
	}
}

func TestVoyageAIEmbedder_Dimension(t *testing.T) {
	e, _ := NewVoyageAIEmbedder("key", "model", "", 0)
	if d := e.Dimension(); d != 1024 {
		t.Errorf("Dimension() = %d, want 1024", d)
	}
}

func TestVoyageAIEmbedder_CustomDimension(t *testing.T) {
	e, _ := NewVoyageAIEmbedder("key", "model", "", 512)
	if d := e.Dimension(); d != 512 {
		t.Errorf("Dimension() = %d, want 512", d)
	}
}

func TestVoyageAIEmbedder_DefaultURL(t *testing.T) {
	e, _ := NewVoyageAIEmbedder("key", "model", "", 0)
	if e.url != defaultVoyageAIURL {
		t.Errorf("url = %q, want %q", e.url, defaultVoyageAIURL)
	}
}

func TestVoyageAIEmbedder_MissingKey(t *testing.T) {
	_, err := NewVoyageAIEmbedder("", "voyage-3", "", 0)
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
}

func TestVoyageAIEmbedder_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	ve, err := NewVoyageAIEmbedder("bad-key", "voyage-3", srv.URL, 0)
	if err != nil {
		t.Fatalf("unexpected error creating embedder: %v", err)
	}
	_, err = ve.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}
