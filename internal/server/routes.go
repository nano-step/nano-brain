package server

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/eventbus"
	"github.com/nano-brain/nano-brain/internal/mcp"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/server/middleware"
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
	h := handlers.NewHealth(s.pool, s.logger, s.version, s.startTime, queueInfo, s.getHealthCfg, counter, s.embedCfg, s.migrationVersion)
	h.SetHarvestStatus(s.harvestStatus)
	s.healthHandler = h

	s.echo.GET("/health", h.Health)
	s.echo.GET("/api/status", h.Status)
	s.echo.GET("/api/version", h.Version)
	s.echo.GET("/api/openapi.json", handlers.OpenAPISpec())

	api := s.echo.Group("/api/v1", contentTypeMiddleware())
	api.POST("/init", handlers.InitWorkspace(s.queries, s.db, s.watcher, s.currentConfig().Watcher, s.logger))
	api.GET("/workspaces", handlers.ListWorkspaces(s.queries, s.logger))
	api.POST("/workspaces/resolve", handlers.ResolveWorkspace(s.queries, s.logger))
	api.DELETE("/workspaces/:hash", handlers.RemoveWorkspace(s.queries, s.db, s.logger))
	api.POST("/reset-workspace", handlers.ResetWorkspace(s.queries, s.db, s.logger))

	api.GET("/config", handlers.GetConfig(s.configPath, s.currentConfig, s.logger))
	api.POST("/config", handlers.PatchConfig(s.configPath, s.currentConfig, func() {
		newCfg, err := config.Load(s.configPath)
		if err != nil {
			s.logger.Warn().Err(err).Msg("config reload after patch failed")
			return
		}
		s.applyReloadedConfig(newCfg, nil)
	}, s.logger))

	api.GET("/doctor", handlers.Doctor(handlers.DoctorDeps{
		ConfigPath: s.configPath,
		LoadConfig: func() (*config.Config, error) { return config.Load(s.configPath) },
	}, s.logger))

	var enqueuer handlers.ChunkEnqueuer
	if s.embedQueue != nil {
		enqueuer = s.embedQueue
	}

	boundAddr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	csrfMW := middleware.CSRF(boundAddr)

	data := api.Group("", workspaceMiddleware(s.db))

	if s.eventBus != nil {
		data.GET("/events", handlers.EventsHandler(s.eventBus, s.logger))
	}

	write := data.Group("", workspaceRegisteredMiddleware(s.db), csrfMW)
	write.POST("/write", handlers.WriteDocument(s.queries, s.db, enqueuer, s.logger, defaultMaxFileSize, s.linkResolver, s.linkExtractor))
	write.POST("/embed", handlers.TriggerEmbed(s.queries, s.embedder, s.embedCfg.Provider, s.embedCfg.Model, s.embedCfg.MaxChars, s.logger))
	var reindexPub eventbus.Publisher
	if s.eventBus != nil {
		reindexPub = s.eventBus
	}
	write.POST("/reindex", handlers.TriggerReindex(s.queries, s.watcher, s.embedQueue, reindexPub, s.logger))
	write.POST("/reindex-cfg", handlers.ReindexCFG(s.queries, s.graphRegistry, s.logger))
	write.POST("/update", handlers.TriggerUpdate(s.watcher, s.logger))
	write.POST("/summarize", handlers.TriggerSummarize(s.getSummarizer, s.queries, s.logger))
	write.POST("/code/summarize", handlers.TriggerCodeSummarize(s.getCodeSummarizer, s.currentConfig().CodeSummarization, s.logger))
	write.GET("/code/summarize/status", handlers.GetCodeSummarizeStatus(s.queries, s.logger))
	write.GET("/code/summarize/failures", handlers.GetCodeSummarizeFailures(s.queries, s.logger))
	write.POST("/code/summarize/retry", handlers.RetryCodeSummarize(s.getCodeSummarizer, s.queries, s.currentConfig().CodeSummarization, s.logger))
	write.POST("/code/summarize/retry-all", handlers.RetryAllCodeSummarize(s.getCodeSummarizer, s.queries, s.currentConfig().CodeSummarization, s.logger))

	data.POST("/collections", handlers.AddCollection(s.queries, s.watcher, s.currentConfig().Watcher, s.logger))
	data.GET("/collections", handlers.ListCollectionsHandler(s.queries, s.logger))
	data.PUT("/collections/:name", handlers.RenameCollectionHandler(s.queries, s.watcher, s.currentConfig().Watcher, s.logger))
	data.DELETE("/collections/:name", handlers.RemoveCollection(s.queries, s.watcher, s.logger))

	data.GET("/tags", handlers.ListTags(s.queries, s.logger))
	data.GET("/documents", handlers.ListDocuments(s.queries, s.logger))
	data.DELETE("/documents/:id", handlers.DeleteDocument(s.queries, s.logger))
	data.POST("/get", handlers.GetDocument(s.queries, s.logger))
	data.POST("/multi-get", handlers.MultiGet(s.queries, s.logger))
	data.GET("/symbols", handlers.ListSymbols(s.queries, s.logger))
	data.POST("/graph/query", handlers.GraphQuery(s.queries, s.logger))
	data.POST("/graph/overview", handlers.GraphOverview(s.queries, s.logger))
	data.POST("/graph/impact", handlers.GraphImpact(s.queries, s.logger))
	data.POST("/graph/trace", handlers.GraphTrace(s.queries, s.logger))
	data.POST("/graph/flow", handlers.GraphFlow(s.queries, s.currentConfig().Flow, s.logger))
	data.POST("/graph/flowchart", handlers.GraphFlowchart(s.queries, s.currentConfig().Flow, s.logger))
	data.GET("/graph/flow/endpoints", handlers.ListFlowEndpoints(s.queries, s.logger))
	write.POST("/flow/materialize", handlers.FlowMaterialize(s.getFlowMaterializer, s.currentConfig().Flow, s.logger))

	data.POST("/vsearch", handlers.VectorSearch(s.queries, s.embedder, s.logger, s.recorder))
	data.POST("/search", handlers.BM25Search(s.queries, s.logger, s.recorder))

	if s.searchService != nil {
		data.POST("/query", handlers.Query(s.searchService, s.logger, s.recorder))
	}

	statsH := handlers.NewStatsHandler(s.queries, s.logger, s.version, s.startTime, s.embedCfg, s.migrationVersion, s.getHealthCfg, s.currentConfig().Watcher, s.watcher)
	statsH.SetHarvestStatus(s.harvestStatus)
	s.statsHandler = statsH
	data.GET("/stats", statsH.Handle)
	write.POST("/graph/pagerank/compute", handlers.GraphPageRankCompute(s.queries, s.logger))
	write.POST("/graph/neighborhood", handlers.GraphNeighborhood(s.queries, s.logger))
	data.GET("/links/:doc_id/backlinks", handlers.Backlinks(s.queries, s.logger))
	if s.concreteLinkRes != nil {
		data.GET("/links/resolve", handlers.ResolveLink(s.concreteLinkRes, s.logger))
	}

	wakeUp := handlers.WakeUpHandler(s.queries, s.logger)
	api.GET("/wake-up", wakeUp)
	data.POST("/wake-up", wakeUp)

	api.GET("/sessions/by-ticket", handlers.TicketHandler(s.queries, s.logger))

	s.echo.POST("/api/harvest", handlers.TriggerHarvest(s.getHarvestRunner))
	s.echo.POST("/api/reload-config", handlers.ReloadConfig(s.configPath, s.currentConfig, s.applyReloadedConfig, s.logger))

	sseHandler := mcp.NewSSEHandler(s.mcpServer)
	streamableHandler := mcp.NewStreamableHTTPHandler(s.mcpServer)

	s.echo.GET("/sse", echo.WrapHandler(sseHandler))
	s.echo.POST("/sse", echo.WrapHandler(sseHandler))

	// Wrap streamableHandler with the default-workspace middleware BEFORE
	// echo.WrapHandler, not as an echo.MiddlewareFunc — the SDK reads
	// req.Context() directly, and Echo's c.Set values never reach it.
	wrappedStreamable := mcp.WrapStreamableHandler(streamableHandler)
	s.echo.GET("/mcp", echo.WrapHandler(wrappedStreamable))
	s.echo.POST("/mcp", echo.WrapHandler(wrappedStreamable))
	s.echo.DELETE("/mcp", echo.WrapHandler(wrappedStreamable))

	s.echo.GET("/ui", func(c echo.Context) error {
		return c.HTML(200, `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>nano-brain dashboard</title></head>
<body style="font-family:system-ui;max-width:480px;margin:4rem auto;text-align:center">
<h1>nano-brain dashboard has moved</h1>
<p>The built-in UI has been replaced by a standalone dashboard.</p>
<pre style="text-align:left;background:#f5f5f5;padding:1rem;border-radius:6px">npx @nano-step/nano-brain-dashboard</pre>
<p><a href="https://github.com/nano-step/nano-brain-dashboard">Documentation &rarr;</a></p>
</body>
</html>`)
	})
}

const defaultMaxFileSize int64 = 307200
