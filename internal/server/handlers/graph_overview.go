package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

const (
	defaultOverviewLimit = 50
	maxOverviewLimit     = 200
	overviewEdgeCap      = 400
)

type OverviewQuerier interface {
	ListTopGraphNodesByDegree(ctx context.Context, arg sqlc.ListTopGraphNodesByDegreeParams) ([]sqlc.ListTopGraphNodesByDegreeRow, error)
	CountDistinctGraphNodes(ctx context.Context, arg sqlc.CountDistinctGraphNodesParams) (int64, error)
	ListEdgesTouchingNodes(ctx context.Context, arg sqlc.ListEdgesTouchingNodesParams) ([]sqlc.GraphEdge, error)
	ListDocumentsByIDs(ctx context.Context, arg sqlc.ListDocumentsByIDsParams) ([]sqlc.ListDocumentsByIDsRow, error)
}

type overviewRequest struct {
	Workspace string   `json:"workspace"`
	Mode      string   `json:"mode"`
	Limit     int      `json:"limit"`
	EdgeTypes []string `json:"edge_types"`
}

// GraphOverview handles POST /api/v1/graph/overview — returns the top-N most-
// connected nodes for the workspace and all edges between them. Used by
// /ui/graph to auto-display a default subgraph without manual focus input.
// See openspec/specs/graph-overview-endpoint for the canonical contract.
//
// @Summary      Top-N most-connected graph nodes overview
// @Description  Returns the top-N most-connected nodes for the workspace and all edges between them
// @Tags         graph
// @Accept       json
// @Produce      json
// @Param        request body overviewRequest true "Overview query"
// @Success      200 {object} neighborhoodResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/graph/overview [post]
func GraphOverview(q OverviewQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req overviewRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			workspace = req.Workspace
		}
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		mode := req.Mode
		if mode == "" {
			mode = "code"
		}

		edgeTypes := req.EdgeTypes
		if len(edgeTypes) == 0 {
			switch mode {
			case "knowledge":
				edgeTypes = []string{"references"}
			default:
				edgeTypes = []string{"calls", "imports", "contains"}
			}
		}

		limit := req.Limit
		if limit <= 0 {
			limit = defaultOverviewLimit
		}
		if limit > maxOverviewLimit {
			limit = maxOverviewLimit
		}

		ctx := c.Request().Context()

		topRows, err := q.ListTopGraphNodesByDegree(ctx, sqlc.ListTopGraphNodesByDegreeParams{
			WorkspaceHash: workspace,
			Column2:       edgeTypes,
			Limit:         int32(limit),
		})
		if err != nil {
			logger.Error().Err(err).Msg("graph_overview: top nodes query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "graph query failed")
		}

		totalDistinct, err := q.CountDistinctGraphNodes(ctx, sqlc.CountDistinctGraphNodesParams{
			WorkspaceHash: workspace,
			Column2:       edgeTypes,
		})
		if err != nil {
			logger.Error().Err(err).Msg("graph_overview: count query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "graph query failed")
		}

		nodeIDs := make([]string, 0, len(topRows))
		for _, r := range topRows {
			nodeIDs = append(nodeIDs, r.Node)
		}

		var edgeRows []sqlc.GraphEdge
		if len(nodeIDs) > 0 {
			edgeRows, err = q.ListEdgesTouchingNodes(ctx, sqlc.ListEdgesTouchingNodesParams{
				WorkspaceHash: workspace,
				Column2:       edgeTypes,
				Column3:       nodeIDs,
				Limit:         int32(overviewEdgeCap),
			})
			if err != nil {
				logger.Error().Err(err).Msg("graph_overview: edges query failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "graph query failed")
			}
		}

		seen := make(map[string]struct{}, len(topRows)+len(edgeRows))
		nodes := make([]neighborhoodNode, 0, len(topRows)+len(edgeRows))
		for _, r := range topRows {
			seen[r.Node] = struct{}{}
			nodes = append(nodes, neighborhoodNode{ID: r.Node})
		}
		for _, e := range edgeRows {
			if _, ok := seen[e.SourceNode]; !ok {
				seen[e.SourceNode] = struct{}{}
				nodes = append(nodes, neighborhoodNode{ID: e.SourceNode})
			}
			if _, ok := seen[e.TargetNode]; !ok {
				seen[e.TargetNode] = struct{}{}
				nodes = append(nodes, neighborhoodNode{ID: e.TargetNode})
			}
		}

		if mode == "knowledge" && len(nodes) > 0 {
			var docIDs []uuid.UUID
			for _, n := range nodes {
				if uid, err := uuid.Parse(n.ID); err == nil {
					docIDs = append(docIDs, uid)
				}
			}
			if len(docIDs) > 0 {
				docRows, err := q.ListDocumentsByIDs(ctx, sqlc.ListDocumentsByIDsParams{
					WorkspaceHash: workspace,
					Column2:       docIDs,
				})
				if err != nil {
					logger.Warn().Err(err).Msg("graph_overview: doc enrichment failed")
				} else {
					docMap := make(map[string]sqlc.ListDocumentsByIDsRow, len(docRows))
					for _, d := range docRows {
						docMap[d.ID.String()] = d
					}
					for i, n := range nodes {
						if d, ok := docMap[n.ID]; ok {
							nodes[i].Title = d.Title
							nodes[i].Collection = d.Collection
							t := d.UpdatedAt
							nodes[i].UpdatedAt = &t
							nodes[i].Tags = d.Tags
						}
					}
				}
			}
		}

		edges := make([]neighborhoodEdge, 0, len(edgeRows))
		for _, e := range edgeRows {
			edges = append(edges, neighborhoodEdge{
				Source:   e.SourceNode,
				Target:   e.TargetNode,
				EdgeType: e.EdgeType,
			})
		}

		resp := neighborhoodResponse{
			NodeKind:  modeToNodeKind(mode),
			Nodes:     nodes,
			Edges:     edges,
			Truncated: totalDistinct > int64(limit),
		}

		return c.JSON(http.StatusOK, resp)
	}
}

func modeToNodeKind(mode string) string {
	if mode == "knowledge" {
		return "doc"
	}
	return "symbol"
}
