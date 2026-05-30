package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// StatsQuerier is the DB interface for the stats endpoint.
type StatsQuerier interface {
	CountDocsByCollectionGrouped(ctx context.Context, workspaceHash string) ([]sqlc.CountDocsByCollectionGroupedRow, error)
	CountChunksByEmbedStatus(ctx context.Context, workspaceHash string) ([]sqlc.CountChunksByEmbedStatusRow, error)
	CountGraphEdgesByType(ctx context.Context, workspaceHash string) ([]sqlc.CountGraphEdgesByTypeRow, error)
	ListTopTags(ctx context.Context, workspaceHash string) ([]sqlc.ListTopTagsRow, error)
	ListRecentDocuments(ctx context.Context, workspaceHash string) ([]sqlc.ListRecentDocumentsRow, error)
	ListRecentQueries(ctx context.Context, workspaceHash string) ([]sqlc.ListRecentQueriesRow, error)
}

type collectionCount struct {
	Collection string `json:"collection"`
	DocCount   int64  `json:"doc_count"`
}

type embedStatusCount struct {
	EmbedStatus string `json:"embed_status"`
	ChunkCount  int64  `json:"chunk_count"`
}

type edgeTypeCount struct {
	EdgeType  string `json:"edge_type"`
	EdgeCount int64  `json:"edge_count"`
}

type tagCount struct {
	Tag      string `json:"tag"`
	DocCount int64  `json:"doc_count"`
}

type recentDoc struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Collection string    `json:"collection"`
	UpdatedAt  time.Time `json:"updated_at"`
	Tags       []string  `json:"tags"`
}

type recentQuery struct {
	Query string    `json:"query"`
	Ts    time.Time `json:"ts"`
}

type statsResponse struct {
	Collections []collectionCount  `json:"collections"`
	Chunks      []embedStatusCount `json:"chunks"`
	GraphEdges  []edgeTypeCount    `json:"graph_edges"`
	TopTags     []tagCount         `json:"top_tags"`
	RecentDocs  []recentDoc        `json:"recent_docs"`
	RecentQueries []recentQuery    `json:"recent_queries"`
}

// Stats returns workspace statistics as a JSON aggregation.
func Stats(q StatsQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)
		ctx := c.Request().Context()

		cols, err := q.CountDocsByCollectionGrouped(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Msg("stats: collections query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
		}
		chunks, err := q.CountChunksByEmbedStatus(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Msg("stats: chunks query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
		}
		edges, err := q.CountGraphEdgesByType(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Msg("stats: edges query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
		}
		tags, err := q.ListTopTags(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Msg("stats: tags query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
		}
		docs, err := q.ListRecentDocuments(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Msg("stats: recent docs query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
		}
		queries, err := q.ListRecentQueries(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Msg("stats: recent queries failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
		}

		resp := statsResponse{
			Collections:   make([]collectionCount, 0, len(cols)),
			Chunks:        make([]embedStatusCount, 0, len(chunks)),
			GraphEdges:    make([]edgeTypeCount, 0, len(edges)),
			TopTags:       make([]tagCount, 0, len(tags)),
			RecentDocs:    make([]recentDoc, 0, len(docs)),
			RecentQueries: make([]recentQuery, 0, len(queries)),
		}
		for _, c := range cols {
			resp.Collections = append(resp.Collections, collectionCount{Collection: c.Collection, DocCount: c.DocCount})
		}
		for _, c := range chunks {
			resp.Chunks = append(resp.Chunks, embedStatusCount{EmbedStatus: c.EmbedStatus, ChunkCount: c.ChunkCount})
		}
		for _, e := range edges {
			resp.GraphEdges = append(resp.GraphEdges, edgeTypeCount{EdgeType: e.EdgeType, EdgeCount: e.EdgeCount})
		}
		for _, t := range tags {
			resp.TopTags = append(resp.TopTags, tagCount{Tag: t.Tag, DocCount: t.DocCount})
		}
		for _, d := range docs {
			t := d.Tags
			if t == nil {
				t = []string{}
			}
			resp.RecentDocs = append(resp.RecentDocs, recentDoc{
				ID: d.ID, Title: d.Title, Collection: d.Collection,
				UpdatedAt: d.UpdatedAt, Tags: t,
			})
		}
		for _, q := range queries {
			resp.RecentQueries = append(resp.RecentQueries, recentQuery{Query: q.QueryText, Ts: q.CreatedAt})
		}

		return c.JSON(http.StatusOK, resp)
	}
}
