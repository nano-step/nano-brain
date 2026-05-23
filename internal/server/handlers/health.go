package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
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

type Health struct {
	pool      PoolChecker
	queue     EmbedQueueInfo
	logger    zerolog.Logger
	version   string
	startTime time.Time
}

func NewHealth(pool PoolChecker, logger zerolog.Logger, version string, startTime time.Time, queue EmbedQueueInfo) *Health {
	return &Health{pool: pool, queue: queue, logger: logger, version: version, startTime: startTime}
}

type healthResponse struct {
	Status         string `json:"status"`
	Ready          bool   `json:"ready"`
	Version        string `json:"version,omitempty"`
	UptimeS        int64  `json:"uptime_s,omitempty"`
	WorkspaceCount int    `json:"workspace_count,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

type statusResponse struct {
	PGStatus             string `json:"pg_status"`
	MigrationVersion     int    `json:"migration_version"`
	EmbeddingQueueDepth  int    `json:"embedding_queue_depth"`
	ActiveProvider       string `json:"active_provider"`
	WorkspaceCount       int    `json:"workspace_count"`
	QueueDepth           int    `json:"queue_depth"`
	QueueCapacity        int    `json:"queue_capacity"`
	QueueStatus          string `json:"queue_status"`
	QueuePending         int64  `json:"queue_pending"`
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
		WorkspaceCount: 0,
	})
}

func (h *Health) Status(c echo.Context) error {
	pgStatus := "healthy"
	if err := h.pool.Ping(c.Request().Context()); err != nil {
		pgStatus = "unreachable"
	}

	resp := statusResponse{
		PGStatus:            pgStatus,
		MigrationVersion:    1,
		EmbeddingQueueDepth: 0,
		ActiveProvider:      "none",
		WorkspaceCount:      0,
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
