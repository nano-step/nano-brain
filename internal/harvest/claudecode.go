package harvest

import (
	"bufio"
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

// harvestResult indicates the outcome of a single claudecode session harvest.
type harvestResult int

const (
	harvestSkipped  harvestResult = iota // unchanged, skipped
	harvestSummary                       // summary-first success
	harvestFallback                      // raw fallback (summarizer nil, errored, or DB upsert failed)
)

// ClaudeCodeHarvester ingests Claude Code JSONL session files into the document store.
type ClaudeCodeHarvester struct {
	db            *sql.DB
	logger        zerolog.Logger
	sessionDir    string
	workspace     string
	summarizer    SessionSummarizer
}

// WorkspaceHash returns the workspace hash this harvester was created for.
func (h *ClaudeCodeHarvester) WorkspaceHash() string { return h.workspace }

func (h *ClaudeCodeHarvester) setSummarizer(s SessionSummarizer) { h.summarizer = s }

func (h *ClaudeCodeHarvester) WithSummarizer(s SessionSummarizer) *ClaudeCodeHarvester {
	h.summarizer = s
	return h
}

// NewClaudeCodeHarvester creates a new Claude Code session harvester.
func NewClaudeCodeHarvester(db *sql.DB, logger zerolog.Logger, sessionDir, workspace string) *ClaudeCodeHarvester {
	return &ClaudeCodeHarvester{
		db:         db,
		logger:     logger.With().Str("component", "claudecode-harvester").Logger(),
		sessionDir: sessionDir,
		workspace:  workspace,
	}
}

// HarvestAll scans the session directory and ingests all JSONL sessions.
// Returns counts of harvested, skipped, and errored sessions.
func (h *ClaudeCodeHarvester) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
	if _, err := os.Stat(h.sessionDir); os.IsNotExist(err) {
		h.logger.Debug().Str("dir", h.sessionDir).Msg("session directory does not exist, skipping")
		return 0, 0, 0
	}

	entries, err := os.ReadDir(h.sessionDir)
	if err != nil {
		h.logger.Error().Err(err).Str("dir", h.sessionDir).Msg("failed to read session directory")
		return 0, 0, 1
	}

	var (
		summarySuccess  int
		summaryFallback int
	)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		filePath := filepath.Join(h.sessionDir, e.Name())
		res, err := h.harvestSession(ctx, filePath, enqueuer)
		if err != nil {
			h.logger.Error().Err(err).Str("file", filePath).Msg("failed to harvest session")
			errCount++
			continue
		}
		switch res {
		case harvestSummary:
			summarySuccess++
		case harvestFallback:
			summaryFallback++
		case harvestSkipped:
			skipped++
		}
	}

	harvested = summarySuccess + summaryFallback
	h.logger.Info().
		Str("source", "claudecode").
		Int("summary_success", summarySuccess).
		Int("summary_fallback", summaryFallback).
		Int("skipped", skipped).
		Int("errors", errCount).
		Msg("harvest cycle complete")
	return
}

func (h *ClaudeCodeHarvester) harvestSession(ctx context.Context, sessionFile string, enqueuer ChunkEnqueuer) (harvestResult, error) {
	msgs, err := parseJSONLFile(sessionFile)
	if err != nil {
		return harvestSkipped, fmt.Errorf("parse JSONL: %w", err)
	}

	if len(msgs) == 0 {
		return harvestSkipped, nil
	}

	sessionID := strings.TrimSuffix(filepath.Base(sessionFile), ".jsonl")
	md := renderClaudeCodeMarkdown(sessionID, msgs)

	sum := sha256.Sum256([]byte(md))
	contentHash := hex.EncodeToString(sum[:])

	sourcePath := "summary://claude/" + sessionID

	queries := sqlc.New(h.db)
	existing, err := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: h.workspace,
	})
	if err == nil && existing.ContentHash == contentHash {
		return harvestSkipped, nil
	}

	var createdAt time.Time
	if len(msgs) > 0 && msgs[0].Timestamp != "" {
		if t, parseErr := time.Parse(time.RFC3339, msgs[0].Timestamp); parseErr == nil {
			createdAt = t
		}
	}

	title := "Claude Code Session " + sessionID

	if h.summarizer != nil {
		smeta := SummaryMeta{
			Source:        "claude",
			SessionID:     sessionID,
			Title:         title,
			CreatedAt:     createdAt,
			WorkspaceHash: h.workspace,
		}
		if sumErr := h.summarizer.SummarizeAndPersist(ctx, md, smeta); sumErr != nil {
			h.logger.Warn().Err(sumErr).Str("session", sessionID).Msg("summarization failed, falling back to raw")
			if fbErr := h.writeRawFallback(ctx, sessionID, md, contentHash, title, sourcePath, len(msgs), enqueuer); fbErr != nil {
				return harvestSkipped, fmt.Errorf("raw fallback failed: %w", fbErr)
			}
			return harvestFallback, nil
		}
		return harvestSummary, nil
	}

	if fbErr := h.writeRawFallback(ctx, sessionID, md, contentHash, title, sourcePath, len(msgs), enqueuer); fbErr != nil {
		return harvestSkipped, fmt.Errorf("raw fallback failed: %w", fbErr)
	}
	return harvestFallback, nil
}

func (h *ClaudeCodeHarvester) writeRawFallback(
	ctx context.Context,
	sessionID, md, contentHash, title, sourcePath string,
	messageCount int,
	enqueuer ChunkEnqueuer,
) error {
	metaBytes, _ := json.Marshal(map[string]any{
		"source":        "claude",
		"session_id":    sessionID,
		"message_count": messageCount,
		"fallback":      true,
	})
	meta := pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true}

	chunks := chunk.Split(md, chunk.DefaultConfig())
	params := sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: h.workspace,
		ContentHash:   contentHash,
		Title:         title,
		Content:       md,
		SourcePath:    sourcePath,
		Collection:    "sessions",
		Tags:          []string{"claude_code", "session", "fallback"},
		Metadata:      meta,
	}

	tx, err := h.db.BeginTx(ctx, nil)
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
		WorkspaceHash: h.workspace,
	}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete old chunks: %w", err)
	}

	var chunkIDs []uuid.UUID
	for i, c := range chunks {
		chunkHash := sha256.Sum256([]byte(c.Content))
		chunkID, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docRow.ID,
			WorkspaceHash:     h.workspace,
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

	h.logger.Info().Str("session", sessionID).Bool("fallback", true).Int("chunks", len(chunkIDs)).Msg("raw fallback persisted")
	return nil
}

// claudeCodeMessage represents a single line from a Claude Code JSONL transcript.
type claudeCodeMessage struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Content   string          `json:"content"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	ToolOutput json.RawMessage `json:"tool_output"`
}

// parseJSONLFile reads a Claude Code JSONL session file and returns the messages.
// Invalid lines are skipped with no error (log + continue pattern).
func parseJSONLFile(path string) ([]claudeCodeMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var msgs []claudeCodeMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg claudeCodeMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // skip invalid lines
		}
		if msg.Type == "" {
			continue
		}
		msgs = append(msgs, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan JSONL: %w", err)
	}

	return msgs, nil
}

// renderClaudeCodeMarkdown renders Claude Code messages into a markdown document.
func renderClaudeCodeMarkdown(sessionID string, msgs []claudeCodeMessage) string {
	var b strings.Builder

	// Determine created_at from first message timestamp
	var createdAt string
	if len(msgs) > 0 && msgs[0].Timestamp != "" {
		createdAt = msgs[0].Timestamp
	}

	// Count user-visible messages (user + tool_use)
	messageCount := 0
	for _, m := range msgs {
		if m.Type == "user" || m.Type == "tool_use" {
			messageCount++
		}
	}

	b.WriteString("---\n")
	fmt.Fprintf(&b, "session_id: %s\n", sessionID)
	b.WriteString("source: claude_code\n")
	fmt.Fprintf(&b, "message_count: %d\n", messageCount)
	if createdAt != "" {
		fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	}
	b.WriteString("---\n")

	for _, msg := range msgs {
		ts := msg.Timestamp
		switch msg.Type {
		case "user":
			fmt.Fprintf(&b, "\n## human (%s)\n\n", ts)
			b.WriteString(msg.Content)
			b.WriteString("\n")

		case "tool_use":
			fmt.Fprintf(&b, "\n## assistant (%s)\n\n", ts)
			fmt.Fprintf(&b, "Tool: %s\n", msg.ToolName)
			if len(msg.ToolInput) > 0 && string(msg.ToolInput) != "null" {
			var inputMap map[string]interface{}
			if err := json.Unmarshal(msg.ToolInput, &inputMap); err == nil {
				keys := make([]string, 0, len(inputMap))
				for k := range inputMap {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(&b, "%s: %v\n", k, inputMap[k])
				}
			}
			}
			b.WriteString("\n")

		case "tool_result":
			// Include tool results inline for context
			fmt.Fprintf(&b, "\n## tool_result (%s)\n\n", ts)
			if len(msg.ToolOutput) > 0 && string(msg.ToolOutput) != "null" {
				var outputMap map[string]interface{}
				if err := json.Unmarshal(msg.ToolOutput, &outputMap); err == nil {
					if out, ok := outputMap["output"]; ok {
						fmt.Fprintf(&b, "%v\n", out)
					} else {
						// Fallback: output key missing (e.g. error or other metadata)
						b.WriteString(string(msg.ToolOutput))
						b.WriteString("\n")
					}
				} else {
					b.WriteString(string(msg.ToolOutput))
					b.WriteString("\n")
				}
			}
		}
	}

	return b.String()
}

