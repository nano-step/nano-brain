package summarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/links"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

// PersisterEnqueuer enqueues chunk IDs for embedding.
// Defined locally to avoid circular imports with the harvest package.
type PersisterEnqueuer interface {
	Enqueue(chunkID uuid.UUID) bool
}

// Persister saves a summary to the DB.
type Persister struct {
	db            *sql.DB
	enqueuer      PersisterEnqueuer
	linkResolver  *links.Resolver
	linkExtractor *links.Extractor
	logger        zerolog.Logger
}

// NewPersister constructs a Persister.
func NewPersister(db *sql.DB, enqueuer PersisterEnqueuer, logger zerolog.Logger) *Persister {
	return &Persister{
		db:       db,
		enqueuer: enqueuer,
		logger:   logger.With().Str("component", "summary-persister").Logger(),
	}
}

// SetLinkExtractor wires optional wikilink extraction after summary persistence.
func (p *Persister) SetLinkExtractor(resolver *links.Resolver, extractor *links.Extractor) {
	p.linkResolver = resolver
	p.linkExtractor = extractor
}

// Save upserts summaryMarkdown into the document store.
// It is idempotent: if the content hash matches the stored hash, it skips all work.
func (p *Persister) Save(ctx context.Context, summaryMarkdown string, meta SessionMetadata) error {
	sum := sha256.Sum256([]byte(summaryMarkdown))
	contentHash := hex.EncodeToString(sum[:])

	sourcePath := buildSourcePath(meta)

	q := sqlc.New(p.db)
	existing, lookupErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: meta.WorkspaceHash,
	})
	if lookupErr == nil && existing.ContentHash == contentHash {
		p.logger.Debug().Str("source_path", sourcePath).Msg("summary unchanged, skipping")
		return nil
	}

	title := "Summary: " + meta.Title
	if meta.Title == "" {
		title = "Summary: Untitled Session"
	}

	metaJSON, _ := json.Marshal(map[string]any{
		"source":     string(meta.Source),
		"session_id": meta.SessionID,
		"summary":    true,
	})

	chunks := chunk.Split(summaryMarkdown, chunk.DefaultConfig())

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("persist: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: meta.WorkspaceHash,
		ContentHash:   contentHash,
		Title:         title,
		Content:       summaryMarkdown,
		SourcePath:    sourcePath,
		Collection:    "session-summary",
		Tags:          []string{"summary", string(meta.Source)},
		Metadata:      pqtype.NullRawMessage{RawMessage: metaJSON, Valid: true},
	})
	if err != nil {
		return fmt.Errorf("persist: upsert document: %w", err)
	}

	var chunkIDs []uuid.UUID
	for i, c := range chunks {
		chunkHash := sha256.Sum256([]byte(c.Content))
		chunkID, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:    docRow.ID,
			WorkspaceHash: meta.WorkspaceHash,
			ContentHash:   hex.EncodeToString(chunkHash[:]),
			Content:       c.Content,
			ChunkIndex:    int32(i),
			StartLine:     sql.NullInt32{Int32: int32(c.StartLine), Valid: true},
			EndLine:       sql.NullInt32{Int32: int32(c.EndLine), Valid: true},
			Metadata:      pqtype.NullRawMessage{},
		})
		if err != nil {
			p.logger.Warn().Err(err).Int("chunk", i).Msg("persist: chunk upsert failed")
			continue
		}
		chunkIDs = append(chunkIDs, chunkID)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("persist: commit tx: %w", err)
	}

	if p.linkResolver != nil && p.linkExtractor != nil {
		p.linkResolver.FlushWorkspace(meta.WorkspaceHash)
		if err := p.linkExtractor.Extract(ctx, links.Document{
			ID:         docRow.ID,
			Workspace:  meta.WorkspaceHash,
			SourcePath: sourcePath,
			Title:      title,
			Content:    summaryMarkdown,
			Collection: "session-summary",
		}); err != nil {
			p.logger.Warn().Err(err).Msg("link extractor failed; summary persisted")
		}
	}

	if p.enqueuer != nil {
		for _, id := range chunkIDs {
			p.enqueuer.Enqueue(id)
		}
	}

	p.logger.Info().
		Str("source_path", sourcePath).
		Int("chunks", len(chunkIDs)).
		Msg("summary persisted")

	return nil
}

// buildSourcePath returns the canonical source path for summary documents.
func buildSourcePath(meta SessionMetadata) string {
	switch meta.Source {
	case SourceClaude:
		return "summary://claude/" + meta.SessionID
	default:
		return "summary://opencode/" + meta.SessionID
	}
}
