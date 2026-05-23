package embed

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	pgvector "github.com/pgvector/pgvector-go"
	"github.com/rs/zerolog"

	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

const (
	channelCapacity     = 10000
	defaultConcurrency  = 4
	startupScanLimit    = 1000
	backoffBase         = 60 * time.Second
	backoffMultiplier   = 1.5
	backoffMax          = 300 * time.Second
)

type QueueQuerier interface {
	GetChunkByID(ctx context.Context, id uuid.UUID) (sqlc.GetChunkByIDRow, error)
	GetAllPendingChunks(ctx context.Context, limit int32) ([]uuid.UUID, error)
	InsertEmbedding(ctx context.Context, arg sqlc.InsertEmbeddingParams) (sqlc.Embedding, error)
	MarkChunkEmbedded(ctx context.Context, arg sqlc.MarkChunkEmbeddedParams) error
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
	}
}

func (q *Queue) Enqueue(chunkID uuid.UUID) bool {
	select {
	case q.ch <- chunkID:
		q.logger.Debug().Str("chunk_id", chunkID.String()).Msg("chunk enqueued")
		return true
	default:
		q.logger.Warn().Str("chunk_id", chunkID.String()).Msg("queue full, chunk dropped")
		return false
	}
}

func (q *Queue) Run(ctx context.Context) error {
	q.scanPending(ctx)

	sem := make(chan struct{}, q.concurrency)
	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case chunkID := <-q.ch:
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
	ids, err := q.queries.GetAllPendingChunks(ctx, startupScanLimit)
	if err != nil {
		q.logger.Error().Err(err).Msg("startup scan failed")
		return
	}
	enqueued := 0
	for _, id := range ids {
		if q.Enqueue(id) {
			enqueued++
		}
	}
	q.logger.Info().Int("count", enqueued).Msg("startup scan: re-queued pending chunks")
}

func (q *Queue) processChunk(ctx context.Context, chunkID uuid.UUID) {
	if ctx.Err() != nil {
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
			return
		case <-time.After(delay):
		}
	}

	chunk, err := q.queries.GetChunkByID(ctx, chunkID)
	if err != nil {
		q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("failed to fetch chunk")
		return
	}

	vec, err := q.embedder.Embed(ctx, chunk.Content)
	if err != nil {
		q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("embedding failed")
		q.increaseBackoff()
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
		q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("insert embedding failed")
		return
	}

	if err := q.queries.MarkChunkEmbedded(ctx, sqlc.MarkChunkEmbeddedParams{
		ID:            chunkID,
		WorkspaceHash: chunk.WorkspaceHash,
	}); err != nil {
		q.logger.Error().Err(err).Str("chunk_id", chunkID.String()).Msg("mark embedded failed")
		return
	}

	q.resetBackoff()
	q.logger.Debug().Str("chunk_id", chunkID.String()).Msg("chunk embedded")
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
