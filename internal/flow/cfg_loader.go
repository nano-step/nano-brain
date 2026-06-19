package flow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type FlowCFGs map[string]*graph.CFG

type CFGQuerier interface {
	ListAllEdgesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
	GetFunctionFlowchartByHandler(ctx context.Context, arg sqlc.GetFunctionFlowchartByHandlerParams) (sqlc.FunctionFlowchart, error)
}

func LoadFlowCFGs(ctx context.Context, q CFGQuerier, workspace, entry string) (FlowCFGs, error) {
	rawEdges, err := q.ListAllEdgesByWorkspace(ctx, workspace)
	if err != nil {
		return nil, err
	}

	handlerNames := make(map[string]string)
	for _, e := range rawEdges {
		if graph.EdgeKind(e.EdgeType) == graph.EdgeHTTP && e.SourceNode == entry {
			bare := lastDottedSegment(e.TargetNode)
			handlerNames[e.TargetNode] = bare
		}
	}

	if len(handlerNames) == 0 {
		return nil, nil
	}

	cfgs := make(FlowCFGs, len(handlerNames))
	for nodeID, bareName := range handlerNames {
		fc, err := q.GetFunctionFlowchartByHandler(ctx, sqlc.GetFunctionFlowchartByHandlerParams{
			WorkspaceHash: workspace,
			Entry:         bareName,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}

		var cfg graph.CFG
		if err := json.Unmarshal(fc.Cfg, &cfg); err != nil {
			continue
		}
		if cfg.Status == "parse_error" || cfg.Status == "unsupported" {
			continue
		}
		cfgs[nodeID] = &cfg
	}

	if len(cfgs) == 0 {
		return nil, nil
	}
	return cfgs, nil
}

func cfgNodeMap(cfg *graph.CFG) map[string]graph.CFGNode {
	m := make(map[string]graph.CFGNode, len(cfg.Nodes))
	for _, n := range cfg.Nodes {
		m[n.ID] = n
	}
	return m
}

func cfgAdj(cfg *graph.CFG) map[string][]graph.CFGEdge {
	adj := make(map[string][]graph.CFGEdge, len(cfg.Edges))
	for _, e := range cfg.Edges {
		adj[e.From] = append(adj[e.From], e)
	}
	return adj
}

func lastDottedSegment(s string) string {
	if idx := strings.LastIndexByte(s, '.'); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}
