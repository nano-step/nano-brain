package flow

import (
	"context"
	"strings"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type StitchQuerier interface {
	ListConsumerEntryNodesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
}

type consumerEntry struct {
	workspaceHash string
	sourceNode    string
}

func Stitch(ctx context.Context, publishEdges []graph.Edge, targetWorkspaces []string, querier StitchQuerier) []FlowEdge {
	if len(publishEdges) == 0 || len(targetWorkspaces) == 0 {
		return nil
	}

	consumers := make(map[string][]consumerEntry)
	for _, ws := range targetWorkspaces {
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
