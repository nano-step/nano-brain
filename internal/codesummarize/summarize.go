// Package codesummarize provides batch code symbol summarization using LLM.
package codesummarize

import (
	"context"
	"errors"
)

// ErrBudgetExhausted is returned when the daily code summarization budget is exhausted.
var ErrBudgetExhausted = errors.New("daily code summarization budget exhausted")

// CodeSummarizer orchestrates batch summarization of code symbols.
type CodeSummarizer interface {
	// RunOnce processes up to maxSummariesPerCycle unsummarized symbols.
	// Returns count of processed, skipped, and errors.
	RunOnce(ctx context.Context, workspaceHash string) (processed, skipped, errors int, err error)
}

// SymbolForSummary represents a code symbol to be summarized.
type SymbolForSummary struct {
	Name        string
	Kind        string
	File        string
	Language    string
	Code        string
	ContentHash string
}

// SymbolSummary represents the LLM's summary output for one symbol.
type SymbolSummary struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Summary string `json:"summary"`
}
