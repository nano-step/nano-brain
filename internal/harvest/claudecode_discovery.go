package harvest

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// ClaudeCodeDiscoveryHarvester re-discovers Claude projects leaf directories
// on every harvest cycle. Unlike the original startup-only construction, first
// sessions, newly registered workspaces, and new profile-manager instances can
// be picked up without restarting the daemon. The resolver is fixed at daemon
// startup, so YAML config edits still require restart.
type ClaudeCodeDiscoveryHarvester struct {
	db             *sql.DB
	logger         zerolog.Logger
	resolveBaseDir func() []string
	summarizer     SessionSummarizer
}

// NewClaudeCodeDiscoveryHarvester constructs a dynamic multi-root Claude
// harvester. resolveBaseDirs is typically ClaudeCodeHarvesterConfig.
// SessionDirsResolved bound to the startup-loaded config.
func NewClaudeCodeDiscoveryHarvester(db *sql.DB, logger zerolog.Logger, resolveBaseDirs func() []string) *ClaudeCodeDiscoveryHarvester {
	return &ClaudeCodeDiscoveryHarvester{
		db:             db,
		logger:         logger.With().Str("component", "claudecode-discovery-harvester").Logger(),
		resolveBaseDir: resolveBaseDirs,
	}
}

func (h *ClaudeCodeDiscoveryHarvester) setSummarizer(s SessionSummarizer) { h.summarizer = s }

// HarvestAll implements Harvester.
func (h *ClaudeCodeDiscoveryHarvester) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
	if h.db == nil || h.resolveBaseDir == nil {
		return 0, 0, 0
	}

	baseDirs := existingClaudeBaseDirs(h.resolveBaseDir())
	if len(baseDirs) == 0 {
		return 0, 0, 0
	}

	q := sqlc.New(h.db)
	workspaces, err := q.ListWorkspaces(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("claude discovery: failed to list workspaces")
		return 0, 0, 1
	}
	if len(workspaces) == 0 {
		return 0, 0, 0
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].Path < workspaces[j].Path })

	seenLeaves := make(map[string]struct{})
	for _, baseDir := range baseDirs {
		for _, ws := range workspaces {
			for _, workspaceCandidate := range claudeWorkspacePathCandidates(ws.Path) {
				leafDir := filepath.Join(baseDir, EncodeClaudeProjectsDir(workspaceCandidate))
				info, statErr := os.Stat(leafDir)
				if statErr != nil || !info.IsDir() {
					continue
				}
				key := canonicalFilesystemPath(leafDir) + "\x00" + ws.Hash
				if _, dup := seenLeaves[key]; dup {
					continue
				}
				seenLeaves[key] = struct{}{}

				leaf := NewClaudeCodeHarvesterWithWorkspacePath(h.db, h.logger, leafDir, ws.Hash, ws.Path)
				leaf.setSummarizer(h.summarizer)
				harvestedN, skippedN, errN := leaf.HarvestAll(ctx, enqueuer)
				harvested += harvestedN
				skipped += skippedN
				errCount += errN
			}
		}
	}
	return harvested, skipped, errCount
}

func claudeWorkspacePathCandidates(workspacePath string) []string {
	candidates := []string{workspacePath}
	canonical := canonicalFilesystemPath(workspacePath)
	if canonical != filepath.Clean(workspacePath) {
		candidates = append(candidates, canonical)
	}
	return candidates
}

func existingClaudeBaseDirs(candidates []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() {
			continue
		}
		key := canonicalFilesystemPath(candidate)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, filepath.Clean(candidate))
	}
	return out
}
