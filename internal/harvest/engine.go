package harvest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

// Compile-time assertion: Engine implements Harvester.
var _ Harvester = (*Engine)(nil)

// Engine is a generic harvest engine that drives the full pipeline:
// Discover → Read → normalize → dedup → skip-active → render → summarize/persist → raw fallback.
// It is source-agnostic: any SessionSource adapter plugs in.
type Engine struct {
	source          SessionSource
	pgDB            *sql.DB
	summarizer      SessionSummarizer
	ticketExtractor *TicketExtractor
	logger          zerolog.Logger
}

// NewEngine constructs an Engine with ticket extraction using default patterns.
// Use NewEngineWithTicketPatterns to supply custom patterns from config.
func NewEngine(source SessionSource, pgDB *sql.DB, summarizer SessionSummarizer, logger zerolog.Logger) *Engine {
	te, _ := NewTicketExtractor(nil) // default patterns; error impossible with nil input
	return &Engine{
		source:          source,
		pgDB:            pgDB,
		summarizer:      summarizer,
		ticketExtractor: te,
		logger:          logger.With().Str("component", "harvest-engine").Str("source", source.Name()).Logger(),
	}
}

// NewEngineWithTicketPatterns constructs an Engine with configurable ticket
// regex patterns. Pass nil to use built-in defaults.
func NewEngineWithTicketPatterns(source SessionSource, pgDB *sql.DB, summarizer SessionSummarizer, patterns []string, logger zerolog.Logger) (*Engine, error) {
	te, err := NewTicketExtractor(patterns)
	if err != nil {
		return nil, err
	}
	return &Engine{
		source:          source,
		pgDB:            pgDB,
		summarizer:      summarizer,
		ticketExtractor: te,
		logger:          logger.With().Str("component", "harvest-engine").Str("source", source.Name()).Logger(),
	}, nil
}

// setSummarizer satisfies the summarizerSettable interface so Runner.WithSummarizer
// propagates to the Engine.
func (e *Engine) setSummarizer(s SessionSummarizer) { e.summarizer = s }

// HarvestAll implements Harvester. It calls Discover on the source, reads each
// Location, processes sessions through the pipeline, and returns aggregate counts.
func (e *Engine) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
	q := sqlc.New(e.pgDB)

	workspaces, wsErr := q.ListWorkspaces(ctx)
	if wsErr != nil {
		e.logger.Error().Err(wsErr).Msg("engine: failed to list registered workspaces")
		return 0, 0, 1
	}
	if len(workspaces) == 0 {
		e.logger.Warn().Msg("engine: no registered workspaces, skipping harvest")
		return 0, 0, 0
	}

	registered := make(map[string]string, len(workspaces))
	for _, ws := range workspaces {
		registered[ws.Path] = ws.Hash
	}

	locs, err := e.source.Discover(ctx, registered)
	if err != nil {
		e.logger.Error().Err(err).Msg("engine: discover failed")
		return 0, 0, 1
	}

	var activeCount int
	for _, loc := range locs {
		sessions, err := e.source.Read(ctx, loc)
		if err != nil {
			e.logger.Warn().Err(err).Str("db_path", loc.DBPath).Str("session_dir", loc.SessionDir).Msg("engine: read failed, skipping location")
			errCount++
			continue
		}

		for _, sess := range sessions {
			if sess.IsActive() {
				activeCount++
				continue
			}

			wsHash := sess.WorkspaceHash
			if wsHash == "" {
				// Fall back to computing hash from Cwd (used by ClaudeSource
				// which does not pre-resolve the workspace hash).
				if sess.Cwd == "" {
					e.logger.Warn().Str("session_id", sess.SessionID).Msg("engine: skipping session with no workspace hash or cwd")
					skipped++
					continue
				}
				computed, hashErr := storage.WorkspaceHash(sess.Cwd)
				if hashErr != nil {
					e.logger.Warn().Err(hashErr).Str("session_id", sess.SessionID).Msg("engine: workspace hash failed, skipping")
					errCount++
					continue
				}
				if _, regErr := q.GetWorkspaceByHash(ctx, computed); regErr != nil {
					e.logger.Warn().Str("session_id", sess.SessionID).Str("cwd", sess.Cwd).Msg("engine: skipping session for unregistered workspace")
					skipped++
					continue
				}
				wsHash = computed
				sess.WorkspaceHash = wsHash
			}

			sourcePath := "summary://" + e.source.Name() + "/" + sess.SessionID

			existing, lookupErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
				SourcePath:    sourcePath,
				WorkspaceHash: wsHash,
			})

			md := RenderMarkdown(sess)
			sum := sha256.Sum256([]byte(md))
			contentHash := hex.EncodeToString(sum[:])

			if lookupErr == nil && existing.ContentHash == contentHash {
				// Opportunistically backfill disk: if this summary was created
				// before write_to_disk was enabled, the file may be absent.
				// Type assertion keeps SessionSummarizer interface stable.
				if dw, ok := e.summarizer.(interface {
					EnsureSummaryOnDisk(context.Context, string, SummaryMeta) error
				}); ok {
					_ = dw.EnsureSummaryOnDisk(ctx, existing.Content, SummaryMeta{
						Source:        e.source.Name(),
						SessionID:     sess.SessionID,
						Title:         sess.Title,
						CreatedAt:     existing.CreatedAt,
						WorkspaceHash: wsHash,
					})
				}
				skipped++
				continue
			}

			// Resolve parent tags for ticket inheritance (best-effort, single lookup).
			var parentTags []string
			if sess.ParentID != "" {
				parentPath := "summary://" + e.source.Name() + "/" + sess.ParentID
				parentDoc, parentErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
					SourcePath:    parentPath,
					WorkspaceHash: wsHash,
				})
				if parentErr == nil {
					parentTags = parentDoc.Tags
				}
			}

			// Extract ticket IDs from content, branch, and inherited parent tags.
			var ticketTags []string
			if e.ticketExtractor != nil {
				tickets := e.ticketExtractor.Extract(md, sess.Branch, parentTags)
				ticketTags = e.ticketExtractor.AsTags(tickets)
			}

			if e.summarizer != nil {
				smeta := SummaryMeta{
					Source:        e.source.Name(),
					SessionID:     sess.SessionID,
					Title:         sess.Title,
					CreatedAt:     sess.CreatedAt,
					WorkspaceHash: wsHash,
					ParentID:      sess.ParentID,
					Branch:        sess.Branch,
					Cwd:           sess.Cwd,
					Tags:          ticketTags,
				}
				if sumErr := e.summarizer.SummarizeAndPersist(ctx, md, smeta); sumErr != nil {
					e.logger.Warn().Err(sumErr).Str("session", sess.SessionID).Msg("engine: summarization failed, falling back to raw")
					if fbErr := e.writeRawFallback(ctx, sess, md, contentHash, wsHash, sourcePath, ticketTags, enqueuer); fbErr != nil {
						e.logger.Error().Err(fbErr).Str("session", sess.SessionID).Msg("engine: raw fallback failed, skipping")
						errCount++
						continue
					}
					harvested++
				} else {
					harvested++
				}
			} else {
				if fbErr := e.writeRawFallback(ctx, sess, md, contentHash, wsHash, sourcePath, ticketTags, enqueuer); fbErr != nil {
					e.logger.Error().Err(fbErr).Str("session", sess.SessionID).Msg("engine: raw fallback failed, skipping")
					errCount++
					continue
				}
				harvested++
			}
		}
	}

	e.logger.Info().
		Str("source", e.source.Name()).
		Int("harvested", harvested).
		Int("skipped", skipped).
		Int("active", activeCount).
		Int("errors", errCount).
		Msg("engine: harvest cycle complete")
	return
}

// writeRawFallback persists raw rendered markdown to the sessions collection.
// ticketTags are merged into the document tags (e.g. ["ticket:DEV-4706"]).
func (e *Engine) writeRawFallback(
	ctx context.Context,
	sess NormalizedSession,
	md, contentHash, wsHash, sourcePath string,
	ticketTags []string,
	enqueuer ChunkEnqueuer,
) error {
	metaBytes, _ := marshalJSON(map[string]any{
		"source":        e.source.Name(),
		"session_id":    sess.SessionID,
		"message_count": len(sess.Messages),
		"fallback":      true,
	})

	title := sess.Title
	if title == "" {
		title = e.source.Name() + " session " + sess.SessionID
	}

	baseTags := []string{e.source.Name(), "session", "fallback"}
	tags := append(baseTags, ticketTags...)

	chunks := chunk.Split(md, chunk.DefaultConfig())
	params := sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: wsHash,
		ContentHash:   contentHash,
		Title:         title,
		Content:       md,
		SourcePath:    sourcePath,
		Collection:    "sessions",
		Tags:          tags,
		Metadata:      pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true},
	}

	tx, err := e.pgDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, params)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("upsert document: %w", err)
	}

	if err := tq.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docRow.ID,
		WorkspaceHash: wsHash,
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete old chunks: %w", err)
	}

	var chunkIDs []uuid.UUID
	for i, c := range chunks {
		chunkHash := sha256.Sum256([]byte(c.Content))
		chunkID, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docRow.ID,
			WorkspaceHash:     wsHash,
			ContentHash:       hex.EncodeToString(chunkHash[:]),
			Content:           c.Content,
			ChunkIndex:        int32(i),
			StartLine:         sql.NullInt32{Int32: int32(c.StartLine), Valid: true},
			EndLine:           sql.NullInt32{Int32: int32(c.EndLine), Valid: true},
			Metadata:          pqtype.NullRawMessage{},
			ChunkType:         "raw",
			EmbeddingStrategy: "raw_code",
		})
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("chunk upsert %d: %w", i, err)
		}
		chunkIDs = append(chunkIDs, chunkID)
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("commit: %w", err)
	}

	if enqueuer != nil {
		for _, id := range chunkIDs {
			enqueuer.Enqueue(id)
		}
	}

	e.logger.Info().Str("session", sess.SessionID).Bool("fallback", true).Int("chunks", len(chunkIDs)).Msg("engine: raw fallback persisted")
	return nil
}
