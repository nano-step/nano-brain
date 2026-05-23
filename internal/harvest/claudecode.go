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

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

// ClaudeCodeHarvester ingests Claude Code JSONL session files into the document store.
type ClaudeCodeHarvester struct {
	db         *sql.DB
	logger     zerolog.Logger
	sessionDir string
	workspace  string
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

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		filePath := filepath.Join(h.sessionDir, e.Name())
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
	return
}

// harvestSession processes a single JSONL session file. Returns true if ingested, false if skipped (unchanged).
func (h *ClaudeCodeHarvester) harvestSession(ctx context.Context, sessionFile string, enqueuer ChunkEnqueuer) (bool, error) {
	msgs, err := parseJSONLFile(sessionFile)
	if err != nil {
		return false, fmt.Errorf("parse JSONL: %w", err)
	}

	if len(msgs) == 0 {
		return false, nil
	}

	sessionID := strings.TrimSuffix(filepath.Base(sessionFile), ".jsonl")
	md := renderClaudeCodeMarkdown(sessionID, msgs)

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
	defer tx.Rollback() // no-op after commit
	tq := sqlc.New(tx)

	meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
	tags := []string{"claude_code", "session"}

	docRow, err := tq.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: h.workspace,
		ContentHash:   contentHash,
		Title:         "Claude Code Session " + sessionID,
		Content:       md,
		SourcePath:    sessionFile,
		Collection:    "sessions",
		Tags:          tags,
		Metadata:      meta,
	})
	if err != nil {
		return false, fmt.Errorf("upsert document: %w", err)
	}

	if err := tq.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docRow.ID,
		WorkspaceHash: h.workspace,
	}); err != nil {
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
			return false, fmt.Errorf("upsert chunk: %w", err)
		}
		chunkIDs = append(chunkIDs, id)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit tx: %w", err)
	}

	if enqueuer != nil {
		for _, id := range chunkIDs {
			enqueuer.Enqueue(id)
		}
	}

	return true, nil
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

