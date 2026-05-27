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
	"github.com/nano-brain/nano-brain/internal/telemetry"
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
	recorder       *telemetry.Recorder
	cleanupCancel  context.CancelFunc
	harvestMu      sync.RWMutex
	harvestRunner  handlers.HarvestRunner
	summarizeMu    sync.RWMutex
	summarizer     handlers.SummarizeSummarizer
	configMu       sync.RWMutex
	fullCfg        *config.Config
	configPath     string
	logger         zerolog.Logger
	cfg            config.ServerConfig
	embedCfg       config.EmbeddingConfig
	searchCfg      config.SearchConfig
	harvesterCfg   config.HarvesterConfig
	telemetryCfg   config.TelemetryConfig
	intervalsCfg   config.IntervalsConfig
	version          string
	startTime        time.Time
	migrationVersion int64
}

func New(fullCfg *config.Config, configPath string, pool PoolChecker, db *sql.DB, queries *sqlc.Queries, fw *watcher.Watcher, eq *embed.Queue, embedder embed.Embedder, logger zerolog.Logger, version string, migrationVersion int64) *Server {
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

	var rec *telemetry.Recorder
	if queries != nil {
		rec = telemetry.NewRecorder(queries, logger)
	}

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
		recorder:       rec,
		fullCfg:        fullCfg,
		configPath:     configPath,
		logger:         logger,
		cfg:            fullCfg.Server,
		embedCfg:       fullCfg.Embedding,
		searchCfg:      fullCfg.Search,
		harvesterCfg:   fullCfg.Harvester,
		telemetryCfg:   fullCfg.Telemetry,
		intervalsCfg:   fullCfg.Intervals,
		version:          version,
		startTime:        time.Now(),
		migrationVersion: migrationVersion,
	}

	registerMiddleware(s)
	registerRoutes(s)

	if queries != nil {
		cleanupCtx, cancel := context.WithCancel(context.Background())
		s.cleanupCancel = cancel
		go s.runTelemetryCleanup(cleanupCtx)
	}

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
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
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

func (s *Server) SetSummarizer(sum handlers.SummarizeSummarizer) {
	s.summarizeMu.Lock()
	defer s.summarizeMu.Unlock()
	s.summarizer = sum
}

func (s *Server) getSummarizer() handlers.SummarizeSummarizer {
	s.summarizeMu.RLock()
	defer s.summarizeMu.RUnlock()
	return s.summarizer
}

func (s *Server) getHealthCfg() (config.HarvesterConfig, config.IntervalsConfig) {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.harvesterCfg, s.intervalsCfg
}

func (s *Server) currentConfig() *config.Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	cp := *s.fullCfg
	return &cp
}

func (s *Server) runTelemetryCleanup(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.configMu.RLock()
			days := s.telemetryCfg.RetentionDays
			s.configMu.RUnlock()

			result, err := s.queries.CleanupTelemetryLogs(ctx, int32(days))
			if err != nil {
				s.logger.Warn().Err(err).Msg("telemetry cleanup failed")
				continue
			}
			if n, _ := result.RowsAffected(); n > 0 {
				s.logger.Info().Int64("deleted", n).Int("retention_days", days).Msg("telemetry logs cleaned up")
			}
		}
	}
}

func (s *Server) applyReloadedConfig(newCfg *config.Config, _ *config.ReloadResult) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	s.fullCfg = newCfg
	s.searchCfg = newCfg.Search

	if s.searchService != nil {
		s.searchService.UpdateConfig(newCfg.Search)
	}

	level, err := zerolog.ParseLevel(newCfg.Logging.Level)
	if err == nil {
		zerolog.SetGlobalLevel(level)
	}
}
