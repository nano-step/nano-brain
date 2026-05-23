package embed

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockEmbedder struct {
	mu       sync.Mutex
	embedFn  func(ctx context.Context, text string) ([]float32, error)
	calls    int
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	if m.embedFn != nil {
		return m.embedFn(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (m *mockEmbedder) Dimension() int { return 3 }

func (m *mockEmbedder) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

type mockQuerier struct {
	mu                     sync.Mutex
	getChunkByIDFn         func(ctx context.Context, id uuid.UUID) (sqlc.GetChunkByIDRow, error)
	getAllPendingChunksFn   func(ctx context.Context, limit int32) ([]uuid.UUID, error)
	insertEmbeddingFn      func(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error)
	markChunkEmbeddedFn    func(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error
	insertEmbeddingCalls   int
	markChunkEmbeddedCalls int
}

func (m *mockQuerier) GetChunkByID(ctx context.Context, id uuid.UUID) (sqlc.GetChunkByIDRow, error) {
	if m.getChunkByIDFn != nil {
		return m.getChunkByIDFn(ctx, id)
	}
	return sqlc.GetChunkByIDRow{
		ID:            id,
		WorkspaceHash: "ws1",
		Content:       "test content",
	}, nil
}

func (m *mockQuerier) GetAllPendingChunks(ctx context.Context, limit int32) ([]uuid.UUID, error) {
	if m.getAllPendingChunksFn != nil {
		return m.getAllPendingChunksFn(ctx, limit)
	}
	return nil, nil
}

func (m *mockQuerier) InsertEmbedding(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error) {
	m.mu.Lock()
	m.insertEmbeddingCalls++
	m.mu.Unlock()
	if m.insertEmbeddingFn != nil {
		return m.insertEmbeddingFn(ctx, arg)
	}
	return sqlc.Embedding{}, nil
}

func (m *mockQuerier) MarkChunkEmbedded(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error {
	m.mu.Lock()
	m.markChunkEmbeddedCalls++
	m.mu.Unlock()
	if m.markChunkEmbeddedFn != nil {
		return m.markChunkEmbeddedFn(ctx, arg)
	}
	return nil
}

func newTestQueue(e Embedder, q QueueQuerier) *Queue {
	return NewQueue(e, q, zerolog.Nop(), "test-provider", "test-model", 2)
}

func TestQueue_Enqueue(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	id := uuid.New()
	if !eq.Enqueue(id) {
		t.Fatal("expected Enqueue to return true")
	}
	select {
	case got := <-eq.ch:
		if got != id {
			t.Errorf("got %s, want %s", got, id)
		}
	default:
		t.Fatal("channel should have one item")
	}
}

func TestQueue_EnqueueFull(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	for i := 0; i < channelCapacity; i++ {
		if !eq.Enqueue(uuid.New()) {
			t.Fatalf("enqueue failed at %d", i)
		}
	}
	if eq.Enqueue(uuid.New()) {
		t.Fatal("expected Enqueue to return false when channel is full")
	}
}

func TestQueue_ProcessChunk_Success(t *testing.T) {
	chunkID := uuid.New()
	me := &mockEmbedder{}
	mq := &mockQuerier{}

	eq := newTestQueue(me, mq)
	eq.processChunk(context.Background(), chunkID)

	if me.callCount() != 1 {
		t.Errorf("embed calls = %d, want 1", me.callCount())
	}
	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.insertEmbeddingCalls != 1 {
		t.Errorf("insertEmbedding calls = %d, want 1", mq.insertEmbeddingCalls)
	}
	if mq.markChunkEmbeddedCalls != 1 {
		t.Errorf("markChunkEmbedded calls = %d, want 1", mq.markChunkEmbeddedCalls)
	}
}

func TestQueue_ProcessChunk_EmbedFailure(t *testing.T) {
	chunkID := uuid.New()
	me := &mockEmbedder{
		embedFn: func(ctx context.Context, text string) ([]float32, error) {
			return nil, fmt.Errorf("provider down")
		},
	}
	mq := &mockQuerier{}

	eq := newTestQueue(me, mq)
	eq.processChunk(context.Background(), chunkID)

	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.insertEmbeddingCalls != 0 {
		t.Errorf("insertEmbedding calls = %d, want 0", mq.insertEmbeddingCalls)
	}
	if mq.markChunkEmbeddedCalls != 0 {
		t.Errorf("markChunkEmbedded calls = %d, want 0", mq.markChunkEmbeddedCalls)
	}
}

func TestQueue_Backoff(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})

	if eq.backoff.current != 0 {
		t.Fatalf("initial backoff = %v, want 0", eq.backoff.current)
	}

	eq.increaseBackoff()
	if eq.backoff.current != backoffBase {
		t.Errorf("after first failure = %v, want %v", eq.backoff.current, backoffBase)
	}

	eq.increaseBackoff()
	want := time.Duration(float64(backoffBase) * backoffMultiplier)
	if eq.backoff.current != want {
		t.Errorf("after second failure = %v, want %v", eq.backoff.current, want)
	}

	eq.resetBackoff()
	if eq.backoff.current != 0 {
		t.Errorf("after reset = %v, want 0", eq.backoff.current)
	}
}

func TestQueue_BackoffMax(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})

	for i := 0; i < 100; i++ {
		eq.increaseBackoff()
	}
	if eq.backoff.current > backoffMax {
		t.Errorf("backoff %v exceeds max %v", eq.backoff.current, backoffMax)
	}
}

func TestQueue_RunStartupScan(t *testing.T) {
	pending := []uuid.UUID{uuid.New(), uuid.New()}
	mq := &mockQuerier{
		getAllPendingChunksFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			return pending, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.scanPending(context.Background())

	if len(eq.ch) != 2 {
		t.Errorf("channel len = %d, want 2", len(eq.ch))
	}
}

func TestQueue_RunContextCancellation(t *testing.T) {
	mq := &mockQuerier{}
	eq := newTestQueue(&mockEmbedder{}, mq)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- eq.Run(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}

func TestNewQueue_DefaultConcurrency(t *testing.T) {
	eq := NewQueue(&mockEmbedder{}, &mockQuerier{}, zerolog.Nop(), "p", "m", 0)
	if eq.concurrency != defaultConcurrency {
		t.Errorf("concurrency = %d, want %d", eq.concurrency, defaultConcurrency)
	}
}

func TestNewQueue_NegativeConcurrency(t *testing.T) {
	eq := NewQueue(&mockEmbedder{}, &mockQuerier{}, zerolog.Nop(), "p", "m", -1)
	if eq.concurrency != defaultConcurrency {
		t.Errorf("concurrency = %d, want %d", eq.concurrency, defaultConcurrency)
	}
}
