// Package harvest manages document ingestion and harvesting.
package harvest

import (
	"context"
	"time"
)

// SummaryMeta carries session metadata from a harvester to the summarizer.
type SummaryMeta struct {
	Source        string
	SessionID     string
	Title         string
	Agent         string
	ProjectPath   string
	CreatedAt     time.Time
	Duration      time.Duration
	ParentID      string
	Branch        string
	Cwd           string
	Tags          []string // extra tags to merge (e.g. ticket:DEV-1234)
	WorkspaceHash string
}

// SessionSummarizer is called after a successful harvest to generate and persist session summaries.
type SessionSummarizer interface {
	SummarizeAndPersist(ctx context.Context, content string, meta SummaryMeta) error
}
