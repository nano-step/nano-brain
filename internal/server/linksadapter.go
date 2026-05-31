package server

import (
	"context"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/links"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type sqlcLinksAdapter struct {
	q *sqlc.Queries
}

func (a *sqlcLinksAdapter) ListDocIDsByTitle(ctx context.Context, arg links.ListDocIDsByTitleParams) ([]uuid.UUID, error) {
	return a.q.ListDocIDsByTitle(ctx, sqlc.ListDocIDsByTitleParams{
		WorkspaceHash: arg.WorkspaceHash,
		Lower:         arg.Lower,
	})
}

func (a *sqlcLinksAdapter) ExistsDocByID(ctx context.Context, arg links.ExistsDocByIDParams) (bool, error) {
	return a.q.ExistsDocByID(ctx, sqlc.ExistsDocByIDParams{
		WorkspaceHash: arg.WorkspaceHash,
		ID:            arg.ID,
	})
}

func (a *sqlcLinksAdapter) ListReferenceEdgesBySource(ctx context.Context, arg links.ListReferenceEdgesBySourceParams) ([]links.GraphEdge, error) {
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
			ID:            r.ID,
			WorkspaceHash: r.WorkspaceHash,
			SourceNode:    r.SourceNode,
			TargetNode:    r.TargetNode,
			EdgeType:      r.EdgeType,
			SourceFile:    r.SourceFile,
			Metadata:      r.Metadata,
			CreatedAt:     r.CreatedAt,
		}
	}
	return out, nil
}

func (a *sqlcLinksAdapter) UpsertReferenceEdge(ctx context.Context, arg links.UpsertReferenceEdgeParams) error {
	return a.q.UpsertReferenceEdge(ctx, sqlc.UpsertReferenceEdgeParams{
		WorkspaceHash: arg.WorkspaceHash,
		SourceNode:    arg.SourceNode,
		TargetNode:    arg.TargetNode,
		SourceFile:    arg.SourceFile,
		Metadata:      arg.Metadata,
	})
}

func (a *sqlcLinksAdapter) DeleteReferenceEdgesBySource(ctx context.Context, arg links.DeleteReferenceEdgesBySourceParams) error {
	return a.q.DeleteReferenceEdgesBySource(ctx, sqlc.DeleteReferenceEdgesBySourceParams{
		WorkspaceHash: arg.WorkspaceHash,
		SourceNode:    arg.SourceNode,
	})
}
