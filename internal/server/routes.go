package server

import (
	"github.com/nano-brain/nano-brain/internal/server/handlers"
)

func registerRoutes(s *Server) {
	var queueInfo handlers.EmbedQueueInfo
	if s.embedQueue != nil {
		queueInfo = s.embedQueue
	}
	h := handlers.NewHealth(s.pool, s.logger, s.version, s.startTime, queueInfo)

	s.echo.GET("/health", h.Health)
	s.echo.GET("/api/status", h.Status)

	api := s.echo.Group("/api/v1", contentTypeMiddleware())
	api.POST("/init", handlers.InitWorkspace(s.queries, s.db, s.logger))
	api.GET("/workspaces", handlers.ListWorkspaces(s.queries, s.logger))

	data := api.Group("", workspaceMiddleware())
	data.POST("/write", handlers.WriteDocument(s.queries, s.db, s.embedQueue, s.logger, defaultMaxFileSize))
	data.POST("/embed", handlers.TriggerEmbed(s.queries, s.embedder, s.embedCfg.Provider, s.embedCfg.Model, s.logger))

	data.POST("/collections", handlers.AddCollection(s.queries, s.watcher, s.logger))
	data.GET("/collections", handlers.ListCollectionsHandler(s.queries, s.logger))
	data.PUT("/collections/:name", handlers.RenameCollectionHandler(s.queries, s.watcher, s.logger))
	data.DELETE("/collections/:name", handlers.RemoveCollection(s.queries, s.watcher, s.logger))

	data.POST("/vsearch", handlers.VectorSearch(s.queries, s.embedder, s.logger))
}

const defaultMaxFileSize int64 = 307200
