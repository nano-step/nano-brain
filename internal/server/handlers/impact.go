package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/symbol"
	"github.com/rs/zerolog"
)

type ImpactQuerier interface {
	GetImpactors(ctx context.Context, arg sqlc.GetImpactorsParams) ([]sqlc.GetImpactorsRow, error)
	GetImpactorsByTargets(ctx context.Context, arg sqlc.GetImpactorsByTargetsParams) ([]sqlc.GetImpactorsByTargetsRow, error)
}

type impactRequest struct {
	Node     string `json:"node"`
	EdgeType string `json:"edge_type"`
	MaxDepth int    `json:"max_depth"`
}

type impactNode struct {
	Node     string `json:"node"`
	Depth    int    `json:"depth"`
	EdgeType string `json:"edge_type"`
}

type impactResponse struct {
	Node     string       `json:"node"`
	Impacted []impactNode `json:"impacted"`
}

// GraphImpact godoc
// @Summary      Impact analysis for a graph node
// @Description  Returns nodes that would be impacted (transitively depend on) a given node, up to max_depth
// @Tags         graph
// @Accept       json
// @Produce      json
// @Param        request body impactRequest true "Impact query"
// @Success      200 {object} impactResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/graph/impact [post]
func GraphImpact(q ImpactQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		var req impactRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Node == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "node is required")
		}
		if req.MaxDepth <= 0 || req.MaxDepth > 3 {
			req.MaxDepth = 1
		}

		ctx := c.Request().Context()
		impacted, err := collectImpact(ctx, q, workspace, req.Node, req.EdgeType, req.MaxDepth)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Str("node", req.Node).Msg("impact query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "impact query failed")
		}

		return c.JSON(http.StatusOK, impactResponse{
			Node:     req.Node,
			Impacted: impacted,
		})
	}
}

func collectImpact(ctx context.Context, q ImpactQuerier, workspace, node, edgeType string, maxDepth int) ([]impactNode, error) {
	seen := map[string]bool{node: true}
	var result []impactNode

	// G1: expand with the bare symbol suffix of qualified nodes so calls-edge targets
	// stored bare (e.g. "checkAccess") are also matched. See symbol.ExpandImpactFrontier.
	frontier := symbol.ExpandImpactFrontier([]string{node})
	queried := map[string]bool{}
	for depth := 1; depth <= maxDepth && len(frontier) > 0; depth++ {
		targets := make([]string, 0, len(frontier))
		for _, f := range frontier {
			if queried[f] {
				continue
			}
			queried[f] = true
			targets = append(targets, f)
		}
		if len(targets) == 0 {
			break
		}
		rows, err := q.GetImpactorsByTargets(ctx, sqlc.GetImpactorsByTargetsParams{
			WorkspaceHash: workspace,
			Column2:       targets,
			Column3:       edgeType,
		})
		if err != nil {
			return nil, err
		}
		var next []string
		for _, r := range rows {
			if seen[r.SourceNode] {
				continue
			}
			seen[r.SourceNode] = true
			result = append(result, impactNode{
				Node:     r.SourceNode,
				Depth:    depth,
				EdgeType: r.EdgeType,
			})
			next = append(next, r.SourceNode)
		}
		frontier = symbol.ExpandImpactFrontier(next)
	}
	return result, nil
}
