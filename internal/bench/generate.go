package bench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type DocumentRow struct {
	ID            uuid.UUID
	WorkspaceHash string
	ContentHash   string
	Title         string
	SourcePath    string
	Collection    string
	Content       string
}

type DataStore interface {
	ListDocumentsByWorkspace(ctx context.Context, workspaceHash string) ([]DocumentRow, error)
	GetDocumentByID(ctx context.Context, id uuid.UUID, workspaceHash string) (*DocumentRow, error)
}

type BenchConfig struct {
	QueryGeneration string
	ProviderURL     string
	APIKey          string
	Model           string
	MaxTokens       int
}

func Generate(ctx context.Context, store DataStore, workspaceHash string, scale int, cfg *BenchConfig) (*BenchmarkDataset, error) {
	if scale < 1 {
		return nil, fmt.Errorf("scale must be a positive integer, got %d", scale)
	}

	docs, err := store.ListDocumentsByWorkspace(ctx, workspaceHash)
	if err != nil {
		return nil, fmt.Errorf("listing documents: %w", err)
	}
	if len(docs) < scale {
		return nil, fmt.Errorf("workspace %q has %d documents, need at least %d", workspaceHash, len(docs), scale)
	}

	shuffled := make([]DocumentRow, len(docs))
	copy(shuffled, docs)
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	sampled := shuffled[:scale]

	useLLM := cfg != nil && cfg.QueryGeneration == "llm" && cfg.ProviderURL != ""

	entries := make([]DatasetEntry, 0, scale)
	for _, doc := range sampled {
		query := deriveQuery(doc)

		if useLLM {
			fetched, err := store.GetDocumentByID(ctx, doc.ID, workspaceHash)
			if err == nil && fetched != nil && len(fetched.Content) > 100 {
				llmQuery, err := generateLLMQuery(ctx, cfg, fetched.Content)
				if err == nil && llmQuery != "" {
					query = llmQuery
				}
				time.Sleep(1 * time.Second)
			}
		}

		entries = append(entries, DatasetEntry{
			Query:          query,
			RelevantDocIDs: []string{doc.ID.String()},
			SourceDocID:    doc.ID.String(),
			SourceTitle:    doc.Title,
		})
	}

	return &BenchmarkDataset{
		Version:       "generated",
		Scale:         scale,
		WorkspaceHash: workspaceHash,
		Entries:       entries,
	}, nil
}

func deriveQuery(doc DocumentRow) string {
	if doc.Title != "" {
		title := strings.TrimSpace(doc.Title)
		// Strip query parameters from title (e.g., "symbol=setup&kind=method")
		if idx := strings.Index(title, "?"); idx > 0 {
			title = title[:idx]
		}
		// Strip "Summary: " prefix and "(var)", "(function)" etc. suffixes
		title = strings.TrimPrefix(title, "Summary: ")
		return title
	}
	if doc.SourcePath != "" {
		// Use just the filename, not the full path
		path := strings.TrimSpace(doc.SourcePath)
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			return path[idx+1:]
		}
		return path
	}
	return doc.ID.String()
}

func generateLLMQuery(ctx context.Context, cfg *BenchConfig, content string) (string, error) {
	if len(content) > 3000 {
		content = content[:3000]
	}

	prompt := fmt.Sprintf(`Given this document content, generate ONE short question that a developer would ask to find this document. 
The question should be specific to the document's topic.
Return ONLY the question text, no quotes, no explanation.

Document content:
%s`, content)

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 100
	}

	reqBody := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": maxTokens,
		"temperature": 0.3,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.ProviderURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("LLM API returned status %d", resp.StatusCode)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in LLM response")
	}

	query := strings.TrimSpace(result.Choices[0].Message.Content)
	query = strings.Trim(query, `"'`)
	return query, nil
}
