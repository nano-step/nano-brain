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
	"regexp"
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

	commits, tip, err := g.gitLogRecovering(repoPath, lastSHA, g.maxCommits)
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

		// The dedup lookup and the write use the same sanitized source path.
		sourcePath := "git://commit/" + sanitizeText(commit.SHA)
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

	// The marker is the batch end git itself reported (rev-list/rev-parse), not
	// a hash parsed out of the log body: a commit message can embed the record
	// separators and forge a "commit" with any SHA (#675).
	if len(commits) > 0 {
		if err := g.updateLastHarvestedSHA(ctx, workspaceHash, tip); err != nil {
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

	md = sanitizeText(md)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, sanitizeDocParams(sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   contentHash,
		Title:         commit.Subject,
		Content:       md,
		SourcePath:    sourcePath,
		Collection:    "git-history",
		Tags:          tags,
		Metadata:      meta,
	}))
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

	_, err := g.queries.UpsertDocumentBySourcePath(ctx, sanitizeDocParams(sqlc.UpsertDocumentBySourcePathParams{
		WorkspaceHash: workspaceHash,
		ContentHash:   contentHash,
		Title:         "Last Harvested Commit",
		Content:       sha,
		SourcePath:    markerPath,
		Collection:    "git-history",
		Tags:          []string{"git", "meta"},
		Metadata:      meta,
	}))
	if err != nil {
		return fmt.Errorf("upsert marker: %w", err)
	}
	return nil
}

var gitObjectID = regexp.MustCompile(`^[0-9a-f]{7,64}$`)

// gitLogRecovering is gitLog with one repair: a marker that is a well-formed
// object id but names no commit in this repository (rewritten history, a
// hand-edited marker) makes every "git log <id>..HEAD" fail, so the marker
// would never advance again. In that case the harvest restarts from the
// beginning; commits already stored are skipped by their content hash. Any
// other git failure is returned unchanged.
func (g *GitHarvester) gitLogRecovering(repoPath string, since string, maxCommits int) ([]gitCommit, string, error) {
	commits, tip, err := g.gitLog(repoPath, since, maxCommits)
	if err == nil || !gitCommitMissing(repoPath, since) {
		return commits, tip, err
	}
	g.logger.Warn().Str("repo", repoPath).Str("since", since).Msg("last-harvested marker is not a commit in this repository, harvesting from the start")
	return g.gitLog(repoPath, "", maxCommits)
}

// gitCommitMissing reports whether id is a well-formed object id that git
// cannot resolve to a commit in repoPath. It is false when id is empty or
// malformed (gitLog already ignores those) and when git itself cannot be run.
func gitCommitMissing(repoPath, id string) bool {
	if !gitObjectID.MatchString(id) {
		return false
	}
	cmd := exec.Command("git", "cat-file", "-e", id+"^{commit}")
	cmd.Dir = repoPath
	_, isExit := cmd.Run().(*exec.ExitError)
	return isExit
}

// gitLog returns the batch of pending commits oldest-first and the batch end
// git itself reported (the marker to store): the Nth pending commit when a
// limit applies, HEAD otherwise — never a hash parsed out of the log body.
func (g *GitHarvester) gitLog(repoPath string, since string, maxCommits int) ([]gitCommit, string, error) {
	// since comes from a stored document. Anything that is not an object id
	// is dropped rather than handed to git, where a value starting with "-"
	// would be parsed as an option.
	if since != "" && !gitObjectID.MatchString(since) {
		g.logger.Warn().Str("since", since).Msg("last-harvested marker is not a commit id, harvesting from the start")
		since = ""
	}

	// With a limit, the batch must be the OLDEST pending commits: "-nN" keeps
	// the newest N, and the marker then jumps to HEAD and skips the rest for
	// good. So the batch's upper bound is the Nth pending commit, and the
	// marker advances contiguously from one cycle to the next.
	until := "HEAD"
	if maxCommits > 0 {
		var err error
		if until, err = gitNthPendingCommit(repoPath, since, maxCommits); err != nil {
			return nil, "", err
		}
		if until == "" {
			return nil, "", nil
		}
	}

	revSpec := "HEAD"
	switch {
	case since != "":
		revSpec = since + ".." + until
	case until != "HEAD":
		revSpec = until
	}

	// Use %x1e (record separator) to delimit commits, %x1f (unit separator) for fields.
	// --name-only output appears after the format string for each commit.
	args := []string{
		"log",
		"--format=%x1e%H%x1f%an%x1f%ae%x1f%aI%x1f%s%x1f%b",
		"--name-only",
		"--reverse",
		revSpec,
		"--",
	}

	output, err := runGitLog(repoPath, args)
	if err != nil {
		return nil, "", err
	}
	commits, err := parseGitLog(output)
	if err != nil {
		return nil, "", err
	}
	if len(commits) == 0 {
		return nil, "", nil
	}

	// A commit message may itself contain \x1e/\x1f, so a parsed "record" can
	// be a forgery embedded in another commit's body and would overwrite that
	// hash's document. Keep only records whose hash is a real commit of this
	// range, and drop every hash that appears more than once: git lists each
	// commit exactly once, so a duplicate means at least one forged record.
	validIDs, err := gitRevListSHAs(repoPath, revSpec)
	if err != nil {
		return nil, "", err
	}
	counts := make(map[string]int, len(commits))
	for _, c := range commits {
		counts[c.SHA]++
	}
	real := commits[:0]
	for _, c := range commits {
		if _, ok := validIDs[c.SHA]; !ok {
			g.logger.Warn().Str("sha", c.SHA).Msg("dropping forged git-log record: hash is not a commit in this range")
			continue
		}
		if counts[c.SHA] > 1 {
			g.logger.Warn().Str("sha", c.SHA).Msg("dropping duplicated git-log record: hash is ambiguous")
			continue
		}
		real = append(real, c)
	}
	if len(real) == 0 {
		return nil, "", nil
	}

	// The marker comes from git output, not the parsed records.
	tip := until
	if maxCommits == 0 {
		if tip, err = gitHeadSHA(repoPath); err != nil {
			return nil, "", err
		}
	}
	return real, tip, nil
}

// gitRevListSHAs returns the full hex ids of every commit in revSpec.
func gitRevListSHAs(repoPath, revSpec string) (map[string]struct{}, error) {
	output, err := runGitLog(repoPath, []string{"rev-list", revSpec, "--"})
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{})
	for _, id := range strings.Fields(output) {
		ids[id] = struct{}{}
	}
	return ids, nil
}

// gitHeadSHA returns HEAD's full commit id, or an error for an empty or
// missing HEAD (an unborn branch has nothing to harvest anyway).
func gitHeadSHA(repoPath string) (string, error) {
	output, err := runGitLog(repoPath, []string{"rev-parse", "HEAD"})
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(output)
	if !gitObjectID.MatchString(id) {
		return "", fmt.Errorf("git rev-parse HEAD returned %q, which is not a commit id", id)
	}
	return id, nil
}

// gitNthPendingCommit returns the id of the nth pending commit, oldest first,
// or the newest pending commit when fewer than n are pending. It returns ""
// when nothing is pending.
//
// With a marker the walk is --ancestry-path since..HEAD: it visits only
// commits that descend from the marker, so the returned batch end is always a
// descendant of the marker and the marker can only move forward. Plain
// since..HEAD would let the Nth commit sit on a branch that forked BEFORE the
// marker; storing it regresses the marker, re-lists since's ancestors every
// cycle and never reaches HEAD on merge history. The cost: the batch
// since..<batchEnd> may exceed n when a large side branch merges into the
// chosen commit — the limit bounds the ancestry path, not the whole batch.
// Without a marker the plain HEAD walk and the nth-in-topo-order cut are
// kept: commits not yet reachable stay pending for a later cycle.
func gitNthPendingCommit(repoPath, since string, n int) (string, error) {
	args := []string{"log", "--format=%H", "--reverse", "--topo-order"}
	if since != "" {
		args = append(args, "--ancestry-path", since+"..HEAD")
	} else {
		args = append(args, "HEAD")
	}
	args = append(args, "--")
	output, err := runGitLog(repoPath, args)
	if err != nil {
		return "", err
	}
	ids := strings.Fields(output)
	if len(ids) == 0 {
		return "", nil
	}
	if len(ids) > n {
		ids = ids[:n]
	}
	id := ids[len(ids)-1]
	if !gitObjectID.MatchString(id) {
		return "", fmt.Errorf("git log returned %q, which is not a commit id", id)
	}
	return id, nil
}

func runGitLog(repoPath string, args []string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git log failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("git log failed: %w", err)
	}
	return string(output), nil
}

func parseGitLog(output string) ([]gitCommit, error) {
	if strings.TrimSpace(output) == "" {
		return nil, nil
	}

	commits := []gitCommit{}
	entries := strings.Split(output, "\x1e")

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, "\x1f", 6)
		if len(parts) < 5 {
			continue
		}

		sha := strings.TrimSpace(parts[0])
		author := strings.TrimSpace(parts[1])
		email := strings.TrimSpace(parts[2])
		dateStr := strings.TrimSpace(parts[3])
		subject := strings.TrimSpace(parts[4])

		var body string
		var filesStr string
		if len(parts) >= 6 {
			remainder := parts[5]
			// --name-only output is separated from body by a double newline
			if idx := strings.Index(remainder, "\n\n"); idx >= 0 {
				body = strings.TrimSpace(remainder[:idx])
				filesStr = strings.TrimSpace(remainder[idx+2:])
			} else {
				body = strings.TrimSpace(remainder)
			}
		}

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

	// Commit subjects, authors and file names come from the repository and
	// need not be UTF-8, so the whole document is sanitized like the session
	// renderers.
	return sanitizeText(b.String())
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
