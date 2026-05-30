package summarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

// ErrWorkspaceNotRegistered is returned by Persister.Save when meta.WorkspaceHash
// does not correspond to a row in the workspaces table. This is defense-in-depth
// against bypassing HTTP middleware or harvester validation. See issue #238.
var ErrWorkspaceNotRegistered = errors.New("workspace_not_registered")

// PersisterEnqueuer enqueues chunk IDs for embedding.
// Defined locally to avoid circular imports with the harvest package.
type PersisterEnqueuer interface {
	Enqueue(chunkID uuid.UUID) bool
}

// Persister saves a summary to the DB.
type Persister struct {
	db       *sql.DB
	enqueuer PersisterEnqueuer
	logger   zerolog.Logger
}

// NewPersister constructs a Persister.
func NewPersister(db *sql.DB, enqueuer PersisterEnqueuer, logger zerolog.Logger) *Persister {
	return &Persister{
		db:       db,
		enqueuer: enqueuer,
		logger:   logger.With().Str("component", "summary-persister").Logger(),
	}
}

// Save upserts summaryMarkdown into the document store.
// It is idempotent: if the content hash matches the stored hash, it skips all work.
//
// Returns ErrWorkspaceNotRegistered if meta.WorkspaceHash is not in the workspaces
// table (defense-in-depth — issue #238).
func (p *Persister) Save(ctx context.Context, summaryMarkdown string, meta SessionMetadata) error {
	q := sqlc.New(p.db)

	if _, err := q.GetWorkspaceByHash(ctx, meta.WorkspaceHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			p.logger.Warn().
				Str("workspace_hash", meta.WorkspaceHash).
				Str("session_id", meta.SessionID).
				Msg("persist: refusing to save summary for unregistered workspace")
			return fmt.Errorf("%w: %s", ErrWorkspaceNotRegistered, meta.WorkspaceHash)
		}
		return fmt.Errorf("persist: workspace lookup failed: %w", err)
	}

	sum := sha256.Sum256([]byte(summaryMarkdown))
	contentHash := hex.EncodeToString(sum[:])

	sourcePath := buildSourcePath(meta)

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
