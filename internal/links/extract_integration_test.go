//go:build integration

package links_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nano-brain/nano-brain/internal/links"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/testutil"
)

type integrationAdapter struct {
	q *sqlc.Queries
}

func (a *integrationAdapter) ListDocIDsByTitle(ctx context.Context, arg links.ListDocIDsByTitleParams) ([]uuid.UUID, error) {
	return a.q.ListDocIDsByTitle(ctx, sqlc.ListDocIDsByTitleParams{
		WorkspaceHash: arg.WorkspaceHash,
		Lower:         arg.Lower,
	})
}

func (a *integrationAdapter) ExistsDocByID(ctx context.Context, arg links.ExistsDocByIDParams) (bool, error) {
	return a.q.ExistsDocByID(ctx, sqlc.ExistsDocByIDParams{
		WorkspaceHash: arg.WorkspaceHash,
		ID:            arg.ID,
	})
}

func (a *integrationAdapter) ListReferenceEdgesBySource(ctx context.Context, arg links.ListReferenceEdgesBySourceParams) ([]links.GraphEdge, error) {
	rows, err := a.q.ListReferenceEdgesBySource(ctx, sqlc.ListReferenceEdgesBySourceParams{
		WorkspaceHash: arg.WorkspaceHash,
		SourceNode:    arg.SourceNode,
	})
	if err != nil {
		return nil, err
	}
	out := make([]links.GraphEdge, len(rows))
	for i, r := range rows {
		out[i] = links.GraphEdge{
			ID: r.ID, WorkspaceHash: r.WorkspaceHash,
			SourceNode: r.SourceNode, TargetNode: r.TargetNode,
			EdgeType: r.EdgeType, SourceFile: r.SourceFile,
			Metadata: r.Metadata, CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

func (a *integrationAdapter) UpsertReferenceEdge(ctx context.Context, arg links.UpsertReferenceEdgeParams) error {
	return a.q.UpsertReferenceEdge(ctx, sqlc.UpsertReferenceEdgeParams{
		WorkspaceHash: arg.WorkspaceHash,
		SourceNode:    arg.SourceNode, TargetNode: arg.TargetNode,
		SourceFile: arg.SourceFile, Metadata: arg.Metadata,
	})
}

func (a *integrationAdapter) DeleteReferenceEdgesBySource(ctx context.Context, arg links.DeleteReferenceEdgesBySourceParams) error {
	return a.q.DeleteReferenceEdgesBySource(ctx, sqlc.DeleteReferenceEdgesBySourceParams{
		WorkspaceHash: arg.WorkspaceHash,
		SourceNode:    arg.SourceNode,
	})
}

func TestExtractIntegration(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	q := sqlc.New(db)
	ctx := context.Background()
	ws := "test-ws-hash"

	_, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: ws, Name: "test", Path: "/tmp/test",
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	writeDoc := func(t *testing.T, title, content, collection string) uuid.UUID {
		t.Helper()
		row, err := q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: ws,
			ContentHash:   title + "-hash",
			Title:         title,
			Content:       content,
			SourcePath:    "test://" + title,
			Collection:    collection,
			Tags:          []string{},
		})
		if err != nil {
			t.Fatalf("write doc %q: %v", title, err)
		}
		return row.ID
	}

	adapter := &integrationAdapter{q: q}
	resolver := links.NewResolver(adapter)
	extractor := links.NewExtractor(adapter, resolver, nil)

	idA := writeDoc(t, "Foo", "doc A content", "memory")

	idB := writeDoc(t, "Doc B", "start [[Foo]] end", "memory")
	resolver.FlushWorkspace(ws)
	if err := extractor.Extract(ctx, links.Document{
		ID: idB, Workspace: ws, SourcePath: "test://Doc B",
		Title: "Doc B", Content: "start [[Foo]] end", Collection: "memory",
	}); err != nil {
		t.Fatal(err)
	}

	edges, err := adapter.ListReferenceEdgesBySource(ctx, links.ListReferenceEdgesBySourceParams{
		WorkspaceHash: ws, SourceNode: idB.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].TargetNode != idA.String() {
		t.Errorf("target = %s, want %s", edges[0].TargetNode, idA.String())
	}

	resolver.FlushWorkspace(ws)
	if err := extractor.Extract(ctx, links.Document{
		ID: idB, Workspace: ws, SourcePath: "test://Doc B",
		Title: "Doc B", Content: "start [[Foo]] end", Collection: "memory",
	}); err != nil {
		t.Fatal(err)
	}
	edges, err = adapter.ListReferenceEdgesBySource(ctx, links.ListReferenceEdgesBySourceParams{
		WorkspaceHash: ws, SourceNode: idB.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) != 1 {
		t.Fatalf("idempotent re-extract: expected 1 edge, got %d", len(edges))
	}

	idC := writeDoc(t, "Doc C", "[[Foo]]", "memory")
	resolver.FlushWorkspace(ws)
	if err := extractor.Extract(ctx, links.Document{
		ID: idC, Workspace: ws, SourcePath: "test://Doc C",
		Title: "Doc C", Content: "[[Foo]]", Collection: "memory",
	}); err != nil {
		t.Fatal(err)
	}

	edgesB, _ := adapter.ListReferenceEdgesBySource(ctx, links.ListReferenceEdgesBySourceParams{
		WorkspaceHash: ws, SourceNode: idB.String(),
	})
	edgesC, _ := adapter.ListReferenceEdgesBySource(ctx, links.ListReferenceEdgesBySourceParams{
		WorkspaceHash: ws, SourceNode: idC.String(),
	})
	if len(edgesB)+len(edgesC) != 2 {
		t.Errorf("total reference edges: got %d, want 2", len(edgesB)+len(edgesC))
	}

	_, err = q.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: ws, ContentHash: "bar-hash-v2",
		Title: "Bar", Content: "updated content", SourcePath: "test://Foo",
		Collection: "memory", Tags: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}

	idE := writeDoc(t, "Doc E", "[[Bar]]", "memory")
	resolver.FlushWorkspace(ws)
	if err := extractor.Extract(ctx, links.Document{
		ID: idE, Workspace: ws, SourcePath: "test://Doc E",
		Title: "Doc E", Content: "[[Bar]]", Collection: "memory",
	}); err != nil {
		t.Fatal(err)
	}
	edgesE, _ := adapter.ListReferenceEdgesBySource(ctx, links.ListReferenceEdgesBySourceParams{
		WorkspaceHash: ws, SourceNode: idE.String(),
	})
	if len(edgesE) != 1 {
		t.Errorf("title rename: expected 1 edge from E, got %d", len(edgesE))
	}

	idF := writeDoc(t, "Doc F", "[[Foo]]", "code:go")
	resolver.FlushWorkspace(ws)
	if err := extractor.Extract(ctx, links.Document{
		ID: idF, Workspace: ws, SourcePath: "test://Doc F",
		Title: "Doc F", Content: "[[Foo]]", Collection: "code:go",
	}); err != nil {
		t.Fatal(err)
	}
	edgesF, _ := adapter.ListReferenceEdgesBySource(ctx, links.ListReferenceEdgesBySourceParams{
		WorkspaceHash: ws, SourceNode: idF.String(),
	})
	if len(edgesF) != 0 {
		t.Errorf("scope skip: expected 0 edges from F (code:go), got %d", len(edgesF))
	}
}
