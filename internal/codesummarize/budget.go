package codesummarize

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type BudgetTracker struct {
	queries *sqlc.Queries
}

func NewBudgetTracker(queries *sqlc.Queries) *BudgetTracker {
	return &BudgetTracker{
		queries: queries,
	}
}

func (b *BudgetTracker) Increment(ctx context.Context, workspaceHash string) error {
	if err := b.queries.IncrementCodeSummarizationUsage(ctx, workspaceHash); err != nil {
		return fmt.Errorf("increment usage: %w", err)
	}
	return nil
}

func (b *BudgetTracker) IsExhausted(ctx context.Context, workspaceHash string, maxPerDay int) (bool, error) {
	if maxPerDay <= 0 {
		return false, nil
	}

	count, err := b.queries.GetCodeSummarizationUsage(ctx, workspaceHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("get usage: %w", err)
	}

	return count >= int32(maxPerDay), nil
}
