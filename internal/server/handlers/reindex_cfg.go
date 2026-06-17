package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type ReindexCFGQuerier interface {
	ListCollections(ctx context.Context, workspaceHash string) ([]sqlc.Collection, error)
	ListDocumentsByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.ListDocumentsByWorkspaceRow, error)
	DeleteFunctionFlowchartsByFile(ctx context.Context, arg sqlc.DeleteFunctionFlowchartsByFileParams) error
	UpsertFunctionFlowchart(ctx context.Context, arg sqlc.UpsertFunctionFlowchartParams) error
}

type reindexCFGRequest struct {
	Workspace string `json:"workspace"`
}

type reindexCFGResponse struct {
	Status        string `json:"status"`
	FilesProcessed int   `json:"files_processed"`
	CFGsExtracted int   `json:"cfgs_extracted"`
	DurationMs    int64  `json:"duration_ms"`
}

var jsTSExts = map[string]bool{
	".js":  true,
	".jsx": true,
	".ts":  true,
	".tsx": true,
}

func ReindexCFG(queries ReindexCFGQuerier, graphReg *graph.Registry, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		var req reindexCFGRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace := c.Get("workspace").(string)
		ctx := c.Request().Context()
		reqLog := LoggerFromCtx(c, logger)

		if graphReg == nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "graph registry not available")
		}

		collections, err := queries.ListCollections(ctx, workspace)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list collections: %v", err))
		}

		var codeRoot string
		for _, col := range collections {
			if col.Name == "code" {
				codeRoot = col.Path
				break
			}
		}

		docs, err := queries.ListDocumentsByWorkspace(ctx, workspace)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list documents: %v", err))
		}

		var filesProcessed, cfgsExtracted int
		for _, doc := range docs {
			if doc.Collection != "code" {
				continue
			}

			ext := strings.ToLower(filepath.Ext(doc.SourcePath))
			if !jsTSExts[ext] {
				continue
			}

			content, err := os.ReadFile(doc.SourcePath)
			if err != nil {
				reqLog.Warn().Err(err).Str("path", doc.SourcePath).Msg("reindex-cfg: read file failed, skipping")
				continue
			}

			relFile := doc.SourcePath
			if codeRoot != "" {
				if rel, rerr := filepath.Rel(codeRoot, doc.SourcePath); rerr == nil {
					relFile = filepath.ToSlash(rel)
				}
			}

			filesProcessed++

			cfgs, err := graphReg.ExtractCFGs(relFile, content)
			if err != nil {
				reqLog.Warn().Err(err).Str("file", relFile).Msg("reindex-cfg: extraction failed")
				continue
			}

			if err := queries.DeleteFunctionFlowchartsByFile(ctx, sqlc.DeleteFunctionFlowchartsByFileParams{
				WorkspaceHash: workspace,
				SourceFile:    relFile,
			}); err != nil {
				reqLog.Warn().Err(err).Str("file", relFile).Msg("reindex-cfg: delete old cfgs failed")
				continue
			}

			for _, cfg := range cfgs {
				cfgJSON, mErr := json.Marshal(cfg)
				if mErr != nil {
					reqLog.Warn().Err(mErr).Str("entry", cfg.Entry).Msg("reindex-cfg: marshal cfg failed, skipping")
					continue
				}
				if err := queries.UpsertFunctionFlowchart(ctx, sqlc.UpsertFunctionFlowchartParams{
					WorkspaceHash: workspace,
					Entry:         cfg.Entry,
					SourceFile:    relFile,
					StartLine:     int32(cfg.StartLine),
					EndLine:       int32(cfg.EndLine),
					Status:        cfg.Status,
					Cfg:           cfgJSON,
				}); err != nil {
					reqLog.Warn().Err(err).Str("entry", cfg.Entry).Msg("reindex-cfg: upsert cfg failed")
					continue
				}
				cfgsExtracted++
			}
		}

		reqLog.Info().
			Str("workspace", workspace).
			Int("files_processed", filesProcessed).
			Int("cfgs_extracted", cfgsExtracted).
			Dur("duration", time.Since(start)).
			Msg("reindex-cfg completed")

		return c.JSON(http.StatusOK, reindexCFGResponse{
			Status:        "completed",
			FilesProcessed: filesProcessed,
			CFGsExtracted: cfgsExtracted,
			DurationMs:    time.Since(start).Milliseconds(),
		})
	}
}
