package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"
)

type CategorizeQuerier interface {
	ListDocumentsWithoutSemanticTags(ctx context.Context, workspace string, maxDocs int) ([]Document, error)
	UpdateDocumentTags(ctx context.Context, workspace, docID string, tags []string) error
}

type Categorizer struct {
	llm        LLM
	queries    CategorizeQuerier
	logger     zerolog.Logger
	confidence float64
}

func NewCategorizer(
	llm LLM,
	queries CategorizeQuerier,
	logger zerolog.Logger,
	confidenceThreshold float64,
) *Categorizer {
	if confidenceThreshold <= 0 || confidenceThreshold > 1 {
		confidenceThreshold = 0.6
	}
	return &Categorizer{
		llm:        llm,
		queries:    queries,
		logger:     logger,
		confidence: confidenceThreshold,
	}
}

type CategorizationResult struct {
	Processed   int
	Categorized int
	Skipped     int
	Errors      int
}

type categorizationResponse struct {
	Tags       []string `json:"tags"`
	Confidence float64  `json:"confidence"`
	Reasoning  string   `json:"reasoning"`
}

var staticTags = map[string]bool{
	"opencode": true,
	"session":  true,
	"claude":   true,
	"summary":  true,
}

func (c *Categorizer) RunCategorization(ctx context.Context, workspace string, maxDocs int) (*CategorizationResult, error) {
	c.logger.Info().
		Str("workspace", workspace).
		Int("max_docs", maxDocs).
		Float64("confidence_threshold", c.confidence).
		Msg("categorization: starting")

	docs, err := c.queries.ListDocumentsWithoutSemanticTags(ctx, workspace, maxDocs)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	c.logger.Info().Int("doc_count", len(docs)).Msg("categorization: documents loaded")

	result := &CategorizationResult{
		Processed: len(docs),
	}

	for _, doc := range docs {
		if err := c.processDocument(ctx, workspace, doc, result); err != nil {
			c.logger.Warn().
				Err(err).
				Str("doc_id", doc.ID).
				Msg("categorization: document processing failed")
			result.Errors++
		}
	}

	c.logger.Info().
		Int("processed", result.Processed).
		Int("categorized", result.Categorized).
		Int("skipped", result.Skipped).
		Int("errors", result.Errors).
		Msg("categorization: completed")

	return result, nil
}

func (c *Categorizer) processDocument(ctx context.Context, workspace string, doc Document, result *CategorizationResult) error {
	summary := DocumentSummary{
		ID:         doc.ID,
		Title:      doc.Title,
		SourcePath: doc.SourcePath,
		Content:    doc.Content,
		Tags:       doc.Tags,
	}

	userPrompt := buildCategorizationUserPrompt(summary)

	c.logger.Info().
		Str("doc_id", doc.ID).
		Str("title", doc.Title).
		Msg("categorization: calling LLM")

	responseText, _, err := c.llm.ChatCompletion(ctx, categorizationSystemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("llm completion: %w", err)
	}

	responseText = strings.TrimSpace(responseText)
	if strings.HasPrefix(responseText, "```json") {
		responseText = strings.TrimPrefix(responseText, "```json")
		responseText = strings.TrimSuffix(responseText, "```")
		responseText = strings.TrimSpace(responseText)
	}

	var response categorizationResponse
	if err := json.Unmarshal([]byte(responseText), &response); err != nil {
		return fmt.Errorf("parse llm response: %w", err)
	}

	c.logger.Info().
		Str("doc_id", doc.ID).
		Strs("suggested_tags", response.Tags).
		Float64("confidence", response.Confidence).
		Str("reasoning", response.Reasoning).
		Msg("categorization: LLM response")

	if response.Confidence < c.confidence {
		c.logger.Info().
			Str("doc_id", doc.ID).
			Float64("confidence", response.Confidence).
			Float64("threshold", c.confidence).
			Msg("categorization: confidence below threshold, skipping")
		result.Skipped++
		return nil
	}

	newTags := mergeWithExisting(doc.Tags, response.Tags)

	if len(newTags) == len(doc.Tags) {
		c.logger.Info().
			Str("doc_id", doc.ID).
			Msg("categorization: no new tags to add")
		result.Skipped++
		return nil
	}

	if err := c.queries.UpdateDocumentTags(ctx, workspace, doc.ID, newTags); err != nil {
		return fmt.Errorf("update tags: %w", err)
	}

	c.logger.Info().
		Str("doc_id", doc.ID).
		Strs("old_tags", doc.Tags).
		Strs("new_tags", newTags).
		Msg("categorization: tags updated")

	result.Categorized++
	return nil
}

func mergeWithExisting(existing, suggested []string) []string {
	tagSet := make(map[string]bool)
	for _, tag := range existing {
		tagSet[tag] = true
	}

	for _, tag := range suggested {
		if !staticTags[tag] {
			tagSet[tag] = true
		}
	}

	var result []string
	for tag := range tagSet {
		result = append(result, tag)
	}
	return result
}

func hasSemanticTags(tags []string) bool {
	for _, tag := range tags {
		if !staticTags[tag] {
			return true
		}
	}
	return false
}
