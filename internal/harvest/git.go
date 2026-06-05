package harvest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

// GitHarvester indexes git commit history into the git-history collection.
// V1 scope: commit messages + changed file list only (no diffs, no blame).
// Incremental via tracking last-harvested SHA in a marker document.
type GitHarvester struct {
	db          *sql.DB
	queries     *sqlc.Queries
	logger      zerolog.Logger
	workspaces  map[string]string // path → workspace hash
	maxCommits  int
	excludeBots bool
}

// gitCommit represents a single git commit.
type gitCommit struct {
	SHA     string
	Author  string
	Email   string
	Date    time.Time
	Subject string
	Body    string
	Files   []string
}

// NewGitHarvester creates a git history harvester for the given workspaces.
// workspaces maps repository root paths to workspace hashes.
func NewGitHarvester(
	db *sql.DB,
	logger zerolog.Logger,
	workspaces map[string]string,
	maxCommits int,
	excludeBots bool,
) *GitHarvester {
	return &GitHarvester{
		db:          db,
		queries:     sqlc.New(db),
		logger:      logger.With().Str("component", "git-harvester").Logger(),
		workspaces:  workspaces,
		maxCommits:  maxCommits,
		excludeBots: excludeBots,
	}
}

// HarvestAll scans all registered workspace repos and indexes new commits.
// Returns counts of harvested, skipped, and errored commits.
func (g *GitHarvester) HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int) {
	for repoPath, workspaceHash := range g.workspaces {
		h, s, err := g.harvestRepo(ctx, repoPath, workspaceHash, enqueuer)
		harvested += h
		skipped += s
		if err != nil {
			g.logger.Error().
				Err(err).
				Str("repo", repoPath).
				Str("workspace", workspaceHash).
				Msg("failed to harvest repo")
			errCount++
		}
	}
	return
}

// harvestRepo processes a single git repository.
func (g *GitHarvester) harvestRepo(
	ctx context.Context,
	repoPath string,
	workspaceHash string,
	enqueuer ChunkEnqueuer,
) (int, int, error) {
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		g.logger.Debug().Str("path", repoPath).Msg("not a git repo, skipping")
		return 0, 0, nil
	}

	lastSHA, err := g.getLastHarvestedSHA(ctx, workspaceHash)
	if err != nil {
		return 0, 0, fmt.Errorf("get last SHA: %w", err)
	}

	commits, err := g.gitLog(repoPath, lastSHA, g.maxCommits)
	if err != nil {
		return 0, 0, fmt.Errorf("git log: %w", err)
	}

	if len(commits) == 0 {
		g.logger.Debug().Str("repo", repoPath).Msg("no new commits")
		return 0, 0, nil
	}

	harvested := 0
	skipped := 0

	for _, commit := range commits {
		if g.excludeBots && isBotCommit(commit.Author, commit.Email) {
			g.logger.Debug().
				Str("sha", commit.SHA).
				Str("author", commit.Author).
				Msg("skipping bot commit")
			skipped++
			continue
		}

		md := renderCommitMarkdown(commit)
		sum := sha256.Sum256([]byte(md))
		contentHash := hex.EncodeToString(sum[:])

		sourcePath := fmt.Sprintf("git://commit/%s", commit.SHA)
		existing, err := g.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
			SourcePath:    sourcePath,
			WorkspaceHash: workspaceHash,
		})
		if err == nil && existing.ContentHash == contentHash {
			skipped++
			continue
		}

		if err := g.upsertCommit(ctx, workspaceHash, commit, md, contentHash, sourcePath, enqueuer); err != nil {
			g.logger.Error().
				Err(err).
				Str("sha", commit.SHA).
				Msg("failed to upsert commit")
			return harvested, skipped, fmt.Errorf("upsert commit %s: %w", commit.SHA, err)
		}

		harvested++
	}

	if len(commits) > 0 {
		latestSHA := commits[len(commits)-1].SHA
		if err := g.updateLastHarvestedSHA(ctx, workspaceHash, latestSHA); err != nil {
			return harvested, skipped, fmt.Errorf("update marker: %w", err)
		}
	}

	return harvested, skipped, nil
}

func (g *GitHarvester) upsertCommit(
	ctx context.Context,
	workspaceHash string,
	commit gitCommit,
	md string,
	contentHash string,
	sourcePath string,
	enqueuer ChunkEnqueuer,
) error {
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	tq := sqlc.New(tx)
	meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
	tags := []string{"git", "commit"}
	semanticTags := InferSemanticTags(md, commit.Subject)
	tags = append(tags, semanticTags...)

	docRow, err := tq.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   contentHash,
		Title:         commit.Subject,
		Content:       md,
		SourcePath:    sourcePath,
		Collection:    "git-history",
		Tags:          tags,
		Metadata:      meta,
	})
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}

	if err := tq.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docRow.ID,
		WorkspaceHash: workspaceHash,
	}); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}

	chunks := chunk.Split(md, chunk.DefaultConfig())
	chunkMeta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
	chunkIDs := make([]uuid.UUID, 0, len(chunks))

	for _, ch := range chunks {
		id, err := tq.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docRow.ID,
			WorkspaceHash:     workspaceHash,
			ContentHash:       ch.Hash,
			Content:           ch.Content,
			ChunkIndex:        int32(ch.Sequence),
			StartLine:         sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
			EndLine:           sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
			Metadata:          chunkMeta,
			ChunkType:         "raw",
			EmbeddingStrategy: "raw_code",
		})
		if err != nil {
			return fmt.Errorf("upsert chunk: %w", err)
		}
		chunkIDs = append(chunkIDs, id)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	if enqueuer != nil {
		for _, id := range chunkIDs {
			enqueuer.Enqueue(id)
		}
	}

	return nil
}

func (g *GitHarvester) getLastHarvestedSHA(ctx context.Context, workspaceHash string) (string, error) {
	markerPath := "git://meta/last-commit"
	doc, err := g.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    markerPath,
		WorkspaceHash: workspaceHash,
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("get marker doc: %w", err)
	}
	return strings.TrimSpace(doc.Content), nil
}

func (g *GitHarvester) updateLastHarvestedSHA(ctx context.Context, workspaceHash, sha string) error {
	markerPath := "git://meta/last-commit"
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(sha)))
	meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}

	_, err := g.queries.UpsertDocumentBySourcePath(ctx, sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   contentHash,
		Title:         "Last Harvested Commit",
		Content:       sha,
		SourcePath:    markerPath,
		Collection:    "git-history",
		Tags:          []string{"git", "meta"},
		Metadata:      meta,
	})
	if err != nil {
		return fmt.Errorf("upsert marker: %w", err)
	}
	return nil
}

func (g *GitHarvester) gitLog(repoPath string, since string, maxCommits int) ([]gitCommit, error) {
	args := []string{
		"log",
		"--format=%x1f%H%x1f%an%x1f%ae%x1f%aI%x1f%s%x1f%b%x1f",
		"--name-only",
	}

	if maxCommits > 0 {
		args = append(args, fmt.Sprintf("-n%d", maxCommits))
	}

	if since != "" {
		args = append(args, fmt.Sprintf("%s..HEAD", since))
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("git log failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return parseGitLog(string(output))
}

func parseGitLog(output string) ([]gitCommit, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	commits := []gitCommit{}
	entries := strings.Split(output, "\x1f\x1f")

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.Split(entry, "\x1f")
		if len(parts) < 8 {
			continue
		}

		sha := strings.TrimSpace(parts[1])
		author := strings.TrimSpace(parts[2])
		email := strings.TrimSpace(parts[3])
		dateStr := strings.TrimSpace(parts[4])
		subject := strings.TrimSpace(parts[5])
		body := strings.TrimSpace(parts[6])
		filesStr := strings.TrimSpace(parts[7])

		if sha == "" {
			continue
		}

		date, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			date = time.Now()
		}

		var files []string
		if filesStr != "" {
			for _, f := range strings.Split(filesStr, "\n") {
				f = strings.TrimSpace(f)
				if f != "" {
					files = append(files, f)
				}
			}
		}

		commits = append(commits, gitCommit{
			SHA:     sha,
			Author:  author,
			Email:   email,
			Date:    date,
			Subject: subject,
			Body:    body,
			Files:   files,
		})
	}

	return commits, nil
}

func renderCommitMarkdown(c gitCommit) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", c.Subject)
	fmt.Fprintf(&b, "**Author:** %s <%s>\n", c.Author, c.Email)
	fmt.Fprintf(&b, "**Date:** %s\n", c.Date.Format(time.RFC3339))
	fmt.Fprintf(&b, "**SHA:** %s\n\n", c.SHA)

	if c.Body != "" {
		b.WriteString(c.Body)
		b.WriteString("\n\n")
	}

	if len(c.Files) > 0 {
		b.WriteString("## Changed Files\n\n")
		for _, f := range c.Files {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}

	return b.String()
}

func isBotCommit(author, email string) bool {
	botPatterns := []string{
		"dependabot",
		"renovate",
		"github-actions",
		"[bot]",
	}

	lowerAuthor := strings.ToLower(author)
	lowerEmail := strings.ToLower(email)

	for _, pattern := range botPatterns {
		if strings.Contains(lowerAuthor, pattern) || strings.Contains(lowerEmail, pattern) {
			return true
		}
	}

	return false
}
