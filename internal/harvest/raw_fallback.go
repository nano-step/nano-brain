package harvest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/sqlc-dev/pqtype"
)

// persistRawFallback upserts a harvested session's rendered markdown as a raw
// (unsummarized) document and enqueues its chunks for embedding. It is the
// shared tx-based persistence skeleton used when a harvester has no
// summarizer configured, or summarization failed for this session.
//
// Extracted from PiHarvester.writeRawFallback, which was a near-verbatim copy
// of ClaudeCodeHarvester.writeRawFallback / OpenCodeSQLiteHarvester's
// equivalent — three copies of pure persistence plumbing with zero
// format-specific logic. Only PiHarvester calls this today; the other two
// harvesters keep their existing copies unchanged (out of scope for this
// change) and can be migrated to call this in a follow-up.
func persistRawFallback(
	ctx context.Context,
	db *sql.DB,
	workspace, sourcePath, title, md, contentHash string,
	tags []string,
	metadata map[string]any,
	enqueuer ChunkEnqueuer,
) (chunkCount int, err error) {
	metaBytes, _ := json.Marshal(metadata)
	meta := pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true}

	chunks := chunk.Split(md, chunk.DefaultConfig())
	params := sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspace,
		ContentHash:   contentHash,
		Title:         title,
		Content:       md,
		SourcePath:    sourcePath,
		Collection:    "sessions",
		Tags:          tags,
		Metadata:      meta,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, params)
	if err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("upsert document: %w", err)
	}

	if err := tq.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docRow.ID,
		WorkspaceHash: workspace,
	}); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("delete old chunks: %w", err)
	}

	var chunkIDs []uuid.UUID
	for i, c := range chunks {
		chunkHash := sha256.Sum256([]byte(c.Content))
		chunkID, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docRow.ID,
			WorkspaceHash:     workspace,
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
			return 0, fmt.Errorf("chunk upsert %d: %w", i, err)
		}
		chunkIDs = append(chunkIDs, chunkID)
	}

	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return 0, fmt.Errorf("commit: %w", err)
	}

	if enqueuer != nil {
		for _, id := range chunkIDs {
			enqueuer.Enqueue(id)
		}
	}

	return len(chunkIDs), nil
}
