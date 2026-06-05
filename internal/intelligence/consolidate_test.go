package intelligence

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"
)

type mockLLM struct {
	response string
	err      error
}

func (m *mockLLM) ChatCompletion(ctx context.Context, systemPrompt, userPrompt string) (string, TokenUsage, error) {
	if m.err != nil {
		return "", TokenUsage{}, m.err
	}
	return m.response, TokenUsage{PromptTokens: 100, CompletionTokens: 50, TotalTokens: 150}, nil
}

type mockConsolidateQuerier struct {
	docs    []Document
	upserts []Document
	deletes []string
	err     error
}

func (m *mockConsolidateQuerier) ListDocumentsByCollection(ctx context.Context, workspace, collection string) ([]Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.docs, nil
}

func (m *mockConsolidateQuerier) GetDocumentByID(ctx context.Context, workspace, docID string) (Document, error) {
	if m.err != nil {
		return Document{}, m.err
	}
	for _, doc := range m.docs {
		if doc.ID == docID {
			return doc, nil
		}
	}
	return Document{}, errors.New("not found")
}

func (m *mockConsolidateQuerier) UpsertDocument(ctx context.Context, doc Document) error {
	if m.err != nil {
		return m.err
	}
	m.upserts = append(m.upserts, doc)
	return nil
}

func (m *mockConsolidateQuerier) DeleteDocument(ctx context.Context, workspace, docID string) error {
	if m.err != nil {
		return m.err
	}
	m.deletes = append(m.deletes, docID)
	return nil
}

type mockSearchService struct {
	results []SearchResult
	err     error
}

func (m *mockSearchService) VectorSearch(ctx context.Context, query string, workspace string, maxResults int) ([]SearchResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func TestConsolidator_RunConsolidation_NoDocuments(t *testing.T) {
	llm := &mockLLM{}
	queries := &mockConsolidateQuerier{docs: []Document{}}
	searchSvc := &mockSearchService{}
	logger := zerolog.Nop()

	consolidator := NewConsolidator(llm, queries, searchSvc, logger, false)

	result, err := consolidator.RunConsolidation(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ClustersFound != 0 {
		t.Errorf("expected 0 clusters, got %d", result.ClustersFound)
	}
	if result.Merged != 0 {
		t.Errorf("expected 0 merged, got %d", result.Merged)
	}
}

func TestConsolidator_RunConsolidation_ShouldMerge(t *testing.T) {
	llm := &mockLLM{
		response: `{
			"should_merge": true,
			"reasoning": "These documents cover the same topic",
			"consolidated_content": "# Merged Content\n\nCombined information from both docs.",
			"title": "Consolidated Document"
		}`,
	}

	docs := []Document{
		{
			ID:         "doc1",
			Title:      "Document 1",
			Content:    "Content of document 1",
			Collection: "memory",
			Tags:       []string{"tag1"},
		},
		{
			ID:         "doc2",
			Title:      "Document 2",
			Content:    "Content of document 2",
			Collection: "memory",
			Tags:       []string{"tag2"},
		},
	}

	queries := &mockConsolidateQuerier{docs: docs}
	searchSvc := &mockSearchService{
		results: []SearchResult{
			{DocumentID: "doc2", Score: 0.90},
		},
	}
	logger := zerolog.Nop()

	consolidator := NewConsolidator(llm, queries, searchSvc, logger, false)

	result, err := consolidator.RunConsolidation(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Merged != 1 {
		t.Errorf("expected 1 merged, got %d", result.Merged)
	}

	if len(queries.upserts) != 1 {
		t.Errorf("expected 1 upsert, got %d", len(queries.upserts))
	}

	if len(queries.deletes) != 2 {
		t.Errorf("expected 2 deletes, got %d", len(queries.deletes))
	}
}

func TestConsolidator_RunConsolidation_ShouldNotMerge(t *testing.T) {
	llm := &mockLLM{
		response: `{
			"should_merge": false,
			"reasoning": "These documents are about different topics"
		}`,
	}

	docs := []Document{
		{
			ID:         "doc1",
			Title:      "Document 1",
			Content:    "Content of document 1",
			Collection: "memory",
		},
		{
			ID:         "doc2",
			Title:      "Document 2",
			Content:    "Content of document 2",
			Collection: "memory",
		},
	}

	queries := &mockConsolidateQuerier{docs: docs}
	searchSvc := &mockSearchService{
		results: []SearchResult{
			{DocumentID: "doc2", Score: 0.90},
		},
	}
	logger := zerolog.Nop()

	consolidator := NewConsolidator(llm, queries, searchSvc, logger, false)

	result, err := consolidator.RunConsolidation(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Merged != 0 {
		t.Errorf("expected 0 merged, got %d", result.Merged)
	}

	if len(queries.upserts) != 0 {
		t.Errorf("expected 0 upserts, got %d", len(queries.upserts))
	}

	if len(queries.deletes) != 0 {
		t.Errorf("expected 0 deletes, got %d", len(queries.deletes))
	}
}

func TestConsolidator_RunConsolidation_DryRun(t *testing.T) {
	llm := &mockLLM{
		response: `{
			"should_merge": true,
			"reasoning": "These documents cover the same topic",
			"consolidated_content": "# Merged Content",
			"title": "Consolidated Document"
		}`,
	}

	docs := []Document{
		{ID: "doc1", Title: "Doc 1", Content: "Content 1", Collection: "memory"},
		{ID: "doc2", Title: "Doc 2", Content: "Content 2", Collection: "memory"},
	}

	queries := &mockConsolidateQuerier{docs: docs}
	searchSvc := &mockSearchService{
		results: []SearchResult{{DocumentID: "doc2", Score: 0.90}},
	}
	logger := zerolog.Nop()

	consolidator := NewConsolidator(llm, queries, searchSvc, logger, true)

	result, err := consolidator.RunConsolidation(context.Background(), "test-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(queries.upserts) != 0 {
		t.Errorf("dry run should not perform upserts, got %d", len(queries.upserts))
	}

	if len(queries.deletes) != 0 {
		t.Errorf("dry run should not perform deletes, got %d", len(queries.deletes))
	}

	if result.Merged != 1 {
		t.Errorf("dry run should still report merged count, got %d", result.Merged)
	}
}
