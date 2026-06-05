// Package intelligence provides memory consolidation and categorization features.
//
// Consolidation finds semantically similar documents and merges them via LLM to reduce
// redundancy. Categorization uses an LLM to automatically tag documents that lack semantic
// tags.
//
// Both features reuse the existing OpenAI-compatible LLM client from internal/summarize.
package intelligence

import "context"

// LLM is the interface for LLM calls. Matches the interface in internal/summarize/pipeline.go.
// Reuses the existing summarize.Client implementation.
type LLM interface {
	ChatCompletion(ctx context.Context, systemPrompt, userPrompt string) (string, TokenUsage, error)
}

// TokenUsage holds token counts from an LLM completion response.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
