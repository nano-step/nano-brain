// Package search handles semantic and keyword search queries.
package search

import (
	"context"
	"time"
)

// Result represents a single search result from any search method.
// JSON tags use snake_case to match the public API + MCP tool response contract.
// Without these tags Go marshals fields as PascalCase (ID, Title, Score) which
// breaks MCP clients that expect snake_case (id, title, score). See issue #303.
type Result struct {
	ID            string    `json:"id"`
	DocumentID    string    `json:"document_id"`
	WorkspaceHash string    `json:"workspace_hash"`
	Title         string    `json:"title"`
	Snippet       string    `json:"snippet"`
	Content       string    `json:"content"`
	Score         float64   `json:"score"`
	Tags          []string  `json:"tags"`
	Collection    string    `json:"collection"`
	SourcePath    string    `json:"source_path"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	// Source labels which DebugSearch leg produced this result: "code",
	// "session", or "config". Empty for all non-debug search paths (#543).
	Source string `json:"source,omitempty"`
}

// Reranker is the interface for search result reranking.
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []Result, topK int) ([]Result, error)
}
