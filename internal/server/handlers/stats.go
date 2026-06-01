package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type StatsQuerier interface {
	CountDocsByCollectionGrouped(ctx context.Context, workspaceHash string) ([]sqlc.CountDocsByCollectionGroupedRow, error)
	CountChunksByEmbedStatus(ctx context.Context, workspaceHash string) ([]sqlc.CountChunksByEmbedStatusRow, error)
	CountGraphEdgesByType(ctx context.Context, workspaceHash string) ([]sqlc.CountGraphEdgesByTypeRow, error)
	ListTopTags(ctx context.Context, workspaceHash string) ([]sqlc.ListTopTagsRow, error)
	ListRecentDocuments(ctx context.Context, workspaceHash string) ([]sqlc.ListRecentDocumentsRow, error)
	CountEmbeddingsByWorkspace(ctx context.Context, workspaceHash string) (int64, error)
}

type embeddingInfo struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Dim      int    `json:"dim"`
}

type chunksByEmbedStatusObj struct {
	Pending     int64 `json:"pending"`
	Embedded    int64 `json:"embedded"`
	EmbedFailed int64 `json:"embed_failed"`
}

type collectionItem struct {
	Name     string `json:"name"`
	DocCount int64  `json:"doc_count"`
}

type tagCountItem struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

type harvestInfo struct {
	Mode         string `json:"mode"`
	LastAt       string `json:"last_at"`
	SessionsSeen int64  `json:"sessions_seen"`
}

type watcherInfo struct {
	CollectionsWatched int `json:"collections_watched"`
	DebounceMs         int `json:"debounce_ms"`
	PollIntervalSec    int `json:"poll_interval_sec"`
	Dirty              int `json:"dirty"`
}

type recentDocItem struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Collection string    `json:"collection"`
	UpdatedAt  time.Time `json:"updated_at"`
	Tags       []string  `json:"tags"`
}

// statsResponse matches web/src/api/types.ts:StatsResponse exactly.
// Field renames or shape changes here are breaking API changes — see
// openspec/specs/stats-api-contract for the canonical contract.
type statsResponse struct {
	ServerVersion       string                 `json:"server_version"`
	UptimeSec           int64                  `json:"uptime_sec"`
	Embedding           embeddingInfo          `json:"embedding"`
	MigrationVersion    int64                  `json:"migration_version"`
	DocsTotal           int64                  `json:"docs_total"`
	ChunksTotal         int64                  `json:"chunks_total"`
	ChunksByEmbedStatus chunksByEmbedStatusObj `json:"chunks_by_embed_status"`
	EmbeddingsTotal     int64                  `json:"embeddings_total"`
	GraphEdgesByType    map[string]int64       `json:"graph_edges_by_type"`
	Collections         []collectionItem       `json:"collections"`
	TagsTop20           []tagCountItem         `json:"tags_top_20"`
	Harvest             harvestInfo            `json:"harvest"`
	Watcher             watcherInfo            `json:"watcher"`
	RecentDocs          []recentDocItem        `json:"recent_docs"`
}

type WatcherInfoGetter interface {
	CollectionsWatched() int
}

type StatsHandler struct {
	queries          StatsQuerier
	logger           zerolog.Logger
	version          string
	startTime        time.Time
	embedCfg         config.EmbeddingConfig
	migrationVersion int64
	getCfg           func() (config.HarvesterConfig, config.IntervalsConfig)
	watcherCfg       config.WatcherConfig
	watcherInfo      WatcherInfoGetter
	harvestStatus    HarvestStatusSnapshot
}

func NewStatsHandler(
	queries StatsQuerier,
	logger zerolog.Logger,
	version string,
	startTime time.Time,
	embedCfg config.EmbeddingConfig,
	migrationVersion int64,
	getCfg func() (config.HarvesterConfig, config.IntervalsConfig),
	watcherCfg config.WatcherConfig,
	watcherInfo WatcherInfoGetter,
) *StatsHandler {
	return &StatsHandler{
		queries:          queries,
		logger:           logger,
		version:          version,
		startTime:        startTime,
		embedCfg:         embedCfg,
		migrationVersion: migrationVersion,
		getCfg:           getCfg,
		watcherCfg:       watcherCfg,
		watcherInfo:      watcherInfo,
	}
}

func (h *StatsHandler) SetHarvestStatus(s HarvestStatusSnapshot) {
	h.harvestStatus = s
}

func (h *StatsHandler) Handle(c echo.Context) error {
	workspace, _ := c.Get("workspace").(string)
	if workspace == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
	}
	ctx := c.Request().Context()

	cols, err := h.queries.CountDocsByCollectionGrouped(ctx, workspace)
	if err != nil {
		h.logger.Error().Err(err).Msg("stats: collections query failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
	}
	chunks, err := h.queries.CountChunksByEmbedStatus(ctx, workspace)
	if err != nil {
		h.logger.Error().Err(err).Msg("stats: chunks query failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
	}
	edges, err := h.queries.CountGraphEdgesByType(ctx, workspace)
	if err != nil {
		h.logger.Error().Err(err).Msg("stats: edges query failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
	}
	tags, err := h.queries.ListTopTags(ctx, workspace)
	if err != nil {
		h.logger.Error().Err(err).Msg("stats: tags query failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
	}
	docs, err := h.queries.ListRecentDocuments(ctx, workspace)
	if err != nil {
		h.logger.Error().Err(err).Msg("stats: recent docs query failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
	}
	embeddingsTotal, err := h.queries.CountEmbeddingsByWorkspace(ctx, workspace)
	if err != nil {
		h.logger.Error().Err(err).Msg("stats: embeddings total failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "stats query failed")
	}

	var docsTotal int64
	for _, c := range cols {
		docsTotal += c.DocCount
	}
	var chunksTotal int64
	for _, c := range chunks {
		chunksTotal += c.ChunkCount
	}

	resp := statsResponse{
		ServerVersion:    h.version,
		UptimeSec:        int64(time.Since(h.startTime).Seconds()),
		MigrationVersion: h.migrationVersion,
		DocsTotal:        docsTotal,
		ChunksTotal:      chunksTotal,
		EmbeddingsTotal:  embeddingsTotal,
		Embedding: embeddingInfo{
			Provider: h.embedCfg.Provider,
			Model:    h.embedCfg.Model,
			Dim:      h.embedCfg.Dimension,
		},
		GraphEdgesByType: make(map[string]int64, len(edges)),
		Collections:      make([]collectionItem, 0, len(cols)),
		TagsTop20:        make([]tagCountItem, 0, len(tags)),
		RecentDocs:       make([]recentDocItem, 0, len(docs)),
	}

	for _, c := range chunks {
		switch c.EmbedStatus {
		case "pending":
			resp.ChunksByEmbedStatus.Pending = c.ChunkCount
		case "embedded":
			resp.ChunksByEmbedStatus.Embedded = c.ChunkCount
		case "embed_failed":
			resp.ChunksByEmbedStatus.EmbedFailed = c.ChunkCount
		}
	}

	for _, e := range edges {
		resp.GraphEdgesByType[e.EdgeType] = e.EdgeCount
	}

	for _, c := range cols {
		resp.Collections = append(resp.Collections, collectionItem{Name: c.Collection, DocCount: c.DocCount})
	}

	for _, t := range tags {
		resp.TagsTop20 = append(resp.TagsTop20, tagCountItem{Tag: t.Tag, Count: t.DocCount})
	}

	for _, d := range docs {
		t := d.Tags
		if t == nil {
			t = []string{}
		}
		resp.RecentDocs = append(resp.RecentDocs, recentDocItem{
			ID:         d.ID,
			Title:      d.Title,
			Collection: d.Collection,
			UpdatedAt:  d.UpdatedAt,
			Tags:       t,
		})
	}

	if h.getCfg != nil {
		harvesterCfg, intervalsCfg := h.getCfg()
		mode := h.harvestStatus.Mode
		if mode == "" {
			mode = deriveOpenCodeMode(harvesterCfg.OpenCode)
		}
		resp.Harvest = harvestInfo{Mode: mode}
		resp.Watcher.PollIntervalSec = intervalsCfg.SessionPoll
	}
	resp.Watcher.DebounceMs = h.watcherCfg.DebounceMs
	if h.watcherInfo != nil {
		resp.Watcher.CollectionsWatched = h.watcherInfo.CollectionsWatched()
	}

	return c.JSON(http.StatusOK, resp)
}
