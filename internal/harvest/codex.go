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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// CodexHarvester ingests Codex CLI rollout JSONL session files into the
// document store. Unlike ClaudeCodeHarvester/PiHarvester (one instance per
// workspace directory), Codex keeps a single global session tree under
// $CODEX_HOME, so one harvester scans it and attributes each session to a
// registered workspace via session_meta.payload.cwd.
type CodexHarvester struct {
	db         *sql.DB
	logger     zerolog.Logger
	codexHome  string
	summarizer SessionSummarizer
}

func (h *CodexHarvester) setSummarizer(s SessionSummarizer) { h.summarizer = s }

func (h *CodexHarvester) WithSummarizer(s SessionSummarizer) *CodexHarvester {
	h.summarizer = s
	return h
}

// NewCodexHarvester creates a new Codex session harvester rooted at codexHome
// (typically ~/.codex or $CODEX_HOME).
func NewCodexHarvester(db *sql.DB, logger zerolog.Logger, codexHome string) *CodexHarvester {
	return &CodexHarvester{
		db:        db,
		logger:    logger.With().Str("component", "codex-harvester").Logger(),
		codexHome: codexHome,
	}
}

// HarvestAll scans <codexHome>/sessions (recursive) and
// <codexHome>/archived_sessions (flat) for rollout-*.jsonl files and ingests
// each attributed session. Returns counts of harvested, skipped, and errored
// sessions.
func (h *CodexHarvester) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
	sessionsDir := filepath.Join(h.codexHome, "sessions")
	archivedDir := filepath.Join(h.codexHome, "archived_sessions")

	files, err := collectCodexRollouts(sessionsDir, archivedDir)
	if err != nil {
		h.logger.Error().Err(err).Str("codex_home", h.codexHome).Msg("failed to list codex session files")
		return 0, 0, 1
	}
	if len(files) == 0 {
		h.logger.Debug().Str("codex_home", h.codexHome).Msg("no codex rollout files found")
		return 0, 0, 0
	}

	// Workspace registry loaded once per cycle — sessions are attributed
	// per-file against this map.
	q := sqlc.New(h.db)
	workspaces, wsErr := q.ListWorkspaces(ctx)
	if wsErr != nil {
		h.logger.Error().Err(wsErr).Msg("codex harvest: failed to list registered workspaces")
		return 0, 0, 1
	}
	if len(workspaces) == 0 {
		h.logger.Warn().Msg("codex harvest: no registered workspaces, skipping")
		return 0, 0, 0
	}
	wsCtx := newCodexWorkspaceCtx(workspaces)

	var summarySuccess, summaryFallback int
	for _, path := range files {
		res, err := h.harvestSession(ctx, path, enqueuer, q, wsCtx)
		if err != nil {
			h.logger.Error().Err(err).Str("file", path).Msg("failed to harvest codex session")
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
		Str("source", "codex").
		Int("files", len(files)).
		Int("summary_success", summarySuccess).
		Int("summary_fallback", summaryFallback).
		Int("skipped", skipped).
		Int("errors", errCount).
		Msg("harvest cycle complete")
	return
}

// collectCodexRollouts returns sorted paths of *.jsonl rollout files under
// sessionsDir (walked recursively — Codex partitions by YYYY/MM/DD) and
// archivedDir (flat). .jsonl.zst files never match the ".jsonl" suffix and are
// skipped implicitly. Missing directories contribute zero files.
func collectCodexRollouts(sessionsDir, archivedDir string) ([]string, error) {
	var files []string

	if info, err := os.Stat(sessionsDir); err == nil && info.IsDir() {
		err := filepath.WalkDir(sessionsDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(d.Name(), ".jsonl") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk sessions dir: %w", err)
		}
	}

	if info, err := os.Stat(archivedDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(archivedDir)
		if err != nil {
			return nil, fmt.Errorf("read archived_sessions dir: %w", err)
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
				files = append(files, filepath.Join(archivedDir, e.Name()))
			}
		}
	}

	sort.Strings(files)
	return files, nil
}

// codexWorkspaceCtx holds the per-cycle workspace registry used to attribute
// sessions. paths is sorted longest-first so the first ancestor match wins.
type codexWorkspaceCtx struct {
	byPath map[string]string // filepath.Clean(ws.Path) -> ws.Hash
	paths  []string          // cleaned workspace paths, longest first
	hashes map[string]bool   // registered workspace hashes (for hash-check fallback)
}

func newCodexWorkspaceCtx(workspaces []sqlc.Workspace) *codexWorkspaceCtx {
	ctx := &codexWorkspaceCtx{
		byPath: make(map[string]string, len(workspaces)),
		paths:  make([]string, 0, len(workspaces)),
		hashes: make(map[string]bool, len(workspaces)),
	}
	for _, ws := range workspaces {
		clean := filepath.Clean(ws.Path)
		ctx.byPath[clean] = ws.Hash
		ctx.paths = append(ctx.paths, clean)
		ctx.hashes[ws.Hash] = true
	}
	sort.Slice(ctx.paths, func(i, j int) bool { return len(ctx.paths[i]) > len(ctx.paths[j]) })
	return ctx
}

// resolve attributes a session cwd to a registered workspace hash. Order:
// exact match, longest-ancestor match (Codex often runs in a subdirectory of
// the registered workspace), then WorkspaceHash existence check for path
// variants Clean missed. Returns ok=false when unattributable.
func (c *codexWorkspaceCtx) resolve(cwd string) (string, bool) {
	if cwd == "" {
		return "", false
	}
	clean := filepath.Clean(cwd)
	if h, ok := c.byPath[clean]; ok {
		return h, true
	}
	sep := string(os.PathSeparator)
	for _, p := range c.paths {
		if strings.HasPrefix(clean, p+sep) {
			return c.byPath[p], true
		}
	}
	hash, err := storage.WorkspaceHash(clean)
	if err != nil {
		return "", false
	}
	if c.hashes[hash] {
		return hash, true
	}
	return "", false
}

// harvestSession ingests one rollout file. The session_meta record supplies
// id+cwd; it is located by scanning at most codexMetaScanLines lines before
// the dedup decision, then the same scanner continues into the render pass.
func (h *CodexHarvester) harvestSession(
	ctx context.Context,
	path string,
	enqueuer ChunkEnqueuer,
	q *sqlc.Queries,
	wsCtx *codexWorkspaceCtx,
) (harvestResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return harvestSkipped, fmt.Errorf("open rollout file: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line

	meta, found, err := scanCodexSessionMeta(scanner)
	if err != nil {
		return harvestSkipped, fmt.Errorf("scan session_meta: %w", err)
	}
	if !found {
		return harvestSkipped, fmt.Errorf("no session_meta record in first %d lines", codexMetaScanLines)
	}

	sessionID := sanitizeID(meta.ID)
	if sessionID == "" {
		return harvestSkipped, fmt.Errorf("session_meta has empty id")
	}

	wsHash, ok := wsCtx.resolve(meta.Cwd)
	if !ok {
		h.logger.Debug().
			Str("file", path).
			Str("cwd", meta.Cwd).
			Str("session", sessionID).
			Msg("codex harvest: skipping session for unregistered workspace")
		return harvestSkipped, nil
	}

	sourcePath := codexSourcePath(sessionID)
	existing, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    sourcePath,
		WorkspaceHash: wsHash,
	})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return harvestSkipped, fmt.Errorf("lookup existing summary: %w", err)
	}
	if err == nil && existing.ContentHash != "" {
		if dw, ok := h.summarizer.(interface {
			EnsureSummaryOnDisk(context.Context, string, SummaryMeta)
		}); ok {
			dw.EnsureSummaryOnDisk(ctx, existing.Content, sanitizeSummaryMeta(SummaryMeta{
				Source:        "codex",
				SessionID:     sessionID,
				Title:         "Codex Session " + sessionID,
				CreatedAt:     existing.CreatedAt,
				WorkspaceHash: wsHash,
			}))
		}
		return harvestSkipped, nil
	}

	// Render pass: continue the same scanner through the rest of the file.
	items, err := parseCodexResponseItems(scanner)
	if err != nil {
		return harvestSkipped, fmt.Errorf("scan rollout items: %w", err)
	}
	if len(items) == 0 {
		return harvestSkipped, nil
	}

	md := renderCodexMarkdown(sessionID, meta, items)
	sum := sha256.Sum256([]byte(md))
	contentHash := hex.EncodeToString(sum[:])

	createdAt := parseCodexTimestamp(meta.Timestamp)
	if createdAt.IsZero() {
		for _, it := range items {
			if t := parseCodexTimestamp(it.Timestamp); !t.IsZero() {
				createdAt = t
				break
			}
		}
	}

	title := "Codex Session " + sessionID

	if h.summarizer != nil {
		smeta := sanitizeSummaryMeta(SummaryMeta{
			Source:        "codex",
			SessionID:     sessionID,
			Title:         title,
			CreatedAt:     createdAt,
			Cwd:           meta.Cwd,
			WorkspaceHash: wsHash,
		})
		if sumErr := h.summarizer.SummarizeAndPersist(ctx, md, smeta); sumErr != nil {
			h.logger.Warn().Err(sumErr).Str("session", sessionID).Msg("codex summarization failed, falling back to raw")
			if fbErr := h.writeRawFallback(ctx, wsHash, sessionID, md, contentHash, title, sourcePath, len(items), enqueuer); fbErr != nil {
				return harvestSkipped, fmt.Errorf("raw fallback failed: %w", fbErr)
			}
			return harvestFallback, nil
		}
		return harvestSummary, nil
	}

	if fbErr := h.writeRawFallback(ctx, wsHash, sessionID, md, contentHash, title, sourcePath, len(items), enqueuer); fbErr != nil {
		return harvestSkipped, fmt.Errorf("raw fallback failed: %w", fbErr)
	}
	return harvestFallback, nil
}

func (h *CodexHarvester) writeRawFallback(
	ctx context.Context,
	wsHash, sessionID, md, contentHash, title, sourcePath string,
	itemCount int,
	enqueuer ChunkEnqueuer,
) error {
	metadata := map[string]any{
		"source":     "codex",
		"session_id": sessionID,
		"item_count": itemCount,
		"fallback":   true,
	}
	chunkCount, err := persistRawFallback(ctx, h.db, wsHash, sourcePath, title, md, contentHash,
		[]string{"codex", "session", "fallback"}, metadata, enqueuer)
	if err != nil {
		return err
	}
	h.logger.Info().Str("session", sessionID).Bool("fallback", true).Int("chunks", chunkCount).Msg("codex raw fallback persisted")
	return nil
}

// codexSourcePath returns the canonical source path for a Codex session,
// matching internal/summarize/persist.go's buildSourcePath case for SourceCodex.
func codexSourcePath(sessionID string) string {
	return "summary://codex/" + sessionID
}

const codexMetaScanLines = 256

// codexSessionMeta is the session_meta record's payload — the only rollout
// record carrying the session's stable identity and working directory.
type codexSessionMeta struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Cwd       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
}

// codexEnvelope is one rollout line. Only type=="response_item" lines carry
// rendered conversation content; session_meta is consumed by the meta scan;
// event_msg/world_state/turn_context/token_usage_record are telemetry/UI
// state and never render.
type codexEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// scanCodexSessionMeta reads lines until the first session_meta record or
// codexMetaScanLines lines, whichever comes first. Malformed lines are skipped
// per-line (matching parseJSONLFile semantics). The scanner is left positioned
// for the caller's render pass.
func scanCodexSessionMeta(scanner *bufio.Scanner) (codexSessionMeta, bool, error) {
	var meta codexSessionMeta
	for i := 0; i < codexMetaScanLines && scanner.Scan(); i++ {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var env codexEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		if env.Type != "session_meta" {
			continue
		}
		var p codexSessionMeta
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			continue
		}
		if p.ID == "" {
			p.ID = p.SessionID
		}
		if p.ID == "" {
			return meta, false, nil
		}
		return p, true, nil
	}
	if err := scanner.Err(); err != nil {
		return meta, false, err
	}
	return meta, false, nil
}

// codexItem is one rendered conversation unit in file order. kind selects the
// renderer: "user"/"assistant" message text, "tool" call, or "result" output.
type codexItem struct {
	Kind      string
	Timestamp string
	Role      string // for kind=="user"/"assistant" header label
	Name      string // tool name for kind=="tool"
	Text      string // message text / tool input / tool output
	CallID    string // for pairing result -> tool
}

// parseCodexResponseItems continues scanning a rollout file (positioned after
// session_meta) and returns renderable items in file order. Only
// type=="response_item" envelopes with renderable payload types produce
// items; everything else is skipped. Malformed lines are skipped per-line.
func parseCodexResponseItems(scanner *bufio.Scanner) ([]codexItem, error) {
	var items []codexItem
	pending := make(map[string]int) // call_id -> index into items of its Tool: block

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var env codexEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			continue
		}
		if env.Type != "response_item" {
			continue
		}

		var head struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(env.Payload, &head); err != nil {
			continue
		}

		switch head.Type {
		case "message":
			it, ok := parseCodexMessage(env.Payload, env.Timestamp)
			if ok {
				items = append(items, it)
			}
		case "function_call", "custom_tool_call":
			it, ok := parseCodexToolCall(env.Payload, env.Timestamp)
			if ok {
				pending[it.CallID] = len(items)
				items = append(items, it)
			}
		case "function_call_output", "custom_tool_call_output":
			it, ok := parseCodexToolOutput(env.Payload, env.Timestamp)
			if ok {
				// Pair the Result: line directly under its Tool: block when the
				// call was seen earlier in this file; otherwise emit standalone.
				if idx, paired := pending[it.CallID]; paired {
					items[idx].Text += "\nResult: " + it.Text
					delete(pending, it.CallID)
				} else {
					items = append(items, codexItem{Kind: "result", Timestamp: it.Timestamp, Text: it.Text})
				}
			}
		case "web_search_call":
			it, ok := parseCodexWebSearch(env.Payload, env.Timestamp)
			if ok {
				items = append(items, it)
			}
		default:
			// reasoning (encrypted_content), additional_tools, and any future
			// payload types are intentionally not rendered.
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// codexMessagePayload mirrors the response_item message shape. Content blocks
// carry type "input_text" (user/developer) or "output_text" (assistant).
type codexMessagePayload struct {
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func parseCodexMessage(payload json.RawMessage, ts string) (codexItem, bool) {
	var m codexMessagePayload
	if err := json.Unmarshal(payload, &m); err != nil {
		return codexItem{}, false
	}
	// developer = system prompts, agent_message = subagent internals — both
	// excluded, matching the spec's render table.
	if m.Role != "user" && m.Role != "assistant" {
		return codexItem{}, false
	}
	var texts []string
	for _, b := range m.Content {
		if (b.Type == "input_text" || b.Type == "output_text") && b.Text != "" {
			texts = append(texts, b.Text)
		}
	}
	if len(texts) == 0 {
		return codexItem{}, false
	}
	return codexItem{Kind: m.Role, Timestamp: ts, Role: m.Role, Text: strings.Join(texts, "\n\n")}, true
}

// codexToolCallPayload covers both function_call (arguments: JSON string) and
// custom_tool_call (input: raw string) — Codex emits both depending on tool
// type. call_id links the paired output record.
type codexToolCallPayload struct {
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Input     string `json:"input"`
}

func parseCodexToolCall(payload json.RawMessage, ts string) (codexItem, bool) {
	var c codexToolCallPayload
	if err := json.Unmarshal(payload, &c); err != nil {
		return codexItem{}, false
	}
	body := c.Arguments
	if body == "" {
		body = c.Input
	}
	return codexItem{Kind: "tool", Timestamp: ts, Name: c.Name, Text: body, CallID: c.CallID}, true
}

// codexToolOutputPayload covers function_call_output (output: string) and
// custom_tool_call_output (output: array of {type,text} blocks).
type codexToolOutputPayload struct {
	CallID string          `json:"call_id"`
	Output json.RawMessage `json:"output"`
}

func parseCodexToolOutput(payload json.RawMessage, ts string) (codexItem, bool) {
	var o codexToolOutputPayload
	if err := json.Unmarshal(payload, &o); err != nil {
		return codexItem{}, false
	}
	text := codexOutputText(o.Output)
	if text == "" {
		return codexItem{}, false
	}
	return codexItem{Kind: "result", Timestamp: ts, Text: text, CallID: o.CallID}, true
}

// codexOutputText normalizes the two output shapes: a plain string
// (function_call_output) or an array of content blocks
// (custom_tool_call_output).
func codexOutputText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var texts []string
	for _, b := range blocks {
		if b.Text != "" {
			texts = append(texts, b.Text)
		}
	}
	return strings.Join(texts, "\n")
}

type codexWebSearchPayload struct {
	CallID string `json:"call_id"`
	Action struct {
		Type  string `json:"type"`
		Query string `json:"query"`
	} `json:"action"`
}

func parseCodexWebSearch(payload json.RawMessage, ts string) (codexItem, bool) {
	var w codexWebSearchPayload
	if err := json.Unmarshal(payload, &w); err != nil {
		return codexItem{}, false
	}
	name := "web_search"
	body := w.Action.Query
	if body == "" {
		body = w.Action.Type
	}
	return codexItem{Kind: "tool", Timestamp: ts, Name: name, Text: body, CallID: w.CallID}, true
}

func parseCodexTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return time.Time{}
	}
	return t
}

// renderCodexMarkdown renders rollout items into the same markdown shape
// ClaudeCodeHarvester/PiHarvester emit — "## <role> (<ts>)" sections plus
// "Tool: <name>" bodies — so StripClaude's compaction regexes apply unchanged.
func renderCodexMarkdown(sessionID string, meta codexSessionMeta, items []codexItem) string {
	var b strings.Builder

	b.WriteString("---\n")
	fmt.Fprintf(&b, "session_id: %s\n", sessionID)
	b.WriteString("source: codex\n")
	if meta.Cwd != "" {
		fmt.Fprintf(&b, "cwd: %s\n", meta.Cwd)
	}
	if meta.Timestamp != "" {
		fmt.Fprintf(&b, "created_at: %s\n", meta.Timestamp)
	}
	b.WriteString("---\n\n")

	for _, it := range items {
		switch it.Kind {
		case "user", "assistant":
			fmt.Fprintf(&b, "## %s (%s)\n\n%s\n\n", it.Role, it.Timestamp, it.Text)
		case "tool":
			name := it.Name
			if name == "" {
				name = "tool"
			}
			fmt.Fprintf(&b, "Tool: %s\n%s\n\n", name, it.Text)
		case "result":
			fmt.Fprintf(&b, "Result: %s\n\n", it.Text)
		}
	}
	return b.String()
}
