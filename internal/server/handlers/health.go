package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

// PoolChecker is the consumer-side interface for database health checks.
type PoolChecker interface {
	Ping(ctx context.Context) error
}

// Health holds dependencies for health-check handlers.
type Health struct {
	pool      PoolChecker
	logger    zerolog.Logger
	version   string
	startTime time.Time
}

// NewHealth creates a new Health handler.
func NewHealth(pool PoolChecker, logger zerolog.Logger, version string, startTime time.Time) *Health {
	return &Health{pool: pool, logger: logger, version: version, startTime: startTime}
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
}

// Health handles GET /health.
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

// Status handles GET /api/status.
func (h *Health) Status(c echo.Context) error {
	pgStatus := "healthy"
	if err := h.pool.Ping(c.Request().Context()); err != nil {
		pgStatus = "unreachable"
	}

	return c.JSON(http.StatusOK, statusResponse{
		PGStatus:            pgStatus,
		MigrationVersion:    1,
		EmbeddingQueueDepth: 0,
		ActiveProvider:      "none",
		WorkspaceCount:      0,
	})
}
