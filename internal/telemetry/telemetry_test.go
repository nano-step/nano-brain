package telemetry_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/telemetry"
	"github.com/rs/zerolog"
)

type mockWriter struct {
	mu      sync.Mutex
	calls   []sqlc.InsertSearchTelemetryParams
	errFunc func() error
}

func (m *mockWriter) InsertSearchTelemetry(_ context.Context, arg sqlc.InsertSearchTelemetryParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, arg)
	if m.errFunc != nil {
		return m.errFunc()
	}
	return nil
}

func (m *mockWriter) getCalls() []sqlc.InsertSearchTelemetryParams {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]sqlc.InsertSearchTelemetryParams, len(m.calls))
	copy(cp, m.calls)
	return cp
}

func TestRecorderWritesToMock(t *testing.T) {
	w := &mockWriter{}
	rec := telemetry.NewRecorder(w, zerolog.Nop())

	rec.Record(context.Background(), "test query", 5, 42, "memory", "ws123")

	deadline := time.After(2 * time.Second)
	for {
		if calls := w.getCalls(); len(calls) > 0 {
			c := calls[0]
			if c.WorkspaceHash != "ws123" {
				t.Errorf("expected workspace=ws123, got %s", c.WorkspaceHash)
			}
			if c.QueryText != "test query" {
				t.Errorf("expected query='test query', got %s", c.QueryText)
			}
			if c.ResultCount != 5 {
				t.Errorf("expected result_count=5, got %d", c.ResultCount)
			}
			if c.LatencyMs != 42 {
				t.Errorf("expected latency_ms=42, got %d", c.LatencyMs)
			}
			if c.Collection != "memory" {
				t.Errorf("expected collection=memory, got %s", c.Collection)
			}
			return
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for Record to write")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestRecorderDoesNotPanicOnError(t *testing.T) {
	w := &mockWriter{
		errFunc: func() error { return context.DeadlineExceeded },
	}
	rec := telemetry.NewRecorder(w, zerolog.Nop())

	rec.Record(context.Background(), "bad query", 0, 10, "", "ws123")

	time.Sleep(100 * time.Millisecond)

	if calls := w.getCalls(); len(calls) != 1 {
		t.Errorf("expected 1 call, got %d", len(calls))
	}
}
