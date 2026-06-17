package flow

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

const flowCollection = "flows"

// MaterializerQuerier is the minimal DB interface needed by Materializer.
// Defined consumer-side so callers can inject any compatible implementation.
type MaterializerQuerier interface {
	ListAllEdgesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
	UpsertDocumentBySourcePath(ctx context.Context, arg sqlc.UpsertDocumentBySourcePathParams) (sqlc.UpsertDocumentBySourcePathRow, error)
	ListDocumentSourcePathsAndHashes(ctx context.Context, arg sqlc.ListDocumentSourcePathsAndHashesParams) ([]sqlc.ListDocumentSourcePathsAndHashesRow, error)
	DeleteDocumentByIDAndWorkspace(ctx context.Context, arg sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error)
	DeleteChunksByDocumentID(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error
	UpsertChunk(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error)
}

// Materializer builds searchable flow-summary documents from graph edges,
// one document per HTTP entry point. Documents are stored in the "flows"
// collection so they don't pollute default search results.
type Materializer struct {
	queries         MaterializerQuerier
	enqueue         func(uuid.UUID)
	logger          zerolog.Logger
	maxDepth        int
	maxFanout       int
	summaryTimeout  time.Duration
	summarizer      FlowSummarizer

	mu        sync.Mutex
	inFlight  map[string]bool
	pending   map[string]bool
}

// NewMaterializer constructs a Materializer. enqueue may be nil (embedding skipped).
// summarizer may be nil (flow summarization disabled).
func NewMaterializer(
	queries MaterializerQuerier,
	enqueue func(uuid.UUID),
	maxDepth, maxFanout int,
	summaryTimeout time.Duration,
	summarizer FlowSummarizer,
	logger zerolog.Logger,
) *Materializer {
	return &Materializer{
		queries:        queries,
		enqueue:        enqueue,
		logger:         logger.With().Str("component", "flow.materializer").Logger(),
		maxDepth:       maxDepth,
		maxFanout:      maxFanout,
		summaryTimeout: summaryTimeout,
		summarizer:     summarizer,
		inFlight:       make(map[string]bool),
		pending:        make(map[string]bool),
	}
}

// Trigger schedules Materialize for workspaceHash. If a run is already in
// progress for that workspace, exactly one follow-up run is coalesced.
// The call returns immediately; work runs in the calling goroutine of any
// subsequent invocation. Callers that want fire-and-forget should wrap with
// go m.Trigger(ctx, ws).
func (m *Materializer) Trigger(ctx context.Context, workspaceHash string) {
	m.mu.Lock()
	if m.inFlight[workspaceHash] {
		m.pending[workspaceHash] = true
		m.mu.Unlock()
		return
	}
	m.inFlight[workspaceHash] = true
	m.mu.Unlock()

	for {
		if err := m.Materialize(ctx, workspaceHash); err != nil {
			m.logger.Error().Err(err).Str("workspace", workspaceHash).Msg("flow materialization failed")
		}

		m.mu.Lock()
		if m.pending[workspaceHash] {
			delete(m.pending, workspaceHash)
			m.mu.Unlock()
			// loop: run again
			continue
		}
		delete(m.inFlight, workspaceHash)
		m.mu.Unlock()
		return
	}
}

// Materialize loads all edges for a workspace, builds a text summary for every
// HTTP entry point, upserts a flow document, and deletes stale flow docs for
// routes that no longer exist.
func (m *Materializer) Materialize(ctx context.Context, workspaceHash string) error {
	// 1. Load all edges.
	rawEdges, err := m.queries.ListAllEdgesByWorkspace(ctx, workspaceHash)
	if err != nil {
		return fmt.Errorf("list edges: %w", err)
	}

	// Convert sqlc GraphEdge → graph.Edge, decoding metadata so conditional /
	// line / topic info survives into the built flow (mirrors the API path).
	edges := make([]graph.Edge, 0, len(rawEdges))
	for _, e := range rawEdges {
		ge := graph.Edge{
			SourceNode: e.SourceNode,
			TargetNode: e.TargetNode,
			Kind:       graph.EdgeKind(e.EdgeType),
			SourceFile: e.SourceFile,
		}
		if len(e.Metadata) > 0 {
			var meta map[string]any
			if jsonErr := json.Unmarshal(e.Metadata, &meta); jsonErr == nil {
				if lang, ok := meta["language"].(string); ok {
					ge.Language = lang
				}
				if line, ok := meta["line"].(float64); ok {
					ge.Line = int(line)
				}
				ge.Metadata = meta
			}
		}
		edges = append(edges, ge)
	}

	// 2. Find all entry nodes (HTTP + consumer).
	entrySet := make(map[string]struct{})
	for _, e := range edges {
		if e.Kind == graph.EdgeHTTP {
			entrySet[e.SourceNode] = struct{}{}
		}
		if e.Kind == graph.EdgeIntegration {
			if strings.HasPrefix(e.SourceNode, "CONSUME ") || strings.HasPrefix(e.SourceNode, "ON ") {
				entrySet[e.SourceNode] = struct{}{}
			}
		}
	}

	// Load existing flow docs once: used both for change-detection (skip
	// unchanged entries) and for staleness cleanup below.
	existing, err := m.queries.ListDocumentSourcePathsAndHashes(ctx, sqlc.ListDocumentSourcePathsAndHashesParams{
		WorkspaceHash: workspaceHash,
		Collection:    flowCollection,
	})
	if err != nil {
		return fmt.Errorf("list existing flow docs: %w", err)
	}
	existingByPath := make(map[string]sqlc.ListDocumentSourcePathsAndHashesRow, len(existing))
	for _, row := range existing {
		existingByPath[row.SourcePath] = row
	}

	// 3. Build and upsert a flow doc for each entry.
	currentSourcePaths := make(map[string]struct{}, len(entrySet))
	for entry := range entrySet {
		sourcePath := "flow://" + entry
		currentSourcePaths[sourcePath] = struct{}{}

		f := BuildFlow(edges, entry, m.maxDepth, m.maxFanout)

		// The deterministic text summary is the dedup signature. The optional
		// LLM summary only changes the *display* content; we keep the signature
		// as the content hash so repeated runs over an unchanged flow don't
		// re-summarize, re-upsert, or re-embed.
		textSummary := renderTextSummary(f)
		sum := sha256.Sum256([]byte(textSummary))
		contentHash := hex.EncodeToString(sum[:])

		if prev, ok := existingByPath[sourcePath]; ok && prev.ContentHash == contentHash {
			continue // unchanged — skip LLM call, upsert, and re-embed
		}

		content := textSummary
		metaRaw := pqtype.NullRawMessage{Valid: false}

		if m.summarizer != nil {
			chain := buildChain(f)
			var integrations []string
			for _, n := range f.Nodes {
				if n.Role == RoleIntegration || n.Role == RoleExternal {
					integrations = append(integrations, n.Name)
				}
			}
			sort.Strings(integrations)

			summaryCtx, cancel := context.WithTimeout(ctx, m.summaryTimeout)
			summary, serr := m.summarizer.Summarize(summaryCtx, entry, chain, integrations)
			cancel() // release timer immediately — no defer inside loop
			if serr != nil {
				m.logger.Warn().Err(serr).Str("entry", entry).Msg("flow summarization failed, using text summary")
			} else {
				meta := map[string]any{"chain": chain}
				metaBytes, _ := json.Marshal(meta)
				metaRaw = pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true}
				content = summary
				// NOTE: contentHash stays the text-summary signature — the stable
				// dedup key — so it is intentionally not recomputed here.
			}
		}

		tags := []string{"flow"}
		if strings.HasPrefix(entry, "CONSUME ") || strings.HasPrefix(entry, "ON ") {
			tags = []string{"flow", "consumer"}
		}

		params := sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: workspaceHash,
			ContentHash:   contentHash,
			Title:         entry + " flow",
			Content:       content,
			SourcePath:    sourcePath,
			Collection:    flowCollection,
			Tags:          tags,
			Metadata:      metaRaw,
			SupersedesID:  uuid.NullUUID{Valid: false},
		}

		docRow, err := m.queries.UpsertDocumentBySourcePath(ctx, params)
		if err != nil {
			m.logger.Warn().Err(err).Str("entry", entry).Msg("flow doc upsert failed")
			continue
		}

		// Upsert a single chunk (flow summaries are short).
		chunkHash := contentHash // reuse — content is identical
		chunkID, err := m.queries.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docRow.ID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       chunkHash,
			Content:           content,
			ChunkIndex:        0,
			StartLine:         sql.NullInt32{Valid: false},
			EndLine:           sql.NullInt32{Valid: false},
			Metadata:          pqtype.NullRawMessage{Valid: false},
			SymbolName:        sql.NullString{Valid: false},
			SymbolKind:        sql.NullString{Valid: false},
			Language:          sql.NullString{Valid: false},
			LineStart:         sql.NullInt32{Valid: false},
			LineEnd:           sql.NullInt32{Valid: false},
			ChunkType:         "flow",
			EmbeddingStrategy: "full",
		})
		if err != nil {
			m.logger.Warn().Err(err).Str("entry", entry).Msg("flow chunk upsert failed")
			continue
		}

		if m.enqueue != nil {
			m.enqueue(chunkID)
		}

		m.logger.Debug().Str("entry", entry).Str("doc_id", docRow.ID.String()).Msg("flow doc materialized")
	}

	// 4. Staleness: delete flow docs whose source_path is not in current entry
	// set (reuses the `existing` snapshot loaded before the build loop).
	for _, row := range existing {
		if _, ok := currentSourcePaths[row.SourcePath]; ok {
			continue
		}
		// Stale: delete chunks first, then document.
		if err := m.queries.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
			DocumentID:    row.ID,
			WorkspaceHash: workspaceHash,
		}); err != nil {
			m.logger.Warn().Err(err).Str("source_path", row.SourcePath).Msg("delete stale flow chunks failed")
		}
		if _, err := m.queries.DeleteDocumentByIDAndWorkspace(ctx, sqlc.DeleteDocumentByIDAndWorkspaceParams{
			ID:            row.ID,
			WorkspaceHash: workspaceHash,
		}); err != nil {
			m.logger.Warn().Err(err).Str("source_path", row.SourcePath).Msg("delete stale flow doc failed")
		} else {
			m.logger.Debug().Str("source_path", row.SourcePath).Msg("stale flow doc deleted")
		}
	}

	return nil
}

// renderTextSummary produces a human-readable text summary of a flow.
// Format:
//
//	Entry: POST /api/topup
//	Chain: POST /api/topup -> AuthMiddleware -> TopupHandler -> PaymentService -> PaymentRepo
//	Externals: stripe.Charge, redis.Set
//	Nodes:
//	  POST /api/topup [entry]
//	  AuthMiddleware [middleware]
//	  ...
func renderTextSummary(f Flow) string {
	var sb strings.Builder

	sb.WriteString("Entry: ")
	sb.WriteString(f.Entry)
	sb.WriteString("\n")

	// Build an ordered chain: BFS/DFS from entry following edges in order.
	chain := buildChain(f)
	if len(chain) > 0 {
		sb.WriteString("Chain: ")
		sb.WriteString(strings.Join(chain, " -> "))
		sb.WriteString("\n")
	}

	// Collect externals (sorted for deterministic output).
	var externals []string
	for _, n := range f.Nodes {
		if n.Role == RoleExternal {
			externals = append(externals, n.Name)
		}
	}
	if len(externals) > 0 {
		sort.Strings(externals)
		sb.WriteString("Externals: ")
		sb.WriteString(strings.Join(externals, ", "))
		sb.WriteString("\n")
	}

	// Node roles (sorted for deterministic output, since f.Nodes comes from a map).
	if len(f.Nodes) > 0 {
		nodes := make([]FlowNode, len(f.Nodes))
		copy(nodes, f.Nodes)
		sort.Slice(nodes, func(i, j int) bool {
			if nodes[i].Name != nodes[j].Name {
				return nodes[i].Name < nodes[j].Name
			}
			return nodes[i].Role < nodes[j].Role
		})
		sb.WriteString("Nodes:\n")
		for _, n := range nodes {
			sb.WriteString("  ")
			sb.WriteString(n.Name)
			sb.WriteString(" [")
			sb.WriteString(string(n.Role))
			sb.WriteString("]\n")
		}
	}

	return sb.String()
}

// buildChain returns node names in traversal order starting from the entry node,
// following edges (entry→handler→calls chain).
func buildChain(f Flow) []string {
	if len(f.Nodes) == 0 {
		return nil
	}

	// Build adjacency: from → []to. f.Edges comes from a map, so sort each
	// target list to make traversal order (and thus the chain) deterministic.
	adj := make(map[string][]string)
	for _, e := range f.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	for k := range adj {
		sort.Strings(adj[k])
	}

	seen := make(map[string]bool)
	var chain []string

	var visit func(id string)
	visit = func(id string) {
		if seen[id] {
			return
		}
		seen[id] = true
		// Look up display name.
		name := id
		for _, n := range f.Nodes {
			if n.ID == id {
				name = n.Name
				break
			}
		}
		chain = append(chain, name)
		for _, next := range adj[id] {
			visit(next)
		}
	}

	// Start from entry node ID (which equals f.Entry).
	visit(f.Entry)
	return chain
}
