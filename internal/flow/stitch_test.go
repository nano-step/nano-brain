package flow

import (
	"context"
	"testing"

	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type mockStitchQuerier struct {
	entries map[string][]sqlc.GraphEdge
}

func (m *mockStitchQuerier) ListConsumerEntryNodesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error) {
	return m.entries[workspaceHash], nil
}

func stitchEdge(edges []FlowEdge, from, to, kind string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return true
		}
	}
	return false
}

func TestStitchMatchesStringLiteralTopics(t *testing.T) {
	publishEdges := []graph.Edge{
		{
			SourceNode: "handlers/trade.go::publishCreated",
			Kind:       graph.EdgeIntegration,
			Metadata:   map[string]any{"topic": "trade.created"},
		},
		{
			SourceNode: "handlers/payment.go::publishPaid",
			Kind:       graph.EdgeIntegration,
			Metadata:   map[string]any{"topic": "payment.completed"},
		},
	}

	mock := &mockStitchQuerier{
		entries: map[string][]sqlc.GraphEdge{
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {
				{SourceNode: "CONSUME trade.created", TargetNode: "HandleTradeCreated", EdgeType: "integration"},
			},
			"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb": {
				{SourceNode: "ON payment.completed", TargetNode: "HandlePaymentCompleted", EdgeType: "integration"},
			},
		},
	}

	result := Stitch(context.Background(), publishEdges, []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, mock)

	if !stitchEdge(result, "handlers/trade.go::publishCreated", "CONSUME trade.created", "cross_service") {
		t.Error("expected cross_service edge for trade.created")
	}
	if !stitchEdge(result, "handlers/payment.go::publishPaid", "ON payment.completed", "cross_service") {
		t.Error("expected cross_service edge for payment.completed")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 cross_service edges, got %d", len(result))
	}
}

func TestStitchSkipsVarPlaceholders(t *testing.T) {
	publishEdges := []graph.Edge{
		{
			SourceNode: "handlers/config.go::load",
			Kind:       graph.EdgeIntegration,
			Metadata:   map[string]any{"topic": "<var:event_topic>"},
		},
		{
			SourceNode: "handlers/trade.go::publishCreated",
			Kind:       graph.EdgeIntegration,
			Metadata:   map[string]any{"topic": "trade.created"},
		},
	}

	mock := &mockStitchQuerier{
		entries: map[string][]sqlc.GraphEdge{
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {
				{SourceNode: "CONSUME trade.created", TargetNode: "HandleTradeCreated", EdgeType: "integration"},
			},
		},
	}

	result := Stitch(context.Background(), publishEdges, []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, mock)

	if stitchEdge(result, "handlers/config.go::load", "CONSUME trade.created", "cross_service") {
		t.Error("<var:...> topic should not match")
	}
	if !stitchEdge(result, "handlers/trade.go::publishCreated", "CONSUME trade.created", "cross_service") {
		t.Error("expected cross_service edge for trade.created")
	}
	if len(result) != 1 {
		t.Errorf("expected 1 cross_service edge, got %d", len(result))
	}
}

func TestStitchEmptyOnNoMatch(t *testing.T) {
	publishEdges := []graph.Edge{
		{
			SourceNode: "handlers/trade.go::publishCreated",
			Kind:       graph.EdgeIntegration,
			Metadata:   map[string]any{"topic": "trade.created"},
		},
	}

	mock := &mockStitchQuerier{
		entries: map[string][]sqlc.GraphEdge{
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": {
				{SourceNode: "CONSUME order.placed", TargetNode: "HandleOrderPlaced", EdgeType: "integration"},
			},
		},
	}

	result := Stitch(context.Background(), publishEdges, []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, mock)

	if len(result) != 0 {
		t.Errorf("expected 0 cross_service edges on no match, got %d", len(result))
	}
}

func TestStitchEmptyEdgesOrWorkspaces(t *testing.T) {
	mock := &mockStitchQuerier{entries: map[string][]sqlc.GraphEdge{}}

	if r := Stitch(context.Background(), nil, []string{"ws1"}, mock); r != nil {
		t.Error("expected nil for nil publishEdges")
	}
	if r := Stitch(context.Background(), []graph.Edge{}, []string{"ws1"}, mock); r != nil {
		t.Error("expected nil for empty publishEdges")
	}
	if r := Stitch(context.Background(), []graph.Edge{{Kind: graph.EdgeIntegration, Metadata: map[string]any{"topic": "x"}}}, nil, mock); r != nil {
		t.Error("expected nil for nil targetWorkspaces")
	}
}

func TestStitchCrossServiceWorkspaceHashes(t *testing.T) {
	ws := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	publishEdges := []graph.Edge{
		{
			SourceNode: "pkg/events.go::publish",
			Kind:       graph.EdgeIntegration,
			Metadata:   map[string]any{"topic": "order.shipped"},
		},
	}

	mock := &mockStitchQuerier{
		entries: map[string][]sqlc.GraphEdge{
			ws: {
				{SourceNode: "CONSUME order.shipped", TargetNode: "HandleShipping", EdgeType: "integration"},
			},
		},
	}

	result := Stitch(context.Background(), publishEdges, []string{ws}, mock)
	if len(result) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(result))
	}
	if result[0].CrossServiceWorkspace != "abcdef12" {
		t.Errorf("expected CrossServiceWorkspace='abcdef12', got %q", result[0].CrossServiceWorkspace)
	}
}
