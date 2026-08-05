package codesummarize

import (
	"context"
	"testing"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type unresolvedGraphContextStore struct{}

func (unresolvedGraphContextStore) BulkGetCallerContext(context.Context, sqlc.BulkGetCallerContextParams) ([]sqlc.BulkGetCallerContextRow, error) {
	return nil, nil
}

func (unresolvedGraphContextStore) BulkGetCalleeNodes(context.Context, sqlc.BulkGetCalleeNodesParams) ([]sqlc.BulkGetCalleeNodesRow, error) {
	return []sqlc.BulkGetCalleeNodesRow{{
		SourceNode: "repo-a/consumer.ts::caller",
		TargetNode: "<unresolved>",
	}}, nil
}

func TestFetchGraphContextExcludesUnresolvedCalleeFromCacheInput(t *testing.T) {
	// Given: the storage layer returns a diagnostically persisted unresolved edge.
	store := unresolvedGraphContextStore{}

	// When
	contexts, err := FetchGraphContext(context.Background(), store, "workspace", []string{"repo-a/consumer.ts::caller"})
	if err != nil {
		t.Fatalf("FetchGraphContext: %v", err)
	}

	// Then: cache/hash input must not retain the sentinel.
	if got := contexts["repo-a/consumer.ts::caller"].Callees; len(got) != 0 {
		t.Fatalf("unresolved callee entered graph-context cache input: %q", got)
	}
}
