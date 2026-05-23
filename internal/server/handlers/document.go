package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

type DocumentQuerier interface {
	UpsertDocument(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error)
	DeleteChunksByDocumentID(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error
	UpsertChunk(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error)
}

type ChunkEnqueuer interface {
	Enqueue(chunkID uuid.UUID) bool
}

type WriteRequest struct {
	Content    string          `json:"content"`
	Tags       []string        `json:"tags"`
	Collection string          `json:"collection"`
	Title      string          `json:"title"`
	SourcePath string          `json:"source_path"`
	Metadata   json.RawMessage `json:"metadata"`
}

type WriteResponse struct {
	ID            string `json:"id"`
	Hash          string `json:"hash"`
	Collection    string `json:"collection"`
	WorkspaceHash string `json:"workspace_hash"`
	ChunkCount    int    `json:"chunk_count"`
}

func writeChunks(ctx context.Context, q DocumentQuerier, docID uuid.UUID, workspace string, chunks []chunk.Chunk, meta pqtype.NullRawMessage) ([]uuid.UUID, error) {
	if err := q.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docID,
		WorkspaceHash: workspace,
	}); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(chunks))
	for _, ch := range chunks {
		id, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:    docID,
			WorkspaceHash: workspace,
			ContentHash:   ch.Hash,
			Content:       ch.Content,
			ChunkIndex:    int32(ch.Sequence),
			StartLine:     sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
			EndLine:       sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
			Metadata:      meta,
		})
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func WriteDocument(q DocumentQuerier, db *sql.DB, enqueuer ChunkEnqueuer, logger zerolog.Logger, maxFileSize int64) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req WriteRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		if req.Content == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "content is required")
		}

		if int64(len(req.Content)) > maxFileSize {
			return echo.NewHTTPError(http.StatusBadRequest, "content exceeds maximum allowed size")
		}

		sum := sha256.Sum256([]byte(req.Content))
		contentHash := hex.EncodeToString(sum[:])

		collection := req.Collection
		if collection == "" {
			collection = "memory"
		}

		tags := req.Tags
		if tags == nil {
			tags = []string{}
		}

		meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
		if len(req.Metadata) > 0 {
			meta = pqtype.NullRawMessage{RawMessage: req.Metadata, Valid: true}
		}

		params := sqlc.UpsertDocumentParams{
			WorkspaceHash: workspace,
			ContentHash:   contentHash,
			Title:         req.Title,
			Content:       req.Content,
			SourcePath:    req.SourcePath,
			Collection:    collection,
			Tags:          tags,
			Metadata:      meta,
		}

		chunks := chunk.Split(req.Content, chunk.DefaultConfig())
		chunkMeta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}

		var row sqlc.UpsertDocumentRow
		var chunkIDs []uuid.UUID
		if db != nil {
			tx, err := db.BeginTx(c.Request().Context(), nil)
			if err != nil {
				logger.Error().Err(err).Msg("begin transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
			tq := sqlc.New(tx)
			row, err = tq.UpsertDocument(c.Request().Context(), params)
			if err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Str("workspace", workspace).Str("hash", contentHash).Msg("upsert document failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
			chunkIDs, err = writeChunks(c.Request().Context(), tq, row.ID, workspace, chunks, chunkMeta)
			if err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Msg("write chunks failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
			if err := tx.Commit(); err != nil {
				logger.Error().Err(err).Msg("commit transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
		} else {
			var err error
			row, err = q.UpsertDocument(c.Request().Context(), params)
			if err != nil {
				logger.Error().Err(err).Str("workspace", workspace).Str("hash", contentHash).Msg("upsert document failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
			chunkIDs, err = writeChunks(c.Request().Context(), q, row.ID, workspace, chunks, chunkMeta)
			if err != nil {
				logger.Error().Err(err).Msg("write chunks failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
		}

		if enqueuer != nil {
			for _, id := range chunkIDs {
				if !enqueuer.Enqueue(id) {
					logger.Warn().Str("chunk_id", id.String()).Msg("embedding queue full, chunk will be picked up on next scan")
				}
			}
		}

		return c.JSON(http.StatusCreated, WriteResponse{
			ID:            row.ID.String(),
			Hash:          row.ContentHash,
			Collection:    row.Collection,
			WorkspaceHash: row.WorkspaceHash,
			ChunkCount:    len(chunks),
		})
	}
}
