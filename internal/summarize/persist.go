package summarize

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

// PersisterEnqueuer enqueues chunk IDs for embedding.
// Defined locally to avoid circular imports with the harvest package.
type PersisterEnqueuer interface {
	Enqueue(chunkID uuid.UUID) bool
}

// Persister saves a summary markdown to disk and upserts it into the DB.
type Persister struct {
	db        *sql.DB
	outputDir string
	workspace string
	enqueuer  PersisterEnqueuer
	logger    zerolog.Logger
}

// NewPersister constructs a Persister and ensures the output directory exists.
func NewPersister(db *sql.DB, outputDir string, workspace string, enqueuer PersisterEnqueuer, logger zerolog.Logger) (*Persister, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("summary output_dir %q: %w", outputDir, err)
	}
	return &Persister{
		db:        db,
		outputDir: outputDir,
		workspace: workspace,
		enqueuer:  enqueuer,
		logger:    logger.With().Str("component", "summary-persister").Logger(),
	}, nil
}

// Save writes summaryMarkdown to a file and upserts it in the document store.
// It is idempotent: if the content hash matches the stored hash, it skips all work.
func (p *Persister) Save(ctx context.Context, summaryMarkdown string, meta SessionMetadata) error {
	sum := sha256.Sum256([]byte(summaryMarkdown))
	contentHash := hex.EncodeToString(sum[:])

	sourcePath := buildSourcePath(meta)

	q := sqlc.New(p.db)
	existing, lookupErr := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: p.workspace,
	})
	if lookupErr == nil && existing.ContentHash == contentHash {
		p.logger.Debug().Str("source_path", sourcePath).Msg("summary unchanged, skipping")
		return nil
	}

	if err := p.writeFile(summaryMarkdown, meta); err != nil {
		return fmt.Errorf("persist: write file: %w", err)
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
		WorkspaceHash: p.workspace,
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
			WorkspaceHash: p.workspace,
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

// writeFile writes the summary markdown to {outputDir}/{source}_{title_slug}_{date}.md.
func (p *Persister) writeFile(summaryMarkdown string, meta SessionMetadata) error {
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return fmt.Errorf("mkdir output_dir: %w", err)
	}
	slug := titleSlug(meta.Title)
	date := meta.CreatedAt.Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s_%s.md", string(meta.Source), slug, date)
	path := filepath.Join(p.outputDir, filename)
	if err := os.WriteFile(path, []byte(summaryMarkdown), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}

var reNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// titleSlug converts a title to a URL-safe slug (max 80 chars).
func titleSlug(title string) string {
	s := strings.ToLower(title)
	s = reNonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = s[:80]
		s = strings.TrimRight(s, "-")
	}
	if s == "" {
		s = "untitled"
	}
	return s
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
