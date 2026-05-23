package harvest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

// HarvesterQuerier defines the database operations the harvester needs.
type HarvesterQuerier interface {
	UpsertDocument(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error)
	DeleteChunksByDocumentID(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error
	UpsertChunk(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error)
	GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error)
}

// ChunkEnqueuer enqueues chunk IDs for embedding.
type ChunkEnqueuer interface {
	Enqueue(chunkID uuid.UUID) bool
	IsPressured() bool
}

// OpenCodeHarvester ingests OpenCode session files into the document store.
type OpenCodeHarvester struct {
	db        *sql.DB
	logger    zerolog.Logger
	sessionDir string
	workspace  string
}

// NewOpenCodeHarvester creates a new OpenCode session harvester.
func NewOpenCodeHarvester(db *sql.DB, logger zerolog.Logger, sessionDir, workspace string) *OpenCodeHarvester {
	return &OpenCodeHarvester{
		db:         db,
		logger:     logger.With().Str("component", "opencode-harvester").Logger(),
		sessionDir: sessionDir,
		workspace:  workspace,
	}
}

// HarvestAll scans the session directory and ingests all sessions.
// Returns counts of harvested, skipped, and errored sessions.
func (h *OpenCodeHarvester) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
	sessionDir := filepath.Join(h.sessionDir, "session")
	if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
		h.logger.Debug().Str("dir", sessionDir).Msg("session directory does not exist, skipping")
		return 0, 0, 0
	}

	projectDirs, err := os.ReadDir(sessionDir)
	if err != nil {
		h.logger.Error().Err(err).Str("dir", sessionDir).Msg("failed to read session directory")
		return 0, 0, 1
	}

	for _, projEntry := range projectDirs {
		if !projEntry.IsDir() {
			continue
		}
		projDir := filepath.Join(sessionDir, projEntry.Name())
		files, err := os.ReadDir(projDir)
		if err != nil {
			h.logger.Error().Err(err).Str("dir", projDir).Msg("failed to read project session directory")
			errCount++
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".json") {
				continue
			}
			filePath := filepath.Join(projDir, f.Name())
			result, err := h.harvestSession(ctx, filePath, enqueuer)
			if err != nil {
				h.logger.Error().Err(err).Str("file", filePath).Msg("failed to harvest session")
				errCount++
				continue
			}
			if result {
				harvested++
			} else {
				skipped++
			}
		}
	}
	return
}

// harvestSession processes a single session file. Returns true if ingested, false if skipped (unchanged).
func (h *OpenCodeHarvester) harvestSession(ctx context.Context, sessionFile string, enqueuer ChunkEnqueuer) (bool, error) {
	sess, err := parseSessionFile(sessionFile)
	if err != nil {
		return false, fmt.Errorf("parse session: %w", err)
	}

	msgs, err := h.loadMessages(sess.ID)
	if err != nil {
		return false, fmt.Errorf("load messages: %w", err)
	}

	if len(msgs) == 0 {
		return false, nil
	}

	md := renderMarkdown(sess, msgs)

	sum := sha256.Sum256([]byte(md))
	contentHash := hex.EncodeToString(sum[:])

	queries := sqlc.New(h.db)
	existing, err := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sessionFile,
		WorkspaceHash: h.workspace,
	})
	if err == nil && existing.ContentHash == contentHash {
		return false, nil
	}

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	tq := sqlc.New(tx)

	meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
	tags := []string{"opencode", "session"}

	docRow, err := tq.UpsertDocument(ctx, sqlc.UpsertDocumentParams{
		WorkspaceHash: h.workspace,
		ContentHash:   contentHash,
		Title:         sess.Title,
		Content:       md,
		SourcePath:    sessionFile,
		Collection:    "sessions",
		Tags:          tags,
		Metadata:      meta,
	})
	if err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("upsert document: %w", err)
	}

	if err := tq.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docRow.ID,
		WorkspaceHash: h.workspace,
	}); err != nil {
		_ = tx.Rollback()
		return false, fmt.Errorf("delete old chunks: %w", err)
	}

	chunks := chunk.Split(md, chunk.DefaultConfig())
	chunkMeta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
	chunkIDs := make([]uuid.UUID, 0, len(chunks))

	for _, ch := range chunks {
		id, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:    docRow.ID,
			WorkspaceHash: h.workspace,
			ContentHash:   ch.Hash,
			Content:       ch.Content,
			ChunkIndex:    int32(ch.Sequence),
			StartLine:     sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
			EndLine:       sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
			Metadata:      chunkMeta,
		})
		if err != nil {
			_ = tx.Rollback()
			return false, fmt.Errorf("upsert chunk: %w", err)
		}
		chunkIDs = append(chunkIDs, id)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit tx: %w", err)
	}

	if enqueuer != nil && !enqueuer.IsPressured() {
		for _, id := range chunkIDs {
			enqueuer.Enqueue(id)
		}
	}

	return true, nil
}

// loadMessages reads all messages and their text parts for a session.
func (h *OpenCodeHarvester) loadMessages(sessionID string) ([]renderedMessage, error) {
	msgDir := filepath.Join(h.sessionDir, "message", sessionID)
	if _, err := os.Stat(msgDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(msgDir)
	if err != nil {
		return nil, fmt.Errorf("read message dir: %w", err)
	}

	var msgs []renderedMessage
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		msg, err := parseMessageFile(filepath.Join(msgDir, e.Name()))
		if err != nil {
			h.logger.Warn().Err(err).Str("file", e.Name()).Msg("skipping invalid message file")
			continue
		}

		if msg.Role != "user" && msg.Role != "assistant" {
			continue
		}

		text, err := h.loadTextParts(msg.ID)
		if err != nil {
			h.logger.Warn().Err(err).Str("msg", msg.ID).Msg("failed to load parts")
			continue
		}
		if text == "" {
			continue
		}

		msgs = append(msgs, renderedMessage{
			Role:      msg.Role,
			Content:   text,
			CreatedAt: msg.CreatedAt,
		})
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})

	return msgs, nil
}

// loadTextParts reads all text-type parts for a message, concatenated.
func (h *OpenCodeHarvester) loadTextParts(messageID string) (string, error) {
	partDir := filepath.Join(h.sessionDir, "part", messageID)
	if _, err := os.Stat(partDir); os.IsNotExist(err) {
		return "", nil
	}

	entries, err := os.ReadDir(partDir)
	if err != nil {
		return "", fmt.Errorf("read part dir: %w", err)
	}

	type sortedPart struct {
		id   string
		text string
	}
	var parts []sortedPart

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p, err := parsePartFile(filepath.Join(partDir, e.Name()))
		if err != nil {
			continue
		}
		if p.Type == "text" && p.Text != "" {
			parts = append(parts, sortedPart{id: p.ID, text: p.Text})
		}
	}

	sort.Slice(parts, func(i, j int) bool {
		return parts[i].id < parts[j].id
	})

	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(p.text)
	}
	return b.String(), nil
}

type sessionFile struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Time  struct {
		Created int64 `json:"created"`
		Updated int64 `json:"updated"`
	} `json:"time"`
	ProjectID string `json:"projectID"`
	Directory string `json:"directory"`
}

type messageFile struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionID"`
	Role      string `json:"role"`
	Time      struct {
		Created   int64 `json:"created"`
		Completed int64 `json:"completed"`
	} `json:"time"`
	CreatedAt time.Time
}

type partFile struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	MessageID string `json:"messageID"`
}

type renderedMessage struct {
	Role      string
	Content   string
	CreatedAt time.Time
}

func parseSessionFile(path string) (*sessionFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s sessionFile
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.ID == "" {
		return nil, fmt.Errorf("session file missing id")
	}
	return &s, nil
}

func parseMessageFile(path string) (*messageFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m messageFile
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, fmt.Errorf("message file missing id")
	}
	if m.Time.Created > 0 {
		m.CreatedAt = time.UnixMilli(m.Time.Created)
	}
	return &m, nil
}

func parsePartFile(path string) (*partFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p partFile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func renderMarkdown(sess *sessionFile, msgs []renderedMessage) string {
	var b strings.Builder

	createdAt := time.UnixMilli(sess.Time.Created).UTC().Format(time.RFC3339)

	b.WriteString("---\n")
	fmt.Fprintf(&b, "session_id: %s\n", sess.ID)
	b.WriteString("source: opencode\n")
	fmt.Fprintf(&b, "message_count: %d\n", len(msgs))
	fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	if sess.Title != "" {
		fmt.Fprintf(&b, "title: %q\n", sess.Title)
	}
	if sess.Directory != "" {
		fmt.Fprintf(&b, "directory: %q\n", sess.Directory)
	}
	b.WriteString("---\n")

	for _, msg := range msgs {
		ts := msg.CreatedAt.UTC().Format(time.RFC3339)
		fmt.Fprintf(&b, "\n## %s (%s)\n\n", msg.Role, ts)
		b.WriteString(msg.Content)
		b.WriteString("\n")
	}

	return b.String()
}
