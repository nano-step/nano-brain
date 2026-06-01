package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type WorkspaceQuerier interface {
	UpsertWorkspace(ctx context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error)
	UpsertCollection(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error)
	ListWorkspacesWithStats(ctx context.Context) ([]sqlc.ListWorkspacesWithStatsRow, error)
}

type initRequest struct {
	RootPath string `json:"root_path"`
}

type initResponse struct {
	WorkspaceHash string `json:"workspace_hash"`
	RootPath      string `json:"root_path"`
	AgentsSnippet string `json:"agents_snippet"`
}

// workspaceItem is the JSON shape of a single workspace returned by
// GET /api/v1/workspaces. Field names match web/src/api/types.ts Workspace
// interface. Renaming these JSON tags is a breaking API change — see
// openspec/specs/workspaces-api-contract for the canonical contract.
type workspaceItem struct {
	Hash                string     `json:"hash"`
	RootPath            string     `json:"root_path"`
	Name                string     `json:"name"`
	DocumentCount       int64      `json:"doc_count"`
	ChunkCount          int64      `json:"chunk_count"`
	LastDocumentUpdated *time.Time `json:"last_document_updated"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// listWorkspacesResponse wraps the workspaces array. The wrapper enables
// future extension (pagination, totals) without breaking clients.
type listWorkspacesResponse struct {
	Workspaces []workspaceItem `json:"workspaces"`
}

func initWorkspace(ctx context.Context, q WorkspaceQuerier, hash, name, absPath string) (sqlc.Workspace, error) {
	ws, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: hash,
		Name: name,
		Path: absPath,
	})
	if err != nil {
		return sqlc.Workspace{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return sqlc.Workspace{}, fmt.Errorf("failed to get home directory: %w", err)
	}
	memoryPath := filepath.Join(home, ".nano-brain", "memory")
	sessionsPath := filepath.Join(home, ".nano-brain", "sessions")

	if _, err := q.UpsertCollection(ctx, sqlc.UpsertCollectionParams{
		WorkspaceHash: ws.Hash,
		Name:          "memory",
		Path:          memoryPath,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
	}); err != nil {
		return sqlc.Workspace{}, err
	}

	if _, err := q.UpsertCollection(ctx, sqlc.UpsertCollectionParams{
		WorkspaceHash: ws.Hash,
		Name:          "sessions",
		Path:          sessionsPath,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
	}); err != nil {
		return sqlc.Workspace{}, err
	}

	if _, err := q.UpsertCollection(ctx, sqlc.UpsertCollectionParams{
		WorkspaceHash: ws.Hash,
		Name:          "code",
		Path:          absPath,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
	}); err != nil {
		return sqlc.Workspace{}, err
	}

	return ws, nil
}

// InitWorkspace handles POST /api/v1/init. When db is non-nil, all three DB
// operations are wrapped in a single transaction. Pass nil db in tests that
// use a mock querier.
func InitWorkspace(q WorkspaceQuerier, db *sql.DB, fw *watcher.Watcher, watcherCfg config.WatcherConfig, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req initRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.RootPath == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "root_path is required")
		}

		absPath, err := filepath.Abs(req.RootPath)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid root_path")
		}

		hash, err := storage.WorkspaceHash(absPath)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid root_path")
		}
		name := filepath.Base(absPath)

		var ws sqlc.Workspace
		if db != nil {
			tx, err := db.BeginTx(c.Request().Context(), nil)
			if err != nil {
				logger.Error().Err(err).Msg("begin transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
			ws, err = initWorkspace(c.Request().Context(), sqlc.New(tx), hash, name, absPath)
			if err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Str("hash", hash).Msg("init workspace failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
			if err := tx.Commit(); err != nil {
				logger.Error().Err(err).Msg("commit transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
		} else {
			ws, err = initWorkspace(c.Request().Context(), q, hash, name, absPath)
			if err != nil {
				logger.Error().Err(err).Str("hash", hash).Msg("init workspace failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
		}

		if fw != nil {
			home, err := os.UserHomeDir()
			if err != nil {
				logger.Warn().Err(err).Msg("failed to get home directory for watcher paths")
				home = "~"
			}
			type colSpec struct{ name, path, glob string }
			cols := []colSpec{
				{"memory", filepath.Join(home, ".nano-brain", "memory"), "**/*"},
				{"sessions", filepath.Join(home, ".nano-brain", "sessions"), "**/*"},
				{"code", absPath, "**/*"},
			}
			cfgExclude, cfgExtensions := watcherCfg.ResolveFilterForPath(absPath)
			for _, col := range cols {
				if err := fw.WatchWithFilter(col.name, col.path, hash, col.glob, cfgExclude, cfgExtensions); err != nil {
					logger.Warn().Err(err).Str("collection", col.name).Msg("failed to attach watcher after init")
				}
			}
		}

		snippet := "## nano-brain Access\n\nnano-brain workspace: " + ws.Hash + "\n" +
			"nano-brain is accessed via CLI: `npx nano-brain <command>`."

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace_hash", ws.Hash).
			Str("root_path", ws.Path).
			Msg("workspace registered")

		return c.JSON(http.StatusOK, initResponse{
			WorkspaceHash: ws.Hash,
			RootPath:      ws.Path,
			AgentsSnippet: snippet,
		})
	}
}

func ListWorkspaces(q WorkspaceQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		rows, err := q.ListWorkspacesWithStats(c.Request().Context())
		if err != nil {
			logger.Error().Err(err).Msg("list workspaces failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workspaces")
		}

		items := make([]workspaceItem, 0, len(rows))
		for _, r := range rows {
			item := workspaceItem{
				Hash:          r.Hash,
				RootPath:      r.Path,
				Name:          r.Name,
				DocumentCount: r.DocumentCount,
				ChunkCount:    r.ChunkCount,
				CreatedAt:     r.CreatedAt,
				UpdatedAt:     r.UpdatedAt,
			}
			if t, ok := r.LastDocumentUpdated.(time.Time); ok {
				item.LastDocumentUpdated = &t
			}
			items = append(items, item)
		}

		return c.JSON(http.StatusOK, listWorkspacesResponse{Workspaces: items})
	}
}
