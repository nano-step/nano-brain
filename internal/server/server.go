// Package server handles HTTP API serving.
package server

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	internalmcp "github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

// PoolChecker is the consumer-side interface for database health checks.
type PoolChecker interface {
	Ping(ctx context.Context) error
}

type Server struct {
	echo           *echo.Echo
	pool           PoolChecker
	db             *sql.DB
	queries        *sqlc.Queries
	watcher        *watcher.Watcher
	embedQueue     *embed.Queue
	embedder       embed.Embedder
	searchService  *search.SearchService
	mcpServer      *mcpsdk.Server
	harvestMu      sync.RWMutex
	harvestRunner  handlers.HarvestRunner
	configMu       sync.RWMutex
	fullCfg        *config.Config
	configPath     string
	logger         zerolog.Logger
	cfg            config.ServerConfig
	embedCfg       config.EmbeddingConfig
	searchCfg      config.SearchConfig
	harvesterCfg   config.HarvesterConfig
	intervalsCfg   config.IntervalsConfig
	version        string
	startTime      time.Time
}

func New(fullCfg *config.Config, configPath string, pool PoolChecker, db *sql.DB, queries *sqlc.Queries, fw *watcher.Watcher, eq *embed.Queue, embedder embed.Embedder, logger zerolog.Logger, version string) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	var ss *search.SearchService
	if queries != nil {
		ss = search.NewSearchService(queries, embedder, fullCfg.Search, logger)
	}

	mcpServer := internalmcp.NewMCPServer(version)

	var eqInfo internalmcp.EmbedQueueInfo
	if eq != nil {
		eqInfo = eq
	}
	mcpAdapter := internalmcp.NewAdapter(queries, db, embedder, ss, eqInfo, fullCfg.Embedding, fullCfg.Search, pool, logger)
	internalmcp.RegisterTools(mcpServer, mcpAdapter)

	s := &Server{
		echo:           e,
		pool:           pool,
		db:             db,
		queries:        queries,
		watcher:        fw,
		embedQueue:     eq,
		embedder:       embedder,
		searchService:  ss,
		mcpServer:      mcpServer,
		fullCfg:        fullCfg,
		configPath:     configPath,
		logger:         logger,
		cfg:            fullCfg.Server,
		embedCfg:       fullCfg.Embedding,
		searchCfg:      fullCfg.Search,
		harvesterCfg:   fullCfg.Harvester,
		intervalsCfg:   fullCfg.Intervals,
		version:        version,
		startTime:      time.Now(),
	}

	registerMiddleware(s)
	registerRoutes(s)

	return s
}

// Start begins serving HTTP requests. Blocks until an error occurs.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.logger.Info().Str("addr", addr).Msg("HTTP server listening")
	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

// ServeHTTP implements http.Handler for testing.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}

// Echo returns the underlying Echo instance (for test route injection).
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

func (s *Server) SetHarvestRunner(r handlers.HarvestRunner) {
	s.harvestMu.Lock()
	s.harvestRunner = r
	s.harvestMu.Unlock()
}

func (s *Server) getHarvestRunner() handlers.HarvestRunner {
	s.harvestMu.RLock()
	defer s.harvestMu.RUnlock()
	return s.harvestRunner
}

func (s *Server) currentConfig() *config.Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	cp := *s.fullCfg
	return &cp
}

func (s *Server) applyReloadedConfig(newCfg *config.Config, result *config.ReloadResult) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	s.fullCfg = newCfg
	s.searchCfg = newCfg.Search
	s.embedCfg = newCfg.Embedding
	s.intervalsCfg = newCfg.Intervals
	s.harvesterCfg = newCfg.Harvester

	if s.searchService != nil {
		s.searchService.UpdateConfig(newCfg.Search)
	}

	level, err := zerolog.ParseLevel(newCfg.Logging.Level)
	if err == nil {
		zerolog.SetGlobalLevel(level)
	}
}
