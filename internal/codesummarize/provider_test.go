package codesummarize

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

func TestLLMProvider_SummarizeBatch(t *testing.T) {
	tests := []struct {
		name           string
		symbols        []SymbolForSummary
		mockResponse   chatResponse
		mockStatusCode int
		expectError    bool
		expectedCount  int
	}{
		{
			name: "successful batch with matching results",
			symbols: []SymbolForSummary{
				{Name: "ProcessData", File: "internal/processor.go", Code: "func ProcessData() {}"},
				{Name: "UserService", File: "internal/user.go", Code: "type UserService struct {}"},
			},
			mockResponse: chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{
						Message: struct {
							Content string `json:"content"`
						}{
							Content: `{
								"summaries": [
									{"name": "ProcessData", "file": "internal/processor.go", "summary": "Processes input data"},
									{"name": "UserService", "file": "internal/user.go", "summary": "Manages user operations"}
								]
							}`,
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  2,
		},
		{
			name: "partial match - only some results match input",
			symbols: []SymbolForSummary{
				{Name: "ProcessData", File: "internal/processor.go", Code: "func ProcessData() {}"},
				{Name: "UserService", File: "internal/user.go", Code: "type UserService struct {}"},
			},
			mockResponse: chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{
						Message: struct {
							Content string `json:"content"`
						}{
							Content: `{
								"summaries": [
									{"name": "ProcessData", "file": "internal/processor.go", "summary": "Processes input data"},
									{"name": "UnknownFunc", "file": "unknown.go", "summary": "Unknown function"}
								]
							}`,
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  1,
		},
		{
			name: "response with markdown code fence",
			symbols: []SymbolForSummary{
				{Name: "TestFunc", File: "test.go", Code: "func TestFunc() {}"},
			},
			mockResponse: chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{
					{
						Message: struct {
							Content string `json:"content"`
						}{
							Content: "```json\n" + `{"summaries": [{"name": "TestFunc", "file": "test.go", "summary": "Test function"}]}` + "\n```",
						},
					},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  1,
		},
		{
			name:           "empty symbols",
			symbols:        []SymbolForSummary{},
			mockStatusCode: http.StatusOK,
			expectError:    false,
			expectedCount:  0,
		},
		{
			name: "HTTP error",
			symbols: []SymbolForSummary{
				{Name: "TestFunc", File: "test.go", Code: "func TestFunc() {}"},
			},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name: "empty choices in response",
			symbols: []SymbolForSummary{
				{Name: "TestFunc", File: "test.go", Code: "func TestFunc() {}"},
			},
			mockResponse: chatResponse{
				Choices: []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				}{},
			},
			mockStatusCode: http.StatusOK,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/chat/completions" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}

				if r.Method != http.MethodPost {
					t.Errorf("unexpected method: %s", r.Method)
				}

				if tt.mockStatusCode != http.StatusOK {
					w.WriteHeader(tt.mockStatusCode)
					_, _ = w.Write([]byte("error"))
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(tt.mockResponse)
			}))
			defer server.Close()

			cfg := config.CodeSummarizationConfig{
				ProviderURL: server.URL,
				APIKey:      "test-key",
				Model:       "test-model",
				MaxOutputTokens: 1000,
			}

			logger := zerolog.Nop()
			provider := NewLLMProvider(cfg, logger)

			ctx := context.Background()
			summaries, err := provider.SummarizeBatch(ctx, tt.symbols, nil)

			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(summaries) != tt.expectedCount {
				t.Errorf("expected %d summaries, got %d", tt.expectedCount, len(summaries))
			}

			for _, summary := range summaries {
				if summary.Name == "" {
					t.Error("summary missing name")
				}
				if summary.File == "" {
					t.Error("summary missing file")
				}
				if summary.Summary == "" {
					t.Error("summary missing summary text")
				}
			}
		})
	}
}
