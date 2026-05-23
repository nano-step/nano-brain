package mcp

import (
	"context"
	"database/sql"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// PoolChecker checks database connectivity.
type PoolChecker interface {
	Ping(ctx context.Context) error
}

// EmbedQueueInfo exposes embed queue statistics without importing the full queue.
type EmbedQueueInfo interface {
	Depth() int
	Capacity() int
	Status() string
	PendingCount() int64
}

// Adapter holds service dependencies for MCP tool handlers.
type Adapter struct {
	queries       *sqlc.Queries
	db            *sql.DB
	embedder      embed.Embedder
	searchService *search.SearchService
	embedQueue    EmbedQueueInfo
	embedCfg      config.EmbeddingConfig
	searchCfg     config.SearchConfig
	pool          PoolChecker
	logger        zerolog.Logger
}

// NewAdapter creates an Adapter with all service dependencies.
func NewAdapter(
	queries *sqlc.Queries,
	db *sql.DB,
	embedder embed.Embedder,
	searchService *search.SearchService,
	embedQueue EmbedQueueInfo,
	embedCfg config.EmbeddingConfig,
	searchCfg config.SearchConfig,
	pool PoolChecker,
	logger zerolog.Logger,
) *Adapter {
	return &Adapter{
		queries:       queries,
		db:            db,
		embedder:      embedder,
		searchService: searchService,
		embedQueue:    embedQueue,
		embedCfg:      embedCfg,
		searchCfg:     searchCfg,
		pool:          pool,
		logger:        logger,
	}
}
