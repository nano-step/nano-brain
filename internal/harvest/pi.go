package harvest

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// PiHarvester ingests Pi CLI agent JSONL session files into the document store.
type PiHarvester struct {
	db         *sql.DB
	logger     zerolog.Logger
	sessionDir string
	workspace  string
	summarizer SessionSummarizer
}

// WorkspaceHash returns the workspace hash this harvester was created for.
func (h *PiHarvester) WorkspaceHash() string { return h.workspace }

func (h *PiHarvester) setSummarizer(s SessionSummarizer) { h.summarizer = s }

func (h *PiHarvester) WithSummarizer(s SessionSummarizer) *PiHarvester {
	h.summarizer = s
	return h
}

// NewPiHarvester creates a new Pi session harvester.
func NewPiHarvester(db *sql.DB, logger zerolog.Logger, sessionDir, workspace string) *PiHarvester {
	return &PiHarvester{
		db:         db,
		logger:     logger.With().Str("component", "pi-harvester").Logger(),
		sessionDir: sessionDir,
		workspace:  workspace,
	}
}

// HarvestAll scans the session directory and ingests all JSONL sessions.
// Returns counts of harvested, skipped, and errored sessions.
func (h *PiHarvester) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
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
		Str("source", "pi").
		Int("summary_success", summarySuccess).
		Int("summary_fallback", summaryFallback).
		Int("skipped", skipped).
		Int("errors", errCount).
		Msg("harvest cycle complete")
	return
}

func (h *PiHarvester) harvestSession(ctx context.Context, sessionFile string, enqueuer ChunkEnqueuer) (harvestResult, error) {
	f, err := os.Open(sessionFile)
	if err != nil {
		return harvestSkipped, fmt.Errorf("open session file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	// The session's identity lives in the header line's "id" field, not the
	// filename (unlike Claude Code, whose filename IS the session ID) — so the
	// header must be read before any dedup check is possible. Only the header
	// is read at this point; a header-level failure discards the whole session
	// (there is no ID to key a partial harvest on), while a failure on any
	// later line skips only that line (see the event loop below).
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return harvestSkipped, fmt.Errorf("read session header: %w", err)
		}
		return harvestSkipped, nil // empty file
	}
	var header piSessionHeader
	if err := json.Unmarshal(scanner.Bytes(), &header); err != nil || header.ID == "" {
		return harvestSkipped, fmt.Errorf("parse session header: %w", err)
	}

	sessionID := header.ID
	sourcePath := piSourcePath(sessionID)

	queries := sqlc.New(h.db)
	existing, err := queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: h.workspace,
	})
	// A non-"no rows" DB error must NOT fall through to re-summarization — see
	// ClaudeCodeHarvester.harvestSession for the same guard and rationale.
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return harvestSkipped, fmt.Errorf("lookup existing summary: %w", err)
	}
	if err == nil && existing.ContentHash != "" {
		if dw, ok := h.summarizer.(interface {
			EnsureSummaryOnDisk(context.Context, string, SummaryMeta)
		}); ok {
			dw.EnsureSummaryOnDisk(ctx, existing.Content, SummaryMeta{
				Source:        "pi",
				SessionID:     sessionID,
				Title:         "Pi Session " + sessionID,
				CreatedAt:     existing.CreatedAt,
				WorkspaceHash: h.workspace,
			})
		}
		return harvestSkipped, nil
	}

	// Only now is the rest of the file's content needed.
	events, parseErr := parsePiEvents(scanner)
	if parseErr != nil {
		return harvestSkipped, fmt.Errorf("scan JSONL: %w", parseErr)
	}

	if len(events) == 0 {
		return harvestSkipped, nil
	}

	md := renderPiMarkdown(sessionID, events)

	sum := sha256.Sum256([]byte(md))
	contentHash := hex.EncodeToString(sum[:])

	createdAt := parsePiTimestamp(header.Timestamp)
	if createdAt.IsZero() {
		for _, ev := range events {
			if t := parsePiTimestamp(ev.Timestamp); !t.IsZero() {
				createdAt = t
				break
			}
		}
	}

	title := "Pi Session " + sessionID

	if h.summarizer != nil {
		smeta := SummaryMeta{
			Source:        "pi",
			SessionID:     sessionID,
			Title:         title,
			CreatedAt:     createdAt,
			WorkspaceHash: h.workspace,
		}
		if sumErr := h.summarizer.SummarizeAndPersist(ctx, md, smeta); sumErr != nil {
			h.logger.Warn().Err(sumErr).Str("session", sessionID).Msg("summarization failed, falling back to raw")
			if fbErr := h.writeRawFallback(ctx, sessionID, md, contentHash, title, sourcePath, len(events), enqueuer); fbErr != nil {
				return harvestSkipped, fmt.Errorf("raw fallback failed: %w", fbErr)
			}
			return harvestFallback, nil
		}
		return harvestSummary, nil
	}

	if fbErr := h.writeRawFallback(ctx, sessionID, md, contentHash, title, sourcePath, len(events), enqueuer); fbErr != nil {
		return harvestSkipped, fmt.Errorf("raw fallback failed: %w", fbErr)
	}
	return harvestFallback, nil
}

func (h *PiHarvester) writeRawFallback(
	ctx context.Context,
	sessionID, md, contentHash, title, sourcePath string,
	messageCount int,
	enqueuer ChunkEnqueuer,
) error {
	metadata := map[string]any{
		"source":        "pi",
		"session_id":    sessionID,
		"message_count": messageCount,
		"fallback":      true,
	}
	chunkCount, err := persistRawFallback(ctx, h.db, h.workspace, sourcePath, title, md, contentHash,
		[]string{"pi", "session", "fallback"}, metadata, enqueuer)
	if err != nil {
		return err
	}
	h.logger.Info().Str("session", sessionID).Bool("fallback", true).Int("chunks", chunkCount).Msg("raw fallback persisted")
	return nil
}

// piSourcePath returns the canonical source path for a Pi session document,
// matching internal/summarize/persist.go's buildSourcePath case for SourcePi.
func piSourcePath(sessionID string) string {
	return "summary://pi/" + sessionID
}

// piSessionHeader is the first line of a Pi JSONL session file.
type piSessionHeader struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
}

// piEvent is one subsequent line. Only type=="message" lines carry rendered
// content; "model_change"/"thinking_level_change" lines are metadata and are
// filtered out by parsePiEvents before rendering ever sees them.
type piEvent struct {
	Type      string    `json:"type"`
	Timestamp string    `json:"timestamp"`
	Message   piMessage `json:"message"`
}

// piMessage is the nested envelope on a type=="message" event. Role
// differentiation lives here, NOT on the top-level Type field: "user",
// "assistant", and "toolResult" are all distinct roles Pi actually emits —
// toolResult is not a sub-part of assistant content.
type piMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// piContentBlock is one element of a message's content array. Real Pi
// transcripts carry four block types: "text", "toolCall", "thinking", and
// "image" — a parser that only handles "text" silently drops roughly half of
// real transcript content (see openspec/changes/pi-session-harvesting/design.md).
type piContentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`      // set when Type=="text" or "thinking"
	Name      string          `json:"name"`      // set when Type=="toolCall"
	Arguments json.RawMessage `json:"arguments"` // set when Type=="toolCall"
}

// parsePiEvents reads the remaining lines of a Pi session file (the header
// line must already have been consumed by the caller). A line that fails to
// parse as JSON is skipped — matching ClaudeCodeHarvester.parseJSONLFile's
// per-line-skip behavior exactly — the rest of the session still renders.
func parsePiEvents(scanner *bufio.Scanner) ([]piEvent, error) {
	var events []piEvent
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev piEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed (or truncated) line, keep parsing
		}
		if ev.Type != "message" {
			continue // skip model_change/thinking_level_change
		}
		events = append(events, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func parsePiTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}

// renderPiMarkdown renders Pi session events into a markdown document. Tool
// calls and tool results are rendered using the same "Tool: <name>" /
// "Result: <text>" line conventions renderClaudeCodeMarkdown already uses, so
// that StripClaude's existing compaction regexes apply unchanged (see
// pipeline.go's Source switch and design.md Decision 5).
func renderPiMarkdown(sessionID string, events []piEvent) string {
	var b strings.Builder

	var createdAt string
	for _, ev := range events {
		if ev.Timestamp != "" {
			createdAt = ev.Timestamp
			break
		}
	}

	b.WriteString("---\n")
	fmt.Fprintf(&b, "session_id: %s\n", sessionID)
	b.WriteString("source: pi\n")
	fmt.Fprintf(&b, "message_count: %d\n", len(events))
	if createdAt != "" {
		fmt.Fprintf(&b, "created_at: %s\n", createdAt)
	}
	b.WriteString("---\n")

	for _, ev := range events {
		ts := ev.Timestamp
		switch ev.Message.Role {
		case "user", "assistant":
			label := "human"
			if ev.Message.Role == "assistant" {
				label = "assistant"
			}
			fmt.Fprintf(&b, "\n## %s (%s)\n\n", label, ts)
			b.WriteString(renderPiContent(ev.Message.Content))
			b.WriteString("\n")

		case "toolResult":
			fmt.Fprintf(&b, "\n## tool result (%s)\n\n", ts)
			if text := strings.TrimSpace(renderPiContent(ev.Message.Content)); text != "" {
				fmt.Fprintf(&b, "Result: %s\n", text)
			}
		}
	}

	return b.String()
}

// renderPiContent renders one message's content blocks to markdown.
func renderPiContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var blocks []piContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var b strings.Builder
	for _, blk := range blocks {
		switch blk.Type {
		case "text":
			if blk.Text != "" {
				if b.Len() > 0 {
					b.WriteString("\n")
				}
				b.WriteString(blk.Text)
			}
		case "toolCall":
			fmt.Fprintf(&b, "\nTool: %s\n", blk.Name)
			if len(blk.Arguments) > 0 {
				var argMap map[string]interface{}
				if json.Unmarshal(blk.Arguments, &argMap) == nil && argMap != nil {
					keys := make([]string, 0, len(argMap))
					for k := range argMap {
						keys = append(keys, k)
					}
					sort.Strings(keys)
					for _, k := range keys {
						fmt.Fprintf(&b, "%s: %v\n", k, argMap[k])
					}
				}
			}
		case "thinking":
			if blk.Text != "" {
				fmt.Fprintf(&b, "\nThinking:\n%s\n", blk.Text)
			}
		case "image":
			b.WriteString("\n[image omitted]\n")
		}
	}
	return b.String()
}
