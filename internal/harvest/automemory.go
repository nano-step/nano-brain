package harvest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/links"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

var (
	decisionHeadingRe = regexp.MustCompile(`(?im)^#{1,3}\s*(key\s+)?decisions?(\s|$)`)
	lessonHeadingRe   = regexp.MustCompile(`(?im)^#{1,3}\s*(key\s+)?lessons?\s*(learned)?(\s|$)`)
	decisionLineRe    = regexp.MustCompile(`(?im)^(?:DECISION|Decision):\s*(.+)`)
	lessonLineRe      = regexp.MustCompile(`(?im)^(?:LESSON|Lesson):\s*(.+)`)
)

type AutoMemoryExtractor struct {
	db            *sql.DB
	workspace     string
	logger        zerolog.Logger
	linkResolver  *links.Resolver
	linkExtractor *links.Extractor
}

func NewAutoMemoryExtractor(db *sql.DB, workspace string, logger zerolog.Logger) *AutoMemoryExtractor {
	return &AutoMemoryExtractor{
		db:        db,
		workspace: workspace,
		logger:    logger.With().Str("component", "auto-memory").Logger(),
	}
}

func (e *AutoMemoryExtractor) SetLinkExtractor(resolver *links.Resolver, extractor *links.Extractor) {
	e.linkResolver = resolver
	e.linkExtractor = extractor
}

type memoryKind string

const (
	kindDecision memoryKind = "decision"
	kindLesson   memoryKind = "lesson"
)

type extractedMemory struct {
	content string
	kind    memoryKind
	tags    []string
}

func (e *AutoMemoryExtractor) ExtractAndStore(ctx context.Context, sessionID, sessionContent string, enqueuer ChunkEnqueuer) int {
	memories := extractMemories(sessionContent)
	if len(memories) == 0 {
		return 0
	}

	q := sqlc.New(e.db)
	stored := 0

memoryLoop:
	for _, m := range memories {
		sourcePath := "automemory://" + sessionID + "/" + string(m.kind) + "/" + shortHash(m.content)
		existing, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sourcePath,
			WorkspaceHash: e.workspace,
		})
		if err == nil && existing.ContentHash != "" {
			continue
		}

		sum := sha256.Sum256([]byte(m.content))
		contentHash := hex.EncodeToString(sum[:])

		meta, _ := json.Marshal(map[string]any{
			"source":     "auto-memory",
			"session_id": sessionID,
			"kind":       string(m.kind),
		})

		chunks := chunk.Split(m.content, chunk.DefaultConfig())
		tx, err := e.db.BeginTx(ctx, nil)
		if err != nil {
			e.logger.Warn().Err(err).Str("session", sessionID).Msg("auto-memory tx begin failed")
			continue
		}

		tq := sqlc.New(tx)
		docRow, err := tq.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: e.workspace,
			ContentHash:   contentHash,
			Title:         titleFromContent(m.content),
			Content:       m.content,
			SourcePath:    sourcePath,
			Collection:    "memory",
			Tags:          append([]string{"auto-memory", string(m.kind)}, m.tags...),
			Metadata:      pqtype.NullRawMessage{RawMessage: meta, Valid: true},
		})
		if err != nil {
			tx.Rollback() //nolint:errcheck
			e.logger.Warn().Err(err).Str("session", sessionID).Msg("auto-memory upsert failed")
			continue
		}

		for i, c := range chunks {
			chunkHash := sha256.Sum256([]byte(c.Content))
			chunkID, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
				DocumentID:        docRow.ID,
				WorkspaceHash:     e.workspace,
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
				e.logger.Warn().Err(err).Str("session", sessionID).Int("chunk", i).Msg("auto-memory chunk upsert failed, rolling back")
				continue memoryLoop
			}
			if enqueuer != nil {
				enqueuer.Enqueue(chunkID)
			}
		}

		if err := tx.Commit(); err != nil {
			tx.Rollback() //nolint:errcheck
			e.logger.Warn().Err(err).Str("session", sessionID).Msg("auto-memory commit failed")
			continue
		}

		if e.linkResolver != nil && e.linkExtractor != nil {
			e.linkResolver.FlushWorkspace(e.workspace)
			if err := e.linkExtractor.Extract(ctx, links.Document{
				ID:         docRow.ID,
				Workspace:  e.workspace,
				SourcePath: sourcePath,
				Title:      titleFromContent(m.content),
				Content:    m.content,
				Collection: "memory",
			}); err != nil {
				e.logger.Warn().Err(err).Msg("link extractor failed; memory write succeeded")
			}
		}

		e.logger.Info().Str("session", sessionID).Str("kind", string(m.kind)).Msg("auto-memory entry created")
		stored++
	}
	return stored
}

func extractMemories(content string) []extractedMemory {
	var memories []extractedMemory

	for _, match := range decisionLineRe.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			memories = append(memories, extractedMemory{
				content: strings.TrimSpace(match[1]),
				kind:    kindDecision,
				tags:    []string{},
			})
		}
	}

	for _, match := range lessonLineRe.FindAllStringSubmatch(content, -1) {
		if len(match) > 1 && strings.TrimSpace(match[1]) != "" {
			memories = append(memories, extractedMemory{
				content: strings.TrimSpace(match[1]),
				kind:    kindLesson,
				tags:    []string{},
			})
		}
	}

	if decisionHeadingRe.MatchString(content) {
		sections := extractSections(content, decisionHeadingRe)
		for _, s := range sections {
			if strings.TrimSpace(s) != "" {
				memories = append(memories, extractedMemory{
					content: strings.TrimSpace(s),
					kind:    kindDecision,
					tags:    []string{},
				})
			}
		}
	}

	if lessonHeadingRe.MatchString(content) {
		sections := extractSections(content, lessonHeadingRe)
		for _, s := range sections {
			if strings.TrimSpace(s) != "" {
				memories = append(memories, extractedMemory{
					content: strings.TrimSpace(s),
					kind:    kindLesson,
					tags:    []string{},
				})
			}
		}
	}

	return dedupMemories(memories)
}

func extractSections(content string, headingRe *regexp.Regexp) []string {
	locs := headingRe.FindAllStringIndex(content, -1)
	if len(locs) == 0 {
		return nil
	}

	lines := strings.Split(content, "\n")
	var sections []string
	for _, loc := range locs {
		startLine := strings.Count(content[:loc[0]], "\n")
		var sectionLines []string
		for i := startLine + 1; i < len(lines); i++ {
			if i > startLine+1 && regexp.MustCompile(`^#{1,3}\s`).MatchString(lines[i]) {
				break
			}
			sectionLines = append(sectionLines, lines[i])
		}
		if section := strings.TrimSpace(strings.Join(sectionLines, "\n")); section != "" {
			sections = append(sections, section)
		}
	}
	return sections
}

func dedupMemories(memories []extractedMemory) []extractedMemory {
	seen := map[string]bool{}
	var result []extractedMemory
	for _, m := range memories {
		key := string(m.kind) + ":" + m.content
		if !seen[key] {
			seen[key] = true
			result = append(result, m)
		}
	}
	return result
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func titleFromContent(s string) string {
	lines := strings.SplitN(s, "\n", 2)
	title := strings.TrimSpace(lines[0])
	if len(title) > 80 {
		title = title[:77] + "..."
	}
	return title
}
