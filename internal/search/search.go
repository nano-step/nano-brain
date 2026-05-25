// Package search handles semantic and keyword search queries.
package search

import "time"

// Result represents a single search result from any search method.
type Result struct {
	ID            string
	DocumentID    string
	WorkspaceHash string
	Title         string
	Snippet       string
	Content       string
	Score         float64
	Tags          []string
	Collection    string
	SourcePath    string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
