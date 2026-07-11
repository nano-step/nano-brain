package search

import (
	"context"
	"database/sql"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// chunkTypeCapturingQuerier records the ChunkType passed to each vector-search
// leg so the test can prove HybridSearch forwards the filter to the vector half
// (issue #571 / #542 F7). It embeds *mockQuerier for all the other methods.
type chunkTypeCapturingQuerier struct {
	*mockQuerier
	got map[string]sql.NullString
}

func (c *chunkTypeCapturingQuerier) VectorSearch(ctx context.Context, arg sqlc.VectorSearchParams) ([]sqlc.VectorSearchRow, error) {
	c.got["VectorSearch"] = arg.ChunkType
	return nil, nil
}
func (c *chunkTypeCapturingQuerier) VectorSearchAll(ctx context.Context, arg sqlc.VectorSearchAllParams) ([]sqlc.VectorSearchAllRow, error) {
	c.got["VectorSearchAll"] = arg.ChunkType
	return nil, nil
}
func (c *chunkTypeCapturingQuerier) VectorSearchWithTags(ctx context.Context, arg sqlc.VectorSearchWithTagsParams) ([]sqlc.VectorSearchWithTagsRow, error) {
	c.got["VectorSearchWithTags"] = arg.ChunkType
	return nil, nil
}
func (c *chunkTypeCapturingQuerier) VectorSearchAllWithTags(ctx context.Context, arg sqlc.VectorSearchAllWithTagsParams) ([]sqlc.VectorSearchAllWithTagsRow, error) {
	c.got["VectorSearchAllWithTags"] = arg.ChunkType
	return nil, nil
}

// F7: the vector leg of HybridSearch previously dropped ChunkType, so
// memory_query chunk_type filtering was ignored on the vector half. Assert all
// four vector paths now receive the filter.
func TestHybridSearch_ForwardsChunkTypeToVectorLeg(t *testing.T) {
	cases := []struct {
		name      string
		workspace string
		tags      []string
		wantKey   string
	}{
		{"workspace+no-tags", "ws-hash", nil, "VectorSearch"},
		{"workspace+tags", "ws-hash", []string{"go"}, "VectorSearchWithTags"},
		{"all+no-tags", "all", nil, "VectorSearchAll"},
		{"all+tags", "all", []string{"go"}, "VectorSearchAllWithTags"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &chunkTypeCapturingQuerier{mockQuerier: &mockQuerier{}, got: map[string]sql.NullString{}}
			svc := NewSearchService(q, &mockEmbedder{}, config.SearchConfig{RrfK: 60, Limit: 20}, zerolog.Nop())

			_, err := svc.HybridSearch(context.Background(), "deposit error handling", tc.workspace, 10, tc.tags, nil, "symbol", "")
			if err != nil {
				t.Fatalf("HybridSearch: %v", err)
			}

			ct, ok := q.got[tc.wantKey]
			if !ok {
				t.Fatalf("vector leg %s was not called; captured: %v", tc.wantKey, q.got)
			}
			if !ct.Valid || ct.String != "symbol" {
				t.Errorf("%s received ChunkType=%+v, want {symbol,valid}", tc.wantKey, ct)
			}
		})
	}
}
