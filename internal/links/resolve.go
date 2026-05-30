package links

import (
	"context"
	"strings"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
)

// ResolverQueries is satisfied by *sqlc.Queries.
type ResolverQueries interface {
	ListDocIDsByTitle(ctx context.Context, arg ListDocIDsByTitleParams) ([]uuid.UUID, error)
	ExistsDocByID(ctx context.Context, arg ExistsDocByIDParams) (bool, error)
}

// ListDocIDsByTitleParams mirrors sqlc.ListDocIDsByTitleParams.
type ListDocIDsByTitleParams struct {
	WorkspaceHash string
	Lower         string
}

// ExistsDocByIDParams mirrors sqlc.ExistsDocByIDParams.
type ExistsDocByIDParams struct {
	WorkspaceHash string
	ID            uuid.UUID
}

const titleCacheSize = 10_000

// Resolver looks up wikilink targets within a workspace.
type Resolver struct {
	queries    ResolverQueries
	titleCache *lru.Cache[string, []uuid.UUID]
}

// NewResolver creates a Resolver with a workspace-scoped LRU title cache.
func NewResolver(queries ResolverQueries) *Resolver {
	c, _ := lru.New[string, []uuid.UUID](titleCacheSize)
	return &Resolver{queries: queries, titleCache: c}
}

func titleCacheKey(workspace, title string) string {
	return workspace + "\x00" + strings.ToLower(title)
}

// ResolveID returns whether the document exists in the workspace.
func (r *Resolver) ResolveID(ctx context.Context, workspace string, id uuid.UUID) (bool, error) {
	return r.queries.ExistsDocByID(ctx, ExistsDocByIDParams{
		WorkspaceHash: workspace,
		ID:            id,
	})
}

// ResolveTitle returns matching doc IDs by case-insensitive title match.
// Results are cached per (workspace, lower(title)); cache hit = zero DB calls.
func (r *Resolver) ResolveTitle(ctx context.Context, workspace, title string) ([]uuid.UUID, error) {
	key := titleCacheKey(workspace, title)
	if ids, ok := r.titleCache.Get(key); ok {
		return ids, nil
	}
	ids, err := r.queries.ListDocIDsByTitle(ctx, ListDocIDsByTitleParams{
		WorkspaceHash: workspace,
		Lower:         title,
	})
	if err != nil {
		return nil, err
	}
	r.titleCache.Add(key, ids)
	return ids, nil
}

// FlushWorkspace invalidates ALL cached titles for the given workspace.
// Must be called before Extract runs on a written/updated/deleted document.
func (r *Resolver) FlushWorkspace(workspace string) {
	prefix := workspace + "\x00"
	for _, k := range r.titleCache.Keys() {
		if strings.HasPrefix(k, prefix) {
			r.titleCache.Remove(k)
		}
	}
}
