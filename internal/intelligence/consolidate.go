package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

type ConsolidateQuerier interface {
	ListDocumentsByCollection(ctx context.Context, workspace, collection string) ([]Document, error)
	GetDocumentByID(ctx context.Context, workspace, docID string) (Document, error)
	UpsertDocument(ctx context.Context, doc Document) error
	DeleteDocument(ctx context.Context, workspace, docID string) error
}

type SearchService interface {
	VectorSearch(ctx context.Context, query string, workspace string, maxResults int) ([]SearchResult, error)
}

type Document struct {
	ID           string
	WorkspaceHash string
	ContentHash  string
	Title        string
	Content      string
	SourcePath   string
	Collection   string
	Tags         []string
	SupersedesID *string
}

type SearchResult struct {
	ChunkID    string
	DocumentID string
	Content    string
	Score      float64
}

type Consolidator struct {
	llm         LLM
	queries     ConsolidateQuerier
	searchSvc   SearchService
	logger      zerolog.Logger
	dryRun      bool
	simThreshold float64
}

func NewConsolidator(
	llm LLM,
	queries ConsolidateQuerier,
	searchSvc SearchService,
	logger zerolog.Logger,
	dryRun bool,
) *Consolidator {
	return &Consolidator{
		llm:         llm,
		queries:     queries,
		searchSvc:   searchSvc,
		logger:      logger,
		dryRun:      dryRun,
		simThreshold: 0.85,
	}
}

type ConsolidationResult struct {
	ClustersFound int
	Merged        int
	Skipped       int
	Errors        int
}

type consolidationResponse struct {
	ShouldMerge         bool   `json:"should_merge"`
	Reasoning           string `json:"reasoning"`
	ConsolidatedContent string `json:"consolidated_content"`
	Title               string `json:"title"`
}

func (c *Consolidator) RunConsolidation(ctx context.Context, workspace string) (*ConsolidationResult, error) {
	c.logger.Info().
		Str("workspace", workspace).
		Bool("dry_run", c.dryRun).
		Msg("consolidation: starting")

	docs, err := c.queries.ListDocumentsByCollection(ctx, workspace, "memory")
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	c.logger.Info().Int("doc_count", len(docs)).Msg("consolidation: documents loaded")

	if len(docs) == 0 {
		return &ConsolidationResult{}, nil
	}

	clusters := c.findClusters(ctx, workspace, docs)

	c.logger.Info().Int("clusters", len(clusters)).Msg("consolidation: clusters identified")

	result := &ConsolidationResult{
		ClustersFound: len(clusters),
	}

	for _, cluster := range clusters {
		if len(cluster) < 2 {
			continue
		}

		merged, err := c.processCluster(ctx, workspace, cluster)
		if err != nil {
			c.logger.Warn().Err(err).Msg("consolidation: cluster processing failed")
			result.Errors++
			continue
		}

		if merged {
			result.Merged++
		} else {
			result.Skipped++
		}
	}

	c.logger.Info().
		Int("clusters", result.ClustersFound).
		Int("merged", result.Merged).
		Int("skipped", result.Skipped).
		Int("errors", result.Errors).
		Msg("consolidation: completed")

	return result, nil
}

func (c *Consolidator) findClusters(ctx context.Context, workspace string, docs []Document) [][]Document {
	var clusters [][]Document
	processed := make(map[string]bool)

	for _, doc := range docs {
		if processed[doc.ID] {
			continue
		}

		similar, err := c.findSimilarDocs(ctx, workspace, doc, docs)
		if err != nil {
			c.logger.Warn().Err(err).Str("doc_id", doc.ID).Msg("consolidation: similarity search failed")
			continue
		}

		if len(similar) > 0 {
			cluster := []Document{doc}
			cluster = append(cluster, similar...)

			for _, d := range cluster {
				processed[d.ID] = true
			}

			clusters = append(clusters, cluster)
		}
	}

	return clusters
}

func (c *Consolidator) findSimilarDocs(ctx context.Context, workspace string, doc Document, allDocs []Document) ([]Document, error) {
	searchQuery := doc.Title + " " + truncate(doc.Content, 500)

	results, err := c.searchSvc.VectorSearch(ctx, searchQuery, workspace, 10)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	var similar []Document
	for _, result := range results {
		if result.DocumentID == doc.ID {
			continue
		}

		if result.Score < c.simThreshold {
			continue
		}

		for _, d := range allDocs {
			if d.ID == result.DocumentID {
				similar = append(similar, d)
				break
			}
		}
	}

	return similar, nil
}

func (c *Consolidator) processCluster(ctx context.Context, workspace string, cluster []Document) (bool, error) {
	summaries := make([]DocumentSummary, len(cluster))
	for i, doc := range cluster {
		summaries[i] = DocumentSummary{
			ID:         doc.ID,
			Title:      doc.Title,
			SourcePath: doc.SourcePath,
			Content:    doc.Content,
			Tags:       doc.Tags,
		}
	}

	userPrompt := buildConsolidationUserPrompt(summaries)

	c.logger.Info().
		Int("cluster_size", len(cluster)).
		Msg("consolidation: calling LLM for cluster")

	responseText, _, err := c.llm.ChatCompletion(ctx, consolidationSystemPrompt, userPrompt)
	if err != nil {
		return false, fmt.Errorf("llm completion: %w", err)
	}

	responseText = strings.TrimSpace(responseText)
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimSuffix(responseText, "```")
		responseText = strings.TrimSpace(responseText)
	}

	var response consolidationResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return false, fmt.Errorf("parse llm response: %w", err)
	}

	c.logger.Info().
		Bool("should_merge", response.ShouldMerge).
		Str("reasoning", response.Reasoning).
		Msg("consolidation: LLM decision")

	if !response.ShouldMerge {
		return false, nil
	}

	if c.dryRun {
		c.logger.Info().Msg("consolidation: dry-run mode, skipping merge")
		return true, nil
	}

	mergedDoc := Document{
		WorkspaceHash: workspace,
		Title:         response.Title,
		Content:       response.ConsolidatedContent,
		SourcePath:    "memory://consolidated/" + cluster[0].ID,
		Collection:    "memory",
		Tags:          mergeTags(cluster),
	}

	if err := c.queries.UpsertDocument(ctx, mergedDoc); err != nil {
		return false, fmt.Errorf("upsert merged document: %w", err)
	}

	for _, doc := range cluster {
		if err := c.queries.DeleteDocument(ctx, workspace, doc.ID); err != nil {
			c.logger.Warn().Err(err).Str("doc_id", doc.ID).Msg("consolidation: failed to delete original")
		}
	}

	return true, nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func mergeTags(docs []Document) []string {
	tagSet := make(map[string]bool)
	for _, doc := range docs {
		for _, tag := range doc.Tags {
			tagSet[tag] = true
		}
	}

	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}
