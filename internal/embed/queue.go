package embed

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

const (
	channelCapacity    = 10000
	defaultConcurrency = 4
	scanBatchSize      = 1000
	rescanInterval     = 5 * time.Minute
	embedTimeout       = 2 * time.Minute
	backoffBase        = 60 * time.Second
	backoffMultiplier  = 1.5
	backoffMax         = 300 * time.Second
	rejectionThreshold = 50000
	maxRetries         = 3
)

type QueueQuerier interface {
	GetChunkByID(ctx context.Context, id uuid.UUID) (sqlc.GetChunkByIDRow, error)
	GetAllPendingChunks(ctx context.Context, limit int32) ([]uuid.UUID, error)
	GetAllFailedChunks(ctx context.Context, limit int32) ([]uuid.UUID, error)
	InsertEmbedding(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error)
	MarkChunkEmbedded(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error
	MarkChunkEmbedFailed(ctx context.Context, arg sqlc.MarkChunkEmbedFailedParams) error
}

type Queue struct {
	ch          chan uuid.UUID
	embedder    Embedder
	queries     QueueQuerier
	logger      zerolog.Logger
	provider    string
	model       string
	concurrency int
	backoff     backoffState
	mu          sync.Mutex
	pending        atomic.Int64
	retries        map[uuid.UUID]int
	retriesMu      sync.Mutex
	lastCapacityLog time.Time
}

type backoffState struct {
	current time.Duration
}

func NewQueue(embedder Embedder, queries QueueQuerier, logger zerolog.Logger, provider, model string, concurrency int) *Queue {
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	return &Queue{
		ch:          make(chan uuid.UUID, channelCapacity),
		embedder:    embedder,
		queries:     queries,
		logger:      logger.With().Str("component", "embed-queue").Logger(),
		provider:    provider,
		model:       model,
		concurrency: concurrency,
		backoff:     backoffState{current: 0},
		retries:     make(map[uuid.UUID]int),
	}
}

func (q *Queue) Enqueue(chunkID uuid.UUID) bool {
	if q.pending.Load() >= rejectionThreshold {
		q.logger.Warn().Str("chunk_id", chunkID.String()).
			Int64("pending", q.pending.Load()).
			Msg("backpressure: rejecting enqueue")
		return false
	}
	select {
	case q.ch <- chunkID:
		q.pending.Add(1)
		q.logger.Debug().Str("chunk_id", chunkID.String()).Msg("chunk enqueued")
		return true
	default:
		q.logger.Warn().Str("chunk_id", chunkID.String()).Msg("queue full, chunk dropped")
		return false
	}
}

// Depth returns the current channel length.
func (q *Queue) Depth() int { return len(q.ch) }

// Capacity returns the channel capacity.
func (q *Queue) Capacity() int { return channelCapacity }

// Status is advisory; reads are not atomic across fields.
func (q *Queue) Status() string {
	if q.pending.Load() >= rejectionThreshold {
		return "rejecting"
	}
	depth := len(q.ch)
	if depth == 0 {
		return "nominal"
	}
	if float64(depth) >= float64(channelCapacity)*0.6 {
		return "backpressure"
	}
	return "busy"
}

// IsPressured returns true when the total pending backlog reaches the rejection threshold.
func (q *Queue) IsPressured() bool {
	return q.pending.Load() >= rejectionThreshold
}

// PendingCount returns the current pending backlog size (for monitoring/testing).
func (q *Queue) PendingCount() int64 {
	return q.pending.Load()
}

func (q *Queue) Run(ctx context.Context) error {
	q.scanPending(ctx)

	rescanTicker := time.NewTicker(rescanInterval)
	defer rescanTicker.Stop()

	sem := make(chan struct{}, q.concurrency)
	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-rescanTicker.C:
			q.scanPending(ctx)
		case chunkID := <-q.ch:
			q.checkCapacity()
			sem <- struct{}{}
			wg.Add(1)
			go func(id uuid.UUID) {
				defer wg.Done()
				defer func() { <-sem }()
				q.processChunk(ctx, id)
			}(chunkID)
		}
	}
}

func (q *Queue) scanPending(ctx context.Context) {
	total := q.scanByStatus(ctx, false)
	recovered := q.scanByStatus(ctx, true)
	q.logger.Info().Int("pending", total).Int("recovered", recovered).Msg("scan complete")
}

func (q *Queue) scanByStatus(ctx context.Context, failed bool) int {
	total := 0
	for {
		var ids []uuid.UUID
		var err error
		if failed {
			ids, err = q.queries.GetAllFailedChunks(ctx, scanBatchSize)
		} else {
			ids, err = q.queries.GetAllPendingChunks(ctx, scanBatchSize)
		}
		if err != nil {
			q.logger.Error().Err(err).Bool("failed", failed).Msg("failed to scan chunks")
			return total
		}
		for _, id := range ids {
			if failed {
				q.clearRetries(id)
			}
			if !q.Enqueue(id) {
				q.logger.Info().Int("total", total).Bool("failed", failed).Msg("scan stopped (queue full)")
				return total
			}
			total++
		}
		if len(ids) < scanBatchSize {
			break
		}
	}
	return total
}

func (q *Queue) processChunk(ctx context.Context, chunkID uuid.UUID) {
	if ctx.Err() != nil {
		q.pending.Add(-1)
		return
	}

	q.mu.Lock()
	delay := q.backoff.current
	q.mu.Unlock()

	if delay > 0 {
		jitter := time.Duration(rand.Int63n(int64(delay / 4)))
		delay += jitter
		select {
		case <-ctx.Done():
			q.pending.Add(-1)
			return
		case <-time.After(delay):
		}
	}

	chunk, err := q.queries.GetChunkByID(ctx, chunkID)
	if err != nil {
		q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("failed to fetch chunk")
		q.pending.Add(-1)
		return
	}

	q.logger.Info().
		Str("chunk_id", chunkID.String()).
		Str("file", chunk.SourcePath).
		Msg("embedding chunk")

	embedCtx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()
	vec, err := q.embedder.Embed(embedCtx, chunk.Content)
	if err != nil {
		q.logger.Error().Err(err).
			Str("chunk_id", chunkID.String()).
			Str("file", chunk.SourcePath).
			Msg("embedding failed")
		q.increaseBackoff()
		q.handleRetry(ctx, chunkID, chunk.WorkspaceHash)
		return
	}

	_, err = q.queries.InsertEmbedding(ctx, sqlc.InsertEmbeddingParams{
		ChunkID:       chunkID,
		WorkspaceHash: chunk.WorkspaceHash,
		Provider:      q.provider,
		Model:         q.model,
		Embedding:     pgvector.NewVector(vec),
	})
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" {
			q.logger.Warn().Str("chunk_id", chunkID.String()).Msg("chunk deleted before embedding insert, skipping stale chunk")
			q.pending.Add(-1)
			q.clearRetries(chunkID)
			return
		}
		q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("insert embedding failed")
		q.pending.Add(-1)
		return
	}

	if err := q.queries.MarkChunkEmbedded(ctx, sqlc.MarkChunkEmbeddedParams{
		ID:            chunkID,
		WorkspaceHash: chunk.WorkspaceHash,
	}); err != nil {
		q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("mark embedded failed")
		q.pending.Add(-1)
		return
	}

	q.pending.Add(-1)
	q.clearRetries(chunkID)
	q.resetBackoff()
	q.logger.Info().
		Str("chunk_id", chunkID.String()).
		Str("file", chunk.SourcePath).
		Msg("chunk embedded")
}

func (q *Queue) increaseBackoff() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.backoff.current == 0 {
		q.backoff.current = backoffBase
	} else {
		q.backoff.current = time.Duration(float64(q.backoff.current) * backoffMultiplier)
	}
	if q.backoff.current > backoffMax {
		q.backoff.current = backoffMax
	}
}

func (q *Queue) resetBackoff() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.backoff.current = 0
}

func (q *Queue) handleRetry(ctx context.Context, chunkID uuid.UUID, workspaceHash string) {
	q.retriesMu.Lock()
	q.retries[chunkID]++
	count := q.retries[chunkID]
	q.retriesMu.Unlock()

	if count >= maxRetries {
		if err := q.queries.MarkChunkEmbedFailed(ctx, sqlc.MarkChunkEmbedFailedParams{
			ID:            chunkID,
			WorkspaceHash: workspaceHash,
		}); err != nil {
			q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("mark embed_failed failed")
		}
		q.pending.Add(-1)
		q.clearRetries(chunkID)
		q.logger.Warn().Str("chunk_id", chunkID.String()).Int("retries", count).
			Msg("chunk marked embed_failed after max retries")
		return
	}

	select {
	case q.ch <- chunkID:
		q.logger.Debug().Str("chunk_id", chunkID.String()).Int("retry", count).Msg("chunk re-enqueued for retry")
	default:
		q.pending.Add(-1)
		q.logger.Warn().Str("chunk_id", chunkID.String()).Msg("retry re-enqueue failed, will be picked up on scan")
	}
}

func (q *Queue) clearRetries(chunkID uuid.UUID) {
	q.retriesMu.Lock()
	delete(q.retries, chunkID)
	q.retriesMu.Unlock()
}

func (q *Queue) checkCapacity() {
	depth := len(q.ch)
	threshold90 := int(float64(channelCapacity) * 0.9)
	threshold60 := int(float64(channelCapacity) * 0.6)

	if depth < threshold60 {
		return
	}

	now := time.Now()
	q.mu.Lock()
	if now.Sub(q.lastCapacityLog) < time.Minute {
		q.mu.Unlock()
		return
	}
	q.lastCapacityLog = now
	q.mu.Unlock()

	if depth >= threshold90 {
		q.logger.Error().Int("queue_depth", depth).Int64("pending_total", q.pending.Load()).
			Msg("queue at 90% capacity")
	} else {
		q.logger.Warn().Int("queue_depth", depth).Int64("pending_total", q.pending.Load()).
			Msg("queue at 60% capacity")
	}
}
