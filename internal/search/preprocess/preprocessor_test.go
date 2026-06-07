package preprocess_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/search/preprocess"
	"github.com/rs/zerolog"
)

func newTestLogger() zerolog.Logger {
	return zerolog.Nop()
}

func newMockServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func chatResponseJSON(content string) []byte {
	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"content": content}},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestProcess_Success(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		llmOutput  string
		wantIntent preprocess.Intent
		wantQuery  string
		wantLang   string
		wantExpLen int
	}{
		{
			name:  "Vietnamese query translated",
			query: "khi nào symbol indexing chạy?",
			llmOutput: `{"language":"vi","english_query":"when does symbol indexing trigger?","intent":"conceptual","expansions":["fsnotify","watcher","tree-sitter"],"time_filter":null}`,
			wantIntent: preprocess.IntentConceptual,
			wantQuery:  "when does symbol indexing trigger?",
			wantLang:   "vi",
			wantExpLen: 3,
		},
		{
			name:  "English keyword query",
			query: "ECONNREFUSED redis",
			llmOutput: `{"language":"en","english_query":"ECONNREFUSED redis","intent":"keyword","expansions":["connection refused","redis client"],"time_filter":null}`,
			wantIntent: preprocess.IntentKeyword,
			wantQuery:  "ECONNREFUSED redis",
			wantLang:   "en",
			wantExpLen: 2,
		},
		{
			name:  "Temporal query with time filter",
			query: "what bugs were fixed last week",
			llmOutput: `{"language":"en","english_query":"what bugs were fixed last week","intent":"temporal","expansions":["bug fix","patch"],"time_filter":{"after":"2026-05-31T00:00:00Z"}}`,
			wantIntent: preprocess.IntentTemporal,
			wantQuery:  "what bugs were fixed last week",
			wantLang:   "en",
			wantExpLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(chatResponseJSON(tt.llmOutput))
			})
			defer srv.Close()

			cfg := config.QueryPreprocessingConfig{
				Enabled:      true,
				ProviderURL:  srv.URL,
				APIKey:       "test-key",
				Model:        "test-model",
				MaxLatencyMs: 5000,
			}

			p := preprocess.NewPreprocessor(cfg, newTestLogger())
			result := p.Process(context.Background(), tt.query)

			if result.OriginalQuery != tt.query {
				t.Errorf("OriginalQuery = %q, want %q", result.OriginalQuery, tt.query)
			}
			if result.EnglishQuery != tt.wantQuery {
				t.Errorf("EnglishQuery = %q, want %q", result.EnglishQuery, tt.wantQuery)
			}
			if result.Intent != tt.wantIntent {
				t.Errorf("Intent = %q, want %q", result.Intent, tt.wantIntent)
			}
			if result.Language != tt.wantLang {
				t.Errorf("Language = %q, want %q", result.Language, tt.wantLang)
			}
			if len(result.Expansions) != tt.wantExpLen {
				t.Errorf("len(Expansions) = %d, want %d", len(result.Expansions), tt.wantExpLen)
			}
		})
	}
}

func TestProcess_TimeFilterParsed(t *testing.T) {
	llmOutput := `{"language":"en","english_query":"bugs fixed last week","intent":"temporal","expansions":[],"time_filter":{"after":"2026-05-31T00:00:00Z","before":"2026-06-07T00:00:00Z"}}`

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatResponseJSON(llmOutput))
	})
	defer srv.Close()

	cfg := config.QueryPreprocessingConfig{
		Enabled:      true,
		ProviderURL:  srv.URL,
		Model:        "test-model",
		MaxLatencyMs: 5000,
	}

	p := preprocess.NewPreprocessor(cfg, newTestLogger())
	result := p.Process(context.Background(), "bugs fixed last week")

	if result.TimeFilter == nil {
		t.Fatal("TimeFilter is nil, want non-nil")
	}
	if result.TimeFilter.After == nil {
		t.Fatal("TimeFilter.After is nil")
	}
	if result.TimeFilter.Before == nil {
		t.Fatal("TimeFilter.Before is nil")
	}

	wantAfter := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	if !result.TimeFilter.After.Equal(wantAfter) {
		t.Errorf("TimeFilter.After = %v, want %v", result.TimeFilter.After, wantAfter)
	}
}

func TestProcess_FallbackOnHTTPError(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal error"))
	})
	defer srv.Close()

	cfg := config.QueryPreprocessingConfig{
		Enabled:      true,
		ProviderURL:  srv.URL,
		Model:        "test-model",
		MaxLatencyMs: 5000,
	}

	p := preprocess.NewPreprocessor(cfg, newTestLogger())
	result := p.Process(context.Background(), "test query")

	if result.OriginalQuery != "test query" {
		t.Errorf("OriginalQuery = %q, want %q", result.OriginalQuery, "test query")
	}
	if result.EnglishQuery != "test query" {
		t.Errorf("EnglishQuery = %q, want %q", result.EnglishQuery, "test query")
	}
	if result.Intent != preprocess.IntentKeyword {
		t.Errorf("Intent = %q, want %q", result.Intent, preprocess.IntentKeyword)
	}
}

func TestProcess_FallbackOnInvalidJSON(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatResponseJSON("not valid json {{{"))
	})
	defer srv.Close()

	cfg := config.QueryPreprocessingConfig{
		Enabled:      true,
		ProviderURL:  srv.URL,
		Model:        "test-model",
		MaxLatencyMs: 5000,
	}

	p := preprocess.NewPreprocessor(cfg, newTestLogger())
	result := p.Process(context.Background(), "test query")

	if result.Intent != preprocess.IntentKeyword {
		t.Errorf("Intent = %q, want %q", result.Intent, preprocess.IntentKeyword)
	}
}

func TestProcess_FallbackOnTimeout(t *testing.T) {
	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatResponseJSON(`{"language":"en","english_query":"q","intent":"keyword","expansions":[],"time_filter":null}`))
	})
	defer srv.Close()

	cfg := config.QueryPreprocessingConfig{
		Enabled:      true,
		ProviderURL:  srv.URL,
		Model:        "test-model",
		MaxLatencyMs: 50,
	}

	p := preprocess.NewPreprocessor(cfg, newTestLogger())
	result := p.Process(context.Background(), "test query")

	if result.Intent != preprocess.IntentKeyword {
		t.Errorf("Intent = %q, want fallback %q", result.Intent, preprocess.IntentKeyword)
	}
	if result.EnglishQuery != "test query" {
		t.Errorf("EnglishQuery = %q, want original query", result.EnglishQuery)
	}
}

func TestProcess_FallbackOnEmptyConfig(t *testing.T) {
	cfg := config.QueryPreprocessingConfig{
		Enabled:      true,
		ProviderURL:  "",
		Model:        "",
		MaxLatencyMs: 500,
	}

	p := preprocess.NewPreprocessor(cfg, newTestLogger())
	result := p.Process(context.Background(), "hello")

	if result.EnglishQuery != "hello" {
		t.Errorf("EnglishQuery = %q, want %q", result.EnglishQuery, "hello")
	}
	if result.Intent != preprocess.IntentKeyword {
		t.Errorf("Intent = %q, want %q", result.Intent, preprocess.IntentKeyword)
	}
}

func TestProcess_RequestFormat(t *testing.T) {
	var receivedReq map[string]interface{}

	srv := newMockServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Authorization header = %q, want Bearer test-api-key", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}

		body, _ := json.Marshal(map[string]interface{}{})
		_ = json.NewDecoder(r.Body).Decode(&receivedReq)
		_ = body

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(chatResponseJSON(`{"language":"en","english_query":"test","intent":"keyword","expansions":[],"time_filter":null}`))
	})
	defer srv.Close()

	cfg := config.QueryPreprocessingConfig{
		Enabled:      true,
		ProviderURL:  srv.URL,
		APIKey:       "test-api-key",
		Model:        "gpt-4o-mini",
		MaxLatencyMs: 5000,
	}

	p := preprocess.NewPreprocessor(cfg, newTestLogger())
	p.Process(context.Background(), "test")

	if receivedReq["model"] != "gpt-4o-mini" {
		t.Errorf("model = %v, want gpt-4o-mini", receivedReq["model"])
	}
	if receivedReq["stream"] != false {
		t.Errorf("stream = %v, want false", receivedReq["stream"])
	}
}
