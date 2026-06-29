package harvest

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

// Compile-time assertion: OpenCodeSource implements SessionSource.
var _ SessionSource = (*OpenCodeSource)(nil)

// OpenCodeSource is a SessionSource adapter for OpenCode SQLite databases.
// It delegates discovery to ScanOpenCodeDBRoot and reading to the existing
// listSessions/listMessages helpers on OpenCodeSQLiteHarvester.
type OpenCodeSource struct {
	root   string        // parent directory scanned by ScanOpenCodeDBRoot
	logger zerolog.Logger
}

// NewOpenCodeSource constructs an OpenCodeSource.
// root is the parent directory of per-project opencode.db files
// (the directory that ScanOpenCodeDBRoot scans).
func NewOpenCodeSource(root string, logger zerolog.Logger) *OpenCodeSource {
	return &OpenCodeSource{
		root:   root,
		logger: logger.With().Str("component", "opencode-source").Logger(),
	}
}

// Name returns the source identifier.
func (s *OpenCodeSource) Name() string { return "opencode" }

// Discover scans the root directory for opencode.db files, matches them
// against registered workspaces, and returns one Location per matched DB.
func (s *OpenCodeSource) Discover(ctx context.Context, registered map[string]string) ([]Location, error) {
	discovered := ScanOpenCodeDBRoot(ctx, s.root, registered, s.logger)
	locs := make([]Location, 0, len(discovered))
	for _, d := range discovered {
		locs = append(locs, Location{
			DBPath:        d.DBPath,
			WorkspaceHash: d.WorkspaceHash,
			WorktreePath:  d.Worktree,
		})
	}
	return locs, nil
}

// Read opens the SQLite DB at loc.DBPath, lists all sessions and their
// messages, and returns them as NormalizedSession values.
func (s *OpenCodeSource) Read(ctx context.Context, loc Location) ([]NormalizedSession, error) {
	db, err := sql.Open("sqlite", loc.DBPath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("opencode-source: open sqlite %s: %w", loc.DBPath, err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("opencode-source: ping sqlite %s: %w", loc.DBPath, err)
	}

	// Use a temporary harvester instance to reuse the existing list helpers.
	h := &OpenCodeSQLiteHarvester{logger: s.logger}
	sqSessions, err := h.listSessions(ctx, db, nil)
	if err != nil {
		return nil, fmt.Errorf("opencode-source: list sessions: %w", err)
	}

	var out []NormalizedSession
	for _, sess := range sqSessions {
		msgs, err := h.listMessages(ctx, db, sess.ID)
		if err != nil {
			s.logger.Warn().Err(err).Str("session_id", sess.ID).Msg("opencode-source: list messages failed, skipping")
			continue
		}

		normMsgs := make([]NormalizedMessage, 0, len(msgs))
		for _, m := range msgs {
			normMsgs = append(normMsgs, NormalizedMessage{
				Role:      m.role,
				Content:   m.content,
				Timestamp: m.createdAt,
			})
		}

		out = append(out, NormalizedSession{
			Source:        s.Name(),
			SessionID:     sess.ID,
			WorkspaceHash: loc.WorkspaceHash,
			Cwd:           loc.WorktreePath,
			Title:         sess.Title,
			CreatedAt:     sess.CreatedAt,
			UpdatedAt:     sess.UpdatedAt,
			Messages:      normMsgs,
		})
	}
	return out, nil
}
