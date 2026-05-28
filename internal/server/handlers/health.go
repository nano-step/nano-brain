package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

type PoolChecker interface {
	Ping(ctx context.Context) error
}

type EmbedQueueInfo interface {
	Depth() int
	Capacity() int
	Status() string
	PendingCount() int64
}

type WorkspaceCounter interface {
	CountWorkspaces(ctx context.Context) (int64, error)
}

type Health struct {
	pool             PoolChecker
	queue            EmbedQueueInfo
	logger           zerolog.Logger
	version          string
	startTime        time.Time
	getCfg           func() (config.HarvesterConfig, config.IntervalsConfig)
	counter          WorkspaceCounter
	embedCfg         config.EmbeddingConfig
	migrationVersion int64
}

func NewHealth(pool PoolChecker, logger zerolog.Logger, version string, startTime time.Time, queue EmbedQueueInfo, getCfg func() (config.HarvesterConfig, config.IntervalsConfig), counter WorkspaceCounter, embedCfg config.EmbeddingConfig, migrationVersion int64) *Health {
	return &Health{pool: pool, queue: queue, logger: logger, version: version, startTime: startTime, getCfg: getCfg, counter: counter, embedCfg: embedCfg, migrationVersion: migrationVersion}
}

func (h *Health) workspaceCount(ctx context.Context) int {
	if h.counter == nil {
		return 0
	}
	n, err := h.counter.CountWorkspaces(ctx)
	if err != nil {
		h.logger.Warn().Err(err).Msg("failed to count workspaces; reporting 0")
		return 0
	}
	return int(n)
}

type healthResponse struct {
	Status         string `json:"status"`
	Ready          bool   `json:"ready"`
	Version        string `json:"version,omitempty"`
	UptimeS        int64  `json:"uptime_s,omitempty"`
	WorkspaceCount int    `json:"workspace_count,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

type harvesterStatusResponse struct {
	PollIntervalSeconds int `json:"poll_interval_seconds"`
	OpenCode            struct {
		Enabled    bool   `json:"enabled"`
		SessionDir string `json:"session_dir"`
	} `json:"opencode"`
	ClaudeCode struct {
		Enabled    bool   `json:"enabled"`
		SessionDir string `json:"session_dir"`
	} `json:"claude_code"`
}

type statusResponse struct {
	PGStatus             string                    `json:"pg_status"`
	MigrationVersion     int                       `json:"migration_version"`
	EmbeddingQueueDepth  int                       `json:"embedding_queue_depth"`
	ActiveProvider       string                    `json:"active_provider"`
	WorkspaceCount       int                       `json:"workspace_count"`
	QueueDepth           int                       `json:"queue_depth"`
	QueueCapacity        int                       `json:"queue_capacity"`
	QueueStatus          string                    `json:"queue_status"`
	QueuePending         int64                     `json:"queue_pending"`
	HarvesterStatus      harvesterStatusResponse   `json:"harvester_status"`
}

func (h *Health) Health(c echo.Context) error {
	if err := h.pool.Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusOK, healthResponse{
			Status: "degraded",
			Ready:  false,
			Reason: "database unreachable",
		})
	}

	return c.JSON(http.StatusOK, healthResponse{
		Status:         "ok",
		Ready:          true,
		Version:        h.version,
		UptimeS:        int64(time.Since(h.startTime).Seconds()),
		WorkspaceCount: h.workspaceCount(c.Request().Context()),
	})
}

func (h *Health) Status(c echo.Context) error {
	pgStatus := "healthy"
	if err := h.pool.Ping(c.Request().Context()); err != nil {
		pgStatus = "unreachable"
	}

	harvesterCfg, intervalsCfg := h.getCfg()

	harvestStatus := harvesterStatusResponse{
		PollIntervalSeconds: intervalsCfg.SessionPoll,
	}
	harvestStatus.OpenCode.Enabled = harvesterCfg.OpenCode.SessionDir != ""
	harvestStatus.OpenCode.SessionDir = harvesterCfg.OpenCode.SessionDir
	harvestStatus.ClaudeCode.Enabled = harvesterCfg.ClaudeCode.Enabled
	harvestStatus.ClaudeCode.SessionDir = harvesterCfg.ClaudeCode.SessionDir

	resp := statusResponse{
		PGStatus:            pgStatus,
		MigrationVersion:    int(h.migrationVersion),
		EmbeddingQueueDepth: 0,
		ActiveProvider:      h.embedCfg.Provider,
		WorkspaceCount:      h.workspaceCount(c.Request().Context()),
		HarvesterStatus:     harvestStatus,
	}

	if h.queue != nil {
		resp.QueueDepth = h.queue.Depth()
		resp.QueueCapacity = h.queue.Capacity()
		resp.QueueStatus = h.queue.Status()
		resp.QueuePending = h.queue.PendingCount()
		resp.EmbeddingQueueDepth = h.queue.Depth()
	}

	return c.JSON(http.StatusOK, resp)
}
