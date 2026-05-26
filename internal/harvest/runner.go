package harvest

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Harvester can scan a source and ingest sessions into the document store.
type Harvester interface {
	HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int)
}

// summarizerSettable is implemented by harvesters that accept a SessionSummarizer.
type summarizerSettable interface {
	setSummarizer(SessionSummarizer)
}

// Runner periodically invokes one or more Harvesters.
type Runner struct {
	mu         sync.Mutex
	harvesters []Harvester
	enqueuer   ChunkEnqueuer
	interval   time.Duration
	logger     zerolog.Logger
	summarizer SessionSummarizer
}

// NewRunner creates a Runner that calls HarvestAll at the given interval.
func NewRunner(harvester Harvester, enqueuer ChunkEnqueuer, interval time.Duration, logger zerolog.Logger) *Runner {
	return &Runner{
		harvesters: []Harvester{harvester},
		enqueuer:   enqueuer,
		interval:   interval,
		logger:     logger.With().Str("component", "harvest-runner").Logger(),
	}
}

// AddHarvester appends an additional harvester to the runner.
func (r *Runner) AddHarvester(h Harvester) {
	r.harvesters = append(r.harvesters, h)
	if r.summarizer != nil {
		if ss, ok := h.(summarizerSettable); ok {
			ss.setSummarizer(r.summarizer)
		}
	}
}

func (r *Runner) WithSummarizer(s SessionSummarizer) *Runner {
	r.summarizer = s
	for _, h := range r.harvesters {
		if ss, ok := h.(summarizerSettable); ok {
			ss.setSummarizer(s)
		}
	}
	return r
}

// Run executes an immediate harvest then ticks at the configured interval.
// It returns nil on context cancellation.
func (r *Runner) Run(ctx context.Context) error {
	r.tick(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info().Msg("harvest runner stopping")
			return nil
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

// RunOnce executes a single harvest cycle synchronously and returns aggregate counts.
// Serialized by mutex to prevent overlapping ticker and API-triggered harvests.
func (r *Runner) RunOnce(ctx context.Context) (harvested, skipped, errCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, harv := range r.harvesters {
		h, s, e := harv.HarvestAll(ctx, r.enqueuer)
		harvested += h
		skipped += s
		errCount += e
	}
	r.logger.Info().
		Int("harvested", harvested).
		Int("skipped", skipped).
		Int("errors", errCount).
		Msg("harvest cycle complete")
	return
}

func (r *Runner) tick(ctx context.Context) {
	r.RunOnce(ctx)
}
