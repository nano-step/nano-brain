package flow_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/flow"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type unresolvedMaterializerStore struct {
	edges []sqlc.GraphEdge
	docs  []sqlc.UpsertDocumentBySourcePathParams
}

func (s *unresolvedMaterializerStore) ListAllEdgesByWorkspace(context.Context, string) ([]sqlc.GraphEdge, error) {
	return s.edges, nil
}

func (s *unresolvedMaterializerStore) UpsertDocumentBySourcePath(_ context.Context, arg sqlc.UpsertDocumentBySourcePathParams) (sqlc.UpsertDocumentBySourcePathRow, error) {
	s.docs = append(s.docs, arg)
	return sqlc.UpsertDocumentBySourcePathRow{ID: uuid.New()}, nil
}

func (*unresolvedMaterializerStore) ListDocumentSourcePathsAndHashes(context.Context, sqlc.ListDocumentSourcePathsAndHashesParams) ([]sqlc.ListDocumentSourcePathsAndHashesRow, error) {
	return nil, nil
}

func (*unresolvedMaterializerStore) DeleteDocumentByIDAndWorkspace(context.Context, sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error) {
	return 0, nil
}

func (*unresolvedMaterializerStore) DeleteChunksByDocumentID(context.Context, sqlc.DeleteChunksByDocumentIDParams) error {
	return nil
}

func (*unresolvedMaterializerStore) UpsertChunk(context.Context, sqlc.UpsertChunkParams) (uuid.UUID, error) {
	return uuid.New(), nil
}

func TestQualifiedCallTraversesOnlyItsExactTarget(t *testing.T) {
	// Given: repo-a and repo-b both define run, but the caller names repo-a exactly.
	edges := []graph.Edge{
		{SourceNode: "POST /run", TargetNode: "repo-a/consumer.ts::caller", Kind: graph.EdgeHTTP},
		{SourceNode: "repo-a/consumer.ts::caller", TargetNode: "repo-a/lib/api.ts::run", Kind: graph.EdgeCalls},
		{SourceNode: "repo-a/lib/api.ts::run", TargetNode: "repo-a/lib/store.ts::persist", Kind: graph.EdgeCalls},
		{SourceNode: "repo-b/lib/api.ts::run", TargetNode: "repo-b/lib/store.ts::leak", Kind: graph.EdgeCalls},
	}

	// When
	got := flow.BuildFlow(edges, "POST /run", 10, 10)

	// Then
	if _, ok := nodeByID(got.Nodes, "repo-a/lib/store.ts::persist"); !ok {
		t.Fatal("qualified repo-a target was not traversed exactly")
	}
	if _, ok := nodeByID(got.Nodes, "repo-b/lib/store.ts::leak"); ok {
		t.Fatal("qualified repo-a target fell back to the unrelated bare run symbol")
	}
}

func TestUnresolvedCallDoesNotEnterDerivedFlow(t *testing.T) {
	// Given: the raw one-hop edge is preserved for diagnostics.
	edges := []graph.Edge{
		{SourceNode: "POST /unresolved", TargetNode: "repo-a/consumer.ts::caller", Kind: graph.EdgeHTTP},
		{SourceNode: "repo-a/consumer.ts::caller", TargetNode: "<unresolved>", Kind: graph.EdgeCalls},
	}

	// When
	got := flow.BuildFlow(edges, "POST /unresolved", 10, 10)

	// Then: flow is derived output, unlike the originating raw one-hop edge.
	if _, ok := nodeByID(got.Nodes, "<unresolved>"); ok {
		t.Fatal("unresolved sentinel entered derived flow nodes")
	}
	if hasEdge(got.Edges, "repo-a/consumer.ts::caller", "<unresolved>", "calls") {
		t.Fatal("unresolved sentinel entered derived flow edges")
	}
}

func TestMaterializerDoesNotCacheUnresolvedFlowContent(t *testing.T) {
	// Given: an HTTP flow with an unresolved raw calls edge.
	store := &unresolvedMaterializerStore{edges: []sqlc.GraphEdge{
		{SourceNode: "POST /materialize", TargetNode: "repo-a/consumer.ts::caller", EdgeType: "http"},
		{SourceNode: "repo-a/consumer.ts::caller", TargetNode: "<unresolved>", EdgeType: "calls"},
	}}
	materializer := flow.NewMaterializer(store, nil, 10, 10, time.Minute, nil, zerolog.Nop())

	// When
	if err := materializer.Materialize(context.Background(), "workspace"); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	// Then: the persisted document/hash input is derived state, never raw diagnostics.
	if len(store.docs) != 1 {
		t.Fatalf("materialized documents = %d, want 1", len(store.docs))
	}
	if strings.Contains(store.docs[0].Content, "<unresolved>") {
		t.Fatal("unresolved sentinel entered materialized flow content")
	}
}

func TestLegacyRubyNamespacedTargetRetainsReconciliation(t *testing.T) {
	// Given: a historical Ruby namespace is not a canonical JS/TS target.
	edges := []graph.Edge{
		{SourceNode: "POST /stories", TargetNode: "Api::V2::StoriesController#sync", Kind: graph.EdgeHTTP},
		{SourceNode: "Api::V2::StoriesController#sync", TargetNode: "app/controllers/stories.rb::Api::V2::StoriesController#sync", Kind: graph.EdgeReconcile},
		{SourceNode: "app/controllers/stories.rb::Api::V2::StoriesController#sync", TargetNode: "Story.sync_all", Kind: graph.EdgeCalls},
	}

	// When
	got := flow.BuildFlow(edges, "POST /stories", 10, 10)

	// Then
	if _, ok := nodeByID(got.Nodes, "Story.sync_all"); !ok {
		t.Fatal("legacy Ruby namespace lost its reconciliation path")
	}
}
