//go:build integration

package search_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
	pgvector_go "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

const vecDim = 768

type fakeEmbedder struct {
	vec []float32
}

func (f *fakeEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return f.vec, nil
}

func (f *fakeEmbedder) Dimension() int { return vecDim }

func makeVec(val float32) []float32 {
	v := make([]float32, vecDim)
	for i := range v {
		v[i] = val
	}
	return v
}

func contentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:16])
}

type workspace struct {
	hash    string
	name    string
	path    string
	keyword string
	vec     []float32
}

func setupQueries(t *testing.T) *sqlc.Queries {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = db.Close() })
	return sqlc.New(db)
}

func seedWorkspace(t *testing.T, ctx context.Context, q *sqlc.Queries, ws workspace, docCount int) {
	t.Helper()

	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: ws.hash,
		Name: ws.name,
		Path: ws.path,
	})
	if err != nil {
		t.Fatalf("UpsertWorkspace(%s): %v", ws.hash, err)
	}

	for i := 0; i < docCount; i++ {
		content := fmt.Sprintf("document about %s topic number %d with unique keyword %s", ws.name, i, ws.keyword)
		title := fmt.Sprintf("%s-doc-%d", ws.name, i)
		cHash := contentHash(content + ws.hash + fmt.Sprintf("%d", i))

		doc, err := q.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
			WorkspaceHash: ws.hash,
			ContentHash:   cHash,
			Title:         title,
			Content:       content,
			SourcePath:    fmt.Sprintf("/src/%s/file_%d.go", ws.name, i),
			Collection:    "codebase",
			Tags:          []string{ws.name},
			Metadata:      pqtype.NullRawMessage{},
		})
		if err != nil {
			t.Fatalf("UpsertDocument(%s, %d): %v", ws.hash, i, err)
		}

		chunkContent := fmt.Sprintf("chunk containing the rare keyword %s in workspace %s item %d", ws.keyword, ws.name, i)
		chunkHash := contentHash(chunkContent + ws.hash + fmt.Sprintf("%d", i))

		chunkID, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        doc.ID,
			WorkspaceHash:     ws.hash,
			ContentHash:       chunkHash,
			Content:           chunkContent,
			ChunkIndex:        int32(i),
			StartLine:         sql.NullInt32{Int32: 1, Valid: true},
			EndLine:           sql.NullInt32{Int32: 10, Valid: true},
			Metadata:          pqtype.NullRawMessage{},
			ChunkType:         "raw",
			EmbeddingStrategy: "raw_code",
		})
		if err != nil {
			t.Fatalf("UpsertChunk(%s, %d): %v", ws.hash, i, err)
		}

		_, err = q.InsertEmbedding(ctx, sqlc.InsertEmbeddingParams{
			ChunkID:       chunkID,
			WorkspaceHash: ws.hash,
			Provider:      "test",
			Model:         "test-model",
			Embedding:     pgvector_go.NewVector(ws.vec),
		})
		if err != nil {
			t.Fatalf("InsertEmbedding(%s, %d): %v", ws.hash, i, err)
		}
	}
}

var (
	wsAlpha = workspace{
		hash:    "ws_alpha_isolation_test",
		name:    "alpha",
		path:    "/tmp/alpha-project",
		keyword: "xylophone",
		vec:     makeVec(0.1),
	}
	wsBeta = workspace{
		hash:    "ws_beta_isolation_test",
		name:    "beta",
		path:    "/tmp/beta-project",
		keyword: "zeppelin",
		vec:     makeVec(0.9),
	}
)

func TestBM25SearchIsolation(t *testing.T) {
	q := setupQueries(t)
	ctx := context.Background()

	seedWorkspace(t, ctx, q, wsAlpha, 5)
	seedWorkspace(t, ctx, q, wsBeta, 5)

	t.Run("alpha_keyword_returns_only_alpha", func(t *testing.T) {
		rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
			Query:         wsAlpha.keyword,
			WorkspaceHash: wsAlpha.hash,
			MaxResults:    50,
		})
		if err != nil {
			t.Fatalf("BM25Search: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected results for alpha keyword in alpha workspace, got 0")
		}
		for _, r := range rows {
			if r.WorkspaceHash != wsAlpha.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsAlpha.hash)
			}
		}
	})

	t.Run("beta_keyword_returns_only_beta", func(t *testing.T) {
		rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
			Query:         wsBeta.keyword,
			WorkspaceHash: wsBeta.hash,
			MaxResults:    50,
		})
		if err != nil {
			t.Fatalf("BM25Search: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected results for beta keyword in beta workspace, got 0")
		}
		for _, r := range rows {
			if r.WorkspaceHash != wsBeta.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsBeta.hash)
			}
		}
	})

	t.Run("beta_keyword_in_alpha_workspace_returns_zero", func(t *testing.T) {
		rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
			Query:         wsBeta.keyword,
			WorkspaceHash: wsAlpha.hash,
			MaxResults:    50,
		})
		if err != nil {
			t.Fatalf("BM25Search: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("cross-workspace leak: searching beta keyword in alpha returned %d results", len(rows))
		}
	})

	t.Run("alpha_keyword_in_beta_workspace_returns_zero", func(t *testing.T) {
		rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
			Query:         wsAlpha.keyword,
			WorkspaceHash: wsBeta.hash,
			MaxResults:    50,
		})
		if err != nil {
			t.Fatalf("BM25Search: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("cross-workspace leak: searching alpha keyword in beta returned %d results", len(rows))
		}
	})
}

func TestVectorSearchIsolation(t *testing.T) {
	q := setupQueries(t)
	ctx := context.Background()

	seedWorkspace(t, ctx, q, wsAlpha, 5)
	seedWorkspace(t, ctx, q, wsBeta, 5)

	t.Run("alpha_vector_returns_only_alpha", func(t *testing.T) {
		rows, err := q.VectorSearch(ctx, sqlc.VectorSearchParams{
			QueryEmbedding: pgvector_go.NewVector(wsAlpha.vec),
			WorkspaceHash:  wsAlpha.hash,
			MaxResults:     50,
		})
		if err != nil {
			t.Fatalf("VectorSearch: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected results for alpha vector in alpha workspace, got 0")
		}
		for _, r := range rows {
			if r.WorkspaceHash != wsAlpha.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsAlpha.hash)
			}
		}
	})

	t.Run("beta_vector_returns_only_beta", func(t *testing.T) {
		rows, err := q.VectorSearch(ctx, sqlc.VectorSearchParams{
			QueryEmbedding: pgvector_go.NewVector(wsBeta.vec),
			WorkspaceHash:  wsBeta.hash,
			MaxResults:     50,
		})
		if err != nil {
			t.Fatalf("VectorSearch: %v", err)
		}
		if len(rows) == 0 {
			t.Fatal("expected results for beta vector in beta workspace, got 0")
		}
		for _, r := range rows {
			if r.WorkspaceHash != wsBeta.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsBeta.hash)
			}
		}
	})

	t.Run("alpha_vector_in_beta_scope_returns_no_alpha", func(t *testing.T) {
		rows, err := q.VectorSearch(ctx, sqlc.VectorSearchParams{
			QueryEmbedding: pgvector_go.NewVector(wsAlpha.vec),
			WorkspaceHash:  wsBeta.hash,
			MaxResults:     50,
		})
		if err != nil {
			t.Fatalf("VectorSearch: %v", err)
		}
		for _, r := range rows {
			if r.WorkspaceHash != wsBeta.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsBeta.hash)
			}
		}
	})

	t.Run("beta_vector_in_alpha_scope_returns_no_beta", func(t *testing.T) {
		rows, err := q.VectorSearch(ctx, sqlc.VectorSearchParams{
			QueryEmbedding: pgvector_go.NewVector(wsBeta.vec),
			WorkspaceHash:  wsAlpha.hash,
			MaxResults:     50,
		})
		if err != nil {
			t.Fatalf("VectorSearch: %v", err)
		}
		for _, r := range rows {
			if r.WorkspaceHash != wsAlpha.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsAlpha.hash)
			}
		}
	})
}

func TestHybridSearchIsolation(t *testing.T) {
	q := setupQueries(t)
	ctx := context.Background()

	seedWorkspace(t, ctx, q, wsAlpha, 5)
	seedWorkspace(t, ctx, q, wsBeta, 5)

	cfg := config.SearchConfig{
		RrfK:                60,
		RecencyWeight:       0.3,
		RecencyHalfLifeDays: 180,
		Limit:               20,
	}

	t.Run("alpha_hybrid_returns_only_alpha", func(t *testing.T) {
		svc := search.NewSearchService(q, &fakeEmbedder{vec: wsAlpha.vec}, cfg, zerolog.Nop())
		results, err := svc.HybridSearch(ctx, wsAlpha.keyword, wsAlpha.hash, 20, nil, nil)
		if err != nil {
			t.Fatalf("HybridSearch: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected results for alpha hybrid search, got 0")
		}
		for _, r := range results {
			if r.WorkspaceHash != wsAlpha.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsAlpha.hash)
			}
		}
	})

	t.Run("beta_hybrid_returns_only_beta", func(t *testing.T) {
		svc := search.NewSearchService(q, &fakeEmbedder{vec: wsBeta.vec}, cfg, zerolog.Nop())
		results, err := svc.HybridSearch(ctx, wsBeta.keyword, wsBeta.hash, 20, nil, nil)
		if err != nil {
			t.Fatalf("HybridSearch: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected results for beta hybrid search, got 0")
		}
		for _, r := range results {
			if r.WorkspaceHash != wsBeta.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsBeta.hash)
			}
		}
	})

	t.Run("beta_keyword_in_alpha_scope_hybrid_returns_zero", func(t *testing.T) {
		svc := search.NewSearchService(q, &fakeEmbedder{vec: wsBeta.vec}, cfg, zerolog.Nop())
		results, err := svc.HybridSearch(ctx, wsBeta.keyword, wsAlpha.hash, 20, nil, nil)
		if err != nil {
			t.Fatalf("HybridSearch: %v", err)
		}
		for _, r := range results {
			if r.WorkspaceHash != wsAlpha.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsAlpha.hash)
			}
		}
	})

	t.Run("alpha_keyword_in_beta_scope_hybrid_returns_zero", func(t *testing.T) {
		svc := search.NewSearchService(q, &fakeEmbedder{vec: wsAlpha.vec}, cfg, zerolog.Nop())
		results, err := svc.HybridSearch(ctx, wsAlpha.keyword, wsBeta.hash, 20, nil, nil)
		if err != nil {
			t.Fatalf("HybridSearch: %v", err)
		}
		for _, r := range results {
			if r.WorkspaceHash != wsBeta.hash {
				t.Errorf("leaked: got workspace %q, want %q", r.WorkspaceHash, wsBeta.hash)
			}
		}
	})
}

func TestHTTP400WhenWorkspaceMissing(t *testing.T) {
	q := setupQueries(t)
	logger := zerolog.Nop()

	cfg := config.SearchConfig{
		RrfK:                60,
		RecencyWeight:       0.3,
		RecencyHalfLifeDays: 180,
		Limit:               20,
	}
	svc := search.NewSearchService(q, &fakeEmbedder{vec: makeVec(0.5)}, cfg, logger)

	tests := []struct {
		name    string
		handler echo.HandlerFunc
	}{
		{"bm25_search", handlers.BM25Search(q, logger)},
		{"vector_search", handlers.VectorSearch(q, &fakeEmbedder{vec: makeVec(0.5)}, logger)},
		{"query_hybrid", handlers.Query(svc, logger)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			body, _ := json.Marshal(map[string]string{"query": "test"})
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := tt.handler(c)
			if err == nil {
				t.Fatal("expected error for missing workspace, got nil")
			}
			httpErr, ok := err.(*echo.HTTPError)
			if !ok {
				t.Fatalf("expected *echo.HTTPError, got %T", err)
			}
			if httpErr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", httpErr.Code)
			}
		})
	}
}

func TestCrossWorkspacePermutations(t *testing.T) {
	q := setupQueries(t)
	ctx := context.Background()

	workspaces := []workspace{
		{hash: "ws_perm_alpha", name: "perm_alpha", path: "/tmp/perm-alpha", keyword: "brontosaurus", vec: makeVec(0.10)},
		{hash: "ws_perm_beta", name: "perm_beta", path: "/tmp/perm-beta", keyword: "velociraptor", vec: makeVec(0.25)},
		{hash: "ws_perm_gamma", name: "perm_gamma", path: "/tmp/perm-gamma", keyword: "pterodactyl", vec: makeVec(0.40)},
		{hash: "ws_perm_delta", name: "perm_delta", path: "/tmp/perm-delta", keyword: "triceratops", vec: makeVec(0.55)},
		{hash: "ws_perm_epsilon", name: "perm_epsilon", path: "/tmp/perm-epsilon", keyword: "stegosaurus", vec: makeVec(0.70)},
	}

	for _, ws := range workspaces {
		seedWorkspace(t, ctx, q, ws, 3)
	}

	cfg := config.SearchConfig{
		RrfK:                60,
		RecencyWeight:       0.3,
		RecencyHalfLifeDays: 180,
		Limit:               20,
	}

	permCount := 0
	for _, queried := range workspaces {
		for _, other := range workspaces {
			if queried.hash == other.hash {
				continue
			}

			t.Run(fmt.Sprintf("bm25_%s_keyword_in_%s", other.name, queried.name), func(t *testing.T) {
				rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
					Query:         other.keyword,
					WorkspaceHash: queried.hash,
					MaxResults:    50,
				})
				if err != nil {
					t.Fatalf("BM25Search: %v", err)
				}
				for _, r := range rows {
					if r.WorkspaceHash != queried.hash {
						t.Errorf("BM25 leak: queried %q, got result from %q", queried.hash, r.WorkspaceHash)
					}
				}
			})
			permCount++

			t.Run(fmt.Sprintf("vector_%s_vec_in_%s", other.name, queried.name), func(t *testing.T) {
				rows, err := q.VectorSearch(ctx, sqlc.VectorSearchParams{
					QueryEmbedding: pgvector_go.NewVector(other.vec),
					WorkspaceHash:  queried.hash,
					MaxResults:     50,
				})
				if err != nil {
					t.Fatalf("VectorSearch: %v", err)
				}
				for _, r := range rows {
					if r.WorkspaceHash != queried.hash {
						t.Errorf("Vector leak: queried %q, got result from %q", queried.hash, r.WorkspaceHash)
					}
				}
			})
			permCount++

			t.Run(fmt.Sprintf("hybrid_%s_keyword_in_%s", other.name, queried.name), func(t *testing.T) {
				svc := search.NewSearchService(q, &fakeEmbedder{vec: other.vec}, cfg, zerolog.Nop())
				results, err := svc.HybridSearch(ctx, other.keyword, queried.hash, 20, nil, nil)
				if err != nil {
					t.Fatalf("HybridSearch: %v", err)
				}
				for _, r := range results {
					if r.WorkspaceHash != queried.hash {
						t.Errorf("Hybrid leak: queried %q, got result from %q", queried.hash, r.WorkspaceHash)
					}
				}
			})
			permCount++

			t.Run(fmt.Sprintf("bm25_own_%s_keyword_scoped_%s", queried.name, queried.name), func(t *testing.T) {
				rows, err := q.BM25Search(ctx, sqlc.BM25SearchParams{
					Query:         queried.keyword,
					WorkspaceHash: queried.hash,
					MaxResults:    50,
				})
				if err != nil {
					t.Fatalf("BM25Search: %v", err)
				}
				if len(rows) == 0 {
					t.Errorf("expected results for own keyword %q in %q", queried.keyword, queried.hash)
				}
				for _, r := range rows {
					if r.WorkspaceHash != queried.hash {
						t.Errorf("BM25 own-keyword leak: queried %q, got %q", queried.hash, r.WorkspaceHash)
					}
				}
			})
			permCount++

			t.Run(fmt.Sprintf("vector_own_%s_vec_scoped_%s", queried.name, queried.name), func(t *testing.T) {
				rows, err := q.VectorSearch(ctx, sqlc.VectorSearchParams{
					QueryEmbedding: pgvector_go.NewVector(queried.vec),
					WorkspaceHash:  queried.hash,
					MaxResults:     50,
				})
				if err != nil {
					t.Fatalf("VectorSearch: %v", err)
				}
				if len(rows) == 0 {
					t.Errorf("expected results for own vector in %q", queried.hash)
				}
				for _, r := range rows {
					if r.WorkspaceHash != queried.hash {
						t.Errorf("Vector own-vec leak: queried %q, got %q", queried.hash, r.WorkspaceHash)
					}
				}
			})
			permCount++
		}
	}

	if permCount < 100 {
		t.Errorf("expected at least 100 cross-workspace permutations, got %d", permCount)
	}
	t.Logf("executed %d cross-workspace isolation permutations", permCount)
}
