package harvest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Compile-time assertion: ClaudeSource implements SessionSource.
var _ SessionSource = (*ClaudeSource)(nil)

// ClaudeSource is a SessionSource adapter for Claude Code JSONL session files.
type ClaudeSource struct {
	sessionDir string // directory containing *.jsonl session files
	logger     zerolog.Logger
}

// NewClaudeSource constructs a ClaudeSource.
func NewClaudeSource(sessionDir string, logger zerolog.Logger) *ClaudeSource {
	return &ClaudeSource{
		sessionDir: sessionDir,
		logger:     logger.With().Str("component", "claude-source").Logger(),
	}
}

// Name returns the source identifier.
func (s *ClaudeSource) Name() string { return "claude" }

// Discover scans the sessionDir for *.jsonl files and returns one Location
// per file. The WorkspaceHash is left empty here — the Engine or caller must
// resolve it from the first message's cwd/gitBranch fields or via registration.
func (s *ClaudeSource) Discover(_ context.Context, _ map[string]string) ([]Location, error) {
	if s.sessionDir == "" {
		return nil, nil
	}
	if _, err := os.Stat(s.sessionDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(s.sessionDir)
	if err != nil {
		return nil, err
	}

	var locs []Location
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		locs = append(locs, Location{
			SessionDir: filepath.Join(s.sessionDir, e.Name()),
		})
	}
	return locs, nil
}

// Read parses the JSONL file at loc.SessionDir and returns a single
// NormalizedSession. Branch and Cwd are populated from the per-record
// gitBranch and cwd fields when present.
func (s *ClaudeSource) Read(_ context.Context, loc Location) ([]NormalizedSession, error) {
	msgs, err := parseJSONLFile(loc.SessionDir)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, nil
	}

	sessionID := strings.TrimSuffix(filepath.Base(loc.SessionDir), ".jsonl")

	var createdAt time.Time
	if msgs[0].Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, msgs[0].Timestamp); err == nil {
			createdAt = t
		}
	}

	// Derive Branch and Cwd from the first message that has them.
	var branch, cwd string
	for _, m := range msgs {
		if branch == "" && m.GitBranch != "" {
			branch = m.GitBranch
		}
		if cwd == "" && m.Cwd != "" {
			cwd = m.Cwd
		}
		if branch != "" && cwd != "" {
			break
		}
	}

	normMsgs := make([]NormalizedMessage, 0, len(msgs))
	for _, m := range msgs {
		var ts time.Time
		if m.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, m.Timestamp); err == nil {
				ts = t
			}
		}
		role := m.Type
		if role == "user" {
			role = "human"
		}
		normMsgs = append(normMsgs, NormalizedMessage{
			Role:        role,
			Content:     m.Content,
			Timestamp:   ts,
			ToolName:    m.ToolName,
			IsSidechain: m.IsSidechain,
		})
	}

	sess := NormalizedSession{
		Source:        s.Name(),
		SessionID:     sessionID,
		WorkspaceHash: loc.WorkspaceHash,
		Branch:        branch,
		Cwd:           cwd,
		Title:         "Claude Code Session " + sessionID,
		CreatedAt:     createdAt,
		Messages:      normMsgs,
	}
	return []NormalizedSession{sess}, nil
}
