package intelligence

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

type mockCategorizeQuerier struct {
	docs       []Document
	tagUpdates map[string][]string
	err        error
}

func (m *mockCategorizeQuerier) ListDocumentsWithoutSemanticTags(ctx context.Context, workspace string, maxDocs int) ([]Document, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	var result []Document
	for _, doc := range m.docs {
		if !hasSemanticTags(doc.Tags) {
			result = append(result, doc)
			if len(result) >= maxDocs {
				break
			}
		}
	}
	return result, nil
}

func (m *mockCategorizeQuerier) UpdateDocumentTags(ctx context.Context, workspace, docID string, tags []string) error {
	if m.err != nil {
		return m.err
	}
	if m.tagUpdates == nil {
		m.tagUpdates = make(map[string][]string)
	}
	m.tagUpdates[docID] = tags
	return nil
}

func TestCategorizer_RunCategorization_NoDocuments(t *testing.T) {
	llm := &mockLLM{}
	queries := &mockCategorizeQuerier{docs: []Document{}}
	logger := zerolog.Nop()

	categorizer := NewCategorizer(llm, queries, logger, 0.6)

	result, err := categorizer.RunCategorization(context.Background(), "test-workspace", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Processed != 0 {
		t.Errorf("expected 0 processed, got %d", result.Processed)
	}
	if result.Categorized != 0 {
		t.Errorf("expected 0 categorized, got %d", result.Categorized)
	}
}

func TestCategorizer_RunCategorization_HighConfidence(t *testing.T) {
	llm := &mockLLM{
		response: `{
			"tags": ["bug-fix", "architecture"],
			"confidence": 0.85,
			"reasoning": "Document describes a bug fix that required architectural changes"
		}`,
	}

	docs := []Document{
		{
			ID:         "doc1",
			Title:      "Fix memory leak in connection pool",
			Content:    "This document describes how we fixed a memory leak...",
			Collection: "memory",
			Tags:       []string{"session"},
		},
	}

	queries := &mockCategorizeQuerier{docs: docs}
	logger := zerolog.Nop()

	categorizer := NewCategorizer(llm, queries, logger, 0.6)

	result, err := categorizer.RunCategorization(context.Background(), "test-workspace", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Categorized != 1 {
		t.Errorf("expected 1 categorized, got %d", result.Categorized)
	}

	if len(queries.tagUpdates) != 1 {
		t.Errorf("expected 1 tag update, got %d", len(queries.tagUpdates))
	}

	updatedTags := queries.tagUpdates["doc1"]
	if len(updatedTags) != 3 {
		t.Errorf("expected 3 tags (session + 2 new), got %d", len(updatedTags))
	}
}

func TestCategorizer_RunCategorization_LowConfidence(t *testing.T) {
	llm := &mockLLM{
		response: `{
			"tags": ["feature"],
			"confidence": 0.4,
			"reasoning": "Not entirely clear what this document is about"
		}`,
	}

	docs := []Document{
		{
			ID:         "doc1",
			Title:      "Some Document",
			Content:    "Unclear content...",
			Collection: "memory",
			Tags:       []string{"opencode"},
		},
	}

	queries := &mockCategorizeQuerier{docs: docs}
	logger := zerolog.Nop()

	categorizer := NewCategorizer(llm, queries, logger, 0.6)

	result, err := categorizer.RunCategorization(context.Background(), "test-workspace", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Categorized != 0 {
		t.Errorf("expected 0 categorized (low confidence), got %d", result.Categorized)
	}

	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped, got %d", result.Skipped)
	}

	if len(queries.tagUpdates) != 0 {
		t.Errorf("expected no tag updates (low confidence), got %d", len(queries.tagUpdates))
	}
}

func TestCategorizer_RunCategorization_FilterStaticTags(t *testing.T) {
	llm := &mockLLM{
		response: `{
			"tags": ["opencode", "session", "bug-fix"],
			"confidence": 0.9,
			"reasoning": "Clear bug fix document"
		}`,
	}

	docs := []Document{
		{
			ID:         "doc1",
			Title:      "Bug Fix",
			Content:    "Fixed a bug...",
			Collection: "memory",
			Tags:       []string{},
		},
	}

	queries := &mockCategorizeQuerier{docs: docs}
	logger := zerolog.Nop()

	categorizer := NewCategorizer(llm, queries, logger, 0.6)

	result, err := categorizer.RunCategorization(context.Background(), "test-workspace", 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Categorized != 1 {
		t.Errorf("expected 1 categorized, got %d", result.Categorized)
	}

	updatedTags := queries.tagUpdates["doc1"]
	hasStaticTag := false
	for _, tag := range updatedTags {
		if tag == "opencode" || tag == "session" {
			hasStaticTag = true
			break
		}
	}

	if hasStaticTag {
		t.Errorf("static tags should be filtered out, but found one in: %v", updatedTags)
	}
}

func TestCategorizer_ConfidenceThresholdValidation(t *testing.T) {
	llm := &mockLLM{}
	queries := &mockCategorizeQuerier{}
	logger := zerolog.Nop()

	tests := []struct {
		input    float64
		expected float64
	}{
		{0.8, 0.8},
		{0.0, 0.6},
		{-0.5, 0.6},
		{1.5, 0.6},
	}

	for _, tt := range tests {
		categorizer := NewCategorizer(llm, queries, logger, tt.input)
		if categorizer.confidence != tt.expected {
			t.Errorf("for input %f, expected confidence %f, got %f", tt.input, tt.expected, categorizer.confidence)
		}
	}
}

func TestHasSemanticTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		expected bool
	}{
		{"no tags", []string{}, false},
		{"only static tags", []string{"opencode", "session"}, false},
		{"has semantic tag", []string{"opencode", "bug-fix"}, true},
		{"only semantic tags", []string{"feature", "architecture"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasSemanticTags(tt.tags)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for tags %v", tt.expected, result, tt.tags)
			}
		})
	}
}
