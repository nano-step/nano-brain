package summarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/links"
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
	db            *sql.DB
	enqueuer      PersisterEnqueuer
	linkResolver  *links.Resolver
	linkExtractor *links.Extractor
	logger        zerolog.Logger
	writeToDisk   bool   // write summary markdown to outputDir after DB commit
	outputDir     string // absolute path (already tilde-expanded by config Load)
}

// NewPersister constructs a Persister.
func NewPersister(db *sql.DB, enqueuer PersisterEnqueuer, writeToDisk bool, outputDir string, logger zerolog.Logger) *Persister {
	return &Persister{
		db:          db,
		enqueuer:    enqueuer,
		writeToDisk: writeToDisk,
		outputDir:   outputDir,
		logger:      logger.With().Str("component", "summary-persister").Logger(),
	}
}

// SetLinkExtractor wires optional wikilink extraction after summary persistence.
func (p *Persister) SetLinkExtractor(resolver *links.Resolver, extractor *links.Extractor) {
	p.linkResolver = resolver
	p.linkExtractor = extractor
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

	baseTags := []string{"summary", string(meta.Source)}
	docTags := append(baseTags, meta.Tags...)

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: meta.WorkspaceHash,
		ContentHash:   contentHash,
		Title:         title,
		Content:       summaryMarkdown,
		SourcePath:    sourcePath,
		Collection:    "sessions",
		Tags:          docTags,
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
			ChunkType:         "raw",
			EmbeddingStrategy: "raw_code",
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

	if p.writeToDisk && p.outputDir != "" {
		p.persistToDisk(ctx, summaryMarkdown, meta, docRow.ID, q)
	}

	if p.linkResolver != nil && p.linkExtractor != nil {
		p.linkResolver.FlushWorkspace(meta.WorkspaceHash)
		if err := p.linkExtractor.Extract(ctx, links.Document{
			ID:         docRow.ID,
			Workspace:  meta.WorkspaceHash,
			SourcePath: sourcePath,
			Title:      title,
			Content:    summaryMarkdown,
			Collection: "sessions",
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

// persistToDisk writes the summary markdown to a file under outputDir. Errors
// are logged but never propagated — DB persist already succeeded; disk is a
// derivative view. See issue #258.
//
// The write is idempotent: if the file already exists at the resolved path and
// its content is byte-for-byte identical to summaryMarkdown, the function
// returns without rewriting. This prevents per-cycle rewrites when the
// harvester calls EnsureSummaryOnDisk on unchanged sessions.
func (p *Persister) persistToDisk(ctx context.Context, summaryMarkdown string, meta SessionMetadata, _ uuid.UUID, q *sqlc.Queries) {
	ws, err := q.GetWorkspaceByHash(ctx, meta.WorkspaceHash)
	var wsName string
	if err == nil {
		wsName = ws.Name
	}
	targetPath := BuildDiskPath(p.outputDir, wsName, meta.WorkspaceHash, string(meta.Source), meta.Title, meta.CreatedAt)
	if err := EnsureDir(targetPath); err != nil {
		p.logger.Warn().Err(err).Str("path", targetPath).Msg("disk persist: ensureDir failed")
		return
	}
	finalPath, err := ResolveCollision(targetPath, []byte(summaryMarkdown), meta.SessionID)
	if err != nil {
		p.logger.Warn().Err(err).Str("path", targetPath).Msg("disk persist: resolveCollision failed")
		return
	}
	// Idempotent skip: ResolveCollision returns the base path (targetPath)
	// when the file exists with matching content. If finalPath == targetPath
	// and the file exists, the content already matches — skip the write.
	// This avoids a second full file read; os.Stat is sufficient to confirm
	// existence (ResolveCollision already did the byte comparison).
	if finalPath == targetPath {
		if _, statErr := os.Stat(finalPath); statErr == nil {
			p.logger.Debug().Str("path", finalPath).Str("session_id", meta.SessionID).Msg("disk persist: file unchanged, skipping rewrite")
			return
		}
	}
	if err := WriteFileAtomic(finalPath, []byte(summaryMarkdown)); err != nil {
		p.logger.Warn().Err(err).Str("path", finalPath).Msg("disk persist: writeFileAtomic failed")
		return
	}
	p.logger.Info().Str("path", finalPath).Str("session_id", meta.SessionID).Msg("summary written to disk")
}

// EnsureSummaryOnDisk writes an already-stored summary to disk if the file is
// absent. It is a no-op when writeToDisk is false, outputDir is empty, or the
// file already exists with identical content (idempotent). Errors are logged
// inside persistToDisk and never propagated.
//
// This is used by harvesters on the content-unchanged skip path to backfill
// summaries that were created before write_to_disk was enabled.
func (p *Persister) EnsureSummaryOnDisk(ctx context.Context, summaryMarkdown string, meta SessionMetadata) {
	if !p.writeToDisk || p.outputDir == "" {
		return
	}
	p.persistToDisk(ctx, summaryMarkdown, meta, uuid.Nil, sqlc.New(p.db))
}

// buildSourcePath returns the canonical source path for summary documents.
// Each known source has an explicit case. Unknown sources fall through to a
// generic pattern ("summary://<source>/<sessionID>") rather than wrongly
// defaulting to opencode.
func buildSourcePath(meta SessionMetadata) string {
	switch meta.Source {
	case SourceClaude:
		return "summary://claude/" + meta.SessionID
	case SourceOpenCode:
		return "summary://opencode/" + meta.SessionID
	default:
		return "summary://" + string(meta.Source) + "/" + meta.SessionID
	}
}
