package server

import (
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
)

func registerRoutes(s *Server) {
	var queueInfo handlers.EmbedQueueInfo
	if s.embedQueue != nil {
		queueInfo = s.embedQueue
	}
	var counter handlers.WorkspaceCounter
	if s.queries != nil {
		counter = s.queries
	}
	h := handlers.NewHealth(s.pool, s.logger, s.version, s.startTime, queueInfo, s.getHealthCfg, counter, s.embedCfg)

	s.echo.GET("/health", h.Health)
	s.echo.GET("/api/status", h.Status)

	api := s.echo.Group("/api/v1", contentTypeMiddleware())
	api.POST("/init", handlers.InitWorkspace(s.queries, s.db, s.logger))
	api.GET("/workspaces", handlers.ListWorkspaces(s.queries, s.logger))
	api.POST("/reset-workspace", handlers.ResetWorkspace(s.queries, s.logger))

	var enqueuer handlers.ChunkEnqueuer
	if s.embedQueue != nil {
		enqueuer = s.embedQueue
	}

	data := api.Group("", workspaceMiddleware())
	data.POST("/write", handlers.WriteDocument(s.queries, s.db, enqueuer, s.logger, defaultMaxFileSize))
	data.POST("/embed", handlers.TriggerEmbed(s.queries, s.embedder, s.embedCfg.Provider, s.embedCfg.Model, s.logger))

	data.POST("/collections", handlers.AddCollection(s.queries, s.watcher, s.currentConfig().Watcher, s.logger))
	data.GET("/collections", handlers.ListCollectionsHandler(s.queries, s.logger))
	data.PUT("/collections/:name", handlers.RenameCollectionHandler(s.queries, s.watcher, s.currentConfig().Watcher, s.logger))
	data.DELETE("/collections/:name", handlers.RemoveCollection(s.queries, s.watcher, s.logger))

	data.GET("/tags", handlers.ListTags(s.queries, s.logger))
	data.GET("/symbols", handlers.ListSymbols(s.queries, s.logger))
	data.POST("/graph/query", handlers.GraphQuery(s.queries, s.logger))
	data.POST("/graph/impact", handlers.GraphImpact(s.queries, s.logger))
	data.POST("/graph/trace", handlers.GraphTrace(s.queries, s.logger))
	data.POST("/reindex", handlers.TriggerReindex(s.queries, s.watcher, s.embedQueue, s.logger))
	data.POST("/update", handlers.TriggerUpdate(s.logger))
	data.POST("/summarize", handlers.TriggerSummarize(s.getSummarizer, s.queries, s.logger))

	data.POST("/vsearch", handlers.VectorSearch(s.queries, s.embedder, s.logger, s.recorder))
	data.POST("/search", handlers.BM25Search(s.queries, s.logger, s.recorder))

	if s.searchService != nil {
		data.POST("/query", handlers.Query(s.searchService, s.logger, s.recorder))
	}

	wakeUp := handlers.WakeUpHandler(s.queries, s.logger)
	api.GET("/wake-up", wakeUp)
	data.POST("/wake-up", wakeUp)

	s.echo.POST("/api/harvest", handlers.TriggerHarvest(s.getHarvestRunner))
	s.echo.POST("/api/reload-config", handlers.ReloadConfig(s.configPath, s.currentConfig, s.applyReloadedConfig, s.logger))

	sseHandler := mcp.NewSSEHandler(s.mcpServer)
	streamableHandler := mcp.NewStreamableHTTPHandler(s.mcpServer)

	s.echo.GET("/sse", echo.WrapHandler(sseHandler))
	s.echo.POST("/sse", echo.WrapHandler(sseHandler))

	s.echo.GET("/mcp", echo.WrapHandler(streamableHandler))
	s.echo.POST("/mcp", echo.WrapHandler(streamableHandler))
	s.echo.DELETE("/mcp", echo.WrapHandler(streamableHandler))
}

const defaultMaxFileSize int64 = 307200
