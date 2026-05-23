package harvest

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// Harvester can scan a source and ingest sessions into the document store.
type Harvester interface {
	HarvestAll(ctx context.Context, enqueuer ChunkEnqueuer) (harvested, skipped, errCount int)
}

// Runner periodically invokes one or more Harvesters.
type Runner struct {
	harvesters []Harvester
	enqueuer   ChunkEnqueuer
	interval   time.Duration
	logger     zerolog.Logger
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

func (r *Runner) tick(ctx context.Context) {
	var totalHarvested, totalSkipped, totalErrors int
	for _, h := range r.harvesters {
		harvested, skipped, errCount := h.HarvestAll(ctx, r.enqueuer)
		totalHarvested += harvested
		totalSkipped += skipped
		totalErrors += errCount
	}
	r.logger.Info().
		Int("harvested", totalHarvested).
		Int("skipped", totalSkipped).
		Int("errors", totalErrors).
		Msg("harvest cycle complete")
}
