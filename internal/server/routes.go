package server

import (
	"github.com/nano-brain/nano-brain/internal/server/handlers"
)

func registerRoutes(s *Server) {
	h := handlers.NewHealth(s.pool, s.logger, s.version, s.startTime)

	s.echo.GET("/health", h.Health)
	s.echo.GET("/api/status", h.Status)

	api := s.echo.Group("/api/v1")
	api.POST("/init", handlers.InitWorkspace(s.queries, s.logger))
	api.GET("/workspaces", handlers.ListWorkspaces(s.queries, s.logger))
}
