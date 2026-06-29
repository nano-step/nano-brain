package harvest

import "context"

// Location describes a single discovered session store.
type Location struct {
	DBPath        string // absolute path to SQLite DB (opencode only)
	SessionDir    string // path to session directory or JSONL file (claude only)
	WorkspaceHash string // nano-brain workspace hash this location belongs to
	WorktreePath  string // project worktree path on disk
}

// SessionSource is the pluggable adapter interface for session harvesters.
// Each agent type (OpenCode, Claude Code, etc.) implements this interface
// so the generic Engine can drive discovery, reading, and normalization
// without per-source duplication.
type SessionSource interface {
	// Name returns the source identifier used in source paths and metadata
	// (e.g. "opencode", "claude").
	Name() string

	// Discover scans for session stores that match the registered workspaces.
	// registered is a map of worktree path → workspace hash.
	// Returns one Location per discovered session store.
	Discover(ctx context.Context, registered map[string]string) ([]Location, error)

	// Read reads and parses all sessions from a single discovered Location,
	// returning them as normalized session objects.
	Read(ctx context.Context, loc Location) ([]NormalizedSession, error)
}
