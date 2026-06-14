package flow

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockSummarizer struct {
	lastEntry        string
	lastChain        []string
	lastIntegrations []string
	summary          string
	err              error
}

func (m *mockSummarizer) Summarize(_ context.Context, entry string, chain []string, integrations []string) (string, error) {
	m.lastEntry = entry
	m.lastChain = chain
	m.lastIntegrations = integrations
	return m.summary, m.err
}

type recordingQuerier struct {
	edges       []sqlc.GraphEdge
	upsertedDoc *sqlc.UpsertDocumentBySourcePathParams
	insertErr   error
}

func (m *recordingQuerier) ListAllEdgesByWorkspace(_ context.Context, _ string) ([]sqlc.GraphEdge, error) {
	return m.edges, nil
}

func (m *recordingQuerier) UpsertDocumentBySourcePath(_ context.Context, arg sqlc.UpsertDocumentBySourcePathParams) (sqlc.UpsertDocumentBySourcePathRow, error) {
	m.upsertedDoc = &arg
	return sqlc.UpsertDocumentBySourcePathRow{ID: uuid.New()}, m.insertErr
}

func (m *recordingQuerier) ListDocumentSourcePathsAndHashes(_ context.Context, _ sqlc.ListDocumentSourcePathsAndHashesParams) ([]sqlc.ListDocumentSourcePathsAndHashesRow, error) {
	return nil, nil
}

func (m *recordingQuerier) DeleteDocumentByIDAndWorkspace(_ context.Context, _ sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error) {
	return 0, nil
}

func (m *recordingQuerier) DeleteChunksByDocumentID(_ context.Context, _ sqlc.DeleteChunksByDocumentIDParams) error {
	return nil
}

func (m *recordingQuerier) UpsertChunk(_ context.Context, _ sqlc.UpsertChunkParams) (uuid.UUID, error) {
	return uuid.New(), nil
}

func TestSummarizerCalledWithCorrectArgs(t *testing.T) {
	ms := &mockSummarizer{summary: "test summary"}
	q := &recordingQuerier{
		edges: []sqlc.GraphEdge{
			{SourceNode: "POST /api/topup", TargetNode: "TopupHandler", EdgeType: string(graph.EdgeHTTP)},
			{SourceNode: "handlers/x.go::TopupHandler", TargetNode: "PaymentService", EdgeType: string(graph.EdgeCalls), SourceFile: "handlers/x.go"},
			{SourceNode: "handlers/x.go::TopupHandler", TargetNode: "POST http://external.api/charge", EdgeType: string(graph.EdgeIntegration), SourceFile: "handlers/x.go"},
		},
	}

	mat := NewMaterializer(q, nil, 10, 10, ms, zerolog.Nop())
	if err := mat.Materialize(context.Background(), "test_ws"); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if ms.lastEntry != "POST /api/topup" {
		t.Errorf("lastEntry = %q, want %q", ms.lastEntry, "POST /api/topup")
	}
	if len(ms.lastChain) == 0 || ms.lastChain[0] != "POST /api/topup" {
		t.Errorf("lastChain[0] = %q, want %q", ms.lastChain[0], "POST /api/topup")
	}
	found := false
	for _, in := range ms.lastIntegrations {
		if in == "POST http://external.api/charge" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("lastIntegrations should include integration edge, got %v", ms.lastIntegrations)
	}
}

func TestFallbackToTextOnSummarizerError(t *testing.T) {
	ms := &mockSummarizer{summary: "", err: context.DeadlineExceeded}
	q := &recordingQuerier{
		edges: []sqlc.GraphEdge{
			{SourceNode: "GET /api/health", TargetNode: "HealthHandler", EdgeType: string(graph.EdgeHTTP)},
		},
	}

	mat := NewMaterializer(q, nil, 10, 10, ms, zerolog.Nop())
	if err := mat.Materialize(context.Background(), "test_ws"); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if q.upsertedDoc == nil {
		t.Fatal("expected upserted doc")
	}

	// Must contain "Entry:" prefix (text summary), not the error placeholder.
	if !containsEntryLine(q.upsertedDoc.Content, "GET /api/health") {
		t.Errorf("content should contain entry line, got: %q", q.upsertedDoc.Content)
	}
	if q.upsertedDoc.Content == "" {
		t.Error("content should not be empty")
	}
	if q.upsertedDoc.Metadata.Valid {
		t.Errorf("metadata should be invalid on error fallback, got valid")
	}
}

func TestDocumentContentMatchesSummary(t *testing.T) {
	ms := &mockSummarizer{summary: "LLM-generated summary"}
	q := &recordingQuerier{
		edges: []sqlc.GraphEdge{
			{SourceNode: "POST /api/topup", TargetNode: "TopupHandler", EdgeType: string(graph.EdgeHTTP)},
		},
	}

	mat := NewMaterializer(q, nil, 10, 10, ms, zerolog.Nop())
	if err := mat.Materialize(context.Background(), "test_ws"); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if q.upsertedDoc == nil {
		t.Fatal("expected upserted doc")
	}

	if q.upsertedDoc.Content != "LLM-generated summary" {
		t.Errorf("content = %q, want %q", q.upsertedDoc.Content, "LLM-generated summary")
	}
	if !q.upsertedDoc.Metadata.Valid {
		t.Error("metadata should be valid when summary is used")
	}
}

func TestTextSummaryWhenNoSummarizer(t *testing.T) {
	q := &recordingQuerier{
		edges: []sqlc.GraphEdge{
			{SourceNode: "POST /api/topup", TargetNode: "TopupHandler", EdgeType: string(graph.EdgeHTTP)},
		},
	}

	mat := NewMaterializer(q, nil, 10, 10, nil, zerolog.Nop())
	if err := mat.Materialize(context.Background(), "test_ws"); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	if q.upsertedDoc == nil {
		t.Fatal("expected upserted doc")
	}

	if !containsEntryLine(q.upsertedDoc.Content, "POST /api/topup") {
		t.Errorf("content should contain entry line, got: %q", q.upsertedDoc.Content)
	}
	if q.upsertedDoc.Metadata.Valid {
		t.Errorf("metadata should be invalid when no summarizer, got valid")
	}
}

func containsEntryLine(content, entry string) bool {
	return strings.Contains(content, "Entry: "+entry)
}
