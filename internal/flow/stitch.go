package flow

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type StitchQuerier interface {
	ListConsumerEntryNodesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
}

type allEdgesQuerier interface {
	ListAllEdgesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
}

type consumerEntry struct {
	workspaceHash string
	sourceNode    string
}

func Stitch(ctx context.Context, publishEdges []graph.Edge, targetWorkspaces []string, querier StitchQuerier) []FlowEdge {
	if len(publishEdges) == 0 || len(targetWorkspaces) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(targetWorkspaces))
	unique := make([]string, 0, len(targetWorkspaces))
	for _, ws := range targetWorkspaces {
		if _, dup := seen[ws]; dup {
			continue
		}
		seen[ws] = struct{}{}
		unique = append(unique, ws)
	}

	consumers := make(map[string][]consumerEntry)
	emitters := make(map[string][]graph.Edge)
	for _, ws := range unique {
		edges, err := querier.ListConsumerEntryNodesByWorkspace(ctx, ws)
		if err != nil {
			continue
		}
		for _, e := range edges {
			topic := extractTopic(e.SourceNode)
			if topic != "" {
				wsID := ws
				if len(wsID) > 8 {
					wsID = wsID[:8]
				}
				consumers[topic] = append(consumers[topic], consumerEntry{
					workspaceHash: wsID,
					sourceNode:    e.SourceNode,
				})
				continue
			}
		}
		if aq, ok := querier.(allEdgesQuerier); ok {
			all, err := aq.ListAllEdgesByWorkspace(ctx, ws)
			if err == nil {
				for _, e := range all {
					var metadata map[string]any
					_ = json.Unmarshal(e.Metadata, &metadata)
					if topic, ok := metadata["topic"].(string); ok && topic != "" && metadata["event_role"] == "emit" {
						emitters[topic] = append(emitters[topic], graph.Edge{SourceNode: e.SourceNode, TargetNode: e.TargetNode, Kind: graph.EdgeKind(e.EdgeType), SourceFile: e.SourceFile})
					}
				}
			}
		}
	}

	var result []FlowEdge
	for _, pe := range publishEdges {
		topic, ok := pe.Metadata["topic"].(string)
		if !ok || topic == "" {
			continue
		}
		if strings.HasPrefix(topic, "<var:") {
			continue
		}

		if targets, ok := consumers[topic]; ok {
			for _, t := range targets {
				result = append(result, FlowEdge{
					From:                  pe.SourceNode,
					To:                    t.sourceNode,
					Kind:                  "cross_service",
					CrossServiceWorkspace: t.workspaceHash,
				})
				for _, emitter := range emitters[topic] {
					if emitter.SourceNode != t.sourceNode {
						continue
					}
					result = append(result, FlowEdge{From: t.sourceNode, To: emitter.TargetNode, Kind: "event_emit", CrossServiceWorkspace: t.workspaceHash})
				}
			}
		}
	}

	return result
}

func extractTopic(sourceNode string) string {
	for _, prefix := range []string{"CONSUME ", "ON "} {
		if after, ok := strings.CutPrefix(sourceNode, prefix); ok {
			return after
		}
	}
	return ""
}
