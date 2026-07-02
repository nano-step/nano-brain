package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

var validCollectionName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

type CollectionQuerier interface {
	UpsertCollection(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error)
	ListCollections(ctx context.Context, workspaceHash string) ([]sqlc.Collection, error)
	ListCollectionsWithDocCount(ctx context.Context, workspaceHash string) ([]sqlc.ListCollectionsWithDocCountRow, error)
	GetCollectionByName(ctx context.Context, arg sqlc.GetCollectionByNameParams) (sqlc.Collection, error)
	RenameCollection(ctx context.Context, arg sqlc.RenameCollectionParams) (sqlc.Collection, error)
	DeleteCollection(ctx context.Context, arg sqlc.DeleteCollectionParams) error
	CountDocumentsByCollection(ctx context.Context, arg sqlc.CountDocumentsByCollectionParams) (int64, error)
	UpdateDocumentsCollection(ctx context.Context, arg sqlc.UpdateDocumentsCollectionParams) error
}

type AddCollectionRequest struct {
	Workspace         string   `json:"workspace"`
	Name              string   `json:"name"`
	Path              string   `json:"path"`
	GlobPattern       string   `json:"glob_pattern"`
	ExcludePatterns   []string `json:"exclude_patterns"`
	AllowedExtensions []string `json:"allowed_extensions"`
}

type RenameCollectionRequest struct {
	Workspace string `json:"workspace"`
	NewName   string `json:"new_name"`
}

type CollectionResponse struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Path              string   `json:"path"`
	GlobPattern       string   `json:"glob_pattern"`
	UpdateMode        string   `json:"update_mode"`
	ExcludePatterns   []string `json:"exclude_patterns"`
	AllowedExtensions []string `json:"allowed_extensions"`
	DocumentCount     int64    `json:"document_count"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

func toCollectionResponse(col sqlc.Collection, docCount int64) CollectionResponse {
	return CollectionResponse{
		ID:                col.ID.String(),
		Name:              col.Name,
		Path:              col.Path,
		GlobPattern:       col.GlobPattern,
		UpdateMode:        col.UpdateMode,
		ExcludePatterns:   col.ExcludePatterns,
		AllowedExtensions: col.AllowedExtensions,
		DocumentCount:     docCount,
		CreatedAt:         col.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:         col.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// AddCollection godoc
// @Summary      Add a collection to a workspace
// @Description  Registers a new file-watched collection under the workspace
// @Tags         collections
// @Accept       json
// @Produce      json
// @Param        request body AddCollectionRequest true "Collection to add"
// @Success      201 {object} CollectionResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/collections [post]
func AddCollection(q CollectionQuerier, fw *watcher.Watcher, watcherCfg config.WatcherConfig, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req AddCollectionRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		if req.Name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "name is required")
		}
		if !validCollectionName.MatchString(req.Name) {
			return echo.NewHTTPError(http.StatusBadRequest, "name must be 1-128 characters: letters, digits, underscores, hyphens only")
		}
		if req.Path == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "path is required")
		}

		cleanPath := filepath.Clean(req.Path)
		if !filepath.IsAbs(cleanPath) {
			return echo.NewHTTPError(http.StatusBadRequest, "path must be absolute")
		}

		if strings.Contains(cleanPath, "..") {
			return echo.NewHTTPError(http.StatusBadRequest, "path must not contain '..'")
		}

		info, err := os.Stat(cleanPath)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "path does not exist on filesystem")
		}
		if !info.IsDir() {
			return echo.NewHTTPError(http.StatusBadRequest, "path must be a directory")
		}
		req.Path = cleanPath

		globPattern := req.GlobPattern
		if globPattern == "" {
			globPattern = "**/*"
		}

		col, err := q.UpsertCollection(c.Request().Context(), sqlc.UpsertCollectionParams{
			WorkspaceHash: workspace,
			Name:          req.Name,
			Path:          req.Path,
			GlobPattern:   globPattern,
			UpdateMode:    "auto",
		})
		if err != nil {
			logger.Error().Err(err).Str("name", req.Name).Msg("upsert collection failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to add collection")
		}

		if fw != nil {
			cfgExclude, cfgExtensions := watcherCfg.ResolveFilterForPath(col.Path)
			excludePatterns := append(cfgExclude, col.ExcludePatterns...)
			allowedExtensions := col.AllowedExtensions
			if len(allowedExtensions) == 0 {
				allowedExtensions = cfgExtensions
			}
			if err := fw.WatchWithFilter(col.Name, col.Path, col.WorkspaceHash, col.GlobPattern, excludePatterns, allowedExtensions); err != nil {
				logger.Warn().Err(err).Str("name", col.Name).Msg("failed to attach watcher")
			}
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("name", col.Name).
			Str("workspace", workspace).
			Msg("collection added")

		return c.JSON(http.StatusCreated, toCollectionResponse(col, 0))
	}
}

// ListCollectionsHandler godoc
// @Summary      List collections in a workspace
// @Description  Returns all collections for the workspace with document counts
// @Tags         collections
// @Produce      json
// @Success      200 {array} CollectionResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/collections [get]
func ListCollectionsHandler(q CollectionQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.QueryParam("workspace")
		if workspace == "" {
			workspace, _ = c.Get("workspace").(string)
		}
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		cols, err := q.ListCollectionsWithDocCount(c.Request().Context(), workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("list collections failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list collections")
		}

		items := make([]CollectionResponse, 0, len(cols))
		for _, col := range cols {
			items = append(items, CollectionResponse{
				ID:                col.ID.String(),
				Name:              col.Name,
				Path:              col.Path,
				GlobPattern:       col.GlobPattern,
				UpdateMode:        col.UpdateMode,
				ExcludePatterns:   col.ExcludePatterns,
				AllowedExtensions: col.AllowedExtensions,
				DocumentCount:     col.DocumentCount,
				CreatedAt:         col.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				UpdatedAt:         col.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}

		return c.JSON(http.StatusOK, items)
	}
}

// RenameCollectionHandler godoc
// @Summary      Rename a collection
// @Description  Renames a collection and re-points its documents to the new name
// @Tags         collections
// @Accept       json
// @Produce      json
// @Param        name path string true "Current collection name"
// @Param        request body RenameCollectionRequest true "New collection name"
// @Success      200 {object} CollectionResponse
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/collections/{name} [put]
func RenameCollectionHandler(q CollectionQuerier, fw *watcher.Watcher, watcherCfg config.WatcherConfig, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("name")
		if name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "collection name is required")
		}

		var req RenameCollectionRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		if req.NewName == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "new_name is required")
		}
	if !validCollectionName.MatchString(req.NewName) {
		return echo.NewHTTPError(http.StatusBadRequest, "new_name must be 1-128 characters: letters, digits, underscores, hyphens only")
	}

	if err := q.UpdateDocumentsCollection(c.Request().Context(), sqlc.UpdateDocumentsCollectionParams{
		Collection:    name,
		Collection_2:  req.NewName,
		WorkspaceHash: workspace,
	}); err != nil {
		logger.Error().Err(err).Str("old", name).Str("new", req.NewName).Msg("update documents collection failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update documents for rename")
	}

	col, err := q.RenameCollection(c.Request().Context(), sqlc.RenameCollectionParams{
		Name:          name,
		Name_2:        req.NewName,
		WorkspaceHash: workspace,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "collection not found")
		}
		logger.Error().Err(err).Str("old", name).Str("new", req.NewName).Msg("rename collection failed")
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to rename collection")
	}

		if fw != nil {
			cfgExclude, cfgExtensions := watcherCfg.ResolveFilterForPath(col.Path)
			excludePatterns := append(cfgExclude, col.ExcludePatterns...)
			allowedExtensions := col.AllowedExtensions
			if len(allowedExtensions) == 0 {
				allowedExtensions = cfgExtensions
			}
			if err := fw.WatchWithFilter(col.Name, col.Path, col.WorkspaceHash, col.GlobPattern, excludePatterns, allowedExtensions); err != nil {
				logger.Warn().Err(err).Str("name", col.Name).Msg("failed to update watcher after rename")
			}
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("from", name).
			Str("to", req.NewName).
			Str("workspace", workspace).
			Msg("collection renamed")

		return c.JSON(http.StatusOK, toCollectionResponse(col, 0))
	}
}

// RemoveCollection godoc
// @Summary      Remove a collection
// @Description  Deletes a collection and detaches its file watcher
// @Tags         collections
// @Produce      json
// @Param        name path string true "Collection name"
// @Success      204 "No Content"
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/collections/{name} [delete]
func RemoveCollection(q CollectionQuerier, fw *watcher.Watcher, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("name")
		if name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "collection name is required")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		col, err := q.GetCollectionByName(c.Request().Context(), sqlc.GetCollectionByNameParams{
			Name:          name,
			WorkspaceHash: workspace,
		})
		if err != nil {
			logger.Error().Err(err).Str("name", name).Msg("get collection failed")
			return echo.NewHTTPError(http.StatusNotFound, "collection not found")
		}

		if err := q.DeleteCollection(c.Request().Context(), sqlc.DeleteCollectionParams{
			Name:          name,
			WorkspaceHash: workspace,
		}); err != nil {
			logger.Error().Err(err).Str("name", name).Msg("delete collection failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove collection")
		}

		if fw != nil {
			if err := fw.Unwatch(col.Path); err != nil {
				logger.Warn().Err(err).Str("path", col.Path).Msg("failed to detach watcher")
			}
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("name", name).
			Str("workspace", workspace).
			Msg("collection removed")

		return c.NoContent(http.StatusNoContent)
	}
}
