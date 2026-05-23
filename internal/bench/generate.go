package bench

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DocumentRow struct {
	ID            uuid.UUID
	WorkspaceHash string
	ContentHash   string
	Title         string
	SourcePath    string
	Collection    string
}

type DataStore interface {
	CountDocumentsByWorkspace(ctx context.Context, workspaceHash string) (int64, error)
	ListDocumentsByWorkspace(ctx context.Context, workspaceHash string) ([]DocumentRow, error)
}

func Generate(ctx context.Context, store DataStore, workspaceHash string, scale int) (*BenchmarkDataset, error) {
	if scale < 1 {
		return nil, fmt.Errorf("scale must be a positive integer, got %d", scale)
	}

	count, err := store.CountDocumentsByWorkspace(ctx, workspaceHash)
	if err != nil {
		return nil, fmt.Errorf("counting documents: %w", err)
	}
	if count < int64(scale) {
		return nil, fmt.Errorf("workspace %q has %d documents, need at least %d", workspaceHash, count, scale)
	}

	docs, err := store.ListDocumentsByWorkspace(ctx, workspaceHash)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}

	shuffled := make([]DocumentRow, len(docs))
	copy(shuffled, docs)
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	sampled := shuffled[:scale]

	entries := make([]DatasetEntry, 0, scale)
	for _, doc := range sampled {
		query := deriveQuery(doc)
		if query == "" {
			continue
		}
		entries = append(entries, DatasetEntry{
			Query:          query,
			RelevantDocIDs: []string{doc.ID.String()},
			SourceDocID:    doc.ID.String(),
			SourceTitle:    doc.Title,
		})
	}

	return &BenchmarkDataset{
		Scale:         scale,
		WorkspaceHash: workspaceHash,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Entries:       entries,
	}, nil
}

func deriveQuery(doc DocumentRow) string {
	if doc.Title != "" {
		return strings.TrimSpace(doc.Title)
	}
	if doc.SourcePath != "" {
		return strings.TrimSpace(doc.SourcePath)
	}
	return ""
}
