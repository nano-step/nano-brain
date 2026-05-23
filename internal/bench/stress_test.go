package bench

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type mockStressWriter struct {
	mu          sync.Mutex
	upsertCalls int
	countBefore int64
	countAfter  int64
	failAt      map[string]bool
}

func (m *mockStressWriter) UpsertDocument(_ context.Context, arg StressUpsertParams) (StressUpsertRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCalls++
	key := arg.Title
	if m.failAt != nil && m.failAt[key] {
		return StressUpsertRow{}, fmt.Errorf("injected error for %s", key)
	}
	return StressUpsertRow{ContentHash: arg.ContentHash, Collection: arg.Collection, WorkspaceHash: arg.WorkspaceHash}, nil
}

func (m *mockStressWriter) CountDocumentsByWorkspace(_ context.Context, _ string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.upsertCalls == 0 {
		return m.countBefore, nil
	}
	return m.countAfter, nil
}

func TestRunStress_Success(t *testing.T) {
	mock := &mockStressWriter{countBefore: 0, countAfter: 15}
	cfg := StressConfig{Concurrency: 3, DocsPerWriter: 5, WorkspaceHash: "test-ws"}

	result, err := RunStress(context.Background(), mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DocumentsWritten != 15 {
		t.Errorf("expected 15 written, got %d", result.DocumentsWritten)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations, got %d", result.Violations)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d", len(result.Errors))
	}
	if mock.upsertCalls != 15 {
		t.Errorf("expected 15 upsert calls, got %d", mock.upsertCalls)
	}
}

func TestRunStress_WithErrors(t *testing.T) {
	mock := &mockStressWriter{
		countBefore: 0,
		countAfter:  13,
		failAt: map[string]bool{
			"stress-g0-d0": true,
			"stress-g1-d2": true,
		},
	}
	cfg := StressConfig{Concurrency: 3, DocsPerWriter: 5, WorkspaceHash: "test-ws"}

	result, err := RunStress(context.Background(), mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DocumentsWritten != 13 {
		t.Errorf("expected 13 written, got %d", result.DocumentsWritten)
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(result.Errors), result.Errors)
	}
	if result.Violations != 1 {
		t.Errorf("expected 1 violation (count mismatch), got %d", result.Violations)
	}
}

func TestRunStress_SingleWriter(t *testing.T) {
	mock := &mockStressWriter{countBefore: 5, countAfter: 15}
	cfg := StressConfig{Concurrency: 1, DocsPerWriter: 10, WorkspaceHash: "test-ws"}

	result, err := RunStress(context.Background(), mock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DocumentsWritten != 10 {
		t.Errorf("expected 10 written, got %d", result.DocumentsWritten)
	}
	if result.Violations != 0 {
		t.Errorf("expected 0 violations, got %d", result.Violations)
	}
}

func TestRunStress_InvalidConfig(t *testing.T) {
	mock := &mockStressWriter{}

	_, err := RunStress(context.Background(), mock, StressConfig{Concurrency: 0, DocsPerWriter: 5, WorkspaceHash: "ws"})
	if err == nil {
		t.Fatal("expected error for concurrency=0")
	}

	_, err = RunStress(context.Background(), mock, StressConfig{Concurrency: 2, DocsPerWriter: 0, WorkspaceHash: "ws"})
	if err == nil {
		t.Fatal("expected error for docsPerWriter=0")
	}
}
