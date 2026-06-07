package search

import (
	"context"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type PageRankQuerier interface {
	GetPageRankScores(ctx context.Context, workspaceHash string) ([]sqlc.GetPageRankScoresRow, error)
}

type SQLPageRankLoader struct {
	queries PageRankQuerier
}

func NewSQLPageRankLoader(q PageRankQuerier) *SQLPageRankLoader {
	return &SQLPageRankLoader{queries: q}
}

func (l *SQLPageRankLoader) LoadScores(ctx context.Context, workspace string) (map[string]float64, error) {
	rows, err := l.queries.GetPageRankScores(ctx, workspace)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	scores := make(map[string]float64, len(rows))
	for _, r := range rows {
		scores[r.NodeName] = r.Score
	}
	return scores, nil
}
