package links

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Publisher accepts events from the extractor. A nil Publisher is valid (no-op).
type Publisher interface {
	Publish(Event)
}

// Event is published after a successful Extract. Defined here so the links
// package has zero internal deps; *eventbus.Bus satisfies Publisher via
// structural subtyping once Story 9.1 merges.
type Event struct {
	Type      string
	Workspace string
	Payload   json.RawMessage
	TS        time.Time
}

// ExtractorQueries combines resolver queries with reference-edge CRUD.
type ExtractorQueries interface {
	ResolverQueries
	ListReferenceEdgesBySource(ctx context.Context, arg ListReferenceEdgesBySourceParams) ([]GraphEdge, error)
	UpsertReferenceEdge(ctx context.Context, arg UpsertReferenceEdgeParams) error
	DeleteReferenceEdgesBySource(ctx context.Context, arg DeleteReferenceEdgesBySourceParams) error
}

// ListReferenceEdgesBySourceParams mirrors sqlc.ListReferenceEdgesBySourceParams.
type ListReferenceEdgesBySourceParams struct {
	WorkspaceHash string
	SourceNode    string
}

// UpsertReferenceEdgeParams mirrors sqlc.UpsertReferenceEdgeParams.
type UpsertReferenceEdgeParams struct {
	WorkspaceHash string
	SourceNode    string
	TargetNode    string
	SourceFile    string
	Metadata      json.RawMessage
}

// DeleteReferenceEdgesBySourceParams mirrors sqlc.DeleteReferenceEdgesBySourceParams.
type DeleteReferenceEdgesBySourceParams struct {
	WorkspaceHash string
	SourceNode    string
}

// GraphEdge mirrors sqlc.GraphEdge.
type GraphEdge struct {
	ID            uuid.UUID
	WorkspaceHash string
	SourceNode    string
	TargetNode    string
	EdgeType      string
	SourceFile    string
	Metadata      json.RawMessage
	CreatedAt     time.Time
}

// Document is the slim view of an indexed doc that the extractor needs.
type Document struct {
	ID         uuid.UUID
	Workspace  string
	SourcePath string
	Title      string
	Content    string
	Collection string
}

// Extractor parses wikilinks and upserts/deletes 'references' edges.
type Extractor struct {
	queries  ExtractorQueries
	resolver *Resolver
	pub      Publisher
	now      func() time.Time
}

// NewExtractor creates an Extractor. pub may be nil.
func NewExtractor(queries ExtractorQueries, resolver *Resolver, pub Publisher) *Extractor {
	return &Extractor{
		queries:  queries,
		resolver: resolver,
		pub:      pub,
		now:      time.Now,
	}
}

// Extract parses doc.Content for wikilinks, resolves them, and atomically
// replaces 'references' edges for the document. No-op if doc.Collection is
// not "memory", "session-summary", or "sessions". Idempotent. Returns nil on
// success even when 0 wikilinks found (stale rows are deleted).
func (e *Extractor) Extract(ctx context.Context, doc Document) error {
	if doc.Collection != "memory" && doc.Collection != "session-summary" && doc.Collection != "sessions" {
		return nil
	}

	parsed := Parse(doc.Content)
	type linkKey struct {
		kind Kind
		ref  string
	}
	seen := make(map[linkKey]struct{}, len(parsed))
	var deduped []Link
	for _, l := range parsed {
		k := linkKey{l.Kind, l.TargetRef}
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			deduped = append(deduped, l)
		}
	}

	type resolvedEdge struct {
		targetID uuid.UUID
		meta     json.RawMessage
	}
	var edges []resolvedEdge

	for _, l := range deduped {
		switch l.Kind {
		case KindID:
			id, err := uuid.Parse(l.TargetRef)
			if err != nil {
				continue
			}
			exists, err := e.resolver.ResolveID(ctx, doc.Workspace, id)
			if err != nil {
				return err
			}
			if !exists {
				continue
			}
			meta, _ := json.Marshal(map[string]any{"raw": l.Raw, "kind": "id"})
			edges = append(edges, resolvedEdge{targetID: id, meta: meta})

		case KindTitle:
			ids, err := e.resolver.ResolveTitle(ctx, doc.Workspace, l.TargetRef)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				continue
			}
			targetID := ids[0]
			m := map[string]any{"raw": l.Raw, "kind": "title", "title": l.TargetRef}
			if len(ids) > 1 {
				sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
				targetID = ids[0]
				candidateStrs := make([]string, len(ids))
				for i, id := range ids {
					candidateStrs[i] = id.String()
				}
				m["ambiguous"] = true
				m["candidate_ids"] = candidateStrs
			}
			meta, _ := json.Marshal(m)
			edges = append(edges, resolvedEdge{targetID: targetID, meta: meta})
		}
	}

	sourceNode := doc.ID.String()

	existing, err := e.queries.ListReferenceEdgesBySource(ctx, ListReferenceEdgesBySourceParams{
		WorkspaceHash: doc.Workspace,
		SourceNode:    sourceNode,
	})
	if err != nil {
		return err
	}
	deletedCount := len(existing)

	if err := e.queries.DeleteReferenceEdgesBySource(ctx, DeleteReferenceEdgesBySourceParams{
		WorkspaceHash: doc.Workspace,
		SourceNode:    sourceNode,
	}); err != nil {
		return err
	}

	for _, edge := range edges {
		if err := e.queries.UpsertReferenceEdge(ctx, UpsertReferenceEdgeParams{
			WorkspaceHash: doc.Workspace,
			SourceNode:    sourceNode,
			TargetNode:    edge.targetID.String(),
			SourceFile:    doc.SourcePath,
			Metadata:      edge.meta,
		}); err != nil {
			return err
		}
	}

	if e.pub != nil {
		payload, _ := json.Marshal(map[string]any{
			"doc_id":        doc.ID.String(),
			"new_count":     len(edges),
			"deleted_count": deletedCount,
		})
		e.pub.Publish(Event{
			Type:      "links_changed",
			Workspace: doc.Workspace,
			Payload:   payload,
			TS:        e.now(),
		})
	}

	return nil
}
