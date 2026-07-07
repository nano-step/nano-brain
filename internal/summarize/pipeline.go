package summarize

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/rs/zerolog"
)

// reGoalHeading matches the "## Goal" heading that opens the 5-section
// summary format (see ReduceSystemPrompt). Some summarizer models echo their
// own instructions, intermediate drafts, or the output template before the
// real summary (#550) despite being told not to; extractFinalSection keeps
// only the LAST such heading onward, discarding any preamble. If no heading
// is found, the completion is returned unchanged rather than risk discarding
// a valid summary that used different wording.
var reGoalHeading = regexp.MustCompile(`(?im)^##\s+Goal\b`)

func extractFinalSection(raw string) string {
	locs := reGoalHeading.FindAllStringIndex(raw, -1)
	if len(locs) == 0 {
		return raw
	}
	return strings.TrimSpace(raw[locs[len(locs)-1][0]:])
}

const (
	SingleShotThreshold = 4000
	ChunkTargetSize     = 4000
	ChunkOverlap        = 200
	ReduceBatchSize     = 10
	ReduceContextLimit  = 50000
	maxReduceDepth      = 3
)

// Source identifies the session source.
type Source string

const (
	SourceOpenCode Source = "opencode"
	SourceClaude   Source = "claude"
)

// SessionMetadata is supplied by the harvester for the metadata header.
type SessionMetadata struct {
	Source        Source
	SessionID     string
	Title         string
	Agent         string
	ProjectPath   string
	CreatedAt     time.Time
	Duration      time.Duration
	ParentID      string
	Branch        string
	Cwd           string
	Tags          []string // extra tags to merge at persist time (e.g. ticket:DEV-1234)
	ParentTitle   string
	Children      []RelatedSession
	Siblings      []RelatedSession
	WorkspaceHash string
}

// RelatedSession describes a parent/child/sibling session.
type RelatedSession struct {
	ID    string
	Title string
	Agent string
}

// RelationshipLookup enriches metadata with parent/child/sibling info.
type RelationshipLookup interface {
	Lookup(ctx context.Context, meta *SessionMetadata) error
}

// LLMClient is the interface the pipeline uses for LLM calls.
type LLMClient interface {
	ChatCompletion(ctx context.Context, systemPrompt, userPrompt string) (string, TokenUsage, error)
}

// Pipeline orchestrates strip, chunk, map, reduce.
type Pipeline struct {
	llm         LLMClient
	lookup      RelationshipLookup
	concurrency int
	logger      zerolog.Logger
}

// NewPipeline constructs a Pipeline.
func NewPipeline(llm LLMClient, lookup RelationshipLookup, concurrency int, logger zerolog.Logger) *Pipeline {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Pipeline{llm: llm, lookup: lookup, concurrency: concurrency, logger: logger}
}

// Summarize runs the full pipeline and returns the final markdown.
func (p *Pipeline) Summarize(ctx context.Context, sessionContent string, meta SessionMetadata) (string, error) {
	var stripped string
	switch meta.Source {
	case SourceClaude:
		stripped = StripClaude(sessionContent)
	default:
		stripped = StripOpenCode(sessionContent)
	}

	p.logger.Info().
		Str("session_id", meta.SessionID).
		Int("raw_len", len(sessionContent)).
		Int("stripped_len", len(stripped)).
		Msg("summarize: content stripped")

	if p.lookup != nil {
		if err := p.lookup.Lookup(ctx, &meta); err != nil {
			p.logger.Warn().Err(err).Msg("summarize: relationship lookup failed, continuing")
		}
	}

	var summaryBody string
	var err error
	if len(stripped) <= SingleShotThreshold {
		p.logger.Info().Str("session_id", meta.SessionID).Msg("summarize: using single-shot mode")
		summaryBody, err = p.singleShot(ctx, stripped)
	} else {
		p.logger.Info().Str("session_id", meta.SessionID).Msg("summarize: using map-reduce mode")
		summaryBody, err = p.mapReduce(ctx, stripped)
	}
	if err != nil {
		return "", err
	}

	return p.formatHeader(meta) + "\n\n" + summaryBody, nil
}

func (p *Pipeline) singleShot(ctx context.Context, content string) (string, error) {
	result, _, err := p.llm.ChatCompletion(ctx, SingleShotSystemPrompt, FormatSingleShotUserPrompt(content))
	if err != nil {
		return "", fmt.Errorf("summarize: single-shot failed: %w", err)
	}
	return extractFinalSection(result), nil
}

func (p *Pipeline) mapReduce(ctx context.Context, content string) (string, error) {
	chunks := chunk.Split(content, chunk.Config{
		TargetSize: ChunkTargetSize,
		Overlap:    ChunkOverlap,
		MinSize:    200,
	})
	if len(chunks) == 0 {
		return "", fmt.Errorf("summarize: no chunks produced from content")
	}

	chunkSummaries := p.runMap(ctx, chunks)

	var nonEmpty []string
	for _, s := range chunkSummaries {
		if s != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	if len(nonEmpty) == 0 {
		return "", fmt.Errorf("summarize: all chunks failed summarization")
	}
	if len(nonEmpty) == 1 {
		return nonEmpty[0], nil
	}

	return p.runReduce(ctx, nonEmpty, 0)
}

func (p *Pipeline) runMap(ctx context.Context, chunks []chunk.Chunk) []string {
	results := make([]string, len(chunks))
	var mu sync.Mutex
	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup

	for i, c := range chunks {
		wg.Add(1)
		go func(idx int, content string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			result, _, err := p.llm.ChatCompletion(ctx, MapSystemPrompt, FormatMapUserPrompt(content))
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				p.logger.Warn().Err(err).Int("chunk", idx).Msg("summarize: map chunk failed")
				return
			}
			results[idx] = result
		}(i, c.Content)
	}
	wg.Wait()
	return results
}

func (p *Pipeline) runReduce(ctx context.Context, chunkSummaries []string, depth int) (string, error) {
	total := 0
	for _, s := range chunkSummaries {
		total += len(s)
	}

	if total <= ReduceContextLimit || depth >= maxReduceDepth {
		if depth >= maxReduceDepth && total > ReduceContextLimit {
			p.logger.Warn().Int("depth", depth).Int("total_chars", total).Msg("summarize: hit max reduce depth, concatenating remaining")
		}
		result, _, err := p.llm.ChatCompletion(ctx, ReduceSystemPrompt, FormatReduceUserPrompt(chunkSummaries))
		if err != nil {
			return "", fmt.Errorf("summarize: reduce failed at depth %d: %w", depth, err)
		}
		return extractFinalSection(result), nil
	}

	var batchResults []string
	for i := 0; i < len(chunkSummaries); i += ReduceBatchSize {
		end := i + ReduceBatchSize
		if end > len(chunkSummaries) {
			end = len(chunkSummaries)
		}
		batch := chunkSummaries[i:end]
		result, _, err := p.llm.ChatCompletion(ctx, ReduceSystemPrompt, FormatReduceUserPrompt(batch))
		if err != nil {
			return "", fmt.Errorf("summarize: batch reduce failed at depth %d: %w", depth, err)
		}
		batchResults = append(batchResults, extractFinalSection(result))
	}

	return p.runReduce(ctx, batchResults, depth+1)
}

func (p *Pipeline) formatHeader(meta SessionMetadata) string {
	var b strings.Builder
	title := meta.Title
	if title == "" {
		title = "Untitled Session"
	}
	fmt.Fprintf(&b, "# Session: %s\n", title)
	b.WriteString("\n")
	fmt.Fprintf(&b, "- Date: %s\n", meta.CreatedAt.Format("2006-01-02"))
	fmt.Fprintf(&b, "- Source: %s\n", meta.Source)

	if meta.SessionID != "" {
		fmt.Fprintf(&b, "- Session ID: %s\n", meta.SessionID)
	}
	if meta.Agent != "" {
		fmt.Fprintf(&b, "- Agent: %s\n", meta.Agent)
	}
	if meta.ProjectPath != "" {
		fmt.Fprintf(&b, "- Project: %s\n", meta.ProjectPath)
	}
	if meta.Branch != "" {
		fmt.Fprintf(&b, "- Branch: %s\n", meta.Branch)
	}
	if meta.Cwd != "" {
		fmt.Fprintf(&b, "- Cwd: %s\n", meta.Cwd)
	}
	if meta.Duration > 0 {
		fmt.Fprintf(&b, "- Duration: %s\n", meta.Duration)
	}
	if meta.ParentID != "" {
		label := meta.ParentTitle
		if label == "" {
			label = meta.ParentID
		}
		fmt.Fprintf(&b, "- Parent Session: %s (%s)\n", label, meta.ParentID)
	}
	if len(meta.Children) > 0 {
		b.WriteString("- Child Sessions:\n")
		for _, c := range meta.Children {
			if c.Agent != "" {
				fmt.Fprintf(&b, "  - %s (%s) [%s]\n", c.Title, c.Agent, c.ID)
			} else {
				fmt.Fprintf(&b, "  - %s [%s]\n", c.Title, c.ID)
			}
		}
	}
	if len(meta.Siblings) > 0 {
		b.WriteString("- Sibling Sessions:\n")
		for _, s := range meta.Siblings {
			fmt.Fprintf(&b, "  - %s [%s]\n", s.Title, s.ID)
		}
	}
	return b.String()
}
