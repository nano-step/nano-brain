package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type WakeUpQuerier interface {
	RecentDocuments(ctx context.Context, arg sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error)
	WorkspaceDocStats(ctx context.Context, workspaceHash string) (sqlc.WorkspaceDocStatsRow, error)
	WorkspaceChunkCount(ctx context.Context, workspaceHash string) (int64, error)
	ListCollectionsWithLastUpdated(ctx context.Context, workspaceHash string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error)
}

type WakeUpRequest struct {
	Workspace string `json:"workspace"`
	Limit     int    `json:"limit,omitempty"`
}

type WakeUpResponse struct {
	Summary           string             `json:"summary"`
	RecentMemories    []RecentMemory     `json:"recent_memories"`
	ActiveCollections []ActiveCollection `json:"active_collections"`
	Stats             WakeUpStats        `json:"stats"`
}

type RecentMemory struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Snippet string   `json:"snippet"`
	Tags    []string `json:"tags"`
	Date    string   `json:"date"`
}

type ActiveCollection struct {
	Name          string `json:"name"`
	DocumentCount int64  `json:"document_count"`
	LastUpdated   string `json:"last_updated"`
}

type WakeUpStats struct {
	TotalDocuments int64  `json:"total_documents"`
	TotalChunks    int64  `json:"total_chunks"`
	LastActivity   string `json:"last_activity"`
}

const defaultWakeUpLimit = 10

// wakeUpUnauthenticatedDoc and wakeUpScopedDoc are never called — they exist
// purely as swag doc-comment anchors. WakeUpHandler backs both GET
// /api/v1/wake-up (api group, no middleware) and POST /api/v1/wake-up (data
// group, gated by workspaceMiddleware) with the SAME implementation, but the
// two routes have DIFFERENT auth requirements — a single shared @Router
// block can't carry two different @Security tags, so each mount point gets
// its own doc-only anchor (mirroring protocol_doc.go's pattern) rather than
// one block silently omitting @Security for the authenticated path.

// wakeUpUnauthenticatedDoc godoc
// @Summary      Get a session-start context summary (unauthenticated)
// @Description  Returns recent memories, active collections, and workspace stats. No workspace middleware on this GET mount.
// @Tags         wakeup
// @Produce      json
// @Success      200 {object} WakeUpResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /api/v1/wake-up [get]
//
//nolint:unused // doc-only anchor for swag, never invoked
func wakeUpUnauthenticatedDoc() {}

// wakeUpScopedDoc godoc
// @Summary      Get a session-start context summary (workspace-scoped)
// @Description  Returns recent memories, active collections, and workspace stats. This POST mount is in the data group, gated by workspaceMiddleware.
// @Tags         wakeup
// @Produce      json
// @Success      200 {object} WakeUpResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/wake-up [post]
//
//nolint:unused // doc-only anchor for swag, never invoked
func wakeUpScopedDoc() {}

// WakeUpHandler backs both /api/v1/wake-up mount points (see the doc anchors
// above for their swag annotations — this function is not itself annotated
// since the two routes require different @Security tags).
func WakeUpHandler(q WakeUpQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req WakeUpRequest
		workspace := c.QueryParam("workspace")
		if workspace == "" {
			workspace, _ = c.Get("workspace").(string)
		}
		if workspace == "" {
			_ = c.Bind(&req)
			workspace = req.Workspace
		}
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		limit := defaultWakeUpLimit
		if lq := c.QueryParam("limit"); lq != "" {
			if v, err := strconv.Atoi(lq); err == nil {
				limit = v
			}
		} else if req.Limit > 0 {
			limit = req.Limit
		}
		if limit <= 0 {
			limit = defaultWakeUpLimit
		}
		if limit > 50 {
			limit = 50
		}

		ctx := c.Request().Context()

		docs, err := q.RecentDocuments(ctx, sqlc.RecentDocumentsParams{
			WorkspaceHash: workspace,
			MaxResults:    int32(limit),
			Collections:   []string{"memory", "sessions"},
		})
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("wake-up: recent documents failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch recent documents")
		}

		docStats, err := q.WorkspaceDocStats(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("wake-up: doc stats failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch workspace stats")
		}

		chunkCount, err := q.WorkspaceChunkCount(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("wake-up: chunk count failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch chunk count")
		}

		collections, err := q.ListCollectionsWithLastUpdated(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("wake-up: collections failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch collections")
		}

		memories := make([]RecentMemory, 0, len(docs))
		for _, d := range docs {
			tags := d.Tags
			if tags == nil {
				tags = []string{}
			}
			memories = append(memories, RecentMemory{
				ID:      d.ID.String(),
				Title:   d.Title,
				Snippet: d.Snippet,
				Tags:    tags,
				Date:    d.UpdatedAt.Format(time.RFC3339),
			})
		}

		cols := make([]ActiveCollection, 0, len(collections))
		for _, col := range collections {
			lastUpdated := ""
			if t, ok := col.LastUpdated.(time.Time); ok {
				lastUpdated = t.Format(time.RFC3339)
			}
			cols = append(cols, ActiveCollection{
				Name:          col.Name,
				DocumentCount: col.DocumentCount,
				LastUpdated:   lastUpdated,
			})
		}

		var lastActivity string
		if t, ok := docStats.LastUpdated.(time.Time); ok {
			lastActivity = t.Format(time.RFC3339)
		}

		summary := formatWakeUpSummary(docStats.TotalDocuments, int64(len(collections)), docStats.LastUpdated)

		return c.JSON(http.StatusOK, WakeUpResponse{
			Summary:           summary,
			RecentMemories:    memories,
			ActiveCollections: cols,
			Stats: WakeUpStats{
				TotalDocuments: docStats.TotalDocuments,
				TotalChunks:    chunkCount,
				LastActivity:   lastActivity,
			},
		})
	}
}

func formatWakeUpSummary(totalDocs, totalCollections int64, lastUpdated interface{}) string {
	timeAgo := "never"
	if t, ok := lastUpdated.(time.Time); ok {
		timeAgo = formatTimeAgo(time.Since(t))
	}
	return fmt.Sprintf("Workspace has %d documents across %d collections. Last activity: %s.", totalDocs, totalCollections, timeAgo)
}

func formatTimeAgo(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}
