package codesummarize

import (
	"strings"
	"testing"
)

func TestBuildBatchPrompt(t *testing.T) {
	tests := []struct {
		name    string
		symbols []SymbolForSummary
		checks  []string
	}{
		{
			name: "single symbol",
			symbols: []SymbolForSummary{
				{
					Name:     "ProcessData",
					Kind:     "function",
					File:     "internal/processor.go",
					Language: "Go",
					Code:     "func ProcessData(input string) error {\n\treturn nil\n}",
				},
			},
			checks: []string{
				"### 1. ProcessData (in internal/processor.go)",
				"Kind: function",
				"Language: Go",
				"func ProcessData(input string) error",
				"JSON object",
				`"summaries"`,
				`"name"`,
				`"file"`,
				`"summary"`,
			},
		},
		{
			name: "three symbols",
			symbols: []SymbolForSummary{
				{
					Name:     "UserService",
					Kind:     "type",
					File:     "internal/user/service.go",
					Language: "Go",
					Code:     "type UserService struct {\n\tdb *DB\n}",
				},
				{
					Name:     "GetUser",
					Kind:     "method",
					File:     "internal/user/service.go",
					Language: "Go",
					Code:     "func (s *UserService) GetUser(id int) (*User, error) {\n\treturn s.db.FindUser(id)\n}",
				},
				{
					Name:     "MaxRetries",
					Kind:     "const",
					File:     "internal/config/constants.go",
					Language: "Go",
					Code:     "const MaxRetries = 3",
				},
			},
			checks: []string{
				"### 1. UserService (in internal/user/service.go)",
				"### 2. GetUser (in internal/user/service.go)",
				"### 3. MaxRetries (in internal/config/constants.go)",
				"Kind: type",
				"Kind: method",
				"Kind: const",
				"type UserService struct",
				"func (s *UserService) GetUser",
				"const MaxRetries = 3",
			},
		},
		{
			name:    "empty symbols",
			symbols: []SymbolForSummary{},
			checks: []string{
				"JSON object",
				`"summaries"`,
				"Symbols to summarize:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := BuildBatchPrompt(tt.symbols)

			if prompt == "" {
				t.Fatal("expected non-empty prompt")
			}

			for _, check := range tt.checks {
				if !strings.Contains(prompt, check) {
					t.Errorf("prompt missing expected content: %q", check)
				}
			}

			if len(tt.symbols) > 0 {
				if !strings.Contains(prompt, "```") {
					t.Error("expected code blocks with ``` delimiters")
				}
			}
		})
	}
}
