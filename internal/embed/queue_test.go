package embed

import (
	"context"
	"errors"
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
	mu                                    sync.Mutex
	getChunkByIDFn                        func(ctx context.Context, id uuid.UUID) (sqlc.GetChunkByIDRow, error)
	getPendingChunksAllWorkspacesFn       func(ctx context.Context, limit int32) ([]uuid.UUID, error)
	getFailedChunksAllWorkspacesFn        func(ctx context.Context, limit int32) ([]uuid.UUID, error)
	insertEmbeddingFn                     func(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error)
	markChunkEmbeddedFn                   func(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error
	markChunkEmbedFailedFn                func(ctx context.Context, arg sqlc.MarkChunkEmbedFailedParams) error
	markChunkEmbedPermanentlyFailedFn     func(ctx context.Context, arg sqlc.MarkChunkEmbedPermanentlyFailedParams) error
	insertEmbeddingCalls                  int
	markChunkEmbeddedCalls                int
	markChunkEmbedFailedCalls             int
	markChunkEmbedPermanentlyFailedCalls  int
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

func (m *mockQuerier) GetPendingChunksAllWorkspaces(ctx context.Context, limit int32) ([]uuid.UUID, error) {
	if m.getPendingChunksAllWorkspacesFn != nil {
		return m.getPendingChunksAllWorkspacesFn(ctx, limit)
	}
	return nil, nil
}

func (m *mockQuerier) GetFailedChunksAllWorkspaces(ctx context.Context, limit int32) ([]uuid.UUID, error) {
	if m.getFailedChunksAllWorkspacesFn != nil {
		return m.getFailedChunksAllWorkspacesFn(ctx, limit)
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

func (m *mockQuerier) MarkChunkEmbedFailed(ctx context.Context, arg sqlc.MarkChunkEmbedFailedParams) error {
	m.mu.Lock()
	m.markChunkEmbedFailedCalls++
	m.mu.Unlock()
	if m.markChunkEmbedFailedFn != nil {
		return m.markChunkEmbedFailedFn(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) MarkChunkEmbedPermanentlyFailed(ctx context.Context, arg sqlc.MarkChunkEmbedPermanentlyFailedParams) error {
	m.mu.Lock()
	m.markChunkEmbedPermanentlyFailedCalls++
	m.mu.Unlock()
	if m.markChunkEmbedPermanentlyFailedFn != nil {
		return m.markChunkEmbedPermanentlyFailedFn(ctx, arg)
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
	eq.pending.Store(1)
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
	eq.pending.Store(1)
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

func TestQueue_ScanPendingSingleBatch(t *testing.T) {
	pending := []uuid.UUID{uuid.New(), uuid.New()}
	mq := &mockQuerier{
		getPendingChunksAllWorkspacesFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			return pending, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.scanPending(context.Background())

	if len(eq.ch) != 2 {
		t.Errorf("channel len = %d, want 2", len(eq.ch))
	}
}

func TestQueue_ScanPendingMultiBatch(t *testing.T) {
	callCount := 0
	mq := &mockQuerier{
		getPendingChunksAllWorkspacesFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			callCount++
			if callCount == 1 {
				ids := make([]uuid.UUID, scanBatchSize)
				for i := range ids {
					ids[i] = uuid.New()
				}
				return ids, nil
			}
			return []uuid.UUID{uuid.New(), uuid.New()}, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.scanPending(context.Background())

	if callCount != 2 {
		t.Errorf("GetPendingChunksAllWorkspaces called %d times, want 2", callCount)
	}
	if len(eq.ch) != scanBatchSize+2 {
		t.Errorf("channel len = %d, want %d", len(eq.ch), scanBatchSize+2)
	}
}

func TestQueue_ScanPendingStopsWhenQueueFull(t *testing.T) {
	mq := &mockQuerier{
		getPendingChunksAllWorkspacesFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			ids := make([]uuid.UUID, scanBatchSize)
			for i := range ids {
				ids[i] = uuid.New()
			}
			return ids, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)

	prefill := channelCapacity - 5
	for i := 0; i < prefill; i++ {
		eq.ch <- uuid.New()
	}

	eq.scanPending(context.Background())

	if len(eq.ch) != channelCapacity {
		t.Errorf("channel len = %d, want %d (full)", len(eq.ch), channelCapacity)
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

func TestQueue_ProcessChunk_EmbedHasDeadline(t *testing.T) {
	chunkID := uuid.New()
	var capturedDeadline time.Time
	var hasDeadline bool
	me := &mockEmbedder{
		embedFn: func(ctx context.Context, text string) ([]float32, error) {
			capturedDeadline, hasDeadline = ctx.Deadline()
			return []float32{0.1, 0.2, 0.3}, nil
		},
	}
	mq := &mockQuerier{}

	eq := newTestQueue(me, mq)
	eq.pending.Store(1)
	eq.processChunk(context.Background(), chunkID)

	if !hasDeadline {
		t.Fatal("embed context should have a deadline")
	}
	remaining := time.Until(capturedDeadline)
	if remaining > embedTimeout || remaining < 0 {
		t.Errorf("deadline remaining = %v, expected within (0, %v]", remaining, embedTimeout)
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

func TestQueue_RetryAndFail(t *testing.T) {
	chunkID := uuid.New()
	me := &mockEmbedder{
		embedFn: func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("provider down")
		},
	}
	mq := &mockQuerier{}
	eq := newTestQueue(me, mq)

	eq.pending.Store(1)

	for i := 0; i < maxRetries; i++ {
		eq.resetBackoff()
		eq.processChunk(context.Background(), chunkID)
	}

	mq.mu.Lock()
	failedCalls := mq.markChunkEmbedFailedCalls
	mq.mu.Unlock()
	if failedCalls != 1 {
		t.Errorf("MarkChunkEmbedFailed calls = %d, want 1", failedCalls)
	}

	if eq.pending.Load() != 0 {
		t.Errorf("pending = %d, want 0 after embed_failed", eq.pending.Load())
	}

	eq.retriesMu.Lock()
	_, exists := eq.retries[chunkID]
	eq.retriesMu.Unlock()
	if exists {
		t.Error("retry entry should be cleared after marking embed_failed")
	}
}

func TestQueue_PendingCounter(t *testing.T) {
	me := &mockEmbedder{}
	mq := &mockQuerier{}
	eq := newTestQueue(me, mq)

	id1 := uuid.New()
	id2 := uuid.New()

	if !eq.Enqueue(id1) {
		t.Fatal("enqueue id1 failed")
	}
	if eq.pending.Load() != 1 {
		t.Errorf("pending after enqueue = %d, want 1", eq.pending.Load())
	}

	if !eq.Enqueue(id2) {
		t.Fatal("enqueue id2 failed")
	}
	if eq.pending.Load() != 2 {
		t.Errorf("pending after second enqueue = %d, want 2", eq.pending.Load())
	}

	<-eq.ch
	eq.processChunk(context.Background(), id1)
	if eq.pending.Load() != 1 {
		t.Errorf("pending after success = %d, want 1", eq.pending.Load())
	}
}

func TestQueue_BackpressureReject(t *testing.T) {
	me := &mockEmbedder{}
	mq := &mockQuerier{}
	eq := newTestQueue(me, mq)

	eq.pending.Store(rejectionThreshold)

	if eq.Enqueue(uuid.New()) {
		t.Fatal("expected Enqueue to return false at rejection threshold")
	}

	if eq.pending.Load() != rejectionThreshold {
		t.Errorf("pending should not change on rejection, got %d", eq.pending.Load())
	}
}

func TestQueue_IsPressured(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})

	if eq.IsPressured() {
		t.Fatal("should not be pressured at zero pending")
	}

	eq.pending.Store(rejectionThreshold - 1)
	if eq.IsPressured() {
		t.Fatal("should not be pressured below threshold")
	}

	eq.pending.Store(rejectionThreshold)
	if !eq.IsPressured() {
		t.Fatal("should be pressured at threshold")
	}

	eq.pending.Store(rejectionThreshold + 100)
	if !eq.IsPressured() {
		t.Fatal("should be pressured above threshold")
	}
}

func TestQueue_RetryReenqueue(t *testing.T) {
	chunkID := uuid.New()
	me := &mockEmbedder{
		embedFn: func(_ context.Context, _ string) ([]float32, error) {
			return nil, fmt.Errorf("provider down")
		},
	}
	mq := &mockQuerier{}
	eq := newTestQueue(me, mq)
	eq.pending.Store(1)
	eq.resetBackoff()

	eq.processChunk(context.Background(), chunkID)

	eq.retriesMu.Lock()
	count := eq.retries[chunkID]
	eq.retriesMu.Unlock()
	if count != 1 {
		t.Errorf("retry count = %d, want 1", count)
	}

	mq.mu.Lock()
	failedCalls := mq.markChunkEmbedFailedCalls
	mq.mu.Unlock()
	if failedCalls != 0 {
		t.Errorf("MarkChunkEmbedFailed should not be called on first failure, got %d", failedCalls)
	}
}

func TestQueue_PendingDecrementsOnGetChunkError(t *testing.T) {
	mq := &mockQuerier{
		getChunkByIDFn: func(_ context.Context, _ uuid.UUID) (sqlc.GetChunkByIDRow, error) {
			return sqlc.GetChunkByIDRow{}, fmt.Errorf("not found")
		},
	}
	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.pending.Store(1)

	eq.processChunk(context.Background(), uuid.New())

	if eq.pending.Load() != 0 {
		t.Errorf("pending = %d, want 0 after GetChunkByID error", eq.pending.Load())
	}
}

func TestQueue_PendingDecrementsOnInsertEmbeddingError(t *testing.T) {
	mq := &mockQuerier{
		insertEmbeddingFn: func(_ context.Context, _ sqlc.InsertEmbeddingParams) (sqlc.Embedding, error) {
			return sqlc.Embedding{}, fmt.Errorf("db error")
		},
	}
	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.pending.Store(1)

	eq.processChunk(context.Background(), uuid.New())

	if eq.pending.Load() != 0 {
		t.Errorf("pending = %d, want 0 after InsertEmbedding error", eq.pending.Load())
	}
}

func TestQueue_Depth(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	if eq.Depth() != 0 {
		t.Errorf("initial depth = %d, want 0", eq.Depth())
	}

	eq.Enqueue(uuid.New())
	eq.Enqueue(uuid.New())
	if eq.Depth() != 2 {
		t.Errorf("depth after 2 enqueues = %d, want 2", eq.Depth())
	}

	<-eq.ch
	if eq.Depth() != 1 {
		t.Errorf("depth after 1 dequeue = %d, want 1", eq.Depth())
	}
}

func TestQueue_Capacity(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	if eq.Capacity() != channelCapacity {
		t.Errorf("capacity = %d, want %d", eq.Capacity(), channelCapacity)
	}
}

func TestQueue_Status_Nominal(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	if s := eq.Status(); s != "nominal" {
		t.Errorf("status = %q, want nominal", s)
	}
}

func TestQueue_Status_Busy(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	eq.ch <- uuid.New()
	if s := eq.Status(); s != "busy" {
		t.Errorf("status = %q, want busy", s)
	}
}

func TestQueue_Status_Backpressure(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	threshold := int(float64(channelCapacity) * 0.6)
	for i := 0; i < threshold; i++ {
		eq.ch <- uuid.New()
	}
	if s := eq.Status(); s != "backpressure" {
		t.Errorf("status = %q, want backpressure", s)
	}
}

func TestQueue_Status_Rejecting(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	eq.pending.Store(rejectionThreshold)
	if s := eq.Status(); s != "rejecting" {
		t.Errorf("status = %q, want rejecting", s)
	}
}

func TestQueue_ScanPendingRecoversFailed(t *testing.T) {
	failedID := uuid.New()
	mq := &mockQuerier{
		getFailedChunksAllWorkspacesFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			return []uuid.UUID{failedID}, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.retries[failedID] = maxRetries

	eq.scanPending(context.Background())

	eq.retriesMu.Lock()
	count := eq.retries[failedID]
	eq.retriesMu.Unlock()
	if count != 0 {
		t.Errorf("retries[failedID] = %d after scan, want 0 (cleared for fresh attempt)", count)
	}

	found := false
	for i := 0; i < len(eq.ch); i++ {
		id := <-eq.ch
		if id == failedID {
			found = true
			break
		}
	}
	if !found {
		t.Error("failedID not enqueued by scanPending, want it re-enqueued for retry")
	}
}

func TestQueue_PendingDecrementsOnMarkEmbeddedError(t *testing.T) {
	mq := &mockQuerier{
		markChunkEmbeddedFn: func(_ context.Context, _ sqlc.MarkChunkEmbeddedParams) error {
			return fmt.Errorf("db error")
		},
	}
	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.pending.Store(1)

	eq.processChunk(context.Background(), uuid.New())

	if eq.pending.Load() != 0 {
		t.Errorf("pending = %d, want 0 after MarkChunkEmbedded error", eq.pending.Load())
	}
}

func TestScanPending_WorkspaceScoped(t *testing.T) {
	mq := &mockQuerier{
		getPendingChunksAllWorkspacesFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			return nil, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.scanPending(context.Background())

	if len(eq.ch) != 0 {
		t.Errorf("channel len = %d, want 0: unregistered workspace chunks must not be enqueued", len(eq.ch))
	}
}

func TestScanPending_RegisteredOnly(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	mq := &mockQuerier{
		getPendingChunksAllWorkspacesFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			return []uuid.UUID{id1, id2}, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.scanPending(context.Background())

	if len(eq.ch) != 2 {
		t.Errorf("channel len = %d, want 2: registered workspace chunks must be enqueued", len(eq.ch))
	}
}

func TestScanFailed_WorkspaceScoped(t *testing.T) {
	mq := &mockQuerier{
		getFailedChunksAllWorkspacesFn: func(ctx context.Context, limit int32) ([]uuid.UUID, error) {
			return nil, nil
		},
	}

	eq := newTestQueue(&mockEmbedder{}, mq)
	eq.scanPending(context.Background())

	if len(eq.ch) != 0 {
		t.Errorf("channel len = %d, want 0: failed chunks for unregistered workspace must not be enqueued", len(eq.ch))
	}
}

func TestIsDeterministicEmbedError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("network timeout"), false},
		{"ollama_context_length", errors.New(`ollama: unexpected status 400: {"error":"the input length exceeds the context length"}`), true},
		{"context_length_lowercase", errors.New("context length exceeded"), true},
		{"input_length", errors.New("input length too large"), true},
		{"exceeds_context", errors.New("text exceeds context window"), true},
		{"maximum_context", errors.New("input larger than maximum context"), true},
		{"transient_500", errors.New("ollama: unexpected status 500: internal error"), false},
		{"transient_503", errors.New("ollama: unexpected status 503: service unavailable"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isDeterministicEmbedError(tc.err)
			if got != tc.want {
				t.Errorf("isDeterministicEmbedError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestWithMaxChars(t *testing.T) {
	eq := newTestQueue(&mockEmbedder{}, &mockQuerier{})
	if eq.maxChars != defaultMaxEmbedChars {
		t.Errorf("default maxChars = %d, want %d", eq.maxChars, defaultMaxEmbedChars)
	}
	eq.WithMaxChars(2000)
	if eq.maxChars != 2000 {
		t.Errorf("after WithMaxChars(2000), got %d", eq.maxChars)
	}
	eq.WithMaxChars(0)
	if eq.maxChars != 2000 {
		t.Errorf("WithMaxChars(0) should be no-op, got %d", eq.maxChars)
	}
	eq.WithMaxChars(-1)
	if eq.maxChars != 2000 {
		t.Errorf("WithMaxChars(-1) should be no-op, got %d", eq.maxChars)
	}
}

func TestProcessChunk_DeterministicErrorMarksPermanentlyFailed(t *testing.T) {
	chunkID := uuid.New()
	mq := &mockQuerier{
		getChunkByIDFn: func(ctx context.Context, id uuid.UUID) (sqlc.GetChunkByIDRow, error) {
			return sqlc.GetChunkByIDRow{
				ID:            chunkID,
				WorkspaceHash: "ws1",
				Content:       "some content",
			}, nil
		},
	}
	me := &mockEmbedder{
		embedFn: func(ctx context.Context, text string) ([]float32, error) {
			return nil, errors.New(`ollama: unexpected status 400: {"error":"the input length exceeds the context length"}`)
		},
	}
	eq := newTestQueue(me, mq)
	eq.pending.Add(1)
	eq.processChunk(context.Background(), chunkID)

	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.markChunkEmbedPermanentlyFailedCalls != 1 {
		t.Errorf("MarkChunkEmbedPermanentlyFailed calls = %d, want 1", mq.markChunkEmbedPermanentlyFailedCalls)
	}
	if mq.markChunkEmbedFailedCalls != 0 {
		t.Errorf("MarkChunkEmbedFailed calls = %d, want 0 (deterministic must skip retry path)", mq.markChunkEmbedFailedCalls)
	}
	if eq.pending.Load() != 0 {
		t.Errorf("pending counter = %d, want 0 (permanent failure must decrement)", eq.pending.Load())
	}
}

func TestProcessChunk_TransientErrorStillRetries(t *testing.T) {
	chunkID := uuid.New()
	mq := &mockQuerier{
		getChunkByIDFn: func(ctx context.Context, id uuid.UUID) (sqlc.GetChunkByIDRow, error) {
			return sqlc.GetChunkByIDRow{
				ID:            chunkID,
				WorkspaceHash: "ws1",
				Content:       "some content",
			}, nil
		},
	}
	me := &mockEmbedder{
		embedFn: func(ctx context.Context, text string) ([]float32, error) {
			return nil, errors.New("connection refused")
		},
	}
	eq := newTestQueue(me, mq)
	eq.pending.Add(1)
	eq.processChunk(context.Background(), chunkID)

	mq.mu.Lock()
	defer mq.mu.Unlock()
	if mq.markChunkEmbedPermanentlyFailedCalls != 0 {
		t.Errorf("MarkChunkEmbedPermanentlyFailed calls = %d, want 0 (transient must NOT permanently fail)", mq.markChunkEmbedPermanentlyFailedCalls)
	}
}
