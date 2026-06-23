package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type ReindexCFGQuerier interface {
	ListCollections(ctx context.Context, workspaceHash string) ([]sqlc.Collection, error)
	ListDocumentsByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.ListDocumentsByWorkspaceRow, error)
	DeleteFunctionFlowchartsByFile(ctx context.Context, arg sqlc.DeleteFunctionFlowchartsByFileParams) error
	DeleteAllFunctionFlowcharts(ctx context.Context, workspaceHash string) error
	UpsertFunctionFlowchart(ctx context.Context, arg sqlc.UpsertFunctionFlowchartParams) error
}

type reindexCFGRequest struct {
	Workspace string `json:"workspace"`
	Full     bool   `json:"full"`
	Wipe     bool   `json:"wipe"`
}

type reindexCFGResponse struct {
	Status        string `json:"status"`
	FilesProcessed int   `json:"files_processed"`
	CFGsExtracted int   `json:"cfgs_extracted"`
	DurationMs    int64  `json:"duration_ms"`
}

var cfgExts = map[string]bool{
	".js":  true,
	".jsx": true,
	".ts":  true,
	".tsx": true,
	".rb":  true,
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

		if codeRoot == "" {
			return echo.NewHTTPError(http.StatusInternalServerError, "no code collection found")
		}

		var filesProcessed, cfgsExtracted int

		if req.Wipe {
			if wipeErr := queries.DeleteAllFunctionFlowcharts(ctx, workspace); wipeErr != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("wipe flowcharts: %v", wipeErr))
			}
			reqLog.Info().Str("workspace", workspace).Msg("reindex-cfg: wiped all flowcharts")
		}

		if req.Full {
			reqLog.Info().Str("root", codeRoot).Msg("reindex-cfg: starting full filesystem walk")
			filesProcessed, cfgsExtracted, err = fullWalkAndExtract(ctx, workspace, codeRoot, graphReg, queries, reqLog)
		} else {
			docs, err := queries.ListDocumentsByWorkspace(ctx, workspace)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list documents: %v", err))
			}
			filesProcessed, cfgsExtracted, err = incrementalExtract(ctx, workspace, codeRoot, docs, graphReg, queries, reqLog)
		}

		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("reindex-cfg failed: %v", err))
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

func incrementalExtract(ctx context.Context, workspace, codeRoot string, docs []sqlc.ListDocumentsByWorkspaceRow, graphReg *graph.Registry, queries ReindexCFGQuerier, reqLog zerolog.Logger) (filesProcessed, cfgsExtracted int, err error) {
	homeDir, _ := os.UserHomeDir()
	globalIgnore, _, _ := watcher.LoadGlobalIgnore(homeDir)

	filter, ferr := watcher.NewFileFilter(codeRoot, nil, nil, globalIgnore)
	if ferr != nil {
		reqLog.Warn().Err(ferr).Str("root", codeRoot).Msg("reindex-cfg: .nano-brainignore load failed, continuing without local matcher")
	}

	for _, doc := range docs {
		if doc.Collection != "code" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(doc.SourcePath))
		if !cfgExts[ext] {
			continue
		}
		if filter != nil && filter.ShouldSkip(doc.SourcePath, false) {
			reqLog.Debug().Str("file", doc.SourcePath).Msg("reindex-cfg: skipped by filter")
			continue
		}
		filesProcessed++
		if err := processFile(ctx, workspace, doc.SourcePath, codeRoot, graphReg, queries); err != nil {
			reqLog.Warn().Err(err).Str("file", doc.SourcePath).Msg("reindex-cfg: failed")
		} else {
			cfgsExtracted++
		}
	}
	return
}

func fullWalkAndExtract(ctx context.Context, workspace, codeRoot string, graphReg *graph.Registry, queries ReindexCFGQuerier, reqLog zerolog.Logger) (filesProcessed, cfgsExtracted int, err error) {
	homeDir, _ := os.UserHomeDir()
	globalIgnore, _, _ := watcher.LoadGlobalIgnore(homeDir)

	filter, ferr := watcher.NewFileFilter(codeRoot, nil, nil, globalIgnore)
	if ferr != nil {
		reqLog.Warn().Err(ferr).Str("root", codeRoot).Msg("reindex-cfg: .nano-brainignore load failed, continuing without local matcher")
	}

	ignoreStack := &watcher.GitignoreStack{}
	reqLog.Info().Str("root", codeRoot).Msg("reindex-cfg: starting full filesystem walk")

	var lastLogTime time.Time
	lastLogTime = time.Now()
	var skipped int

	err = filepath.WalkDir(codeRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			reqLog.Warn().Err(walkErr).Str("path", path).Msg("walk error")
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		ignoreStack.PopAbove(path)

		if d.IsDir() {
			giPath := filepath.Join(path, ".gitignore")
			if info, statErr := os.Stat(giPath); statErr == nil && !info.IsDir() {
				if gi, giErr := gitignore.CompileIgnoreFile(giPath); giErr == nil {
					ignoreStack.Push(path, gi)
				}
			}
			localIgnorePath := filepath.Join(path, ".nano-brainignore")
			if info, statErr := os.Stat(localIgnorePath); statErr == nil && !info.IsDir() {
				if li, liErr := gitignore.CompileIgnoreFile(localIgnorePath); liErr == nil {
					ignoreStack.Push(path, li)
				}
			}
		}

		if filter.ShouldSkip(path, d.IsDir()) || ignoreStack.Matches(path) {
			skipped++
			if d.IsDir() {
				reqLog.Debug().Str("dir", path).Msg("reindex-cfg: skipping dir")
				return filepath.SkipDir
			}
			return nil
		}

		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if cfgExts[ext] {
				filesProcessed++
				reqLog.Debug().Str("file", path).Int("processed", filesProcessed).Msg("reindex-cfg: processing file")
				if err := processFile(ctx, workspace, path, codeRoot, graphReg, queries); err != nil {
					reqLog.Warn().Err(err).Str("file", path).Msg("reindex-cfg: failed")
				} else {
					cfgsExtracted++
				}
			}
		}

		if time.Since(lastLogTime) > 10*time.Second {
			reqLog.Info().
				Str("current_path", path).
				Int("files_processed", filesProcessed).
				Int("cfgs_extracted", cfgsExtracted).
				Int("skipped", skipped).
				Msg("reindex-cfg: progress")
			lastLogTime = time.Now()
		}
		return nil
	})
	return
}

func processFile(ctx context.Context, workspace, absPath, codeRoot string, graphReg *graph.Registry, queries ReindexCFGQuerier) error {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	relFile := absPath
	if codeRoot != "" {
		if rel, rerr := filepath.Rel(codeRoot, absPath); rerr == nil {
			relFile = filepath.ToSlash(rel)
		}
	}

	cfgs, err := graphReg.ExtractCFGs(relFile, content)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	if err := queries.DeleteFunctionFlowchartsByFile(ctx, sqlc.DeleteFunctionFlowchartsByFileParams{
		WorkspaceHash: workspace,
		SourceFile:    relFile,
	}); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	for _, cfg := range cfgs {
		cfgJSON, mErr := json.Marshal(cfg)
		if mErr != nil {
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
			return fmt.Errorf("upsert %s: %w", cfg.Entry, err)
		}
	}
	return nil
}
