// Package codesummarize provides batch code symbol summarization using LLM.
package codesummarize

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var ErrBudgetExhausted = errors.New("daily code summarization budget exhausted")

type CodeSummarizer interface {
	RunOnce(ctx context.Context, workspaceHash string) (processed, skipped, errors int, err error)
}

type SymbolForSummary struct {
	ChunkID     uuid.UUID
	Name        string
	Kind        string
	File        string
	Language    string
	Code        string
	ContentHash string
}

type SymbolSummary struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Summary string `json:"summary"`
}
