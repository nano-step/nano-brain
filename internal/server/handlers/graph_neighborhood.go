package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

const maxNeighborhoodNodes = 500

// NeighborhoodQuerier is the DB interface for graph neighborhood traversal.
type NeighborhoodQuerier interface {
	GetOutgoingEdges(ctx context.Context, arg sqlc.GetOutgoingEdgesParams) ([]sqlc.GraphEdge, error)
	GetIncomingEdges(ctx context.Context, arg sqlc.GetIncomingEdgesParams) ([]sqlc.GraphEdge, error)
	GetEdgesByNodes(ctx context.Context, arg sqlc.GetEdgesByNodesParams) ([]sqlc.GraphEdge, error)
	ListDocumentsByIDs(ctx context.Context, arg sqlc.ListDocumentsByIDsParams) ([]sqlc.ListDocumentsByIDsRow, error)
}

type neighborhoodRequest struct {
	Focus     string   `json:"focus"`
	Depth     int      `json:"depth"`
	Direction string   `json:"direction"`
	EdgeTypes []string `json:"edge_types"`
	Workspace string   `json:"workspace"`
	NodeKind  string   `json:"node_kind"`
}

type neighborhoodNode struct {
	ID         string    `json:"id"`
	Title      string    `json:"title,omitempty"`
	Collection string    `json:"collection,omitempty"`
	UpdatedAt  *time.Time `json:"updated_at,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
}

type neighborhoodEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	EdgeType string `json:"edge_type"`
}

type neighborhoodResponse struct {
	NodeKind      string             `json:"node_kind"`
	Nodes         []neighborhoodNode `json:"nodes"`
	Edges         []neighborhoodEdge `json:"edges"`
	Truncated     bool               `json:"truncated"`
	FrontierNodes []string           `json:"frontier_nodes,omitempty"`
}

// GraphNeighborhood handles POST /api/v1/graph/neighborhood with BFS traversal.
func GraphNeighborhood(q NeighborhoodQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req neighborhoodRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Focus == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "focus is required")
		}
		if req.Depth < 1 || req.Depth > 5 {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "depth must be between 1 and 5")
		}
		if req.Direction == "" {
			req.Direction = "both"
		}
		if req.NodeKind == "" {
			req.NodeKind = "symbol"
		}
		if req.NodeKind != "symbol" && req.NodeKind != "doc" {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "node_kind must be 'symbol' or 'doc'")
		}

		workspace := c.Get("workspace").(string)
		ctx := c.Request().Context()

		edgeTypes := req.EdgeTypes
		if req.NodeKind == "doc" {
			edgeTypes = []string{"references"}
		}

		visited := map[string]bool{}
		var allEdges []neighborhoodEdge
		frontier := []string{req.Focus}
		visited[req.Focus] = true
		truncated := false
		var frontierNodes []string

		for depth := 0; depth < req.Depth && len(frontier) > 0; depth++ {
			var nextFrontier []string
			for _, node := range frontier {
				if len(visited) >= maxNeighborhoodNodes {
					truncated = true
					frontierNodes = append(frontierNodes, node)
					continue
				}

				var edges []sqlc.GraphEdge
				var err error

				edgeFilter := ""
				if len(edgeTypes) == 1 {
					edgeFilter = edgeTypes[0]
				}

				switch req.Direction {
				case "out":
					edges, err = q.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
						WorkspaceHash: workspace,
						SourceNode:    node,
						Column3:       edgeFilter,
					})
				case "in":
					edges, err = q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
						WorkspaceHash: workspace,
						TargetNode:    node,
						Column3:       edgeFilter,
					})
				default:
					out, errOut := q.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
						WorkspaceHash: workspace,
						SourceNode:    node,
						Column3:       edgeFilter,
					})
					if errOut != nil {
						err = errOut
						break
					}
					in, errIn := q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
						WorkspaceHash: workspace,
						TargetNode:    node,
						Column3:       edgeFilter,
					})
					if errIn != nil {
						err = errIn
						break
					}
					edges = append(out, in...)
				}

				if err != nil {
					logger.Error().Err(err).Str("node", node).Msg("neighborhood BFS query failed")
					return echo.NewHTTPError(http.StatusInternalServerError, "graph query failed")
				}

				for _, e := range edges {
					if len(edgeTypes) > 1 && !containsStr(edgeTypes, e.EdgeType) {
						continue
					}
					allEdges = append(allEdges, neighborhoodEdge{
						Source: e.SourceNode, Target: e.TargetNode, EdgeType: e.EdgeType,
					})
					neighbor := e.TargetNode
					if neighbor == node {
						neighbor = e.SourceNode
					}
					if !visited[neighbor] {
						if len(visited) >= maxNeighborhoodNodes {
							truncated = true
							frontierNodes = append(frontierNodes, neighbor)
							continue
						}
						visited[neighbor] = true
						nextFrontier = append(nextFrontier, neighbor)
					}
				}
			}
			frontier = nextFrontier
		}

		if len(frontier) > 0 && truncated {
			frontierNodes = append(frontierNodes, frontier...)
		}

		nodes := make([]neighborhoodNode, 0, len(visited))
		for id := range visited {
			nodes = append(nodes, neighborhoodNode{ID: id})
		}

		if req.NodeKind == "doc" && len(nodes) > 0 {
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
					logger.Warn().Err(err).Msg("neighborhood doc enrichment failed")
				} else {
					docMap := map[string]sqlc.ListDocumentsByIDsRow{}
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

		if allEdges == nil {
			allEdges = []neighborhoodEdge{}
		}

		return c.JSON(http.StatusOK, neighborhoodResponse{
			NodeKind:      req.NodeKind,
			Nodes:         nodes,
			Edges:         allEdges,
			Truncated:     truncated,
			FrontierNodes: frontierNodes,
		})
	}
}

func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
